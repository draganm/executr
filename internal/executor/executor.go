package executor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/draganm/executr/internal/models"
	"github.com/draganm/executr/pkg/client"
	"github.com/google/uuid"
)

type Config struct {
	ServerURL         string
	Name              string
	CacheDir          string
	WorkDir           string
	MaxJobs           int
	PollInterval      int
	MaxCacheSize      int
	HeartbeatInterval int
	NetworkTimeout    int
}

type Executor struct {
	cfg        *Config
	client     client.Client
	cache      *BinaryCache
	executorID string
	
	// Job tracking
	runningJobs sync.Map
	jobSem      chan struct{}
	
	// Shutdown coordination
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(cfg *Config) (*Executor, error) {
	// Expand home directory in cache dir
	if cfg.CacheDir[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		cfg.CacheDir = filepath.Join(home, cfg.CacheDir[2:])
	}
	
	// Generate unique executor ID
	executorID := fmt.Sprintf("%s-%s", cfg.Name, uuid.New().String()[:8])
	
	// Create client
	c := client.New(cfg.ServerURL)
	
	// Create binary cache
	cache, err := NewBinaryCache(cfg.CacheDir, cfg.MaxCacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create binary cache: %w", err)
	}
	
	// Create work directory
	if err := os.MkdirAll(cfg.WorkDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}
	
	return &Executor{
		cfg:        cfg,
		client:     c,
		cache:      cache,
		executorID: executorID,
		jobSem:     make(chan struct{}, cfg.MaxJobs),
	}, nil
}

func (e *Executor) Run(ctx context.Context) error {
	e.ctx, e.cancel = context.WithCancel(ctx)
	defer e.cancel()
	
	slog.Info("Starting executor", 
		"executor_id", e.executorID,
		"name", e.cfg.Name,
		"max_jobs", e.cfg.MaxJobs,
		"cache_dir", e.cfg.CacheDir,
		"work_dir", e.cfg.WorkDir,
	)
	
	// Clean up orphaned job directories from previous runs
	e.cleanupOrphanedDirectories()
	
	// Start polling for jobs
	e.wg.Add(1)
	go e.pollForJobs()
	
	// Wait for shutdown signal
	<-e.ctx.Done()
	slog.Info("Shutting down executor, waiting for running jobs to complete...")
	
	// Wait for all jobs to complete
	e.wg.Wait()
	
	slog.Info("Executor shutdown complete")
	return nil
}

func (e *Executor) cleanupOrphanedDirectories() {
	entries, err := os.ReadDir(e.cfg.WorkDir)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("Failed to read work directory for cleanup", "error", err)
		}
		return
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			dirPath := filepath.Join(e.cfg.WorkDir, entry.Name())
			if err := os.RemoveAll(dirPath); err != nil {
				slog.Warn("Failed to remove orphaned job directory", 
					"path", dirPath,
					"error", err,
				)
			} else {
				slog.Debug("Removed orphaned job directory", "path", dirPath)
			}
		}
	}
}

func (e *Executor) pollForJobs() {
	defer e.wg.Done()
	
	pollTicker := time.NewTicker(time.Duration(e.cfg.PollInterval) * time.Second)
	defer pollTicker.Stop()
	
	networkFailureStart := time.Time{}
	
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-pollTicker.C:
			// Try to claim a job if we have capacity
			select {
			case e.jobSem <- struct{}{}:
				job, err := e.claimJob()
				if err != nil {
					<-e.jobSem // Release semaphore
					
					// Track network failures
					if networkFailureStart.IsZero() {
						networkFailureStart = time.Now()
					} else if time.Since(networkFailureStart) > time.Duration(e.cfg.NetworkTimeout)*time.Second {
						slog.Error("Network failure timeout exceeded, stopping job claims", 
							"timeout", e.cfg.NetworkTimeout,
						)
						return
					}
					
					slog.Error("Failed to claim job", "error", err)
					continue
				}
				
				// Reset network failure tracking on success
				networkFailureStart = time.Time{}
				
				if job != nil {
					e.wg.Add(1)
					go e.executeJob(job)
				} else {
					// No job available, release semaphore
					<-e.jobSem
				}
			default:
				// At capacity, skip this poll
				slog.Debug("At maximum job capacity, skipping poll")
			}
		}
	}
}

func (e *Executor) claimJob() (*models.Job, error) {
	// Get executor's IP address
	executorIP := e.getExecutorIP()
	
	job, err := e.client.ClaimNextJob(context.Background(), e.executorID, executorIP)
	if err != nil {
		return nil, err
	}
	
	if job != nil {
		slog.Info("Claimed job", 
			"job_id", job.ID,
			"type", job.Type,
			"priority", job.Priority,
		)
	}
	
	return job, nil
}

func (e *Executor) getExecutorIP() string {
	// For now, return a placeholder. In production, we'd get the actual IP
	// This could be enhanced to get the actual IP address
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

func (e *Executor) executeJob(job *models.Job) {
	defer e.wg.Done()
	defer func() { <-e.jobSem }()
	
	slog.Info("Starting job execution", "job_id", job.ID)
	
	// Store job in running jobs map
	jobIDStr := job.ID.String()
	e.runningJobs.Store(jobIDStr, job)
	defer e.runningJobs.Delete(jobIDStr)
	
	// Start heartbeat
	heartbeatCtx, cancelHeartbeat := context.WithCancel(e.ctx)
	defer cancelHeartbeat()
	go e.sendHeartbeats(heartbeatCtx, jobIDStr)
	
	// Create job working directory
	jobDir := filepath.Join(e.cfg.WorkDir, jobIDStr)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		slog.Error("Failed to create job directory", 
			"job_id", job.ID,
			"error", err,
		)
		e.failJob(jobIDStr, &models.JobResult{
			ExitCode: -1,
			Stderr:   fmt.Sprintf("Failed to create job directory: %v", err),
		})
		return
	}
	
	// Clean up job directory after completion (best effort)
	defer func() {
		if err := os.RemoveAll(jobDir); err != nil {
			slog.Warn("Failed to clean up job directory",
				"job_id", job.ID,
				"path", jobDir,
				"error", err,
			)
		}
	}()
	
	// Get binary from cache or download
	binaryPath, err := e.cache.GetBinary(job.BinaryURL, job.BinarySHA256)
	if err != nil {
		slog.Error("Failed to get binary",
			"job_id", job.ID,
			"error", err,
		)
		e.failJob(jobIDStr, &models.JobResult{
			ExitCode: -1,
			Stderr:   fmt.Sprintf("Failed to get binary: %v", err),
		})
		return
	}
	
	// Execute the job
	runner := &JobRunner{
		JobID:      jobIDStr,
		BinaryPath: binaryPath,
		Arguments:  job.Arguments,
		EnvVars:    job.EnvVariables,
		WorkDir:    jobDir,
	}
	
	result := runner.Execute(e.ctx)
	
	// Report result to server
	if result.ExitCode == 0 {
		completeReq := &models.CompleteRequest{
			ExecutorID: e.executorID,
			Stdout:     result.Stdout,
			Stderr:     result.Stderr,
			ExitCode:   result.ExitCode,
		}
		if err := e.client.CompleteJob(context.Background(), job.ID, completeReq); err != nil {
			slog.Error("Failed to report job completion",
				"job_id", job.ID,
				"error", err,
			)
		} else {
			slog.Info("Job completed successfully",
				"job_id", job.ID,
				"exit_code", result.ExitCode,
			)
		}
	} else {
		failReq := &models.FailRequest{
			ExecutorID:   e.executorID,
			ErrorMessage: "Job failed with non-zero exit code",
			Stdout:       result.Stdout,
			Stderr:       result.Stderr,
			ExitCode:     result.ExitCode,
		}
		if err := e.client.FailJob(context.Background(), job.ID, failReq); err != nil {
			slog.Error("Failed to report job failure",
				"job_id", job.ID,
				"error", err,
			)
		} else {
			slog.Info("Job failed",
				"job_id", job.ID,
				"exit_code", result.ExitCode,
			)
		}
	}
}

func (e *Executor) sendHeartbeats(ctx context.Context, jobID string) {
	ticker := time.NewTicker(time.Duration(e.cfg.HeartbeatInterval) * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			jobUUID, err := uuid.Parse(jobID)
			if err != nil {
				slog.Error("Invalid job ID", "job_id", jobID, "error", err)
				continue
			}
			if err := e.client.Heartbeat(context.Background(), jobUUID, e.executorID); err != nil {
				slog.Error("Failed to send heartbeat",
					"job_id", jobID,
					"error", err,
				)
			} else {
				slog.Debug("Heartbeat sent", "job_id", jobID)
			}
		}
	}
}

func (e *Executor) failJob(jobID string, result *models.JobResult) {
	jobUUID, err := uuid.Parse(jobID)
	if err != nil {
		slog.Error("Invalid job ID", "job_id", jobID, "error", err)
		return
	}
	
	failReq := &models.FailRequest{
		ExecutorID:   e.executorID,
		ErrorMessage: result.Stderr,
		Stdout:       result.Stdout,
		Stderr:       result.Stderr,
		ExitCode:     result.ExitCode,
	}
	
	if err := e.client.FailJob(context.Background(), jobUUID, failReq); err != nil {
		slog.Error("Failed to report job failure",
			"job_id", jobID,
			"error", err,
		)
	}
}