// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/thepixelabs/kubecat/internal/ai"
	"github.com/thepixelabs/kubecat/internal/audit"
	"github.com/thepixelabs/kubecat/internal/config"
)

// AIAgentQuery starts an agentic query session using the observe-think-act loop.
// The agent uses tool-calling to autonomously investigate Kubernetes issues and
// emits events to the frontend for each step:
//   - "ai:agent:thinking" — LLM is reasoning
//   - "ai:agent:tool-call" — agent is invoking a tool
//   - "ai:agent:tool-result" — tool returned a result
//   - "ai:agent:approval-needed" — user must approve/reject a write/destructive tool
//   - "ai:agent:complete" — final response ready
//
// The method returns immediately; the agent loop runs in a background goroutine.
// namespace is optional; it scopes guardrail checks to a specific namespace.
func (a *App) AIAgentQuery(query string, conversationID string, namespace string, providerID string, modelID string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if !cfg.Kubecat.AI.Enabled {
		return fmt.Errorf("AI features are not enabled. Please enable them in Settings.")
	}

	selectedProvider := providerID
	if selectedProvider == "" {
		selectedProvider = cfg.Kubecat.AI.SelectedProvider
	}
	if selectedProvider == "" {
		return fmt.Errorf("no AI provider selected")
	}

	selectedModel := modelID
	if selectedModel == "" {
		selectedModel = cfg.Kubecat.AI.SelectedModel
	}

	providerConfig, ok := cfg.Kubecat.AI.Providers[selectedProvider]
	if !ok || !providerConfig.Enabled {
		return fmt.Errorf("AI provider %q is not enabled or configured", selectedProvider)
	}

	providerCfg := ai.ProviderConfig{
		Model:    selectedModel,
		APIKey:   providerConfig.APIKey,
		Endpoint: providerConfig.Endpoint,
		Timeout:  120 * time.Second,
	}

	provider, err := ai.NewProvider(selectedProvider, providerCfg)
	if err != nil {
		return fmt.Errorf("unknown AI provider: %s", selectedProvider)
	}

	// Build guardrails from config.
	guardrails := ai.NewGuardrails(ai.GuardrailsFromConfig(cfg))

	cluster := a.nexus.Clusters.ActiveContext()
	if namespace == "" {
		namespace = "default"
	}

	sanitizedQuery := query
	if ai.IsCloudProvider(selectedProvider) {
		sanitizedQuery = ai.SanitizeForCloud(query)
	}

	audit.LogAIQuery(selectedProvider, cluster, namespace, sanitizedQuery)

	// Register a cancellable session and an approval channel so the frontend
	// can stop the agent or approve/reject tool calls mid-loop.
	sessionID := conversationID
	if sessionID == "" {
		sessionID = fmt.Sprintf("agent_%d", time.Now().UnixNano())
	}

	sessionCtx, cancel := context.WithCancel(a.ctx)
	sess := &agentSession{
		cancel:     cancel,
		approvalCh: make(chan agentApprovalMsg, 1),
	}

	a.agentMu.Lock()
	a.agentSessions[sessionID] = sess
	a.agentMu.Unlock()

	agent := ai.NewAgent(provider, guardrails, a.emitter, a, cluster, namespace).
		WithApproval(a, sessionID)

	// Run the agent loop in a background goroutine so this method returns
	// immediately to the frontend.
	go func() {
		defer func() { _ = provider.Close() }()
		defer func() {
			// Clean up session on completion.
			a.agentMu.Lock()
			delete(a.agentSessions, sessionID)
			a.agentMu.Unlock()
		}()

		start := time.Now()

		resp, err := agent.QueryWithTools(sessionCtx, sanitizedQuery)
		if err != nil {
			slog.Warn("AIAgentQuery: agent loop failed",
				slog.String("conversationId", conversationID),
				slog.Any("error", err))
			if a.emitter != nil {
				a.emitter.Emit("ai:agent:error", map[string]interface{}{
					"conversationId": conversationID,
					"error":          err.Error(),
				})
			}
			return
		}

		slog.Info("AIAgentQuery: completed",
			slog.String("conversationId", conversationID),
			slog.Int("responseBytes", len(resp)),
			slog.Duration("duration", time.Since(start)),
		)
	}()

	return nil
}

// RequestApproval implements ai.ApprovalHandler. It emits an approval-needed
// event (already done by the agent loop before calling this) and blocks until
// the user sends a decision via ApproveAgentAction / RejectAgentAction, or until
// the context is canceled (agent stopped or timed out).
func (a *App) RequestApproval(ctx context.Context, sessionID, toolCallID, toolName string, params map[string]string, category ai.ToolCategory) (ai.ApprovalDecision, error) {
	a.agentMu.Lock()
	sess, ok := a.agentSessions[sessionID]
	a.agentMu.Unlock()

	if !ok {
		return ai.ApprovalDecision{}, fmt.Errorf("agent session %q not found", sessionID)
	}

	// Block until the user responds or the context is canceled.
	select {
	case msg := <-sess.approvalCh:
		if msg.toolCallID != toolCallID {
			// Stale message from a previous call — treat as rejection for safety.
			slog.Warn("agent: received approval for unexpected toolCallID",
				slog.String("expected", toolCallID),
				slog.String("got", msg.toolCallID))
			return ai.ApprovalDecision{Approved: false}, nil
		}
		return ai.ApprovalDecision{Approved: msg.approved}, nil
	case <-ctx.Done():
		return ai.ApprovalDecision{}, ctx.Err()
	}
}

// ApproveAgentAction approves a pending tool call in the given agent session.
func (a *App) ApproveAgentAction(sessionID string, toolCallID string) error {
	return a.sendApprovalDecision(sessionID, toolCallID, true)
}

// RejectAgentAction rejects a pending tool call in the given agent session.
func (a *App) RejectAgentAction(sessionID string, toolCallID string) error {
	return a.sendApprovalDecision(sessionID, toolCallID, false)
}

// sendApprovalDecision is the shared implementation for Approve/Reject.
func (a *App) sendApprovalDecision(sessionID, toolCallID string, approved bool) error {
	a.agentMu.Lock()
	sess, ok := a.agentSessions[sessionID]
	a.agentMu.Unlock()

	if !ok {
		return fmt.Errorf("agent session %q not found or already complete", sessionID)
	}

	// Non-blocking send: if the channel is full (shouldn't happen with buffer=1
	// per pending call), return an error rather than deadlocking.
	select {
	case sess.approvalCh <- agentApprovalMsg{toolCallID: toolCallID, approved: approved}:
		return nil
	default:
		return fmt.Errorf("approval channel busy — another decision may already be pending")
	}
}

// StopAgentSession cancels the context for the given agent session, causing
// the agent loop to exit at the next context-check point.
func (a *App) StopAgentSession(sessionID string) error {
	a.agentMu.Lock()
	sess, ok := a.agentSessions[sessionID]
	a.agentMu.Unlock()

	if !ok {
		return fmt.Errorf("agent session %q not found or already complete", sessionID)
	}

	sess.cancel()
	return nil
}
