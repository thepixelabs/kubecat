package main

import (
	"sync"

	"github.com/thepixelabs/kubecat/internal/mcp"
)

var (
	mcpMu     sync.Mutex
	mcpServer *mcp.Server
	mcpCancel func()
)

// StartMCPServer starts the MCP stdio server if not already running.
func (a *App) StartMCPServer() error {
	mcpMu.Lock()
	defer mcpMu.Unlock()

	if mcpServer != nil {
		return nil // already running
	}

	handler := mcp.NewHandler(a.nexus.Clusters.Manager())
	mcpServer = mcp.NewServer(handler, nil, nil)

	ctx, cancel := a.ctx, func() {}
	mcpCancel = cancel
	go mcpServer.Start(ctx)
	return nil
}

// StopMCPServer stops the running MCP server.
func (a *App) StopMCPServer() {
	mcpMu.Lock()
	defer mcpMu.Unlock()

	if mcpCancel != nil {
		mcpCancel()
		mcpCancel = nil
	}
	mcpServer = nil
}

// GetMCPStatus returns whether the MCP server is running.
func (a *App) GetMCPStatus() map[string]interface{} {
	mcpMu.Lock()
	defer mcpMu.Unlock()
	return map[string]interface{}{
		"running":   mcpServer != nil,
		"transport": "stdio",
	}
}
