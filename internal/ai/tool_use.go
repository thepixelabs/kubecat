// SPDX-License-Identifier: Apache-2.0

package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Message represents a single turn in a multi-turn conversation.
type Message struct {
	// Role is "user", "assistant", or "tool".
	Role string `json:"role"`
	// Content is the text content (for user/assistant turns).
	Content string `json:"content,omitempty"`
	// ToolCallID links a tool result back to the request that produced it.
	ToolCallID string `json:"toolCallId,omitempty"`
	// ToolName is the name of the tool that produced a result turn.
	ToolName string `json:"toolName,omitempty"`
	// ToolCalls holds the tool invocations requested by an assistant turn.
	// Populated by native tool-use providers so subsequent turns can reference
	// the original call IDs when submitting results.
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
}

// ToolResponse wraps the result of a provider tool-use call.
type ToolResponse struct {
	// ToolCalls contains any tool invocations the LLM wants to make.
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	// Text is the final text response when no more tool calls are needed.
	Text string `json:"text,omitempty"`
	// InputTokens is the number of input tokens consumed (best-effort).
	InputTokens int `json:"inputTokens"`
	// OutputTokens is the number of output tokens produced (best-effort).
	OutputTokens int `json:"outputTokens"`
}

// ToolAwareProvider extends Provider with native tool-use support.
// Providers that implement this interface use their native structured
// function-calling APIs; callers SHOULD type-assert before using it.
//
// Providers that do NOT implement this interface fall back to the text-based
// tool-call parsing in Agent.QueryWithTools.
type ToolAwareProvider interface {
	Provider

	// QueryWithTools sends a multi-turn conversation with tool definitions to
	// the LLM using the provider's native tool-use format.
	// tools may be nil; in that case the call behaves like a plain Query.
	QueryWithTools(ctx context.Context, messages []Message, tools []ToolDefinition) (*ToolResponse, error)
}

// ToolSchema builds the provider-agnostic JSON Schema representation of a
// tool's parameter list, shared across provider implementations.
func toolParameterSchema(params []ParameterSchema) map[string]interface{} {
	properties := make(map[string]interface{}, len(params))
	var required []string
	for _, p := range params {
		prop := map[string]interface{}{
			"type":        p.Type,
			"description": p.Description,
		}
		properties[p.Name] = prop
		if p.Required {
			required = append(required, p.Name)
		}
	}
	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// ---------------------------------------------------------------------------
// OpenAI tool-use implementation
// ---------------------------------------------------------------------------

// openAIToolDef is the OpenAI function definition format.
type openAIToolDef struct {
	Type     string `json:"type"`
	Function struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Parameters  map[string]interface{} `json:"parameters"`
	} `json:"function"`
}

// openAIToolCallMsg wraps the OpenAI message format for tool calls.
type openAIToolCallMsg struct {
	Role       string                 `json:"role"`
	Content    string                 `json:"content,omitempty"`
	ToolCalls  []openAIToolCallDetail `json:"tool_calls,omitempty"`
	ToolCallID string                 `json:"tool_call_id,omitempty"`
	Name       string                 `json:"name,omitempty"`
}

type openAIToolCallDetail struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAIToolResponse struct {
	Choices []struct {
		Message struct {
			Content   string                 `json:"content"`
			ToolCalls []openAIToolCallDetail `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// QueryWithTools implements ToolAwareProvider for OpenAI using the native
// function-calling API.
func (p *OpenAIProvider) QueryWithTools(ctx context.Context, messages []Message, tools []ToolDefinition) (*ToolResponse, error) {
	var oaiMessages []openAIToolCallMsg
	for _, m := range messages {
		msg := openAIToolCallMsg{Role: m.Role}
		switch m.Role {
		case "tool":
			// OpenAI tool result format: role=tool, tool_call_id, content.
			msg.ToolCallID = m.ToolCallID
			msg.Content = m.Content
		case "assistant":
			msg.Content = m.Content
			// Re-attach tool_calls so OpenAI can correlate subsequent tool results.
			for _, tc := range m.ToolCalls {
				args, _ := json.Marshal(tc.Parameters)
				msg.ToolCalls = append(msg.ToolCalls, openAIToolCallDetail{
					ID:   tc.ID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: tc.Name, Arguments: string(args)},
				})
			}
		default:
			msg.Content = m.Content
		}
		oaiMessages = append(oaiMessages, msg)
	}

	var oaiTools []openAIToolDef
	for _, t := range tools {
		def := openAIToolDef{Type: "function"}
		def.Function.Name = t.Name
		def.Function.Description = t.Description
		def.Function.Parameters = toolParameterSchema(t.Parameters)
		oaiTools = append(oaiTools, def)
	}

	reqBody := map[string]interface{}{
		"model":    p.config.Model,
		"messages": oaiMessages,
	}
	if len(oaiTools) > 0 {
		reqBody["tools"] = oaiTools
	}
	if p.config.MaxTokens > 0 {
		reqBody["max_tokens"] = p.config.MaxTokens
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("openai QueryWithTools: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.Endpoint+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("openai QueryWithTools: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai QueryWithTools: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai QueryWithTools: %s — %s", resp.Status, string(body))
	}

	var result openAIToolResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("openai QueryWithTools: decode: %w", err)
	}

	out := &ToolResponse{
		InputTokens:  result.Usage.PromptTokens,
		OutputTokens: result.Usage.CompletionTokens,
	}

	if len(result.Choices) == 0 {
		return out, nil
	}

	choice := result.Choices[0]
	out.Text = choice.Message.Content

	for _, tc := range choice.Message.ToolCalls {
		var params map[string]string
		// OpenAI arguments is a JSON string like `{"namespace":"default","kind":"pods"}`
		var rawParams map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &rawParams); err == nil {
			params = make(map[string]string, len(rawParams))
			for k, v := range rawParams {
				params[k] = fmt.Sprintf("%v", v)
			}
		}
		out.ToolCalls = append(out.ToolCalls, ToolCall{
			ID:         tc.ID,
			Name:       tc.Function.Name,
			Parameters: params,
		})
	}

	return out, nil
}

// ---------------------------------------------------------------------------
// Anthropic tool-use implementation
// ---------------------------------------------------------------------------

type anthropicToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type anthropicToolUseResponse struct {
	Content []struct {
		Type  string                 `json:"type"`
		Text  string                 `json:"text,omitempty"`
		ID    string                 `json:"id,omitempty"`
		Name  string                 `json:"name,omitempty"`
		Input map[string]interface{} `json:"input,omitempty"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// QueryWithTools implements ToolAwareProvider for Anthropic using the native
// tool_use messages API.
func (p *AnthropicProvider) QueryWithTools(ctx context.Context, messages []Message, tools []ToolDefinition) (*ToolResponse, error) {
	// Convert our generic messages to Anthropic format.
	// Anthropic requires alternating user/assistant turns; tool results go
	// as "tool_result" blocks inside a user turn.
	type anthropicMsgContent = interface{} // either string or []map[string]interface{}
	type anthropicMsgEntry struct {
		Role    string              `json:"role"`
		Content anthropicMsgContent `json:"content"`
	}

	var anthMsgs []anthropicMsgEntry
	for _, m := range messages {
		switch m.Role {
		case "tool":
			// Tool result — wrap as a user turn with tool_result content block.
			anthMsgs = append(anthMsgs, anthropicMsgEntry{
				Role: "user",
				Content: []map[string]interface{}{
					{
						"type":        "tool_result",
						"tool_use_id": m.ToolCallID,
						"content":     m.Content,
					},
				},
			})
		default:
			anthMsgs = append(anthMsgs, anthropicMsgEntry{
				Role:    m.Role,
				Content: m.Content,
			})
		}
	}

	var anthTools []anthropicToolDef
	for _, t := range tools {
		anthTools = append(anthTools, anthropicToolDef{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: toolParameterSchema(t.Parameters),
		})
	}

	reqMap := map[string]interface{}{
		"model":      p.config.Model,
		"max_tokens": p.config.MaxTokens,
		"messages":   anthMsgs,
		"system":     "You are a Kubernetes expert assistant integrated with Kubecat.",
	}
	if len(anthTools) > 0 {
		reqMap["tools"] = anthTools
	}

	jsonBody, err := json.Marshal(reqMap)
	if err != nil {
		return nil, fmt.Errorf("anthropic QueryWithTools: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.Endpoint+"/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("anthropic QueryWithTools: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic QueryWithTools: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic QueryWithTools: %s — %s", resp.Status, string(body))
	}

	var result anthropicToolUseResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("anthropic QueryWithTools: decode: %w", err)
	}

	out := &ToolResponse{
		InputTokens:  result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
	}

	for _, block := range result.Content {
		switch block.Type {
		case "text":
			out.Text += block.Text
		case "tool_use":
			params := make(map[string]string, len(block.Input))
			for k, v := range block.Input {
				params[k] = fmt.Sprintf("%v", v)
			}
			out.ToolCalls = append(out.ToolCalls, ToolCall{
				ID:         block.ID,
				Name:       block.Name,
				Parameters: params,
			})
		}
	}

	return out, nil
}

// ---------------------------------------------------------------------------
// Ollama fallback — no native tool-use; uses text-based tool call format
// ---------------------------------------------------------------------------

// QueryWithTools for Ollama falls back to the text-based tool preamble approach
// since most Ollama models don't have a standardized tool-calling protocol.
// The result will have an empty ToolCalls slice; callers should parse the Text
// field using parseToolCalls for compatibility.
func (p *OllamaProvider) QueryWithTools(ctx context.Context, messages []Message, tools []ToolDefinition) (*ToolResponse, error) {
	// Build a single prompt from the conversation + tool preamble.
	var parts []string
	if len(tools) > 0 {
		parts = append(parts, buildToolPreamble())
	}
	for _, m := range messages {
		switch m.Role {
		case "assistant":
			parts = append(parts, "Assistant: "+m.Content)
		case "tool":
			parts = append(parts, fmt.Sprintf("Tool result (%s): %s", m.ToolName, m.Content))
		default:
			parts = append(parts, m.Content)
		}
	}

	prompt := strings.Join(parts, "\n\n")
	text, err := p.Query(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return &ToolResponse{Text: text}, nil
}

// ---------------------------------------------------------------------------
// Google Gemini tool-use implementation (stub — Gemini function calling)
// ---------------------------------------------------------------------------

// QueryWithTools for Google Gemini uses the function declarations API.
// This is a best-effort implementation; Gemini's tool-calling protocol
// uses a different schema from OpenAI/Anthropic.
func (p *GoogleProvider) QueryWithTools(ctx context.Context, messages []Message, tools []ToolDefinition) (*ToolResponse, error) {
	// Gemini API supports function calling via "tools" array with
	// "functionDeclarations". For now we fall back to text-based tool calling
	// to avoid adding a large Gemini-specific implementation here.
	// TODO: implement native Gemini function declarations in a follow-up.

	var parts []string
	if len(tools) > 0 {
		parts = append(parts, buildToolPreamble())
	}
	for _, m := range messages {
		parts = append(parts, m.Content)
	}

	prompt := strings.Join(parts, "\n\n")
	text, err := p.Query(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return &ToolResponse{Text: text}, nil
}
