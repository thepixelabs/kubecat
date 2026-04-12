// SPDX-License-Identifier: Apache-2.0

// Package audit provides structured JSON audit logging for security-sensitive
// operations. Logs are written to ~/.local/state/kubecat/audit.log with
// automatic 50 MiB rotation and a 90-day retention purge at startup.
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/thepixelabs/kubecat/internal/config"
)

const (
	maxLogSize    = 50 * 1024 * 1024 // 50 MiB
	retentionDays = 90
	rotatedSuffix = ".1"
)

// EventType classifies an audit event.
type EventType string

const (
	EventAIQuery          EventType = "ai_query"
	EventSecretAccess     EventType = "secret_access"
	EventResourceDeletion EventType = "resource_deletion"
	EventCommandExecution EventType = "command_execution"
	EventProviderConfig   EventType = "provider_config_change"
	EventTerminalSession  EventType = "terminal_session"
)

// Entry is a single structured audit log entry.
type Entry struct {
	Timestamp  time.Time         `json:"timestamp"`
	EventType  EventType         `json:"eventType"`
	User       string            `json:"user,omitempty"`
	Cluster    string            `json:"cluster,omitempty"`
	Namespace  string            `json:"namespace,omitempty"`
	Resource   string            `json:"resource,omitempty"`
	Name       string            `json:"name,omitempty"`
	Provider   string            `json:"provider,omitempty"`
	PromptHash string            `json:"promptHash,omitempty"` // SHA-256 of prompt/command
	Meta       map[string]string `json:"meta,omitempty"`
}

// Logger is the package-level audit logger singleton.
type Logger struct {
	mu   sync.Mutex
	f    *os.File
	path string
}

var (
	once   sync.Once
	global *Logger
)

// Init initializes the global audit logger. Safe to call multiple times;
// only the first call has effect. Call from main before any other service.
func Init() error {
	var initErr error
	once.Do(func() {
		stateDir := config.StateDir()
		if err := os.MkdirAll(stateDir, 0700); err != nil {
			initErr = fmt.Errorf("audit: failed to create state dir: %w", err)
			return
		}

		logPath := filepath.Join(stateDir, "audit.log")
		l := &Logger{path: logPath}

		if err := l.purgeOldEntries(); err != nil {
			// Non-fatal: log to slog but keep going.
			slog.Warn("audit: retention purge failed", slog.Any("error", err))
		}

		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			initErr = fmt.Errorf("audit: failed to open log file: %w", err)
			return
		}

		l.f = f
		global = l
	})
	return initErr
}

// Shutdown flushes and closes the audit log. Call from app shutdown.
func Shutdown() {
	if global == nil {
		return
	}
	global.mu.Lock()
	defer global.mu.Unlock()
	if global.f != nil {
		_ = global.f.Sync()
		_ = global.f.Close()
		global.f = nil
	}
}

// Log writes an audit entry. If the global logger was not initialized (e.g.
// in tests) it silently discards the entry rather than panicking.
func Log(entry Entry) {
	if global == nil {
		return
	}
	entry.Timestamp = time.Now().UTC()
	global.write(entry)
}

// LogAIQuery logs an AI prompt query (hashes the prompt for privacy).
func LogAIQuery(provider, cluster, namespace, prompt string) {
	Log(Entry{
		EventType:  EventAIQuery,
		Cluster:    cluster,
		Namespace:  namespace,
		Provider:   provider,
		PromptHash: hashString(prompt),
	})
}

// LogSecretAccess logs access to a Kubernetes secret.
func LogSecretAccess(cluster, namespace, name string) {
	Log(Entry{
		EventType: EventSecretAccess,
		Cluster:   cluster,
		Namespace: namespace,
		Resource:  "secrets",
		Name:      name,
	})
}

// LogResourceDeletion logs a resource deletion.
func LogResourceDeletion(cluster, namespace, kind, name string) {
	Log(Entry{
		EventType: EventResourceDeletion,
		Cluster:   cluster,
		Namespace: namespace,
		Resource:  kind,
		Name:      name,
	})
}

// LogCommandExecution logs a command execution attempt.
// command is hashed — never stored in plain text.
// binary is the allowlisted binary name (no arguments).
// outcome is "allowed" or "denied".
func LogCommandExecution(cluster, command, binary, outcome string) {
	Log(Entry{
		EventType:  EventCommandExecution,
		Cluster:    cluster,
		PromptHash: hashString(command),
		Meta: map[string]string{
			"binary":  binary,
			"outcome": outcome,
		},
	})
}

// LogProviderConfig logs when a provider's configuration changes.
func LogProviderConfig(provider string) {
	Log(Entry{
		EventType: EventProviderConfig,
		Provider:  provider,
	})
}

// LogTerminalSession logs the start or end of a terminal session.
func LogTerminalSession(sessionID, action string) {
	Log(Entry{
		EventType: EventTerminalSession,
		Meta:      map[string]string{"sessionId": sessionID, "action": action},
	})
}

// write serializes and appends an entry to the log file, rotating if needed.
func (l *Logger) write(entry Entry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	data, err := json.Marshal(entry)
	if err != nil {
		slog.Warn("audit: failed to marshal entry", slog.Any("error", err))
		return
	}
	data = append(data, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.f == nil {
		return
	}

	// Check size and rotate if needed.
	if info, err := l.f.Stat(); err == nil && info.Size() >= maxLogSize {
		l.rotate()
	}

	if _, err := l.f.Write(data); err != nil {
		slog.Warn("audit: failed to write entry", slog.Any("error", err))
	}
}

// rotate closes the current log, renames it to audit.log.1, opens a new one.
func (l *Logger) rotate() {
	_ = l.f.Sync()
	_ = l.f.Close()
	l.f = nil

	rotated := l.path + rotatedSuffix
	// Best-effort: ignore rename errors.
	_ = os.Rename(l.path, rotated)

	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		slog.Warn("audit: failed to open new log file after rotation", slog.Any("error", err))
		return
	}
	l.f = f
}

// purgeOldEntries removes log entries older than retentionDays by rewriting
// the log file with only the entries that are still within the retention window.
func (l *Logger) purgeOldEntries() error {
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)

	data, err := os.ReadFile(l.path)
	if os.IsNotExist(err) {
		return nil // Nothing to purge yet.
	}
	if err != nil {
		return err
	}

	// Filter lines.
	var kept []byte
	start := 0
	for i, b := range data {
		if b != '\n' {
			continue
		}
		line := data[start:i]
		start = i + 1
		if len(line) == 0 {
			continue
		}

		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			// Keep malformed lines to avoid losing data unexpectedly.
			kept = append(kept, line...)
			kept = append(kept, '\n')
			continue
		}
		if e.Timestamp.After(cutoff) {
			kept = append(kept, line...)
			kept = append(kept, '\n')
		}
	}

	if len(kept) == len(data) {
		return nil // Nothing changed.
	}

	tmp := l.path + ".tmp"
	if err := os.WriteFile(tmp, kept, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, l.path)
}

// hashString returns the hex-encoded SHA-256 of s.
func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
