package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/thepixelabs/kubecat/internal/audit"
	"github.com/thepixelabs/kubecat/internal/events"
)

const maxIterations = 10

// ToolCall represents a tool invocation parsed from an LLM response.
type ToolCall struct {
	// ID is a provider-assigned identifier used to correlate tool results
	// with the originating call in native tool-use APIs (OpenAI, Anthropic).
	ID         string            `json:"id,omitempty"`
	Name       string            `json:"name"`
	Parameters map[string]string `json:"parameters"`
}

// ToolExecutor is the interface that the App must satisfy to provide tool
// execution back to the agent.
type ToolExecutor interface {
	// ExecuteTool runs the named tool with the provided parameters and returns
	// the result as a string to be fed back to the LLM.
	ExecuteTool(ctx context.Context, toolName string, params map[string]string) (string, error)
}

// ApprovalDecision carries the outcome of a user approval request.
type ApprovalDecision struct {
	// Approved is true when the user clicked "Approve", false for "Reject".
	Approved bool
}

// ApprovalHandler is an optional interface the agent calls when a tool
// requires user confirmation before execution.  Implementations block until
// the user responds (or the context is canceled) and then return the decision.
type ApprovalHandler interface {
	// RequestApproval blocks until the user approves or rejects the tool call,
	// or until ctx is canceled.  toolCallID is a unique identifier for this
	// specific invocation so the UI can route responses back correctly.
	RequestApproval(ctx context.Context, sessionID, toolCallID, toolName string, params map[string]string, category ToolCategory) (ApprovalDecision, error)
}

// Agent orchestrates the observe-think-act loop.
type Agent struct {
	provider   Provider
	guardrails *Guardrails
	emitter    events.EmitterInterface
	executor   ToolExecutor
	// approver is optional; if nil, tools that require approval are executed
	// without prompting (backwards-compatible behavior).
	approver  ApprovalHandler
	sessionID string
	cluster   string
	namespace string
}

// NewAgent creates a new Agent.
func NewAgent(
	provider Provider,
	guardrails *Guardrails,
	emitter events.EmitterInterface,
	executor ToolExecutor,
	cluster, namespace string,
) *Agent {
	return &Agent{
		provider:   provider,
		guardrails: guardrails,
		emitter:    emitter,
		executor:   executor,
		cluster:    cluster,
		namespace:  namespace,
	}
}

// WithApproval attaches an ApprovalHandler to the agent and sets the session
// ID used to scope approval events.  Call before QueryWithTools.
func (a *Agent) WithApproval(handler ApprovalHandler, sessionID string) *Agent {
	a.approver = handler
	a.sessionID = sessionID
	return a
}

// agentThinkingEvent is emitted when the LLM is processing.
type agentThinkingEvent struct {
	Iteration int `json:"iteration"`
}

// agentToolCallEvent is emitted when the agent invokes a tool.
type agentToolCallEvent struct {
	Tool       string            `json:"tool"`
	Parameters map[string]string `json:"parameters"`
	Iteration  int               `json:"iteration"`
}

// agentToolResultEvent is emitted with the tool execution result.
type agentToolResultEvent struct {
	Tool      string `json:"tool"`
	Result    string `json:"result"`
	Allowed   bool   `json:"allowed"`
	Reason    string `json:"reason,omitempty"`
	Iteration int    `json:"iteration"`
}

// agentCompleteEvent is emitted when the loop finishes.
type agentCompleteEvent struct {
	Response   string `json:"response"`
	Iterations int    `json:"iterations"`
}

// QueryWithTools runs the observe-think-act loop.  It returns the final natural
// language response from the LLM after all tool calls are resolved.
//
// When the provider implements ToolAwareProvider the native structured
// tool-calling API is used (OpenAI function_calling, Anthropic tool_use).
// All other providers fall back to text-based tool-call parsing.
func (a *Agent) QueryWithTools(ctx context.Context, prompt string) (string, error) {
	a.guardrails.Reset()
	audit.LogAIQuery(a.provider.Name(), a.cluster, a.namespace, prompt)

	systemPreamble := buildToolPreamble()
	fullPrompt := systemPreamble + "\n\n" + prompt

	if tap, ok := a.provider.(ToolAwareProvider); ok {
		return a.queryWithNativeTools(ctx, tap, fullPrompt)
	}
	return a.queryWithTextFallback(ctx, fullPrompt)
}

// queryWithNativeTools runs the agent loop using a provider's structured
// tool-calling API.  Tool call IDs are preserved so multi-turn conversations
// correctly correlate results with requests.
func (a *Agent) queryWithNativeTools(ctx context.Context, tap ToolAwareProvider, fullPrompt string) (string, error) {
	tools := Registry
	messages := []Message{{Role: "user", Content: fullPrompt}}

	for i := 0; i < maxIterations; i++ {
		a.emitter.Emit("ai:agent:thinking", agentThinkingEvent{Iteration: i + 1})

		resp, err := tap.QueryWithTools(ctx, messages, tools)
		if err != nil {
			return "", fmt.Errorf("agent iteration %d: native tool-use failed: %w", i+1, err)
		}
		a.guardrails.AddTokenUsage(resp.InputTokens + resp.OutputTokens)

		if len(resp.ToolCalls) == 0 {
			a.emitter.Emit("ai:agent:complete", agentCompleteEvent{Response: resp.Text, Iterations: i + 1})
			return resp.Text, nil
		}

		// Append the assistant turn (with its tool_calls) so the provider can
		// correlate subsequent tool results.
		messages = append(messages, Message{
			Role:      "assistant",
			Content:   resp.Text,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool call and append results.
		for _, call := range resp.ToolCalls {
			result, _ := a.executeToolCall(ctx, call, i+1, resp.InputTokens+resp.OutputTokens)
			messages = append(messages, Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: call.ID,
				ToolName:   call.Name,
			})
		}
	}

	slog.Warn("agent: max native iterations reached", slog.String("provider", tap.Name()))
	return fmt.Sprintf("Maximum iterations (%d) reached. Please refine your question.", maxIterations), nil
}

// queryWithTextFallback runs the agent loop for providers without native
// tool-use support by injecting a tool schema into the system prompt and
// parsing markdown tool-call blocks from the response text.
func (a *Agent) queryWithTextFallback(ctx context.Context, fullPrompt string) (string, error) {
	var conversation []string
	conversation = append(conversation, fullPrompt)
	tokensBudgeted := 0

	for i := 0; i < maxIterations; i++ {
		a.emitter.Emit("ai:agent:thinking", agentThinkingEvent{Iteration: i + 1})

		msg := strings.Join(conversation, "\n\n---\n\n")

		resp, err := a.provider.Query(ctx, msg)
		if err != nil {
			return "", fmt.Errorf("agent iteration %d: provider query failed: %w", i+1, err)
		}

		// Rough token estimate: 4 chars ≈ 1 token.
		tokensBudgeted += (len(msg) + len(resp)) / 4
		a.guardrails.AddTokenUsage(tokensBudgeted)

		toolCalls := parseToolCalls(resp)
		if len(toolCalls) == 0 {
			a.emitter.Emit("ai:agent:complete", agentCompleteEvent{Response: resp, Iterations: i + 1})
			return resp, nil
		}

		var toolResults strings.Builder
		for _, call := range toolCalls {
			result, _ := a.executeToolCall(ctx, call, i+1, tokensBudgeted)
			toolResults.WriteString(fmt.Sprintf("Tool %q result:\n%s\n\n", call.Name, result))
		}

		conversation = append(conversation, resp)
		conversation = append(conversation, "Tool results:\n"+toolResults.String())
	}

	slog.Warn("agent: max text iterations reached", slog.String("provider", a.provider.Name()))
	return fmt.Sprintf("Maximum iterations (%d) reached. Please refine your question.", maxIterations), nil
}

// agentApprovalNeededEvent is emitted when a tool requires user confirmation.
type agentApprovalNeededEvent struct {
	SessionID  string            `json:"sessionId"`
	ToolCallID string            `json:"toolCallId"`
	Tool       string            `json:"tool"`
	Parameters map[string]string `json:"parameters"`
	Category   string            `json:"category"`
	Iteration  int               `json:"iteration"`
}

// executeToolCall checks guardrails and, if allowed, executes a single tool.
// Returns the result string and whether the call was permitted.
func (a *Agent) executeToolCall(ctx context.Context, call ToolCall, iteration, tokens int) (string, bool) {
	slog.Debug("agent tool call", slog.String("tool", call.Name), slog.Int("iteration", iteration))

	a.emitter.Emit("ai:agent:tool-call", agentToolCallEvent{
		Tool:       call.Name,
		Parameters: call.Parameters,
		Iteration:  iteration,
	})

	// If the tool call explicitly targets a namespace, use that for guardrail
	// checks — the tool's target namespace determines the protection boundary,
	// not the agent's default namespace.
	ns := call.Parameters["namespace"]
	if ns == "" {
		ns = a.namespace
	}

	check := a.guardrails.CheckTool(call.Name, ns, a.cluster, tokens)
	if !check.Allowed {
		slog.Info("agent: tool blocked by guardrails",
			slog.String("tool", call.Name),
			slog.String("reason", check.Reason))

		result := fmt.Sprintf("[BLOCKED by policy: %s]", check.Reason)
		a.emitter.Emit("ai:agent:tool-result", agentToolResultEvent{
			Tool:      call.Name,
			Result:    result,
			Allowed:   false,
			Reason:    check.Reason,
			Iteration: iteration,
		})
		return result, false
	}

	// For write/destructive tools, request user approval before executing.
	// This is done after guardrails so a blocked tool never reaches this point.
	if toolDef, ok := ToolByName(call.Name); ok && toolDef.ApprovalPolicy != ApprovalNever && a.approver != nil {
		toolCallID := fmt.Sprintf("%s_%d_%d", call.Name, iteration, tokens)
		a.emitter.Emit("ai:agent:approval-needed", agentApprovalNeededEvent{
			SessionID:  a.sessionID,
			ToolCallID: toolCallID,
			Tool:       call.Name,
			Parameters: call.Parameters,
			Category:   string(toolDef.Category),
			Iteration:  iteration,
		})

		decision, err := a.approver.RequestApproval(ctx, a.sessionID, toolCallID, call.Name, call.Parameters, toolDef.Category)
		if err != nil {
			result := fmt.Sprintf("[APPROVAL CANCELED: %s]", err.Error())
			a.emitter.Emit("ai:agent:tool-result", agentToolResultEvent{
				Tool:      call.Name,
				Result:    result,
				Allowed:   false,
				Reason:    "approval request canceled or timed out",
				Iteration: iteration,
			})
			return result, false
		}
		if !decision.Approved {
			result := "[REJECTED by user]"
			a.emitter.Emit("ai:agent:tool-result", agentToolResultEvent{
				Tool:      call.Name,
				Result:    result,
				Allowed:   false,
				Reason:    "user rejected the tool call",
				Iteration: iteration,
			})
			return result, false
		}
	}

	// Execute with per-tool timeout.
	toolCtx, cancel := toolCallContext(ctx, check.Timeout)
	defer cancel()

	result, err := a.executor.ExecuteTool(toolCtx, call.Name, call.Parameters)
	if err != nil {
		result = fmt.Sprintf("[ERROR: %s]", err.Error())
		slog.Warn("agent: tool execution error",
			slog.String("tool", call.Name),
			slog.Any("error", err))
	}

	a.emitter.Emit("ai:agent:tool-result", agentToolResultEvent{
		Tool:      call.Name,
		Result:    result,
		Allowed:   true,
		Iteration: iteration,
	})
	return result, true
}

// parseToolCalls extracts ToolCall structs from an LLM response.
// We look for JSON blocks delimited by ```tool ... ``` markers.
func parseToolCalls(response string) []ToolCall {
	var calls []ToolCall

	// Find ```tool\n...\n``` blocks.
	const open = "```tool"
	const close = "```"

	remaining := response
	for {
		start := strings.Index(remaining, open)
		if start == -1 {
			break
		}
		inner := remaining[start+len(open):]
		end := strings.Index(inner, close)
		if end == -1 {
			break
		}
		block := strings.TrimSpace(inner[:end])

		var call ToolCall
		if err := json.Unmarshal([]byte(block), &call); err == nil && call.Name != "" {
			calls = append(calls, call)
		}

		remaining = inner[end+len(close):]
	}

	return calls
}

// buildToolPreamble constructs the system message listing all available tools.
func buildToolPreamble() string {
	var sb strings.Builder
	sb.WriteString("You are a Kubernetes expert AI agent integrated with Kubecat.\n")
	sb.WriteString("You have access to the following tools. To call a tool, emit a code block with the language tag `tool` containing a JSON object with `name` and `parameters`:\n\n")
	sb.WriteString("```tool\n{\"name\": \"tool_name\", \"parameters\": {\"key\": \"value\"}}\n```\n\n")
	sb.WriteString("Available tools:\n\n")

	for _, t := range Registry {
		sb.WriteString(fmt.Sprintf("**%s** (%s): %s\n", t.Name, string(t.Category), t.Description))
		if len(t.Parameters) > 0 {
			sb.WriteString("  Parameters:\n")
			for _, p := range t.Parameters {
				req := ""
				if p.Required {
					req = " [required]"
				}
				sb.WriteString(fmt.Sprintf("    - %s (%s)%s: %s\n", p.Name, p.Type, req, p.Description))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("When you have gathered enough information, provide your final answer in HTML format as described in the system prompt. Do NOT emit tool blocks in your final answer.\n")
	return sb.String()
}
