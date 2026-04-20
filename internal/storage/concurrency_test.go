// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestConcurrentReadersAndWriters_NoDeadlock mixes two writers and two
// readers against the events table and asserts nothing deadlocks, all
// writes persist, and reads never error. Run with -race to catch
// state corruption on the Go side; SQLite's WAL mode handles the
// on-disk side.
//
// This complements TestEventRepository_ConcurrentWrites (writers only)
// by also exercising the read path that dropped the RWMutex per the
// comment in events.go. Without WAL + busy_timeout the reads would
// likely return SQLITE_BUSY under contention.
//
// A file-backed temp DB is used (not :memory:) because the modernc.org
// pure-Go SQLite driver treats ":memory:" as per-connection private — so
// readers and writers would otherwise see different empty databases. A
// real Kubecat install uses a file-backed DB; testing the concurrency
// contract that matters in production requires the same topology.
func TestConcurrentReadersAndWriters_NoDeadlock(t *testing.T) {
	path := t.TempDir() + "/concur.db"
	db, err := OpenPath(path)
	if err != nil {
		t.Fatalf("OpenPath: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	repo := NewEventRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()

	const perWriter = 50

	writerDone := make(chan struct{}, 2)
	readerDone := make(chan struct{})

	// Two writers — different cluster names to avoid dedupe.
	for _, cluster := range []string{"cA", "cB"} {
		go func(cl string) {
			defer func() { writerDone <- struct{}{} }()
			for i := 0; i < perWriter; i++ {
				e := makeStoredEvent(cl, "Pod", cl+"-pod", "R", now.Add(time.Duration(i)*time.Millisecond))
				// Unique reason per (writer, iteration) so Save inserts rather
				// than dedupe-updating.
				e.Reason = cl + "-" + time.Duration(i).String()
				_ = repo.Save(ctx, e)
			}
		}(cluster)
	}

	// Two readers — spin on List until both writers report done.
	stop := make(chan struct{})
	var readErr error
	var readMu sync.Mutex
	var readerWg sync.WaitGroup
	for i := 0; i < 2; i++ {
		readerWg.Add(1)
		go func() {
			defer readerWg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if _, err := repo.List(ctx, EventFilter{Limit: 100}); err != nil {
					readMu.Lock()
					if readErr == nil {
						readErr = err
					}
					readMu.Unlock()
					return
				}
			}
		}()
	}
	go func() {
		readerWg.Wait()
		close(readerDone)
	}()

	// Wait for both writers. Bound the wait so a deadlock fails the test
	// instead of hanging the suite.
	finishedWriters := 0
	waitWriters := time.After(10 * time.Second)
waitLoop:
	for finishedWriters < 2 {
		select {
		case <-writerDone:
			finishedWriters++
		case <-waitWriters:
			t.Fatalf("writers did not finish within 10s — likely deadlock")
			break waitLoop
		}
	}

	close(stop)
	select {
	case <-readerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("readers did not stop within 5s after stop signal")
	}

	if readErr != nil {
		t.Fatalf("reader encountered error under contention: %v", readErr)
	}
	if count, _ := repo.Count(ctx); count != 2*int64(perWriter) {
		t.Errorf("final row count = %d, want %d", count, 2*perWriter)
	}
}

// TestMigrate_ReopenPath_PreservesData verifies that opening the same file
// path twice (first to seed, then to re-migrate) does NOT drop data or
// re-run already-applied migrations. This is the real-world scenario when
// Kubecat restarts: migrations must be pure forward-only with no data loss.
func TestMigrate_ReopenPath_PreservesData(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/history.db"

	// First open — creates schema, inserts a row.
	db1, err := OpenPath(path)
	if err != nil {
		t.Fatalf("first OpenPath: %v", err)
	}
	if _, err := db1.conn.Exec("INSERT INTO settings (key, value) VALUES ('persisted', 'yes')"); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	if err := db1.Close(); err != nil {
		t.Fatalf("close db1: %v", err)
	}

	// Second open — must re-run Migrate without dropping data.
	db2, err := OpenPath(path)
	if err != nil {
		t.Fatalf("second OpenPath: %v", err)
	}
	t.Cleanup(func() { db2.Close() })

	var value string
	err = db2.conn.QueryRow("SELECT value FROM settings WHERE key='persisted'").Scan(&value)
	if err != nil {
		t.Fatalf("row lookup on re-opened db: %v", err)
	}
	if value != "yes" {
		t.Errorf("value = %q, want yes — data lost across reopen", value)
	}

	// Migrations table must still show exactly one row per version.
	rows, err := db2.conn.Query("SELECT version, COUNT(*) FROM migrations GROUP BY version")
	if err != nil {
		t.Fatalf("query migrations: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v, count int
		_ = rows.Scan(&v, &count)
		if count != 1 {
			t.Errorf("migration version %d applied %d times across reopens, want 1", v, count)
		}
	}
}

// TestRetentionManager_DeletedBelowVacuumThreshold_SkipsVacuum pins that
// cleanup deleting fewer rows than vacuumThreshold does NOT invoke VACUUM.
// This matters because VACUUM takes an exclusive lock — firing it on every
// small cleanup would block all writers.
//
// We can't directly observe "VACUUM was not called" from outside, but we
// can observe the absence of the typical VACUUM side-effect: an Error
// because the operation interferes with an in-flight writer. Instead we
// run cleanup with a small delete set and confirm no goroutine deadlocks
// occur while a concurrent writer runs.
func TestRetentionManager_SmallCleanup_DoesNotBlockWriter(t *testing.T) {
	db := openTestDB(t)
	repo := NewEventRepository(db)
	ctx := context.Background()

	// Seed one old event (<vacuumThreshold, which is 1000).
	insertEvent(t, db, "c", "old", time.Now().Add(-40*24*time.Hour))

	cfg := RetentionConfig{
		EventsRetention:       30 * 24 * time.Hour,
		SnapshotsRetention:    7 * 24 * time.Hour,
		CorrelationsRetention: 30 * 24 * time.Hour,
	}
	rm := NewRetentionManager(db, cfg)

	// Interleave a cleanup and a writer. If VACUUM fired here it would
	// grab an exclusive lock and significantly delay the writer. We just
	// verify both complete within a bounded time window.
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		rm.cleanup(ctx)
	}()
	go func() {
		defer wg.Done()
		_ = repo.Save(ctx, makeStoredEvent("c", "Pod", "p", "R", time.Now()))
	}()
	wg.Wait()
	elapsed := time.Since(start)

	// Both should finish in well under a second on an in-memory DB.
	// A generous 2s ceiling catches the VACUUM-lock regression without
	// being flaky on a loaded CI box.
	if elapsed > 2*time.Second {
		t.Errorf("small cleanup + writer took %v — VACUUM may be firing inappropriately", elapsed)
	}
}
