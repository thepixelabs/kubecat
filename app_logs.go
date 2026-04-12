// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"

	"github.com/thepixelabs/kubecat/internal/core"
)

// LogMatchInfo is a JSON-friendly log match.
type LogMatchInfo struct {
	LineNumber int    `json:"lineNumber"`
	Line       string `json:"line"`
}

// LogLine represents a single log line with metadata for multi-pod logging.
type LogLine struct {
	Pod       string `json:"pod"`
	Container string `json:"container"`
	Line      string `json:"line"`
	ColorIdx  int    `json:"colorIdx"`
}

// StartLogStream starts streaming logs from a pod.
func (a *App) StartLogStream(namespace, pod, container string, tailLines int64) error {
	// Use Background context to avoid Wails context cancellation issues
	_, err := a.nexus.Logs.StreamLogs(context.Background(), core.LogOptions{
		Namespace: namespace,
		Pod:       pod,
		Container: container,
		Follow:    true,
		TailLines: tailLines,
	})
	return err
}

// StopLogStream stops the current log stream.
func (a *App) StopLogStream() {
	a.nexus.Logs.StopStreaming()
}

// GetBufferedLogs returns all buffered log lines.
func (a *App) GetBufferedLogs() []string {
	return a.nexus.Logs.GetBufferedLines()
}

// SearchLogs searches logs for a pattern.
func (a *App) SearchLogs(pattern string) []LogMatchInfo {
	matches := a.nexus.Logs.SearchLogs(pattern, false)
	result := make([]LogMatchInfo, len(matches))
	for i, m := range matches {
		result[i] = LogMatchInfo{
			LineNumber: m.LineNumber,
			Line:       m.Line,
		}
	}
	return result
}

// StartWorkloadLogStream starts streaming logs from all pods in a deployment/statefulset.
func (a *App) StartWorkloadLogStream(kind, namespace, name string, tailLines int64) error {
	// Use Background context to avoid Wails context cancellation issues
	return a.nexus.Logs.StreamWorkloadLogs(context.Background(), kind, namespace, name, tailLines)
}

// GetBufferedWorkloadLogs returns all buffered log lines with pod metadata.
func (a *App) GetBufferedWorkloadLogs() []LogLine {
	lines := a.nexus.Logs.GetBufferedWorkloadLines()
	result := make([]LogLine, len(lines))
	for i, l := range lines {
		result[i] = LogLine{
			Pod:       l.Pod,
			Container: l.Container,
			Line:      l.Line,
			ColorIdx:  l.ColorIdx,
		}
	}
	return result
}
