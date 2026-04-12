package main

import (
	"github.com/thepixelabs/kubecat/internal/network"
)

// AnalyzeNetworkPolicies returns the connectivity graph for a namespace.
func (a *App) AnalyzeNetworkPolicies(clusterContext, namespace string) (*network.NetworkGraph, error) {
	mgr := a.nexus.Clusters.Manager()
	cl, err := mgr.Get(clusterContext)
	if err != nil {
		cl, err = mgr.Active()
		if err != nil {
			return nil, err
		}
	}
	return network.AnalyzeNamespace(a.ctx, cl, namespace)
}

// GetNetworkPolicyYAML returns the raw YAML for a NetworkPolicy resource.
func (a *App) GetNetworkPolicyYAML(clusterContext, namespace, name string) (string, error) {
	mgr := a.nexus.Clusters.Manager()
	cl, err := mgr.Get(clusterContext)
	if err != nil {
		cl, err = mgr.Active()
		if err != nil {
			return "", err
		}
	}
	res, err := cl.Get(a.ctx, "networkpolicies", namespace, name)
	if err != nil {
		return "", err
	}
	return string(res.Raw), nil
}
