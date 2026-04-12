// SPDX-License-Identifier: Apache-2.0

package security

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
)

// Scanner provides security scanning capabilities.
type Scanner struct {
	client client.ClusterClient
}

// NewScanner creates a new security scanner.
func NewScanner(cl client.ClusterClient) *Scanner {
	return &Scanner{client: cl}
}

// GetSecuritySummary returns an overall security summary.
func (s *Scanner) GetSecuritySummary(ctx context.Context, namespace string) (*SecuritySummary, error) {
	summary := &SecuritySummary{
		IssuesByCategory: make(map[Category]int),
		TopIssues:        []SecurityIssue{},
	}

	// Collect all issues
	var allIssues []SecurityIssue

	// RBAC issues
	rbacIssues, _ := s.scanRBACIssues(ctx)
	allIssues = append(allIssues, rbacIssues...)

	// Runtime issues (pods running as root, privileged, etc.)
	runtimeIssues, _ := s.scanRuntimeIssues(ctx, namespace)
	allIssues = append(allIssues, runtimeIssues...)

	// Network policy issues
	netpolIssues, _ := s.scanNetworkPolicyIssues(ctx, namespace)
	allIssues = append(allIssues, netpolIssues...)

	// Count by severity and category
	for _, issue := range allIssues {
		switch issue.Severity {
		case SeverityCritical:
			summary.CriticalCount++
		case SeverityHigh:
			summary.HighCount++
		case SeverityMedium:
			summary.MediumCount++
		case SeverityLow:
			summary.LowCount++
		}
		summary.IssuesByCategory[issue.Category]++
	}
	summary.TotalIssues = len(allIssues)

	// Top issues (first 10 by severity)
	sortedIssues := sortBySeverity(allIssues)
	if len(sortedIssues) > 10 {
		summary.TopIssues = sortedIssues[:10]
	} else {
		summary.TopIssues = sortedIssues
	}

	// Calculate score
	summary.Score = s.calculateScore(summary)

	return summary, nil
}

// AnalyzeRBAC performs RBAC analysis.
func (s *Scanner) AnalyzeRBAC(ctx context.Context) (*RBACAnalysis, error) {
	analysis := &RBACAnalysis{
		Bindings:        []RBACBinding{},
		SubjectAccess:   make(map[string][]string),
		DangerousAccess: []DangerousAccess{},
		WildcardAccess:  []WildcardAccess{},
	}

	// Get ClusterRoleBindings
	crbList, err := s.client.List(ctx, "clusterrolebindings", client.ListOptions{Limit: 1000})
	if err == nil {
		for _, r := range crbList.Items {
			binding := s.parseClusterRoleBinding(ctx, r)
			if binding != nil {
				analysis.Bindings = append(analysis.Bindings, *binding)
				s.analyzeBinding(analysis, binding)
			}
		}
	}

	// Get RoleBindings
	rbList, err := s.client.List(ctx, "rolebindings", client.ListOptions{Limit: 1000})
	if err == nil {
		for _, r := range rbList.Items {
			binding := s.parseRoleBinding(ctx, r)
			if binding != nil {
				analysis.Bindings = append(analysis.Bindings, *binding)
				s.analyzeBinding(analysis, binding)
			}
		}
	}

	return analysis, nil
}

// GetPolicySummary returns policy enforcement summary.
func (s *Scanner) GetPolicySummary(ctx context.Context) (*PolicySummary, error) {
	summary := &PolicySummary{
		Provider:   "none",
		Policies:   []PolicyInfo{},
		Violations: []PolicyViolation{},
	}

	// Check for Gatekeeper
	constraints, err := s.client.List(ctx, "constraints", client.ListOptions{Limit: 100})
	if err == nil && len(constraints.Items) > 0 {
		summary.Provider = "gatekeeper"
		// Parse Gatekeeper constraints
		for _, c := range constraints.Items {
			info := s.parseGatekeeperConstraint(c)
			if info != nil {
				summary.Policies = append(summary.Policies, *info)
				summary.TotalPolicies++
				summary.TotalViolations += info.Violations
			}
		}
		return summary, nil
	}

	// Check for Kyverno
	cpList, err := s.client.List(ctx, "clusterpolicies", client.ListOptions{Limit: 100})
	if err == nil && len(cpList.Items) > 0 {
		summary.Provider = "kyverno"
		for _, p := range cpList.Items {
			info := s.parseKyvernoPolicy(p)
			if info != nil {
				summary.Policies = append(summary.Policies, *info)
				summary.TotalPolicies++
			}
		}
	}

	return summary, nil
}

// GetNetworkPolicyAnalysis analyzes network policies for a pod.
func (s *Scanner) GetNetworkPolicyAnalysis(ctx context.Context, namespace, podName string) (*NetworkPolicyAnalysis, error) {
	analysis := &NetworkPolicyAnalysis{
		Pod:             podName,
		Namespace:       namespace,
		IngressPolicies: []NetworkPolicyRule{},
		EgressPolicies:  []NetworkPolicyRule{},
	}

	// Get the pod to find its labels
	pod, err := s.client.Get(ctx, "pods", namespace, podName)
	if err != nil {
		return nil, err
	}

	var podSpec struct {
		Metadata struct {
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(pod.Raw, &podSpec); err != nil {
		return nil, err
	}

	// Get network policies in the namespace
	netpols, err := s.client.List(ctx, "networkpolicies", client.ListOptions{Namespace: namespace, Limit: 100})
	if err != nil {
		return analysis, nil // No network policies
	}

	for _, np := range netpols.Items {
		rule := s.parseNetworkPolicy(np, podSpec.Metadata.Labels)
		if rule != nil {
			analysis.HasPolicies = true
			if rule.Direction == "ingress" {
				analysis.IngressPolicies = append(analysis.IngressPolicies, *rule)
			} else {
				analysis.EgressPolicies = append(analysis.EgressPolicies, *rule)
			}
		}
	}

	return analysis, nil
}

// scanRBACIssues finds RBAC-related security issues.
func (s *Scanner) scanRBACIssues(ctx context.Context) ([]SecurityIssue, error) {
	var issues []SecurityIssue

	rbacAnalysis, err := s.AnalyzeRBAC(ctx)
	if err != nil {
		return issues, err
	}

	// Create issues for dangerous access
	for _, da := range rbacAnalysis.DangerousAccess {
		issues = append(issues, SecurityIssue{
			ID:          fmt.Sprintf("rbac.dangerous.%s", da.Subject.Name),
			Category:    CategoryRBAC,
			Severity:    SeverityHigh,
			Title:       "Dangerous RBAC permissions",
			Description: da.Reason,
			Resource:    da.Subject.Name,
			Namespace:   da.Subject.Namespace,
			Kind:        da.Subject.Kind,
			Remediation: "Review and restrict permissions using least privilege principle",
			Details: map[string]interface{}{
				"binding":     da.Binding,
				"permissions": da.Permissions,
			},
			DetectedAt: time.Now(),
		})
	}

	// Create issues for wildcard access
	for _, wa := range rbacAnalysis.WildcardAccess {
		issues = append(issues, SecurityIssue{
			ID:          fmt.Sprintf("rbac.wildcard.%s", wa.Subject.Name),
			Category:    CategoryRBAC,
			Severity:    SeverityMedium,
			Title:       "Wildcard permissions detected",
			Description: fmt.Sprintf("Subject has wildcard permissions: verbs=%v, resources=%v", wa.Verbs, wa.Resources),
			Resource:    wa.Subject.Name,
			Kind:        wa.Subject.Kind,
			Remediation: "Replace wildcard permissions with specific permissions",
			DetectedAt:  time.Now(),
		})
	}

	return issues, nil
}

// scanRuntimeIssues finds runtime security issues (privileged pods, root users, etc.).
func (s *Scanner) scanRuntimeIssues(ctx context.Context, namespace string) ([]SecurityIssue, error) {
	var issues []SecurityIssue

	pods, err := s.client.List(ctx, "pods", client.ListOptions{Namespace: namespace, Limit: 5000})
	if err != nil {
		return issues, err
	}

	for _, pod := range pods.Items {
		podIssues := s.analyzePodSecurity(pod)
		issues = append(issues, podIssues...)
	}

	return issues, nil
}

// scanNetworkPolicyIssues finds network policy issues.
func (s *Scanner) scanNetworkPolicyIssues(ctx context.Context, namespace string) ([]SecurityIssue, error) {
	var issues []SecurityIssue

	// Get all namespaces
	nsList, err := s.client.List(ctx, "namespaces", client.ListOptions{Limit: 100})
	if err != nil {
		return issues, err
	}

	for _, ns := range nsList.Items {
		nsName := ns.Name
		if namespace != "" && nsName != namespace {
			continue
		}

		// Check if namespace has any network policies
		netpols, err := s.client.List(ctx, "networkpolicies", client.ListOptions{Namespace: nsName, Limit: 100})
		if err != nil || len(netpols.Items) == 0 {
			// Skip system namespaces
			if nsName == "kube-system" || nsName == "kube-public" || nsName == "kube-node-lease" {
				continue
			}
			issues = append(issues, SecurityIssue{
				ID:       fmt.Sprintf("netpol.missing.%s", nsName),
				Category: CategoryNetwork,
				// HIGH: namespace without any NetworkPolicy allows unrestricted
				// pod-to-pod and pod-to-external egress — a single compromised pod
				// can reach every other workload and exfiltrate data freely.
				Severity:    SeverityHigh,
				Title:       "No network policies",
				Description: fmt.Sprintf("Namespace '%s' has no network policies - all traffic is allowed", nsName),
				Namespace:   nsName,
				Kind:        "Namespace",
				Remediation: "Apply a default-deny-all NetworkPolicy and add explicit allow rules for required traffic",
				DetectedAt:  time.Now(),
			})
		}
	}

	return issues, nil
}

// analyzePodSecurity checks a pod for security issues.
func (s *Scanner) analyzePodSecurity(pod client.Resource) []SecurityIssue {
	var issues []SecurityIssue

	var podSpec struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Spec struct {
			Containers []struct {
				Name            string `json:"name"`
				Image           string `json:"image"`
				SecurityContext *struct {
					Privileged               *bool  `json:"privileged"`
					RunAsRoot                *bool  `json:"runAsRoot"`
					RunAsUser                *int64 `json:"runAsUser"`
					AllowPrivilegeEscalation *bool  `json:"allowPrivilegeEscalation"`
					ReadOnlyRootFilesystem   *bool  `json:"readOnlyRootFilesystem"`
				} `json:"securityContext"`
			} `json:"containers"`
			SecurityContext *struct {
				RunAsNonRoot *bool  `json:"runAsNonRoot"`
				RunAsUser    *int64 `json:"runAsUser"`
			} `json:"securityContext"`
			HostNetwork bool `json:"hostNetwork"`
			HostPID     bool `json:"hostPID"`
			HostIPC     bool `json:"hostIPC"`
		} `json:"spec"`
	}

	if err := json.Unmarshal(pod.Raw, &podSpec); err != nil {
		return issues
	}

	// Check host namespace usage
	if podSpec.Spec.HostNetwork {
		issues = append(issues, SecurityIssue{
			ID:          fmt.Sprintf("runtime.hostnetwork.%s.%s", podSpec.Metadata.Namespace, podSpec.Metadata.Name),
			Category:    CategoryRuntime,
			Severity:    SeverityHigh,
			Title:       "Pod uses host network",
			Description: "Pod has access to the host network namespace",
			Resource:    podSpec.Metadata.Name,
			Namespace:   podSpec.Metadata.Namespace,
			Kind:        "Pod",
			Remediation: "Remove hostNetwork: true unless absolutely required",
			DetectedAt:  time.Now(),
		})
	}

	if podSpec.Spec.HostPID {
		issues = append(issues, SecurityIssue{
			ID:          fmt.Sprintf("runtime.hostpid.%s.%s", podSpec.Metadata.Namespace, podSpec.Metadata.Name),
			Category:    CategoryRuntime,
			Severity:    SeverityHigh,
			Title:       "Pod uses host PID namespace",
			Description: "Pod has access to the host PID namespace",
			Resource:    podSpec.Metadata.Name,
			Namespace:   podSpec.Metadata.Namespace,
			Kind:        "Pod",
			Remediation: "Remove hostPID: true unless absolutely required",
			DetectedAt:  time.Now(),
		})
	}

	// Check containers
	for _, container := range podSpec.Spec.Containers {
		if container.SecurityContext != nil {
			sc := container.SecurityContext

			// Check privileged
			if sc.Privileged != nil && *sc.Privileged {
				issues = append(issues, SecurityIssue{
					ID:          fmt.Sprintf("runtime.privileged.%s.%s.%s", podSpec.Metadata.Namespace, podSpec.Metadata.Name, container.Name),
					Category:    CategoryRuntime,
					Severity:    SeverityCritical,
					Title:       "Privileged container",
					Description: fmt.Sprintf("Container '%s' is running in privileged mode", container.Name),
					Resource:    podSpec.Metadata.Name,
					Namespace:   podSpec.Metadata.Namespace,
					Kind:        "Pod",
					Remediation: "Remove privileged: true and use specific capabilities instead",
					DetectedAt:  time.Now(),
				})
			}

			// Check root user
			if sc.RunAsUser != nil && *sc.RunAsUser == 0 {
				issues = append(issues, SecurityIssue{
					ID:          fmt.Sprintf("runtime.root.%s.%s.%s", podSpec.Metadata.Namespace, podSpec.Metadata.Name, container.Name),
					Category:    CategoryRuntime,
					Severity:    SeverityHigh,
					Title:       "Container runs as root",
					Description: fmt.Sprintf("Container '%s' is configured to run as root (UID 0)", container.Name),
					Resource:    podSpec.Metadata.Name,
					Namespace:   podSpec.Metadata.Namespace,
					Kind:        "Pod",
					Remediation: "Set runAsUser to a non-root UID and runAsNonRoot: true",
					DetectedAt:  time.Now(),
				})
			}
		}
	}

	return issues
}

// Helper functions

func (s *Scanner) parseClusterRoleBinding(ctx context.Context, r client.Resource) *RBACBinding {
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

	if err := json.Unmarshal(r.Raw, &crb); err != nil {
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

	binding.Permissions = s.getRolePermissions(ctx, crb.RoleRef.Kind, "", crb.RoleRef.Name)
	return binding
}

func (s *Scanner) parseRoleBinding(ctx context.Context, r client.Resource) *RBACBinding {
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

	if err := json.Unmarshal(r.Raw, &rb); err != nil {
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

	for _, subj := range rb.Subjects {
		binding.Subjects = append(binding.Subjects, RBACSubject{
			Kind:      subj.Kind,
			Name:      subj.Name,
			Namespace: subj.Namespace,
		})
	}

	ns := ""
	if rb.RoleRef.Kind == "Role" {
		ns = rb.Metadata.Namespace
	}
	binding.Permissions = s.getRolePermissions(ctx, rb.RoleRef.Kind, ns, rb.RoleRef.Name)
	return binding
}

func (s *Scanner) getRolePermissions(ctx context.Context, kind, namespace, name string) []RBACPermission {
	var permissions []RBACPermission

	var roleKind string
	if kind == "ClusterRole" {
		roleKind = "clusterroles"
	} else {
		roleKind = "roles"
	}

	role, err := s.client.Get(ctx, roleKind, namespace, name)
	if err != nil {
		return permissions
	}

	var roleSpec struct {
		Rules []struct {
			Verbs         []string `json:"verbs"`
			Resources     []string `json:"resources"`
			ResourceNames []string `json:"resourceNames"`
			APIGroups     []string `json:"apiGroups"`
		} `json:"rules"`
	}

	if err := json.Unmarshal(role.Raw, &roleSpec); err != nil {
		return permissions
	}

	for _, rule := range roleSpec.Rules {
		permissions = append(permissions, RBACPermission{
			Verbs:         rule.Verbs,
			Resources:     rule.Resources,
			ResourceNames: rule.ResourceNames,
			APIGroups:     rule.APIGroups,
			Namespace:     namespace,
		})
	}

	return permissions
}

func (s *Scanner) analyzeBinding(analysis *RBACAnalysis, binding *RBACBinding) {
	for _, subject := range binding.Subjects {
		subjectKey := fmt.Sprintf("%s:%s", subject.Kind, subject.Name)
		if subject.Namespace != "" {
			subjectKey = fmt.Sprintf("%s:%s/%s", subject.Kind, subject.Namespace, subject.Name)
		}

		// Track namespace access
		ns := binding.Namespace
		if ns == "" {
			ns = "*"
		}
		analysis.SubjectAccess[subjectKey] = appendUnique(analysis.SubjectAccess[subjectKey], ns)

		// Check for dangerous permissions
		for _, perm := range binding.Permissions {
			// Check for cluster-admin equivalent
			if containsString(perm.Verbs, "*") && containsString(perm.Resources, "*") {
				analysis.DangerousAccess = append(analysis.DangerousAccess, DangerousAccess{
					Subject:     subject,
					Binding:     binding.Name,
					Reason:      "Has cluster-admin equivalent permissions (all verbs on all resources)",
					Permissions: []string{"*.*"},
				})
			}

			// Check for secrets access
			if containsString(perm.Resources, "secrets") && (containsString(perm.Verbs, "*") || containsString(perm.Verbs, "get") || containsString(perm.Verbs, "list")) {
				analysis.DangerousAccess = append(analysis.DangerousAccess, DangerousAccess{
					Subject:     subject,
					Binding:     binding.Name,
					Reason:      "Can read secrets",
					Permissions: []string{"secrets: " + strings.Join(perm.Verbs, ", ")},
				})
			}

			// Check for wildcards
			if containsString(perm.Verbs, "*") || containsString(perm.Resources, "*") {
				analysis.WildcardAccess = append(analysis.WildcardAccess, WildcardAccess{
					Subject:   subject,
					Binding:   binding.Name,
					Verbs:     perm.Verbs,
					Resources: perm.Resources,
				})
			}
		}
	}
}

func (s *Scanner) parseGatekeeperConstraint(r client.Resource) *PolicyInfo {
	var constraint struct {
		Kind     string `json:"kind"`
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			EnforcementAction string `json:"enforcementAction"`
			Match             struct {
				Kinds []struct {
					Kinds []string `json:"kinds"`
				} `json:"kinds"`
			} `json:"match"`
		} `json:"spec"`
		Status struct {
			TotalViolations int `json:"totalViolations"`
		} `json:"status"`
	}

	if err := json.Unmarshal(r.Raw, &constraint); err != nil {
		return nil
	}

	enforcement := constraint.Spec.EnforcementAction
	if enforcement == "" {
		enforcement = "deny"
	}

	var targets []string
	for _, k := range constraint.Spec.Match.Kinds {
		targets = append(targets, k.Kinds...)
	}

	return &PolicyInfo{
		Name:        constraint.Metadata.Name,
		Kind:        constraint.Kind,
		Enforcement: enforcement,
		Violations:  constraint.Status.TotalViolations,
		Targets:     targets,
	}
}

func (s *Scanner) parseKyvernoPolicy(r client.Resource) *PolicyInfo {
	var policy struct {
		Kind     string `json:"kind"`
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			ValidationFailureAction string `json:"validationFailureAction"`
			Rules                   []struct {
				Match struct {
					Resources struct {
						Kinds []string `json:"kinds"`
					} `json:"resources"`
				} `json:"match"`
			} `json:"rules"`
		} `json:"spec"`
	}

	if err := json.Unmarshal(r.Raw, &policy); err != nil {
		return nil
	}

	enforcement := policy.Spec.ValidationFailureAction
	if enforcement == "" {
		enforcement = "audit"
	}

	var targets []string
	for _, rule := range policy.Spec.Rules {
		targets = append(targets, rule.Match.Resources.Kinds...)
	}

	return &PolicyInfo{
		Name:        policy.Metadata.Name,
		Kind:        policy.Kind,
		Enforcement: enforcement,
		Targets:     targets,
	}
}

func (s *Scanner) parseNetworkPolicy(r client.Resource, podLabels map[string]string) *NetworkPolicyRule {
	var np struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			PodSelector struct {
				MatchLabels map[string]string `json:"matchLabels"`
			} `json:"podSelector"`
			PolicyTypes []string `json:"policyTypes"`
		} `json:"spec"`
	}

	if err := json.Unmarshal(r.Raw, &np); err != nil {
		return nil
	}

	// Check if policy applies to this pod
	if !matchLabels(np.Spec.PodSelector.MatchLabels, podLabels) {
		return nil
	}

	direction := "ingress"
	if len(np.Spec.PolicyTypes) > 0 {
		direction = strings.ToLower(np.Spec.PolicyTypes[0])
	}

	return &NetworkPolicyRule{
		PolicyName: np.Metadata.Name,
		Direction:  direction,
		Allowed:    []string{"See policy for details"},
	}
}

func (s *Scanner) calculateScore(summary *SecuritySummary) SecurityScore {
	score := SecurityScore{
		Categories: make(map[string]int),
		ScannedAt:  time.Now(),
	}

	// Start with 100 and deduct based on issues
	overall := 100

	// Deduct points based on severity
	overall -= summary.CriticalCount * 15
	overall -= summary.HighCount * 10
	overall -= summary.MediumCount * 5
	overall -= summary.LowCount * 2

	if overall < 0 {
		overall = 0
	}
	score.Overall = overall

	// Calculate grade
	switch {
	case overall >= 90:
		score.Grade = "A"
	case overall >= 80:
		score.Grade = "B"
	case overall >= 70:
		score.Grade = "C"
	case overall >= 60:
		score.Grade = "D"
	default:
		score.Grade = "F"
	}

	// Category scores (simplified)
	for cat, count := range summary.IssuesByCategory {
		catScore := 100 - (count * 10)
		if catScore < 0 {
			catScore = 0
		}
		score.Categories[string(cat)] = catScore
	}

	return score
}

func sortBySeverity(issues []SecurityIssue) []SecurityIssue {
	// Simple bubble sort by severity
	severityOrder := map[Severity]int{
		SeverityCritical: 0,
		SeverityHigh:     1,
		SeverityMedium:   2,
		SeverityLow:      3,
		SeverityInfo:     4,
	}

	result := make([]SecurityIssue, len(issues))
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

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func matchLabels(selector, labels map[string]string) bool {
	if len(selector) == 0 {
		return true // Empty selector matches all
	}
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}
