package main

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/thepixelabs/kubecat/internal/ai"
	"github.com/thepixelabs/kubecat/internal/audit"
	"github.com/thepixelabs/kubecat/internal/config"
	"github.com/thepixelabs/kubecat/internal/storage"
)

// AIQuery sends a query to the configured AI provider and returns the response.
func (a *App) AIQuery(query string) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}

	if !cfg.Kubecat.AI.Enabled {
		return "", fmt.Errorf("AI features are not enabled. Please enable them in Settings.")
	}

	selectedProvider := cfg.Kubecat.AI.SelectedProvider
	if selectedProvider == "" {
		return "", fmt.Errorf("No AI provider selected. Please configure one in Settings.")
	}

	providerConfig, ok := cfg.Kubecat.AI.Providers[selectedProvider]
	if !ok || !providerConfig.Enabled {
		return "", fmt.Errorf("Selected AI provider (%s) is not enabled or configured.", selectedProvider)
	}

	// Create provider based on configuration
	providerCfg := ai.ProviderConfig{
		Model:    cfg.Kubecat.AI.SelectedModel,
		APIKey:   providerConfig.APIKey,
		Endpoint: providerConfig.Endpoint,
	}

	provider, err := ai.NewProvider(selectedProvider, providerCfg)
	if err != nil {
		return "", fmt.Errorf("Unknown AI provider: %s", selectedProvider)
	}
	defer func() { _ = provider.Close() }()

	// Check if provider is available
	if !provider.Available(a.ctx) {
		if selectedProvider == "ollama" {
			return "", fmt.Errorf("Ollama is not running. Please start Ollama and try again.")
		}
		return "", fmt.Errorf("AI provider is not available. Please check your API key and settings.")
	}

	// Sanitize the raw user query before sending to cloud providers.
	// The user may have typed resource names or pasted YAML into the query box.
	sanitizedQuery := query
	if ai.IsCloudProvider(selectedProvider) {
		sanitizedQuery = ai.SanitizeForCloud(query)
	}

	// Audit the query — log hash and provider metadata only, never the query text.
	audit.LogAIQuery(selectedProvider, a.nexus.Clusters.ActiveContext(), "", sanitizedQuery)

	// Send query
	response, err := provider.Query(a.ctx, sanitizedQuery)
	if err != nil {
		return "", fmt.Errorf("AI query failed: %w", err)
	}

	return response, nil
}

// conversationHistoryLimit is the maximum number of previous messages included
// in the conversation history injected into the prompt.  Older messages beyond
// this window are dropped to keep prompts within reasonable token budgets.
const conversationHistoryLimit = 10

// ConversationMessage is a serializable message type used when passing
// conversation history from the frontend.  Wails requires all bound-method
// parameters to be JSON-serializable; using this local struct avoids importing
// internal/ai from the method signature.
type ConversationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AIQueryWithContext sends a query to the AI with additional resource context
// and optional conversation history for multi-turn interactions.
func (a *App) AIQueryWithContext(query string, contextItems []AIContextItem, providerID string, modelID string, previousMessages []ConversationMessage) (string, error) {
	// Note: do not log the query text — it may contain sensitive resource names or PII.
	log := slog.With(
		slog.String("operation", "AIQueryWithContext"),
		slog.Int("contextItems", len(contextItems)),
		slog.Int("historyMessages", len(previousMessages)),
		slog.String("provider", providerID),
		slog.String("model", modelID),
	)
	log.Debug("AI query with context called")

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config for AI query", slog.Any("error", err))
		return "", err
	}

	if !cfg.Kubecat.AI.Enabled {
		log.Info("AI query rejected: AI features not enabled")
		return "", fmt.Errorf("AI features are not enabled. Please enable them in Settings.")
	}

	// Use provided settings or fallback to config defaults
	selectedProvider := providerID
	if selectedProvider == "" {
		selectedProvider = cfg.Kubecat.AI.SelectedProvider
	}

	selectedModel := modelID
	if selectedModel == "" {
		selectedModel = cfg.Kubecat.AI.SelectedModel
	}

	if selectedProvider == "" {
		log.Warn("AI query rejected: no provider selected")
		return "", fmt.Errorf("No AI provider selected. Please configure one in Settings.")
	}

	providerConfig, ok := cfg.Kubecat.AI.Providers[selectedProvider]
	if !ok || !providerConfig.Enabled {
		return "", fmt.Errorf("Selected AI provider (%s) is not enabled or configured.", selectedProvider)
	}

	log.Debug("resolved AI provider",
		slog.String("resolvedProvider", selectedProvider),
		slog.String("resolvedModel", selectedModel),
	)

	// Create provider based on configuration.
	// Note: APIKey is never logged — it is passed only to the provider.
	providerCfg := ai.ProviderConfig{
		Model:    selectedModel,
		APIKey:   providerConfig.APIKey,
		Endpoint: providerConfig.Endpoint,
	}

	provider, err := ai.NewProvider(selectedProvider, providerCfg)
	if err != nil {
		return "", fmt.Errorf("Unknown AI provider: %s", selectedProvider)
	}
	defer func() { _ = provider.Close() }()

	if !provider.Available(a.ctx) {
		if selectedProvider == "ollama" {
			log.Warn("AI provider not available: Ollama not running")
			return "", fmt.Errorf("Ollama is not running. Please start Ollama and try again.")
		}
		log.Warn("AI provider not available")
		return "", fmt.Errorf("AI provider is not available. Please check your API key and settings.")
	}

	log.Debug("building contextual prompt")
	// Build enhanced prompt with context
	enhancedPrompt, err := a.buildContextualPrompt(query, contextItems)
	if err != nil {
		log.Error("failed to build contextual prompt", slog.Any("error", err))
		return "", fmt.Errorf("failed to build context: %w", err)
	}

	// Prepend conversation history so the model can follow up on prior turns.
	// We cap the window to conversationHistoryLimit messages (most recent) to
	// avoid unbounded token growth.  Each message is formatted as a simple
	// labeled block that any model can parse without provider-specific APIs.
	if len(previousMessages) > 0 {
		history := previousMessages
		if len(history) > conversationHistoryLimit {
			history = history[len(history)-conversationHistoryLimit:]
		}

		var historyBuilder strings.Builder
		historyBuilder.WriteString("=== CONVERSATION HISTORY (most recent last) ===\n")
		for _, m := range history {
			role := m.Role
			if role != "user" && role != "assistant" {
				role = "user" // default to user for unknown roles
			}
			historyBuilder.WriteString(fmt.Sprintf("[%s]: %s\n\n", strings.ToUpper(role), m.Content))
		}
		historyBuilder.WriteString("=== END CONVERSATION HISTORY ===\n\n")
		historyBuilder.WriteString(enhancedPrompt)
		enhancedPrompt = historyBuilder.String()
	}

	// Sanitize the full prompt (resource YAML + user query) before sending to
	// a cloud provider.  buildContextualPrompt embeds raw resource YAML; we
	// must ensure no residual secrets survive the GatherContext path.
	if ai.IsCloudProvider(selectedProvider) {
		enhancedPrompt = ai.SanitizeForCloud(enhancedPrompt)
	}

	// Audit the query — log hash and provider metadata only, never the prompt text.
	audit.LogAIQuery(selectedProvider, a.nexus.Clusters.ActiveContext(), "", enhancedPrompt)

	log.Debug("sending query to AI provider")
	start := time.Now()
	// Send query
	response, err := provider.Query(a.ctx, enhancedPrompt)
	if err != nil {
		log.Error("AI query failed", slog.Any("error", err), slog.Duration("duration", time.Since(start)))
		return "", fmt.Errorf("AI query failed: %w", err)
	}

	log.Info("AI query completed",
		slog.Int("responseBytes", len(response)),
		slog.Duration("duration", time.Since(start)),
	)
	return response, nil
}

// buildContextualPrompt enhances the user query with resource context.
func (a *App) buildContextualPrompt(query string, contextItems []AIContextItem) (string, error) {
	if len(contextItems) == 0 {
		return query, nil
	}

	var contextBuilder strings.Builder
	contextBuilder.WriteString("You are a Kubernetes expert assistant running inside 'Kubecat', a powerful desktop dashboard. The user has attached the following resources for context:\n")
	contextBuilder.WriteString("IMPORTANT: You are integrated with the user's terminal. You CANNOT execute commands directly, but the user can Run any code block you provide with one click. NEVER say 'I cannot execute commands'. Instead, provide the exact `kubectl` or shell commands to solve the prompt in ```bash``` blocks and ask the user to run them.\n\n")

	// Initialize ContextBuilder
	eventRepo := storage.NewEventRepository(a.db)
	manager := a.nexus.Clusters.Manager()
	aiBuilder := ai.NewContextBuilder(manager, eventRepo)

	// Gather context for each item
	for i, item := range contextItems {
		contextBuilder.WriteString(fmt.Sprintf("## Resource %d: %s/%s", i+1, item.Type, item.Name))
		if item.Namespace != "" {
			contextBuilder.WriteString(fmt.Sprintf(" (namespace: %s)", item.Namespace))
		}
		contextBuilder.WriteString("\n\n")

		// Gather full context similar to AnalyzeResource
		req := ai.AnalysisRequest{
			Kind:      item.Type,
			Namespace: item.Namespace,
			Name:      item.Name,
		}

		actx, err := aiBuilder.GatherContext(a.ctx, req)
		if err != nil {
			contextBuilder.WriteString(fmt.Sprintf("⚠️ Could not gather context: %v\n\n", err))
			continue
		}

		// Add YAML
		if actx.ResourceYAML != "" {
			contextBuilder.WriteString("### Resource YAML:\n```yaml\n")
			contextBuilder.WriteString(actx.ResourceYAML)
			contextBuilder.WriteString("\n```\n\n")
		}

		// Add events
		if len(actx.Events) > 0 {
			contextBuilder.WriteString("### Recent Events:\n")
			for _, event := range actx.Events {
				contextBuilder.WriteString(fmt.Sprintf("- [%s] %s: %s\n",
					event.Type, event.Reason, event.Message))
			}
			contextBuilder.WriteString("\n")
		}

		// Add logs for pods
		if strings.ToLower(item.Type) == "pod" && actx.Logs != "" {
			contextBuilder.WriteString("### Recent Logs:\n```\n")
			// Limit log output to avoid token overflow
			logLines := strings.Split(actx.Logs, "\n")
			maxLines := 30
			if len(logLines) > maxLines {
				contextBuilder.WriteString(strings.Join(logLines[len(logLines)-maxLines:], "\n"))
			} else {
				contextBuilder.WriteString(actx.Logs)
			}
			contextBuilder.WriteString("\n```\n\n")
		}
	}

	contextBuilder.WriteString("---\n\n")
	contextBuilder.WriteString("User Question: ")
	contextBuilder.WriteString(query)
	contextBuilder.WriteString("\n\nYour output must be in HTML format wrapped in a <div class=\"ai-summary\"> block.\n")
	contextBuilder.WriteString("Use the following structure:\n")
	contextBuilder.WriteString(`<div class="ai-summary">
  <div class="summary-header">
    <h3>Executive Summary</h3>
    <p>Direct answer to the question.</p>
  </div>
  <div class="key-findings">
    <h3>Key Points</h3>
    <ul class="findings-list">
      <li class="finding-item">Point 1</li>
    </ul>
  </div>
  <div class="recommendations">
    <h3>Next Steps</h3>
    <div class="recommendation-item">
      Actionable step.
    </div>
  </div>
</div>`)
	contextBuilder.WriteString("\n\nUse <code> tags for inline names. **Crucial:** For actionable commands that the user should run, use standard Markdown code blocks (```bash ... ```) so they can be executed. Do NOT put actionable commands inside HTML tags.\n")

	return contextBuilder.String(), nil
}

// AIAnalyzeResource performs a deep AI analysis of a specific resource.
func (a *App) AIAnalyzeResource(kind, namespace, name string) (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}

	if !cfg.Kubecat.AI.Enabled {
		return "", fmt.Errorf("AI features are not enabled. Please enable them in Settings.")
	}

	aiCfg := cfg.Kubecat.AI
	selectedProvider := aiCfg.SelectedProvider
	if selectedProvider == "" {
		return "", fmt.Errorf("No AI provider selected. Please configure one in Settings.")
	}

	providerConfig, ok := aiCfg.Providers[selectedProvider]
	if !ok || !providerConfig.Enabled {
		return "", fmt.Errorf("Selected AI provider (%s) is not enabled or configured.", selectedProvider)
	}

	// Create provider based on configuration
	providerCfg := ai.ProviderConfig{
		Model:    aiCfg.SelectedModel,
		APIKey:   providerConfig.APIKey,
		Endpoint: providerConfig.Endpoint,
	}

	provider, err := ai.NewProvider(selectedProvider, providerCfg)
	if err != nil {
		return "", fmt.Errorf("Unknown AI provider: %s", selectedProvider)
	}
	defer func() { _ = provider.Close() }()

	if !provider.Available(a.ctx) {
		return "", fmt.Errorf("AI provider is not available. Please check your API key and settings.")
	}

	// Initialize ContextBuilder
	eventRepo := storage.NewEventRepository(a.db)
	manager := a.nexus.Clusters.Manager()
	builder := ai.NewContextBuilder(manager, eventRepo)

	// Analyze
	req := ai.AnalysisRequest{
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
	}

	// Create a context with timeout to avoid hanging
	ctx, cancel := context.WithTimeout(a.ctx, 2*time.Minute)
	defer cancel()

	// Audit the analysis request — kind/namespace/name are metadata, not sensitive.
	audit.LogAIQuery(selectedProvider, a.nexus.Clusters.ActiveContext(), namespace, kind+"/"+namespace+"/"+name)

	return builder.AnalyzeResource(ctx, provider, req)
}

// allowedCommands is the strict allowlist of binary names that ExecuteCommand
// may invoke. Shell interpreters (bash, sh, zsh, python, …) are intentionally
// absent. Extend only after a security review.
var allowedCommands = map[string]bool{
	"kubectl": true,
	"helm":    true,
	"flux":    true,
	"argocd":  true,
}

// shellMetaRe matches characters that have special meaning to a POSIX shell.
// Any command string containing one of these characters is rejected outright
// before argument splitting so that no injection is possible even if the
// tokeniser has a bug.
var shellMetaRe = strings.NewReplacer(
	";", "", "&&", "", "||", "", "|", "", "$", "", "`", "",
	">", "", "<", "", "!", "", "#", "", "~", "", "*", "",
	"?", "", "(", "", ")", "", "{", "", "}", "", "\n", "",
	"\r", "", "\t", "",
)

// containsShellMeta reports whether s contains any shell metacharacter.
func containsShellMeta(s string) bool {
	return shellMetaRe.Replace(s) != s
}

// ExecuteCommand executes a validated, allowlisted command and returns the
// combined stdout+stderr output. It NEVER uses a shell interpreter — the
// command string is split into tokens and passed directly to exec.Command so
// that no shell expansion or injection is possible.
//
// Security invariants:
//   - Only binaries in allowedCommands may be executed.
//   - The raw command string is rejected if it contains any shell metacharacter.
//   - Arguments are passed individually to the OS — never through a shell.
//   - Every invocation (allowed and denied) is audit-logged via slog.
func (a *App) ExecuteCommand(command string) (string, error) {
	// Audit log helper — always fires, regardless of outcome.
	auditLog := func(allowed bool, reason string) {
		outcome := "allowed"
		if !allowed {
			outcome = "denied"
		}
		// Structured audit entry — command is hashed, never stored in plain text.
		tokens := strings.Fields(command)
		binary := ""
		if len(tokens) > 0 {
			binary = tokens[0]
		}
		audit.LogCommandExecution(a.nexus.Clusters.ActiveContext(), command, binary, outcome)
		if allowed {
			slog.Info("ExecuteCommand: allowed",
				slog.String("command", command),
			)
		} else {
			slog.Warn("ExecuteCommand: denied",
				slog.String("command", command),
				slog.String("reason", reason),
			)
		}
	}

	if err := a.checkReadOnly(); err != nil {
		auditLog(false, "read-only mode")
		return "", err
	}

	// Trim surrounding whitespace.
	command = strings.TrimSpace(command)
	if command == "" {
		auditLog(false, "empty command")
		return "", fmt.Errorf("command must not be empty")
	}

	// Reject shell metacharacters before any further processing.
	if containsShellMeta(command) {
		auditLog(false, "shell metacharacter detected")
		return "", fmt.Errorf("command rejected: shell metacharacters are not allowed")
	}

	// Tokenise by whitespace. Because we already rejected all shell metacharacters
	// this is safe — there are no quotes, escapes, or expansions to honor.
	tokens := strings.Fields(command)
	if len(tokens) == 0 {
		auditLog(false, "empty token list after split")
		return "", fmt.Errorf("command must not be empty")
	}

	binary := tokens[0]

	// Enforce the allowlist.
	if !allowedCommands[binary] {
		auditLog(false, fmt.Sprintf("binary %q not in allowlist", binary))
		return "", fmt.Errorf("command rejected: %q is not an allowed command (allowed: kubectl, helm, flux, argocd)", binary)
	}

	auditLog(true, "")

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// Pass args individually — no shell involved.
	//nolint:gosec // binary is allowlist-validated above; args are not shell-expanded.
	cmd := exec.CommandContext(ctx, binary, tokens[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("ExecuteCommand: execution failed",
			slog.String("binary", binary),
			slog.Any("error", err),
		)
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}
