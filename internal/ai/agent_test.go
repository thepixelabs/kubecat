package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// fakeProvider implements Provider for agent tests.
type fakeProvider struct {
	name      string
	responses []string // consumed in order; last is repeated once exhausted
	queryErr  error
	callCount int
}

func newFakeProvider(name string, responses ...string) *fakeProvider {
	return &fakeProvider{name: name, responses: responses}
}

func (p *fakeProvider) Name() string                     { return p.name }
func (p *fakeProvider) Available(_ context.Context) bool { return true }
func (p *fakeProvider) Close() error                     { return nil }

func (p *fakeProvider) Query(_ context.Context, _ string) (string, error) {
	if p.queryErr != nil {
		return "", p.queryErr
	}
	p.callCount++
	if len(p.responses) == 0 {
		return "no response configured", nil
	}
	idx := p.callCount - 1
	if idx >= len(p.responses) {
		idx = len(p.responses) - 1
	}
	return p.responses[idx], nil
}

func (p *fakeProvider) StreamQuery(_ context.Context, _ string) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- "stream response"
	close(ch)
	return ch, nil
}

// fakeEmitter records emitted events.
type fakeEmitter struct {
	events []struct {
		name string
		data []interface{}
	}
}

func (e *fakeEmitter) Emit(event string, data ...interface{}) {
	e.events = append(e.events, struct {
		name string
		data []interface{}
	}{name: event, data: data})
}

func (e *fakeEmitter) SetContext(_ context.Context) {}

func (e *fakeEmitter) emittedNames() []string {
	names := make([]string, len(e.events))
	for i, ev := range e.events {
		names[i] = ev.name
	}
	return names
}

// fakeExecutor implements ToolExecutor.
type fakeExecutor struct {
	results map[string]string
	err     map[string]error
	calls   []string
}

func newFakeExecutor() *fakeExecutor {
	return &fakeExecutor{
		results: make(map[string]string),
		err:     make(map[string]error),
	}
}

func (e *fakeExecutor) ExecuteTool(_ context.Context, toolName string, _ map[string]string) (string, error) {
	e.calls = append(e.calls, toolName)
	if err, ok := e.err[toolName]; ok {
		return "", err
	}
	if result, ok := e.results[toolName]; ok {
		return result, nil
	}
	return fmt.Sprintf("[result of %s]", toolName), nil
}

// buildAgent is a factory for use in agent tests.
func buildAgent(provider Provider, cfg GuardrailsConfig, emitter *fakeEmitter, executor *fakeExecutor) *Agent {
	g := NewGuardrails(cfg)
	return NewAgent(provider, g, emitter, executor, "test-cluster", "default")
}

// toolCallBlock builds a ```tool\n...\n``` block for a given tool call JSON.
func toolCallBlock(json string) string {
	return "```tool\n" + json + "\n```"
}

// ---------------------------------------------------------------------------
// Stop: no tool calls in response → return final answer
// ---------------------------------------------------------------------------

func TestAgent_Stop_ReturnsAnswerWhenNoToolCalls(t *testing.T) {
	provider := newFakeProvider("ollama", "The cluster is healthy.")
	emitter := &fakeEmitter{}
	executor := newFakeExecutor()
	agent := buildAgent(provider, permissiveCfg(), emitter, executor)

	resp, err := agent.QueryWithTools(context.Background(), "How is the cluster?")
	if err != nil {
		t.Fatalf("QueryWithTools returned error: %v", err)
	}
	if resp != "The cluster is healthy." {
		t.Errorf("QueryWithTools = %q, want exact LLM response", resp)
	}
}

// ---------------------------------------------------------------------------
// Tool-use loop
// ---------------------------------------------------------------------------

func TestAgent_ToolUseLoop_ExecutesToolAndContinues(t *testing.T) {
	// First response: one tool call. Second response: final answer.
	toolCall := toolCallBlock(`{"name":"list_resources","parameters":{"kind":"pods","namespace":"default"}}`)
	provider := newFakeProvider("ollama", toolCall, "Here are the pods.")
	emitter := &fakeEmitter{}
	executor := newFakeExecutor()
	executor.results["list_resources"] = "pod-a\npod-b"

	agent := buildAgent(provider, permissiveCfg(), emitter, executor)
	resp, err := agent.QueryWithTools(context.Background(), "List pods")
	if err != nil {
		t.Fatalf("QueryWithTools: %v", err)
	}
	if resp != "Here are the pods." {
		t.Errorf("final response = %q, want 'Here are the pods.'", resp)
	}
	if len(executor.calls) != 1 || executor.calls[0] != "list_resources" {
		t.Errorf("executor calls = %v, want [list_resources]", executor.calls)
	}
}

// ---------------------------------------------------------------------------
// Max iterations
// ---------------------------------------------------------------------------

func TestAgent_MaxIterations_ReturnsGracefulMessage(t *testing.T) {
	// Always return a tool call so the loop never stops naturally.
	toolCall := toolCallBlock(`{"name":"list_resources","parameters":{"kind":"pods"}}`)
	responses := make([]string, maxIterations+2)
	for i := range responses {
		responses[i] = toolCall
	}
	provider := newFakeProvider("ollama", responses...)
	emitter := &fakeEmitter{}
	executor := newFakeExecutor()

	agent := buildAgent(provider, permissiveCfg(), emitter, executor)
	resp, err := agent.QueryWithTools(context.Background(), "keep listing forever")
	if err != nil {
		t.Fatalf("QueryWithTools: %v", err)
	}
	if !strings.Contains(resp, "Maximum iterations") {
		t.Errorf("expected 'Maximum iterations' in response, got: %q", resp)
	}
	if provider.callCount != maxIterations {
		t.Errorf("provider.callCount = %d, want %d (maxIterations)", provider.callCount, maxIterations)
	}
}

// ---------------------------------------------------------------------------
// Max tokens (guardrail layer 7)
// ---------------------------------------------------------------------------

func TestAgent_TokenBudgetExhausted_ToolBlocked(t *testing.T) {
	cfg := permissiveCfg()
	cfg.TokenBudget = 1 // tiny budget — exhausted immediately

	// First response: tool call. Second: final answer.
	toolCall := toolCallBlock(`{"name":"list_resources","parameters":{"kind":"pods"}}`)
	provider := newFakeProvider("ollama", toolCall, "done")
	emitter := &fakeEmitter{}
	executor := newFakeExecutor()

	agent := buildAgent(provider, cfg, emitter, executor)
	// The tool will be blocked because token budget is tiny, but agent should still complete.
	_, err := agent.QueryWithTools(context.Background(), "list pods")
	if err != nil {
		t.Fatalf("QueryWithTools: %v", err)
	}
	// Tool may have been blocked; we just ensure no panic and no error.
}

// ---------------------------------------------------------------------------
// Tool execution error
// ---------------------------------------------------------------------------

func TestAgent_ToolExecutionError_ErrorWrappedInResult(t *testing.T) {
	toolCall := toolCallBlock(`{"name":"list_resources","parameters":{"kind":"pods"}}`)
	provider := newFakeProvider("ollama", toolCall, "failed to list pods")
	emitter := &fakeEmitter{}
	executor := newFakeExecutor()
	executor.err["list_resources"] = errors.New("connection refused")

	agent := buildAgent(provider, permissiveCfg(), emitter, executor)
	resp, err := agent.QueryWithTools(context.Background(), "list pods")
	if err != nil {
		t.Fatalf("QueryWithTools should not return error even on tool error: %v", err)
	}
	// Final response is returned normally
	if resp == "" {
		t.Error("agent should return a response even when tool fails")
	}
}

// ---------------------------------------------------------------------------
// Write tool blocked by guardrails
// ---------------------------------------------------------------------------

func TestAgent_WriteToolBlocked_ByProtectedNamespace(t *testing.T) {
	cfg := permissiveCfg()
	cfg.ProtectedNamespaces = []string{"kube-system"}

	toolCall := toolCallBlock(`{"name":"delete_resource","parameters":{"kind":"Pod","namespace":"kube-system","name":"coredns"}}`)
	provider := newFakeProvider("ollama", toolCall, "ok")
	emitter := &fakeEmitter{}
	executor := newFakeExecutor()

	agent := buildAgent(provider, cfg, emitter, executor)
	_, err := agent.QueryWithTools(context.Background(), "delete coredns")
	if err != nil {
		t.Fatalf("QueryWithTools: %v", err)
	}

	// The executor must NOT have been called for the blocked tool
	for _, call := range executor.calls {
		if call == "delete_resource" {
			t.Error("delete_resource should have been blocked by guardrails, but executor was called")
		}
	}
}

// ---------------------------------------------------------------------------
// LLM query error
// ---------------------------------------------------------------------------

func TestAgent_LLMQueryError_ReturnsError(t *testing.T) {
	provider := newFakeProvider("ollama")
	provider.queryErr = errors.New("provider unavailable")
	emitter := &fakeEmitter{}
	executor := newFakeExecutor()

	agent := buildAgent(provider, permissiveCfg(), emitter, executor)
	_, err := agent.QueryWithTools(context.Background(), "anything")
	if err == nil {
		t.Error("QueryWithTools should return error when provider query fails")
	}
	if !strings.Contains(err.Error(), "provider query failed") {
		t.Errorf("error message = %q, want substring 'provider query failed'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Events emitted
// ---------------------------------------------------------------------------

func TestAgent_EmitsThinkingAndCompleteEvents(t *testing.T) {
	provider := newFakeProvider("ollama", "Direct answer, no tools needed.")
	emitter := &fakeEmitter{}
	executor := newFakeExecutor()
	agent := buildAgent(provider, permissiveCfg(), emitter, executor)

	_, err := agent.QueryWithTools(context.Background(), "test")
	if err != nil {
		t.Fatalf("QueryWithTools: %v", err)
	}

	names := emitter.emittedNames()
	if !containsStr(names, "ai:agent:thinking") {
		t.Errorf("expected 'ai:agent:thinking' event, got events: %v", names)
	}
	if !containsStr(names, "ai:agent:complete") {
		t.Errorf("expected 'ai:agent:complete' event, got events: %v", names)
	}
}

// ---------------------------------------------------------------------------
// parseToolCalls
// ---------------------------------------------------------------------------

func TestParseToolCalls_ValidBlock(t *testing.T) {
	input := "Let me check.\n" +
		"```tool\n{\"name\":\"list_resources\",\"parameters\":{\"kind\":\"pods\"}}\n```\n" +
		"Done."

	calls := parseToolCalls(input)
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Name != "list_resources" {
		t.Errorf("tool name = %q, want list_resources", calls[0].Name)
	}
}

func TestParseToolCalls_MultipleBlocks(t *testing.T) {
	input := toolCallBlock(`{"name":"list_resources","parameters":{}}`) +
		"\n" +
		toolCallBlock(`{"name":"get_pod_logs","parameters":{"pod":"web"}}`)

	calls := parseToolCalls(input)
	if len(calls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(calls))
	}
}

func TestParseToolCalls_NoBlocks_ReturnsEmpty(t *testing.T) {
	calls := parseToolCalls("This is just text with no tool calls.")
	if len(calls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(calls))
	}
}

func TestParseToolCalls_InvalidJSON_Skipped(t *testing.T) {
	input := "```tool\nnot valid json\n```"
	calls := parseToolCalls(input)
	if len(calls) != 0 {
		t.Errorf("invalid JSON block should produce 0 calls, got %d", len(calls))
	}
}

func TestParseToolCalls_MissingNameField_Skipped(t *testing.T) {
	input := toolCallBlock(`{"parameters":{"kind":"pods"}}`)
	calls := parseToolCalls(input)
	if len(calls) != 0 {
		t.Errorf("tool call without name should be skipped, got %d", len(calls))
	}
}

// ---------------------------------------------------------------------------
// toolCallContext
// ---------------------------------------------------------------------------

func TestToolCallContext_DefaultsTo30s(t *testing.T) {
	ctx, cancel := toolCallContext(context.Background(), 0)
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("toolCallContext should set a deadline")
	}
	remaining := time.Until(deadline)
	if remaining < 29*time.Second || remaining > 31*time.Second {
		t.Errorf("default timeout should be ~30s, got ~%v", remaining.Round(time.Second))
	}
}

func TestToolCallContext_UsesProvidedTimeout(t *testing.T) {
	ctx, cancel := toolCallContext(context.Background(), 5*time.Second)
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("toolCallContext should set a deadline")
	}
	remaining := time.Until(deadline)
	if remaining > 6*time.Second || remaining < 4*time.Second {
		t.Errorf("timeout should be ~5s, got ~%v", remaining.Round(time.Second))
	}
}
