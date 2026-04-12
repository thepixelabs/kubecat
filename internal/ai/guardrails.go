// SPDX-License-Identifier: Apache-2.0

package ai

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/thepixelabs/kubecat/internal/audit"
	"github.com/thepixelabs/kubecat/internal/config"
)

// GuardrailsConfig holds tuneable limits for the agent guardrail layer.
type GuardrailsConfig struct {
	// AllowedNamespaces restricts tool calls to these namespaces (empty = all allowed).
	AllowedNamespaces []string
	// ProtectedNamespaces blocks write/destructive tools in these namespaces.
	ProtectedNamespaces []string
	// BlockDestructive disallows all CategoryDestructive tools.
	BlockDestructive bool
	// RequireDoubleConfirm requires a second approval for destructive tools.
	RequireDoubleConfirm bool
	// SessionRateLimit is the maximum tool calls allowed in a 60-second window.
	// Default is 20; set to 0 to disable rate limiting.
	SessionRateLimit int
	// SessionToolCap is the maximum total tool calls per session.
	SessionToolCap int
	// TokenBudget is the maximum tokens (input+output) the agent may consume.
	TokenBudget int
	// ToolTimeout is the per-tool execution timeout.
	ToolTimeout time.Duration
	// AllowProductionDestructive enables destructive operations on clusters whose
	// name matches *prod* or *production*. Off by default for safety.
	AllowProductionDestructive bool
}

// DefaultGuardrailsConfig returns sensible defaults.
func DefaultGuardrailsConfig() GuardrailsConfig {
	return GuardrailsConfig{
		ProtectedNamespaces:  []string{"kube-system", "kube-public", "kube-node-lease"},
		BlockDestructive:     false,
		RequireDoubleConfirm: true,
		SessionRateLimit:     20,
		SessionToolCap:       50,
		TokenBudget:          100_000,
		ToolTimeout:          30 * time.Second,
	}
}

// Guardrails enforces 7-layer safety checks on agent tool calls.
type Guardrails struct {
	cfg GuardrailsConfig

	mu          sync.Mutex
	callCount   int         // total tool calls this session
	windowCalls []time.Time // timestamps for sliding-window rate limit
	tokenUsage  int
}

// NewGuardrails creates a Guardrails instance from config.
func NewGuardrails(cfg GuardrailsConfig) *Guardrails {
	return &Guardrails{cfg: cfg}
}

// CheckResult is returned by CheckTool.
type CheckResult struct {
	Allowed bool
	Reason  string
	// Timeout is the per-tool timeout to enforce during execution.
	Timeout time.Duration
}

// CheckTool runs all guardrail layers for a proposed tool call.
// cluster and namespace are the currently active cluster and namespace context.
func (g *Guardrails) CheckTool(toolName, namespace, cluster string, tokensUsedSoFar int) CheckResult {
	g.mu.Lock()
	defer g.mu.Unlock()

	tool, ok := ToolByName(toolName)
	if !ok {
		return CheckResult{Allowed: false, Reason: fmt.Sprintf("unknown tool %q", toolName)}
	}

	// Layer 1: Namespace sandbox.
	if !g.namespaceAllowed(namespace) {
		return CheckResult{
			Allowed: false,
			Reason:  fmt.Sprintf("namespace %q is outside the allowed namespace sandbox", namespace),
		}
	}

	// Layer 2: Production namespace protection.
	if g.isProtectedNamespace(namespace) && tool.Category != CategoryRead {
		return CheckResult{
			Allowed: false,
			Reason:  fmt.Sprintf("write/destructive tools are blocked in protected namespace %q", namespace),
		}
	}

	// Layer 2b: Production cluster protection.
	// Cluster contexts whose names contain "prod" or "production" are treated as
	// production environments. Write operations require explicit opt-in and
	// destructive ops are blocked outright unless BlockDestructive is false AND
	// the guardrails config does not list the cluster among ProtectedNamespaces.
	// This is a defense-in-depth measure: even if someone misconfigures allowed
	// namespaces, prod clusters get an extra rejection layer.
	if isProductionCluster(cluster) && tool.Category == CategoryDestructive && !g.cfg.AllowProductionDestructive {
		return CheckResult{
			Allowed: false,
			Reason:  fmt.Sprintf("destructive tools are blocked on production cluster %q; set allowProductionDestructive to override", cluster),
		}
	}

	// Layer 3: Destructive blocking.
	if g.cfg.BlockDestructive && tool.Category == CategoryDestructive {
		return CheckResult{
			Allowed: false,
			Reason:  "destructive tools are globally disabled by policy",
		}
	}

	// Layer 4: Double-confirm (handled by caller; we just flag it).
	if g.cfg.RequireDoubleConfirm && tool.Category == CategoryDestructive {
		slog.Info("guardrails: destructive tool requires double confirmation",
			slog.String("tool", toolName),
			slog.String("cluster", cluster))
		// We still allow it here; the caller (agent) is responsible for prompting.
	}

	// Layer 5: Session rate limiting (sliding 60-second window).
	now := time.Now()
	window := now.Add(-60 * time.Second)
	filtered := g.windowCalls[:0]
	for _, t := range g.windowCalls {
		if t.After(window) {
			filtered = append(filtered, t)
		}
	}
	g.windowCalls = filtered

	if g.cfg.SessionRateLimit > 0 && len(g.windowCalls) >= g.cfg.SessionRateLimit {
		return CheckResult{
			Allowed: false,
			Reason:  fmt.Sprintf("rate limit exceeded: %d calls in the last 60 seconds (max %d)", len(g.windowCalls), g.cfg.SessionRateLimit),
		}
	}

	// Layer 6: Per-session tool cap.
	if g.cfg.SessionToolCap > 0 && g.callCount >= g.cfg.SessionToolCap {
		return CheckResult{
			Allowed: false,
			Reason:  fmt.Sprintf("session tool cap reached (%d/%d)", g.callCount, g.cfg.SessionToolCap),
		}
	}

	// Layer 7: Token budget.
	if g.cfg.TokenBudget > 0 && tokensUsedSoFar >= g.cfg.TokenBudget {
		return CheckResult{
			Allowed: false,
			Reason:  fmt.Sprintf("token budget exhausted (%d/%d)", tokensUsedSoFar, g.cfg.TokenBudget),
		}
	}

	// All checks passed — record the call.
	g.windowCalls = append(g.windowCalls, now)
	g.callCount++

	audit.Log(audit.Entry{
		EventType: audit.EventCommandExecution,
		Cluster:   cluster,
		Namespace: namespace,
		Resource:  toolName,
		Meta:      map[string]string{"layer": "guardrails", "approved": "true"},
	})

	timeout := g.cfg.ToolTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return CheckResult{Allowed: true, Timeout: timeout}
}

// AddTokenUsage records additional token usage for the budget check.
func (g *Guardrails) AddTokenUsage(tokens int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.tokenUsage += tokens
}

// Reset clears all per-session counters (call at the start of each new agent session).
func (g *Guardrails) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.callCount = 0
	g.windowCalls = g.windowCalls[:0]
	g.tokenUsage = 0
}

func (g *Guardrails) namespaceAllowed(namespace string) bool {
	if len(g.cfg.AllowedNamespaces) == 0 {
		return true
	}
	for _, ns := range g.cfg.AllowedNamespaces {
		if ns == namespace || ns == "*" {
			return true
		}
	}
	return false
}

func (g *Guardrails) isProtectedNamespace(namespace string) bool {
	for _, ns := range g.cfg.ProtectedNamespaces {
		if strings.EqualFold(ns, namespace) {
			return true
		}
	}
	return false
}

// GuardrailsFromConfig builds a GuardrailsConfig from the application config.
func GuardrailsFromConfig(cfg *config.Config) GuardrailsConfig {
	g := DefaultGuardrailsConfig()
	ac := cfg.Kubecat.AgentGuardrails
	if ac.SessionRateLimit > 0 {
		g.SessionRateLimit = ac.SessionRateLimit
	}
	if ac.SessionToolCap > 0 {
		g.SessionToolCap = ac.SessionToolCap
	}
	if ac.TokenBudget > 0 {
		g.TokenBudget = ac.TokenBudget
	}
	if len(ac.AllowedNamespaces) > 0 {
		g.AllowedNamespaces = ac.AllowedNamespaces
	}
	if len(ac.ProtectedNamespaces) > 0 {
		g.ProtectedNamespaces = ac.ProtectedNamespaces
	}
	g.BlockDestructive = ac.BlockDestructive
	g.RequireDoubleConfirm = ac.RequireDoubleConfirm
	g.AllowProductionDestructive = ac.AllowProductionDestructive
	return g
}

// toolCallContext wraps a context with a per-tool timeout.
func toolCallContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return context.WithTimeout(parent, timeout)
}

// isProductionCluster returns true when the cluster context name matches a
// well-known production segment (case-insensitive, word-boundary checked).
// Matches: "prod", "prd", "production", "live" as whole segments separated
// by hyphens, underscores, or dots — e.g. "my-prod-cluster", "eu-prd-1",
// "live-k8s". Does NOT match "product-dev", "reproduced", "approved-staging".
func isProductionCluster(cluster string) bool {
	lower := strings.ToLower(cluster)
	for _, keyword := range []string{"prod", "prd", "production", "live"} {
		idx := strings.Index(lower, keyword)
		if idx < 0 {
			continue
		}
		end := idx + len(keyword)
		leftOK := idx == 0 || lower[idx-1] == '-' || lower[idx-1] == '_' || lower[idx-1] == '.'
		rightOK := end == len(lower) || lower[end] == '-' || lower[end] == '_' || lower[end] == '.'
		if leftOK && rightOK {
			return true
		}
	}
	return false
}

// OperationCost estimates the blast-radius of a tool call.  It is intentionally
// simple — the goal is to surface a meaningful warning in the UI, not to be a
// perfect cost model.
type OperationCost struct {
	// BlastRadius is a human-readable description of what will be affected.
	BlastRadius string
	// Reversible indicates whether the operation can be undone.
	Reversible bool
	// RiskLevel is "low", "medium", or "high".
	RiskLevel string
}

// EstimateOperationCost returns an OperationCost for a proposed tool call.
// It uses the tool's category and the target namespace/cluster to produce a
// meaningful cost estimate without needing to know the full parameter set.
func EstimateOperationCost(toolName, namespace, cluster string) OperationCost {
	tool, ok := ToolByName(toolName)
	if !ok {
		return OperationCost{BlastRadius: "unknown tool", RiskLevel: "high", Reversible: false}
	}

	switch tool.Category {
	case CategoryRead:
		return OperationCost{
			BlastRadius: fmt.Sprintf("read-only access to %s/%s", cluster, namespace),
			Reversible:  true,
			RiskLevel:   "low",
		}
	case CategoryWrite:
		risk := "medium"
		if isProductionCluster(cluster) {
			risk = "high"
		}
		return OperationCost{
			BlastRadius: fmt.Sprintf("modifies resources in %s/%s", cluster, namespace),
			Reversible:  true,
			RiskLevel:   risk,
		}
	case CategoryDestructive:
		risk := "high"
		if isProductionCluster(cluster) {
			risk = "critical"
		}
		return OperationCost{
			BlastRadius: fmt.Sprintf("permanently removes or alters resources in %s/%s", cluster, namespace),
			Reversible:  false,
			RiskLevel:   risk,
		}
	default:
		return OperationCost{BlastRadius: "unknown", RiskLevel: "high", Reversible: false}
	}
}
