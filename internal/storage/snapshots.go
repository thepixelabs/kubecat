package storage

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"time"
)

// Snapshot represents a cluster state snapshot.
type Snapshot struct {
	ID        int64
	Cluster   string
	Timestamp time.Time
	Data      []byte
}

// SnapshotData represents the actual snapshot content.
type SnapshotData struct {
	Cluster    string                    `json:"cluster"`
	Timestamp  time.Time                 `json:"timestamp"`
	Namespaces []string                  `json:"namespaces"`
	Resources  map[string][]ResourceInfo `json:"resources"` // kind -> resources
}

// ResourceInfo contains minimal resource information for snapshots.
type ResourceInfo struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	ResourceVersion string            `json:"resourceVersion"`
	Labels          map[string]string `json:"labels,omitempty"`
	Status          string            `json:"status,omitempty"`
}

// SnapshotRepository handles snapshot storage operations.
type SnapshotRepository struct {
	db *DB
}

// NewSnapshotRepository creates a new snapshot repository.
func NewSnapshotRepository(db *DB) *SnapshotRepository {
	return &SnapshotRepository{db: db}
}

// Save saves a snapshot.
func (r *SnapshotRepository) Save(ctx context.Context, cluster string, data *SnapshotData) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	// Serialize and compress
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	compressed, err := compress(jsonData)
	if err != nil {
		return err
	}

	_, err = r.db.conn.ExecContext(ctx, `
		INSERT INTO snapshots (cluster, timestamp, data, compressed)
		VALUES (?, ?, ?, 1)
	`, cluster, data.Timestamp, compressed)

	return err
}

// Get retrieves a snapshot by ID.
// Read path: no Go-level lock, see EventRepository.List for rationale.
func (r *SnapshotRepository) Get(ctx context.Context, id int64) (*SnapshotData, error) {
	var compressed int
	var data []byte
	err := r.db.conn.QueryRowContext(ctx,
		"SELECT data, compressed FROM snapshots WHERE id = ?", id,
	).Scan(&data, &compressed)
	if err != nil {
		return nil, err
	}

	if compressed == 1 {
		data, err = decompress(data)
		if err != nil {
			return nil, err
		}
	}

	var snapshot SnapshotData
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}

	return &snapshot, nil
}

// GetLatest retrieves the latest snapshot for a cluster.
func (r *SnapshotRepository) GetLatest(ctx context.Context, cluster string) (*SnapshotData, error) {
	var compressed int
	var data []byte
	err := r.db.conn.QueryRowContext(ctx, `
		SELECT data, compressed FROM snapshots 
		WHERE cluster = ? 
		ORDER BY timestamp DESC LIMIT 1
	`, cluster).Scan(&data, &compressed)
	if err != nil {
		return nil, err
	}

	if compressed == 1 {
		data, err = decompress(data)
		if err != nil {
			return nil, err
		}
	}

	var snapshot SnapshotData
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}

	return &snapshot, nil
}

// GetAt retrieves the snapshot closest to the specified time.
func (r *SnapshotRepository) GetAt(ctx context.Context, cluster string, at time.Time) (*SnapshotData, error) {
	var compressed int
	var data []byte
	err := r.db.conn.QueryRowContext(ctx, `
		SELECT data, compressed FROM snapshots 
		WHERE cluster = ? AND timestamp <= ?
		ORDER BY timestamp DESC LIMIT 1
	`, cluster, at).Scan(&data, &compressed)
	if err != nil {
		return nil, err
	}

	if compressed == 1 {
		data, err = decompress(data)
		if err != nil {
			return nil, err
		}
	}

	var snapshot SnapshotData
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}

	return &snapshot, nil
}

// ListTimestamps lists snapshot timestamps for a cluster.
func (r *SnapshotRepository) ListTimestamps(ctx context.Context, cluster string, limit int) ([]time.Time, error) {
	query := "SELECT timestamp FROM snapshots WHERE cluster = ? ORDER BY timestamp DESC"
	args := []interface{}{cluster}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := r.db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var timestamps []time.Time
	for rows.Next() {
		var ts time.Time
		if err := rows.Scan(&ts); err != nil {
			return nil, err
		}
		timestamps = append(timestamps, ts)
	}

	return timestamps, rows.Err()
}

// DeleteOlderThan deletes snapshots older than the specified time.
func (r *SnapshotRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	result, err := r.db.conn.ExecContext(ctx, "DELETE FROM snapshots WHERE timestamp < ?", before)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Count returns the total number of snapshots.
func (r *SnapshotRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM snapshots").Scan(&count)
	return count, err
}

// compress compresses data using gzip.
func compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decompress decompresses gzip data.
func decompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
