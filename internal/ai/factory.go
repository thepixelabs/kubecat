package ai

import "fmt"

// NewProvider constructs a Provider for the given provider id and base config.
//
// Centralizing this switch eliminates the four copy-pasted constructions that
// previously lived in app.go (AIQuery, AIQueryWithContext, AIAnalyzeResource,
// QuerySecurityAI). Callers should still call provider.Close() when done.
func NewProvider(providerID string, cfg ProviderConfig) (Provider, error) {
	switch providerID {
	case "openai", "litellm":
		return NewOpenAIProvider(cfg), nil
	case "anthropic":
		return NewAnthropicProvider(cfg), nil
	case "google":
		return NewGoogleProvider(cfg), nil
	case "ollama":
		return NewOllamaProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unknown AI provider: %s", providerID)
	}
}
