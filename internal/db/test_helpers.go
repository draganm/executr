package db

import (
	"context"
	"testing"
	"time"
)

// TestConfig returns a test database configuration
func TestConfig() Config {
	return Config{
		DatabaseURL: "postgres://postgres:password@localhost:5432/executr_test?sslmode=disable",
	}
}

// SetupTestDB creates a test database connection and runs migrations
func SetupTestDB(t *testing.T) (*Connection, func()) {
	ctx := context.Background()
	cfg := TestConfig()
	
	// Create connection with retries
	conn, err := WaitForConnection(ctx, cfg, 5)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	
	// Run migrations
	if err := conn.RunMigrations(ctx); err != nil {
		conn.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}
	
	// Return cleanup function
	cleanup := func() {
		// Clean up test data
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		// Delete all jobs and attempts (cascade will handle attempts)
		_, _ = conn.pool.Exec(ctx, "DELETE FROM jobs")
		
		conn.Close()
	}
	
	return conn, cleanup
}