// SPDX-License-Identifier: Apache-2.0

package client

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// writeKubeconfig writes a kubeconfig YAML body to a temp dir and redirects
// KUBECONFIG to it for the duration of the test.
func writeKubeconfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	t.Setenv("KUBECONFIG", path)
	return path
}

const multiContextKubeconfig = `
apiVersion: v1
kind: Config
clusters:
- name: cluster-a
  cluster:
    server: https://api.a.example.com:6443
- name: cluster-b
  cluster:
    server: https://api.b.example.com:6443
contexts:
- name: ctx-a
  context:
    cluster: cluster-a
    user: user-a
    namespace: team-a
- name: ctx-b
  context:
    cluster: cluster-b
    user: user-b
users:
- name: user-a
  user:
    token: abc
- name: user-b
  user:
    token: def
current-context: ctx-a
`

// ---------------------------------------------------------------------------
// NewKubeConfigLoader / Contexts / CurrentContext
// ---------------------------------------------------------------------------

// TestNewKubeConfigLoader_MultiContext verifies that every context in a
// well-formed kubeconfig is surfaced and the current-context is identified.
func TestNewKubeConfigLoader_MultiContext(t *testing.T) {
	writeKubeconfig(t, multiContextKubeconfig)

	loader, err := NewKubeConfigLoader()
	if err != nil {
		t.Fatalf("NewKubeConfigLoader: %v", err)
	}

	got := loader.Contexts()
	sort.Strings(got)
	want := []string{"ctx-a", "ctx-b"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Contexts() = %v, want %v", got, want)
	}

	if cur := loader.CurrentContext(); cur != "ctx-a" {
		t.Errorf("CurrentContext = %q, want ctx-a", cur)
	}
}

// TestNewKubeConfigLoader_BrokenYAML_ReturnsError ensures a malformed
// kubeconfig surfaces a clean error rather than panicking.
func TestNewKubeConfigLoader_BrokenYAML_ReturnsError(t *testing.T) {
	writeKubeconfig(t, "this: is: not: valid: yaml:\n  [broken")

	if _, err := NewKubeConfigLoader(); err == nil {
		t.Fatal("expected error for broken kubeconfig, got nil")
	}
}

// TestNewKubeConfigLoader_MissingFile_ReturnsError pins that a non-existent
// KUBECONFIG path returns an error from clientcmd rather than panicking.
func TestNewKubeConfigLoader_MissingFile_ReturnsError(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "does-not-exist"))

	if _, err := NewKubeConfigLoader(); err == nil {
		t.Fatal("expected error for missing kubeconfig file, got nil")
	}
}

// ---------------------------------------------------------------------------
// ContextInfo
// ---------------------------------------------------------------------------

// TestContextInfo_FoundContext returns cluster + server + namespace info.
func TestContextInfo_FoundContext(t *testing.T) {
	writeKubeconfig(t, multiContextKubeconfig)
	loader, err := NewKubeConfigLoader()
	if err != nil {
		t.Fatalf("NewKubeConfigLoader: %v", err)
	}

	info, err := loader.ContextInfo("ctx-a")
	if err != nil {
		t.Fatalf("ContextInfo(ctx-a): %v", err)
	}
	if info.Cluster != "cluster-a" {
		t.Errorf("Cluster = %q, want cluster-a", info.Cluster)
	}
	if info.Server != "https://api.a.example.com:6443" {
		t.Errorf("Server = %q", info.Server)
	}
	if info.Namespace != "team-a" {
		t.Errorf("Namespace = %q, want team-a", info.Namespace)
	}
	if info.User != "user-a" {
		t.Errorf("User = %q, want user-a", info.User)
	}
}

// TestContextInfo_MissingContext_ReturnsErr pins the ErrContextNotFound
// contract.
func TestContextInfo_MissingContext_ReturnsErr(t *testing.T) {
	writeKubeconfig(t, multiContextKubeconfig)
	loader, err := NewKubeConfigLoader()
	if err != nil {
		t.Fatalf("NewKubeConfigLoader: %v", err)
	}

	_, err = loader.ContextInfo("ctx-ghost")
	if !errors.Is(err, ErrContextNotFound) {
		t.Errorf("expected ErrContextNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Reload
// ---------------------------------------------------------------------------

// TestReload_PicksUpNewContexts writes the initial kubeconfig, loads it,
// rewrites it with an additional context, and confirms Reload surfaces the
// new entry. This guards the kubeconfig-watch refresh path.
func TestReload_PicksUpNewContexts(t *testing.T) {
	path := writeKubeconfig(t, multiContextKubeconfig)
	loader, err := NewKubeConfigLoader()
	if err != nil {
		t.Fatalf("NewKubeConfigLoader: %v", err)
	}
	if got := len(loader.Contexts()); got != 2 {
		t.Fatalf("initial Contexts len = %d, want 2", got)
	}

	expanded := `
apiVersion: v1
kind: Config
clusters:
- name: cluster-a
  cluster:
    server: https://api.a.example.com:6443
- name: cluster-b
  cluster:
    server: https://api.b.example.com:6443
contexts:
- name: ctx-a
  context:
    cluster: cluster-a
    user: user-a
    namespace: team-a
- name: ctx-b
  context:
    cluster: cluster-b
    user: user-b
- name: ctx-c
  context:
    cluster: cluster-a
    user: user-a
users:
- name: user-a
  user:
    token: abc
- name: user-b
  user:
    token: def
current-context: ctx-a
`
	if err := os.WriteFile(path, []byte(expanded), 0600); err != nil {
		t.Fatalf("rewrite kubeconfig: %v", err)
	}

	if err := loader.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	got := loader.Contexts()
	sort.Strings(got)
	if len(got) != 3 {
		t.Errorf("after Reload Contexts() = %v, want 3 entries", got)
	}
}

// TestReload_BrokenFile_PreservesPrevious pins the safety property: if the
// kubeconfig is rewritten to something unparseable, Reload returns an error
// and the previously-loaded state is retained.
func TestReload_BrokenFile_PreservesPrevious(t *testing.T) {
	path := writeKubeconfig(t, multiContextKubeconfig)
	loader, err := NewKubeConfigLoader()
	if err != nil {
		t.Fatalf("NewKubeConfigLoader: %v", err)
	}

	if err := os.WriteFile(path, []byte("this is not yaml:[["), 0600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if err := loader.Reload(); err == nil {
		t.Fatal("expected Reload to return an error for bad YAML, got nil")
	}
	if got := len(loader.Contexts()); got != 2 {
		t.Errorf("after failed reload, Contexts len = %d, want 2 (previous state)", got)
	}
}

// ---------------------------------------------------------------------------
// kubeConfigPath resolution
// ---------------------------------------------------------------------------

// TestKubeConfigPath_SelectsFirstOfMulti confirms that when KUBECONFIG
// contains multiple paths separated by the OS path list separator, only the
// first is used.
func TestKubeConfigPath_SelectsFirstOfMulti(t *testing.T) {
	// Write two files and point KUBECONFIG at both.
	p1 := writeKubeconfig(t, multiContextKubeconfig)
	// Overwrite KUBECONFIG to "p1:p2" (unix) or "p1;p2" (windows).
	multi := p1 + string(filepath.ListSeparator) + filepath.Join(t.TempDir(), "other")
	t.Setenv("KUBECONFIG", multi)

	loader, err := NewKubeConfigLoader()
	if err != nil {
		t.Fatalf("NewKubeConfigLoader: %v", err)
	}
	if got := len(loader.Contexts()); got != 2 {
		t.Errorf("Contexts len = %d, want 2 (from first path)", got)
	}
}

// TestKubeConfigPath_EnvUnset_FallsBackToHome pins the documented fallback:
// if KUBECONFIG is unset, the loader reads from ~/.kube/config. We don't want
// the test to depend on the user's real home directory, so we redirect HOME
// to a temp dir that is empty.
func TestKubeConfigPath_EnvUnset_FallsBackToHome(t *testing.T) {
	t.Setenv("KUBECONFIG", "")
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// No ~/.kube/config exists, so the loader must return an error.
	if _, err := NewKubeConfigLoader(); err == nil {
		t.Fatal("expected error when ~/.kube/config does not exist, got nil")
	}
}
