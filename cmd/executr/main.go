package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/draganm/executr/internal/executor"
	"github.com/draganm/executr/internal/models"
	"github.com/draganm/executr/internal/server"
	"github.com/draganm/executr/pkg/client"
	"github.com/google/uuid"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "executr",
		Usage: "Distributed job execution system",
		Commands: []*cli.Command{
			serverCommand(),
			executorCommand(),
			submitCommand(),
			statusCommand(),
			cancelCommand(),
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func serverCommand() *cli.Command {
	return &cli.Command{
		Name:  "server",
		Usage: "Run the executr server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "db-url",
				Usage:   "PostgreSQL connection string",
				EnvVars: []string{"EXECUTR_DB_URL"},
			},
			&cli.IntFlag{
				Name:    "port",
				Usage:   "Server listen port",
				Value:   8080,
				EnvVars: []string{"EXECUTR_PORT"},
			},
			&cli.DurationFlag{
				Name:    "cleanup-interval",
				Usage:   "Cleanup frequency (e.g. 30m, 1h)",
				Value:   time.Hour,
				EnvVars: []string{"EXECUTR_CLEANUP_INTERVAL"},
			},
			&cli.DurationFlag{
				Name:    "job-retention",
				Usage:   "Keep completed jobs duration (e.g. 24h, 48h)",
				Value:   48 * time.Hour,
				EnvVars: []string{"EXECUTR_JOB_RETENTION"},
			},
			&cli.DurationFlag{
				Name:    "heartbeat-timeout",
				Usage:   "Stale job timeout (e.g. 15s, 30s)",
				Value:   15 * time.Second,
				EnvVars: []string{"EXECUTR_HEARTBEAT_TIMEOUT"},
			},
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "Log level (debug/info/warn/error)",
				Value:   "info",
				EnvVars: []string{"EXECUTR_LOG_LEVEL"},
			},
		},
		Action: func(c *cli.Context) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
			defer cancel()

			cfg := &server.Config{
				DatabaseURL:       c.String("db-url"),
				Port:              c.Int("port"),
				CleanupInterval:   int(c.Duration("cleanup-interval").Seconds()),
				JobRetention:      int(c.Duration("job-retention").Seconds()),
				HeartbeatTimeout:  int(c.Duration("heartbeat-timeout").Seconds()),
				LogLevel:          c.String("log-level"),
			}

			// Setup logging
			var logLevel slog.Level
			switch cfg.LogLevel {
			case "debug":
				logLevel = slog.LevelDebug
			case "info":
				logLevel = slog.LevelInfo
			case "warn":
				logLevel = slog.LevelWarn
			case "error":
				logLevel = slog.LevelError
			default:
				logLevel = slog.LevelInfo
			}
			slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
				Level: logLevel,
			})))

			if cfg.DatabaseURL == "" {
				return fmt.Errorf("database URL is required")
			}

			srv, err := server.New(cfg)
			if err != nil {
				return fmt.Errorf("failed to create server: %w", err)
			}

			return srv.Run(ctx)
		},
	}
}

func executorCommand() *cli.Command {
	return &cli.Command{
		Name:  "executor",
		Usage: "Run an executr job executor",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "server-url",
				Usage:    "Server API endpoint",
				Required: true,
				EnvVars:  []string{"EXECUTR_SERVER_URL"},
			},
			&cli.StringFlag{
				Name:     "name",
				Usage:    "Executor name (used as prefix for executor ID)",
				Required: true,
				EnvVars:  []string{"EXECUTR_NAME"},
			},
			&cli.StringFlag{
				Name:    "cache-dir",
				Usage:   "Binary cache directory",
				Value:   "~/.executr/cache",
				EnvVars: []string{"EXECUTR_CACHE_DIR"},
			},
			&cli.StringFlag{
				Name:    "work-dir",
				Usage:   "Root directory for job working directories",
				Value:   "/tmp/executr-jobs",
				EnvVars: []string{"EXECUTR_WORK_DIR"},
			},
			&cli.IntFlag{
				Name:    "max-jobs",
				Usage:   "Maximum concurrent jobs",
				Value:   1,
				EnvVars: []string{"EXECUTR_MAX_JOBS"},
			},
			&cli.DurationFlag{
				Name:    "poll-interval",
				Usage:   "Job polling frequency (e.g. 5s, 10s)",
				Value:   5 * time.Second,
				EnvVars: []string{"EXECUTR_POLL_INTERVAL"},
			},
			&cli.IntFlag{
				Name:    "max-cache-size",
				Usage:   "Maximum cache size in MB",
				Value:   400,
				EnvVars: []string{"EXECUTR_MAX_CACHE_SIZE"},
			},
			&cli.DurationFlag{
				Name:    "heartbeat-interval",
				Usage:   "Heartbeat frequency (e.g. 5s, 10s)",
				Value:   5 * time.Second,
				EnvVars: []string{"EXECUTR_HEARTBEAT_INTERVAL"},
			},
			&cli.DurationFlag{
				Name:    "network-timeout",
				Usage:   "Stop claiming after network failure (e.g. 60s, 2m)",
				Value:   60 * time.Second,
				EnvVars: []string{"EXECUTR_NETWORK_TIMEOUT"},
			},
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "Log level (debug/info/warn/error)",
				Value:   "info",
				EnvVars: []string{"EXECUTR_LOG_LEVEL"},
			},
		},
		Action: func(c *cli.Context) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
			defer cancel()

			// Setup logging
			var logLevel slog.Level
			switch c.String("log-level") {
			case "debug":
				logLevel = slog.LevelDebug
			case "info":
				logLevel = slog.LevelInfo
			case "warn":
				logLevel = slog.LevelWarn
			case "error":
				logLevel = slog.LevelError
			default:
				logLevel = slog.LevelInfo
			}
			slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
				Level: logLevel,
			})))

			cfg := &executor.Config{
				ServerURL:         c.String("server-url"),
				Name:              c.String("name"),
				CacheDir:          c.String("cache-dir"),
				WorkDir:           c.String("work-dir"),
				MaxJobs:           c.Int("max-jobs"),
				PollInterval:      int(c.Duration("poll-interval").Seconds()),
				MaxCacheSize:      c.Int("max-cache-size"),
				HeartbeatInterval: int(c.Duration("heartbeat-interval").Seconds()),
				NetworkTimeout:    int(c.Duration("network-timeout").Seconds()),
			}

			exec, err := executor.New(cfg)
			if err != nil {
				return fmt.Errorf("failed to create executor: %w", err)
			}

			return exec.Run(ctx)
		},
	}
}

func submitCommand() *cli.Command {
	return &cli.Command{
		Name:  "submit",
		Usage: "Submit a job to executr",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "server-url",
				Usage:    "Server API endpoint",
				Required: true,
				EnvVars:  []string{"EXECUTR_SERVER_URL"},
			},
			&cli.StringFlag{
				Name:     "binary-url",
				Usage:    "Binary download URL",
				Required: true,
				EnvVars:  []string{"EXECUTR_BINARY_URL"},
			},
			&cli.StringFlag{
				Name:    "binary-sha256",
				Usage:   "Binary SHA256 (optional, auto-calculated if not provided)",
				EnvVars: []string{"EXECUTR_BINARY_SHA256"},
			},
			&cli.StringSliceFlag{
				Name:    "args",
				Usage:   "Arguments to pass to the binary (can be specified multiple times)",
				EnvVars: []string{"EXECUTR_ARGS"},
			},
			&cli.StringSliceFlag{
				Name:    "env",
				Usage:   "Environment variables KEY=VALUE (can be specified multiple times)",
				EnvVars: []string{"EXECUTR_ENV"},
			},
			&cli.StringFlag{
				Name:    "type",
				Usage:   "Job type (informational, cannot contain spaces)",
				Value:   "default",
				EnvVars: []string{"EXECUTR_TYPE"},
			},
			&cli.StringFlag{
				Name:    "priority",
				Usage:   "Priority (foreground/background/best_effort)",
				Value:   "background",
				EnvVars: []string{"EXECUTR_PRIORITY"},
			},
			&cli.StringFlag{
				Name:    "output",
				Usage:   "Output format (json/table)",
				Value:   "table",
				EnvVars: []string{"EXECUTR_OUTPUT"},
			},
		},
		Action: func(c *cli.Context) error {
			return submitJob(c)
		},
	}
}

func statusCommand() *cli.Command {
	return &cli.Command{
		Name:      "status",
		Usage:     "Get status of a job",
		ArgsUsage: "<job-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "server-url",
				Usage:    "Server API endpoint",
				Required: true,
				EnvVars:  []string{"EXECUTR_SERVER_URL"},
			},
			&cli.StringFlag{
				Name:    "output",
				Usage:   "Output format (json/table)",
				Value:   "table",
				EnvVars: []string{"EXECUTR_OUTPUT"},
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return fmt.Errorf("job ID is required")
			}
			return getJobStatus(c)
		},
	}
}

func cancelCommand() *cli.Command {
	return &cli.Command{
		Name:      "cancel",
		Usage:     "Cancel a pending job",
		ArgsUsage: "<job-id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "server-url",
				Usage:    "Server API endpoint",
				Required: true,
				EnvVars:  []string{"EXECUTR_SERVER_URL"},
			},
			&cli.StringFlag{
				Name:    "output",
				Usage:   "Output format (json/table)",
				Value:   "table",
				EnvVars: []string{"EXECUTR_OUTPUT"},
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return fmt.Errorf("job ID is required")
			}
			return cancelJob(c)
		},
	}
}

// submitJob handles the job submission logic
func submitJob(c *cli.Context) error {
	serverURL := c.String("server-url")
	binaryURL := c.String("binary-url")
	binarySHA256 := c.String("binary-sha256")
	jobType := c.String("type")
	priority := c.String("priority")
	outputFormat := c.String("output")

	// Validate job type (no spaces allowed)
	if strings.Contains(jobType, " ") {
		return fmt.Errorf("job type cannot contain spaces")
	}

	// Validate priority
	var jobPriority models.Priority
	switch priority {
	case "foreground":
		jobPriority = models.PriorityForeground
	case "background":
		jobPriority = models.PriorityBackground
	case "best_effort":
		jobPriority = models.PriorityBestEffort
	default:
		return fmt.Errorf("invalid priority: %s (must be foreground/background/best_effort)", priority)
	}

	// Parse environment variables
	envVars := make(map[string]string)
	for _, env := range c.StringSlice("env") {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid environment variable format: %s (expected KEY=VALUE)", env)
		}
		envVars[parts[0]] = parts[1]
	}

	// Calculate SHA256 if not provided
	if binarySHA256 == "" {
		calculatedSHA, err := calculateSHA256FromURL(binaryURL)
		if err != nil {
			return fmt.Errorf("failed to calculate SHA256: %w", err)
		}
		binarySHA256 = calculatedSHA
		if outputFormat != "json" {
			fmt.Fprintf(os.Stderr, "Calculated SHA256: %s\n", binarySHA256)
		}
	}

	// Create client
	cl := client.New(serverURL)

	// Submit job
	submission := &models.JobSubmission{
		Type:         jobType,
		BinaryURL:    binaryURL,
		BinarySHA256: binarySHA256,
		Arguments:    c.StringSlice("args"),
		EnvVariables: envVars,
		Priority:     jobPriority,
	}

	job, err := cl.SubmitJob(context.Background(), submission)
	if err != nil {
		return fmt.Errorf("failed to submit job: %w", err)
	}

	// Output result
	switch outputFormat {
	case "json":
		output := map[string]string{
			"job_id": job.ID.String(),
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(output)
	default:
		fmt.Printf("Job submitted successfully\n")
		fmt.Printf("Job ID: %s\n", job.ID.String())
		return nil
	}
}

// calculateSHA256FromURL streams the binary from the URL and calculates SHA256
func calculateSHA256FromURL(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download binary: HTTP %d", resp.StatusCode)
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, resp.Body); err != nil {
		return "", fmt.Errorf("failed to read binary: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// getJobStatus handles fetching and displaying job status
func getJobStatus(c *cli.Context) error {
	serverURL := c.String("server-url")
	outputFormat := c.String("output")
	jobIDStr := c.Args().First()

	// Parse job ID
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		return fmt.Errorf("invalid job ID: %w", err)
	}

	// Create client
	cl := client.New(serverURL)

	// Get job
	job, err := cl.GetJob(context.Background(), jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Output result
	switch outputFormat {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(job)
	default:
		return printJobTable(job)
	}
}

// printJobTable prints job details in a formatted table
func printJobTable(job *models.Job) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintf(w, "Job ID:\t%s\n", job.ID)
	fmt.Fprintf(w, "Type:\t%s\n", job.Type)
	fmt.Fprintf(w, "Status:\t%s\n", job.Status)
	fmt.Fprintf(w, "Priority:\t%s\n", job.Priority)
	fmt.Fprintf(w, "Binary URL:\t%s\n", job.BinaryURL)
	fmt.Fprintf(w, "Binary SHA256:\t%s\n", job.BinarySHA256)
	
	if len(job.Arguments) > 0 {
		fmt.Fprintf(w, "Arguments:\t%s\n", strings.Join(job.Arguments, " "))
	}
	
	if len(job.EnvVariables) > 0 {
		fmt.Fprintf(w, "Environment:\n")
		for k, v := range job.EnvVariables {
			fmt.Fprintf(w, "  %s:\t%s\n", k, v)
		}
	}
	
	if job.ExecutorID != "" {
		fmt.Fprintf(w, "Executor ID:\t%s\n", job.ExecutorID)
	}
	
	fmt.Fprintf(w, "Created At:\t%s\n", job.CreatedAt.Format("2006-01-02 15:04:05 MST"))
	
	if job.StartedAt != nil {
		fmt.Fprintf(w, "Started At:\t%s\n", job.StartedAt.Format("2006-01-02 15:04:05 MST"))
	}
	
	if job.CompletedAt != nil {
		fmt.Fprintf(w, "Completed At:\t%s\n", job.CompletedAt.Format("2006-01-02 15:04:05 MST"))
	}
	
	if job.LastHeartbeat != nil {
		fmt.Fprintf(w, "Last Heartbeat:\t%s\n", job.LastHeartbeat.Format("2006-01-02 15:04:05 MST"))
	}
	
	if job.ErrorMessage != "" {
		fmt.Fprintf(w, "Error:\t%s\n", job.ErrorMessage)
	}
	
	if job.ExitCode != nil {
		fmt.Fprintf(w, "Exit Code:\t%d\n", *job.ExitCode)
	}
	
	// Show output if job is completed or failed
	if job.Status == models.StatusCompleted || job.Status == models.StatusFailed {
		if job.Stdout != "" {
			fmt.Fprintf(w, "\n=== STDOUT ===\n")
			fmt.Fprintln(w, job.Stdout)
		}
		
		if job.Stderr != "" {
			fmt.Fprintf(w, "\n=== STDERR ===\n")
			fmt.Fprintln(w, job.Stderr)
		}
	}
	
	return nil
}

// cancelJob handles job cancellation
func cancelJob(c *cli.Context) error {
	serverURL := c.String("server-url")
	outputFormat := c.String("output")
	jobIDStr := c.Args().First()

	// Parse job ID
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		return fmt.Errorf("invalid job ID: %w", err)
	}

	// Create client
	cl := client.New(serverURL)

	// Cancel job
	err = cl.CancelJob(context.Background(), jobID)
	if err != nil {
		return fmt.Errorf("failed to cancel job: %w", err)
	}

	// Output result
	switch outputFormat {
	case "json":
		output := map[string]string{
			"status": "cancelled",
			"job_id": jobID.String(),
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(output)
	default:
		fmt.Printf("Job cancelled successfully\n")
		fmt.Printf("Job ID: %s\n", jobID.String())
		return nil
	}
}