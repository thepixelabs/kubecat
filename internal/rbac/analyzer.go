// SPDX-License-Identifier: Apache-2.0

// Package rbac provides read-only RBAC analysis for Kubernetes namespaces.
package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thepixelabs/kubecat/internal/client"
)

// SubjectKind is the kind of a RBAC subject.
type SubjectKind string

const (
	SubjectUser           SubjectKind = "User"
	SubjectGroup          SubjectKind = "Group"
	SubjectServiceAccount SubjectKind = "ServiceAccount"
)

// PolicyRule is a simplified RBAC rule.
type PolicyRule struct {
	Verbs      []string `json:"verbs"`
	Resources  []string `json:"resources"`
	APIGroups  []string `json:"apiGroups"`
	IsWildcard bool     `json:"isWildcard"` // true when verbs or resources contain "*"
}

// RBACBinding is a single role binding reference.
type RBACBinding struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"` // RoleBinding | ClusterRoleBinding
	RoleName    string `json:"roleName"`
	RoleKind    string `json:"roleKind"` // Role | ClusterRole
	Namespace   string `json:"namespace"`
	ClusterWide bool   `json:"clusterWide"`
}

// SubjectPermissions holds all RBAC info for one subject.
type SubjectPermissions struct {
	Subject  string        `json:"subject"`
	Kind     SubjectKind   `json:"kind"`
	Bindings []RBACBinding `json:"bindings"`
	Rules    []PolicyRule  `json:"rules"`
}

// RBACMatrix is the full permission picture for a namespace.
type RBACMatrix struct {
	Namespace string               `json:"namespace"`
	Subjects  []SubjectPermissions `json:"subjects"`
	Warning   string               `json:"warning,omitempty"`
}

// ListNamespaceRBAC returns all subjects with bindings affecting namespace.
func ListNamespaceRBAC(ctx context.Context, cl client.ClusterClient, namespace string) (*RBACMatrix, error) {
	matrix := &RBACMatrix{Namespace: namespace}

	// RoleBindings in the namespace
	rbList, err := cl.List(ctx, "rolebindings", client.ListOptions{Namespace: namespace, Limit: 500})
	if err != nil {
		matrix.Warning = fmt.Sprintf("Cannot list RoleBindings: %v", err)
		return matrix, nil
	}

	// ClusterRoleBindings (cluster-scoped)
	crbList, err := cl.List(ctx, "clusterrolebindings", client.ListOptions{Limit: 500})
	if err != nil {
		// Non-fatal — may lack cluster-level RBAC read permission
		matrix.Warning = "Insufficient permissions to list ClusterRoleBindings"
	}

	subjectMap := map[string]*SubjectPermissions{}

	addBinding := func(subjectName string, kind SubjectKind, binding RBACBinding) {
		key := string(kind) + "/" + subjectName
		sp, ok := subjectMap[key]
		if !ok {
			sp = &SubjectPermissions{Subject: subjectName, Kind: kind}
			subjectMap[key] = sp
		}
		sp.Bindings = append(sp.Bindings, binding)
	}

	for _, r := range rbList.Items {
		rb := parseRoleBinding(r)
		if rb == nil {
			continue
		}
		binding := RBACBinding{
			Name:      rb.Name,
			Kind:      "RoleBinding",
			RoleName:  rb.RoleRef.Name,
			RoleKind:  rb.RoleRef.Kind,
			Namespace: namespace,
		}
		for _, s := range rb.Subjects {
			addBinding(s.Name, SubjectKind(s.Kind), binding)
		}
	}

	if crbList != nil {
		for _, r := range crbList.Items {
			crb := parseClusterRoleBinding(r)
			if crb == nil {
				continue
			}
			binding := RBACBinding{
				Name:        crb.Name,
				Kind:        "ClusterRoleBinding",
				RoleName:    crb.RoleRef.Name,
				RoleKind:    crb.RoleRef.Kind,
				ClusterWide: true,
			}
			for _, s := range crb.Subjects {
				addBinding(s.Name, SubjectKind(s.Kind), binding)
			}
		}
	}

	// Fetch rules for each unique role referenced
	roleRules := map[string][]PolicyRule{}
	for _, sp := range subjectMap {
		for _, b := range sp.Bindings {
			key := b.RoleKind + "/" + b.RoleName
			if _, seen := roleRules[key]; seen {
				continue
			}
			rules, err := fetchRoleRules(ctx, cl, b.RoleKind, b.RoleName, namespace)
			if err == nil {
				roleRules[key] = rules
			}
		}
	}

	// Assign flattened rules to each subject
	for _, sp := range subjectMap {
		seen := map[string]bool{}
		for _, b := range sp.Bindings {
			key := b.RoleKind + "/" + b.RoleName
			for _, rule := range roleRules[key] {
				ruleKey := strings.Join(rule.Verbs, ",") + "|" + strings.Join(rule.Resources, ",")
				if !seen[ruleKey] {
					seen[ruleKey] = true
					sp.Rules = append(sp.Rules, rule)
				}
			}
		}
		matrix.Subjects = append(matrix.Subjects, *sp)
	}

	return matrix, nil
}

// ── internal parsing ──────────────────────────────────────────────────────────

type k8sSubject struct{ Kind, Name, Namespace string }
type k8sRoleRef struct{ Kind, Name string }

type k8sRoleBinding struct {
	Name     string
	Subjects []k8sSubject
	RoleRef  k8sRoleRef
}

func parseRoleBinding(r client.Resource) *k8sRoleBinding {
	var rb struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Subjects []struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"subjects"`
		RoleRef struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"roleRef"`
	}
	if err := json.Unmarshal(r.Raw, &rb); err != nil {
		return nil
	}
	result := &k8sRoleBinding{Name: rb.Metadata.Name, RoleRef: k8sRoleRef{Kind: rb.RoleRef.Kind, Name: rb.RoleRef.Name}}
	for _, s := range rb.Subjects {
		result.Subjects = append(result.Subjects, k8sSubject{Kind: s.Kind, Name: s.Name, Namespace: s.Namespace})
	}
	return result
}

func parseClusterRoleBinding(r client.Resource) *k8sRoleBinding {
	return parseRoleBinding(r) // same wire format
}

func fetchRoleRules(ctx context.Context, cl client.ClusterClient, roleKind, roleName, namespace string) ([]PolicyRule, error) {
	var kind string
	var ns string
	switch roleKind {
	case "ClusterRole":
		kind = "clusterroles"
	default:
		kind = "roles"
		ns = namespace
	}

	r, err := cl.Get(ctx, kind, ns, roleName)
	if err != nil {
		return nil, err
	}

	var role struct {
		Rules []struct {
			Verbs     []string `json:"verbs"`
			Resources []string `json:"resources"`
			APIGroups []string `json:"apiGroups"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(r.Raw, &role); err != nil {
		return nil, err
	}

	rules := make([]PolicyRule, 0, len(role.Rules))
	for _, rule := range role.Rules {
		pr := PolicyRule{
			Verbs:     rule.Verbs,
			Resources: rule.Resources,
			APIGroups: rule.APIGroups,
		}
		for _, v := range rule.Verbs {
			if v == "*" {
				pr.IsWildcard = true
				break
			}
		}
		for _, res := range rule.Resources {
			if res == "*" {
				pr.IsWildcard = true
				break
			}
		}
		rules = append(rules, pr)
	}
	return rules, nil
}
