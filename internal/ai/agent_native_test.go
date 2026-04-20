// SPDX-License-Identifier: Apache-2.0

package ai

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Fake ToolAwareProvider for native tool-use tests
// ---------------------------------------------------------------------------

type fakeToolAwareProvider struct {
	fakeProvider
	turns    []*ToolResponse
	turnErr  error
	received [][]Message
	mu       sync.Mutex
	idx      int
}

func newFakeToolAwareProvider(name string, turns ...*ToolResponse) *fakeToolAwareProvider {
	return &fakeToolAwareProvider{
		fakeProvider: fakeProvider{name: name},
		turns:        turns,
	}
}

func (p *fakeToolAwareProvider) QueryWithTools(_ context.Context, messages []Message, _ []ToolDefinition) (*ToolResponse, error) {
	if p.turnErr != nil {
		return nil, p.turnErr
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Record a copy of the incoming messages so the test can assert on turn content.
	p.received = append(p.received, append([]Message(nil), messages...))

	if p.idx >= len(p.turns) {
		// After exhausting scripted turns, stop tool-calling (final text).
		return &ToolResponse{Text: "done"}, nil
	}
	resp := p.turns[p.idx]
	p.idx++
	return resp, nil
}

// ---------------------------------------------------------------------------
// Fake ApprovalHandler
// ---------------------------------------------------------------------------

type fakeApprover struct {
	decisions map[string]bool // toolCallID → approved
	defaultOK bool
	err       error
	seen      []string
	mu        sync.Mutex
}

func (h *fakeApprover) RequestApproval(_ context.Context, _, toolCallID, _ string, _ map[string]string, _ ToolCategory) (ApprovalDecision, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seen = append(h.seen, toolCallID)
	if h.err != nil {
		return ApprovalDecision{}, h.err
	}
	if v, ok := h.decisions[toolCallID]; ok {
		return ApprovalDecision{Approved: v}, nil
	}
	return ApprovalDecision{Approved: h.defaultOK}, nil
}

// ---------------------------------------------------------------------------
// Native tool-use: provider emits a ToolCall, agent executes and feeds back.
// ---------------------------------------------------------------------------

func TestAgent_NativeToolUse_ExecutesAndLoops(t *testing.T) {
	// Turn 1: LLM requests list_resources via native tool-use.
	// Turn 2: LLM finishes with a final text response (no tool_calls).
	turn1 := &ToolResponse{
		ToolCalls: []ToolCall{
			{ID: "tc_1", Name: "list_resources", Parameters: map[string]string{"kind": "pods"}},
		},
		InputTokens:  5,
		OutputTokens: 3,
	}
	turn2 := &ToolResponse{Text: "There are 2 pods.", InputTokens: 2, OutputTokens: 2}

	provider := newFakeToolAwareProvider("openai", turn1, turn2)
	emitter := &fakeEmitter{}
	executor := newFakeExecutor()
	executor.results["list_resources"] = "pod-a\npod-b"

	agent := buildAgent(provider, permissiveCfg(), emitter, executor)
	resp, err := agent.QueryWithTools(context.Background(), "List pods")
	if err != nil {
		t.Fatalf("QueryWithTools: %v", err)
	}
	if resp != "There are 2 pods." {
		t.Errorf("final response = %q", resp)
	}
	if len(executor.calls) != 1 || executor.calls[0] != "list_resources" {
		t.Errorf("executor calls = %v", executor.calls)
	}

	// The provider must receive a tool-result turn on iteration 2, correlated
	// via ToolCallID.
	if len(provider.received) != 2 {
		t.Fatalf("provider received %d turns, want 2", len(provider.received))
	}
	second := provider.received[1]
	// Expect: original user turn, assistant turn with tool_calls, tool result.
	if len(second) < 3 {
		t.Fatalf("second turn has %d messages, want >= 3 (user, assistant, tool)", len(second))
	}
	last := second[len(second)-1]
	if last.Role != "tool" {
		t.Errorf("last message role = %q, want 'tool'", last.Role)
	}
	if last.ToolCallID != "tc_1" {
		t.Errorf("last message ToolCallID = %q, want tc_1", last.ToolCallID)
	}
}

// ---------------------------------------------------------------------------
// Native tool-use: provider error bubbles up with iteration context.
// ---------------------------------------------------------------------------

func TestAgent_NativeToolUse_ProviderError(t *testing.T) {
	provider := newFakeToolAwareProvider("openai")
	provider.turnErr = fmt.Errorf("network blip")
	emitter := &fakeEmitter{}
	executor := newFakeExecutor()

	agent := buildAgent(provider, permissiveCfg(), emitter, executor)
	_, err := agent.QueryWithTools(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error from provider.QueryWithTools")
	}
	if !strings.Contains(err.Error(), "native tool-use failed") {
		t.Errorf("error should mention 'native tool-use failed', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Native tool-use: max iterations with unending tool calls.
// ---------------------------------------------------------------------------

func TestAgent_NativeToolUse_MaxIterations(t *testing.T) {
	// Every turn requests a tool; agent will hit maxIterations.
	turns := make([]*ToolResponse, maxIterations+2)
	for i := range turns {
		turns[i] = &ToolResponse{
			ToolCalls: []ToolCall{
				{ID: fmt.Sprintf("tc_%d", i), Name: "list_resources", Parameters: map[string]string{"kind": "pods"}},
			},
		}
	}
	provider := newFakeToolAwareProvider("openai", turns...)
	emitter := &fakeEmitter{}
	executor := newFakeExecutor()

	agent := buildAgent(provider, permissiveCfg(), emitter, executor)
	resp, err := agent.QueryWithTools(context.Background(), "infinite tools")
	if err != nil {
		t.Fatalf("QueryWithTools: %v", err)
	}
	if !strings.Contains(resp, "Maximum iterations") {
		t.Errorf("expected graceful 'Maximum iterations' message, got: %q", resp)
	}
	if len(executor.calls) != maxIterations {
		t.Errorf("executor calls = %d, want %d (one per iteration)", len(executor.calls), maxIterations)
	}
}

// ---------------------------------------------------------------------------
// Approval flow: write tool requires user approval; denial blocks execution.
// ---------------------------------------------------------------------------

func TestAgent_ApprovalDenied_ToolNotExecuted(t *testing.T) {
	// Turn 1: request scale_deployment (write → requires approval).
	// Turn 2: final answer after the rejection is reported back.
	turn1 := &ToolResponse{
		ToolCalls: []ToolCall{
			{ID: "tc_1", Name: "scale_deployment", Parameters: map[string]string{"namespace": "default", "name": "web", "replicas": "3"}},
		},
	}
	turn2 := &ToolResponse{Text: "acknowledged rejection"}

	provider := newFakeToolAwareProvider("openai", turn1, turn2)
	emitter := &fakeEmitter{}
	executor := newFakeExecutor()
	approver := &fakeApprover{defaultOK: false}

	agent := buildAgent(provider, permissiveCfg(), emitter, executor).WithApproval(approver, "sess-1")
	_, err := agent.QueryWithTools(context.Background(), "scale to 3")
	if err != nil {
		t.Fatalf("QueryWithTools: %v", err)
	}

	if len(executor.calls) != 0 {
		t.Errorf("executor was called %d times despite rejection: %v", len(executor.calls), executor.calls)
	}
	if len(approver.seen) != 1 {
		t.Errorf("approver should have been asked exactly once, got %d", len(approver.seen))
	}

	// An "approval-needed" event must have been emitted.
	names := emitter.emittedNames()
	found := false
	for _, n := range names {
		if n == "ai:agent:approval-needed" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ai:agent:approval-needed event, got: %v", names)
	}
}

func TestAgent_ApprovalApproved_ToolExecutes(t *testing.T) {
	turn1 := &ToolResponse{
		ToolCalls: []ToolCall{
			{ID: "tc_1", Name: "scale_deployment", Parameters: map[string]string{"namespace": "default", "name": "web", "replicas": "3"}},
		},
	}
	turn2 := &ToolResponse{Text: "ok"}

	provider := newFakeToolAwareProvider("openai", turn1, turn2)
	emitter := &fakeEmitter{}
	executor := newFakeExecutor()
	executor.results["scale_deployment"] = "scaled"
	approver := &fakeApprover{defaultOK: true}

	agent := buildAgent(provider, permissiveCfg(), emitter, executor).WithApproval(approver, "sess-1")
	_, err := agent.QueryWithTools(context.Background(), "scale to 3")
	if err != nil {
		t.Fatalf("QueryWithTools: %v", err)
	}

	if len(executor.calls) != 1 || executor.calls[0] != "scale_deployment" {
		t.Errorf("executor should have run scale_deployment, got %v", executor.calls)
	}
}

// ---------------------------------------------------------------------------
// Approval flow: approval handler error surfaces as a tool rejection.
// ---------------------------------------------------------------------------

func TestAgent_ApprovalError_ReportedAsCanceled(t *testing.T) {
	turn1 := &ToolResponse{
		ToolCalls: []ToolCall{
			{ID: "tc_1", Name: "delete_resource", Parameters: map[string]string{"kind": "Pod", "namespace": "default", "name": "broken"}},
		},
	}
	turn2 := &ToolResponse{Text: "gave up"}

	provider := newFakeToolAwareProvider("openai", turn1, turn2)
	emitter := &fakeEmitter{}
	executor := newFakeExecutor()
	approver := &fakeApprover{err: context.Canceled}

	agent := buildAgent(provider, permissiveCfg(), emitter, executor).WithApproval(approver, "sess-1")
	if _, err := agent.QueryWithTools(context.Background(), "delete pod"); err != nil {
		t.Fatalf("QueryWithTools: %v", err)
	}

	if len(executor.calls) != 0 {
		t.Error("executor should not have been called when approval returns an error")
	}

	// The last tool-result event should indicate the approval was canceled.
	var lastResult string
	for _, e := range emitter.events {
		if e.name == "ai:agent:tool-result" && len(e.data) > 0 {
			if tr, ok := e.data[0].(agentToolResultEvent); ok {
				lastResult = tr.Result
			}
		}
	}
	if !strings.Contains(lastResult, "APPROVAL CANCELED") {
		t.Errorf("expected APPROVAL CANCELED marker in tool-result, got %q", lastResult)
	}
}

// ---------------------------------------------------------------------------
// Read tools auto-execute: no approval handler ever called.
// ---------------------------------------------------------------------------

func TestAgent_ReadTool_AutoExecutesWithoutApproval(t *testing.T) {
	// Text-fallback path (fakeProvider is not ToolAwareProvider) — read tool.
	toolCall := toolCallBlock(`{"name":"get_pod_logs","parameters":{"namespace":"default","pod":"web"}}`)
	provider := newFakeProvider("ollama", toolCall, "here are the logs")
	emitter := &fakeEmitter{}
	executor := newFakeExecutor()
	executor.results["get_pod_logs"] = "log line 1\nlog line 2"
	approver := &fakeApprover{defaultOK: false} // would reject if asked

	agent := buildAgent(provider, permissiveCfg(), emitter, executor).WithApproval(approver, "sess-1")
	_, err := agent.QueryWithTools(context.Background(), "show me logs")
	if err != nil {
		t.Fatalf("QueryWithTools: %v", err)
	}

	if len(approver.seen) != 0 {
		t.Errorf("read-only tool should NOT trigger approval, but approver was called %d times", len(approver.seen))
	}
	if len(executor.calls) != 1 || executor.calls[0] != "get_pod_logs" {
		t.Errorf("expected get_pod_logs to auto-execute, got %v", executor.calls)
	}
}
