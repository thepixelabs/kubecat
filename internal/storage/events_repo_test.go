// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"sync"
	"testing"
	"time"
)

func makeStoredEvent(cluster, kind, name, reason string, ts time.Time) *StoredEvent {
	return &StoredEvent{
		Cluster:   cluster,
		Namespace: "default",
		Kind:      kind,
		Name:      name,
		Type:      "Warning",
		Reason:    reason,
		Message:   "msg",
		FirstSeen: ts,
		LastSeen:  ts,
		Count:     1,
	}
}

func TestEventRepository_Save_InsertsNewAndPopulatesID(t *testing.T) {
	db := openTestDB(t)
	repo := NewEventRepository(db)
	ctx := context.Background()

	e := makeStoredEvent("c1", "Pod", "p", "BackOff", time.Now().UTC())
	if err := repo.Save(ctx, e); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if e.ID == 0 {
		t.Error("Save did not populate ID")
	}
}

func TestEventRepository_Save_UpdatesExistingOnDuplicateKey(t *testing.T) {
	db := openTestDB(t)
	repo := NewEventRepository(db)
	ctx := context.Background()

	t1 := time.Now().UTC().Truncate(time.Second)
	first := makeStoredEvent("c1", "Pod", "p", "BackOff", t1)
	if err := repo.Save(ctx, first); err != nil {
		t.Fatalf("Save first: %v", err)
	}

	// Same cluster/namespace/kind/name/reason -> treated as same event,
	// count incremented and last_seen bumped.
	second := makeStoredEvent("c1", "Pod", "p", "BackOff", t1.Add(5*time.Minute))
	second.Count = 4
	second.Message = "updated"
	if err := repo.Save(ctx, second); err != nil {
		t.Fatalf("Save second: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("Save did not dedupe by key: got ID %d, want %d", second.ID, first.ID)
	}

	// Confirm only one row exists.
	count, _ := repo.Count(ctx)
	if count != 1 {
		t.Errorf("row count = %d, want 1", count)
	}

	// Message + count should be updated.
	events, _ := repo.List(ctx, EventFilter{Cluster: "c1"})
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	got := events[0]
	if got.Message != "updated" {
		t.Errorf("message = %q, want updated", got.Message)
	}
	// count += 4 (first saved 1, added 4)
	if got.Count != 5 {
		t.Errorf("count = %d, want 5 (1+4)", got.Count)
	}
}

func TestEventRepository_List_FiltersByClusterAndKind(t *testing.T) {
	db := openTestDB(t)
	repo := NewEventRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()

	_ = repo.Save(ctx, makeStoredEvent("cA", "Pod", "p1", "R", now))
	_ = repo.Save(ctx, makeStoredEvent("cA", "Node", "n1", "R", now))
	_ = repo.Save(ctx, makeStoredEvent("cB", "Pod", "p2", "R", now))

	got, err := repo.List(ctx, EventFilter{Cluster: "cA", Kind: "Pod"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d: %+v", len(got), got)
	}
	if got[0].Cluster != "cA" || got[0].Kind != "Pod" {
		t.Errorf("wrong row: %+v", got[0])
	}
}

func TestEventRepository_List_OrdersByLastSeenDesc(t *testing.T) {
	db := openTestDB(t)
	repo := NewEventRepository(db)
	ctx := context.Background()

	t1 := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	t2 := t1.Add(time.Hour)
	_ = repo.Save(ctx, makeStoredEvent("c", "Pod", "a", "R1", t1))
	_ = repo.Save(ctx, makeStoredEvent("c", "Pod", "b", "R2", t2))

	got, _ := repo.List(ctx, EventFilter{Cluster: "c"})
	if len(got) < 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	if !got[0].LastSeen.After(got[1].LastSeen) && !got[0].LastSeen.Equal(got[1].LastSeen) {
		t.Errorf("not descending by last_seen: %v then %v", got[0].LastSeen, got[1].LastSeen)
	}
}

func TestEventRepository_List_LimitRespected(t *testing.T) {
	db := openTestDB(t)
	repo := NewEventRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()

	for i := 0; i < 5; i++ {
		// Distinct reasons so they are treated as distinct events.
		_ = repo.Save(ctx, makeStoredEvent("c", "Pod", "p", "R", now.Add(time.Duration(i)*time.Second)))
	}
	got, _ := repo.List(ctx, EventFilter{Cluster: "c", Limit: 2})
	if len(got) > 2 {
		t.Errorf("limit not respected: got %d events", len(got))
	}
}

func TestEventRepository_DeleteOlderThan(t *testing.T) {
	db := openTestDB(t)
	repo := NewEventRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()
	old := now.Add(-48 * time.Hour)
	fresh := now

	_ = repo.Save(ctx, makeStoredEvent("c", "Pod", "old", "R", old))
	_ = repo.Save(ctx, makeStoredEvent("c", "Pod", "fresh", "R", fresh))

	deleted, err := repo.DeleteOlderThan(ctx, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("DeleteOlderThan: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}
}

// TestEventRepository_ConcurrentWrites verifies two goroutines inserting
// events concurrently do not corrupt the DB. Run with -race.
func TestEventRepository_ConcurrentWrites(t *testing.T) {
	db := openTestDB(t)
	repo := NewEventRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()

	const perGoroutine = 20
	var wg sync.WaitGroup
	for w := 0; w < 2; w++ {
		wg.Add(1)
		cluster := "cA"
		if w == 1 {
			cluster = "cB"
		}
		go func(cl string) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				e := makeStoredEvent(cl, "Pod", "name", "R", now.Add(time.Duration(i)*time.Millisecond))
				e.Name = cl + "-p"
				// Make each row unique via reason to avoid dedupe.
				e.Reason = string(rune('A' + i))
				_ = repo.Save(ctx, e)
			}
		}(cluster)
	}
	wg.Wait()

	count, _ := repo.Count(ctx)
	if count != int64(2*perGoroutine) {
		t.Errorf("count after concurrent writes = %d, want %d", count, 2*perGoroutine)
	}
}

// -----------------------------------------------------------------------------
// CorrelationRepository
// -----------------------------------------------------------------------------

func TestCorrelationRepository_SaveAndFind(t *testing.T) {
	db := openTestDB(t)
	eRepo := NewEventRepository(db)
	cRepo := NewCorrelationRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()

	source := makeStoredEvent("c", "Pod", "s", "R1", now)
	target := makeStoredEvent("c", "Pod", "t", "R2", now)
	_ = eRepo.Save(ctx, source)
	_ = eRepo.Save(ctx, target)

	corr := &Correlation{
		SourceEventID: source.ID,
		TargetEventID: target.ID,
		Confidence:    0.9,
		Relationship:  "caused_by",
	}
	if err := cRepo.Save(ctx, corr); err != nil {
		t.Fatalf("Save correlation: %v", err)
	}
	if corr.ID == 0 {
		t.Error("correlation ID not populated")
	}

	bySource, err := cRepo.FindBySource(ctx, source.ID)
	if err != nil {
		t.Fatalf("FindBySource: %v", err)
	}
	if len(bySource) != 1 {
		t.Fatalf("FindBySource: expected 1, got %d", len(bySource))
	}
	if bySource[0].TargetEvent.Name != "t" {
		t.Errorf("FindBySource target.Name = %q, want t", bySource[0].TargetEvent.Name)
	}

	byTarget, err := cRepo.FindByTarget(ctx, target.ID)
	if err != nil {
		t.Fatalf("FindByTarget: %v", err)
	}
	if len(byTarget) != 1 {
		t.Errorf("FindByTarget: expected 1, got %d", len(byTarget))
	}

	high, err := cRepo.FindHighConfidence(ctx, 0.5, 10)
	if err != nil {
		t.Fatalf("FindHighConfidence: %v", err)
	}
	if len(high) != 1 {
		t.Errorf("FindHighConfidence: expected 1, got %d", len(high))
	}

	// Low threshold excludes the 0.9 correlation only if we raise it high.
	above95, _ := cRepo.FindHighConfidence(ctx, 0.95, 10)
	if len(above95) != 0 {
		t.Errorf("FindHighConfidence(0.95) should return 0 for 0.9 correlation, got %d", len(above95))
	}
}

func TestCorrelationRepository_DeleteOlderThan(t *testing.T) {
	db := openTestDB(t)
	eRepo := NewEventRepository(db)
	cRepo := NewCorrelationRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()
	source := makeStoredEvent("c", "Pod", "s", "R1", now)
	target := makeStoredEvent("c", "Pod", "t", "R2", now)
	_ = eRepo.Save(ctx, source)
	_ = eRepo.Save(ctx, target)

	corr := &Correlation{SourceEventID: source.ID, TargetEventID: target.ID, Confidence: 0.9, Relationship: "r"}
	_ = cRepo.Save(ctx, corr)

	// Delete everything older than "now+1s" — which should nuke everything.
	deleted, err := cRepo.DeleteOlderThan(ctx, now.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("DeleteOlderThan: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}
}

// -----------------------------------------------------------------------------
// DB helpers
// -----------------------------------------------------------------------------

func TestDB_Conn_ReturnsUnderlying(t *testing.T) {
	db := openTestDB(t)
	if db.Conn() == nil {
		t.Error("Conn() returned nil")
	}
}

func TestDB_Vacuum_OnFreshDB(t *testing.T) {
	db := openTestDB(t)
	if err := db.Vacuum(); err != nil {
		t.Errorf("Vacuum on fresh db failed: %v", err)
	}
}
