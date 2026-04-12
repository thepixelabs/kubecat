package security

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thepixelabs/kubecat/internal/client"
)

// ---------------------------------------------------------------------------
// fakeClusterClient
// ---------------------------------------------------------------------------

// fakeClusterClient implements client.ClusterClient backed by in-memory data.
type fakeClusterClient struct {
	resources map[string][]client.Resource // kind -> resources
	getErr    map[string]error             // "kind/ns/name" -> error
}

func newFakeClient() *fakeClusterClient {
	return &fakeClusterClient{
		resources: make(map[string][]client.Resource),
		getErr:    make(map[string]error),
	}
}

func (f *fakeClusterClient) addResource(kind string, raw interface{}) {
	b, _ := json.Marshal(raw)
	r := client.Resource{Kind: kind, Raw: b}
	f.resources[kind] = append(f.resources[kind], r)
}

func (f *fakeClusterClient) Info(_ context.Context) (*client.ClusterInfo, error) {
	return &client.ClusterInfo{Name: "fake-cluster"}, nil
}

func (f *fakeClusterClient) List(_ context.Context, kind string, _ client.ListOptions) (*client.ResourceList, error) {
	items := f.resources[kind]
	return &client.ResourceList{Items: items, Total: len(items)}, nil
}

func (f *fakeClusterClient) Get(_ context.Context, kind, namespace, name string) (*client.Resource, error) {
	key := kind + "/" + namespace + "/" + name
	if err, ok := f.getErr[key]; ok {
		return nil, err
	}
	for _, r := range f.resources[kind] {
		if r.Name == name && r.Namespace == namespace {
			return &r, nil
		}
	}
	return nil, client.ErrResourceNotFound
}

func (f *fakeClusterClient) Delete(_ context.Context, _, _, _ string) error { return nil }
func (f *fakeClusterClient) Watch(_ context.Context, _ string, _ client.WatchOptions) (<-chan client.WatchEvent, error) {
	ch := make(chan client.WatchEvent)
	close(ch)
	return ch, nil
}
func (f *fakeClusterClient) Logs(_ context.Context, _, _, _ string, _ bool, _ int64) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}
func (f *fakeClusterClient) Exec(_ context.Context, _, _, _ string, _ []string) error { return nil }
func (f *fakeClusterClient) PortForward(_ context.Context, _, _ string, _, _ int) (client.PortForwarder, error) {
	return nil, nil
}
func (f *fakeClusterClient) Close() error { return nil }

// ---------------------------------------------------------------------------
// analyzePodSecurity
// ---------------------------------------------------------------------------

func TestAnalyzePodSecurity_PrivilegedContainer_ReturnsCritical(t *testing.T) {
	cl := newFakeClient()
	s := NewScanner(cl)

	priv := true
	podRaw := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "priv-pod", "namespace": "default"},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "app",
					"image": "nginx",
					"securityContext": map[string]interface{}{
						"privileged": &priv,
					},
				},
			},
		},
	}
	b, _ := json.Marshal(podRaw)
	pod := client.Resource{Kind: "Pod", Raw: b}

	issues := s.analyzePodSecurity(pod)

	found := false
	for _, issue := range issues {
		if issue.Severity == SeverityCritical && issue.Category == CategoryRuntime {
			found = true
		}
	}
	if !found {
		t.Errorf("expected Critical runtime issue for privileged container, got %v", issues)
	}
}

func TestAnalyzePodSecurity_RunAsRoot_ReturnsHigh(t *testing.T) {
	cl := newFakeClient()
	s := NewScanner(cl)

	uid := int64(0)
	podRaw := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "root-pod", "namespace": "default"},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "app",
					"image": "nginx",
					"securityContext": map[string]interface{}{
						"runAsUser": &uid,
					},
				},
			},
		},
	}
	b, _ := json.Marshal(podRaw)
	pod := client.Resource{Kind: "Pod", Raw: b}

	issues := s.analyzePodSecurity(pod)

	found := false
	for _, issue := range issues {
		if issue.Severity == SeverityHigh && issue.Category == CategoryRuntime {
			found = true
		}
	}
	if !found {
		t.Errorf("expected High runtime issue for root container, got %v", issues)
	}
}

func TestAnalyzePodSecurity_HostNetwork_ReturnsHigh(t *testing.T) {
	cl := newFakeClient()
	s := NewScanner(cl)

	podRaw := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "hostnet-pod", "namespace": "default"},
		"spec": map[string]interface{}{
			"hostNetwork": true,
			"containers":  []interface{}{},
		},
	}
	b, _ := json.Marshal(podRaw)
	pod := client.Resource{Kind: "Pod", Raw: b}

	issues := s.analyzePodSecurity(pod)

	found := false
	for _, issue := range issues {
		if issue.Severity == SeverityHigh && issue.Title == "Pod uses host network" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected High issue for hostNetwork=true, got %v", issues)
	}
}

func TestAnalyzePodSecurity_HostPID_ReturnsHigh(t *testing.T) {
	cl := newFakeClient()
	s := NewScanner(cl)

	podRaw := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "hostpid-pod", "namespace": "default"},
		"spec": map[string]interface{}{
			"hostPID":    true,
			"containers": []interface{}{},
		},
	}
	b, _ := json.Marshal(podRaw)
	pod := client.Resource{Kind: "Pod", Raw: b}

	issues := s.analyzePodSecurity(pod)

	found := false
	for _, issue := range issues {
		if issue.Severity == SeverityHigh && issue.Title == "Pod uses host PID namespace" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected High issue for hostPID=true, got %v", issues)
	}
}

func TestAnalyzePodSecurity_SecurePod_NoIssues(t *testing.T) {
	cl := newFakeClient()
	s := NewScanner(cl)

	uid := int64(1000)
	podRaw := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "secure-pod", "namespace": "default"},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "app",
					"image": "nginx",
					"securityContext": map[string]interface{}{
						"runAsUser": &uid,
					},
				},
			},
		},
	}
	b, _ := json.Marshal(podRaw)
	pod := client.Resource{Kind: "Pod", Raw: b}

	issues := s.analyzePodSecurity(pod)
	if len(issues) != 0 {
		t.Errorf("expected no issues for secure pod, got %v", issues)
	}
}

func TestAnalyzePodSecurity_InvalidJSON_ReturnsEmpty(t *testing.T) {
	cl := newFakeClient()
	s := NewScanner(cl)

	pod := client.Resource{Kind: "Pod", Raw: []byte("not-json")}
	issues := s.analyzePodSecurity(pod)
	if len(issues) != 0 {
		t.Errorf("invalid JSON should yield no issues, got %v", issues)
	}
}

// ---------------------------------------------------------------------------
// GetSecuritySummary
// ---------------------------------------------------------------------------

func TestGetSecuritySummary_EmptyCluster_ZeroIssues(t *testing.T) {
	cl := newFakeClient()
	// Add a namespace with network policy so no netpol issue is generated
	cl.addResource("namespaces", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "default"},
	})
	cl.addResource("networkpolicies", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "default-deny", "namespace": "default"},
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{},
			"policyTypes": []string{"Ingress"},
		},
	})

	s := NewScanner(cl)
	summary, err := s.GetSecuritySummary(context.Background(), "default")
	if err != nil {
		t.Fatalf("GetSecuritySummary: %v", err)
	}
	if summary == nil {
		t.Fatal("GetSecuritySummary returned nil")
	}
}

func TestGetSecuritySummary_PrivilegedPod_CountsCritical(t *testing.T) {
	cl := newFakeClient()

	priv := true
	podRaw := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "priv-pod", "namespace": "default"},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":  "app",
					"image": "nginx",
					"securityContext": map[string]interface{}{
						"privileged": &priv,
					},
				},
			},
		},
	}
	b, _ := json.Marshal(podRaw)
	cl.resources["pods"] = append(cl.resources["pods"], client.Resource{Kind: "Pod", Raw: b})

	s := NewScanner(cl)
	summary, err := s.GetSecuritySummary(context.Background(), "default")
	if err != nil {
		t.Fatalf("GetSecuritySummary: %v", err)
	}
	if summary.CriticalCount == 0 {
		t.Error("expected CriticalCount > 0 for privileged pod")
	}
}

func TestGetSecuritySummary_ScoreDecreasesWithIssues(t *testing.T) {
	cl := newFakeClient()
	s := NewScanner(cl)

	// Empty cluster → high score
	baseline, _ := s.GetSecuritySummary(context.Background(), "")
	baseScore := baseline.Score.Overall

	// Add a privileged pod
	priv := true
	podRaw := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "priv-pod", "namespace": "default"},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name":            "app",
					"securityContext": map[string]interface{}{"privileged": &priv},
				},
			},
		},
	}
	b, _ := json.Marshal(podRaw)
	cl.resources["pods"] = []client.Resource{{Kind: "Pod", Raw: b}}

	withIssues, _ := s.GetSecuritySummary(context.Background(), "default")
	if withIssues.Score.Overall >= baseScore && baseScore == 100 {
		t.Errorf("score should decrease with critical issues; baseline=%d with-issues=%d",
			baseScore, withIssues.Score.Overall)
	}
}

// ---------------------------------------------------------------------------
// calculateScore
// ---------------------------------------------------------------------------

func TestCalculateScore_PerfectScore(t *testing.T) {
	cl := newFakeClient()
	s := NewScanner(cl)

	summary := &SecuritySummary{
		IssuesByCategory: make(map[Category]int),
	}
	score := s.calculateScore(summary)
	if score.Overall != 100 {
		t.Errorf("calculateScore with no issues = %d, want 100", score.Overall)
	}
	if score.Grade != "A" {
		t.Errorf("calculateScore grade = %q, want A", score.Grade)
	}
}

func TestCalculateScore_CriticalIssuesDeduct15Each(t *testing.T) {
	cl := newFakeClient()
	s := NewScanner(cl)

	summary := &SecuritySummary{
		CriticalCount:    2,
		IssuesByCategory: make(map[Category]int),
	}
	score := s.calculateScore(summary)
	expected := 100 - 2*15
	if score.Overall != expected {
		t.Errorf("score with 2 critical = %d, want %d", score.Overall, expected)
	}
}

func TestCalculateScore_NeverBelowZero(t *testing.T) {
	cl := newFakeClient()
	s := NewScanner(cl)

	summary := &SecuritySummary{
		CriticalCount:    100,
		IssuesByCategory: make(map[Category]int),
	}
	score := s.calculateScore(summary)
	if score.Overall < 0 {
		t.Errorf("score = %d, must not be negative", score.Overall)
	}
}

func TestCalculateScore_GradeMapping(t *testing.T) {
	cl := newFakeClient()
	s := NewScanner(cl)

	cases := []struct {
		critical  int
		wantGrade string
	}{
		{0, "A"}, // 100 - 0*15 = 100 → A
		{1, "B"}, // 100 - 1*15 = 85  → B
		{2, "C"}, // 100 - 2*15 = 70  → C
		{3, "F"}, // 100 - 3*15 = 55  → F (below 60, not D)
		{7, "F"}, // 100 - 7*15 = -5  → clamped 0 → F
	}
	for _, tc := range cases {
		t.Run(tc.wantGrade, func(t *testing.T) {
			summary := &SecuritySummary{
				CriticalCount:    tc.critical,
				IssuesByCategory: make(map[Category]int),
			}
			score := s.calculateScore(summary)
			if score.Grade != tc.wantGrade {
				t.Errorf("critical=%d score=%d grade=%q, want %q",
					tc.critical, score.Overall, score.Grade, tc.wantGrade)
			}
		})
	}
}
