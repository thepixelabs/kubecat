// Package events provides an injectable Wails event emitter.
package events

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Emitter wraps runtime.EventsEmit and is safe to call before SetContext.
// Before a context is set, all Emit calls are silently dropped.
type Emitter struct {
	mu  sync.RWMutex
	ctx context.Context
}

// SetContext stores the Wails runtime context required for event emission.
// Call this from App.startup.
func (e *Emitter) SetContext(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ctx = ctx
}

// Emit sends a named event with optional data to the frontend.
// It is a no-op if SetContext has not yet been called.
func (e *Emitter) Emit(event string, data ...interface{}) {
	e.mu.RLock()
	ctx := e.ctx
	e.mu.RUnlock()

	if ctx == nil {
		return
	}
	runtime.EventsEmit(ctx, event, data...)
}

// EmitterInterface describes what callers need from an emitter, useful for
// testing without a live Wails runtime.
type EmitterInterface interface {
	Emit(event string, data ...interface{})
	SetContext(ctx context.Context)
}
