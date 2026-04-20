// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"context"
	"testing"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
)

// buildEvent constructs a raw k8s-event map with the common fields.
func buildEvent(involvedKind, involvedName, involvedNS, eventType, reason, msg string, ts time.Time) map[string]interface{} {
	return map[string]interface{}{
		"involvedObject": map[string]interface{}{
			"kind":      involvedKind,
			"name":      involvedName,
			"namespace": involvedNS,
		},
		"type":           eventType,
		"reason":         reason,
		"message":        msg,
		"count":          float64(1),
		"firstTimestamp": ts.Format(time.RFC3339),
		"lastTimestamp":  ts.Format(time.RFC3339),
	}
}

func TestGetRelatedEvents_ReturnsMatchingResourceOnly(t *testing.T) {
	cl := newFakeClient()
	now := time.Now().UTC().Truncate(time.Second)

	// Add two events — one for our pod, one for another pod.
	matching := buildEvent("Pod", "crashy", "default", "Warning", "BackOff", "crash", now)
	other := buildEvent("Pod", "other", "default", "Warning", "BackOff", "unrelated", now)
	m1 := cl.addResourceRaw("events", matching)
	m1.Namespace = "default"
	cl.resources["events"][0] = m1
	m2 := cl.addResourceRaw("events", other)
	m2.Namespace = "default"
	cl.resources["events"][1] = m2

	resource := client.Resource{Kind: "Pod", Name: "crashy", Namespace: "default"}
	events, err := GetRelatedEvents(context.Background(), cl, resource)
	if err != nil {
		t.Fatalf("GetRelatedEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 related event, got %d: %+v", len(events), events)
	}
	if events[0].Reason != "BackOff" || events[0].Message != "crash" {
		t.Errorf("unexpected event content: %+v", events[0])
	}
}

func TestGetRelatedEvents_SortedByLastTimestampDesc(t *testing.T) {
	cl := newFakeClient()
	t1 := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	t2 := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Second)

	e1 := buildEvent("Pod", "p", "ns", "Warning", "A", "first", t1)
	e2 := buildEvent("Pod", "p", "ns", "Warning", "B", "second", t2)

	cl.resources["events"] = []client.Resource{}
	r1 := cl.addResourceRaw("events", e1)
	r1.Namespace = "ns"
	cl.resources["events"][0] = r1
	r2 := cl.addResourceRaw("events", e2)
	r2.Namespace = "ns"
	cl.resources["events"][1] = r2

	resource := client.Resource{Kind: "Pod", Name: "p", Namespace: "ns"}
	events, _ := GetRelatedEvents(context.Background(), cl, resource)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if !events[0].LastTimestamp.After(events[1].LastTimestamp) {
		t.Errorf("events not sorted desc: got %v then %v", events[0].LastTimestamp, events[1].LastTimestamp)
	}
}

func TestGetRecentEvents_FiltersByTimeWindow(t *testing.T) {
	cl := newFakeClient()
	now := time.Now().UTC().Truncate(time.Second)

	old := buildEvent("Pod", "p", "ns", "Warning", "Old", "m", now.Add(-3*time.Hour))
	fresh := buildEvent("Pod", "p", "ns", "Warning", "Fresh", "m", now.Add(-10*time.Minute))
	cl.resources["events"] = []client.Resource{}
	cl.addResourceRaw("events", old)
	cl.addResourceRaw("events", fresh)

	events, err := GetRecentEvents(context.Background(), cl, "ns", 30*time.Minute)
	if err != nil {
		t.Fatalf("GetRecentEvents: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected only fresh event, got %d: %+v", len(events), events)
	}
	if len(events) > 0 && events[0].Reason != "Fresh" {
		t.Errorf("expected Fresh, got %q", events[0].Reason)
	}
}

func TestGetWarningEvents_FiltersByType(t *testing.T) {
	cl := newFakeClient()
	now := time.Now().UTC().Truncate(time.Second)

	cl.resources["events"] = []client.Resource{}
	cl.addResourceRaw("events", buildEvent("Pod", "p", "ns", "Normal", "Scheduled", "ok", now))
	cl.addResourceRaw("events", buildEvent("Pod", "p", "ns", "Warning", "BackOff", "bad", now))

	events, err := GetWarningEvents(context.Background(), cl, "ns")
	if err != nil {
		t.Fatalf("GetWarningEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(events))
	}
	if events[0].Type != "Warning" {
		t.Errorf("type = %q, want Warning", events[0].Type)
	}
}

func TestParseEvent_DefaultsCountToOne(t *testing.T) {
	// An event raw blob without a count field should default to 1.
	e := buildEvent("Pod", "p", "ns", "Warning", "R", "m", time.Now())
	delete(e, "count")
	cl := newFakeClient()
	r := cl.addResourceRaw("events", e)
	re, err := parseEvent(r)
	if err != nil {
		t.Fatalf("parseEvent: %v", err)
	}
	if re.Count != 1 {
		t.Errorf("default count = %d, want 1", re.Count)
	}
}

func TestIsEventRelated_InvalidJSON_ReturnsFalse(t *testing.T) {
	bad := client.Resource{Raw: []byte(`{not-json`)}
	if isEventRelated(bad, client.Resource{Name: "x", Namespace: "y"}) {
		t.Error("invalid JSON event should not match")
	}
}
