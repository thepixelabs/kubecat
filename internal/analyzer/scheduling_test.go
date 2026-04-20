// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"context"
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// buildPendingPod builds a raw-JSON Pod in Pending phase.
func buildPendingPod(opts func(p *corev1.Pod)) map[string]interface{} {
	pod := corev1.Pod{}
	pod.Name = "pending"
	pod.Namespace = "default"
	pod.Status.Phase = corev1.PodPending
	if opts != nil {
		opts(&pod)
	}
	b, _ := json.Marshal(pod)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	return m
}

func TestSchedulingAnalyzer_Metadata(t *testing.T) {
	a := NewSchedulingAnalyzer()
	if a.Name() != "scheduling" {
		t.Errorf("Name() = %q, want scheduling", a.Name())
	}
	if a.Category() != CategoryScheduling {
		t.Errorf("Category() = %q", a.Category())
	}
}

func TestSchedulingAnalyzer_NonPendingPod_ReturnsNil(t *testing.T) {
	cl := newFakeClient()
	a := NewSchedulingAnalyzer()

	r := cl.addResourceRaw("Pod", newPod("running", "ns")) // Phase=Running
	issues, err := a.Analyze(context.Background(), cl, r)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if issues != nil {
		t.Errorf("running pod should have no scheduling issues, got %+v", issues)
	}
}

func TestSchedulingAnalyzer_NodeSelector_NoMatch_Critical(t *testing.T) {
	cl := newFakeClient()
	a := NewSchedulingAnalyzer()

	// Node with a different label than the selector — no match.
	cl.addResourceRaw("nodes", newNode("n1",
		func(n map[string]interface{}) {
			n["metadata"].(map[string]interface{})["labels"] = map[string]interface{}{"zone": "us-east-1a"}
		},
	))

	pod := buildPendingPod(func(p *corev1.Pod) {
		p.Spec.NodeSelector = map[string]string{"zone": "us-west-2"}
	})
	r := cl.addResourceRaw("Pod", pod)

	issues, err := a.Analyze(context.Background(), cl, r)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	i := findIssue(issues, "scheduling.nodeselector.nomatch")
	if i == nil {
		t.Fatalf("expected nodeselector issue, got %+v", issues)
	}
	if i.Severity != SeverityCritical {
		t.Errorf("severity = %v, want Critical", i.Severity)
	}
}

func TestSchedulingAnalyzer_NodeSelector_Match_NoIssue(t *testing.T) {
	cl := newFakeClient()
	a := NewSchedulingAnalyzer()

	cl.addResourceRaw("nodes", newNode("n1",
		func(n map[string]interface{}) {
			n["metadata"].(map[string]interface{})["labels"] = map[string]interface{}{"zone": "us-west-2"}
		},
	))

	pod := buildPendingPod(func(p *corev1.Pod) {
		p.Spec.NodeSelector = map[string]string{"zone": "us-west-2"}
	})
	r := cl.addResourceRaw("Pod", pod)

	issues, _ := a.Analyze(context.Background(), cl, r)
	if hasIssueID(issues, "scheduling.nodeselector.nomatch") {
		t.Errorf("matching selector should not produce issue, got %+v", issues)
	}
}

func TestSchedulingAnalyzer_UntoleratedTaints_Critical(t *testing.T) {
	cl := newFakeClient()
	a := NewSchedulingAnalyzer()

	// Every node tainted with something the pod won't tolerate.
	cl.addResourceRaw("nodes", newNode("gpu-node",
		func(n map[string]interface{}) {
			n["spec"].(map[string]interface{})["taints"] = []interface{}{
				map[string]interface{}{
					"key":    "gpu",
					"value":  "nvidia",
					"effect": "NoSchedule",
				},
			}
		},
	))

	pod := buildPendingPod(nil) // no tolerations
	r := cl.addResourceRaw("Pod", pod)

	issues, _ := a.Analyze(context.Background(), cl, r)
	i := findIssue(issues, "scheduling.tolerations.missing")
	if i == nil {
		t.Fatalf("expected tolerations issue, got %+v", issues)
	}
	if i.Severity != SeverityCritical {
		t.Errorf("severity = %v, want Critical", i.Severity)
	}
	if len(i.Fixes) == 0 {
		t.Error("tolerations issue should include a toleration-YAML fix")
	}
}

// ---- Helper-level tests (pure functions) ----------------------------------

func TestParseCPU(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"500m", 500},
		{"2", 2000},
		{"1", 1000},
	}
	for _, tt := range tests {
		if got := parseCPU(tt.in); got != tt.want {
			t.Errorf("parseCPU(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestParseMemory(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"1Ki", 1024},
		{"1Mi", 1024 * 1024},
		{"1Gi", 1024 * 1024 * 1024},
		{"1Ti", 1024 * 1024 * 1024 * 1024},
		{"123", 123},
	}
	for _, tt := range tests {
		if got := parseMemory(tt.in); got != tt.want {
			t.Errorf("parseMemory(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{512, "512"},
		{2048, "2Ki"},
		{5 * 1024 * 1024, "5Mi"},
		{3 * 1024 * 1024 * 1024, "3Gi"},
	}
	for _, tt := range tests {
		if got := formatBytes(tt.in); got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTolerationMatchesTaint(t *testing.T) {
	tests := []struct {
		name string
		tol  corev1.Toleration
		tnt  corev1.Taint
		want bool
	}{
		{
			name: "exists_with_empty_key_matches_all",
			tol:  corev1.Toleration{Operator: corev1.TolerationOpExists},
			tnt:  corev1.Taint{Key: "anything", Effect: corev1.TaintEffectNoSchedule},
			want: true,
		},
		{
			name: "equal_matches_same_key_value",
			tol:  corev1.Toleration{Key: "gpu", Operator: corev1.TolerationOpEqual, Value: "nvidia", Effect: corev1.TaintEffectNoSchedule},
			tnt:  corev1.Taint{Key: "gpu", Value: "nvidia", Effect: corev1.TaintEffectNoSchedule},
			want: true,
		},
		{
			name: "equal_mismatch_value",
			tol:  corev1.Toleration{Key: "gpu", Operator: corev1.TolerationOpEqual, Value: "amd"},
			tnt:  corev1.Taint{Key: "gpu", Value: "nvidia"},
			want: false,
		},
		{
			name: "different_key",
			tol:  corev1.Toleration{Key: "cpu", Operator: corev1.TolerationOpEqual, Value: "x"},
			tnt:  corev1.Taint{Key: "gpu", Value: "x"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tolerationMatchesTaint(tt.tol, tt.tnt); got != tt.want {
				t.Errorf("tolerationMatchesTaint = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchesLabels(t *testing.T) {
	if !matchesLabels(
		map[string]string{"a": "1", "b": "2"},
		map[string]string{"a": "1"},
	) {
		t.Error("subset selector must match")
	}
	if matchesLabels(
		map[string]string{"a": "1"},
		map[string]string{"a": "2"},
	) {
		t.Error("mismatched value must not match")
	}
	if matchesLabels(
		map[string]string{"a": "1"},
		map[string]string{"missing": "x"},
	) {
		t.Error("missing key must not match")
	}
}
