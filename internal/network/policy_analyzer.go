// Package network provides NetworkPolicy analysis and visualization helpers.
package network

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thepixelabs/kubecat/internal/client"
)

// NodeKind classifies a graph node.
type NodeKind string

const (
	NodeKindPod      NodeKind = "Pod"
	NodeKindService  NodeKind = "Service"
	NodeKindExternal NodeKind = "External"
)

// NetworkNode is a vertex in the connectivity graph.
type NetworkNode struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Kind      NodeKind          `json:"kind"`
	Labels    map[string]string `json:"labels"`
}

// NetworkEdge is a directed connection between two nodes.
type NetworkEdge struct {
	ID         string `json:"id"`
	Source     string `json:"source"`
	Target     string `json:"target"`
	Allowed    bool   `json:"allowed"`
	Direction  string `json:"direction"` // "ingress" | "egress" | "both"
	PolicyName string `json:"policyName"`
	Ports      string `json:"ports"` // human-readable, e.g. "TCP:80,8080"
}

// NetworkGraph is the full connectivity model for a namespace.
type NetworkGraph struct {
	Nodes       []NetworkNode `json:"nodes"`
	Edges       []NetworkEdge `json:"edges"`
	HasPolicies bool          `json:"hasPolicies"`
	Warning     string        `json:"warning,omitempty"`
}

// AnalyzeNamespace returns the connectivity graph for the given namespace.
// If no NetworkPolicies exist the graph is fully-connected (all pods can talk
// to all). If policies exist only explicitly allowed edges are returned.
func AnalyzeNamespace(ctx context.Context, cl client.ClusterClient, namespace string) (*NetworkGraph, error) {
	graph := &NetworkGraph{}

	// --- fetch pods ---
	podList, err := cl.List(ctx, "pods", client.ListOptions{Namespace: namespace})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	for _, r := range podList.Items {
		labels := extractLabels(r.Object)
		graph.Nodes = append(graph.Nodes, NetworkNode{
			ID:        "pod/" + r.Name,
			Name:      r.Name,
			Namespace: namespace,
			Kind:      NodeKindPod,
			Labels:    labels,
		})
	}

	// --- fetch services ---
	svcList, err := cl.List(ctx, "services", client.ListOptions{Namespace: namespace})
	if err == nil {
		for _, r := range svcList.Items {
			graph.Nodes = append(graph.Nodes, NetworkNode{
				ID:        "svc/" + r.Name,
				Name:      r.Name,
				Namespace: namespace,
				Kind:      NodeKindService,
				Labels:    extractLabels(r.Object),
			})
		}
	}

	// --- fetch network policies ---
	npList, err := cl.List(ctx, "networkpolicies", client.ListOptions{Namespace: namespace})
	if err != nil || len(npList.Items) == 0 {
		// No policies → fully connected; add a single synthetic "allow all" edge
		graph.HasPolicies = false
		if len(graph.Nodes) >= 2 {
			graph.Edges = append(graph.Edges, NetworkEdge{
				ID:         "allow-all",
				Source:     "*",
				Target:     "*",
				Allowed:    true,
				Direction:  "both",
				PolicyName: "(no policies — fully open)",
			})
		}
		return graph, nil
	}

	graph.HasPolicies = true

	// Build pod-to-pod edges from policies.
	edgeSet := map[string]NetworkEdge{}
	for _, np := range npList.Items {
		policy := parseNetworkPolicy(np.Object)
		if policy == nil {
			continue
		}

		// Pods selected by this policy
		selectedPods := selectPods(graph.Nodes, policy.Spec.PodSelector.MatchLabels)

		for _, targetPod := range selectedPods {
			// Ingress rules
			for _, rule := range policy.Spec.Ingress {
				if len(rule.From) == 0 {
					// Allow all ingress
					edgeKey := "all→" + targetPod.ID
					edgeSet[edgeKey] = NetworkEdge{
						ID:         edgeKey,
						Source:     "*",
						Target:     targetPod.ID,
						Allowed:    true,
						Direction:  "ingress",
						PolicyName: policy.Name,
						Ports:      formatPorts(rule.Ports),
					}
					continue
				}
				for _, peer := range rule.From {
					sourcePods := selectPods(graph.Nodes, peer.PodSelector.MatchLabels)
					for _, src := range sourcePods {
						edgeKey := src.ID + "→" + targetPod.ID
						edgeSet[edgeKey] = NetworkEdge{
							ID:         edgeKey,
							Source:     src.ID,
							Target:     targetPod.ID,
							Allowed:    true,
							Direction:  "ingress",
							PolicyName: policy.Name,
							Ports:      formatPorts(rule.Ports),
						}
					}
					// Namespace selector — represent as external
					if peer.NamespaceSelector != nil {
						extID := "external/other-ns"
						addExternalNode(graph, extID)
						edgeKey := extID + "→" + targetPod.ID
						edgeSet[edgeKey] = NetworkEdge{
							ID:         edgeKey,
							Source:     extID,
							Target:     targetPod.ID,
							Allowed:    true,
							Direction:  "ingress",
							PolicyName: policy.Name,
							Ports:      formatPorts(rule.Ports),
						}
					}
				}
			}
		}
	}

	for _, e := range edgeSet {
		graph.Edges = append(graph.Edges, e)
	}
	return graph, nil
}

// ---- internal helpers ----

type netpolSpec struct {
	Name string
	Spec struct {
		PodSelector struct {
			MatchLabels map[string]string `json:"matchLabels"`
		} `json:"podSelector"`
		Ingress []struct {
			From []struct {
				PodSelector *struct {
					MatchLabels map[string]string `json:"matchLabels"`
				} `json:"podSelector"`
				NamespaceSelector *struct {
					MatchLabels map[string]string `json:"matchLabels"`
				} `json:"namespaceSelector"`
			} `json:"from"`
			Ports []struct {
				Protocol string      `json:"protocol"`
				Port     interface{} `json:"port"`
			} `json:"ports"`
		} `json:"ingress"`
	} `json:"spec"`
}

func parseNetworkPolicy(obj map[string]interface{}) *netpolSpec {
	if obj == nil {
		return nil
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return nil
	}
	var p netpolSpec
	if err := json.Unmarshal(b, &p); err != nil {
		return nil
	}
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		if name, ok := meta["name"].(string); ok {
			p.Name = name
		}
	}
	return &p
}

func extractLabels(obj map[string]interface{}) map[string]string {
	if obj == nil {
		return nil
	}
	meta, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}
	labels, ok := meta["labels"].(map[string]interface{})
	if !ok {
		return nil
	}
	result := make(map[string]string, len(labels))
	for k, v := range labels {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result
}

func selectPods(nodes []NetworkNode, matchLabels map[string]string) []NetworkNode {
	if len(matchLabels) == 0 {
		// Empty selector matches all pods
		var pods []NetworkNode
		for _, n := range nodes {
			if n.Kind == NodeKindPod {
				pods = append(pods, n)
			}
		}
		return pods
	}
	var matched []NetworkNode
	for _, n := range nodes {
		if n.Kind != NodeKindPod {
			continue
		}
		if labelsMatch(n.Labels, matchLabels) {
			matched = append(matched, n)
		}
	}
	return matched
}

func labelsMatch(nodeLabels, selector map[string]string) bool {
	for k, v := range selector {
		if nodeLabels[k] != v {
			return false
		}
	}
	return true
}

func addExternalNode(graph *NetworkGraph, id string) {
	for _, n := range graph.Nodes {
		if n.ID == id {
			return
		}
	}
	graph.Nodes = append(graph.Nodes, NetworkNode{
		ID:   id,
		Name: "External (other namespace)",
		Kind: NodeKindExternal,
	})
}

func formatPorts(ports []struct {
	Protocol string      `json:"protocol"`
	Port     interface{} `json:"port"`
}) string {
	if len(ports) == 0 {
		return "all"
	}
	result := ""
	for i, p := range ports {
		if i > 0 {
			result += ","
		}
		proto := p.Protocol
		if proto == "" {
			proto = "TCP"
		}
		result += fmt.Sprintf("%s:%v", proto, p.Port)
	}
	return result
}
