package security

import (
	"time"
)

// Severity represents the severity of a security finding.
type Severity string

const (
	SeverityCritical Severity = "Critical"
	SeverityHigh     Severity = "High"
	SeverityMedium   Severity = "Medium"
	SeverityLow      Severity = "Low"
	SeverityInfo     Severity = "Info"
)

// Category represents the category of a security finding.
type Category string

const (
	CategoryRBAC    Category = "RBAC"
	CategoryPolicy  Category = "Policy"
	CategoryImage   Category = "Image"
	CategoryNetwork Category = "Network"
	CategorySecrets Category = "Secrets"
	CategoryRuntime Category = "Runtime"
)

// SecurityScore represents the overall security score.
type SecurityScore struct {
	Overall    int            `json:"overall"`
	Categories map[string]int `json:"categories"`
	Grade      string         `json:"grade"` // A, B, C, D, F
	ScannedAt  time.Time      `json:"scannedAt"`
}

// SecuritySummary provides an overview of security status.
type SecuritySummary struct {
	Score            SecurityScore    `json:"score"`
	TotalIssues      int              `json:"totalIssues"`
	CriticalCount    int              `json:"criticalCount"`
	HighCount        int              `json:"highCount"`
	MediumCount      int              `json:"mediumCount"`
	LowCount         int              `json:"lowCount"`
	IssuesByCategory map[Category]int `json:"issuesByCategory"`
	TopIssues        []SecurityIssue  `json:"topIssues"`
}

// SecurityIssue represents a security finding.
type SecurityIssue struct {
	ID          string                 `json:"id"`
	Category    Category               `json:"category"`
	Severity    Severity               `json:"severity"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Resource    string                 `json:"resource"`
	Namespace   string                 `json:"namespace"`
	Kind        string                 `json:"kind"`
	Remediation string                 `json:"remediation"`
	Details     map[string]interface{} `json:"details,omitempty"`
	DetectedAt  time.Time              `json:"detectedAt"`
}

// RBACSubject represents a user, group, or service account.
type RBACSubject struct {
	Kind      string `json:"kind"` // User, Group, ServiceAccount
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// RBACPermission represents permissions granted.
type RBACPermission struct {
	Verbs         []string `json:"verbs"`
	Resources     []string `json:"resources"`
	ResourceNames []string `json:"resourceNames,omitempty"`
	APIGroups     []string `json:"apiGroups"`
	Namespace     string   `json:"namespace,omitempty"` // Empty means cluster-wide
}

// RBACBinding represents a role binding.
type RBACBinding struct {
	Name        string           `json:"name"`
	Namespace   string           `json:"namespace,omitempty"`
	RoleName    string           `json:"roleName"`
	RoleKind    string           `json:"roleKind"`
	Subjects    []RBACSubject    `json:"subjects"`
	Permissions []RBACPermission `json:"permissions"`
	IsCluster   bool             `json:"isCluster"`
}

// RBACAnalysis contains RBAC analysis results.
type RBACAnalysis struct {
	Bindings        []RBACBinding       `json:"bindings"`
	SubjectAccess   map[string][]string `json:"subjectAccess"` // subject -> namespaces
	DangerousAccess []DangerousAccess   `json:"dangerousAccess"`
	WildcardAccess  []WildcardAccess    `json:"wildcardAccess"`
}

// DangerousAccess represents a subject with dangerous permissions.
type DangerousAccess struct {
	Subject     RBACSubject `json:"subject"`
	Binding     string      `json:"binding"`
	Reason      string      `json:"reason"`
	Permissions []string    `json:"permissions"`
}

// WildcardAccess represents wildcard permissions.
type WildcardAccess struct {
	Subject   RBACSubject `json:"subject"`
	Binding   string      `json:"binding"`
	Verbs     []string    `json:"verbs"`
	Resources []string    `json:"resources"`
}

// PolicyViolation represents a policy violation.
type PolicyViolation struct {
	Policy     string   `json:"policy"`
	PolicyKind string   `json:"policyKind"` // ConstraintTemplate, ClusterPolicy, etc.
	Severity   Severity `json:"severity"`
	Message    string   `json:"message"`
	Resource   string   `json:"resource"`
	Namespace  string   `json:"namespace"`
	Kind       string   `json:"kind"`
	Action     string   `json:"action"` // deny, warn, audit
}

// PolicySummary contains policy enforcement information.
type PolicySummary struct {
	Provider        string            `json:"provider"` // gatekeeper, kyverno, none
	Policies        []PolicyInfo      `json:"policies"`
	Violations      []PolicyViolation `json:"violations"`
	TotalPolicies   int               `json:"totalPolicies"`
	TotalViolations int               `json:"totalViolations"`
}

// PolicyInfo contains policy metadata.
type PolicyInfo struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Enforcement string   `json:"enforcement"` // deny, dryrun, warn
	Violations  int      `json:"violations"`
	Targets     []string `json:"targets"`
}

// NetworkPolicyAnalysis contains network policy analysis.
type NetworkPolicyAnalysis struct {
	Pod             string              `json:"pod"`
	Namespace       string              `json:"namespace"`
	HasPolicies     bool                `json:"hasPolicies"`
	IngressPolicies []NetworkPolicyRule `json:"ingressPolicies"`
	EgressPolicies  []NetworkPolicyRule `json:"egressPolicies"`
	DefaultDeny     bool                `json:"defaultDeny"`
}

// NetworkPolicyRule represents an effective network policy rule.
type NetworkPolicyRule struct {
	PolicyName string   `json:"policyName"`
	Direction  string   `json:"direction"` // ingress, egress
	Allowed    []string `json:"allowed"`   // Description of what's allowed
	Ports      []string `json:"ports"`
}

// ImageVulnerability represents a CVE in an image.
type ImageVulnerability struct {
	CVE         string   `json:"cve"`
	Severity    Severity `json:"severity"`
	Package     string   `json:"package"`
	Version     string   `json:"version"`
	FixedIn     string   `json:"fixedIn,omitempty"`
	Description string   `json:"description"`
}

// ImageScanResult represents scan results for an image.
type ImageScanResult struct {
	Image           string               `json:"image"`
	Digest          string               `json:"digest,omitempty"`
	Critical        int                  `json:"critical"`
	High            int                  `json:"high"`
	Medium          int                  `json:"medium"`
	Low             int                  `json:"low"`
	Vulnerabilities []ImageVulnerability `json:"vulnerabilities,omitempty"`
	ScannedAt       time.Time            `json:"scannedAt"`
}
