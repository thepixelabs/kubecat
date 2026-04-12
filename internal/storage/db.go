// Package storage provides persistent storage for Kubecat using SQLite.
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite" // Pure Go SQLite driver

	"github.com/thepixelabs/kubecat/internal/config"
)

// DB wraps the SQLite database connection.
type DB struct {
	mu   sync.RWMutex
	conn *sql.DB
	path string
}

// Open opens or creates a SQLite database at the default location.
func Open() (*DB, error) {
	stateDir := config.StateDir()
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	dbPath := filepath.Join(stateDir, "history.db")
	return OpenPath(dbPath)
}

// OpenPath opens or creates a SQLite database at the specified path.
func OpenPath(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrent access
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Set busy_timeout so SQLite waits instead of immediately returning
	// SQLITE_BUSY when another connection holds the writer slot. This lets
	// us safely drop the Go-level read mutex: WAL allows concurrent readers
	// with a single writer, and busy_timeout absorbs the brief writer-vs-writer
	// contention that used to be serialized by the Go RWMutex.
	if _, err := conn.Exec("PRAGMA busy_timeout=5000"); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to set busy_timeout: %w", err)
	}

	db := &DB{
		conn: conn,
		path: path,
	}

	// Run migrations
	if err := db.Migrate(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// Conn returns the underlying SQL connection.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// Migrate runs database migrations atomically inside a single transaction.
// Either all pending migrations apply or none do — avoids leaving the schema
// in a half-migrated state if a later migration fails.
func (db *DB) Migrate() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Create migrations table outside the transaction so the version lookup
	// below works on a fresh database.
	if _, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id INTEGER PRIMARY KEY,
			version INTEGER UNIQUE NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	var currentVersion int
	if err := db.conn.QueryRow("SELECT COALESCE(MAX(version), 0) FROM migrations").Scan(&currentVersion); err != nil {
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	// Determine whether there is anything to do; skip the transaction
	// entirely in the steady state so repeated opens are cheap.
	pending := false
	for _, m := range migrations {
		if m.Version > currentVersion {
			pending = true
			break
		}
	}
	if !pending {
		return nil
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin migration transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, m := range migrations {
		if m.Version <= currentVersion {
			continue
		}
		if _, err := tx.Exec(m.SQL); err != nil {
			return fmt.Errorf("failed to run migration %d: %w", m.Version, err)
		}
		if _, err := tx.Exec("INSERT INTO migrations (version) VALUES (?)", m.Version); err != nil {
			return fmt.Errorf("failed to record migration %d: %w", m.Version, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migrations: %w", err)
	}
	return nil
}

// Transaction executes a function within a transaction.
func (db *DB) Transaction(ctx context.Context, fn func(tx *sql.Tx) error) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Vacuum runs VACUUM to reclaim space.
func (db *DB) Vacuum() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.conn.Exec("VACUUM")
	return err
}

// Size returns the database file size in bytes.
func (db *DB) Size() (int64, error) {
	info, err := os.Stat(db.path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
