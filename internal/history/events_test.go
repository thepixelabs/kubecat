// SPDX-License-Identifier: Apache-2.0

package history

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
)

func TestParseK8sEvent_PopulatesAllFields(t *testing.T) {
	ts := time.Now().UTC().Truncate(time.Second)
	raw, _ := json.Marshal(map[string]interface{}{
		"involvedObject": map[string]interface{}{"kind": "Pod", "name": "p"},
		"type":           "Warning",
		"reason":         "BackOff",
		"message":        "crash",
		"count":          float64(3),
		"firstTimestamp": ts.Format(time.RFC3339),
		"lastTimestamp":  ts.Format(time.RFC3339),
		"source":         map[string]interface{}{"component": "kubelet", "host": "node-1"},
	})
	r := &client.Resource{Namespace: "ns", Raw: raw}

	ev, err := parseK8sEvent(r)
	if err != nil {
		t.Fatalf("parseK8sEvent: %v", err)
	}
	if ev.InvolvedObjectKind != "Pod" || ev.InvolvedObjectName != "p" {
		t.Errorf("involved obj = %s/%s", ev.InvolvedObjectKind, ev.InvolvedObjectName)
	}
	if ev.Type != "Warning" || ev.Reason != "BackOff" || ev.Message != "crash" {
		t.Errorf("type/reason/message mismatch: %+v", ev)
	}
	if ev.Count != 3 {
		t.Errorf("count = %d, want 3", ev.Count)
	}
	if !ev.FirstTimestamp.Equal(ts) || !ev.LastTimestamp.Equal(ts) {
		t.Errorf("timestamps not parsed: first=%v last=%v want=%v", ev.FirstTimestamp, ev.LastTimestamp, ts)
	}
	if ev.SourceComponent != "kubelet" || ev.SourceHost != "node-1" {
		t.Errorf("source mismatch: %+v", ev)
	}
	if ev.Namespace != "ns" {
		t.Errorf("namespace = %q, want ns", ev.Namespace)
	}
}

func TestParseK8sEvent_DefaultsLastTimestampWhenMissing(t *testing.T) {
	raw, _ := json.Marshal(map[string]interface{}{
		"involvedObject": map[string]interface{}{"kind": "Pod", "name": "p"},
		"type":           "Normal",
		"reason":         "R",
	})
	r := &client.Resource{Raw: raw}

	ev, err := parseK8sEvent(r)
	if err != nil {
		t.Fatalf("parseK8sEvent: %v", err)
	}
	if ev.LastTimestamp.IsZero() {
		t.Error("LastTimestamp must default to now when missing, got zero")
	}
}

func TestParseK8sEvent_InvalidJSON_ReturnsError(t *testing.T) {
	r := &client.Resource{Raw: []byte(`{not json`)}
	if _, err := parseK8sEvent(r); err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestDefaultEventCollectorConfig_ReasonableDefaults(t *testing.T) {
	cfg := DefaultEventCollectorConfig()
	if cfg.Retention <= 0 {
		t.Errorf("Retention = %v, want > 0", cfg.Retention)
	}
	if cfg.CleanupInterval <= 0 {
		t.Errorf("CleanupInterval = %v, want > 0", cfg.CleanupInterval)
	}
}
