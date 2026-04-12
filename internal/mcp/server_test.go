package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"
)

func TestDispatch_Initialize_ReturnsServerInfo(t *testing.T) {
	srv := &Server{handler: &Handler{}}
	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	resp := srv.dispatch(context.Background(), []byte(req))
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	info, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatal("expected serverInfo in result")
	}
	if info["name"] != "kubecat" {
		t.Errorf("expected server name kubecat, got %v", info["name"])
	}
}

func TestDispatch_ToolsList_ReturnsToolDefinitions(t *testing.T) {
	srv := &Server{handler: &Handler{}}
	req := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	resp := srv.dispatch(context.Background(), []byte(req))
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	result := resp.Result.(map[string]interface{})
	tools := result["tools"].([]ToolDefinition)
	if len(tools) == 0 {
		t.Error("expected at least one tool definition")
	}
	names := map[string]bool{}
	for _, td := range tools {
		names[td.Name] = true
	}
	for _, expected := range []string{"list_clusters", "get_resource", "list_resources", "exec_kubectl", "ai_query"} {
		if !names[expected] {
			t.Errorf("missing tool definition: %s", expected)
		}
	}
}

func TestDispatch_UnknownMethod_ReturnsMethodNotFound(t *testing.T) {
	srv := &Server{handler: &Handler{}}
	req := `{"jsonrpc":"2.0","id":3,"method":"unknown/method"}`
	resp := srv.dispatch(context.Background(), []byte(req))
	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestDispatch_ParseError_ReturnsBadJSON(t *testing.T) {
	srv := &Server{handler: &Handler{}}
	resp := srv.dispatch(context.Background(), []byte("not json"))
	if resp.Error == nil {
		t.Fatal("expected parse error")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("expected code -32700, got %d", resp.Error.Code)
	}
}

func TestServer_StartStop_WritesResponses(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	// Use a pipe so Start() reads the line before ctx is canceled.
	pr, pw := io.Pipe()
	var out bytes.Buffer
	srv := NewServer(&Handler{}, pr, &out)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.Start(ctx)
	}()

	_, _ = pw.Write([]byte(input))
	// Give the server time to process the request before canceling.
	time.Sleep(50 * time.Millisecond)
	cancel()
	pw.Close()
	// Wait for server to stop — ensures all writes to out are complete.
	<-done

	if out.Len() == 0 {
		t.Fatal("expected output from server")
	}
	var resp rpcResponse
	if err := json.NewDecoder(&out).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %q", resp.JSONRPC)
	}
}

func TestExecKubectl_MutatingVerb_Rejected(t *testing.T) {
	h := &Handler{}
	for _, verb := range []string{"delete", "apply", "patch", "create", "replace"} {
		_, err := h.execKubectl(map[string]interface{}{
			"args": []interface{}{verb, "pod", "my-pod"},
		})
		if err == nil {
			t.Errorf("expected error for mutating verb %q, got nil", verb)
		}
	}
}

func TestExecKubectl_ReadOnlyVerb_Allowed(t *testing.T) {
	h := &Handler{}
	for _, verb := range []string{"get", "describe", "logs", "top"} {
		result, err := h.execKubectl(map[string]interface{}{
			"args": []interface{}{verb, "pods"},
		})
		if err != nil {
			t.Errorf("read-only verb %q should not error: %v", verb, err)
		}
		if result == "" {
			t.Errorf("expected non-empty result for verb %q", verb)
		}
	}
}
