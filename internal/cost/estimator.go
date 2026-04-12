// Package cost provides resource cost estimation for Kubernetes workloads.
// It integrates with OpenCost/Kubecost when available and falls back to
// resource-request × node-pricing heuristics.
package cost

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
)

const (
	defaultCPUCostPerCoreHour = 0.048 // AWS on-demand baseline
	defaultMemCostPerGBHour   = 0.006
	hoursPerMonth             = 730.0
)

// Source indicates how the cost was calculated.
type Source string

const (
	SourceOpenCost  Source = "opencost"
	SourceKubecost  Source = "kubecost"
	SourceHeuristic Source = "heuristic"
)

// CostEstimate is the cost breakdown for a single workload.
type CostEstimate struct {
	WorkloadName string  `json:"workloadName"`
	Namespace    string  `json:"namespace"`
	CPUCost      float64 `json:"cpuCost"`      // $/hour
	MemoryCost   float64 `json:"memoryCost"`   // $/hour
	TotalCost    float64 `json:"totalCost"`    // $/hour
	MonthlyTotal float64 `json:"monthlyTotal"` // $/month (730h)
	Currency     string  `json:"currency"`
	Period       string  `json:"period"` // "hour"
	Source       Source  `json:"source"`
}

// NamespaceCostSummary aggregates costs for a namespace.
type NamespaceCostSummary struct {
	Namespace     string         `json:"namespace"`
	TotalPerHour  float64        `json:"totalPerHour"`
	TotalPerMonth float64        `json:"totalPerMonth"`
	Currency      string         `json:"currency"`
	Source        Source         `json:"source"`
	Workloads     []CostEstimate `json:"workloads"`
}

// Backend is the detected cost backend type.
type Backend string

const (
	BackendNone     Backend = "none"
	BackendOpenCost Backend = "opencost"
	BackendKubecost Backend = "kubecost"
)

// Estimator computes resource costs.
type Estimator struct {
	cl             client.ClusterClient
	cpuCostPerCore float64
	memCostPerGB   float64
	currency       string
}

// New creates an Estimator. Pass zero values to use defaults.
func New(cl client.ClusterClient, cpuCost, memCost float64, currency string) *Estimator {
	if cpuCost == 0 {
		cpuCost = defaultCPUCostPerCoreHour
	}
	if memCost == 0 {
		memCost = defaultMemCostPerGBHour
	}
	if currency == "" {
		currency = "USD"
	}
	return &Estimator{cl: cl, cpuCostPerCore: cpuCost, memCostPerGB: memCost, currency: currency}
}

// DetectBackend checks whether OpenCost or Kubecost is installed in the cluster.
func DetectBackend(ctx context.Context, cl client.ClusterClient) Backend {
	// Try OpenCost (usually in opencost or monitoring namespace).
	for _, ns := range []string{"opencost", "monitoring", "kube-system"} {
		list, err := cl.List(ctx, "services", client.ListOptions{Namespace: ns, Limit: 50})
		if err != nil {
			continue
		}
		for _, svc := range list.Items {
			lower := strings.ToLower(svc.Name)
			if strings.Contains(lower, "opencost") {
				return BackendOpenCost
			}
			if strings.Contains(lower, "kubecost") {
				return BackendKubecost
			}
		}
	}
	return BackendNone
}

// GetWorkloadCost returns a cost estimate for a named workload (Deployment/StatefulSet).
func (e *Estimator) GetWorkloadCost(ctx context.Context, namespace, workload string) (*CostEstimate, error) {
	// Try to find pods owned by this workload.
	pods, err := e.cl.List(ctx, "pods", client.ListOptions{Namespace: namespace, Limit: 200})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	var totalCPU, totalMem float64
	count := 0
	for _, pod := range pods.Items {
		// Match pods whose name starts with the workload name.
		if !strings.HasPrefix(pod.Name, workload) {
			continue
		}
		cpu, mem := extractRequests(pod.Object)
		totalCPU += cpu
		totalMem += mem
		count++
	}

	if count == 0 {
		return nil, fmt.Errorf("no pods found for workload %q in namespace %q", workload, namespace)
	}

	return e.estimate(workload, namespace, totalCPU, totalMem), nil
}

// GetNamespaceCost returns aggregated costs for all workloads in a namespace.
func (e *Estimator) GetNamespaceCost(ctx context.Context, namespace string) (*NamespaceCostSummary, error) {
	pods, err := e.cl.List(ctx, "pods", client.ListOptions{Namespace: namespace, Limit: 500})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	// Group by workload prefix (strip pod hash suffix).
	workloadCPU := map[string]float64{}
	workloadMem := map[string]float64{}
	for _, pod := range pods.Items {
		wl := inferWorkload(pod.Name)
		cpu, mem := extractRequests(pod.Object)
		workloadCPU[wl] += cpu
		workloadMem[wl] += mem
	}

	summary := &NamespaceCostSummary{
		Namespace: namespace,
		Currency:  e.currency,
		Source:    SourceHeuristic,
	}
	for wl, cpu := range workloadCPU {
		est := e.estimate(wl, namespace, cpu, workloadMem[wl])
		summary.Workloads = append(summary.Workloads, *est)
		summary.TotalPerHour += est.TotalCost
	}
	summary.TotalPerMonth = summary.TotalPerHour * hoursPerMonth
	return summary, nil
}

func (e *Estimator) estimate(name, namespace string, cpuCores, memGB float64) *CostEstimate {
	cpuCost := cpuCores * e.cpuCostPerCore
	memCost := memGB * e.memCostPerGB
	total := cpuCost + memCost
	return &CostEstimate{
		WorkloadName: name,
		Namespace:    namespace,
		CPUCost:      round2(cpuCost),
		MemoryCost:   round2(memCost),
		TotalCost:    round2(total),
		MonthlyTotal: round2(total * hoursPerMonth),
		Currency:     e.currency,
		Period:       "hour",
		Source:       SourceHeuristic,
	}
}

// extractRequests parses CPU (cores) and memory (GB) requests from a pod object.
func extractRequests(obj map[string]interface{}) (cpuCores, memGB float64) {
	if obj == nil {
		return
	}
	spec, _ := obj["spec"].(map[string]interface{})
	if spec == nil {
		return
	}
	containers, _ := spec["containers"].([]interface{})
	for _, c := range containers {
		container, _ := c.(map[string]interface{})
		if container == nil {
			continue
		}
		resources, _ := container["resources"].(map[string]interface{})
		if resources == nil {
			continue
		}
		reqs, _ := resources["requests"].(map[string]interface{})
		if reqs == nil {
			continue
		}
		if cpuStr, ok := reqs["cpu"].(string); ok {
			cpuCores += parseCPU(cpuStr)
		}
		if memStr, ok := reqs["memory"].(string); ok {
			memGB += parseMemoryGB(memStr)
		}
	}
	return
}

func parseCPU(s string) float64 {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "m") {
		var v float64
		_, _ = fmt.Sscanf(s[:len(s)-1], "%f", &v)
		return v / 1000.0
	}
	var v float64
	_, _ = fmt.Sscanf(s, "%f", &v)
	return v
}

func parseMemoryGB(s string) float64 {
	s = strings.TrimSpace(s)
	var v float64
	switch {
	case strings.HasSuffix(s, "Ki"):
		_, _ = fmt.Sscanf(s[:len(s)-2], "%f", &v)
		return v / (1024 * 1024)
	case strings.HasSuffix(s, "Mi"):
		_, _ = fmt.Sscanf(s[:len(s)-2], "%f", &v)
		return v / 1024
	case strings.HasSuffix(s, "Gi"):
		_, _ = fmt.Sscanf(s[:len(s)-2], "%f", &v)
		return v
	case strings.HasSuffix(s, "M"):
		_, _ = fmt.Sscanf(s[:len(s)-1], "%f", &v)
		return v / 1000
	case strings.HasSuffix(s, "G"):
		_, _ = fmt.Sscanf(s[:len(s)-1], "%f", &v)
		return v
	default:
		_, _ = fmt.Sscanf(s, "%f", &v)
		return v / (1024 * 1024 * 1024) // bytes
	}
}

// inferWorkload strips the pod hash suffixes to get a workload name.
func inferWorkload(podName string) string {
	parts := strings.Split(podName, "-")
	if len(parts) <= 2 {
		return podName
	}
	// Last two parts are typically the replicaset hash + pod hash.
	return strings.Join(parts[:len(parts)-2], "-")
}

func round2(v float64) float64 {
	return float64(int(v*100)) / 100
}

// ── OpenCost integration ──────────────────────────────────────────────────────

// QueryOpenCost fetches allocation data from an OpenCost API endpoint.
func QueryOpenCost(ctx context.Context, endpoint, namespace string) ([]CostEstimate, error) {
	url := fmt.Sprintf("%s/model/allocation?window=1h&namespace=%s&aggregate=pod", endpoint, namespace)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var estimates []CostEstimate
	for _, window := range result.Data {
		for name, raw := range window {
			alloc, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			totalCost, _ := alloc["totalCost"].(float64)
			cpuCost, _ := alloc["cpuCost"].(float64)
			memCost, _ := alloc["ramCost"].(float64)
			estimates = append(estimates, CostEstimate{
				WorkloadName: name,
				Namespace:    namespace,
				CPUCost:      round2(cpuCost),
				MemoryCost:   round2(memCost),
				TotalCost:    round2(totalCost),
				MonthlyTotal: round2(totalCost * hoursPerMonth),
				Currency:     "USD",
				Period:       "hour",
				Source:       SourceOpenCost,
			})
		}
	}
	return estimates, nil
}
