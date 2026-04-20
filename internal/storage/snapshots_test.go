// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"sync"
	"testing"
	"time"
)

// makeSnapshotData builds minimal-but-valid SnapshotData for tests.
func makeSnapshotData(cluster string, ts time.Time, kinds ...string) *SnapshotData {
	res := make(map[string][]ResourceInfo, len(kinds))
	for _, k := range kinds {
		res[k] = []ResourceInfo{
			{Name: "a", Namespace: "default", ResourceVersion: "1", Status: "Running"},
		}
	}
	return &SnapshotData{
		Cluster:   cluster,
		Timestamp: ts,
		Resources: res,
	}
}

func TestSnapshotRepository_SaveAndGetLatest_RoundTrip(t *testing.T) {
	db := openTestDB(t)
	repo := NewSnapshotRepository(db)
	ctx := context.Background()

	in := makeSnapshotData("c1", time.Now().UTC().Truncate(time.Second), "pods", "services")
	if err := repo.Save(ctx, "c1", in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	out, err := repo.GetLatest(ctx, "c1")
	if err != nil {
		t.Fatalf("GetLatest: %v", err)
	}
	if out.Cluster != in.Cluster {
		t.Errorf("cluster = %q, want %q", out.Cluster, in.Cluster)
	}
	if len(out.Resources) != len(in.Resources) {
		t.Errorf("resource kinds = %d, want %d", len(out.Resources), len(in.Resources))
	}
	// resource round-trip
	if out.Resources["pods"][0].Name != "a" {
		t.Errorf("pod name lost on round-trip, got %+v", out.Resources["pods"])
	}
}

func TestSnapshotRepository_GetAt_SelectsSnapshotAtOrBefore(t *testing.T) {
	db := openTestDB(t)
	repo := NewSnapshotRepository(db)
	ctx := context.Background()

	t0 := time.Now().UTC().Add(-3 * time.Hour).Truncate(time.Second)
	t1 := t0.Add(1 * time.Hour)
	t2 := t0.Add(2 * time.Hour)

	for _, ts := range []time.Time{t0, t1, t2} {
		if err := repo.Save(ctx, "c1", makeSnapshotData("c1", ts, "pods")); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	// Request slightly after t1 — should return t1.
	got, err := repo.GetAt(ctx, "c1", t1.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("GetAt: %v", err)
	}
	if !got.Timestamp.Equal(t1) {
		t.Errorf("GetAt(t1+10m) = %v, want %v", got.Timestamp, t1)
	}
}

func TestSnapshotRepository_ListTimestamps_DescendingOrder(t *testing.T) {
	db := openTestDB(t)
	repo := NewSnapshotRepository(db)
	ctx := context.Background()

	t0 := time.Now().UTC().Truncate(time.Second)
	for i := 0; i < 3; i++ {
		_ = repo.Save(ctx, "c1", makeSnapshotData("c1", t0.Add(time.Duration(i)*time.Minute), "pods"))
	}

	ts, err := repo.ListTimestamps(ctx, "c1", 0)
	if err != nil {
		t.Fatalf("ListTimestamps: %v", err)
	}
	if len(ts) != 3 {
		t.Fatalf("expected 3 timestamps, got %d", len(ts))
	}
	for i := 0; i+1 < len(ts); i++ {
		if !ts[i].After(ts[i+1]) {
			t.Errorf("ListTimestamps not descending at index %d: %v !> %v", i, ts[i], ts[i+1])
		}
	}
}

func TestSnapshotRepository_DeleteOlderThan_RemovesStale(t *testing.T) {
	db := openTestDB(t)
	repo := NewSnapshotRepository(db)
	ctx := context.Background()

	old := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second)
	fresh := time.Now().UTC().Truncate(time.Second)

	_ = repo.Save(ctx, "c1", makeSnapshotData("c1", old, "pods"))
	_ = repo.Save(ctx, "c1", makeSnapshotData("c1", fresh, "pods"))

	deleted, err := repo.DeleteOlderThan(ctx, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("DeleteOlderThan: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	count, _ := repo.Count(ctx)
	if count != 1 {
		t.Errorf("remaining count = %d, want 1", count)
	}
}

func TestSnapshotRepository_Get_ReturnsErrorForMissing(t *testing.T) {
	db := openTestDB(t)
	repo := NewSnapshotRepository(db)
	_, err := repo.Get(context.Background(), 99999)
	if err == nil {
		t.Error("expected error for missing snapshot ID")
	}
}

// -----------------------------------------------------------------------------
// Concurrency
// -----------------------------------------------------------------------------

// TestSnapshotRepository_ConcurrentWrites verifies that two goroutines writing
// snapshots simultaneously do not corrupt the DB. Run with -race.
func TestSnapshotRepository_ConcurrentWrites(t *testing.T) {
	db := openTestDB(t)
	repo := NewSnapshotRepository(db)
	ctx := context.Background()

	const perGoroutine = 10
	start := time.Now().UTC().Truncate(time.Second)

	var wg sync.WaitGroup
	for w := 0; w < 2; w++ {
		wg.Add(1)
		cluster := "c1"
		if w == 1 {
			cluster = "c2"
		}
		go func(cl string) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				// Use unique timestamps per goroutine to avoid UNIQUE(cluster,timestamp) collision.
				ts := start.Add(time.Duration(i) * time.Millisecond * 50)
				_ = repo.Save(ctx, cl, makeSnapshotData(cl, ts, "pods"))
			}
		}(cluster)
	}
	wg.Wait()

	got, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != int64(2*perGoroutine) {
		t.Errorf("count = %d, want %d", got, 2*perGoroutine)
	}
}

// -----------------------------------------------------------------------------
// Migration: old schema → current
// -----------------------------------------------------------------------------

// TestMigrate_AppliesVersion2_OnV1SeededDB confirms re-running Migrate on a DB
// that only has version 1 applied will forward-migrate to the current version.
func TestMigrate_AppliesVersion2_OnV1SeededDB(t *testing.T) {
	db := openTestDB(t)

	// Simulate a pre-v2 state: erase v2 record from migrations table and
	// drop the settings table.
	if _, err := db.conn.Exec("DELETE FROM migrations WHERE version >= 2"); err != nil {
		t.Fatalf("reset migrations table: %v", err)
	}
	if _, err := db.conn.Exec("DROP TABLE IF EXISTS settings"); err != nil {
		t.Fatalf("drop settings: %v", err)
	}

	// Re-run migrate: should re-apply v2.
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate forward: %v", err)
	}

	// settings table must be usable again.
	if _, err := db.conn.Exec("INSERT INTO settings (key, value) VALUES ('k','v')"); err != nil {
		t.Errorf("settings not usable after forward migration: %v", err)
	}

	var maxVer int
	_ = db.conn.QueryRow("SELECT COALESCE(MAX(version),0) FROM migrations").Scan(&maxVer)
	if maxVer < 2 {
		t.Errorf("max migration version = %d, want >= 2", maxVer)
	}
}
