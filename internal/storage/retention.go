// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"
)

const (
	retentionCleanupInterval = time.Hour
	dbSizeWarnThreshold      = 500 * 1024 * 1024 // 500 MiB
	vacuumThreshold          = 1000              // rows deleted before VACUUM
)

// RetentionConfig controls how long different record types are kept.
type RetentionConfig struct {
	// EventsRetention is how long event rows are kept (default 30 days).
	EventsRetention time.Duration
	// SnapshotsRetention is how long snapshot rows are kept (default 7 days).
	SnapshotsRetention time.Duration
	// CorrelationsRetention is how long correlation rows are kept (default 30 days).
	CorrelationsRetention time.Duration
}

// DefaultRetentionConfig returns sensible defaults.
func DefaultRetentionConfig() RetentionConfig {
	return RetentionConfig{
		EventsRetention:       30 * 24 * time.Hour,
		SnapshotsRetention:    7 * 24 * time.Hour,
		CorrelationsRetention: 30 * 24 * time.Hour,
	}
}

// RetentionManager runs periodic cleanup jobs against the storage DB.
type RetentionManager struct {
	db     *DB
	cfg    RetentionConfig
	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
}

// NewRetentionManager creates a RetentionManager. Call Start to begin cleanup.
func NewRetentionManager(db *DB, cfg RetentionConfig) *RetentionManager {
	return &RetentionManager{
		db:   db,
		cfg:  cfg,
		done: make(chan struct{}),
	}
}

// Start runs an immediate cleanup pass then schedules hourly cleanup.
func (r *RetentionManager) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	go func() {
		defer close(r.done)

		// Startup cleanup.
		r.cleanup(ctx)

		ticker := time.NewTicker(retentionCleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.cleanup(ctx)
			}
		}
	}()
}

// Stop shuts down the cleanup goroutine.
func (r *RetentionManager) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	<-r.done
}

// cleanup deletes rows older than the configured retention windows.
func (r *RetentionManager) cleanup(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	totalDeleted := 0

	totalDeleted += r.deleteOldRows(ctx, "events", "last_seen", r.cfg.EventsRetention)
	totalDeleted += r.deleteOldRows(ctx, "snapshots", "timestamp", r.cfg.SnapshotsRetention)
	totalDeleted += r.deleteOldRows(ctx, "correlations", "created_at", r.cfg.CorrelationsRetention)

	if totalDeleted > 0 {
		slog.Info("retention: deleted old rows", slog.Int("total", totalDeleted))
	}

	if totalDeleted >= vacuumThreshold {
		r.vacuum(ctx)
	}

	r.checkDBSize()
}

// deleteOldRows removes rows from table where timeColumn is older than retention.
// Returns the number of deleted rows.
func (r *RetentionManager) deleteOldRows(ctx context.Context, table, timeColumn string, retention time.Duration) int {
	if retention <= 0 {
		return 0
	}

	cutoff := time.Now().UTC().Add(-retention)

	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	// Use parameterized query — no string interpolation of user data.
	query := "DELETE FROM " + table + " WHERE " + timeColumn + " < ?" //nolint:gosec
	res, err := r.db.conn.ExecContext(ctx, query, cutoff)
	if err != nil {
		if err != sql.ErrNoRows {
			slog.Warn("retention: delete failed",
				slog.String("table", table),
				slog.Any("error", err))
		}
		return 0
	}

	n, _ := res.RowsAffected()
	if n > 0 {
		slog.Debug("retention: cleaned table",
			slog.String("table", table),
			slog.Int64("rows_deleted", n))
	}
	return int(n)
}

// vacuum runs VACUUM to reclaim disk space after large deletions.
func (r *RetentionManager) vacuum(ctx context.Context) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	_, err := r.db.conn.ExecContext(ctx, "VACUUM")
	if err != nil {
		slog.Warn("retention: VACUUM failed", slog.Any("error", err))
		return
	}
	slog.Info("retention: VACUUM completed")
}

// checkDBSize logs a warning when the database file exceeds the threshold.
func (r *RetentionManager) checkDBSize() {
	var pageCount, pageSize int64
	_ = r.db.conn.QueryRow("PRAGMA page_count").Scan(&pageCount)
	_ = r.db.conn.QueryRow("PRAGMA page_size").Scan(&pageSize)

	size := pageCount * pageSize
	if size > dbSizeWarnThreshold {
		slog.Warn("retention: database size exceeds warning threshold",
			slog.Int64("size_bytes", size),
			slog.Int64("threshold_bytes", dbSizeWarnThreshold))
	}
}
