// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/thepixelabs/kubecat/internal/client"
)

// WorkloadLogLine represents a log line with pod metadata.
type WorkloadLogLine struct {
	Pod       string
	Container string
	Line      string
	ColorIdx  int
}

// LogService provides pod log operations.
type LogService struct {
	clusterService *ClusterService

	// State for single pod logs
	mu       sync.RWMutex
	cancel   context.CancelFunc
	lines    []string
	maxLines int

	// State for workload (multi-pod) logs
	workloadLines []WorkloadLogLine
	podColorMap   map[string]int
	nextColorIdx  int
}

// NewLogService creates a new log service.
func NewLogService(cs *ClusterService) *LogService {
	return &LogService{
		clusterService: cs,
		maxLines:       10000,
	}
}

// LogOptions configures log streaming.
type LogOptions struct {
	Namespace string
	Pod       string
	Container string
	Follow    bool
	TailLines int64
}

// StreamLogs starts streaming logs from a pod.
func (s *LogService) StreamLogs(_ context.Context, opts LogOptions) (<-chan string, error) {
	c, err := s.clusterService.Manager().Active()
	if err != nil {
		return nil, err
	}

	// Cancel any existing stream
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	// Use Background() to avoid race conditions where the caller's context
	// gets canceled (e.g., React component unmount) before the API call completes
	streamCtx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.lines = nil
	s.workloadLines = nil // Clear workload logs too
	s.mu.Unlock()

	logChan, err := c.Logs(streamCtx, opts.Namespace, opts.Pod, opts.Container, opts.Follow, opts.TailLines)
	if err != nil {
		return nil, err
	}

	// Wrap the channel to buffer lines
	bufferedChan := make(chan string, 100)
	go func() {
		defer close(bufferedChan)
		for line := range logChan {
			s.mu.Lock()
			s.lines = append(s.lines, line)
			if len(s.lines) > s.maxLines {
				s.lines = s.lines[len(s.lines)-s.maxLines:]
			}
			s.mu.Unlock()

			select {
			case bufferedChan <- line:
			case <-streamCtx.Done():
				return
			}
		}
	}()

	return bufferedChan, nil
}

// StopStreaming stops the current log stream.
func (s *LogService) StopStreaming() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// GetBufferedLines returns all buffered log lines.
func (s *LogService) GetBufferedLines() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, len(s.lines))
	copy(result, s.lines)
	return result
}

// SearchLogs searches buffered logs for a pattern.
func (s *LogService) SearchLogs(pattern string, useRegex bool) []LogMatch {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matches []LogMatch
	var re *regexp.Regexp
	if useRegex {
		var err error
		re, err = regexp.Compile(pattern)
		if err != nil {
			// Fall back to literal search
			useRegex = false
		}
	}

	pattern = strings.ToLower(pattern)
	for i, line := range s.lines {
		var matched bool
		if useRegex && re != nil {
			matched = re.MatchString(line)
		} else {
			matched = strings.Contains(strings.ToLower(line), pattern)
		}
		if matched {
			matches = append(matches, LogMatch{
				LineNumber: i,
				Line:       line,
			})
		}
	}

	return matches
}

// LogMatch represents a search match in logs.
type LogMatch struct {
	LineNumber int
	Line       string
}

// ClearBuffer clears the log buffer.
func (s *LogService) ClearBuffer() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines = nil
}

// StreamWorkloadLogs starts streaming logs from all pods in a deployment/statefulset.
func (s *LogService) StreamWorkloadLogs(_ context.Context, kind, namespace, name string, tailLines int64) error {
	c, err := s.clusterService.Manager().Active()
	if err != nil {
		return err
	}

	// Cancel any existing stream
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	streamCtx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.lines = nil
	s.workloadLines = nil
	s.podColorMap = make(map[string]int)
	s.nextColorIdx = 0
	s.mu.Unlock()

	// Use background context for API calls to avoid cancellation from caller
	apiCtx := context.Background()

	// Get the workload to find its selector
	resource, err := c.Get(apiCtx, kind, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to get %s: %w", kind, err)
	}

	// Parse the selector from the workload
	selector, err := s.getWorkloadSelector(resource)
	if err != nil {
		return fmt.Errorf("failed to get selector: %w", err)
	}

	// List pods matching the selector
	pods, err := c.List(apiCtx, "pods", client.ListOptions{
		Namespace:     namespace,
		LabelSelector: selector,
		Limit:         100,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for %s/%s", kind, name)
	}

	// Sort pods by name for consistent coloring
	sort.Slice(pods.Items, func(i, j int) bool {
		return pods.Items[i].Name < pods.Items[j].Name
	})

	// Assign colors to pods
	s.mu.Lock()
	for i, pod := range pods.Items {
		s.podColorMap[pod.Name] = i
	}
	s.nextColorIdx = len(pods.Items)
	s.mu.Unlock()

	// Start streaming logs from each pod
	for _, pod := range pods.Items {
		podName := pod.Name
		colorIdx := s.podColorMap[podName]

		go func(pName string, cIdx int) {
			logChan, err := c.Logs(streamCtx, namespace, pName, "", true, tailLines)
			if err != nil {
				return
			}

			for line := range logChan {
				s.mu.Lock()
				logLine := WorkloadLogLine{
					Pod:      pName,
					Line:     line,
					ColorIdx: cIdx,
				}
				s.workloadLines = append(s.workloadLines, logLine)
				if len(s.workloadLines) > s.maxLines {
					s.workloadLines = s.workloadLines[len(s.workloadLines)-s.maxLines:]
				}
				s.mu.Unlock()
			}
		}(podName, colorIdx)
	}

	return nil
}

// getWorkloadSelector extracts the pod selector from a workload resource.
func (s *LogService) getWorkloadSelector(resource *client.Resource) (string, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(resource.Raw, &obj); err != nil {
		return "", err
	}

	// Navigate to spec.selector.matchLabels
	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no spec found")
	}

	selector, ok := spec["selector"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no selector found")
	}

	matchLabels, ok := selector["matchLabels"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no matchLabels found")
	}

	// Convert to label selector string
	var parts []string
	for k, v := range matchLabels {
		if vs, ok := v.(string); ok {
			parts = append(parts, fmt.Sprintf("%s=%s", k, vs))
		}
	}

	return strings.Join(parts, ","), nil
}

// GetBufferedWorkloadLines returns all buffered workload log lines.
func (s *LogService) GetBufferedWorkloadLines() []WorkloadLogLine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]WorkloadLogLine, len(s.workloadLines))
	copy(result, s.workloadLines)
	return result
}
