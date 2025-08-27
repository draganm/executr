package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/draganm/executr/internal/server"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Commands: server, executor, submit, status, cancel\n")
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	switch os.Args[1] {
	case "server":
		if err := runServer(ctx, os.Args[2:]); err != nil {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	case "executor":
		fmt.Fprintf(os.Stderr, "Executor not yet implemented\n")
		os.Exit(1)
	case "submit":
		fmt.Fprintf(os.Stderr, "Submit not yet implemented\n")
		os.Exit(1)
	case "status":
		fmt.Fprintf(os.Stderr, "Status not yet implemented\n")
		os.Exit(1)
	case "cancel":
		fmt.Fprintf(os.Stderr, "Cancel not yet implemented\n")
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		fmt.Fprintf(os.Stderr, "Commands: server, executor, submit, status, cancel\n")
		os.Exit(1)
	}
}

func runServer(ctx context.Context, args []string) error {
	cfg := &server.Config{
		Port:              8080,
		CleanupInterval:   3600,  // 1 hour in seconds
		JobRetention:      172800, // 48 hours in seconds
		HeartbeatTimeout:  15,     // 15 seconds
		LogLevel:          "info",
	}

	fs := flag.NewFlagSet("server", flag.ExitOnError)
	fs.StringVar(&cfg.DatabaseURL, "db-url", os.Getenv("EXECUTR_DB_URL"), "PostgreSQL connection string")
	fs.IntVar(&cfg.Port, "port", cfg.Port, "Server listen port")
	fs.IntVar(&cfg.CleanupInterval, "cleanup-interval", cfg.CleanupInterval, "Cleanup frequency in seconds")
	fs.IntVar(&cfg.JobRetention, "job-retention", cfg.JobRetention, "Keep completed jobs duration in seconds")
	fs.IntVar(&cfg.HeartbeatTimeout, "heartbeat-timeout", cfg.HeartbeatTimeout, "Stale job timeout in seconds")
	fs.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level (debug/info/warn/error)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Override with env vars if set
	if port := os.Getenv("EXECUTR_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &cfg.Port)
	}
	if interval := os.Getenv("EXECUTR_CLEANUP_INTERVAL"); interval != "" {
		fmt.Sscanf(interval, "%d", &cfg.CleanupInterval)
	}
	if retention := os.Getenv("EXECUTR_JOB_RETENTION"); retention != "" {
		fmt.Sscanf(retention, "%d", &cfg.JobRetention)
	}
	if timeout := os.Getenv("EXECUTR_HEARTBEAT_TIMEOUT"); timeout != "" {
		fmt.Sscanf(timeout, "%d", &cfg.HeartbeatTimeout)
	}
	if level := os.Getenv("EXECUTR_LOG_LEVEL"); level != "" {
		cfg.LogLevel = level
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
}
