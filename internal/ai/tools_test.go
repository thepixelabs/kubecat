package ai

import (
	"testing"
)

// ---------------------------------------------------------------------------
// No duplicate names
// ---------------------------------------------------------------------------

func TestRegistry_NoDuplicateNames(t *testing.T) {
	seen := make(map[string]int)
	for _, tool := range Registry {
		seen[tool.Name]++
	}
	for name, count := range seen {
		if count > 1 {
			t.Errorf("tool name %q appears %d times in Registry (must be unique)", name, count)
		}
	}
}

// ---------------------------------------------------------------------------
// Approval / category consistency
// ---------------------------------------------------------------------------

func TestRegistry_ReadToolsNeverRequireApproval(t *testing.T) {
	for _, tool := range Registry {
		if tool.Category == CategoryRead && tool.ApprovalPolicy != ApprovalNever {
			t.Errorf("read tool %q has ApprovalPolicy %v; read tools must use ApprovalNever",
				tool.Name, tool.ApprovalPolicy)
		}
	}
}

func TestRegistry_WriteToolsRequireApproval(t *testing.T) {
	for _, tool := range Registry {
		if tool.Category == CategoryWrite && tool.ApprovalPolicy == ApprovalNever {
			t.Errorf("write tool %q has ApprovalPolicy=ApprovalNever; write tools must require approval",
				tool.Name)
		}
	}
}

func TestRegistry_DestructiveToolsAlwaysRequireApproval(t *testing.T) {
	for _, tool := range Registry {
		if tool.Category == CategoryDestructive && tool.ApprovalPolicy != ApprovalAlways {
			t.Errorf("destructive tool %q has ApprovalPolicy %v; destructive tools must use ApprovalAlways",
				tool.Name, tool.ApprovalPolicy)
		}
	}
}

// ---------------------------------------------------------------------------
// Required field symmetry
// ---------------------------------------------------------------------------

func TestRegistry_AllToolsHaveNonEmptyName(t *testing.T) {
	for i, tool := range Registry {
		if tool.Name == "" {
			t.Errorf("Registry[%d] has empty Name", i)
		}
	}
}

func TestRegistry_AllToolsHaveNonEmptyDescription(t *testing.T) {
	for _, tool := range Registry {
		if tool.Description == "" {
			t.Errorf("tool %q has empty Description", tool.Name)
		}
	}
}

func TestRegistry_AllToolsHaveNonEmptyBackendMethod(t *testing.T) {
	for _, tool := range Registry {
		if tool.BackendMethod == "" {
			t.Errorf("tool %q has empty BackendMethod", tool.Name)
		}
	}
}

func TestRegistry_AllToolsHaveValidCategory(t *testing.T) {
	valid := map[ToolCategory]bool{
		CategoryRead:        true,
		CategoryWrite:       true,
		CategoryDestructive: true,
	}
	for _, tool := range Registry {
		if !valid[tool.Category] {
			t.Errorf("tool %q has unknown category %q", tool.Name, tool.Category)
		}
	}
}

func TestRegistry_RequiredParametersAreNamed(t *testing.T) {
	for _, tool := range Registry {
		for _, p := range tool.Parameters {
			if p.Required && p.Name == "" {
				t.Errorf("tool %q has a required parameter with empty Name", tool.Name)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// All 12 tools present (exact count guard)
// ---------------------------------------------------------------------------

func TestRegistry_ExactlyTwelveTools(t *testing.T) {
	const expectedCount = 12
	got := len(Registry)
	if got != expectedCount {
		names := make([]string, len(Registry))
		for i, t := range Registry {
			names[i] = t.Name
		}
		t.Errorf("Registry has %d tools, want %d. Current tools: %v", got, expectedCount, names)
	}
}

// ---------------------------------------------------------------------------
// ToolByName
// ---------------------------------------------------------------------------

func TestToolByName_Found(t *testing.T) {
	for _, tool := range Registry {
		got, ok := ToolByName(tool.Name)
		if !ok {
			t.Errorf("ToolByName(%q) returned not-found, want found", tool.Name)
			continue
		}
		if got.Name != tool.Name {
			t.Errorf("ToolByName(%q).Name = %q, want %q", tool.Name, got.Name, tool.Name)
		}
	}
}

func TestToolByName_NotFound(t *testing.T) {
	_, ok := ToolByName("nonexistent_tool_xyz")
	if ok {
		t.Error("ToolByName(nonexistent) returned ok=true, want false")
	}
}

func TestToolByName_EmptyName_NotFound(t *testing.T) {
	_, ok := ToolByName("")
	if ok {
		t.Error("ToolByName(\"\") returned ok=true, want false")
	}
}
