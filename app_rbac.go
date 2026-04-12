package main

import (
	"github.com/thepixelabs/kubecat/internal/rbac"
)

// GetNamespaceRBAC returns the RBAC permission matrix for a namespace.
func (a *App) GetNamespaceRBAC(clusterContext, namespace string) (*rbac.RBACMatrix, error) {
	mgr := a.nexus.Clusters.Manager()
	cl, err := mgr.Get(clusterContext)
	if err != nil {
		cl, err = mgr.Active()
		if err != nil {
			return nil, err
		}
	}
	return rbac.ListNamespaceRBAC(a.ctx, cl, namespace)
}
