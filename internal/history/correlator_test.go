// SPDX-License-Identifier: Apache-2.0

package history

import (
	"context"
	"testing"
	"time"

	"github.com/thepixelabs/kubecat/internal/storage"
)

// setupCorrelator opens an in-memory storage DB and returns a Correlator
// using the default rules.
func setupCorrelator(t *testing.T) (*Correlator, *storage.EventRepository, *storage.DB) {
	t.Helper()
	db, err := storage.OpenPath(":memory:")
	if err != nil {
		t.Fatalf("storage.OpenPath: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	c := NewCorrelator(db)
	return c, storage.NewEventRepository(db), db
}

// saveEvent persists an event and returns the populated row.
func saveEvent(t *testing.T, repo *storage.EventRepository, e storage.StoredEvent) storage.StoredEvent {
	t.Helper()
	if e.LastSeen.IsZero() {
		e.LastSeen = time.Now().UTC()
	}
	if e.FirstSeen.IsZero() {
		e.FirstSeen = e.LastSeen
	}
	if e.Count == 0 {
		e.Count = 1
	}
	if err := repo.Save(context.Background(), &e); err != nil {
		t.Fatalf("repo.Save: %v", err)
	}
	return e
}

func TestCorrelator_DefaultRules_NotEmpty(t *testing.T) {
	if len(DefaultCorrelationRules) == 0 {
		t.Fatal("DefaultCorrelationRules must contain rules")
	}
}

func TestCorrelator_MatchesKindReason(t *testing.T) {
	c := NewCorrelator(nil) // nil db is fine; we don't hit it.

	tests := []struct {
		name        string
		eventKind   string
		eventReason string
		ruleKind    string
		ruleReason  string
		want        bool
	}{
		{"empty_rule_matches_any", "Pod", "Foo", "", "", true},
		{"case_insensitive_match", "POD", "SCAling", "pod", "scaling", true},
		{"kind_mismatch", "Service", "", "Pod", "", false},
		{"reason_mismatch", "Pod", "Other", "Pod", "Scaling", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := storage.StoredEvent{Kind: tt.eventKind, Reason: tt.eventReason}
			if got := c.matchesKindReason(e, tt.ruleKind, tt.ruleReason); got != tt.want {
				t.Errorf("matchesKindReason(%+v, %q, %q) = %v, want %v",
					e, tt.ruleKind, tt.ruleReason, got, tt.want)
			}
		})
	}
}

func TestCorrelator_CalculateConfidence_BoostedWithinTimeWindow(t *testing.T) {
	c := NewCorrelator(nil)
	now := time.Now().UTC()
	rule := CorrelationRule{TimeWindow: 10 * time.Minute, Confidence: 0.7}

	source := storage.StoredEvent{Namespace: "ns", Type: "Normal", LastSeen: now}
	// Target very close in time and same namespace — should boost.
	closeTarget := storage.StoredEvent{Namespace: "ns", Type: "Normal", LastSeen: now.Add(1 * time.Minute)}
	farTarget := storage.StoredEvent{Namespace: "other", Type: "Normal", LastSeen: now.Add(9 * time.Minute)}

	closeConf := c.calculateConfidence(source, closeTarget, rule)
	farConf := c.calculateConfidence(source, farTarget, rule)

	if closeConf <= farConf {
		t.Errorf("close-in-time+same-ns confidence (%.2f) must exceed far/diff-ns (%.2f)", closeConf, farConf)
	}
	if closeConf > 1.0 {
		t.Errorf("confidence must be capped at 1.0, got %.2f", closeConf)
	}
}

func TestCorrelator_CalculateConfidence_WarningBoost(t *testing.T) {
	// Holding time proximity and namespace constant, a Warning-typed event
	// should confidence-boost relative to an otherwise-identical Normal event.
	c := NewCorrelator(nil)
	rule := CorrelationRule{TimeWindow: 5 * time.Minute, Confidence: 0.6}
	now := time.Now().UTC()

	source := storage.StoredEvent{Namespace: "ns", Type: "Normal", LastSeen: now}
	normalTarget := storage.StoredEvent{Namespace: "ns", Type: "Normal", LastSeen: now.Add(time.Minute)}
	warnTarget := storage.StoredEvent{Namespace: "ns", Type: "Warning", LastSeen: now.Add(time.Minute)}

	normalConf := c.calculateConfidence(source, normalTarget, rule)
	warnConf := c.calculateConfidence(source, warnTarget, rule)
	if warnConf <= normalConf {
		t.Errorf("warning-target confidence (%.2f) must exceed normal-target (%.2f)", warnConf, normalConf)
	}
}

func TestCorrelator_CorrelateEvent_LinksWithinTimeWindow(t *testing.T) {
	c, repo, _ := setupCorrelator(t)

	// Seed a pod event as potential target.
	target := saveEvent(t, repo, storage.StoredEvent{
		Cluster:   "c",
		Namespace: "ns",
		Kind:      "Pod",
		Name:      "p",
		Type:      "Warning",
		Reason:    "CrashLoop",
	})

	// Source event: a Deployment scaling event — rule source for pods.
	source := saveEvent(t, repo, storage.StoredEvent{
		Cluster:   "c",
		Namespace: "ns",
		Kind:      "Deployment",
		Name:      "app",
		Type:      "Normal",
		Reason:    "ScalingReplicaSet",
		LastSeen:  target.LastSeen.Add(-30 * time.Second),
	})

	corrs, err := c.CorrelateEvent(context.Background(), source)
	if err != nil {
		t.Fatalf("CorrelateEvent: %v", err)
	}
	if len(corrs) == 0 {
		t.Fatal("expected at least one correlation between Deployment scaling and Pod events")
	}
	// All correlations must point back to our target.
	for _, corr := range corrs {
		if corr.SourceEventID != source.ID {
			t.Errorf("correlation source = %d, want %d", corr.SourceEventID, source.ID)
		}
		if corr.TargetEventID != target.ID {
			t.Errorf("correlation target = %d, want %d", corr.TargetEventID, target.ID)
		}
		if corr.Confidence <= 0 || corr.Confidence > 1 {
			t.Errorf("confidence out of range: %.2f", corr.Confidence)
		}
	}
}

func TestCorrelator_CorrelateEvent_SelfNotCorrelated(t *testing.T) {
	c, repo, _ := setupCorrelator(t)

	// A Pod event that matches the target side of the "replicaset-to-pod" rule.
	// We seed ourselves as both the "source" (a ReplicaSet) and a Pod — the code
	// should exclude the source ID from match results.
	source := saveEvent(t, repo, storage.StoredEvent{
		Cluster: "c", Namespace: "ns", Kind: "ReplicaSet", Name: "r", Type: "Normal",
	})
	corrs, err := c.CorrelateEvent(context.Background(), source)
	if err != nil {
		t.Fatalf("CorrelateEvent: %v", err)
	}
	for _, corr := range corrs {
		if corr.TargetEventID == source.ID {
			t.Errorf("source event must not correlate with itself: %+v", corr)
		}
	}
}

func TestCorrelator_AddRule_AppendsToExisting(t *testing.T) {
	c := NewCorrelator(nil)
	before := len(c.rules)
	c.AddRule(CorrelationRule{Name: "custom"})
	if len(c.rules) != before+1 {
		t.Errorf("AddRule did not append: %d → %d", before, len(c.rules))
	}
}

func TestCorrelator_SetRules_ReplacesAll(t *testing.T) {
	c := NewCorrelator(nil)
	c.SetRules([]CorrelationRule{{Name: "only"}})
	if len(c.rules) != 1 || c.rules[0].Name != "only" {
		t.Errorf("SetRules did not replace: %+v", c.rules)
	}
}
