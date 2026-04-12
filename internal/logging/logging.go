// SPDX-License-Identifier: Apache-2.0

// Package logging configures structured logging for Kubecat using log/slog.
//
// Logs are written to a rotating file at ~/.local/state/kubecat/kubecat.log.
// The log level is configurable via the kubecat config file (logLevel: "info").
//
// Security rules enforced here:
//   - Never log API keys, tokens, or secrets.
//   - Never log full resource YAML at any level.
//   - Sensitive fields must be redacted before calling any slog function.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// rotatingWriter is a simple file writer that reopens the log file on first
// write and rotates (truncates) when the file exceeds maxBytes.
// For production use a proper rotation library is preferred, but lumberjack
// would add a dependency — this covers the basic "don't grow unbounded" need
// without external deps.
type rotatingWriter struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	f        *os.File
}

func newRotatingWriter(path string, maxBytes int64) (*rotatingWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("logging: failed to create log directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("logging: failed to open log file: %w", err)
	}
	return &rotatingWriter{path: path, maxBytes: maxBytes, f: f}, nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check file size and rotate if needed.
	if info, err := w.f.Stat(); err == nil && info.Size() >= w.maxBytes {
		_ = w.f.Close()
		// Rename existing file to .1 (simple single-backup rotation).
		_ = os.Rename(w.path, w.path+".1")
		f, err := os.OpenFile(w.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return 0, fmt.Errorf("logging: failed to rotate log file: %w", err)
		}
		w.f = f
	}

	return w.f.Write(p)
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}

// maxLogFileBytes is the rotation threshold (10 MiB).
const maxLogFileBytes = 10 * 1024 * 1024

// global holds the active log writer so it can be closed on shutdown.
var global struct {
	mu     sync.Mutex
	writer io.Closer
}

// Setup initializes the global slog default logger.
//
// Parameters:
//   - logPath: absolute path to the log file (created if absent).
//   - levelStr: "debug", "info", "warn", or "error". Defaults to "info".
//
// The handler writes newline-delimited JSON so logs are grep/jq-friendly.
// Returns a closer that flushes and closes the underlying file; callers
// should defer it from main().
func Setup(logPath, levelStr string) (io.Closer, error) {
	level, err := parseLevel(levelStr)
	if err != nil {
		level = slog.LevelInfo
	}

	writer, err := newRotatingWriter(logPath, maxLogFileBytes)
	if err != nil {
		// Fall back to stderr so we at least get logs somewhere.
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		})))
		return nil, err
	}

	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug, // source file only in debug mode
	})
	slog.SetDefault(slog.New(handler))

	global.mu.Lock()
	global.writer = writer
	global.mu.Unlock()

	slog.Info("logging initialized",
		slog.String("path", logPath),
		slog.String("level", levelStr),
	)

	return writer, nil
}

// parseLevel converts a human-readable level string to slog.Level.
func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level %q", s)
	}
}

// WithCluster returns a logger pre-populated with the cluster name field.
// Use this for all operations scoped to a specific cluster.
func WithCluster(clusterName string) *slog.Logger {
	return slog.With(slog.String("cluster", clusterName))
}

// WithOperation returns a logger pre-populated with cluster, namespace, and
// operation fields. Use this inside individual Wails method handlers.
func WithOperation(clusterName, namespace, operation string) *slog.Logger {
	return slog.With(
		slog.String("cluster", clusterName),
		slog.String("namespace", namespace),
		slog.String("operation", operation),
	)
}
