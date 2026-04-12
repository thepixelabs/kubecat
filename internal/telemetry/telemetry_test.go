package telemetry

import (
	"testing"
)

func TestTrack_DisabledByDefault_BufferEmpty(t *testing.T) {
	tel := New()
	tel.Track("app:opened", nil)
	if got := tel.BufferedCount(); got != 0 {
		t.Errorf("expected 0 buffered events when disabled, got %d", got)
	}
}

func TestTrack_Enabled_BuffersEvent(t *testing.T) {
	tel := New()
	tel.SetEnabled(true)
	tel.Track("app:opened", map[string]string{"version": "1.0"})
	if got := tel.BufferedCount(); got != 1 {
		t.Errorf("expected 1 buffered event, got %d", got)
	}
}

func TestSetEnabled_False_ClearsBuffer(t *testing.T) {
	tel := New()
	tel.SetEnabled(true)
	for i := 0; i < 5; i++ {
		tel.Track("app:opened", nil)
	}
	if tel.BufferedCount() != 5 {
		t.Fatal("setup: expected 5 events")
	}
	tel.SetEnabled(false)
	if got := tel.BufferedCount(); got != 0 {
		t.Errorf("opt-out should clear buffer, got %d events", got)
	}
}

func TestTrack_MaxBufferSize_OldestDropped(t *testing.T) {
	tel := New()
	tel.SetEnabled(true)
	for i := 0; i < maxBufferSize+10; i++ {
		tel.Track("app:opened", nil)
	}
	if got := tel.BufferedCount(); got > maxBufferSize {
		t.Errorf("buffer exceeded max size: got %d, want <= %d", got, maxBufferSize)
	}
}

func TestSetAnonymousID_AttachedToEvents(t *testing.T) {
	tel := New()
	tel.SetEnabled(true)
	tel.SetAnonymousID("test-anon-id")
	tel.Track("app:opened", nil)

	tel.mu.Lock()
	defer tel.mu.Unlock()
	if len(tel.buffer) == 0 {
		t.Fatal("expected at least one event")
	}
	if got := tel.buffer[0].AnonymousID; got != "test-anon-id" {
		t.Errorf("expected anonymousID %q, got %q", "test-anon-id", got)
	}
}

func TestTrack_Disabled_NoEventsBuffered(t *testing.T) {
	tel := New()
	tel.SetEnabled(true)
	tel.Track("app:opened", nil)
	tel.SetEnabled(false)
	tel.Track("app:opened", nil) // should be dropped
	if got := tel.BufferedCount(); got != 0 {
		t.Errorf("expected 0 after disable, got %d", got)
	}
}
