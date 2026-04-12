// Package version provides build and version information for Kubecat.
package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the semantic version of Kubecat.
	Version = "0.1.0-dev"

	// GitCommit is the git commit hash, set at build time.
	GitCommit = "unknown"

	// BuildDate is the build timestamp, set at build time.
	BuildDate = "unknown"
)

// Info returns a formatted version string with all build details.
func Info() string {
	return fmt.Sprintf(`Kubecat - Kubernetes Command Center

Version:    %s
Git Commit: %s
Build Date: %s
Go Version: %s
OS/Arch:    %s/%s`,
		Version, GitCommit, BuildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// Short returns just the version number.
func Short() string {
	return Version
}
