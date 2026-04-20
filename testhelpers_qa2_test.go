// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// isolateConfigDir redirects KUBECAT_CONFIG_DIR to a fresh empty directory for
// the duration of the test. When config.Load() is called it will fall through
// to Default() because no config.yaml exists there. This guarantees tests
// don't pick up stray settings (including readOnly) from the host machine.
func isolateConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev, had := os.LookupEnv("KUBECAT_CONFIG_DIR")
	if err := os.Setenv("KUBECAT_CONFIG_DIR", dir); err != nil {
		t.Fatalf("setenv KUBECAT_CONFIG_DIR: %v", err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("KUBECAT_CONFIG_DIR", prev)
		} else {
			_ = os.Unsetenv("KUBECAT_CONFIG_DIR")
		}
	})
	return dir
}

// withReadOnlyConfig writes a minimal config.yaml that toggles readOnly and
// points the config loader at it for the duration of the test.
func withReadOnlyConfig(t *testing.T, readOnly bool) {
	t.Helper()
	dir := isolateConfigDir(t)
	content := "kubecat:\n  readOnly: false\n"
	if readOnly {
		content = "kubecat:\n  readOnly: true\n"
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
}
