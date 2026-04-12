package core_test

import (
	"testing"
)

func TestClusterService_NilManager(t *testing.T) {
	// Create a new ClusterService which might have a nil manager locally if no kubeconfig is present,
	// BUT since we can't easily force NewManager to fail in this environment without mocking,
	// we will manually construct a ClusterService with a nil manager to test our safety guards.
	// Note: We can't access private fields of ClusterService from outside the package.
	// So we will rely on creating a test file inside the package `core`.
}
