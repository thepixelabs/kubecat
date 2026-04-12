package main

import (
	"log/slog"

	"github.com/thepixelabs/kubecat/internal/client"
	"github.com/thepixelabs/kubecat/internal/version"
)

// GetContexts returns available kubeconfig contexts.
func (a *App) GetContexts() ([]string, error) {
	return a.nexus.Clusters.GetContexts(a.ctx)
}

// RefreshContexts reloads the kubeconfig and returns available contexts.
func (a *App) RefreshContexts() ([]string, error) {
	return a.nexus.Clusters.RefreshContexts(a.ctx)
}

// Connect connects to a cluster by context name.
func (a *App) Connect(contextName string) error {
	slog.Info("connecting to cluster", slog.String("cluster", contextName))
	err := a.nexus.Clusters.Connect(a.ctx, contextName)
	if err == nil {
		slog.Info("cluster connected", slog.String("cluster", contextName))
		if a.healthMonitor != nil {
			a.healthMonitor.NotifyConnected()
		}
		// Trigger event collection refresh
		if a.eventCollector != nil {
			go a.eventCollector.Refresh()
		}
	} else {
		slog.Error("cluster connection failed", slog.String("cluster", contextName), slog.Any("error", err))
	}
	return err
}

// Disconnect disconnects from a cluster.
func (a *App) Disconnect(contextName string) error {
	slog.Info("disconnecting from cluster", slog.String("cluster", contextName))
	err := a.nexus.Clusters.Disconnect(contextName)
	if err != nil {
		slog.Error("cluster disconnect failed", slog.String("cluster", contextName), slog.Any("error", err))
	} else {
		slog.Info("cluster disconnected", slog.String("cluster", contextName))
		if a.healthMonitor != nil {
			a.healthMonitor.NotifyDisconnected()
		}
	}

	// Trigger event collection refresh
	if a.eventCollector != nil {
		go a.eventCollector.Refresh()
	}

	return err
}

// GetActiveContext returns the currently active context name.
func (a *App) GetActiveContext() string {
	return a.nexus.Clusters.ActiveContext()
}

// IsConnected returns true if connected to any cluster.
func (a *App) IsConnected() bool {
	return a.nexus.Clusters.IsConnected()
}

// GetAppVersion returns the application version.
func (a *App) GetAppVersion() string {
	return version.Version
}

// GetClusterInfo returns info about the active cluster.
func (a *App) GetClusterInfo() (*client.ClusterInfo, error) {
	return a.nexus.Clusters.GetClusterInfo(a.ctx)
}
