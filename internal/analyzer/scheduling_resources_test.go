// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// buildPendingPodWithResources constructs a Pending pod whose single container
// requests the given CPU/memory. Returns a raw-JSON map ready for addResourceRaw.
func buildPendingPodWithResources(cpu, mem string) map[string]interface{} {
	return buildPendingPod(func(p *corev1.Pod) {
		p.Spec.Containers = []corev1.Container{{
			Name: "app",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(cpu),
					corev1.ResourceMemory: resource.MustParse(mem),
				},
			},
		}}
	})
}

// nodeWithCapacity returns a node raw-map with the given allocatable cpu/mem.
func nodeWithCapacity(name, cpu, mem string) map[string]interface{} {
	return newNode(name,
		func(n map[string]interface{}) {
			n["status"].(map[string]interface{})["allocatable"] = map[string]interface{}{
				"cpu":    cpu,
				"memory": mem,
			}
		},
	)
}

// TestSchedulingAnalyzer_InsufficientResources_Critical exercises the path
// where a Pending pod requests more CPU+memory than any single node can
// satisfy. This is effectively the "Unschedulable due to capacity" case —
// pods stuck because the scheduler emits PodScheduled=False with a
// FailedScheduling reason.
func TestSchedulingAnalyzer_InsufficientResources_Critical(t *testing.T) {
	cl := newFakeClient()
	a := NewSchedulingAnalyzer()

	// Only one small node; pod wants more than it has.
	cl.addResourceRaw("nodes", nodeWithCapacity("small", "500m", "1Gi"))

	pod := buildPendingPodWithResources("2", "4Gi")
	r := cl.addResourceRaw("Pod", pod)

	issues, err := a.Analyze(context.Background(), cl, r)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	i := findIssue(issues, "scheduling.resources.insufficient")
	if i == nil {
		t.Fatalf("expected scheduling.resources.insufficient, got %+v", issues)
	}
	if i.Severity != SeverityCritical {
		t.Errorf("severity = %v, want Critical", i.Severity)
	}
	// Requested values must appear in details for debugging.
	if got, _ := i.Details["requested_cpu"].(int64); got != 2000 {
		t.Errorf("requested_cpu = %v, want 2000", i.Details["requested_cpu"])
	}
	// Max available must reflect the small node.
	if got, _ := i.Details["max_cpu"].(int64); got != 500 {
		t.Errorf("max_cpu = %v, want 500", i.Details["max_cpu"])
	}
}

// TestSchedulingAnalyzer_SufficientResources_NoIssue pins that a Pending pod
// whose requests fit on at least one node does NOT produce an
// insufficient-resources issue (the analyzer must not be trigger-happy).
func TestSchedulingAnalyzer_SufficientResources_NoIssue(t *testing.T) {
	cl := newFakeClient()
	a := NewSchedulingAnalyzer()

	cl.addResourceRaw("nodes", nodeWithCapacity("big", "8", "16Gi"))

	pod := buildPendingPodWithResources("500m", "1Gi")
	r := cl.addResourceRaw("Pod", pod)

	issues, _ := a.Analyze(context.Background(), cl, r)
	if hasIssueID(issues, "scheduling.resources.insufficient") {
		t.Errorf("fitting pod must not trigger insufficient resources, got %+v", issues)
	}
}

// TestSchedulingAnalyzer_NoResourceRequests_NoIssue pins the early-return
// path: containers without resource.requests should not produce an issue
// even when no nodes are available.
func TestSchedulingAnalyzer_NoResourceRequests_NoIssue(t *testing.T) {
	cl := newFakeClient()
	a := NewSchedulingAnalyzer()

	// No nodes at all — but pod requests nothing.
	pod := buildPendingPod(func(p *corev1.Pod) {
		p.Spec.Containers = []corev1.Container{{Name: "app"}}
	})
	r := cl.addResourceRaw("Pod", pod)

	issues, _ := a.Analyze(context.Background(), cl, r)
	if hasIssueID(issues, "scheduling.resources.insufficient") {
		t.Errorf("pod with no requests must not trigger insufficient, got %+v", issues)
	}
}

// TestSchedulingAnalyzer_NodeAffinity_NoMatch_Critical covers the node
// affinity path that the existing tests miss — required-affinity with no
// matching label on any node must emit scheduling.affinity.nomatch.
func TestSchedulingAnalyzer_NodeAffinity_NoMatch_Critical(t *testing.T) {
	cl := newFakeClient()
	a := NewSchedulingAnalyzer()

	cl.addResourceRaw("nodes", newNode("n1",
		func(n map[string]interface{}) {
			n["metadata"].(map[string]interface{})["labels"] = map[string]interface{}{"tier": "standard"}
		},
	))

	pod := buildPendingPod(func(p *corev1.Pod) {
		p.Spec.Affinity = &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key:      "tier",
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{"gpu"}, // no node has this label value
						}},
					}},
				},
			},
		}
	})
	r := cl.addResourceRaw("Pod", pod)

	issues, _ := a.Analyze(context.Background(), cl, r)
	i := findIssue(issues, "scheduling.affinity.nomatch")
	if i == nil {
		t.Fatalf("expected scheduling.affinity.nomatch, got %+v", issues)
	}
	if i.Severity != SeverityCritical {
		t.Errorf("severity = %v, want Critical", i.Severity)
	}
}
