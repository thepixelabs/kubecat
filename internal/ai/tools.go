package ai

// ApprovalPolicy controls whether a tool call requires user confirmation.
type ApprovalPolicy int

const (
	// ApprovalNever — tool is read-only and never needs approval.
	ApprovalNever ApprovalPolicy = iota
	// ApprovalOnce — tool should be approved once per session.
	ApprovalOnce
	// ApprovalAlways — tool must be approved before every invocation.
	ApprovalAlways
)

// ToolCategory classifies the blast-radius of a tool.
type ToolCategory string

const (
	CategoryRead        ToolCategory = "read"
	CategoryWrite       ToolCategory = "write"
	CategoryDestructive ToolCategory = "destructive"
)

// ParameterSchema describes a single parameter accepted by a tool.
type ParameterSchema struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// ToolDefinition describes a single tool the AI agent can invoke.
type ToolDefinition struct {
	// Name is the canonical snake_case tool name sent to the LLM.
	Name string `json:"name"`
	// Description is sent verbatim to the LLM as the tool description.
	Description string `json:"description"`
	// Category classifies the tool's blast-radius.
	Category ToolCategory `json:"category"`
	// ApprovalPolicy controls whether the user must confirm execution.
	ApprovalPolicy ApprovalPolicy `json:"approvalPolicy"`
	// Parameters lists accepted input parameters.
	Parameters []ParameterSchema `json:"parameters"`
	// BackendMethod is the Go method name on App that implements this tool.
	// It is excluded from the JSON sent to the LLM.
	BackendMethod string `json:"-"`
}

// Registry is the ordered list of all tools available to the AI agent.
// Tools are alphabetical within each category (read, write, destructive).
var Registry = []ToolDefinition{
	// ---- Read tools ----
	{
		Name:           "describe_resource",
		Description:    "Describe a Kubernetes resource and return its full YAML spec.",
		Category:       CategoryRead,
		ApprovalPolicy: ApprovalNever,
		BackendMethod:  "AgentDescribeResource",
		Parameters: []ParameterSchema{
			{Name: "kind", Type: "string", Description: "Resource kind, e.g. Pod, Deployment", Required: true},
			{Name: "namespace", Type: "string", Description: "Namespace (empty for cluster-scoped)", Required: false},
			{Name: "name", Type: "string", Description: "Resource name", Required: true},
		},
	},
	{
		Name:           "get_events",
		Description:    "List recent Kubernetes events for a namespace or specific resource.",
		Category:       CategoryRead,
		ApprovalPolicy: ApprovalNever,
		BackendMethod:  "AgentGetEvents",
		Parameters: []ParameterSchema{
			{Name: "namespace", Type: "string", Description: "Namespace to scope events", Required: false},
			{Name: "kind", Type: "string", Description: "Filter by resource kind", Required: false},
			{Name: "name", Type: "string", Description: "Filter by resource name", Required: false},
		},
	},
	{
		Name:           "get_pod_logs",
		Description:    "Retrieve recent log lines from a pod container.",
		Category:       CategoryRead,
		ApprovalPolicy: ApprovalNever,
		BackendMethod:  "AgentGetPodLogs",
		Parameters: []ParameterSchema{
			{Name: "namespace", Type: "string", Description: "Pod namespace", Required: true},
			{Name: "pod", Type: "string", Description: "Pod name", Required: true},
			{Name: "container", Type: "string", Description: "Container name (optional)", Required: false},
			{Name: "tail_lines", Type: "integer", Description: "Number of log lines to return (default 100)", Required: false},
		},
	},
	{
		Name:           "get_resource_yaml",
		Description:    "Return the YAML of a Kubernetes resource with managedFields stripped.",
		Category:       CategoryRead,
		ApprovalPolicy: ApprovalNever,
		BackendMethod:  "AgentGetResourceYAML",
		Parameters: []ParameterSchema{
			{Name: "kind", Type: "string", Description: "Resource kind", Required: true},
			{Name: "namespace", Type: "string", Description: "Namespace", Required: false},
			{Name: "name", Type: "string", Description: "Resource name", Required: true},
		},
	},
	{
		Name:           "list_resources",
		Description:    "List Kubernetes resources of a given kind, optionally filtered by namespace.",
		Category:       CategoryRead,
		ApprovalPolicy: ApprovalNever,
		BackendMethod:  "AgentListResources",
		Parameters: []ParameterSchema{
			{Name: "kind", Type: "string", Description: "Resource kind, e.g. pods, deployments", Required: true},
			{Name: "namespace", Type: "string", Description: "Namespace (empty for all namespaces)", Required: false},
		},
	},
	{
		Name:           "get_security_summary",
		Description:    "Run a security scan and return a summary of issues.",
		Category:       CategoryRead,
		ApprovalPolicy: ApprovalNever,
		BackendMethod:  "AgentGetSecuritySummary",
		Parameters: []ParameterSchema{
			{Name: "namespace", Type: "string", Description: "Namespace to scan (empty for all)", Required: false},
		},
	},
	// ---- Write tools ----
	{
		Name:           "apply_manifest",
		Description:    "Apply a Kubernetes manifest (kubectl apply equivalent).",
		Category:       CategoryWrite,
		ApprovalPolicy: ApprovalAlways,
		BackendMethod:  "AgentApplyManifest",
		Parameters: []ParameterSchema{
			{Name: "manifest", Type: "string", Description: "YAML manifest to apply", Required: true},
		},
	},
	{
		Name:           "restart_deployment",
		Description:    "Perform a rolling restart of a Deployment.",
		Category:       CategoryWrite,
		ApprovalPolicy: ApprovalAlways,
		BackendMethod:  "AgentRestartDeployment",
		Parameters: []ParameterSchema{
			{Name: "namespace", Type: "string", Description: "Deployment namespace", Required: true},
			{Name: "name", Type: "string", Description: "Deployment name", Required: true},
		},
	},
	{
		Name:           "scale_deployment",
		Description:    "Scale a Deployment to a specified replica count.",
		Category:       CategoryWrite,
		ApprovalPolicy: ApprovalAlways,
		BackendMethod:  "AgentScaleDeployment",
		Parameters: []ParameterSchema{
			{Name: "namespace", Type: "string", Description: "Deployment namespace", Required: true},
			{Name: "name", Type: "string", Description: "Deployment name", Required: true},
			{Name: "replicas", Type: "integer", Description: "Desired replica count", Required: true},
		},
	},
	// ---- Destructive tools ----
	{
		Name:           "delete_resource",
		Description:    "Delete a Kubernetes resource. This is irreversible.",
		Category:       CategoryDestructive,
		ApprovalPolicy: ApprovalAlways,
		BackendMethod:  "AgentDeleteResource",
		Parameters: []ParameterSchema{
			{Name: "kind", Type: "string", Description: "Resource kind", Required: true},
			{Name: "namespace", Type: "string", Description: "Namespace", Required: false},
			{Name: "name", Type: "string", Description: "Resource name", Required: true},
		},
	},
	{
		Name:           "exec_command",
		Description:    "Run an allowlisted command (kubectl/helm/flux/argocd) on the local host. NOTE: this does NOT exec into a container — use get_pod_logs or describe_resource for pod inspection.",
		Category:       CategoryDestructive,
		ApprovalPolicy: ApprovalAlways,
		BackendMethod:  "AgentExecCommand",
		Parameters: []ParameterSchema{
			{Name: "namespace", Type: "string", Description: "Target namespace (informational; scopes the command context)", Required: false},
			{Name: "pod", Type: "string", Description: "Target pod (informational; include in the command string if needed)", Required: false},
			{Name: "container", Type: "string", Description: "Target container (informational)", Required: false},
			{Name: "command", Type: "string", Description: "Full command string, e.g. 'kubectl get pods -n default'", Required: true},
		},
	},
	{
		Name:           "patch_resource",
		Description:    "Apply a strategic merge patch to a Kubernetes resource.",
		Category:       CategoryDestructive,
		ApprovalPolicy: ApprovalAlways,
		BackendMethod:  "AgentPatchResource",
		Parameters: []ParameterSchema{
			{Name: "kind", Type: "string", Description: "Resource kind", Required: true},
			{Name: "namespace", Type: "string", Description: "Namespace", Required: false},
			{Name: "name", Type: "string", Description: "Resource name", Required: true},
			{Name: "patch", Type: "string", Description: "JSON/YAML strategic merge patch", Required: true},
		},
	},
}

// ToolByName returns the ToolDefinition with the given name, or (zero, false).
func ToolByName(name string) (ToolDefinition, bool) {
	for _, t := range Registry {
		if t.Name == name {
			return t, true
		}
	}
	return ToolDefinition{}, false
}
