package storage

import (
	"context"
	"database/sql"
	"testing"
)

// openTestDB opens an in-memory SQLite database suitable for tests.
// Each call returns an independent database instance.
func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := OpenPath(":memory:")
	if err != nil {
		t.Fatalf("openTestDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// ---------------------------------------------------------------------------
// Migrate
// ---------------------------------------------------------------------------

func TestMigrate_CreatesRequiredTables(t *testing.T) {
	db := openTestDB(t)

	tables := []string{"migrations", "snapshots", "events", "correlations", "resources", "settings"}
	for _, table := range tables {
		t.Run(table, func(t *testing.T) {
			var name string
			err := db.conn.QueryRow(
				"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
			).Scan(&name)
			if err == sql.ErrNoRows {
				t.Errorf("table %q not created by Migrate", table)
			} else if err != nil {
				t.Fatalf("querying sqlite_master: %v", err)
			}
		})
	}
}

func TestMigrate_CreatesExpectedIndexes(t *testing.T) {
	db := openTestDB(t)

	indexes := []string{
		"idx_snapshots_cluster",
		"idx_snapshots_timestamp",
		"idx_events_cluster",
		"idx_events_namespace",
		"idx_events_kind_name",
		"idx_events_last_seen",
		"idx_events_reason",
		"idx_correlations_source",
		"idx_correlations_target",
		"idx_resources_cluster",
		"idx_resources_kind",
	}
	for _, idx := range indexes {
		t.Run(idx, func(t *testing.T) {
			var name string
			err := db.conn.QueryRow(
				"SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx,
			).Scan(&name)
			if err == sql.ErrNoRows {
				t.Errorf("index %q not created by Migrate", idx)
			} else if err != nil {
				t.Fatalf("querying sqlite_master: %v", err)
			}
		})
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	db := openTestDB(t)

	// Run migrate three more times on the same db — must not error.
	for i := 0; i < 3; i++ {
		if err := db.Migrate(); err != nil {
			t.Fatalf("Migrate() call %d returned error: %v", i+1, err)
		}
	}

	// Migration version table should record each migration exactly once.
	rows, err := db.conn.Query("SELECT version FROM migrations ORDER BY version")
	if err != nil {
		t.Fatalf("querying migrations: %v", err)
	}
	defer rows.Close()

	seen := make(map[int]int)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scanning version: %v", err)
		}
		seen[v]++
	}
	for v, count := range seen {
		if count != 1 {
			t.Errorf("migration version %d recorded %d times, want 1", v, count)
		}
	}
}

func TestMigrate_ForwardMigration_SettingsTableAppliedAfterEvents(t *testing.T) {
	// Confirm migration version 2 (settings) runs after version 1.
	db := openTestDB(t)

	var maxVersion int
	if err := db.conn.QueryRow("SELECT COALESCE(MAX(version), 0) FROM migrations").Scan(&maxVersion); err != nil {
		t.Fatalf("reading max migration version: %v", err)
	}
	if maxVersion < 2 {
		t.Errorf("expected max migration version >= 2, got %d", maxVersion)
	}

	// settings table must be queryable
	_, err := db.conn.Exec("INSERT INTO settings (key, value) VALUES ('test', 'val')")
	if err != nil {
		t.Errorf("settings table not usable after migration: %v", err)
	}
}

func TestMigrate_ForeignKeysEnabled(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	// Insert a parent event
	result, err := db.conn.ExecContext(ctx, `
		INSERT INTO events (cluster, namespace, kind, name, type, reason, message, last_seen, count)
		VALUES ('c1', 'default', 'Pod', 'pod-a', 'Warning', 'Backoff', 'msg', CURRENT_TIMESTAMP, 1)
	`)
	if err != nil {
		t.Fatalf("inserting parent event: %v", err)
	}
	parentID, _ := result.LastInsertId()

	// Insert correlation referencing parent — must succeed
	_, err = db.conn.ExecContext(ctx, `
		INSERT INTO correlations (source_event_id, target_event_id, confidence, relationship)
		VALUES (?, ?, 0.9, 'related')
	`, parentID, parentID)
	if err != nil {
		t.Fatalf("valid correlation insert failed: %v", err)
	}

	// Insert correlation referencing non-existent event — must fail due to FK
	_, err = db.conn.ExecContext(ctx, `
		INSERT INTO correlations (source_event_id, target_event_id, confidence, relationship)
		VALUES (99999, 99998, 0.9, 'related')
	`)
	if err == nil {
		t.Error("expected foreign key violation for non-existent event IDs, got nil")
	}
}

func TestOpenPath_InMemory_IndependentInstances(t *testing.T) {
	db1 := openTestDB(t)
	db2 := openTestDB(t)

	// Write to db1
	_, err := db1.conn.Exec("INSERT INTO settings (key, value) VALUES ('k', 'v')")
	if err != nil {
		t.Fatalf("insert in db1: %v", err)
	}

	// db2 must not see db1's data
	var count int
	if err := db2.conn.QueryRow("SELECT COUNT(*) FROM settings").Scan(&count); err != nil {
		t.Fatalf("count in db2: %v", err)
	}
	if count != 0 {
		t.Errorf("db2 unexpectedly contains %d rows from db1 (memory isolation broken)", count)
	}
}

func TestTransaction_RollsBackOnError(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	insertErr := db.Transaction(ctx, func(tx *sql.Tx) error {
		if _, err := tx.Exec("INSERT INTO settings (key, value) VALUES ('rollback-key', 'v')"); err != nil {
			return err
		}
		return errTest("forced rollback")
	})
	if insertErr == nil {
		t.Fatal("expected transaction to return the forced error, got nil")
	}

	var count int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM settings WHERE key='rollback-key'").Scan(&count); err != nil {
		t.Fatalf("post-rollback count: %v", err)
	}
	if count != 0 {
		t.Errorf("transaction rollback did not remove row; count=%d", count)
	}
}

func TestTransaction_CommitsOnSuccess(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := db.Transaction(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO settings (key, value) VALUES ('commit-key', 'v')")
		return err
	}); err != nil {
		t.Fatalf("transaction failed: %v", err)
	}

	var count int
	if err := db.conn.QueryRow("SELECT COUNT(*) FROM settings WHERE key='commit-key'").Scan(&count); err != nil {
		t.Fatalf("post-commit count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 committed row, got %d", count)
	}
}

// errTest is a minimal error for forcing transaction rollback in tests.
type errTest string

func (e errTest) Error() string { return string(e) }
