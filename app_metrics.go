package main

import (
	"context"
	"encoding/json"

	"github.com/thepixelabs/kubecat/internal/client"
)

// NodeMetricsInfo contains resource utilization metrics for a node.
type NodeMetricsInfo struct {
	NodeName       string `json:"nodeName"`
	PodCount       int    `json:"podCount"`
	CPURequests    string `json:"cpuRequests"`    // Total CPU requests from pods
	CPULimits      string `json:"cpuLimits"`      // Total CPU limits from pods
	MemRequests    string `json:"memRequests"`    // Total memory requests from pods
	MemLimits      string `json:"memLimits"`      // Total memory limits from pods
	CPUAllocatable string `json:"cpuAllocatable"` // Node allocatable CPU
	MemAllocatable string `json:"memAllocatable"` // Node allocatable memory
	CPURequestPct  int    `json:"cpuRequestPct"`  // CPU request percentage of allocatable
	MemRequestPct  int    `json:"memRequestPct"`  // Memory request percentage of allocatable
	CPULimitPct    int    `json:"cpuLimitPct"`    // CPU limit percentage of allocatable
	MemLimitPct    int    `json:"memLimitPct"`    // Memory limit percentage of allocatable
}

// GetNodeMetrics returns resource utilization metrics for all nodes.
func (a *App) GetNodeMetrics() ([]NodeMetricsInfo, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	cl, err := a.nexus.Clusters.Manager().Active()
	if err != nil {
		return nil, err
	}

	// Get all nodes
	nodes, err := cl.List(ctx, "nodes", client.ListOptions{Limit: 1000})
	if err != nil {
		return nil, err
	}

	// Get all pods
	pods, err := cl.List(ctx, "pods", client.ListOptions{Limit: 10000})
	if err != nil {
		return nil, err
	}

	// Build a map of node allocatable resources
	type nodeResources struct {
		cpuAllocatable int64
		memAllocatable int64
	}
	nodeAlloc := make(map[string]nodeResources)

	for _, node := range nodes.Items {
		var nodeObj map[string]interface{}
		if err := json.Unmarshal(node.Raw, &nodeObj); err != nil {
			continue
		}
		if status, ok := nodeObj["status"].(map[string]interface{}); ok {
			if alloc, ok := status["allocatable"].(map[string]interface{}); ok {
				nr := nodeResources{}
				if cpu, ok := alloc["cpu"].(string); ok {
					nr.cpuAllocatable = parseResourceQuantity(cpu) * 1000 // convert to millicores
				}
				if mem, ok := alloc["memory"].(string); ok {
					nr.memAllocatable = parseResourceQuantity(mem)
				}
				nodeAlloc[node.Name] = nr
			}
		}
	}

	// Aggregate pod resources per node
	type podAggregation struct {
		podCount    int
		cpuRequests int64
		cpuLimits   int64
		memRequests int64
		memLimits   int64
	}
	nodePods := make(map[string]*podAggregation)

	// Initialize all nodes
	for _, node := range nodes.Items {
		nodePods[node.Name] = &podAggregation{}
	}

	for _, pod := range pods.Items {
		var podObj map[string]interface{}
		if err := json.Unmarshal(pod.Raw, &podObj); err != nil {
			continue
		}

		// Get node name from spec
		spec, _ := podObj["spec"].(map[string]interface{})
		nodeName, _ := spec["nodeName"].(string)
		if nodeName == "" {
			continue
		}

		agg, ok := nodePods[nodeName]
		if !ok {
			agg = &podAggregation{}
			nodePods[nodeName] = agg
		}

		// Only count running pods
		status, _ := podObj["status"].(map[string]interface{})
		phase, _ := status["phase"].(string)
		if phase != "Running" && phase != "Pending" {
			continue
		}

		agg.podCount++

		// Sum container resources
		if containers, ok := spec["containers"].([]interface{}); ok {
			for _, c := range containers {
				container, _ := c.(map[string]interface{})
				if resources, ok := container["resources"].(map[string]interface{}); ok {
					if requests, ok := resources["requests"].(map[string]interface{}); ok {
						agg.cpuRequests += parseResourceQuantity(requests["cpu"])
						agg.memRequests += parseResourceQuantity(requests["memory"])
					}
					if limits, ok := resources["limits"].(map[string]interface{}); ok {
						agg.cpuLimits += parseResourceQuantity(limits["cpu"])
						agg.memLimits += parseResourceQuantity(limits["memory"])
					}
				}
			}
		}
	}

	// Build result
	var results []NodeMetricsInfo
	for _, node := range nodes.Items {
		agg := nodePods[node.Name]
		alloc := nodeAlloc[node.Name]

		metrics := NodeMetricsInfo{
			NodeName:       node.Name,
			PodCount:       agg.podCount,
			CPURequests:    formatCPU(agg.cpuRequests),
			CPULimits:      formatCPU(agg.cpuLimits),
			MemRequests:    formatMemory(agg.memRequests),
			MemLimits:      formatMemory(agg.memLimits),
			CPUAllocatable: formatCPU(alloc.cpuAllocatable),
			MemAllocatable: formatMemory(alloc.memAllocatable),
		}

		// Calculate percentages
		if alloc.cpuAllocatable > 0 {
			metrics.CPURequestPct = int(agg.cpuRequests * 100 / alloc.cpuAllocatable)
			metrics.CPULimitPct = int(agg.cpuLimits * 100 / alloc.cpuAllocatable)
		}
		if alloc.memAllocatable > 0 {
			metrics.MemRequestPct = int(agg.memRequests * 100 / alloc.memAllocatable)
			metrics.MemLimitPct = int(agg.memLimits * 100 / alloc.memAllocatable)
		}

		results = append(results, metrics)
	}

	return results, nil
}
