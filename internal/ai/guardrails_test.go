package ai

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestGuardrails(cfg GuardrailsConfig) *Guardrails {
	return NewGuardrails(cfg)
}

func permissiveCfg() GuardrailsConfig {
	return GuardrailsConfig{
		ProtectedNamespaces:        []string{},
		BlockDestructive:           false,
		RequireDoubleConfirm:       false,
		SessionRateLimit:           100,
		SessionToolCap:             100,
		TokenBudget:                1_000_000,
		ToolTimeout:                5 * time.Second,
		AllowProductionDestructive: true, // tests explicitly opt in to bypass prod guard
	}
}

// ---------------------------------------------------------------------------
// DefaultGuardrailsConfig
// ---------------------------------------------------------------------------

func TestDefaultGuardrailsConfig_SensibleDefaults(t *testing.T) {
	cfg := DefaultGuardrailsConfig()

	if len(cfg.ProtectedNamespaces) == 0 {
		t.Error("DefaultGuardrailsConfig: ProtectedNamespaces should not be empty")
	}
	if cfg.SessionRateLimit <= 0 {
		t.Errorf("DefaultGuardrailsConfig.SessionRateLimit = %d, want > 0", cfg.SessionRateLimit)
	}
	if cfg.SessionToolCap <= 0 {
		t.Errorf("DefaultGuardrailsConfig.SessionToolCap = %d, want > 0", cfg.SessionToolCap)
	}
	if cfg.TokenBudget <= 0 {
		t.Errorf("DefaultGuardrailsConfig.TokenBudget = %d, want > 0", cfg.TokenBudget)
	}
	if cfg.ToolTimeout <= 0 {
		t.Errorf("DefaultGuardrailsConfig.ToolTimeout = %v, want > 0", cfg.ToolTimeout)
	}
}

// ---------------------------------------------------------------------------
// Layer 1: Namespace sandbox
// ---------------------------------------------------------------------------

func TestCheckTool_AllowedNamespaces_AllowsMatching(t *testing.T) {
	cfg := permissiveCfg()
	cfg.AllowedNamespaces = []string{"default", "staging"}
	g := newTestGuardrails(cfg)

	result := g.CheckTool("list_resources", "default", "test-cluster", 0)
	if !result.Allowed {
		t.Errorf("CheckTool in allowed namespace should be allowed, got reason: %s", result.Reason)
	}
}

func TestCheckTool_AllowedNamespaces_BlocksNonMatching(t *testing.T) {
	cfg := permissiveCfg()
	cfg.AllowedNamespaces = []string{"default"}
	g := newTestGuardrails(cfg)

	result := g.CheckTool("list_resources", "production", "test-cluster", 0)
	if result.Allowed {
		t.Error("CheckTool in disallowed namespace should be blocked")
	}
}

func TestCheckTool_EmptyAllowedNamespaces_AllowsAll(t *testing.T) {
	cfg := permissiveCfg()
	cfg.AllowedNamespaces = []string{} // empty = all allowed
	g := newTestGuardrails(cfg)

	result := g.CheckTool("list_resources", "production", "test-cluster", 0)
	if !result.Allowed {
		t.Errorf("empty AllowedNamespaces should allow all namespaces, got reason: %s", result.Reason)
	}
}

func TestCheckTool_AllowedNamespaces_WildcardAllowsAll(t *testing.T) {
	cfg := permissiveCfg()
	cfg.AllowedNamespaces = []string{"*"}
	g := newTestGuardrails(cfg)

	result := g.CheckTool("list_resources", "anything", "test-cluster", 0)
	if !result.Allowed {
		t.Errorf("wildcard AllowedNamespaces should allow all, got: %s", result.Reason)
	}
}

// ---------------------------------------------------------------------------
// Layer 2: Protected namespace
// ---------------------------------------------------------------------------

func TestCheckTool_ProtectedNamespace_BlocksWrite(t *testing.T) {
	cfg := permissiveCfg()
	cfg.ProtectedNamespaces = []string{"kube-system"}
	g := newTestGuardrails(cfg)

	result := g.CheckTool("delete_resource", "kube-system", "prod", 0)
	if result.Allowed {
		t.Error("write/destructive tool must be blocked in protected namespace")
	}
}

func TestCheckTool_ProtectedNamespace_AllowsRead(t *testing.T) {
	cfg := permissiveCfg()
	cfg.ProtectedNamespaces = []string{"kube-system"}
	g := newTestGuardrails(cfg)

	result := g.CheckTool("list_resources", "kube-system", "prod", 0)
	if !result.Allowed {
		t.Errorf("read tool should be allowed in protected namespace, got: %s", result.Reason)
	}
}

func TestCheckTool_ProtectedNamespace_CaseInsensitive(t *testing.T) {
	cfg := permissiveCfg()
	cfg.ProtectedNamespaces = []string{"kube-system"}
	g := newTestGuardrails(cfg)

	result := g.CheckTool("delete_resource", "KUBE-SYSTEM", "prod", 0)
	if result.Allowed {
		t.Error("protected namespace check should be case-insensitive")
	}
}

// ---------------------------------------------------------------------------
// Layer 3: BlockDestructive
// ---------------------------------------------------------------------------

func TestCheckTool_BlockDestructive_BlocksDestructiveTools(t *testing.T) {
	cfg := permissiveCfg()
	cfg.BlockDestructive = true
	g := newTestGuardrails(cfg)

	result := g.CheckTool("delete_resource", "default", "prod", 0)
	if result.Allowed {
		t.Error("BlockDestructive=true must block destructive tools")
	}
}

func TestCheckTool_BlockDestructive_AllowsWriteTools(t *testing.T) {
	cfg := permissiveCfg()
	cfg.BlockDestructive = true
	g := newTestGuardrails(cfg)

	result := g.CheckTool("apply_manifest", "default", "prod", 0)
	if !result.Allowed {
		t.Errorf("BlockDestructive=true should not block write tools, got: %s", result.Reason)
	}
}

func TestCheckTool_BlockDestructiveDisabled_AllowsDestructive(t *testing.T) {
	cfg := permissiveCfg()
	cfg.BlockDestructive = false
	g := newTestGuardrails(cfg)

	result := g.CheckTool("delete_resource", "default", "prod", 0)
	if !result.Allowed {
		t.Errorf("BlockDestructive=false should allow destructive tools, got: %s", result.Reason)
	}
}

// ---------------------------------------------------------------------------
// Layer 5: Rate limit
// ---------------------------------------------------------------------------

func TestCheckTool_RateLimit_BlocksWhenExceeded(t *testing.T) {
	cfg := permissiveCfg()
	cfg.SessionRateLimit = 3
	cfg.SessionToolCap = 100
	g := newTestGuardrails(cfg)

	// Make 3 allowed calls
	for i := 0; i < 3; i++ {
		r := g.CheckTool("list_resources", "default", "cluster", 0)
		if !r.Allowed {
			t.Fatalf("call %d should be allowed, got: %s", i+1, r.Reason)
		}
	}

	// 4th call must be blocked
	r := g.CheckTool("list_resources", "default", "cluster", 0)
	if r.Allowed {
		t.Error("4th call should be blocked by rate limit")
	}
}

func TestCheckTool_RateLimitZero_Unlimited(t *testing.T) {
	cfg := permissiveCfg()
	cfg.SessionRateLimit = 0 // disabled
	g := newTestGuardrails(cfg)

	for i := 0; i < 10; i++ {
		r := g.CheckTool("list_resources", "default", "cluster", 0)
		if !r.Allowed {
			t.Errorf("call %d should be allowed with rate limit disabled, got: %s", i+1, r.Reason)
		}
	}
}

// ---------------------------------------------------------------------------
// Layer 6: Session tool cap
// ---------------------------------------------------------------------------

func TestCheckTool_SessionToolCap_BlocksWhenExceeded(t *testing.T) {
	cfg := permissiveCfg()
	cfg.SessionToolCap = 2
	cfg.SessionRateLimit = 0 // disable rate limit so we isolate cap
	g := newTestGuardrails(cfg)

	for i := 0; i < 2; i++ {
		r := g.CheckTool("list_resources", "default", "cluster", 0)
		if !r.Allowed {
			t.Fatalf("call %d should be allowed under cap, got: %s", i+1, r.Reason)
		}
	}

	r := g.CheckTool("list_resources", "default", "cluster", 0)
	if r.Allowed {
		t.Error("3rd call should be blocked by session tool cap")
	}
}

// ---------------------------------------------------------------------------
// Layer 7: Token budget
// ---------------------------------------------------------------------------

func TestCheckTool_TokenBudget_BlocksWhenExceeded(t *testing.T) {
	cfg := permissiveCfg()
	cfg.TokenBudget = 1000
	g := newTestGuardrails(cfg)

	// Call with tokens already at budget
	r := g.CheckTool("list_resources", "default", "cluster", 1000)
	if r.Allowed {
		t.Error("call at token budget should be blocked")
	}
}

func TestCheckTool_TokenBudgetZero_Unlimited(t *testing.T) {
	cfg := permissiveCfg()
	cfg.TokenBudget = 0 // disabled
	g := newTestGuardrails(cfg)

	r := g.CheckTool("list_resources", "default", "cluster", 999_999_999)
	if !r.Allowed {
		t.Errorf("zero token budget should not block, got: %s", r.Reason)
	}
}

// ---------------------------------------------------------------------------
// Unknown tool
// ---------------------------------------------------------------------------

func TestCheckTool_UnknownTool_Blocked(t *testing.T) {
	g := newTestGuardrails(permissiveCfg())

	r := g.CheckTool("nonexistent_tool", "default", "cluster", 0)
	if r.Allowed {
		t.Error("unknown tool should be blocked")
	}
}

// ---------------------------------------------------------------------------
// Reset
// ---------------------------------------------------------------------------

func TestGuardrails_Reset_ClearsCounters(t *testing.T) {
	cfg := permissiveCfg()
	cfg.SessionToolCap = 2
	cfg.SessionRateLimit = 0
	g := newTestGuardrails(cfg)

	for i := 0; i < 2; i++ {
		g.CheckTool("list_resources", "default", "cluster", 0)
	}

	// Now blocked
	if r := g.CheckTool("list_resources", "default", "cluster", 0); r.Allowed {
		t.Fatal("precondition: should be blocked before Reset")
	}

	g.Reset()

	// After reset, allowed again
	r := g.CheckTool("list_resources", "default", "cluster", 0)
	if !r.Allowed {
		t.Errorf("should be allowed after Reset, got: %s", r.Reason)
	}
}

// ---------------------------------------------------------------------------
// ToolTimeout propagated in result
// ---------------------------------------------------------------------------

func TestCheckTool_TimeoutPropagated(t *testing.T) {
	cfg := permissiveCfg()
	cfg.ToolTimeout = 15 * time.Second
	g := newTestGuardrails(cfg)

	r := g.CheckTool("list_resources", "default", "cluster", 0)
	if !r.Allowed {
		t.Fatalf("should be allowed: %s", r.Reason)
	}
	if r.Timeout != 15*time.Second {
		t.Errorf("CheckResult.Timeout = %v, want 15s", r.Timeout)
	}
}

// ---------------------------------------------------------------------------
// namespaceAllowed / isProtectedNamespace (indirectly via CheckTool)
// ---------------------------------------------------------------------------

func TestCheckTool_MultipleAllowedNamespaces(t *testing.T) {
	cfg := permissiveCfg()
	cfg.AllowedNamespaces = []string{"ns-a", "ns-b", "ns-c"}
	g := newTestGuardrails(cfg)

	for _, ns := range []string{"ns-a", "ns-b", "ns-c"} {
		r := g.CheckTool("list_resources", ns, "cluster", 0)
		if !r.Allowed {
			t.Errorf("namespace %q should be allowed, got: %s", ns, r.Reason)
		}
	}

	r := g.CheckTool("list_resources", "ns-d", "cluster", 0)
	if r.Allowed {
		t.Error("namespace ns-d should be blocked")
	}
}

// ---------------------------------------------------------------------------
// Layer 2b: Production cluster protection
// ---------------------------------------------------------------------------

func TestCheckTool_ProductionCluster_BlocksDestructive(t *testing.T) {
	cfg := permissiveCfg()
	cfg.AllowProductionDestructive = false // explicitly disable
	g := newTestGuardrails(cfg)

	r := g.CheckTool("delete_resource", "default", "prod-cluster", 0)
	if r.Allowed {
		t.Error("destructive tool should be blocked on production cluster")
	}
	if r.Reason == "" {
		t.Error("blocked result should have a reason")
	}
}

func TestCheckTool_ProductionCluster_AllowsWrite(t *testing.T) {
	cfg := permissiveCfg()
	cfg.AllowProductionDestructive = false
	g := newTestGuardrails(cfg)

	// Write tools (not destructive) are allowed on prod clusters even without override.
	r := g.CheckTool("scale_deployment", "default", "prod-cluster", 0)
	if !r.Allowed {
		t.Errorf("write tool should be allowed on prod cluster, got: %s", r.Reason)
	}
}

func TestCheckTool_ProductionCluster_AllowedWithOverride(t *testing.T) {
	cfg := permissiveCfg()
	cfg.AllowProductionDestructive = true
	g := newTestGuardrails(cfg)

	r := g.CheckTool("delete_resource", "default", "my-production-k8s", 0)
	if !r.Allowed {
		t.Errorf("destructive tool should be allowed on prod cluster with override, got: %s", r.Reason)
	}
}

func TestCheckTool_NonProductionCluster_AllowsDestructive(t *testing.T) {
	cfg := permissiveCfg()
	cfg.AllowProductionDestructive = false
	g := newTestGuardrails(cfg)

	r := g.CheckTool("delete_resource", "default", "dev-cluster", 0)
	if !r.Allowed {
		t.Errorf("destructive tool should be allowed on non-prod cluster, got: %s", r.Reason)
	}
}

// ---------------------------------------------------------------------------
// EstimateOperationCost
// ---------------------------------------------------------------------------

func TestEstimateOperationCost_Read_LowRisk(t *testing.T) {
	cost := EstimateOperationCost("list_resources", "default", "dev-cluster")
	if cost.RiskLevel != "low" {
		t.Errorf("read tool should be low risk, got %q", cost.RiskLevel)
	}
	if !cost.Reversible {
		t.Error("read tool should be reversible")
	}
}

func TestEstimateOperationCost_DestructiveOnProd_CriticalRisk(t *testing.T) {
	cost := EstimateOperationCost("delete_resource", "default", "production-cluster")
	if cost.RiskLevel != "critical" {
		t.Errorf("destructive tool on prod should be critical risk, got %q", cost.RiskLevel)
	}
	if cost.Reversible {
		t.Error("destructive tool should not be reversible")
	}
}

func TestEstimateOperationCost_WriteOnNonProd_MediumRisk(t *testing.T) {
	cost := EstimateOperationCost("scale_deployment", "default", "dev-cluster")
	if cost.RiskLevel != "medium" {
		t.Errorf("write tool on non-prod should be medium risk, got %q", cost.RiskLevel)
	}
	if !cost.Reversible {
		t.Error("write tool should be reversible")
	}
}

func TestEstimateOperationCost_UnknownTool(t *testing.T) {
	cost := EstimateOperationCost("nonexistent_tool", "default", "cluster")
	if cost.RiskLevel != "high" {
		t.Errorf("unknown tool should be high risk, got %q", cost.RiskLevel)
	}
}
