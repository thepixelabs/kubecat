package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AnthropicProvider implements Provider for Anthropic Claude.
type AnthropicProvider struct {
	config ProviderConfig
	client *http.Client
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(config ProviderConfig) *AnthropicProvider {
	if config.Endpoint == "" {
		config.Endpoint = "https://api.anthropic.com/v1"
	}
	if config.Model == "" {
		config.Model = "claude-3-sonnet-20240229"
	}
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}

	return &AnthropicProvider{
		config: config,
		client: NewCloudHTTPClient(config.Timeout),
	}
}

// Name returns the provider name.
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// Available checks if Anthropic is available.
func (p *AnthropicProvider) Available(ctx context.Context) bool {
	// Just check if API key is configured
	return p.config.APIKey != ""
}

// Query sends a query and returns the response.
func (p *AnthropicProvider) Query(ctx context.Context, prompt string) (string, error) {
	reqBody := anthropicRequest{
		Model:     p.config.Model,
		MaxTokens: p.config.MaxTokens,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
		System: "You are a Kubernetes expert assistant. Help users understand and troubleshoot their Kubernetes clusters.",
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.Endpoint+"/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Anthropic error: %s - %s", resp.Status, string(body))
	}

	var result anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("no response from Anthropic")
	}

	return result.Content[0].Text, nil
}

// StreamQuery sends a query and streams the response.
func (p *AnthropicProvider) StreamQuery(ctx context.Context, prompt string) (<-chan string, error) {
	reqBody := anthropicRequest{
		Model:     p.config.Model,
		MaxTokens: p.config.MaxTokens,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
		System: "You are a Kubernetes expert assistant. Help users understand and troubleshoot their Kubernetes clusters.",
		Stream: true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.Endpoint+"/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("Anthropic error: %s - %s", resp.Status, string(body))
	}

	out := make(chan string, 100)

	go func() {
		defer close(out)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "" {
				continue
			}

			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			if event.Type == "content_block_delta" && event.Delta.Text != "" {
				select {
				case out <- event.Delta.Text:
				case <-ctx.Done():
					return
				}
			}

			if event.Type == "message_stop" {
				return
			}
		}
	}()

	return out, nil
}

// Close closes any connections.
func (p *AnthropicProvider) Close() error {
	p.client.CloseIdleConnections()
	return nil
}

// Anthropic API types

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
	Stream    bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Text string `json:"text"`
	} `json:"delta,omitempty"`
}
