// SPDX-License-Identifier: Apache-2.0

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

// OpenAIProvider implements Provider for OpenAI.
type OpenAIProvider struct {
	config ProviderConfig
	client *http.Client
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(config ProviderConfig) *OpenAIProvider {
	if config.Endpoint == "" {
		config.Endpoint = "https://api.openai.com/v1"
	}
	if config.Model == "" {
		config.Model = "gpt-4"
	}
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}

	return &OpenAIProvider{
		config: config,
		client: NewCloudHTTPClient(config.Timeout),
	}
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// Available checks if OpenAI is available.
func (p *OpenAIProvider) Available(ctx context.Context) bool {
	if p.config.APIKey == "" {
		return false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.config.Endpoint+"/models", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Query sends a query and returns the response.
func (p *OpenAIProvider) Query(ctx context.Context, prompt string) (string, error) {
	reqBody := openAIChatRequest{
		Model: p.config.Model,
		Messages: []openAIMessage{
			{Role: "system", Content: "You are a Kubernetes expert assistant."},
			{Role: "user", Content: prompt},
		},
		Temperature: p.config.Temperature,
		MaxTokens:   p.config.MaxTokens,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.Endpoint+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenAI error: %s - %s", resp.Status, string(body))
	}

	var result openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	return result.Choices[0].Message.Content, nil
}

// StreamQuery sends a query and streams the response.
func (p *OpenAIProvider) StreamQuery(ctx context.Context, prompt string) (<-chan string, error) {
	reqBody := openAIChatRequest{
		Model: p.config.Model,
		Messages: []openAIMessage{
			{Role: "system", Content: "You are a Kubernetes expert assistant."},
			{Role: "user", Content: prompt},
		},
		Temperature: p.config.Temperature,
		MaxTokens:   p.config.MaxTokens,
		Stream:      true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.Endpoint+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("OpenAI error: %s - %s", resp.Status, string(body))
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
			if data == "[DONE]" {
				return
			}

			var chunk openAIStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				select {
				case out <- chunk.Choices[0].Delta.Content:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out, nil
}

// Close closes any connections.
func (p *OpenAIProvider) Close() error {
	p.client.CloseIdleConnections()
	return nil
}

// OpenAI API types

type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}
