package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"

	"github.com/draganm/executr/internal/executor"
	"github.com/draganm/executr/internal/server"
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
			&cli.IntFlag{
				Name:    "cleanup-interval",
				Usage:   "Cleanup frequency in seconds",
				Value:   3600,
				EnvVars: []string{"EXECUTR_CLEANUP_INTERVAL"},
			},
			&cli.IntFlag{
				Name:    "job-retention",
				Usage:   "Keep completed jobs duration in seconds",
				Value:   172800,
				EnvVars: []string{"EXECUTR_JOB_RETENTION"},
			},
			&cli.IntFlag{
				Name:    "heartbeat-timeout",
				Usage:   "Stale job timeout in seconds",
				Value:   15,
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
				CleanupInterval:   c.Int("cleanup-interval"),
				JobRetention:      c.Int("job-retention"),
				HeartbeatTimeout:  c.Int("heartbeat-timeout"),
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
			&cli.IntFlag{
				Name:    "poll-interval",
				Usage:   "Job polling frequency in seconds",
				Value:   5,
				EnvVars: []string{"EXECUTR_POLL_INTERVAL"},
			},
			&cli.IntFlag{
				Name:    "max-cache-size",
				Usage:   "Maximum cache size in MB",
				Value:   400,
				EnvVars: []string{"EXECUTR_MAX_CACHE_SIZE"},
			},
			&cli.IntFlag{
				Name:    "heartbeat-interval",
				Usage:   "Heartbeat frequency in seconds",
				Value:   5,
				EnvVars: []string{"EXECUTR_HEARTBEAT_INTERVAL"},
			},
			&cli.IntFlag{
				Name:    "network-timeout",
				Usage:   "Stop claiming after network failure in seconds",
				Value:   60,
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
				PollInterval:      c.Int("poll-interval"),
				MaxCacheSize:      c.Int("max-cache-size"),
				HeartbeatInterval: c.Int("heartbeat-interval"),
				NetworkTimeout:    c.Int("network-timeout"),
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
		Action: func(c *cli.Context) error {
			return fmt.Errorf("submit not yet implemented")
		},
	}
}

func statusCommand() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "Get status of a job",
		Action: func(c *cli.Context) error {
			return fmt.Errorf("status not yet implemented")
		},
	}
}

func cancelCommand() *cli.Command {
	return &cli.Command{
		Name:  "cancel",
		Usage: "Cancel a pending job",
		Action: func(c *cli.Context) error {
			return fmt.Errorf("cancel not yet implemented")
		},
	}
}