package db

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Config holds database configuration
type Config struct {
	DatabaseURL string
}

// Connection represents a database connection with queries
type Connection struct {
	*Queries
	pool *pgxpool.Pool
}

// NewConnection creates a new database connection with pgx default pool settings
func NewConnection(ctx context.Context, cfg Config) (*Connection, error) {
	// Parse config for pgxpool
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Use default pool settings (can be customized here if needed)
	// poolConfig.MaxConns = 10
	// poolConfig.MinConns = 2
	// poolConfig.MaxConnLifetime = time.Hour
	// poolConfig.MaxConnIdleTime = time.Minute * 30

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Connection{
		Queries: New(pool),
		pool:    pool,
	}, nil
}

// RunMigrations runs all pending database migrations
func (c *Connection) RunMigrations(ctx context.Context) error {
	// Get a standard database/sql connection for migrations
	db := stdlib.OpenDBFromPool(c.pool)
	
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// Close closes the database connection pool
func (c *Connection) Close() {
	c.pool.Close()
}

// Pool returns the underlying connection pool
func (c *Connection) Pool() *pgxpool.Pool {
	return c.pool
}

// Health checks if the database is healthy
func (c *Connection) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	return c.pool.Ping(ctx)
}

// Stats returns pool statistics
func (c *Connection) Stats() *pgxpool.Stat {
	return c.pool.Stat()
}

// WaitForConnection waits for the database to be available with exponential backoff
func WaitForConnection(ctx context.Context, cfg Config, maxRetries int) (*Connection, error) {
	var conn *Connection
	var err error
	
	backoff := time.Second
	for i := 0; i < maxRetries; i++ {
		conn, err = NewConnection(ctx, cfg)
		if err == nil {
			return conn, nil
		}
		
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
	
	return nil, fmt.Errorf("failed to connect after %d attempts: %w", maxRetries, err)
}