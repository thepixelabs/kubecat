// SPDX-License-Identifier: Apache-2.0

package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/thepixelabs/kubecat/internal/client"
)

// SchedulingAnalyzer analyzes scheduling-related issues for pods.
type SchedulingAnalyzer struct{}

// NewSchedulingAnalyzer creates a new scheduling analyzer.
func NewSchedulingAnalyzer() *SchedulingAnalyzer {
	return &SchedulingAnalyzer{}
}

// Name returns the analyzer name.
func (a *SchedulingAnalyzer) Name() string {
	return "scheduling"
}

// Category returns the issue category.
func (a *SchedulingAnalyzer) Category() Category {
	return CategoryScheduling
}

// Analyze analyzes a single resource for scheduling issues.
func (a *SchedulingAnalyzer) Analyze(ctx context.Context, cl client.ClusterClient, resource client.Resource) ([]Issue, error) {
	// Only analyze pods
	if resource.Kind != "Pod" && resource.Kind != "" {
		kind := strings.ToLower(resource.Kind)
		if kind != "pod" && kind != "pods" {
			return nil, nil
		}
	}

	// Parse the pod
	pod, err := parsePod(resource)
	if err != nil {
		return nil, err
	}

	// Only analyze pending pods
	if pod.Status.Phase != corev1.PodPending {
		return nil, nil
	}

	var issues []Issue

	// Get nodes for comparison
	nodes, err := getNodes(ctx, cl)
	if err != nil {
		return nil, err
	}

	// Check various scheduling constraints
	issues = append(issues, a.checkTolerations(resource, pod, nodes)...)
	issues = append(issues, a.checkNodeSelector(resource, pod, nodes)...)
	issues = append(issues, a.checkNodeAffinity(resource, pod, nodes)...)
	issues = append(issues, a.checkResources(resource, pod, nodes)...)

	return issues, nil
}

// Scan scans all pods for scheduling issues.
func (a *SchedulingAnalyzer) Scan(ctx context.Context, cl client.ClusterClient, namespace string) ([]Issue, error) {
	// List all pods
	pods, err := cl.List(ctx, "pods", client.ListOptions{
		Namespace: namespace,
		Limit:     10000,
	})
	if err != nil {
		return nil, err
	}

	var allIssues []Issue
	for _, resource := range pods.Items {
		issues, err := a.Analyze(ctx, cl, resource)
		if err != nil {
			continue
		}
		allIssues = append(allIssues, issues...)
	}

	return allIssues, nil
}

// checkTolerations checks if the pod has tolerations for node taints.
func (a *SchedulingAnalyzer) checkTolerations(resource client.Resource, pod *corev1.Pod, nodes []nodeInfo) []Issue {
	var issues []Issue

	// Get all unique taints from nodes
	taintedNodes := make(map[string][]corev1.Taint)
	for _, node := range nodes {
		if len(node.Taints) > 0 {
			taintedNodes[node.Name] = node.Taints
		}
	}

	if len(taintedNodes) == 0 {
		return nil
	}

	// Check which taints the pod doesn't tolerate
	untolerated := make(map[string][]string) // taint -> nodes with that taint
	for nodeName, taints := range taintedNodes {
		for _, taint := range taints {
			if !toleratesTaint(pod.Spec.Tolerations, taint) {
				key := fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect)
				untolerated[key] = append(untolerated[key], nodeName)
			}
		}
	}

	if len(untolerated) == 0 {
		return nil
	}

	// Count blocked nodes
	blockedNodes := make(map[string]bool)
	for _, nodeNames := range untolerated {
		for _, name := range nodeNames {
			blockedNodes[name] = true
		}
	}

	// Create issue if all nodes are blocked by taints
	if len(blockedNodes) >= len(nodes) {
		// Build fix YAML
		var tolerationYAML strings.Builder
		tolerationYAML.WriteString("tolerations:\n")
		for taint := range untolerated {
			parts := strings.SplitN(taint, "=", 2)
			key := parts[0]
			rest := ""
			if len(parts) > 1 {
				rest = parts[1]
			}
			valueParts := strings.SplitN(rest, ":", 2)
			value := valueParts[0]
			effect := ""
			if len(valueParts) > 1 {
				effect = valueParts[1]
			}

			tolerationYAML.WriteString(fmt.Sprintf("- key: \"%s\"\n", key))
			tolerationYAML.WriteString("  operator: \"Equal\"\n")
			if value != "" {
				tolerationYAML.WriteString(fmt.Sprintf("  value: \"%s\"\n", value))
			}
			if effect != "" {
				tolerationYAML.WriteString(fmt.Sprintf("  effect: \"%s\"\n", effect))
			}
		}

		issues = append(issues, Issue{
			ID:       "scheduling.tolerations.missing",
			Category: CategoryScheduling,
			Severity: SeverityCritical,
			Title:    "Missing tolerations",
			Message:  fmt.Sprintf("%d node(s) blocked by taints the pod doesn't tolerate", len(blockedNodes)),
			Resource: resource,
			Details: map[string]interface{}{
				"untolerated_taints": untolerated,
				"blocked_nodes":      len(blockedNodes),
				"total_nodes":        len(nodes),
			},
			Fixes: []Fix{
				{
					Description: "Add tolerations to pod spec",
					YAML:        tolerationYAML.String(),
				},
			},
			DetectedAt: time.Now(),
		})
	}

	return issues
}

// checkNodeSelector checks if node selector matches any nodes.
func (a *SchedulingAnalyzer) checkNodeSelector(resource client.Resource, pod *corev1.Pod, nodes []nodeInfo) []Issue {
	var issues []Issue

	if len(pod.Spec.NodeSelector) == 0 {
		return nil
	}

	// Find nodes that match the selector
	var matchingNodes []string
	for _, node := range nodes {
		if matchesLabels(node.Labels, pod.Spec.NodeSelector) {
			matchingNodes = append(matchingNodes, node.Name)
		}
	}

	if len(matchingNodes) > 0 {
		return nil
	}

	// No matching nodes - create issue
	selectorStr := formatLabels(pod.Spec.NodeSelector)

	issues = append(issues, Issue{
		ID:       "scheduling.nodeselector.nomatch",
		Category: CategoryScheduling,
		Severity: SeverityCritical,
		Title:    "Node selector has no matches",
		Message:  fmt.Sprintf("No nodes match selector: %s", selectorStr),
		Resource: resource,
		Details: map[string]interface{}{
			"node_selector":  pod.Spec.NodeSelector,
			"matching_nodes": 0,
			"total_nodes":    len(nodes),
		},
		Fixes: []Fix{
			{
				Description: "Remove or modify node selector, or add matching labels to nodes",
				Command:     fmt.Sprintf("kubectl label nodes <node-name> %s", selectorStr),
			},
		},
		DetectedAt: time.Now(),
	})

	return issues
}

// checkNodeAffinity checks if node affinity matches any nodes.
func (a *SchedulingAnalyzer) checkNodeAffinity(resource client.Resource, pod *corev1.Pod, nodes []nodeInfo) []Issue {
	var issues []Issue

	if pod.Spec.Affinity == nil || pod.Spec.Affinity.NodeAffinity == nil {
		return nil
	}

	affinity := pod.Spec.Affinity.NodeAffinity
	if affinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		return nil
	}

	required := affinity.RequiredDuringSchedulingIgnoredDuringExecution

	// Check if any node matches the required affinity
	var matchingNodes []string
	for _, node := range nodes {
		if matchesNodeSelectorTerms(node.Labels, required.NodeSelectorTerms) {
			matchingNodes = append(matchingNodes, node.Name)
		}
	}

	if len(matchingNodes) > 0 {
		return nil
	}

	// Build a description of the affinity requirements
	var requirements []string
	for _, term := range required.NodeSelectorTerms {
		for _, expr := range term.MatchExpressions {
			req := fmt.Sprintf("%s %s %v", expr.Key, expr.Operator, expr.Values)
			requirements = append(requirements, req)
		}
	}

	issues = append(issues, Issue{
		ID:       "scheduling.affinity.nomatch",
		Category: CategoryScheduling,
		Severity: SeverityCritical,
		Title:    "Node affinity has no matches",
		Message:  fmt.Sprintf("No nodes match required affinity: %s", strings.Join(requirements, ", ")),
		Resource: resource,
		Details: map[string]interface{}{
			"affinity_requirements": requirements,
			"matching_nodes":        0,
			"total_nodes":           len(nodes),
		},
		Fixes: []Fix{
			{
				Description: "Modify pod affinity or add matching labels to nodes",
			},
		},
		DetectedAt: time.Now(),
	})

	return issues
}

// checkResources checks if the pod's resource requests can be satisfied.
func (a *SchedulingAnalyzer) checkResources(resource client.Resource, pod *corev1.Pod, nodes []nodeInfo) []Issue {
	var issues []Issue

	// Calculate total resource requests
	var totalCPU, totalMem int64
	for _, container := range pod.Spec.Containers {
		if container.Resources.Requests != nil {
			if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
				totalCPU += cpu.MilliValue()
			}
			if mem, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
				totalMem += mem.Value()
			}
		}
	}

	if totalCPU == 0 && totalMem == 0 {
		return nil
	}

	// Check if any node can satisfy the requests
	var fittingNodes []string
	for _, node := range nodes {
		if node.AllocatableCPU >= totalCPU && node.AllocatableMem >= totalMem {
			fittingNodes = append(fittingNodes, node.Name)
		}
	}

	if len(fittingNodes) > 0 {
		return nil
	}

	// No node can fit - create issue
	cpuStr := fmt.Sprintf("%dm", totalCPU)
	memStr := formatBytes(totalMem)

	var maxCPU, maxMem int64
	for _, node := range nodes {
		if node.AllocatableCPU > maxCPU {
			maxCPU = node.AllocatableCPU
		}
		if node.AllocatableMem > maxMem {
			maxMem = node.AllocatableMem
		}
	}

	issues = append(issues, Issue{
		ID:       "scheduling.resources.insufficient",
		Category: CategoryScheduling,
		Severity: SeverityCritical,
		Title:    "Insufficient resources",
		Message:  fmt.Sprintf("Requested cpu=%s, memory=%s; max available: cpu=%dm, memory=%s", cpuStr, memStr, maxCPU, formatBytes(maxMem)),
		Resource: resource,
		Details: map[string]interface{}{
			"requested_cpu":    totalCPU,
			"requested_memory": totalMem,
			"max_cpu":          maxCPU,
			"max_memory":       maxMem,
		},
		Fixes: []Fix{
			{
				Description: "Reduce resource requests or add nodes with more capacity",
			},
		},
		DetectedAt: time.Now(),
	})

	return issues
}

// nodeInfo holds relevant node information for scheduling analysis.
type nodeInfo struct {
	Name           string
	Labels         map[string]string
	Taints         []corev1.Taint
	AllocatableCPU int64 // millicores
	AllocatableMem int64 // bytes
	Ready          bool
}

// getNodes retrieves all nodes with their scheduling-relevant info.
func getNodes(ctx context.Context, cl client.ClusterClient) ([]nodeInfo, error) {
	nodeList, err := cl.List(ctx, "nodes", client.ListOptions{
		Limit: 1000,
	})
	if err != nil {
		return nil, err
	}

	var nodes []nodeInfo
	for _, resource := range nodeList.Items {
		node, err := parseNodeInfo(resource)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// parseNodeInfo parses a node resource into nodeInfo.
func parseNodeInfo(resource client.Resource) (nodeInfo, error) {
	var node struct {
		Metadata struct {
			Name   string            `json:"name"`
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
		Spec struct {
			Taints []corev1.Taint `json:"taints"`
		} `json:"spec"`
		Status struct {
			Allocatable map[string]string `json:"allocatable"`
			Conditions  []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"conditions"`
		} `json:"status"`
	}

	if err := json.Unmarshal(resource.Raw, &node); err != nil {
		return nodeInfo{}, err
	}

	info := nodeInfo{
		Name:   node.Metadata.Name,
		Labels: node.Metadata.Labels,
		Taints: node.Spec.Taints,
	}

	// Parse allocatable resources
	if cpu, ok := node.Status.Allocatable["cpu"]; ok {
		info.AllocatableCPU = parseCPU(cpu)
	}
	if mem, ok := node.Status.Allocatable["memory"]; ok {
		info.AllocatableMem = parseMemory(mem)
	}

	// Check if node is ready
	for _, cond := range node.Status.Conditions {
		if cond.Type == "Ready" && cond.Status == "True" {
			info.Ready = true
			break
		}
	}

	return info, nil
}

// parsePod parses a resource into a Pod.
func parsePod(resource client.Resource) (*corev1.Pod, error) {
	var pod corev1.Pod
	if err := json.Unmarshal(resource.Raw, &pod); err != nil {
		return nil, err
	}
	return &pod, nil
}

// toleratesTaint checks if tolerations include a toleration for the taint.
func toleratesTaint(tolerations []corev1.Toleration, taint corev1.Taint) bool {
	for _, toleration := range tolerations {
		if tolerationMatchesTaint(toleration, taint) {
			return true
		}
	}
	return false
}

// tolerationMatchesTaint checks if a toleration matches a taint.
func tolerationMatchesTaint(toleration corev1.Toleration, taint corev1.Taint) bool {
	// Empty key with Exists operator matches all taints
	if toleration.Key == "" && toleration.Operator == corev1.TolerationOpExists {
		return true
	}

	// Key must match
	if toleration.Key != taint.Key {
		return false
	}

	// Check effect (empty matches all)
	if toleration.Effect != "" && toleration.Effect != taint.Effect {
		return false
	}

	// Check operator
	switch toleration.Operator {
	case corev1.TolerationOpExists:
		return true
	case corev1.TolerationOpEqual, "":
		return toleration.Value == taint.Value
	}

	return false
}

// matchesLabels checks if a node's labels match a selector.
func matchesLabels(nodeLabels, selector map[string]string) bool {
	for key, value := range selector {
		if nodeLabels[key] != value {
			return false
		}
	}
	return true
}

// matchesNodeSelectorTerms checks if node labels match any of the selector terms.
func matchesNodeSelectorTerms(labels map[string]string, terms []corev1.NodeSelectorTerm) bool {
	for _, term := range terms {
		if matchesNodeSelectorTerm(labels, term) {
			return true
		}
	}
	return false
}

// matchesNodeSelectorTerm checks if node labels match a selector term.
func matchesNodeSelectorTerm(labels map[string]string, term corev1.NodeSelectorTerm) bool {
	for _, expr := range term.MatchExpressions {
		if !matchesExpression(labels, expr) {
			return false
		}
	}
	for _, field := range term.MatchFields {
		if !matchesExpression(labels, field) {
			return false
		}
	}
	return true
}

// matchesExpression checks if labels match a label selector expression.
func matchesExpression(labels map[string]string, expr corev1.NodeSelectorRequirement) bool {
	value, exists := labels[expr.Key]

	switch expr.Operator {
	case corev1.NodeSelectorOpIn:
		for _, v := range expr.Values {
			if v == value {
				return true
			}
		}
		return false
	case corev1.NodeSelectorOpNotIn:
		for _, v := range expr.Values {
			if v == value {
				return false
			}
		}
		return true
	case corev1.NodeSelectorOpExists:
		return exists
	case corev1.NodeSelectorOpDoesNotExist:
		return !exists
	case corev1.NodeSelectorOpGt, corev1.NodeSelectorOpLt:
		// Simplified - would need numeric comparison
		return true
	}

	return false
}

// formatLabels formats labels as a string.
func formatLabels(labels map[string]string) string {
	var parts []string
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ",")
}

// formatBytes formats bytes as human-readable.
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%dGi", bytes/GB)
	case bytes >= MB:
		return fmt.Sprintf("%dMi", bytes/MB)
	case bytes >= KB:
		return fmt.Sprintf("%dKi", bytes/KB)
	default:
		return fmt.Sprintf("%d", bytes)
	}
}

// parseCPU parses a CPU string (e.g., "2", "500m") to millicores.
func parseCPU(cpu string) int64 {
	if strings.HasSuffix(cpu, "m") {
		var m int64
		_, _ = fmt.Sscanf(cpu, "%dm", &m)
		return m
	}
	var cores int64
	_, _ = fmt.Sscanf(cpu, "%d", &cores)
	return cores * 1000
}

// parseMemory parses a memory string (e.g., "8Gi", "512Mi") to bytes.
func parseMemory(mem string) int64 {
	const (
		Ki = 1024
		Mi = Ki * 1024
		Gi = Mi * 1024
		Ti = Gi * 1024
	)

	var value int64
	if strings.HasSuffix(mem, "Ki") {
		_, _ = fmt.Sscanf(mem, "%dKi", &value)
		return value * Ki
	}
	if strings.HasSuffix(mem, "Mi") {
		_, _ = fmt.Sscanf(mem, "%dMi", &value)
		return value * Mi
	}
	if strings.HasSuffix(mem, "Gi") {
		_, _ = fmt.Sscanf(mem, "%dGi", &value)
		return value * Gi
	}
	if strings.HasSuffix(mem, "Ti") {
		_, _ = fmt.Sscanf(mem, "%dTi", &value)
		return value * Ti
	}
	// Plain bytes
	_, _ = fmt.Sscanf(mem, "%d", &value)
	return value
}

func init() {
	// Register the scheduling analyzer with the default registry
	Register(NewSchedulingAnalyzer())
}
