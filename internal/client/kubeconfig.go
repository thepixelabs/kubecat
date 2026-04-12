// SPDX-License-Identifier: Apache-2.0

package client

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// KubeConfigLoader handles loading and parsing kubeconfig files.
type KubeConfigLoader struct {
	// path is the path to the kubeconfig file.
	path string
	// config is the loaded kubeconfig.
	config *api.Config
}

// NewKubeConfigLoader creates a new kubeconfig loader.
func NewKubeConfigLoader() (*KubeConfigLoader, error) {
	path := kubeConfigPath()

	config, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return nil, err
	}

	return &KubeConfigLoader{
		path:   path,
		config: config,
	}, nil
}

// kubeConfigPath returns the path to the kubeconfig file.
func kubeConfigPath() string {
	// Check KUBECONFIG env var first
	if path := os.Getenv("KUBECONFIG"); path != "" {
		// Handle multiple paths - use the first one
		paths := filepath.SplitList(path)
		if len(paths) > 0 {
			return paths[0]
		}
	}

	// Default to ~/.kube/config
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}

// Contexts returns all available context names.
func (l *KubeConfigLoader) Contexts() []string {
	contexts := make([]string, 0, len(l.config.Contexts))
	for name := range l.config.Contexts {
		contexts = append(contexts, name)
	}
	return contexts
}

// CurrentContext returns the current context name.
func (l *KubeConfigLoader) CurrentContext() string {
	return l.config.CurrentContext
}

// ContextInfo returns information about a specific context.
func (l *KubeConfigLoader) ContextInfo(name string) (*ContextInfo, error) {
	ctx, exists := l.config.Contexts[name]
	if !exists {
		return nil, ErrContextNotFound
	}

	cluster, exists := l.config.Clusters[ctx.Cluster]
	if !exists {
		return nil, ErrClusterNotFound
	}

	return &ContextInfo{
		Name:      name,
		Cluster:   ctx.Cluster,
		Server:    cluster.Server,
		Namespace: ctx.Namespace,
		User:      ctx.AuthInfo,
	}, nil
}

// ClientConfig returns a REST config for a specific context.
func (l *KubeConfigLoader) ClientConfig(contextName string) (*rest.Config, error) {
	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: contextName,
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: l.path},
		configOverrides,
	)

	return clientConfig.ClientConfig()
}

// ContextInfo contains information about a kubeconfig context.
type ContextInfo struct {
	// Name is the context name.
	Name string
	// Cluster is the cluster name.
	Cluster string
	// Server is the API server URL.
	Server string
	// Namespace is the default namespace.
	Namespace string
	// User is the user/auth info name.
	User string
}

// Reload reloads the kubeconfig from disk.
func (l *KubeConfigLoader) Reload() error {
	config, err := clientcmd.LoadFromFile(l.path)
	if err != nil {
		return err
	}
	l.config = config
	return nil
}
