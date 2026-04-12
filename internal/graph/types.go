// SPDX-License-Identifier: Apache-2.0

// Package graph computes the resource relationship graph used by the cluster
// visualizer.
package graph

// ResourceNode represents a single Kubernetes resource in the graph.
type ResourceNode struct {
	// ID is the unique node identifier: "Kind/namespace/name"
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Status    string            `json:"status"`
	Labels    map[string]string `json:"labels"`
}

// Edge represents a directed relationship between two nodes.
type Edge struct {
	// ID is a unique edge identifier.
	ID       string `json:"id"`
	Source   string `json:"source"` // ResourceNode.ID
	Target   string `json:"target"` // ResourceNode.ID
	EdgeType string `json:"edgeType"`
	Label    string `json:"label,omitempty"`
}

// Graph holds the full set of nodes and edges.
type Graph struct {
	Nodes []ResourceNode `json:"nodes"`
	Edges []Edge         `json:"edges"`
}
