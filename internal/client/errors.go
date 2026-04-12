// SPDX-License-Identifier: Apache-2.0

package client

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrContextNotFound is returned when a kubeconfig context doesn't exist.
	ErrContextNotFound = errors.New("context not found")

	// ErrClusterNotFound is returned when a kubeconfig cluster doesn't exist.
	ErrClusterNotFound = errors.New("cluster not found")

	// ErrNotConnected is returned when trying to use a disconnected client.
	ErrNotConnected = errors.New("not connected to cluster")

	// ErrNoActiveCluster is returned when no cluster is active.
	ErrNoActiveCluster = errors.New("no active cluster")

	// ErrClusterAlreadyExists is returned when adding a duplicate cluster.
	ErrClusterAlreadyExists = errors.New("cluster already exists")

	// ErrResourceNotFound is returned when a resource doesn't exist.
	ErrResourceNotFound = errors.New("resource not found")

	// ErrTimeout is returned when an operation times out.
	ErrTimeout = errors.New("operation timed out")

	// ErrForbidden is returned when access is denied.
	ErrForbidden = errors.New("access forbidden")
)

// FormatConnectionError formats connection errors with helpful context.
// It detects common issues like missing credential plugins and provides
// user-friendly error messages with suggestions.
func FormatConnectionError(err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Detect exec credential plugin errors
	if strings.Contains(errStr, "executable file not found") ||
		strings.Contains(errStr, "exec: ") {
		// Extract the command name if possible
		if strings.Contains(errStr, "\"aws\"") || strings.Contains(errStr, "'aws'") {
			return fmt.Errorf("AWS CLI not found in PATH - required for EKS cluster authentication.\n\n⚠️  GUI apps don't inherit your shell's PATH!\n\nTo fix this:\n1. Restart the app from terminal: 'cd /path/to/app && ./kubecat'\n2. Or set PATH in ~/.zshenv (macOS) / ~/.profile (Linux)\n3. Or install AWS CLI to /usr/local/bin/\n\nIf AWS CLI is installed, verify with: which aws\n\nOriginal error: %s", errStr)
		}
		if strings.Contains(errStr, "\"gcloud\"") || strings.Contains(errStr, "'gcloud'") {
			return fmt.Errorf("gcloud CLI not found in PATH - required for GKE cluster authentication.\n\n⚠️  GUI apps don't inherit your shell's PATH!\n\nTo fix this:\n1. Restart the app from terminal: 'cd /path/to/app && ./kubecat'\n2. Or set PATH in ~/.zshenv (macOS) / ~/.profile (Linux)\n3. Or install gcloud to /usr/local/bin/\n\nIf gcloud is installed, verify with: which gcloud\n\nOriginal error: %s", errStr)
		}
		if strings.Contains(errStr, "\"az\"") || strings.Contains(errStr, "'az'") {
			return fmt.Errorf("Azure CLI not found in PATH - required for AKS cluster authentication.\n\n⚠️  GUI apps don't inherit your shell's PATH!\n\nTo fix this:\n1. Restart the app from terminal: 'cd /path/to/app && ./kubecat'\n2. Or set PATH in ~/.zshenv (macOS) / ~/.profile (Linux)\n3. Or install Azure CLI to /usr/local/bin/\n\nIf Azure CLI is installed, verify with: which az\n\nOriginal error: %s", errStr)
		}
		// Generic exec plugin error
		return fmt.Errorf("credential plugin not found in PATH - your kubeconfig requires an external authentication tool.\n\n⚠️  GUI apps don't inherit your shell's PATH!\n\nTo fix this:\n1. Restart the app from terminal\n2. Or set PATH in ~/.zshenv (macOS) / ~/.profile (Linux)\n3. Or check your kubeconfig file for 'exec' sections\n\nOriginal error: %s", errStr)
	}

	// Detect connection refused
	if strings.Contains(errStr, "connection refused") {
		return fmt.Errorf("cluster connection refused - the cluster may be offline or unreachable.\n\nCheck:\n1. Cluster is running\n2. Network connectivity\n3. VPN connection (if required)\n\nOriginal error: %s", errStr)
	}

	// Detect timeout
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return fmt.Errorf("connection timeout - the cluster is not responding.\n\nCheck:\n1. Cluster is running\n2. Network connectivity\n3. Firewall rules\n\nOriginal error: %s", errStr)
	}

	// Return original error if no special case matched
	return err
}
