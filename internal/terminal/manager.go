// SPDX-License-Identifier: Apache-2.0

package terminal

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/creack/pty"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// allowedShells is the set of shell base-names that may be launched via
// StartTerminal. Any command whose resolved basename is not in this set is
// rejected before a PTY is created.
var allowedShells = map[string]bool{
	"bash": true,
	"zsh":  true,
	"sh":   true,
}

// shellMetacharacters contains characters that have special meaning to a POSIX
// shell. If any arg supplied to StartTerminal contains one of these runes the
// request is rejected outright — they have no legitimate place in a plain
// shell invocation and are a reliable signal of injection attempts.
const shellMetacharacters = ";&|`$()<>\\!{}"

// validateCommand checks that the requested command is an approved shell.
// It resolves the binary via PATH so that callers cannot bypass the allowlist
// by supplying an absolute path to an unapproved binary.
// Returns the resolved absolute path on success, or an error on failure.
func validateCommand(command string) (string, error) {
	// Reject anything that looks like a path — approved shells must be
	// referenced by bare name only.
	if strings.ContainsRune(command, '/') || strings.ContainsRune(command, '\\') {
		return "", fmt.Errorf("command must be a bare name, not a path: %q", command)
	}

	if !allowedShells[command] {
		return "", fmt.Errorf("command %q is not in the approved shell allowlist (allowed: bash, zsh, sh)", command)
	}

	// Resolve to an absolute path so exec.Command cannot be tricked by a
	// malicious PATH entry after this point.
	resolved, err := exec.LookPath(command)
	if err != nil {
		return "", fmt.Errorf("shell %q not found on this system: %w", command, err)
	}

	// Double-check: the base name of the resolved path must still be in the
	// allowlist. This guards against a symlink (e.g. /usr/bin/sh -> /bin/dash)
	// where the base name itself changes.
	base := filepath.Base(resolved)
	if !allowedShells[base] {
		return "", fmt.Errorf("resolved shell binary %q (from %q) is not in the approved allowlist", base, command)
	}

	return resolved, nil
}

// validateArgs rejects any argument that contains shell metacharacters.
// Shell arguments for an interactive terminal invocation have no legitimate
// need for these characters, and their presence is a strong indicator of an
// injection attempt.
func validateArgs(args []string) error {
	for i, arg := range args {
		if idx := strings.IndexAny(arg, shellMetacharacters); idx != -1 {
			return fmt.Errorf("arg[%d] contains a disallowed shell metacharacter %q", i, string(arg[idx]))
		}
	}
	return nil
}

// Session represents an active PTY session
type Session struct {
	ID    string
	Cmd   *exec.Cmd
	Pty   *os.File
	Stdin io.Writer
}

// Manager handles terminal sessions
type Manager struct {
	ctx      context.Context
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewManager creates a new terminal manager
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// SetContext sets the Wails context
func (m *Manager) SetContext(ctx context.Context) {
	m.ctx = ctx
}

// Start launches a new terminal session.
// command must be one of the approved shells (bash, zsh, sh); any other value
// is rejected. args must not contain shell metacharacters.
func (m *Manager) Start(id string, command string, args ...string) error {
	// --- Security: validate command and args before acquiring the lock or
	// touching any session state. Fail fast and loudly. ---
	resolvedCmd, err := validateCommand(command)
	if err != nil {
		return fmt.Errorf("StartTerminal rejected: %w", err)
	}
	if err := validateArgs(args); err != nil {
		return fmt.Errorf("StartTerminal rejected: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[id]; exists {
		return fmt.Errorf("session %s already exists", id)
	}

	cmd := exec.Command(resolvedCmd, args...)

	// Set environment variables if needed
	cmd.Env = os.Environ()
	// Ensure we are in a reasonable directory
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("failed to start pty: %w", err)
	}

	session := &Session{
		ID:    id,
		Cmd:   cmd,
		Pty:   ptmx,
		Stdin: ptmx,
	}

	m.sessions[id] = session

	// Start reading from PTY
	go func() {
		defer func() {
			m.Close(id)
			runtime.EventsEmit(m.ctx, "terminal:closed:"+id)
		}()

		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				if err != io.EOF {
					fmt.Printf("Read error: %v\n", err)
				}
				break
			}
			if n > 0 {
				// Encode to Base64 to safely transmit binary data over JSON/Events
				data := base64.StdEncoding.EncodeToString(buf[:n])
				runtime.EventsEmit(m.ctx, "terminal:data:"+id, data)
			}
		}
	}()

	return nil
}

// Resize resizes the PTY
func (m *Manager) Resize(id string, rows, cols int) error {
	m.mu.RLock()
	session, exists := m.sessions[id]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found", id)
	}

	if err := pty.Setsize(session.Pty, &pty.Winsize{
		Rows: uint16(rows), //nolint:gosec
		Cols: uint16(cols), //nolint:gosec
	}); err != nil {
		return fmt.Errorf("failed to resize pty: %w", err)
	}

	return nil
}

// Write writes data to the PTY
func (m *Manager) Write(id string, data string) error {
	m.mu.RLock()
	session, exists := m.sessions[id]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found", id)
	}

	_, err := session.Stdin.Write([]byte(data))
	return err
}

// Close terminates a session
func (m *Manager) Close(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[id]
	if !exists {
		return nil
	}

	// Close PTY and kill command
	_ = session.Pty.Close()
	if session.Cmd.Process != nil {
		_ = session.Cmd.Process.Kill()
	}
	_ = session.Cmd.Wait()

	delete(m.sessions, id)
	return nil
}
