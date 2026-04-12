// SPDX-License-Identifier: Apache-2.0

package mcp

// ToolDefinition is a single MCP tool descriptor.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolDefinitions returns all tools exposed by the kubecat MCP server.
func ToolDefinitions() []ToolDefinition {
	str := func(desc string) map[string]interface{} {
		return map[string]interface{}{"type": "string", "description": desc}
	}
	arr := func(desc string) map[string]interface{} {
		return map[string]interface{}{
			"type":        "array",
			"description": desc,
			"items":       map[string]interface{}{"type": "string"},
		}
	}
	props := func(required []string, fields map[string]map[string]interface{}) map[string]interface{} {
		p := make(map[string]interface{}, len(fields))
		for k, v := range fields {
			p[k] = v
		}
		return map[string]interface{}{
			"type":       "object",
			"properties": p,
			"required":   required,
		}
	}

	return []ToolDefinition{
		{
			Name:        "list_clusters",
			Description: "List all connected Kubernetes clusters and their status.",
			InputSchema: props(nil, nil),
		},
		{
			Name:        "get_resource",
			Description: "Get a single Kubernetes resource as JSON.",
			InputSchema: props(
				[]string{"cluster", "namespace", "kind", "name"},
				map[string]map[string]interface{}{
					"cluster":   str("Kubeconfig context name"),
					"namespace": str("Kubernetes namespace"),
					"kind":      str("Resource kind, e.g. Pod, Deployment"),
					"name":      str("Resource name"),
				},
			),
		},
		{
			Name:        "list_resources",
			Description: "List Kubernetes resources of a given kind in a namespace.",
			InputSchema: props(
				[]string{"cluster", "namespace", "kind"},
				map[string]map[string]interface{}{
					"cluster":   str("Kubeconfig context name"),
					"namespace": str("Kubernetes namespace (empty = all namespaces)"),
					"kind":      str("Resource kind, e.g. Pod, Service, Deployment"),
				},
			),
		},
		{
			Name:        "get_events",
			Description: "Get recent Kubernetes events for a resource.",
			InputSchema: props(
				[]string{"cluster", "namespace"},
				map[string]map[string]interface{}{
					"cluster":       str("Kubeconfig context name"),
					"namespace":     str("Kubernetes namespace"),
					"resource_name": str("Optional: filter events by involved object name"),
				},
			),
		},
		{
			Name:        "exec_kubectl",
			Description: "Run a read-only kubectl command (get, describe, logs, top only).",
			InputSchema: props(
				[]string{"cluster", "args"},
				map[string]map[string]interface{}{
					"cluster": str("Kubeconfig context name"),
					"args":    arr("kubectl arguments, e.g. [\"get\", \"pods\", \"-n\", \"default\"]"),
				},
			),
		},
		{
			Name:        "ai_query",
			Description: "Run an AI analysis query against the cluster context.",
			InputSchema: props(
				[]string{"question"},
				map[string]map[string]interface{}{
					"question": str("Natural language question about the cluster"),
					"context":  str("Optional additional context"),
				},
			),
		},
	}
}
