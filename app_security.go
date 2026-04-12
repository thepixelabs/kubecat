// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/thepixelabs/kubecat/internal/ai"
	"github.com/thepixelabs/kubecat/internal/audit"
	"github.com/thepixelabs/kubecat/internal/client"
	"github.com/thepixelabs/kubecat/internal/config"
)

// RBACSubject represents a subject (user, group, or service account).
type RBACSubject struct {
	Kind      string `json:"kind"` // User, Group, ServiceAccount
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"` // Only for ServiceAccount
}

// RBACPermission represents a permission rule.
type RBACPermission struct {
	Verbs         []string `json:"verbs"`
	Resources     []string `json:"resources"`
	ResourceNames []string `json:"resourceNames,omitempty"`
	APIGroups     []string `json:"apiGroups"`
}

// RBACBinding represents a role binding with its subjects and permissions.
type RBACBinding struct {
	Name        string           `json:"name"`
	Namespace   string           `json:"namespace,omitempty"` // Empty for ClusterRoleBinding
	RoleName    string           `json:"roleName"`
	RoleKind    string           `json:"roleKind"` // Role or ClusterRole
	Subjects    []RBACSubject    `json:"subjects"`
	Permissions []RBACPermission `json:"permissions"`
	IsCluster   bool             `json:"isCluster"`
}

// RBACSummary contains the full RBAC analysis.
type RBACSummary struct {
	Bindings        []RBACBinding         `json:"bindings"`
	SubjectSummary  map[string][]string   `json:"subjectSummary"`  // subject -> namespaces they have access to
	DangerousAccess []DangerousAccessInfo `json:"dangerousAccess"` // subjects with dangerous permissions
}

// DangerousAccessInfo highlights potentially dangerous permissions.
type DangerousAccessInfo struct {
	Subject     RBACSubject `json:"subject"`
	Reason      string      `json:"reason"`
	Binding     string      `json:"binding"`
	Namespace   string      `json:"namespace,omitempty"`
	Permissions []string    `json:"permissions"`
}

// SecuritySummaryInfo is a JSON-friendly security summary.
type SecuritySummaryInfo struct {
	Score            SecurityScoreInfo   `json:"score"`
	TotalIssues      int                 `json:"totalIssues"`
	CriticalCount    int                 `json:"criticalCount"`
	HighCount        int                 `json:"highCount"`
	MediumCount      int                 `json:"mediumCount"`
	LowCount         int                 `json:"lowCount"`
	IssuesByCategory map[string]int      `json:"issuesByCategory"`
	TopIssues        []SecurityIssueInfo `json:"topIssues"`
}

// SecurityScoreInfo contains the security score.
type SecurityScoreInfo struct {
	Overall    int            `json:"overall"`
	Grade      string         `json:"grade"`
	Categories map[string]int `json:"categories"`
	ScannedAt  string         `json:"scannedAt"`
}

// SecurityIssueInfo is a JSON-friendly security issue.
type SecurityIssueInfo struct {
	ID          string                 `json:"id"`
	Category    string                 `json:"category"`
	Severity    string                 `json:"severity"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Resource    string                 `json:"resource"`
	Namespace   string                 `json:"namespace"`
	Kind        string                 `json:"kind"`
	Remediation string                 `json:"remediation"`
	Details     map[string]interface{} `json:"details,omitempty"`
	DetectedAt  string                 `json:"detectedAt"`
}

// GetRBACAnalysis returns a comprehensive RBAC analysis.
func (a *App) GetRBACAnalysis() (*RBACSummary, error) {
	cl, err := a.nexus.Clusters.Manager().Active()
	if err != nil {
		return nil, fmt.Errorf("no active cluster: %w", err)
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	summary := &RBACSummary{
		Bindings:       make([]RBACBinding, 0),
		SubjectSummary: make(map[string][]string),
	}

	// Get all ClusterRoleBindings
	crbList, err := cl.List(ctx, "clusterrolebindings", client.ListOptions{Limit: 1000})
	if err == nil {
		for _, crb := range crbList.Items {
			binding := a.parseClusterRoleBinding(ctx, cl, crb)
			if binding != nil {
				summary.Bindings = append(summary.Bindings, *binding)
				a.checkDangerousAccess(summary, binding)
			}
		}
	}

	// Get all RoleBindings
	rbList, err := cl.List(ctx, "rolebindings", client.ListOptions{Limit: 1000})
	if err == nil {
		for _, rb := range rbList.Items {
			binding := a.parseRoleBinding(ctx, cl, rb)
			if binding != nil {
				summary.Bindings = append(summary.Bindings, *binding)
				a.checkDangerousAccess(summary, binding)
			}
		}
	}

	// Build subject summary
	for _, binding := range summary.Bindings {
		for _, subject := range binding.Subjects {
			key := fmt.Sprintf("%s:%s", subject.Kind, subject.Name)
			if subject.Namespace != "" {
				key = fmt.Sprintf("%s:%s/%s", subject.Kind, subject.Namespace, subject.Name)
			}
			ns := binding.Namespace
			if ns == "" {
				ns = "*" // cluster-wide
			}
			summary.SubjectSummary[key] = appendUnique(summary.SubjectSummary[key], ns)
		}
	}

	return summary, nil
}

func (a *App) parseClusterRoleBinding(ctx context.Context, cl client.ClusterClient, resource client.Resource) *RBACBinding {
	var crb struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		RoleRef struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"roleRef"`
		Subjects []struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"subjects"`
	}

	if err := json.Unmarshal(resource.Raw, &crb); err != nil {
		return nil
	}

	binding := &RBACBinding{
		Name:      crb.Metadata.Name,
		RoleName:  crb.RoleRef.Name,
		RoleKind:  crb.RoleRef.Kind,
		IsCluster: true,
		Subjects:  make([]RBACSubject, 0),
	}

	for _, s := range crb.Subjects {
		binding.Subjects = append(binding.Subjects, RBACSubject{
			Kind:      s.Kind,
			Name:      s.Name,
			Namespace: s.Namespace,
		})
	}

	// Get permissions from the role
	binding.Permissions = a.getRolePermissions(ctx, cl, crb.RoleRef.Kind, "", crb.RoleRef.Name)

	return binding
}

func (a *App) parseRoleBinding(ctx context.Context, cl client.ClusterClient, resource client.Resource) *RBACBinding {
	var rb struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		RoleRef struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"roleRef"`
		Subjects []struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"subjects"`
	}

	if err := json.Unmarshal(resource.Raw, &rb); err != nil {
		return nil
	}

	binding := &RBACBinding{
		Name:      rb.Metadata.Name,
		Namespace: rb.Metadata.Namespace,
		RoleName:  rb.RoleRef.Name,
		RoleKind:  rb.RoleRef.Kind,
		IsCluster: false,
		Subjects:  make([]RBACSubject, 0),
	}

	for _, s := range rb.Subjects {
		binding.Subjects = append(binding.Subjects, RBACSubject{
			Kind:      s.Kind,
			Name:      s.Name,
			Namespace: s.Namespace,
		})
	}

	// Get permissions from the role
	ns := ""
	if rb.RoleRef.Kind == "Role" {
		ns = rb.Metadata.Namespace
	}
	binding.Permissions = a.getRolePermissions(ctx, cl, rb.RoleRef.Kind, ns, rb.RoleRef.Name)

	return binding
}

func (a *App) getRolePermissions(ctx context.Context, cl client.ClusterClient, kind, namespace, name string) []RBACPermission {
	var permissions []RBACPermission

	var roleKind string
	if kind == "ClusterRole" {
		roleKind = "clusterroles"
	} else {
		roleKind = "roles"
	}

	role, err := cl.Get(ctx, roleKind, namespace, name)
	if err != nil {
		return permissions
	}

	var roleData struct {
		Rules []struct {
			Verbs         []string `json:"verbs"`
			Resources     []string `json:"resources"`
			ResourceNames []string `json:"resourceNames"`
			APIGroups     []string `json:"apiGroups"`
		} `json:"rules"`
	}

	if err := json.Unmarshal(role.Raw, &roleData); err != nil {
		return permissions
	}

	for _, rule := range roleData.Rules {
		permissions = append(permissions, RBACPermission{
			Verbs:         rule.Verbs,
			Resources:     rule.Resources,
			ResourceNames: rule.ResourceNames,
			APIGroups:     rule.APIGroups,
		})
	}

	return permissions
}

func (a *App) checkDangerousAccess(summary *RBACSummary, binding *RBACBinding) {
	// Skip system bindings unless they grant access to non-system subjects
	isSystemBinding := strings.HasPrefix(binding.Name, "system:") ||
		strings.HasPrefix(binding.Name, "kube-") ||
		binding.Namespace == "kube-system" ||
		binding.Namespace == "kube-public" ||
		binding.Namespace == "kube-node-lease"

	dangerousVerbs := map[string]string{
		"*":                "wildcard access (all verbs)",
		"delete":           "can delete resources",
		"deletecollection": "can delete multiple resources at once",
		"create":           "can create resources",
		"patch":            "can patch/modify resources",
		"update":           "can update resources",
	}

	dangerousResources := map[string]string{
		"*":                   "access to all resources",
		"secrets":             "access to secrets",
		"pods/exec":           "can exec into pods",
		"serviceaccounts":     "can manage service accounts",
		"clusterroles":        "can manage cluster roles",
		"clusterrolebindings": "can manage cluster role bindings",
		"roles":               "can manage roles",
		"rolebindings":        "can manage role bindings",
	}

	for _, perm := range binding.Permissions {
		for _, verb := range perm.Verbs {
			for _, resource := range perm.Resources {
				// Check for dangerous verbs
				if reason, ok := dangerousVerbs[verb]; ok && verb != "*" {
					_ = reason
					// reasons = append(reasons, reason)
				}

				// Check for dangerous resources
				if reason, ok := dangerousResources[resource]; ok {
					_ = reason
					// reasons = append(reasons, reason)
				}

				// Check for wildcard resources with dangerous verbs
				if resource == "*" && (verb == "delete" || verb == "patch" || verb == "update" || verb == "*") {
					for _, subject := range binding.Subjects {
						// Skip system subjects
						if a.isSystemSubject(subject) {
							continue
						}
						// If binding is system binding and subject is valid (e.g. user unbound to system role), we might want to show it
						// But usually system bindings bind system subjects.
						// The main case we want to catch is "cluster-admin" bound to a user.

						info := DangerousAccessInfo{
							Subject:     subject,
							Reason:      fmt.Sprintf("Has %s on all resources", verb),
							Binding:     binding.Name,
							Namespace:   binding.Namespace,
							Permissions: []string{fmt.Sprintf("%s %s", verb, resource)},
						}
						summary.DangerousAccess = append(summary.DangerousAccess, info)
					}
				}

				// Check for secrets access
				if resource == "secrets" && (verb == "get" || verb == "list" || verb == "*") {
					for _, subject := range binding.Subjects {
						if a.isSystemSubject(subject) {
							continue
						}
						info := DangerousAccessInfo{
							Subject:     subject,
							Reason:      "Can read secrets",
							Binding:     binding.Name,
							Namespace:   binding.Namespace,
							Permissions: []string{fmt.Sprintf("%s %s", verb, resource)},
						}
						summary.DangerousAccess = append(summary.DangerousAccess, info)
					}
				}

				// Check for cluster-admin like access
				if binding.RoleName == "cluster-admin" || (verb == "*" && resource == "*") {
					for _, subject := range binding.Subjects {
						if a.isSystemSubject(subject) && isSystemBinding {
							continue
						}

						// Special case: system:masters group is expected to have cluster-admin
						if subject.Kind == "Group" && subject.Name == "system:masters" {
							continue
						}

						info := DangerousAccessInfo{
							Subject:     subject,
							Reason:      "Has cluster-admin or equivalent privileges",
							Binding:     binding.Name,
							Namespace:   binding.Namespace,
							Permissions: []string{"cluster-admin equivalent"},
						}
						summary.DangerousAccess = append(summary.DangerousAccess, info)
					}
				}
			}
		}
	}
}

func (a *App) isSystemSubject(subject RBACSubject) bool {
	// Check for system: prefix
	if strings.HasPrefix(subject.Name, "system:") || strings.HasPrefix(subject.Name, "kube-") {
		return true
	}
	// Check for kube-system namespace for service accounts
	if subject.Kind == "ServiceAccount" && (subject.Namespace == "kube-system" || subject.Namespace == "kube-public" || subject.Namespace == "kube-node-lease") {
		return true
	}
	return false
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

// GetSubjectPermissions returns permissions for a specific subject.
func (a *App) GetSubjectPermissions(kind, name, namespace string) ([]RBACBinding, error) {
	summary, err := a.GetRBACAnalysis()
	if err != nil {
		return nil, err
	}

	var result []RBACBinding
	for _, binding := range summary.Bindings {
		for _, subject := range binding.Subjects {
			if subject.Kind == kind && subject.Name == name {
				if kind == "ServiceAccount" && namespace != "" && subject.Namespace != namespace {
					continue
				}
				result = append(result, binding)
				break
			}
		}
	}

	return result, nil
}

// QuerySecurityAI sends a security query to AI with cluster context
func (a *App) QuerySecurityAI(query string) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}

	if !cfg.Kubecat.AI.Enabled {
		return "", fmt.Errorf("AI features are not enabled. Please enable them in Settings.")
	}

	cl, err := a.nexus.Clusters.Manager().Active()
	if err != nil {
		return "", fmt.Errorf("no active cluster: %w", err)
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	var contextBuilder strings.Builder
	contextBuilder.WriteString("Cluster Security Context:\n\n")

	// 1. RBAC Analysis
	rbac, err := a.GetRBACAnalysis()
	if err == nil {
		contextBuilder.WriteString("## RBAC Analysis Summary\n")
		contextBuilder.WriteString(fmt.Sprintf("- Total Bindings: %d\n", len(rbac.Bindings)))
		contextBuilder.WriteString(fmt.Sprintf("- Dangerous Access Warnings: %d\n", len(rbac.DangerousAccess)))

		if len(rbac.DangerousAccess) > 0 {
			contextBuilder.WriteString("\n### Dangerous Access Details:\n")
			for i, da := range rbac.DangerousAccess {
				if i > 15 {
					contextBuilder.WriteString("- ... (more warnings truncated)\n")
					break
				}
				contextBuilder.WriteString(fmt.Sprintf("- %s %s: %s (Reason: %s)\n", da.Subject.Kind, da.Subject.Name, da.Permissions[0], da.Reason))
			}
		}
	} else {
		contextBuilder.WriteString(fmt.Sprintf("## RBAC Analysis Error: %v\n", err))
	}

	// 2. Network Policies
	npList, err := cl.List(ctx, "networkpolicies", client.ListOptions{Limit: 100})
	if err == nil {
		contextBuilder.WriteString(fmt.Sprintf("\n## Network Policies: %d found\n", len(npList.Items)))
		for i, np := range npList.Items {
			if i < 10 {
				contextBuilder.WriteString(fmt.Sprintf("- %s/%s\n", np.Namespace, np.Name))
			}
		}
		if len(npList.Items) > 10 {
			contextBuilder.WriteString("- ... (more policies truncated)\n")
		}
	}

	// 3. Exposed Services
	svcList, err := cl.List(ctx, "services", client.ListOptions{Limit: 500})
	if err == nil {
		contextBuilder.WriteString("\n## Exposed Services (NodePort/LoadBalancer):\n")
		count := 0
		for _, svc := range svcList.Items {
			var spec struct {
				Spec struct {
					Type  string `json:"type"`
					Ports []struct {
						Port     int32 `json:"port"`
						NodePort int32 `json:"nodePort,omitempty"`
					} `json:"ports"`
				} `json:"spec"`
			}
			if err := json.Unmarshal(svc.Raw, &spec); err == nil {
				if spec.Spec.Type == "LoadBalancer" || spec.Spec.Type == "NodePort" {
					contextBuilder.WriteString(fmt.Sprintf("- %s/%s (%s)\n", svc.Namespace, svc.Name, spec.Spec.Type))
					count++
					if count >= 15 {
						contextBuilder.WriteString("- ... (more services truncated)\n")
						break
					}
				}
			}
		}
		if count == 0 {
			contextBuilder.WriteString("None found.\n")
		}
	}

	// 4. Privileged Pods
	podList, err := cl.List(ctx, "pods", client.ListOptions{Limit: 500})
	if err == nil {
		contextBuilder.WriteString("\n## Host/Privileged Pods:\n")
		count := 0
		for _, pod := range podList.Items {
			var spec struct {
				Spec struct {
					HostNetwork bool `json:"hostNetwork"`
					Containers  []struct {
						SecurityContext *struct {
							Privileged *bool `json:"privileged"`
						} `json:"securityContext"`
					} `json:"containers"`
				} `json:"spec"`
			}
			if err := json.Unmarshal(pod.Raw, &spec); err == nil {
				isPrivileged := false
				if spec.Spec.HostNetwork {
					isPrivileged = true
				}
				for _, c := range spec.Spec.Containers {
					if c.SecurityContext != nil && c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
						isPrivileged = true
						break
					}
				}

				if isPrivileged {
					contextBuilder.WriteString(fmt.Sprintf("- %s/%s (HostNetwork: %v)\n", pod.Namespace, pod.Name, spec.Spec.HostNetwork))
					count++
					if count > 15 {
						contextBuilder.WriteString("- ... (more pods truncated)\n")
						break
					}
				}
			}
		}
		if count == 0 {
			contextBuilder.WriteString("None found.\n")
		}
	}

	contextBuilder.WriteString("\n---\n")
	contextBuilder.WriteString("User Question: " + query + "\n\n")
	contextBuilder.WriteString("Role: You are a Kubernetes security expert. Analyze the above context and answer the user question.\n")
	contextBuilder.WriteString("Style: Structured, concise, authoritative using markdown. Use tables for lists if applicable. Highlight risks. Use severity levels (Info, Warning, Critical) if finding issues. If suggesting commands, use code blocks.\n")

	// Call AI provider
	aiCfg := cfg.Kubecat.AI
	selectedProvider := aiCfg.SelectedProvider
	if selectedProvider == "" {
		return "", fmt.Errorf("No AI provider selected. Please configure one in Settings.")
	}

	providerConfig, ok := aiCfg.Providers[selectedProvider]
	if !ok || !providerConfig.Enabled {
		return "", fmt.Errorf("Selected AI provider (%s) is not enabled or configured.", selectedProvider)
	}

	providerCfg := ai.ProviderConfig{
		Model:    aiCfg.SelectedModel,
		APIKey:   providerConfig.APIKey,
		Endpoint: providerConfig.Endpoint,
	}

	provider, err := ai.NewProvider(selectedProvider, providerCfg)
	if err != nil {
		return "", fmt.Errorf("Unknown AI provider: %s", selectedProvider)
	}
	defer func() { _ = provider.Close() }()

	if !provider.Available(ctx) {
		if selectedProvider == "ollama" {
			return "", fmt.Errorf("Ollama is not running. Please start Ollama and try again.")
		}
		return "", fmt.Errorf("AI provider is not available. Check your API key or connection.")
	}

	// Sanitize the security context prompt before transmitting to a cloud provider.
	// The context includes RBAC bindings, service names, and pod specs that may
	// contain sensitive patterns.
	securityPrompt := contextBuilder.String()
	if ai.IsCloudProvider(selectedProvider) {
		securityPrompt = ai.SanitizeForCloud(securityPrompt)
	}

	// Audit the security AI query — log hash only, never the raw prompt.
	audit.LogAIQuery(selectedProvider, a.nexus.Clusters.ActiveContext(), "", securityPrompt)

	return provider.Query(ctx, securityPrompt)
}

// GetSecuritySummary returns a security summary for the active cluster.
func (a *App) GetSecuritySummary(namespace string) (*SecuritySummaryInfo, error) {
	cl, err := a.nexus.Clusters.Manager().Active()
	if err != nil {
		return nil, fmt.Errorf("no active cluster: %w", err)
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	summary := &SecuritySummaryInfo{
		IssuesByCategory: make(map[string]int),
		TopIssues:        []SecurityIssueInfo{},
		Score: SecurityScoreInfo{
			Categories: make(map[string]int),
			ScannedAt:  time.Now().Format(time.RFC3339),
		},
	}

	var allIssues []SecurityIssueInfo

	// Scan for runtime security issues
	pods, err := cl.List(ctx, "pods", client.ListOptions{Namespace: namespace, Limit: 5000})
	if err == nil {
		for _, pod := range pods.Items {
			issues := scanPodSecurityIssues(pod)
			allIssues = append(allIssues, issues...)
		}
	}

	// Scan for RBAC issues
	rbacIssues := a.scanRBACSecurityIssues(ctx, cl)
	allIssues = append(allIssues, rbacIssues...)

	// Scan for network policy gaps
	netpolIssues := a.scanNetworkPolicyGaps(ctx, cl, namespace)
	allIssues = append(allIssues, netpolIssues...)

	// Count issues
	for _, issue := range allIssues {
		switch issue.Severity {
		case "Critical":
			summary.CriticalCount++
		case "High":
			summary.HighCount++
		case "Medium":
			summary.MediumCount++
		case "Low":
			summary.LowCount++
		}
		summary.IssuesByCategory[issue.Category]++
	}
	summary.TotalIssues = len(allIssues)

	// Top 10 issues by severity
	sortedIssues := sortSecurityIssuesBySeverity(allIssues)
	if len(sortedIssues) > 10 {
		summary.TopIssues = sortedIssues[:10]
	} else {
		summary.TopIssues = sortedIssues
	}

	// Calculate score
	summary.Score.Overall = calculateSecurityScore(summary)
	summary.Score.Grade = calculateSecurityGrade(summary.Score.Overall)

	return summary, nil
}

// GetSecurityIssues returns all security issues for a namespace.
func (a *App) GetSecurityIssues(namespace, category string) ([]SecurityIssueInfo, error) {
	summary, err := a.GetSecuritySummary(namespace)
	if err != nil {
		return nil, err
	}

	if category == "" || category == "all" {
		return summary.TopIssues, nil
	}

	var filtered []SecurityIssueInfo
	for _, issue := range summary.TopIssues {
		if issue.Category == category {
			filtered = append(filtered, issue)
		}
	}
	return filtered, nil
}

func scanPodSecurityIssues(pod client.Resource) []SecurityIssueInfo {
	var issues []SecurityIssueInfo

	var podSpec struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Spec struct {
			Containers []struct {
				Name            string `json:"name"`
				SecurityContext *struct {
					Privileged               *bool  `json:"privileged"`
					RunAsUser                *int64 `json:"runAsUser"`
					AllowPrivilegeEscalation *bool  `json:"allowPrivilegeEscalation"`
				} `json:"securityContext"`
			} `json:"containers"`
			HostNetwork bool `json:"hostNetwork"`
			HostPID     bool `json:"hostPID"`
			HostIPC     bool `json:"hostIPC"`
		} `json:"spec"`
	}

	if err := json.Unmarshal(pod.Raw, &podSpec); err != nil {
		return issues
	}

	if podSpec.Spec.HostNetwork {
		issues = append(issues, SecurityIssueInfo{
			ID:          fmt.Sprintf("runtime.hostnetwork.%s.%s", podSpec.Metadata.Namespace, podSpec.Metadata.Name),
			Category:    "Runtime",
			Severity:    "High",
			Title:       "Pod uses host network",
			Description: "Pod has access to the host network namespace",
			Resource:    podSpec.Metadata.Name,
			Namespace:   podSpec.Metadata.Namespace,
			Kind:        "Pod",
			Remediation: "Remove hostNetwork: true unless absolutely required",
			DetectedAt:  time.Now().Format(time.RFC3339),
		})
	}

	if podSpec.Spec.HostPID {
		issues = append(issues, SecurityIssueInfo{
			ID:          fmt.Sprintf("runtime.hostpid.%s.%s", podSpec.Metadata.Namespace, podSpec.Metadata.Name),
			Category:    "Runtime",
			Severity:    "High",
			Title:       "Pod uses host PID namespace",
			Description: "Pod has access to the host PID namespace",
			Resource:    podSpec.Metadata.Name,
			Namespace:   podSpec.Metadata.Namespace,
			Kind:        "Pod",
			Remediation: "Remove hostPID: true unless absolutely required",
			DetectedAt:  time.Now().Format(time.RFC3339),
		})
	}

	for _, container := range podSpec.Spec.Containers {
		if container.SecurityContext != nil {
			sc := container.SecurityContext

			if sc.Privileged != nil && *sc.Privileged {
				issues = append(issues, SecurityIssueInfo{
					ID:          fmt.Sprintf("runtime.privileged.%s.%s.%s", podSpec.Metadata.Namespace, podSpec.Metadata.Name, container.Name),
					Category:    "Runtime",
					Severity:    "Critical",
					Title:       "Privileged container",
					Description: fmt.Sprintf("Container '%s' is running in privileged mode", container.Name),
					Resource:    podSpec.Metadata.Name,
					Namespace:   podSpec.Metadata.Namespace,
					Kind:        "Pod",
					Remediation: "Remove privileged: true and use specific capabilities instead",
					DetectedAt:  time.Now().Format(time.RFC3339),
				})
			}

			if sc.RunAsUser != nil && *sc.RunAsUser == 0 {
				issues = append(issues, SecurityIssueInfo{
					ID:          fmt.Sprintf("runtime.root.%s.%s.%s", podSpec.Metadata.Namespace, podSpec.Metadata.Name, container.Name),
					Category:    "Runtime",
					Severity:    "High",
					Title:       "Container runs as root",
					Description: fmt.Sprintf("Container '%s' is configured to run as root (UID 0)", container.Name),
					Resource:    podSpec.Metadata.Name,
					Namespace:   podSpec.Metadata.Namespace,
					Kind:        "Pod",
					Remediation: "Set runAsUser to a non-root UID and runAsNonRoot: true",
					DetectedAt:  time.Now().Format(time.RFC3339),
				})
			}
		}
	}

	return issues
}

func (a *App) scanRBACSecurityIssues(ctx context.Context, cl client.ClusterClient) []SecurityIssueInfo {
	var issues []SecurityIssueInfo

	crbList, err := cl.List(ctx, "clusterrolebindings", client.ListOptions{Limit: 1000})
	if err != nil {
		return issues
	}

	for _, crb := range crbList.Items {
		var binding struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			RoleRef struct {
				Name string `json:"name"`
			} `json:"roleRef"`
			Subjects []struct {
				Kind      string `json:"kind"`
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"subjects"`
		}

		if err := json.Unmarshal(crb.Raw, &binding); err != nil {
			continue
		}

		if binding.RoleRef.Name == "cluster-admin" {
			for _, subj := range binding.Subjects {
				if subj.Kind == "ServiceAccount" && (subj.Namespace == "kube-system" || subj.Name == "default") {
					continue
				}

				issues = append(issues, SecurityIssueInfo{
					ID:          fmt.Sprintf("rbac.clusteradmin.%s.%s", binding.Metadata.Name, subj.Name),
					Category:    "RBAC",
					Severity:    "High",
					Title:       "Cluster-admin binding",
					Description: fmt.Sprintf("%s '%s' has cluster-admin privileges via %s", subj.Kind, subj.Name, binding.Metadata.Name),
					Resource:    subj.Name,
					Namespace:   subj.Namespace,
					Kind:        subj.Kind,
					Remediation: "Review if cluster-admin is necessary; use more restrictive roles",
					DetectedAt:  time.Now().Format(time.RFC3339),
				})
			}
		}
	}

	return issues
}

func (a *App) scanNetworkPolicyGaps(ctx context.Context, cl client.ClusterClient, namespace string) []SecurityIssueInfo {
	var issues []SecurityIssueInfo

	nsList, err := cl.List(ctx, "namespaces", client.ListOptions{Limit: 100})
	if err != nil {
		return issues
	}

	for _, ns := range nsList.Items {
		nsName := ns.Name

		if namespace != "" && nsName != namespace {
			continue
		}

		if nsName == "kube-system" || nsName == "kube-public" || nsName == "kube-node-lease" {
			continue
		}

		netpols, err := cl.List(ctx, "networkpolicies", client.ListOptions{Namespace: nsName, Limit: 100})
		if err != nil || len(netpols.Items) == 0 {
			issues = append(issues, SecurityIssueInfo{
				ID:          fmt.Sprintf("network.nopolicy.%s", nsName),
				Category:    "Network",
				Severity:    "Medium",
				Title:       "No network policies",
				Description: fmt.Sprintf("Namespace '%s' has no network policies - all traffic is allowed", nsName),
				Namespace:   nsName,
				Kind:        "Namespace",
				Remediation: "Implement network policies to restrict pod-to-pod traffic",
				DetectedAt:  time.Now().Format(time.RFC3339),
			})
		}
	}

	return issues
}

func sortSecurityIssuesBySeverity(issues []SecurityIssueInfo) []SecurityIssueInfo {
	severityOrder := map[string]int{
		"Critical": 0,
		"High":     1,
		"Medium":   2,
		"Low":      3,
		"Info":     4,
	}

	result := make([]SecurityIssueInfo, len(issues))
	copy(result, issues)

	for i := 0; i < len(result)-1; i++ {
		for j := 0; j < len(result)-i-1; j++ {
			if severityOrder[result[j].Severity] > severityOrder[result[j+1].Severity] {
				result[j], result[j+1] = result[j+1], result[j]
			}
		}
	}

	return result
}

func calculateSecurityScore(summary *SecuritySummaryInfo) int {
	score := 100
	score -= summary.CriticalCount * 15
	score -= summary.HighCount * 10
	score -= summary.MediumCount * 5
	score -= summary.LowCount * 2
	if score < 0 {
		score = 0
	}
	return score
}

func calculateSecurityGrade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}
