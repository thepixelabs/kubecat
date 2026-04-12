// SPDX-License-Identifier: Apache-2.0

package client

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// EnsureCommonPathsForCLITools adds common CLI tool installation paths to PATH.
// This is necessary for GUI applications which don't inherit the shell's PATH.
func EnsureCommonPathsForCLITools() {
	currentPath := os.Getenv("PATH")

	var commonPaths []string

	if runtime.GOOS == "darwin" {
		// macOS common paths
		commonPaths = []string{
			"/usr/local/bin",
			"/opt/homebrew/bin",                 // Homebrew on Apple Silicon
			"/usr/local/opt/aws-cli/bin",        // AWS CLI installed via Homebrew
			"/usr/local/aws-cli/v2/current/bin", // AWS CLI v2
			filepath.Join(os.Getenv("HOME"), ".local/bin"),
			filepath.Join(os.Getenv("HOME"), "google-cloud-sdk/bin"), // gcloud SDK
			"/Applications/Docker.app/Contents/Resources/bin",        // Docker Desktop
		}
	} else if runtime.GOOS == "linux" {
		// Linux common paths
		commonPaths = []string{
			"/usr/local/bin",
			"/usr/bin",
			"/bin",
			filepath.Join(os.Getenv("HOME"), ".local/bin"),
			filepath.Join(os.Getenv("HOME"), "bin"),
			filepath.Join(os.Getenv("HOME"), "google-cloud-sdk/bin"), // gcloud SDK
			"/snap/bin", // Snap packages
		}
	} else {
		// Windows
		commonPaths = []string{
			`C:\Program Files\Amazon\AWSCLIV2`,
			`C:\Program Files (x86)\Google\Cloud SDK\google-cloud-sdk\bin`,
			`C:\Program Files\Microsoft SDKs\Azure\CLI2\wbin`,
		}
	}

	// Build new PATH with common paths prepended
	var pathsToAdd []string
	for _, p := range commonPaths {
		if p == "" {
			continue
		}
		// Only add if path exists and not already in PATH
		if _, err := os.Stat(p); err == nil && !strings.Contains(currentPath, p) {
			pathsToAdd = append(pathsToAdd, p)
		}
	}

	if len(pathsToAdd) > 0 {
		separator := ":"
		if runtime.GOOS == "windows" {
			separator = ";"
		}
		newPath := strings.Join(append(pathsToAdd, currentPath), separator)
		os.Setenv("PATH", newPath)
	}
}
