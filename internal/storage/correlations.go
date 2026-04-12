package storage

import (
	"context"
	"time"
)

// Correlation represents a correlation between two events.
type Correlation struct {
	ID            int64
	SourceEventID int64
	TargetEventID int64
	Confidence    float64
	Relationship  string
	CreatedAt     time.Time
}

// CorrelationWithEvents includes the full event data.
type CorrelationWithEvents struct {
	Correlation
	SourceEvent StoredEvent
	TargetEvent StoredEvent
}

// CorrelationRepository handles correlation storage operations.
type CorrelationRepository struct {
	db *DB
}

// NewCorrelationRepository creates a new correlation repository.
func NewCorrelationRepository(db *DB) *CorrelationRepository {
	return &CorrelationRepository{db: db}
}

// Save saves a correlation.
func (r *CorrelationRepository) Save(ctx context.Context, correlation *Correlation) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	result, err := r.db.conn.ExecContext(ctx, `
		INSERT INTO correlations (source_event_id, target_event_id, confidence, relationship)
		VALUES (?, ?, ?, ?)
	`, correlation.SourceEventID, correlation.TargetEventID,
		correlation.Confidence, correlation.Relationship)
	if err != nil {
		return err
	}

	correlation.ID, _ = result.LastInsertId()
	return nil
}

// FindBySource finds correlations where the given event is the source.
func (r *CorrelationRepository) FindBySource(ctx context.Context, sourceEventID int64) ([]CorrelationWithEvents, error) {
	rows, err := r.db.conn.QueryContext(ctx, `
		SELECT 
			c.id, c.source_event_id, c.target_event_id, c.confidence, c.relationship, c.created_at,
			se.id, se.cluster, se.namespace, se.kind, se.name, se.type, se.reason, se.message,
			se.first_seen, se.last_seen, se.count, se.source_component, se.source_host,
			te.id, te.cluster, te.namespace, te.kind, te.name, te.type, te.reason, te.message,
			te.first_seen, te.last_seen, te.count, te.source_component, te.source_host
		FROM correlations c
		JOIN events se ON c.source_event_id = se.id
		JOIN events te ON c.target_event_id = te.id
		WHERE c.source_event_id = ?
		ORDER BY c.confidence DESC
	`, sourceEventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanCorrelationsWithEvents(rows)
}

// FindByTarget finds correlations where the given event is the target.
func (r *CorrelationRepository) FindByTarget(ctx context.Context, targetEventID int64) ([]CorrelationWithEvents, error) {
	rows, err := r.db.conn.QueryContext(ctx, `
		SELECT 
			c.id, c.source_event_id, c.target_event_id, c.confidence, c.relationship, c.created_at,
			se.id, se.cluster, se.namespace, se.kind, se.name, se.type, se.reason, se.message,
			se.first_seen, se.last_seen, se.count, se.source_component, se.source_host,
			te.id, te.cluster, te.namespace, te.kind, te.name, te.type, te.reason, te.message,
			te.first_seen, te.last_seen, te.count, te.source_component, te.source_host
		FROM correlations c
		JOIN events se ON c.source_event_id = se.id
		JOIN events te ON c.target_event_id = te.id
		WHERE c.target_event_id = ?
		ORDER BY c.confidence DESC
	`, targetEventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanCorrelationsWithEvents(rows)
}

// FindHighConfidence finds correlations above a confidence threshold.
func (r *CorrelationRepository) FindHighConfidence(ctx context.Context, minConfidence float64, limit int) ([]CorrelationWithEvents, error) {
	query := `
		SELECT 
			c.id, c.source_event_id, c.target_event_id, c.confidence, c.relationship, c.created_at,
			se.id, se.cluster, se.namespace, se.kind, se.name, se.type, se.reason, se.message,
			se.first_seen, se.last_seen, se.count, se.source_component, se.source_host,
			te.id, te.cluster, te.namespace, te.kind, te.name, te.type, te.reason, te.message,
			te.first_seen, te.last_seen, te.count, te.source_component, te.source_host
		FROM correlations c
		JOIN events se ON c.source_event_id = se.id
		JOIN events te ON c.target_event_id = te.id
		WHERE c.confidence >= ?
		ORDER BY c.created_at DESC
	`
	args := []interface{}{minConfidence}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := r.db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanCorrelationsWithEvents(rows)
}

// scanCorrelationsWithEvents scans correlation rows with event data.
func (r *CorrelationRepository) scanCorrelationsWithEvents(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]CorrelationWithEvents, error) {
	var correlations []CorrelationWithEvents

	for rows.Next() {
		var c CorrelationWithEvents
		err := rows.Scan(
			&c.ID, &c.SourceEventID, &c.TargetEventID, &c.Confidence, &c.Relationship, &c.CreatedAt,
			&c.SourceEvent.ID, &c.SourceEvent.Cluster, &c.SourceEvent.Namespace,
			&c.SourceEvent.Kind, &c.SourceEvent.Name, &c.SourceEvent.Type,
			&c.SourceEvent.Reason, &c.SourceEvent.Message, &c.SourceEvent.FirstSeen,
			&c.SourceEvent.LastSeen, &c.SourceEvent.Count,
			&c.SourceEvent.SourceComponent, &c.SourceEvent.SourceHost,
			&c.TargetEvent.ID, &c.TargetEvent.Cluster, &c.TargetEvent.Namespace,
			&c.TargetEvent.Kind, &c.TargetEvent.Name, &c.TargetEvent.Type,
			&c.TargetEvent.Reason, &c.TargetEvent.Message, &c.TargetEvent.FirstSeen,
			&c.TargetEvent.LastSeen, &c.TargetEvent.Count,
			&c.TargetEvent.SourceComponent, &c.TargetEvent.SourceHost,
		)
		if err != nil {
			return nil, err
		}
		correlations = append(correlations, c)
	}

	return correlations, rows.Err()
}

// DeleteOlderThan deletes correlations older than the specified time.
func (r *CorrelationRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	result, err := r.db.conn.ExecContext(ctx, "DELETE FROM correlations WHERE created_at < ?", before)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
