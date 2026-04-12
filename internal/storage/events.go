package storage

import (
	"context"
	"database/sql"
	"time"
)

// StoredEvent represents an event stored in the database.
type StoredEvent struct {
	ID              int64
	Cluster         string
	Namespace       string
	Kind            string
	Name            string
	Type            string
	Reason          string
	Message         string
	FirstSeen       time.Time
	LastSeen        time.Time
	Count           int
	SourceComponent string
	SourceHost      string
}

// EventFilter is used to filter events.
type EventFilter struct {
	Cluster   string
	Namespace string
	Kind      string
	Name      string
	Reason    string
	Type      string
	Since     time.Time
	Until     time.Time
	Limit     int
}

// EventRepository handles event storage operations.
type EventRepository struct {
	db *DB
}

// NewEventRepository creates a new event repository.
func NewEventRepository(db *DB) *EventRepository {
	return &EventRepository{db: db}
}

// Save saves or updates an event.
func (r *EventRepository) Save(ctx context.Context, event *StoredEvent) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	// Try to find existing event
	var existingID int64
	err := r.db.conn.QueryRowContext(ctx, `
		SELECT id FROM events 
		WHERE cluster = ? AND namespace = ? AND kind = ? AND name = ? AND reason = ?
		ORDER BY last_seen DESC LIMIT 1
	`, event.Cluster, event.Namespace, event.Kind, event.Name, event.Reason).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Insert new event
		result, err := r.db.conn.ExecContext(ctx, `
			INSERT INTO events (cluster, namespace, kind, name, type, reason, message, 
				first_seen, last_seen, count, source_component, source_host)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, event.Cluster, event.Namespace, event.Kind, event.Name, event.Type, event.Reason,
			event.Message, event.FirstSeen, event.LastSeen, event.Count,
			event.SourceComponent, event.SourceHost)
		if err != nil {
			return err
		}
		event.ID, _ = result.LastInsertId()
		return nil
	} else if err != nil {
		return err
	}

	// Update existing event
	_, err = r.db.conn.ExecContext(ctx, `
		UPDATE events SET 
			last_seen = ?, count = count + ?, message = ?
		WHERE id = ?
	`, event.LastSeen, event.Count, event.Message, existingID)
	event.ID = existingID
	return err
}

// List lists events matching the filter.
//
// No Go-level read lock: SQLite WAL mode permits concurrent readers alongside
// a single writer, and database/sql manages its own connection pool. Adding
// an application-level RWMutex here only serialized readers behind writers
// for no benefit.
func (r *EventRepository) List(ctx context.Context, filter EventFilter) ([]StoredEvent, error) {
	query := `
		SELECT id, cluster, namespace, kind, name, type, reason, message, 
			first_seen, last_seen, count, source_component, source_host
		FROM events WHERE 1=1
	`
	args := make([]interface{}, 0)

	if filter.Cluster != "" {
		query += " AND cluster = ?"
		args = append(args, filter.Cluster)
	}
	if filter.Namespace != "" {
		query += " AND namespace = ?"
		args = append(args, filter.Namespace)
	}
	if filter.Kind != "" {
		query += " AND kind = ?"
		args = append(args, filter.Kind)
	}
	if filter.Name != "" {
		query += " AND name = ?"
		args = append(args, filter.Name)
	}
	if filter.Reason != "" {
		query += " AND reason = ?"
		args = append(args, filter.Reason)
	}
	if filter.Type != "" {
		query += " AND type = ?"
		args = append(args, filter.Type)
	}
	if !filter.Since.IsZero() {
		query += " AND last_seen >= ?"
		args = append(args, filter.Since)
	}
	if !filter.Until.IsZero() {
		query += " AND last_seen <= ?"
		args = append(args, filter.Until)
	}

	query += " ORDER BY last_seen DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	rows, err := r.db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []StoredEvent
	for rows.Next() {
		var e StoredEvent
		err := rows.Scan(&e.ID, &e.Cluster, &e.Namespace, &e.Kind, &e.Name,
			&e.Type, &e.Reason, &e.Message, &e.FirstSeen, &e.LastSeen,
			&e.Count, &e.SourceComponent, &e.SourceHost)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}

	return events, rows.Err()
}

// DeleteOlderThan deletes events older than the specified time.
func (r *EventRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	result, err := r.db.conn.ExecContext(ctx, "DELETE FROM events WHERE last_seen < ?", before)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Count returns the total number of events.
func (r *EventRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&count)
	return count, err
}
