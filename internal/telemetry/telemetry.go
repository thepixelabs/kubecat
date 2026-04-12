// Package telemetry provides opt-in anonymous usage telemetry.
// All data is buffered in memory and flushed periodically.
// No data leaves the process unless the user explicitly enables telemetry.
package telemetry

import (
	"log/slog"
	"sync"
	"time"
)

const (
	maxBufferSize = 100
	flushInterval = 5 * time.Minute
)

// Event is a single telemetry event.
type Event struct {
	Name        string            `json:"name"`
	AnonymousID string            `json:"anonymousId,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
	Properties  map[string]string `json:"properties,omitempty"`
}

// Telemetry manages the in-memory event buffer.
type Telemetry struct {
	mu          sync.Mutex
	enabled     bool
	anonymousID string
	buffer      []Event

	cancel func()
	done   chan struct{}
}

// New creates a Telemetry instance. Call Start to begin periodic flushing.
func New() *Telemetry {
	return &Telemetry{
		buffer: make([]Event, 0, maxBufferSize),
		done:   make(chan struct{}),
	}
}

// SetEnabled enables or disables telemetry at runtime.
func (t *Telemetry) SetEnabled(enabled bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.enabled = enabled
	if !enabled {
		// Discard buffered events immediately on opt-out.
		t.buffer = t.buffer[:0]
	}
}

// SetAnonymousID sets the anonymous identifier for this installation.
func (t *Telemetry) SetAnonymousID(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.anonymousID = id
}

// Track records a named event. It is a no-op if telemetry is disabled.
func (t *Telemetry) Track(name string, properties map[string]string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.enabled {
		return
	}

	if len(t.buffer) >= maxBufferSize {
		// Drop oldest event to make room (ring-buffer behavior).
		t.buffer = t.buffer[1:]
	}

	t.buffer = append(t.buffer, Event{
		Name:        name,
		AnonymousID: t.anonymousID,
		Timestamp:   time.Now().UTC(),
		Properties:  properties,
	})
}

// Start begins the periodic flush goroutine.
func (t *Telemetry) Start() {
	stopCh := make(chan struct{})
	done := make(chan struct{})
	t.done = done

	var once sync.Once
	t.cancel = func() {
		once.Do(func() { close(stopCh) })
	}

	go func() {
		defer close(done)
		ticker := time.NewTicker(flushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				t.flush()
			}
		}
	}()
}

// Stop halts flushing and discards remaining buffered events.
func (t *Telemetry) Stop() {
	if t.cancel != nil {
		t.cancel()
	}
	<-t.done
	t.mu.Lock()
	t.buffer = t.buffer[:0]
	t.mu.Unlock()
}

// flush drains the buffer. Currently a no-op stub — extend to POST to a
// telemetry endpoint if/when one is configured.
func (t *Telemetry) flush() {
	t.mu.Lock()
	if len(t.buffer) == 0 || !t.enabled {
		t.mu.Unlock()
		return
	}
	// Drain.
	events := make([]Event, len(t.buffer))
	copy(events, t.buffer)
	t.buffer = t.buffer[:0]
	t.mu.Unlock()

	// Silent failure — do not surface telemetry errors to the user.
	slog.Debug("telemetry: flushed events", slog.Int("count", len(events)))
}

// BufferedCount returns the number of events currently in the buffer.
// Useful for tests.
func (t *Telemetry) BufferedCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.buffer)
}
