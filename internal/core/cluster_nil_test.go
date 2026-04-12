package core

import (
	"context"
	"testing"
)

// ---------------------------------------------------------------------------
// ClusterService nil manager (in-package access to unexported fields)
// ---------------------------------------------------------------------------

func TestClusterService_NilManager_GetContextsReturnsError(t *testing.T) {
	svc := &ClusterService{manager: nil}
	_, err := svc.GetContexts(context.Background())
	if err == nil {
		t.Error("GetContexts with nil manager should return error")
	}
}

func TestClusterService_NilManager_RefreshContextsReturnsError(t *testing.T) {
	svc := &ClusterService{manager: nil}
	_, err := svc.RefreshContexts(context.Background())
	if err == nil {
		t.Error("RefreshContexts with nil manager should return error")
	}
}

func TestClusterService_NilManager_ConnectReturnsError(t *testing.T) {
	svc := &ClusterService{manager: nil}
	err := svc.Connect(context.Background(), "prod")
	if err == nil {
		t.Error("Connect with nil manager should return error")
	}
}

func TestClusterService_NilManager_DisconnectReturnsError(t *testing.T) {
	svc := &ClusterService{manager: nil}
	err := svc.Disconnect("prod")
	if err == nil {
		t.Error("Disconnect with nil manager should return error")
	}
}

func TestClusterService_NilManager_GetClusterInfoReturnsError(t *testing.T) {
	svc := &ClusterService{manager: nil}
	_, err := svc.GetClusterInfo(context.Background())
	if err == nil {
		t.Error("GetClusterInfo with nil manager should return error")
	}
}

func TestClusterService_NilManager_CloseReturnsNil(t *testing.T) {
	svc := &ClusterService{manager: nil}
	if err := svc.Close(); err != nil {
		t.Errorf("Close with nil manager should return nil, got %v", err)
	}
}

func TestClusterService_NilManager_IsConnectedFalse(t *testing.T) {
	svc := &ClusterService{manager: nil}
	if svc.IsConnected() {
		t.Error("IsConnected with nil manager should return false")
	}
}

func TestClusterService_NilManager_ActiveContextEmpty(t *testing.T) {
	svc := &ClusterService{manager: nil}
	if got := svc.ActiveContext(); got != "" {
		t.Errorf("ActiveContext with nil manager = %q, want empty", got)
	}
}
