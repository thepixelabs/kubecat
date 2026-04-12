// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/thepixelabs/kubecat/internal/ai"
	"github.com/thepixelabs/kubecat/internal/analyzer"
	"github.com/thepixelabs/kubecat/internal/config"
)

// AlertSettings configures the proactive alert monitor.
type AlertSettings struct {
	// Enabled controls whether the alert monitor runs.
	Enabled bool `json:"enabled"`
	// ScanIntervalSeconds is the number of seconds between cluster scans.
	ScanIntervalSeconds int `json:"scanIntervalSeconds"`
	// CooldownMinutes is the minimum minutes between repeated alerts for the
	// same issue.
	CooldownMinutes int `json:"cooldownMinutes"`
	// IgnoredNamespaces lists namespaces that should not trigger alerts.
	IgnoredNamespaces []string `json:"ignoredNamespaces"`
}

// GetAlertSettings returns the current alert monitor configuration.
func (a *App) GetAlertSettings() (*AlertSettings, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	ac := cfg.Kubecat.Alerts
	settings := &AlertSettings{
		Enabled:             ac.Enabled,
		ScanIntervalSeconds: ac.ScanIntervalSeconds,
		CooldownMinutes:     ac.CooldownMinutes,
		IgnoredNamespaces:   ac.IgnoredNamespaces,
	}

	// Apply defaults when fields are zero.
	if settings.ScanIntervalSeconds == 0 {
		settings.ScanIntervalSeconds = 60
	}
	if settings.CooldownMinutes == 0 {
		settings.CooldownMinutes = 30
	}

	return settings, nil
}

// SaveAlertSettings persists updated alert monitor configuration.
func (a *App) SaveAlertSettings(settings AlertSettings) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	cfg.Kubecat.Alerts = config.AlertsConfig{
		Enabled:             settings.Enabled,
		ScanIntervalSeconds: settings.ScanIntervalSeconds,
		CooldownMinutes:     settings.CooldownMinutes,
		IgnoredNamespaces:   settings.IgnoredNamespaces,
	}

	return cfg.Save()
}

// ProactiveAlertEvent is emitted as the payload for "ai:proactive-alert" events.
// It carries enough context for the frontend to surface a one-click investigation
// prompt without the user having to navigate to the affected resource first.
type ProactiveAlertEvent struct {
	// ID is a stable deduplication key built from issue ID + resource identity.
	ID string `json:"id"`
	// Severity is one of "Critical", "Warning", "Info".
	Severity string `json:"severity"`
	// Title is a short human-readable issue title (e.g. "CrashLoopBackOff").
	Title string `json:"title"`
	// Message is the AI-generated diagnosis, empty until AI analysis completes.
	Message string `json:"message"`
	// ResourceKind is the Kubernetes resource kind (e.g. "Pod").
	ResourceKind string `json:"resourceKind"`
	// ResourceName is the resource name.
	ResourceName string `json:"resourceName"`
	// Namespace is the resource namespace.
	Namespace string `json:"namespace"`
	// DetectedAt is the RFC3339 timestamp the issue was detected.
	DetectedAt string `json:"detectedAt"`
}

// proactiveAlertMonitor drives periodic cluster scans and AI-powered alerting.
// It runs as a long-lived background goroutine started from app startup.
type proactiveAlertMonitor struct {
	app *App

	mu       sync.Mutex
	cooldown map[string]time.Time // issueID → last-alerted time
}

// StartProactiveAlertMonitor begins the background scan loop.
// It loads settings from config on every scan so changes take effect without restart.
func (a *App) StartProactiveAlertMonitor(ctx context.Context) {
	m := &proactiveAlertMonitor{
		app:      a,
		cooldown: make(map[string]time.Time),
	}
	go m.loop(ctx)
}

// loop runs the periodic scan at the configured interval.
func (m *proactiveAlertMonitor) loop(ctx context.Context) {
	// Initial short delay to let the cluster connection stabilize.
	select {
	case <-ctx.Done():
		return
	case <-time.After(15 * time.Second):
	}

	// Use a ticker driven by settings; reload settings each iteration so the
	// user can change the interval without restarting.
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		m.scan(ctx)

		// Wait for the next tick, updating the interval from config.
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Reload interval from config.
			if settings, err := m.app.GetAlertSettings(); err == nil && settings.ScanIntervalSeconds > 0 {
				ticker.Reset(time.Duration(settings.ScanIntervalSeconds) * time.Second)
			}
		}
	}
}

// scan runs one full issue-scan pass and emits proactive alerts for any
// critical/warning issues that are not on cooldown and not in an ignored
// namespace.
func (m *proactiveAlertMonitor) scan(ctx context.Context) {
	settings, err := m.app.GetAlertSettings()
	if err != nil || !settings.Enabled {
		return
	}

	cl, err := m.app.activeCluster()
	if err != nil {
		// No active cluster — skip silently.
		return
	}

	scanCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	summary, err := analyzer.DefaultRegistry.Scan(scanCtx, cl, "" /* all namespaces */)
	if err != nil {
		slog.Debug("proactive alert monitor: scan failed", slog.Any("error", err))
		return
	}
	if summary == nil {
		return
	}

	ignored := make(map[string]bool, len(settings.IgnoredNamespaces))
	for _, ns := range settings.IgnoredNamespaces {
		ignored[strings.ToLower(ns)] = true
	}
	cooldownDur := time.Duration(settings.CooldownMinutes) * time.Minute

	// Flatten the per-category issue map. Order is non-deterministic but the
	// downstream cooldown map handles dedupe, and severity filtering happens
	// inside the loop.
	var issues []analyzer.Issue
	for _, catIssues := range summary.IssuesByCategory {
		issues = append(issues, catIssues...)
	}

	for _, issue := range issues {
		// Only surface Warning and Critical.
		if issue.Severity == analyzer.SeverityInfo {
			continue
		}
		if ignored[strings.ToLower(issue.Resource.Namespace)] {
			continue
		}

		alertID := fmt.Sprintf("%s/%s/%s/%s", issue.ID, issue.Resource.Kind, issue.Resource.Namespace, issue.Resource.Name)

		m.mu.Lock()
		lastAlert, seen := m.cooldown[alertID]
		if seen && time.Since(lastAlert) < cooldownDur {
			m.mu.Unlock()
			continue
		}
		m.cooldown[alertID] = time.Now()
		m.mu.Unlock()

		// Emit the alert immediately so the UI surfaces it — the AI diagnosis
		// follows asynchronously to keep latency low.
		evt := ProactiveAlertEvent{
			ID:           alertID,
			Severity:     issue.Severity.String(),
			Title:        issue.Title,
			ResourceKind: issue.Resource.Kind,
			ResourceName: issue.Resource.Name,
			Namespace:    issue.Resource.Namespace,
			DetectedAt:   issue.DetectedAt.Format(time.RFC3339),
		}
		m.app.emitter.Emit("ai:proactive-alert", evt)

		slog.Info("proactive alert: issue detected",
			slog.String("id", alertID),
			slog.String("severity", issue.Severity.String()),
			slog.String("title", issue.Title),
		)

		// Kick off AI diagnosis in the background so we don't block the scan loop.
		go m.diagnose(ctx, evt, issue, settings)
	}
}

// diagnose generates an AI-powered diagnosis for an issue and emits an updated
// alert event with the Message field populated.
func (m *proactiveAlertMonitor) diagnose(ctx context.Context, evt ProactiveAlertEvent, issue analyzer.Issue, settings *AlertSettings) {
	cfg, err := config.Load()
	if err != nil || !cfg.Kubecat.AI.Enabled {
		return
	}

	selectedProvider := cfg.Kubecat.AI.SelectedProvider
	if selectedProvider == "" {
		return
	}

	providerConfig, ok := cfg.Kubecat.AI.Providers[selectedProvider]
	if !ok || !providerConfig.Enabled {
		return
	}

	providerCfg := ai.ProviderConfig{
		Model:    cfg.Kubecat.AI.SelectedModel,
		APIKey:   providerConfig.APIKey,
		Endpoint: providerConfig.Endpoint,
		Timeout:  30 * time.Second,
	}

	var provider ai.Provider
	switch selectedProvider {
	case "openai", "litellm":
		provider = ai.NewOpenAIProvider(providerCfg)
	case "anthropic":
		provider = ai.NewAnthropicProvider(providerCfg)
	case "google":
		provider = ai.NewGoogleProvider(providerCfg)
	case "ollama":
		provider = ai.NewOllamaProvider(providerCfg)
	default:
		return
	}
	defer func() { _ = provider.Close() }()

	diagCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	if !provider.Available(diagCtx) {
		return
	}

	// Build a focused diagnostic prompt using only the issue metadata.
	// We deliberately keep this short to minimize token cost for background analysis.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("You are a Kubernetes expert. A %s issue has been automatically detected.\n\n", issue.Severity.String()))
	sb.WriteString(fmt.Sprintf("Resource: %s %s/%s\n", issue.Resource.Kind, issue.Resource.Namespace, issue.Resource.Name))
	sb.WriteString(fmt.Sprintf("Issue: %s\n", issue.Title))
	sb.WriteString(fmt.Sprintf("Details: %s\n\n", issue.Message))
	if len(issue.Fixes) > 0 {
		sb.WriteString("Suggested fixes from static analysis:\n")
		for _, fix := range issue.Fixes {
			sb.WriteString(fmt.Sprintf("- %s\n", fix.Description))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("In 2-3 sentences, explain the likely root cause and the single most important action to take. Be concise — this is a notification, not a full report.")

	prompt := sb.String()
	if ai.IsCloudProvider(selectedProvider) {
		prompt = ai.SanitizeForCloud(prompt)
	}

	response, err := provider.Query(diagCtx, prompt)
	if err != nil {
		slog.Debug("proactive alert: AI diagnosis failed",
			slog.String("alertID", evt.ID),
			slog.Any("error", err))
		return
	}

	// Emit an updated event with the AI diagnosis attached.
	evt.Message = response
	m.app.emitter.Emit("ai:proactive-alert:diagnosed", evt)
	slog.Info("proactive alert: AI diagnosis emitted", slog.String("alertID", evt.ID))
}
