// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
	"github.com/thepixelabs/kubecat/internal/core"
)

// ---------------------------------------------------------------------------
// StartLogStream — single-pod streaming contract
// ---------------------------------------------------------------------------

// TestStartLogStream_DelegatesToLogsAPI confirms that StartLogStream reaches
// the underlying cluster client with the expected (namespace, pod, container)
// triple and follow=true.
func TestStartLogStream_DelegatesToLogsAPI(t *testing.T) {
	var gotNS, gotPod, gotContainer string
	var gotFollow bool
	var gotTail int64

	cl := newFakeClusterClient()
	cl.logsFn = func(_ context.Context, ns, pod, container string, follow bool, tailLines int64) (<-chan string, error) {
		gotNS, gotPod, gotContainer = ns, pod, container
		gotFollow = follow
		gotTail = tailLines
		ch := make(chan string)
		close(ch)
		return ch, nil
	}
	a := newAppWithFakes(cl)

	if err := a.StartLogStream("kube-system", "coredns-xyz", "coredns", 50); err != nil {
		t.Fatalf("StartLogStream: %v", err)
	}
	if gotNS != "kube-system" || gotPod != "coredns-xyz" || gotContainer != "coredns" {
		t.Errorf("Logs called with ns=%q pod=%q container=%q, want kube-system/coredns-xyz/coredns",
			gotNS, gotPod, gotContainer)
	}
	if !gotFollow {
		t.Error("StartLogStream should pass follow=true")
	}
	if gotTail != 50 {
		t.Errorf("TailLines = %d, want 50", gotTail)
	}
}

// TestStartLogStream_NoActiveCluster_ReturnsError ensures a disconnected app
// fails cleanly rather than panicking.
func TestStartLogStream_NoActiveCluster_ReturnsError(t *testing.T) {
	a := newAppWithFakes(nil)
	if err := a.StartLogStream("default", "pod-x", "", 0); err == nil {
		t.Fatal("expected error when no active cluster, got nil")
	}
}

// TestStartLogStream_PropagatesAPIError surfaces errors returned by the
// underlying Logs() call to the caller.
func TestStartLogStream_PropagatesAPIError(t *testing.T) {
	cl := newFakeClusterClient()
	cl.logsFn = func(_ context.Context, _, _, _ string, _ bool, _ int64) (<-chan string, error) {
		return nil, fmt.Errorf("pod not ready")
	}
	a := newAppWithFakes(cl)
	err := a.StartLogStream("default", "p", "", 0)
	if err == nil || !strings.Contains(err.Error(), "pod not ready") {
		t.Fatalf("expected 'pod not ready' error, got %v", err)
	}
}

// TestStartLogStream_BuffersIncomingLines verifies that lines written to the
// underlying Logs channel are appended to the buffer reachable via
// GetBufferedLogs. Three lines fit under the LogService's 100-slot forwarding
// buffer, so StartLogStream (which discards the returned channel) can still
// observe them.
func TestStartLogStream_BuffersIncomingLines(t *testing.T) {
	ch := make(chan string, 3)
	ch <- "line-1"
	ch <- "line-2"
	ch <- "line-3"
	close(ch)

	cl := newFakeClusterClient()
	cl.logsFn = func(_ context.Context, _, _, _ string, _ bool, _ int64) (<-chan string, error) {
		return ch, nil
	}
	a := newAppWithFakes(cl)

	if err := a.StartLogStream("default", "p", "", 0); err != nil {
		t.Fatalf("StartLogStream: %v", err)
	}

	// The background goroutine reads asynchronously — give it a moment.
	if !waitFor(func() bool { return len(a.GetBufferedLogs()) == 3 }, time.Second) {
		t.Fatalf("expected 3 buffered lines, got %v", a.GetBufferedLogs())
	}

	lines := a.GetBufferedLogs()
	if lines[0] != "line-1" || lines[1] != "line-2" || lines[2] != "line-3" {
		t.Errorf("buffered lines = %v, want [line-1 line-2 line-3]", lines)
	}
}

// TestStartLogStream_BoundedBuffer_DropsOldest pins the 10k-line ring
// buffer contract: once the buffer exceeds 10000 lines, the oldest are
// dropped. We exercise LogService directly (draining the returned channel)
// because StartLogStream discards the channel and cannot consume more than
// its internal 100-slot pending capacity.
func TestStartLogStream_BoundedBuffer_DropsOldest(t *testing.T) {
	const total = 12000
	const limit = 10000

	ch := make(chan string, total)
	for i := 0; i < total; i++ {
		ch <- fmt.Sprintf("l-%d", i)
	}
	close(ch)

	cl := newFakeClusterClient()
	cl.logsFn = func(_ context.Context, _, _, _ string, _ bool, _ int64) (<-chan string, error) {
		return ch, nil
	}
	a := newAppWithFakes(cl)

	// Use the service directly so we can drain the outbound channel, which is
	// required for the ring-buffer trimming loop to keep making progress.
	out, err := a.nexus.Logs.StreamLogs(context.Background(), core.LogOptions{
		Namespace: "default", Pod: "p", Container: "", Follow: true,
	})
	if err != nil {
		t.Fatalf("StreamLogs: %v", err)
	}
	// Drain the outbound channel until it closes. Each receive unblocks the
	// append-and-forward goroutine in LogService.
	for range out {
	}

	// At this point the goroutine has appended all 12k lines and trimmed to 10k.
	lines := a.GetBufferedLogs()
	if len(lines) != limit {
		t.Fatalf("buffer not bounded: got %d lines, want %d", len(lines), limit)
	}
	if got, want := lines[0], fmt.Sprintf("l-%d", total-limit); got != want {
		t.Errorf("lines[0] = %q, want %q (buffer did not drop oldest)", got, want)
	}
	if got, want := lines[len(lines)-1], fmt.Sprintf("l-%d", total-1); got != want {
		t.Errorf("lines[last] = %q, want %q", got, want)
	}
}

// TestStopLogStream_CancelsActiveStream verifies that calling StopLogStream
// unblocks the background consumer even if the Logs channel never closes.
func TestStopLogStream_CancelsActiveStream(t *testing.T) {
	streamDone := make(chan struct{})

	cl := newFakeClusterClient()
	cl.logsFn = func(ctx context.Context, _, _, _ string, _ bool, _ int64) (<-chan string, error) {
		out := make(chan string)
		go func() {
			<-ctx.Done()
			close(out)
			close(streamDone)
		}()
		return out, nil
	}
	a := newAppWithFakes(cl)

	if err := a.StartLogStream("default", "p", "", 0); err != nil {
		t.Fatalf("StartLogStream: %v", err)
	}

	a.StopLogStream()

	select {
	case <-streamDone:
	case <-time.After(2 * time.Second):
		t.Fatal("StopLogStream did not cancel upstream Logs context")
	}
}

// ---------------------------------------------------------------------------
// SearchLogs
// ---------------------------------------------------------------------------

// TestSearchLogs_FindsSubstringMatches seeds the buffer with a handful of
// lines, then searches for a literal substring.
func TestSearchLogs_FindsSubstringMatches(t *testing.T) {
	ch := make(chan string, 4)
	ch <- "starting server"
	ch <- "ERROR: disk full"
	ch <- "handled request"
	ch <- "error: timeout"
	close(ch)

	cl := newFakeClusterClient()
	cl.logsFn = func(_ context.Context, _, _, _ string, _ bool, _ int64) (<-chan string, error) {
		return ch, nil
	}
	a := newAppWithFakes(cl)

	if err := a.StartLogStream("default", "p", "", 0); err != nil {
		t.Fatalf("StartLogStream: %v", err)
	}
	if !waitFor(func() bool { return len(a.GetBufferedLogs()) == 4 }, time.Second) {
		t.Fatalf("expected 4 buffered lines, got %d", len(a.GetBufferedLogs()))
	}

	matches := a.SearchLogs("error")
	if len(matches) != 2 {
		t.Errorf("expected 2 matches for case-insensitive 'error', got %d: %+v", len(matches), matches)
	}
}

// ---------------------------------------------------------------------------
// StartWorkloadLogStream — multi-pod fan-in
// ---------------------------------------------------------------------------

// TestStartWorkloadLogStream_FansOutAcrossPods seeds a deployment with two
// matching pods and verifies that logs from both are merged with per-pod
// metadata in the workload buffer.
func TestStartWorkloadLogStream_FansOutAcrossPods(t *testing.T) {
	cl := newFakeClusterClient()

	// Deployment with selector app=web
	cl.addResource("deployments", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "web", "namespace": "default"},
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{"app": "web"},
			},
		},
	})
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "web-1", "namespace": "default",
			"labels": map[string]interface{}{"app": "web"},
		},
	})
	cl.addResource("pods", map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "web-2", "namespace": "default",
			"labels": map[string]interface{}{"app": "web"},
		},
	})

	// Per-pod log channels
	var mu sync.Mutex
	perPodCh := map[string]chan string{
		"web-1": make(chan string, 2),
		"web-2": make(chan string, 2),
	}
	perPodCh["web-1"] <- "from-web-1-A"
	perPodCh["web-1"] <- "from-web-1-B"
	close(perPodCh["web-1"])
	perPodCh["web-2"] <- "from-web-2-A"
	close(perPodCh["web-2"])

	cl.logsFn = func(_ context.Context, _, pod, _ string, _ bool, _ int64) (<-chan string, error) {
		mu.Lock()
		defer mu.Unlock()
		ch, ok := perPodCh[pod]
		if !ok {
			closed := make(chan string)
			close(closed)
			return closed, nil
		}
		return ch, nil
	}

	a := newAppWithFakes(cl)

	if err := a.StartWorkloadLogStream("deployments", "default", "web", 0); err != nil {
		t.Fatalf("StartWorkloadLogStream: %v", err)
	}

	if !waitFor(func() bool { return len(a.GetBufferedWorkloadLogs()) == 3 }, 3*time.Second) {
		t.Fatalf("expected 3 workload lines, got %d: %+v",
			len(a.GetBufferedWorkloadLogs()), a.GetBufferedWorkloadLogs())
	}

	logs := a.GetBufferedWorkloadLogs()
	perPod := map[string]int{}
	colorByPod := map[string]int{}
	for _, l := range logs {
		perPod[l.Pod]++
		if prev, ok := colorByPod[l.Pod]; ok && prev != l.ColorIdx {
			t.Errorf("pod %q inconsistent ColorIdx: %d vs %d", l.Pod, prev, l.ColorIdx)
		}
		colorByPod[l.Pod] = l.ColorIdx
	}
	if perPod["web-1"] != 2 {
		t.Errorf("web-1 lines = %d, want 2", perPod["web-1"])
	}
	if perPod["web-2"] != 1 {
		t.Errorf("web-2 lines = %d, want 1", perPod["web-2"])
	}
	// Distinct ColorIdx per pod.
	if colorByPod["web-1"] == colorByPod["web-2"] {
		t.Errorf("web-1 and web-2 must have distinct ColorIdx, both = %d", colorByPod["web-1"])
	}
}

// TestStartWorkloadLogStream_MissingWorkload_ReturnsError surfaces the "no such
// deployment" case clearly.
func TestStartWorkloadLogStream_MissingWorkload_ReturnsError(t *testing.T) {
	cl := newFakeClusterClient()
	a := newAppWithFakes(cl)
	err := a.StartWorkloadLogStream("deployments", "default", "ghost", 0)
	if err == nil {
		t.Fatal("expected error for missing workload, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "deployments") {
		t.Errorf("error should mention kind, got: %v", err)
	}
}

// TestStartWorkloadLogStream_NoMatchingPods_ReturnsError pins that a deployment
// with zero matching pods produces a clean error rather than a silent stall.
func TestStartWorkloadLogStream_NoMatchingPods_ReturnsError(t *testing.T) {
	cl := newFakeClusterClient()
	cl.addResource("deployments", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "web", "namespace": "default"},
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{"app": "web"},
			},
		},
	})
	a := newAppWithFakes(cl)

	err := a.StartWorkloadLogStream("deployments", "default", "web", 0)
	if err == nil {
		t.Fatal("expected error when no pods match, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "no pods") {
		t.Errorf("error should mention missing pods, got: %v", err)
	}
}

// TestWorkloadLogStream_NoGoroutineLeakAfterStop starts a workload stream with
// two pods whose Logs calls block on ctx.Done, then asserts StopLogStream
// unwinds every per-pod goroutine.
func TestWorkloadLogStream_NoGoroutineLeakAfterStop(t *testing.T) {
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	base := runtime.NumGoroutine()

	cl := newFakeClusterClient()
	cl.addResource("deployments", map[string]interface{}{
		"metadata": map[string]interface{}{"name": "web", "namespace": "default"},
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{"app": "web"},
			},
		},
	})
	for i := 0; i < 3; i++ {
		cl.addResource("pods", map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": fmt.Sprintf("web-%d", i), "namespace": "default",
				"labels": map[string]interface{}{"app": "web"},
			},
		})
	}

	// Pre-Add the WaitGroup synchronously (before goroutines that might race
	// with Wait) so concurrent logsFn invocations only call Done().
	const nPods = 3
	var openWG sync.WaitGroup
	openWG.Add(nPods)
	cl.logsFn = func(ctx context.Context, _, _, _ string, _ bool, _ int64) (<-chan string, error) {
		ch := make(chan string)
		go func() {
			defer openWG.Done()
			<-ctx.Done()
			close(ch)
		}()
		return ch, nil
	}

	a := newAppWithFakes(cl)
	if err := a.StartWorkloadLogStream("deployments", "default", "web", 0); err != nil {
		t.Fatalf("StartWorkloadLogStream: %v", err)
	}

	a.StopLogStream()

	// Wait for every fake Logs goroutine to return.
	done := make(chan struct{})
	go func() {
		openWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("workload log goroutines did not exit after StopLogStream")
	}

	// Allow scheduler to reap.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= base+3 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Errorf("goroutine count after StopLogStream = %d, baseline %d (leak suspected)",
		runtime.NumGoroutine(), base)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// waitFor polls cond until it returns true or the timeout elapses.
func waitFor(cond func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

// Silence unused import if the test matrix shifts.
var _ = client.ErrNoActiveCluster
