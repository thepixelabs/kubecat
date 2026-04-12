// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/thepixelabs/kubecat/internal/audit"

// StartTerminal starts a new terminal session.
func (a *App) StartTerminal(id string, command string, args []string) error {
	if err := a.checkReadOnly(); err != nil {
		return err
	}
	audit.LogTerminalSession(id, "start")
	return a.terminalManager.Start(id, command, args...)
}

// ResizeTerminal resizes a terminal session.
func (a *App) ResizeTerminal(id string, rows, cols int) error {
	return a.terminalManager.Resize(id, rows, cols)
}

// WriteTerminal writes data to a terminal session.
func (a *App) WriteTerminal(id string, data string) error {
	return a.terminalManager.Write(id, data)
}

// CloseTerminal closes a terminal session.
func (a *App) CloseTerminal(id string) error {
	audit.LogTerminalSession(id, "stop")
	return a.terminalManager.Close(id)
}
