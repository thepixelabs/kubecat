// SPDX-License-Identifier: Apache-2.0

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/thepixelabs/kubecat/internal/storage"
)

// AnalysisContext contains all the gathered data for analysis.
type AnalysisContext struct {
	ResourceRequest AnalysisRequest
	ResourceYAML    string
	Events          []EventContext
	Logs            string
}

// AnalysisRequest defines the target for analysis.
type AnalysisRequest struct {
	Kind      string
	Namespace string
	Name      string
}

// AnalyzeResource gathers context and performs the analysis.
func (b *ContextBuilder) AnalyzeResource(ctx context.Context, provider Provider, req AnalysisRequest) (string, error) {
	// 1. Gather Context
	actx, err := b.GatherContext(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to gather context: %w", err)
	}

	// 2. Build Prompt
	prompt := b.buildAnalysisPrompt(actx)

	// 3. Sanitize before sending to any provider.
	//    SanitizeForCloud is always applied — it is a no-op for non-sensitive
	//    content and a critical safety net for cloud providers.
	if isCloudProvider(provider.Name()) {
		prompt = SanitizeForCloud(prompt)
	}

	// 4. Query Provider
	return provider.Query(ctx, prompt)
}

// GatherContext gathers resource context (YAML, events, logs) for analysis.
// This is exported so it can be used by other parts of the application.
func (b *ContextBuilder) GatherContext(ctx context.Context, req AnalysisRequest) (*AnalysisContext, error) {
	actx := &AnalysisContext{
		ResourceRequest: req,
	}

	if b.manager == nil {
		return nil, fmt.Errorf("client manager not initialized")
	}

	clusterClient, err := b.manager.Active()
	if err != nil {
		return nil, fmt.Errorf("no active cluster: %w", err)
	}

	// 1. Get Resource YAML
	resource, err := clusterClient.Get(ctx, req.Kind, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}

	// Convert to YAML
	var obj map[string]interface{}
	if err := json.Unmarshal(resource.Raw, &obj); err != nil {
		// Fallback to simpler representation if unmarshal fails
		actx.ResourceYAML = fmt.Sprintf("Error parsing resource: %v", err)
	} else {
		// Remove managedFields to save tokens.
		if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
			delete(metadata, "managedFields")
		}

		// Strip sensitive values from the object before marshaling.
		// This removes Secret .data/.stringData values and sensitive env vars
		// at the structured level — before any text encoding occurs.
		SanitizeResourceObject(obj)

		yamlBytes, err := yaml.Marshal(obj)
		if err != nil {
			actx.ResourceYAML = fmt.Sprintf("Error marshaling to YAML: %v", err)
		} else {
			actx.ResourceYAML = string(yamlBytes)
		}
	}

	// 2. Get Events
	// We try to filter by the specific resource
	if b.events != nil {
		filter := storage.EventFilter{
			Namespace: req.Namespace,
			Name:      req.Name,                        // Filter by name directly in DB
			Since:     time.Now().Add(-24 * time.Hour), // Last 24h
			Limit:     100,
		}
		events, err := b.events.List(ctx, filter)
		if err == nil {
			for _, e := range events {
				// Use flexible Kind matching (case-insensitive)
				if strings.EqualFold(e.Kind, req.Kind) {
					actx.Events = append(actx.Events, EventContext{
						Kind:    e.Kind,
						Name:    e.Name,
						Type:    e.Type,
						Reason:  e.Reason,
						Message: e.Message,
						Time:    e.LastSeen,
					})
				}
			}
		}
	}

	// 3. Get Logs (if it's a Pod)
	if strings.EqualFold(req.Kind, "pod") || strings.EqualFold(req.Kind, "pods") {
		// If it's a pod, try to find failing containers first
		// We need to parse the status from the resource object we just got
		// Simple approach: Get logs from the first container or all if it's crashing
		// For now, let's just get the logs of the first container or "main"
		// We need to know container names. We can inspect the obj map.

		containers := b.extractContainerNames(obj)
		if len(containers) > 0 {
			// Just grab the first one for now, or maybe the one with restart count > 0?
			// Let's grab the first one to start simple.
			containerName := containers[0]

			logsChan, err := clusterClient.Logs(ctx, req.Namespace, req.Name, containerName, false, 50)
			if err == nil {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("--- Logs for container '%s' ---\n", containerName))

				// Read from channel with timeout
				timeout := time.After(5 * time.Second)
			loop:
				for {
					select {
					case line, ok := <-logsChan:
						if !ok {
							break loop
						}
						sb.WriteString(line)
						sb.WriteString("\n")
					case <-timeout:
						sb.WriteString("... (timeout fetching logs)\n")
						break loop
					}
				}
				actx.Logs = sb.String()
			}
		}
	}

	return actx, nil
}

func (b *ContextBuilder) extractContainerNames(obj map[string]interface{}) []string {
	var names []string
	// Traverse spec.containers
	if spec, ok := obj["spec"].(map[string]interface{}); ok {
		if containers, ok := spec["containers"].([]interface{}); ok {
			for _, c := range containers {
				if container, ok := c.(map[string]interface{}); ok {
					if name, ok := container["name"].(string); ok {
						names = append(names, name)
					}
				}
			}
		}
	}
	return names
}

func (b *ContextBuilder) buildAnalysisPrompt(actx *AnalysisContext) string {
	var sb strings.Builder

	sb.WriteString("You are a Senior DevOps Architect with immense experience running inside 'Kubecat', a powerful desktop dashboard. Analyze this Kubernetes resource telemetry.\n")
	sb.WriteString("IMPORTANT: The user can Run any code block you provide with one click. NEVER say 'I cannot execute commands'. Instead, provide the exact `kubectl` or shell commands to solve the highlighted issues in ```bash``` blocks.\n")
	sb.WriteString("Provide a **concise, high-impact diagnostic** report. Avoid fluff, unnecessary explanations, and generic advice. Be extremely direct.\n")
	sb.WriteString("Your output must be in HTML format wrapped in a <div class=\"ai-summary\"> block. Do not use Markdown backticks for the outer block.\n")
	sb.WriteString("Use the following structure:\n")
	sb.WriteString(`<div class="ai-summary">
  <div class="summary-header">
    <h3>Diagnostic Summary</h3>
    <p>One clear sentence on current state (e.g. <strong>HEALTHY</strong> or <strong>CRITICAL FAILURE</strong>).</p>
  </div>
  <div class="key-findings">
    <h3>Key Findings</h3>
    <ul class="findings-list">
      <li class="finding-item">Specific error or risk found.</li>
    </ul>
  </div>
  <div class="recommendations">
    <h3>Recommendations</h3>
    <div class="recommendation-item">
      Concrete actionable step (e.g. <code>kubectl delete pod ...</code>).
    </div>
  </div>
</div>`)
	sb.WriteString("\n\nUse <code> tags for inline names. **Crucial:** For actionable commands that the user should run, use standard Markdown code blocks (```bash ... ```) so they can be executed. Do NOT put actionable commands inside HTML tags, keep them outside or let them be extracted.\n")

	sb.WriteString("--- Resource YAML ---\n")
	sb.WriteString("```yaml\n")
	sb.WriteString(actx.ResourceYAML)
	sb.WriteString("\n```\n\n")

	if len(actx.Events) > 0 {
		sb.WriteString("--- Recent Events ---\n")
		for _, e := range actx.Events {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", e.Type, e.Reason, e.Message))
		}
		sb.WriteString("\n")
	}

	if actx.Logs != "" {
		sb.WriteString("--- Recent Logs ---\n")
		sb.WriteString("```\n")
		sb.WriteString(actx.Logs)
		sb.WriteString("\n```\n\n")
	}

	return sb.String()
}
