// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"

	"github.com/thepixelabs/kubecat/internal/config"
	"github.com/thepixelabs/kubecat/internal/cost"
)

// GetWorkloadCost returns a cost estimate for a named workload. If an
// OpenCost endpoint is configured, it is queried and the matching workload
// entry is returned with source="opencost". Otherwise the heuristic
// (requests × default rates) is used with source="heuristic".
func (a *App) GetWorkloadCost(clusterContext, namespace, workload string) (*cost.CostEstimate, error) {
	cl, err := a.nexus.Clusters.Manager().Get(clusterContext)
	if err != nil {
		cl, err = a.nexus.Clusters.Manager().Active()
		if err != nil {
			return nil, err
		}
	}
	cfg, _ := config.Load()
	costCfg := cfg.Kubecat.Cost

	if ep := strings.TrimSpace(costCfg.OpenCostEndpoint); ep != "" {
		estimates, err := cost.QueryOpenCost(a.ctx, ep, namespace)
		if err == nil {
			// QueryOpenCost aggregates by pod; match on the workload
			// prefix (same inference heuristic used in the fallback
			// path to group pod hashes back to their workload).
			for _, est := range estimates {
				if est.WorkloadName == workload || strings.HasPrefix(est.WorkloadName, workload) {
					e := est
					return &e, nil
				}
			}
			// Endpoint responded but didn't have this workload; fall
			// through to the heuristic rather than return a misleading
			// "not found" — local estimate is better than nothing.
		}
		// On transport/parse error, fall through to heuristic. We could
		// surface the error to the caller, but the frontend currently
		// has no way to distinguish "opencost temporarily unreachable"
		// from "no data", and the heuristic is still a reasonable
		// answer.
	}

	est := cost.New(cl, costCfg.CPUCostPerCoreHour, costCfg.MemCostPerGBHour, costCfg.Currency)
	return est.GetWorkloadCost(a.ctx, namespace, workload)
}

// GetNamespaceCostSummary returns aggregated costs for all workloads in a
// namespace. Uses OpenCost when configured, else falls back to the heuristic.
func (a *App) GetNamespaceCostSummary(clusterContext, namespace string) (*cost.NamespaceCostSummary, error) {
	cl, err := a.nexus.Clusters.Manager().Get(clusterContext)
	if err != nil {
		cl, err = a.nexus.Clusters.Manager().Active()
		if err != nil {
			return nil, err
		}
	}
	cfg, _ := config.Load()
	costCfg := cfg.Kubecat.Cost

	if ep := strings.TrimSpace(costCfg.OpenCostEndpoint); ep != "" {
		estimates, err := cost.QueryOpenCost(a.ctx, ep, namespace)
		if err == nil && len(estimates) > 0 {
			summary := &cost.NamespaceCostSummary{
				Namespace: namespace,
				Currency:  firstCurrency(estimates, costCfg.Currency),
				Source:    cost.SourceOpenCost,
				Workloads: estimates,
			}
			for _, e := range estimates {
				summary.TotalPerHour += e.TotalCost
			}
			summary.TotalPerMonth = summary.TotalPerHour * 730.0
			return summary, nil
		}
		// Fall through on error or empty response.
	}

	est := cost.New(cl, costCfg.CPUCostPerCoreHour, costCfg.MemCostPerGBHour, costCfg.Currency)
	return est.GetNamespaceCost(a.ctx, namespace)
}

// firstCurrency returns the first non-empty currency from OpenCost estimates,
// falling back to the configured default.
func firstCurrency(estimates []cost.CostEstimate, fallback string) string {
	for _, e := range estimates {
		if e.Currency != "" {
			return e.Currency
		}
	}
	if fallback != "" {
		return fallback
	}
	return "USD"
}

// DetectCostBackend probes the active cluster for an installed OpenCost or
// Kubecost service and returns "opencost", "kubecost", or "none". This is a
// hint for the UI: when it returns "opencost" but no endpoint is configured,
// the UI can prompt the user to set OpenCostEndpoint.
func (a *App) DetectCostBackend() (string, error) {
	cl, err := a.nexus.Clusters.Manager().Active()
	if err != nil {
		return string(cost.BackendNone), err
	}
	return string(cost.DetectBackend(a.ctx, cl)), nil
}

// GetCostSettings returns the current cost configuration.
func (a *App) GetCostSettings() (*config.CostConfig, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	c := cfg.Kubecat.Cost
	if c.CPUCostPerCoreHour == 0 {
		c.CPUCostPerCoreHour = 0.048
	}
	if c.MemCostPerGBHour == 0 {
		c.MemCostPerGBHour = 0.006
	}
	if c.Currency == "" {
		c.Currency = "USD"
	}
	return &c, nil
}

// SaveCostSettings persists cost configuration.
func (a *App) SaveCostSettings(settings config.CostConfig) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Kubecat.Cost = settings
	return cfg.Save()
}
