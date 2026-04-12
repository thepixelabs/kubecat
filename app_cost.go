// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/thepixelabs/kubecat/internal/config"
	"github.com/thepixelabs/kubecat/internal/cost"
)

// GetWorkloadCost returns a cost estimate for a named workload.
func (a *App) GetWorkloadCost(clusterContext, namespace, workload string) (*cost.CostEstimate, error) {
	cl, err := a.nexus.Clusters.Manager().Get(clusterContext)
	if err != nil {
		cl, err = a.nexus.Clusters.Manager().Active()
		if err != nil {
			return nil, err
		}
	}
	cfg, _ := config.Load()
	est := cost.New(cl, cfg.Kubecat.Cost.CPUCostPerCoreHour, cfg.Kubecat.Cost.MemCostPerGBHour, cfg.Kubecat.Cost.Currency)
	return est.GetWorkloadCost(a.ctx, namespace, workload)
}

// GetNamespaceCostSummary returns aggregated costs for all workloads in a namespace.
func (a *App) GetNamespaceCostSummary(clusterContext, namespace string) (*cost.NamespaceCostSummary, error) {
	cl, err := a.nexus.Clusters.Manager().Get(clusterContext)
	if err != nil {
		cl, err = a.nexus.Clusters.Manager().Active()
		if err != nil {
			return nil, err
		}
	}
	cfg, _ := config.Load()
	est := cost.New(cl, cfg.Kubecat.Cost.CPUCostPerCoreHour, cfg.Kubecat.Cost.MemCostPerGBHour, cfg.Kubecat.Cost.Currency)
	return est.GetNamespaceCost(a.ctx, namespace)
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
