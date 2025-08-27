package server

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/draganm/executr/internal/db"
	"github.com/draganm/executr/internal/metrics"
	"github.com/draganm/executr/internal/models"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Config holds server configuration
type Config struct {
	DatabaseURL      string
	Port             int
	CleanupInterval  int // seconds
	JobRetention     int // seconds
	HeartbeatTimeout int // seconds
	LogLevel         string
}

// Server represents the job server
type Server struct {
	config  *Config
	pool    *pgxpool.Pool
	queries *db.Queries
	server  *http.Server
	wg      sync.WaitGroup
	port    int // actual port (for testing with port 0)
}

// New creates a new server instance
func New(cfg *Config) (*Server, error) {
	return &Server{
		config: cfg,
	}, nil
}

// Run starts the server
func (s *Server) Run(ctx context.Context) error {
	// Connect to database
	if err := s.connectDB(ctx); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer s.pool.Close()

	// Run migrations
	if err := s.runMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Start background workers
	s.startWorkers(ctx)

	// Setup HTTP server
	mux := http.NewServeMux()
	s.setupRoutes(mux)

	// Wrap with metrics middleware
	handler := metrics.HTTPMiddleware(mux)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.Port),
		Handler: handler,
	}

	// Start HTTP server
	serverErr := make(chan error, 1)
	
	// If port is 0, use a listener to get the actual port
	if s.config.Port == 0 {
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			return fmt.Errorf("failed to create listener: %w", err)
		}
		s.port = listener.Addr().(*net.TCPAddr).Port
		slog.Info("Starting server", "port", s.port)
		
		go func() {
			if err := s.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serverErr <- err
			}
		}()
	} else {
		s.port = s.config.Port
		go func() {
			slog.Info("Starting server", "port", s.port)
			if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serverErr <- err
			}
		}()
	}

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		slog.Info("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}
	case err := <-serverErr:
		return fmt.Errorf("server failed: %w", err)
	}

	// Wait for background workers to finish
	s.wg.Wait()
	return nil
}

func (s *Server) connectDB(ctx context.Context) error {
	config, err := pgxpool.ParseConfig(s.config.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Use default pool settings
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	s.pool = pool
	s.queries = db.New(pool)
	slog.Info("Connected to database")
	return nil
}

func (s *Server) runMigrations() error {
	// Create stdlib connection for migrations
	sqlDB := stdlib.OpenDBFromPool(s.pool)
	defer sqlDB.Close()

	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	slog.Info("Migrations completed successfully")
	return nil
}

func (s *Server) setupRoutes(mux *http.ServeMux) {
	// Metrics endpoint (no middleware for this)
	mux.Handle("/api/v1/metrics", promhttp.Handler())
	
	// Health check
	mux.HandleFunc("/api/v1/health", s.handleHealth)

	// Job endpoints
	mux.HandleFunc("/api/v1/jobs", s.handleJobs)
	mux.HandleFunc("/api/v1/jobs/", s.handleJobByID)
	mux.HandleFunc("/api/v1/jobs/claim", s.handleClaimJob)
	
	// Bulk operations
	mux.HandleFunc("/api/v1/jobs/bulk", s.handleBulkJobs)
	mux.HandleFunc("/api/v1/jobs/bulk/cancel", s.handleBulkCancel)
	
	// Admin endpoints
	mux.HandleFunc("/api/v1/admin/stats", s.handleAdminStats)
	mux.HandleFunc("/api/v1/admin/executors", s.handleAdminExecutors)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	status := "healthy"
	dbStatus := "connected"

	if err := s.pool.Ping(ctx); err != nil {
		dbStatus = "disconnected"
		status = "unhealthy"
	}

	response := map[string]string{
		"status":   status,
		"database": dbStatus,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleSubmitJob(w, r)
	case http.MethodGet:
		s.handleListJobs(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleJobByID(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from path
	path := r.URL.Path
	prefix := "/api/v1/jobs/"
	if len(path) <= len(prefix) {
		http.Error(w, "Job ID required", http.StatusBadRequest)
		return
	}

	idStr := path[len(prefix):]
	
	// Handle special endpoints
	if idStr == "claim" {
		s.handleClaimJob(w, r)
		return
	}

	// Parse job ID
	jobID, err := uuid.Parse(idStr[:36]) // UUID is 36 chars
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid job ID", map[string]interface{}{"id": idStr})
		return
	}

	// Handle sub-paths
	subPath := ""
	if len(idStr) > 36 {
		subPath = idStr[36:]
	}

	switch subPath {
	case "", "/":
		switch r.Method {
		case http.MethodGet:
			s.handleGetJob(w, r, jobID)
		case http.MethodDelete:
			s.handleCancelJob(w, r, jobID)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	case "/heartbeat":
		if r.Method == http.MethodPut {
			s.handleHeartbeat(w, r, jobID)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	case "/complete":
		if r.Method == http.MethodPut {
			s.handleCompleteJob(w, r, jobID)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	case "/fail":
		if r.Method == http.MethodPut {
			s.handleFailJob(w, r, jobID)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleSubmitJob(w http.ResponseWriter, r *http.Request) {
	var submission models.JobSubmission
	if err := json.NewDecoder(r.Body).Decode(&submission); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	// Validate required fields
	if submission.Type == "" || submission.BinaryURL == "" {
		s.writeError(w, http.StatusBadRequest, "type and binary_url are required", nil)
		return
	}

	// Create job in database
	envJSON, _ := json.Marshal(submission.EnvVariables)
	
	job, err := s.queries.CreateJob(r.Context(), db.CreateJobParams{
		Type:         submission.Type,
		BinaryUrl:    submission.BinaryURL,
		BinarySha256: submission.BinarySHA256,
		Arguments:    submission.Arguments,
		EnvVariables: envJSON,
		Priority:     string(submission.Priority),
	})
	if err != nil {
		slog.Error("Failed to create job", "error", err)
		s.writeError(w, http.StatusInternalServerError, "Failed to create job", nil)
		return
	}

	// Track metrics
	metrics.JobsSubmitted.WithLabelValues(submission.Type, string(submission.Priority)).Inc()
	
	// Convert to response model
	response := s.dbJobToModel(job)
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	
	status := q.Get("status")
	jobType := q.Get("type")
	priority := q.Get("priority")
	
	limit := int32(100)
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = int32(parsed)
		}
	}
	
	offset := int32(0)
	if o := q.Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = int32(parsed)
		}
	}

	jobs, err := s.queries.ListJobs(r.Context(), db.ListJobsParams{
		Column1: status,
		Column2: jobType,
		Column3: priority,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		slog.Error("Failed to list jobs", "error", err)
		s.writeError(w, http.StatusInternalServerError, "Failed to list jobs", nil)
		return
	}

	// Convert to response models
	response := make([]models.Job, len(jobs))
	for i, job := range jobs {
		response[i] = s.dbJobToModel(job)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request, jobID uuid.UUID) {
	job, err := s.queries.GetJob(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.writeError(w, http.StatusNotFound, "Job not found", map[string]interface{}{"job_id": jobID})
		} else {
			slog.Error("Failed to get job", "error", err, "job_id", jobID)
			s.writeError(w, http.StatusInternalServerError, "Failed to get job", nil)
		}
		return
	}

	// Get job attempts
	attempts, err := s.queries.GetJobAttempts(r.Context(), jobID)
	if err != nil {
		slog.Error("Failed to get job attempts", "error", err, "job_id", jobID)
	}

	response := s.dbJobToModel(job)
	
	// Add attempts to response
	if len(attempts) > 0 {
		attemptModels := make([]models.JobAttempt, len(attempts))
		for i, attempt := range attempts {
			attemptModels[i] = models.JobAttempt{
				ID:           attempt.ID,
				JobID:        attempt.JobID,
				ExecutorID:   attempt.ExecutorID,
				ExecutorIP:   attempt.ExecutorIp,
				StartedAt:    attempt.StartedAt.Time,
				Status:       attempt.Status,
			}
			if attempt.EndedAt.Valid {
				attemptModels[i].EndedAt = &attempt.EndedAt.Time
			}
			if attempt.ErrorMessage.Valid {
				attemptModels[i].ErrorMessage = attempt.ErrorMessage.String
			}
		}
		// We'll need to extend the response to include attempts
		// For now, return the job without attempts
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request, jobID uuid.UUID) {
	_, err := s.queries.CancelJob(r.Context(), jobID)
	if err != nil {
		slog.Error("Failed to cancel job", "error", err, "job_id", jobID)
		s.writeError(w, http.StatusInternalServerError, "Failed to cancel job", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleClaimJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var claim models.ClaimRequest
	if err := json.NewDecoder(r.Body).Decode(&claim); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if claim.ExecutorID == "" || claim.ExecutorIP == "" {
		s.writeError(w, http.StatusBadRequest, "executor_id and executor_ip are required", nil)
		return
	}

	executorID := pgtype.Text{String: claim.ExecutorID, Valid: true}
	job, err := s.queries.ClaimNextJob(r.Context(), executorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		slog.Error("Failed to claim job", "error", err)
		s.writeError(w, http.StatusInternalServerError, "Failed to claim job", nil)
		return
	}

	// Record job attempt
	_, err = s.queries.RecordJobAttempt(r.Context(), db.RecordJobAttemptParams{
		JobID:      job.ID,
		ExecutorID: claim.ExecutorID,
		ExecutorIp: claim.ExecutorIP,
	})
	if err != nil {
		slog.Error("Failed to record job attempt", "error", err, "job_id", job.ID)
		// Don't fail the claim, just log the error
	}

	response := s.dbJobToModel(job)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request, jobID uuid.UUID) {
	var req models.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if req.ExecutorID == "" {
		s.writeError(w, http.StatusBadRequest, "executor_id is required", nil)
		return
	}

	executorID := pgtype.Text{String: req.ExecutorID, Valid: true}
	err := s.queries.UpdateHeartbeat(r.Context(), db.UpdateHeartbeatParams{
		ID:         jobID,
		ExecutorID: executorID,
	})
	if err != nil {
		slog.Error("Failed to update heartbeat", "error", err, "job_id", jobID)
		s.writeError(w, http.StatusInternalServerError, "Failed to update heartbeat", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCompleteJob(w http.ResponseWriter, r *http.Request, jobID uuid.UUID) {
	var req models.CompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if req.ExecutorID == "" {
		s.writeError(w, http.StatusBadRequest, "executor_id is required", nil)
		return
	}

	_, err := s.queries.CompleteJob(r.Context(), db.CompleteJobParams{
		ID:         jobID,
		Stdout:     pgtype.Text{String: req.Stdout, Valid: true},
		Stderr:     pgtype.Text{String: req.Stderr, Valid: true},
		ExitCode:   pgtype.Int4{Int32: int32(req.ExitCode), Valid: true},
	})
	if err != nil {
		slog.Error("Failed to complete job", "error", err, "job_id", jobID)
		s.writeError(w, http.StatusInternalServerError, "Failed to complete job", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleFailJob(w http.ResponseWriter, r *http.Request, jobID uuid.UUID) {
	var req models.FailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	if req.ExecutorID == "" || req.ErrorMessage == "" {
		s.writeError(w, http.StatusBadRequest, "executor_id and error_message are required", nil)
		return
	}

	var stdout, stderr pgtype.Text
	var exitCode pgtype.Int4
	if req.Stdout != "" {
		stdout = pgtype.Text{String: req.Stdout, Valid: true}
	}
	if req.Stderr != "" {
		stderr = pgtype.Text{String: req.Stderr, Valid: true}
	}
	if req.ExitCode != 0 {
		exitCode = pgtype.Int4{Int32: int32(req.ExitCode), Valid: true}
	}
	_, err := s.queries.FailJob(r.Context(), db.FailJobParams{
		ID:           jobID,
		ErrorMessage: pgtype.Text{String: req.ErrorMessage, Valid: true},
		Stdout:       stdout,
		Stderr:       stderr,
		ExitCode:     exitCode,
	})
	if err != nil {
		slog.Error("Failed to fail job", "error", err, "job_id", jobID)
		s.writeError(w, http.StatusInternalServerError, "Failed to mark job as failed", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) startWorkers(ctx context.Context) {
	// Heartbeat monitor
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.heartbeatMonitor(ctx)
	}()

	// Job cleaner
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.jobCleaner(ctx)
	}()
	
	// Job retry worker
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.jobRetryWorker(ctx)
	}()
}

func (s *Server) heartbeatMonitor(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkStaleJobs(ctx)
		}
	}
}

func (s *Server) checkStaleJobs(ctx context.Context) {
	jobs, err := s.queries.FindStaleJobs(ctx)
	if err != nil {
		slog.Error("Failed to find stale jobs", "error", err)
		return
	}

	for _, job := range jobs {
		slog.Info("Resetting stale job", "job_id", job.ID)
		if err := s.queries.ResetStaleJob(ctx, job.ID); err != nil {
			slog.Error("Failed to reset stale job", "error", err, "job_id", job.ID)
		}
	}
}

func (s *Server) jobCleaner(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.config.CleanupInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanupOldJobs(ctx)
		}
	}
}

func (s *Server) cleanupOldJobs(ctx context.Context) {
	interval := pgtype.Interval{}
	// Convert hours to microseconds (1 hour = 3600 seconds = 3600000000 microseconds)
	interval.Microseconds = int64(s.config.JobRetention) * 3600000000
	interval.Valid = true
	err := s.queries.CleanupOldJobs(ctx, interval)
	if err != nil {
		slog.Error("Failed to cleanup old jobs", "error", err)
	} else {
		slog.Debug("Cleaned up old jobs")
		metrics.OldJobsCleaned.Inc()
	}
}

func (s *Server) jobRetryWorker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.retryFailedJobs(ctx)
		}
	}
}

func (s *Server) retryFailedJobs(ctx context.Context) {
	jobs, err := s.queries.GetRetriableJobs(ctx)
	if err != nil {
		slog.Error("Failed to get retriable jobs", "error", err)
		return
	}

	for _, job := range jobs {
		if err := s.queries.IncrementJobRetry(ctx, job.ID); err != nil {
			slog.Error("Failed to retry job", "job_id", job.ID, "error", err)
			continue
		}
		
		slog.Info("Retrying failed job", 
			"job_id", job.ID, 
			"type", job.Type,
			"retry_count", job.RetryCount+1,
			"max_retries", job.MaxRetries)
	}
}

func (s *Server) dbJobToModel(job db.Job) models.Job {
	var envVars map[string]string
	if job.EnvVariables != nil {
		json.Unmarshal(job.EnvVariables, &envVars)
	}

	model := models.Job{
		ID:            job.ID,
		Type:          job.Type,
		BinaryURL:     job.BinaryUrl,
		BinarySHA256:  job.BinarySha256,
		Arguments:     job.Arguments,
		EnvVariables:  envVars,
		Priority:      models.Priority(job.Priority),
		Status:        models.Status(job.Status),
		CreatedAt:     job.CreatedAt.Time,
	}

	if job.ExecutorID.Valid {
		model.ExecutorID = job.ExecutorID.String
	}
	if job.Stdout.Valid {
		model.Stdout = job.Stdout.String
	}
	if job.Stderr.Valid {
		model.Stderr = job.Stderr.String
	}
	if job.ExitCode.Valid {
		exitCode := int(job.ExitCode.Int32)
		model.ExitCode = &exitCode
	}
	if job.ErrorMessage.Valid {
		model.ErrorMessage = job.ErrorMessage.String
	}
	if job.StartedAt.Valid {
		model.StartedAt = &job.StartedAt.Time
	}
	if job.CompletedAt.Valid {
		model.CompletedAt = &job.CompletedAt.Time
	}
	if job.LastHeartbeat.Valid {
		model.LastHeartbeat = &job.LastHeartbeat.Time
	}

	return model
}

func (s *Server) writeError(w http.ResponseWriter, code int, message string, context map[string]interface{}) {
	response := map[string]interface{}{
		"error": message,
	}
	if context != nil {
		response["context"] = context
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(response)
}

// Port returns the actual port the server is listening on
func (s *Server) Port() int {
	return s.port
}

// Admin endpoints

func (s *Server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	
	// Get job counts by status and priority
	stats := make(map[string]interface{})
	
	// Count jobs by status
	statusCounts, err := s.queries.CountJobsByStatus(ctx)
	if err != nil {
		slog.Error("Failed to get job status counts", "error", err)
		s.writeError(w, http.StatusInternalServerError, "Failed to get statistics", nil)
		return
	}
	
	// Count pending jobs by priority  
	priorityCounts, err := s.queries.CountPendingJobsByPriority(ctx)
	if err != nil {
		slog.Error("Failed to get priority counts", "error", err)
		s.writeError(w, http.StatusInternalServerError, "Failed to get statistics", nil)
		return
	}
	
	// Get active executors count
	executors, err := s.queries.GetActiveExecutors(ctx)
	if err != nil {
		slog.Error("Failed to get active executors", "error", err)
		executors = []db.GetActiveExecutorsRow{}
	}
	
	// Build response
	stats["jobs_by_status"] = statusCounts
	stats["pending_by_priority"] = priorityCounts
	stats["active_executors"] = len(executors)
	stats["timestamp"] = time.Now().UTC()
	
	// Update Prometheus metrics
	pendingMap := make(map[string]int)
	for _, p := range priorityCounts {
		pendingMap[p.Priority] = int(p.Count)
	}
	
	statusMaps := map[string]map[string]int{
		"pending": pendingMap,
		"running": make(map[string]int),
		"completed": make(map[string]int),
		"failed": make(map[string]int),
		"cancelled": make(map[string]int),
	}
	
	for _, s := range statusCounts {
		if s.Status == "running" {
			statusMaps["running"]["all"] = int(s.Count)
		} else if s.Status == "completed" {
			statusMaps["completed"]["all"] = int(s.Count)
		} else if s.Status == "failed" {
			statusMaps["failed"]["all"] = int(s.Count)
		} else if s.Status == "cancelled" {
			statusMaps["cancelled"]["all"] = int(s.Count)
		}
	}
	
	metrics.UpdateQueueMetrics(
		statusMaps["pending"],
		statusMaps["running"],
		statusMaps["completed"],
		statusMaps["failed"],
		statusMaps["cancelled"],
	)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleAdminExecutors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	
	// Get active executors (those with recent heartbeats)
	executors, err := s.queries.GetActiveExecutors(ctx)
	if err != nil {
		slog.Error("Failed to get active executors", "error", err)
		s.writeError(w, http.StatusInternalServerError, "Failed to get executors", nil)
		return
	}
	
	// Format response
	type executorInfo struct {
		ExecutorID    string    `json:"executor_id"`
		CurrentJobID  *string   `json:"current_job_id,omitempty"`
		JobType       *string   `json:"job_type,omitempty"`
		LastHeartbeat time.Time `json:"last_heartbeat"`
		JobsCompleted int64     `json:"jobs_completed"`
	}
	
	var response []executorInfo
	for _, e := range executors {
		info := executorInfo{
			ExecutorID:    e.ExecutorID.String,
			LastHeartbeat: e.LastHeartbeat.Time,
			JobsCompleted: e.JobsCompleted,
		}
		
		// JobID is a UUID, not a nullable field
		jobID := e.JobID.String()
		info.CurrentJobID = &jobID
		
		// JobType is a string, not a nullable field  
		info.JobType = &e.JobType
		
		response = append(response, info)
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Bulk operations

func (s *Server) handleBulkJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse bulk submission request
	var submissions []models.JobSubmission
	if err := json.NewDecoder(r.Body).Decode(&submissions); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	// Validate submissions
	if len(submissions) == 0 {
		s.writeError(w, http.StatusBadRequest, "No jobs provided", nil)
		return
	}

	if len(submissions) > 100 {
		s.writeError(w, http.StatusBadRequest, "Too many jobs (max 100)", nil)
		return
	}

	// Submit jobs
	type jobResult struct {
		Index   int        `json:"index"`
		Success bool       `json:"success"`
		JobID   *uuid.UUID `json:"job_id,omitempty"`
		Error   string     `json:"error,omitempty"`
	}

	results := make([]jobResult, len(submissions))
	successCount := 0

	for i, submission := range submissions {
		// Validate required fields
		if submission.Type == "" || submission.BinaryURL == "" {
			results[i] = jobResult{
				Index:   i,
				Success: false,
				Error:   "type and binary_url are required",
			}
			continue
		}

		// Create job
		envJSON, _ := json.Marshal(submission.EnvVariables)
		
		job, err := s.queries.CreateJobWithRetries(r.Context(), db.CreateJobWithRetriesParams{
			Type:         submission.Type,
			BinaryUrl:    submission.BinaryURL,
			BinarySha256: submission.BinarySHA256,
			Arguments:    submission.Arguments,
			EnvVariables: envJSON,
			Priority:     string(submission.Priority),
			Status:       "pending",
			MaxRetries:   int32(submission.MaxRetries),
		})

		if err != nil {
			results[i] = jobResult{
				Index:   i,
				Success: false,
				Error:   err.Error(),
			}
		} else {
			results[i] = jobResult{
				Index:   i,
				Success: true,
				JobID:   &job.ID,
			}
			successCount++
			
			// Track metrics
			metrics.JobsSubmitted.WithLabelValues(submission.Type, string(submission.Priority)).Inc()
		}
	}

	// Return results
	response := map[string]interface{}{
		"total":      len(submissions),
		"successful": successCount,
		"failed":     len(submissions) - successCount,
		"results":    results,
	}

	w.Header().Set("Content-Type", "application/json")
	if successCount == 0 {
		w.WriteHeader(http.StatusBadRequest)
	} else if successCount < len(submissions) {
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleBulkCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var request struct {
		JobIDs []string `json:"job_ids"`
		Type   string   `json:"type,omitempty"`
		Status string   `json:"status,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	// Cancel jobs
	cancelledCount := 0
	failedCount := 0

	if len(request.JobIDs) > 0 {
		// Cancel specific jobs
		for _, jobIDStr := range request.JobIDs {
			jobID, err := uuid.Parse(jobIDStr)
			if err != nil {
				failedCount++
				continue
			}

			_, err = s.queries.CancelJob(r.Context(), jobID)
			if err != nil {
				failedCount++
			} else {
				cancelledCount++
				metrics.JobsCancelled.Inc()
			}
		}
	} else if request.Type != "" || request.Status != "" {
		// Cancel by criteria
		// This would need a new query to cancel jobs by type/status
		s.writeError(w, http.StatusNotImplemented, "Cancellation by criteria not yet implemented", nil)
		return
	} else {
		s.writeError(w, http.StatusBadRequest, "Must provide job_ids or criteria", nil)
		return
	}

	// Return results
	response := map[string]interface{}{
		"cancelled": cancelledCount,
		"failed":    failedCount,
		"total":     cancelledCount + failedCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}