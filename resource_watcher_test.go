// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
)

// ---------------------------------------------------------------------------
// StartResourceWatch — lifecycle and wiring
// ---------------------------------------------------------------------------

// TestStartResourceWatch_NoActiveCluster_ReturnsError confirms that without an
// active cluster the watcher refuses to start rather than panicking or leaking
// a goroutine.
func TestStartResourceWatch_NoActiveCluster_ReturnsError(t *testing.T) {
	a := newAppWithFakes(nil)

	err := a.StartResourceWatch("pods", "default")
	if err == nil {
		t.Fatal("expected error when no active cluster, got nil")
	}
}

// TestStartResourceWatch_RegistersCancelFunc verifies that a successful start
// populates a.watchers with a cancel func keyed by resource kind.
func TestStartResourceWatch_RegistersCancelFunc(t *testing.T) {
	cl := newFakeClusterClient()
	// Use a long-lived watch channel so the goroutine stays parked.
	cl.watchFn = func(ctx context.Context, _ string, _ client.WatchOptions) (<-chan client.WatchEvent, error) {
		ch := make(chan client.WatchEvent)
		go func() {
			<-ctx.Done()
			close(ch)
		}()
		return ch, nil
	}
	a := newAppWithFakes(cl)
	t.Cleanup(a.StopAllResourceWatchers)

	if err := a.StartResourceWatch("pods", "default"); err != nil {
		t.Fatalf("StartResourceWatch: %v", err)
	}

	a.mu.Lock()
	_, ok := a.watchers["pods"]
	a.mu.Unlock()
	if !ok {
		t.Fatal("expected a.watchers[pods] to be registered after StartResourceWatch")
	}
}

// TestStartResourceWatch_StartTwiceReplacesPrevious verifies the documented
// "start replaces any existing watcher" contract. The second start must not
// leak the first goroutine's cancel into the registry.
func TestStartResourceWatch_StartTwiceReplacesPrevious(t *testing.T) {
	starts := 0
	var mu sync.Mutex
	cl := newFakeClusterClient()
	cl.watchFn = func(ctx context.Context, _ string, _ client.WatchOptions) (<-chan client.WatchEvent, error) {
		mu.Lock()
		starts++
		mu.Unlock()
		ch := make(chan client.WatchEvent)
		go func() {
			<-ctx.Done()
			close(ch)
		}()
		return ch, nil
	}
	a := newAppWithFakes(cl)
	t.Cleanup(a.StopAllResourceWatchers)

	if err := a.StartResourceWatch("pods", "default"); err != nil {
		t.Fatalf("first start: %v", err)
	}
	if err := a.StartResourceWatch("pods", "default"); err != nil {
		t.Fatalf("second start: %v", err)
	}

	mu.Lock()
	count := starts
	mu.Unlock()
	if count != 2 {
		t.Errorf("expected 2 watchFn invocations, got %d", count)
	}

	a.mu.Lock()
	n := len(a.watchers)
	a.mu.Unlock()
	if n != 1 {
		t.Errorf("expected exactly one registered watcher after restart, got %d", n)
	}
}

// TestStartResourceWatch_WatchErrorClearsRegistration confirms that if the
// underlying Watch call fails, StartResourceWatch does NOT leave a cancel
// func stranded in a.watchers (would mask future starts).
func TestStartResourceWatch_WatchErrorClearsRegistration(t *testing.T) {
	cl := newFakeClusterClient()
	cl.watchFn = func(_ context.Context, _ string, _ client.WatchOptions) (<-chan client.WatchEvent, error) {
		return nil, fmt.Errorf("apiserver unavailable")
	}
	a := newAppWithFakes(cl)

	err := a.StartResourceWatch("pods", "default")
	if err == nil {
		t.Fatal("expected error from failing watch, got nil")
	}

	a.mu.Lock()
	_, registered := a.watchers["pods"]
	a.mu.Unlock()
	if registered {
		t.Error("failed watch must not leave a watcher registered")
	}
}

// ---------------------------------------------------------------------------
// StopResourceWatch / StopAllResourceWatchers
// ---------------------------------------------------------------------------

// TestStopResourceWatch_CancelsGoroutine verifies that StopResourceWatch both
// deregisters the cancel func and unblocks the goroutine so it returns.
func TestStopResourceWatch_CancelsGoroutine(t *testing.T) {
	done := make(chan struct{})
	cl := newFakeClusterClient()
	cl.watchFn = func(ctx context.Context, _ string, _ client.WatchOptions) (<-chan client.WatchEvent, error) {
		ch := make(chan client.WatchEvent)
		go func() {
			<-ctx.Done()
			close(ch)
			close(done)
		}()
		return ch, nil
	}
	a := newAppWithFakes(cl)

	if err := a.StartResourceWatch("pods", "default"); err != nil {
		t.Fatalf("StartResourceWatch: %v", err)
	}

	a.StopResourceWatch("pods")

	select {
	case <-done:
		// watch goroutine observed ctx cancellation
	case <-time.After(2 * time.Second):
		t.Fatal("watch goroutine did not terminate after StopResourceWatch")
	}

	// Registry must be clean.
	a.mu.Lock()
	_, stillRegistered := a.watchers["pods"]
	a.mu.Unlock()
	if stillRegistered {
		t.Error("StopResourceWatch should delete watchers[kind]")
	}
}

// TestStopResourceWatch_UnknownKind_NoPanic confirms stopping a never-started
// kind is a no-op.
func TestStopResourceWatch_UnknownKind_NoPanic(t *testing.T) {
	a := newAppWithFakes(nil)
	a.StopResourceWatch("ghosts") // must not panic
}

// TestStopAllResourceWatchers_CancelsEveryKind verifies bulk shutdown.
func TestStopAllResourceWatchers_CancelsEveryKind(t *testing.T) {
	cl := newFakeClusterClient()
	cl.watchFn = func(ctx context.Context, _ string, _ client.WatchOptions) (<-chan client.WatchEvent, error) {
		ch := make(chan client.WatchEvent)
		go func() {
			<-ctx.Done()
			close(ch)
		}()
		return ch, nil
	}
	a := newAppWithFakes(cl)

	for _, kind := range []string{"pods", "services", "deployments"} {
		if err := a.StartResourceWatch(kind, ""); err != nil {
			t.Fatalf("StartResourceWatch(%s): %v", kind, err)
		}
	}

	a.StopAllResourceWatchers()

	a.mu.Lock()
	n := len(a.watchers)
	a.mu.Unlock()
	if n != 0 {
		t.Errorf("after StopAllResourceWatchers, expected 0 registered watchers, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// Event emission — seam: nil ctx silently drops emits
// ---------------------------------------------------------------------------

// TestWatchGoroutine_ConsumesEvents_NoPanicWithNilCtx verifies our testability
// seam: with a.ctx==nil, events arriving on the watch channel are processed
// and dropped without calling runtime.EventsEmit (which would log.Fatal on
// nil ctx). This pins the guard behavior we added in resource_watcher.go.
func TestWatchGoroutine_ConsumesEvents_NoPanicWithNilCtx(t *testing.T) {
	consumed := make(chan struct{}, 2)
	ch := make(chan client.WatchEvent, 2)
	ch <- client.WatchEvent{Type: "ADDED", Resource: client.Resource{Kind: "Pod", Name: "p1"}}
	ch <- client.WatchEvent{Type: "DELETED", Resource: client.Resource{Kind: "Pod", Name: "p1"}}
	close(ch)

	cl := newFakeClusterClient()
	cl.watchFn = func(_ context.Context, _ string, _ client.WatchOptions) (<-chan client.WatchEvent, error) {
		// Wrap to observe drain.
		out := make(chan client.WatchEvent)
		go func() {
			defer close(out)
			for ev := range ch {
				out <- ev
				consumed <- struct{}{}
			}
		}()
		return out, nil
	}
	a := newAppWithFakes(cl)
	a.ctx = nil // <- force the nil-ctx path

	if err := a.StartResourceWatch("pods", ""); err != nil {
		t.Fatalf("StartResourceWatch: %v", err)
	}

	for i := 0; i < 2; i++ {
		select {
		case <-consumed:
		case <-time.After(time.Second):
			t.Fatalf("event %d was not consumed", i+1)
		}
	}
}

// ---------------------------------------------------------------------------
// Goroutine leak safety
// ---------------------------------------------------------------------------

// TestResourceWatchers_NoGoroutineLeakAfterStopAll starts several watchers,
// stops them all, and asserts the goroutine count returns to baseline (within
// a small tolerance for runtime noise).
func TestResourceWatchers_NoGoroutineLeakAfterStopAll(t *testing.T) {
	// Let the runtime settle.
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	base := runtime.NumGoroutine()

	stopped := make(chan struct{}, 10)
	cl := newFakeClusterClient()
	cl.watchFn = func(ctx context.Context, _ string, _ client.WatchOptions) (<-chan client.WatchEvent, error) {
		ch := make(chan client.WatchEvent)
		go func() {
			<-ctx.Done()
			close(ch)
			stopped <- struct{}{}
		}()
		return ch, nil
	}
	a := newAppWithFakes(cl)

	const n = 8
	for i := 0; i < n; i++ {
		if err := a.StartResourceWatch(fmt.Sprintf("kind-%d", i), ""); err != nil {
			t.Fatalf("StartResourceWatch: %v", err)
		}
	}

	a.StopAllResourceWatchers()

	// Drain stop signals
	for i := 0; i < n; i++ {
		select {
		case <-stopped:
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d/%d watch goroutines stopped", i, n)
		}
	}

	// Allow the main consumer goroutines to unwind.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= base+2 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Errorf("goroutine count after StopAllResourceWatchers = %d, baseline %d (leak suspected)",
		runtime.NumGoroutine(), base)
}
