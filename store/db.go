// Package store provides database access for mvChat2.
package store

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/scalecode-solutions/mvchat2/config"
)

//go:embed schema.sql
var schemaFS embed.FS

// DB holds the database connection pool.
type DB struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

// New creates a new database connection pool.
func New(cfg *config.DatabaseConfig) (*DB, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.MaxIdleConns)
	poolConfig.MaxConnLifetime = time.Duration(cfg.ConnMaxLifetime) * time.Second

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{
		pool:    pool,
		timeout: time.Duration(cfg.SQLTimeout) * time.Second,
	}, nil
}

// Close closes the database connection pool.
func (db *DB) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
}

// Pool returns the underlying connection pool.
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// Context returns a context with the configured timeout.
func (db *DB) Context() (context.Context, context.CancelFunc) {
	if db.timeout > 0 {
		return context.WithTimeout(context.Background(), db.timeout)
	}
	return context.Background(), func() {}
}

// InitSchema creates the database schema if it doesn't exist.
func (db *DB) InitSchema() error {
	ctx, cancel := db.Context()
	defer cancel()

	// Check if schema already exists
	var exists bool
	err := db.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'schema_version'
		)
	`).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check schema: %w", err)
	}

	if exists {
		return nil // Schema already initialized
	}

	// Read and execute schema
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	_, err = db.pool.Exec(ctx, string(schema))
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	return nil
}

// GetSchemaVersion returns the current schema version.
func (db *DB) GetSchemaVersion() (int, error) {
	ctx, cancel := db.Context()
	defer cancel()

	var version int
	err := db.pool.QueryRow(ctx, `SELECT version FROM schema_version ORDER BY version DESC LIMIT 1`).Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}
