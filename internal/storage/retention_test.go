package storage

import (
	"context"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// insertEvent inserts a raw event row bypassing the repository layer so we can
// control the last_seen timestamp precisely.
func insertEvent(t *testing.T, db *DB, cluster, name string, lastSeen time.Time) {
	t.Helper()
	_, err := db.conn.Exec(`
		INSERT INTO events (cluster, namespace, kind, name, type, reason, message,
			first_seen, last_seen, count)
		VALUES (?, 'default', 'Pod', ?, 'Warning', 'Backoff', 'msg', ?, ?, 1)
	`, cluster, name, lastSeen, lastSeen)
	if err != nil {
		t.Fatalf("insertEvent(%q): %v", name, err)
	}
}

func countEvents(t *testing.T, db *DB) int {
	t.Helper()
	var n int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM events").Scan(&n); err != nil {
		t.Fatalf("countEvents: %v", err)
	}
	return n
}

func insertSnapshot(t *testing.T, db *DB, cluster string, ts time.Time) {
	t.Helper()
	data := &SnapshotData{
		Cluster:   cluster,
		Timestamp: ts,
		Resources: map[string][]ResourceInfo{},
	}
	repo := NewSnapshotRepository(db)
	if err := repo.Save(context.Background(), cluster, data); err != nil {
		t.Fatalf("insertSnapshot: %v", err)
	}
}

func countSnapshots(t *testing.T, db *DB) int {
	t.Helper()
	var n int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM snapshots").Scan(&n); err != nil {
		t.Fatalf("countSnapshots: %v", err)
	}
	return n
}

// ---------------------------------------------------------------------------
// RetentionManager.cleanup via deleteOldRows
// ---------------------------------------------------------------------------

func TestRetentionManager_DeletesStaleEvents(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	now := time.Now()
	old := now.Add(-40 * 24 * time.Hour) // 40 days ago
	fresh := now.Add(-1 * time.Hour)

	insertEvent(t, db, "c1", "old-event", old)
	insertEvent(t, db, "c1", "fresh-event", fresh)

	cfg := RetentionConfig{
		EventsRetention:       30 * 24 * time.Hour, // keep 30 days
		SnapshotsRetention:    7 * 24 * time.Hour,
		CorrelationsRetention: 30 * 24 * time.Hour,
	}
	rm := NewRetentionManager(db, cfg)
	rm.cleanup(ctx)

	remaining := countEvents(t, db)
	if remaining != 1 {
		t.Errorf("expected 1 event remaining after cleanup, got %d", remaining)
	}
}

func TestRetentionManager_PreservesFreshEvents(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	now := time.Now()
	for i := 0; i < 5; i++ {
		insertEvent(t, db, "c1", "recent-event", now.Add(-time.Duration(i)*time.Hour))
	}

	cfg := RetentionConfig{
		EventsRetention:       30 * 24 * time.Hour,
		SnapshotsRetention:    7 * 24 * time.Hour,
		CorrelationsRetention: 30 * 24 * time.Hour,
	}
	rm := NewRetentionManager(db, cfg)
	rm.cleanup(ctx)

	if n := countEvents(t, db); n != 5 {
		t.Errorf("expected all 5 fresh events preserved, got %d", n)
	}
}

func TestRetentionManager_ZeroRetention_KeepsEverything(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	insertEvent(t, db, "c1", "ancient", time.Now().Add(-365*24*time.Hour))

	cfg := RetentionConfig{
		EventsRetention:       0, // disabled
		SnapshotsRetention:    0,
		CorrelationsRetention: 0,
	}
	rm := NewRetentionManager(db, cfg)
	rm.cleanup(ctx)

	if n := countEvents(t, db); n != 1 {
		t.Errorf("zero retention should preserve all rows, got %d", n)
	}
}

func TestRetentionManager_DefaultConfig_HasSensibleValues(t *testing.T) {
	cfg := DefaultRetentionConfig()

	if cfg.EventsRetention <= 0 {
		t.Errorf("DefaultRetentionConfig.EventsRetention = %v, want positive duration", cfg.EventsRetention)
	}
	if cfg.SnapshotsRetention <= 0 {
		t.Errorf("DefaultRetentionConfig.SnapshotsRetention = %v, want positive duration", cfg.SnapshotsRetention)
	}
	if cfg.CorrelationsRetention <= 0 {
		t.Errorf("DefaultRetentionConfig.CorrelationsRetention = %v, want positive duration", cfg.CorrelationsRetention)
	}
}

func TestRetentionManager_DeletesStaleSnapshots(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	now := time.Now()
	insertSnapshot(t, db, "c1", now.Add(-10*24*time.Hour)) // older than 7-day window
	insertSnapshot(t, db, "c1", now.Add(-1*time.Hour))     // fresh

	cfg := RetentionConfig{
		EventsRetention:       30 * 24 * time.Hour,
		SnapshotsRetention:    7 * 24 * time.Hour,
		CorrelationsRetention: 30 * 24 * time.Hour,
	}
	rm := NewRetentionManager(db, cfg)
	rm.cleanup(ctx)

	if n := countSnapshots(t, db); n != 1 {
		t.Errorf("expected 1 snapshot after cleanup, got %d", n)
	}
}
