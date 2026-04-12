// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
)

// ClusterHealthInfo contains health information for a cluster.
type ClusterHealthInfo struct {
	Context     string `json:"context"`
	Status      string `json:"status"`
	NodeCount   int    `json:"nodeCount"`
	PodCount    int    `json:"podCount"`
	CPUPercent  int    `json:"cpuPercent"`
	MemPercent  int    `json:"memPercent"`
	Issues      int    `json:"issues"`
	LastChecked string `json:"lastChecked"`
}

// CrossClusterSearchResult contains search results across clusters.
type CrossClusterSearchResult struct {
	Cluster   string         `json:"cluster"`
	Resources []ResourceInfo `json:"resources"`
	Error     string         `json:"error,omitempty"`
}

// GetMultiClusterHealth returns health info for all connected clusters.
func (a *App) GetMultiClusterHealth() ([]ClusterHealthInfo, error) {
	var results []ClusterHealthInfo

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	manager := a.nexus.Clusters.Manager()
	clusters := manager.List()

	for _, info := range clusters {
		health := ClusterHealthInfo{
			Context:     info.Context,
			Status:      info.Status.String(),
			LastChecked: time.Now().Format(time.RFC3339),
		}

		cl, err := manager.Get(info.Context)
		if err != nil {
			health.Status = "Error"
			results = append(results, health)
			continue
		}

		nodes, err := cl.List(ctx, "nodes", client.ListOptions{Limit: 100})
		if err == nil {
			health.NodeCount = len(nodes.Items)
		}

		pods, err := cl.List(ctx, "pods", client.ListOptions{Limit: 10000})
		if err == nil {
			health.PodCount = len(pods.Items)
		}

		for _, pod := range pods.Items {
			if pod.Status != "Running" && pod.Status != "Succeeded" && pod.Status != "Completed" {
				health.Issues++
			}
		}

		results = append(results, health)
	}

	return results, nil
}

// SearchAcrossClusters searches for resources across all connected clusters.
func (a *App) SearchAcrossClusters(kind, query string) ([]CrossClusterSearchResult, error) {
	var results []CrossClusterSearchResult

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	manager := a.nexus.Clusters.Manager()
	clusters := manager.List()

	for _, info := range clusters {
		result := CrossClusterSearchResult{
			Cluster:   info.Context,
			Resources: []ResourceInfo{},
		}

		cl, err := manager.Get(info.Context)
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}

		resources, err := cl.List(ctx, kind, client.ListOptions{Limit: 1000})
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}

		for _, r := range resources.Items {
			if query != "" {
				if !stringContainsIgnoreCase(r.Name, query) && !stringContainsIgnoreCase(r.Namespace, query) {
					continue
				}
			}

			result.Resources = append(result.Resources, ResourceInfo{
				Kind:      r.Kind,
				Name:      r.Name,
				Namespace: r.Namespace,
				Status:    r.Status,
				Age:       formatDuration(time.Since(r.CreatedAt)),
			})
		}

		results = append(results, result)
	}

	return results, nil
}

func stringContainsIgnoreCase(s, substr string) bool {
	sLower := stringToLower(s)
	substrLower := stringToLower(substr)
	return len(sLower) >= len(substrLower) && stringContains(sLower, substrLower)
}

func stringContains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func stringToLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}
