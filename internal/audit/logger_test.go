package audit

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// openTestLogger creates a Logger writing to a temp file, returning it and a
// cleanup function that closes the file.
func openTestLogger(t *testing.T) (*Logger, string) {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/audit.log"
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatalf("openTestLogger: %v", err)
	}
	l := &Logger{path: path, f: f}
	t.Cleanup(func() {
		_ = f.Sync()
		_ = f.Close()
	})
	return l, path
}

func readEntries(t *testing.T, path string) []Entry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readEntries: %v", err)
	}
	var entries []Entry
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("unmarshal entry %q: %v", line, err)
		}
		entries = append(entries, e)
	}
	return entries
}

func TestLogger_write_SecretAccess(t *testing.T) {
	l, path := openTestLogger(t)

	// Inject entry using write directly (bypasses global).
	l.write(Entry{
		EventType: EventSecretAccess,
		Cluster:   "prod",
		Namespace: "kube-system",
		Resource:  "secrets",
		Name:      "my-tls-cert",
	})

	entries := readEntries(t, path)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry got %d", len(entries))
	}
	e := entries[0]
	if e.EventType != EventSecretAccess {
		t.Errorf("EventType: got %q want %q", e.EventType, EventSecretAccess)
	}
	if e.Cluster != "prod" {
		t.Errorf("Cluster: got %q want %q", e.Cluster, "prod")
	}
	if e.Name != "my-tls-cert" {
		t.Errorf("Name: got %q want %q", e.Name, "my-tls-cert")
	}
	if e.Timestamp.IsZero() {
		t.Error("Timestamp must be set")
	}
}

func TestLogger_write_NoSensitiveData(t *testing.T) {
	l, path := openTestLogger(t)

	sensitiveCommand := "kubectl exec -it mypod -- cat /etc/secrets/token"
	sensitivePrompt := "show me secret token=sk-12345abcdef"

	l.write(Entry{
		EventType:  EventCommandExecution,
		PromptHash: hashString(sensitiveCommand),
		Meta:       map[string]string{"binary": "kubectl", "outcome": "allowed"},
	})
	l.write(Entry{
		EventType:  EventAIQuery,
		Provider:   "anthropic",
		PromptHash: hashString(sensitivePrompt),
	})

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(raw)

	if strings.Contains(content, "sk-12345abcdef") {
		t.Error("sensitive prompt text must not appear in audit log")
	}
	if strings.Contains(content, "cat /etc/secrets/token") {
		t.Error("sensitive command text must not appear in audit log")
	}
	// Provider name and binary are safe metadata, must appear.
	if !strings.Contains(content, "anthropic") {
		t.Error("provider name should appear in audit log")
	}
	if !strings.Contains(content, "kubectl") {
		t.Error("binary name should appear in audit log")
	}
}

func TestLogger_write_TimestampSet(t *testing.T) {
	l, path := openTestLogger(t)

	before := time.Now().UTC().Add(-time.Second)
	l.write(Entry{EventType: EventTerminalSession, Meta: map[string]string{"sessionId": "s1", "action": "start"}})
	after := time.Now().UTC().Add(time.Second)

	entries := readEntries(t, path)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry got %d", len(entries))
	}
	ts := entries[0].Timestamp
	if ts.Before(before) || ts.After(after) {
		t.Errorf("Timestamp %v outside expected range [%v, %v]", ts, before, after)
	}
}

func TestHashString_Deterministic(t *testing.T) {
	h1 := hashString("hello")
	h2 := hashString("hello")
	if h1 != h2 {
		t.Error("hashString must be deterministic")
	}
	h3 := hashString("world")
	if h1 == h3 {
		t.Error("different inputs must produce different hashes")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char SHA-256 hex, got %d", len(h1))
	}
}

func TestPurgeOldEntries_RemovesExpired(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/audit.log"

	fresh := Entry{
		Timestamp: time.Now().UTC(),
		EventType: EventSecretAccess,
		Name:      "fresh-secret",
	}
	expired := Entry{
		Timestamp: time.Now().UTC().AddDate(0, 0, -100),
		EventType: EventSecretAccess,
		Name:      "old-secret",
	}

	// Write both entries.
	f, _ := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	for _, e := range []Entry{expired, fresh} {
		b, _ := json.Marshal(e)
		_, _ = f.Write(append(b, '\n'))
	}
	_ = f.Close()

	l := &Logger{path: path}
	if err := l.purgeOldEntries(); err != nil {
		t.Fatalf("purgeOldEntries: %v", err)
	}

	entries := readEntries(t, path)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after purge, got %d", len(entries))
	}
	if entries[0].Name != "fresh-secret" {
		t.Errorf("expected fresh-secret to survive purge, got %q", entries[0].Name)
	}
}

func TestLog_NilGlobal_NoPanic(t *testing.T) {
	// Ensure Log is a no-op when the logger has not been initialized.
	prev := global
	global = nil
	t.Cleanup(func() { global = prev })

	// None of these should panic.
	Log(Entry{EventType: EventSecretAccess})
	LogAIQuery("openai", "c", "ns", "query")
	LogSecretAccess("c", "ns", "name")
	LogResourceDeletion("c", "ns", "Pod", "mypod")
	LogCommandExecution("c", "kubectl get pods", "kubectl", "allowed")
	LogTerminalSession("s1", "start")
	LogProviderConfig("openai")
}
