package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GoogleProvider implements Provider for Google Gemini.
type GoogleProvider struct {
	config ProviderConfig
	client *http.Client
}

// NewGoogleProvider creates a new Google provider.
func NewGoogleProvider(config ProviderConfig) *GoogleProvider {
	if config.Endpoint == "" {
		config.Endpoint = "https://generativelanguage.googleapis.com/v1beta"
	}
	if config.Model == "" {
		config.Model = "gemini-pro"
	}
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}

	return &GoogleProvider{
		config: config,
		client: NewCloudHTTPClient(config.Timeout),
	}
}

// Name returns the provider name.
func (p *GoogleProvider) Name() string {
	return "google"
}

// Available checks if Google Gemini is available.
func (p *GoogleProvider) Available(ctx context.Context) bool {
	if p.config.APIKey == "" {
		return false
	}

	// For Google, we can check availability by listing models or making a dry run.
	// We'll use list models endpoint with key.
	url := fmt.Sprintf("%s/models?key=%s", p.config.Endpoint, p.config.APIKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// Query sends a query and returns the response.
func (p *GoogleProvider) Query(ctx context.Context, prompt string) (string, error) {
	reqBody := googleContentRequest{
		Contents: []googleContent{
			{
				Parts: []googlePart{
					{Text: "You are a Kubernetes expert assistant.\n\n" + prompt},
				},
			},
		},
		GenerationConfig: googleGenerationConfig{
			Temperature:     p.config.Temperature,
			MaxOutputTokens: p.config.MaxTokens,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use model name directly — Gemini API accepts both "gemini-pro" and "models/gemini-pro".
	model := p.config.Model

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.config.Endpoint, model, p.config.APIKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Google error: %s - %s", resp.Status, string(body))
	}

	var result googleContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Google")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}

// StreamQuery sends a query and streams the response.
func (p *GoogleProvider) StreamQuery(ctx context.Context, prompt string) (<-chan string, error) {
	reqBody := googleContentRequest{
		Contents: []googleContent{
			{
				Parts: []googlePart{
					{Text: "You are a Kubernetes expert assistant.\n\n" + prompt},
				},
			},
		},
		GenerationConfig: googleGenerationConfig{
			Temperature:     p.config.Temperature,
			MaxOutputTokens: p.config.MaxTokens,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?key=%s", p.config.Endpoint, p.config.Model, p.config.APIKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("Google error: %s - %s", resp.Status, string(body))
	}

	out := make(chan string, 100)

	go func() {
		defer close(out)
		defer resp.Body.Close()

		// Google stream returns a JSON array, but each chunk is sent progressively?
		// Actually, streamGenerateContent returns a stream of JSON objects, usually separated by newline or just concatenated.
		// It is not Server-Sent Events (SSE) like OpenAI.
		// But it returns a partial response.
		// "The response is a standard HTTP response with a stream of JSON objects."
		// Wait, typically it sends a JSON array `[{}, {}, ...]` but streamed?
		// Or acts like SSE?
		// Documentation says: "The response is a stream of GenerateContentResponse messages."
		// For REST, it returns "partial JSON objects".

		// Typically, we can read the body. It might be a persistent connection sending chunks.
		// However, decoding complex streamed JSON is tricky.
		// Let's assume for now we might need to handle it carefully.
		// Simplest approach for stream in Go with `json.Decoder`:

		decoder := json.NewDecoder(resp.Body)
		// It returns a JSON array logic?
		// Actually, looking at Google's docs, `streamGenerateContent` returns a stream of JSON objects.
		// In Go, `json.Decoder` can decode a stream of objects if they are just concatenated.
		// But if it's an array `[...]`, we need to consume `[` first.

		// Let's try to sense the start token.
		token, err := decoder.Token()
		if err != nil {
			return
		}

		// Expect start of array '['
		if delim, ok := token.(json.Delim); !ok || delim != '[' {
			// If not array, maybe just single object? Or error.
			return
		}

		for decoder.More() {
			var chunk googleContentResponse
			if err := decoder.Decode(&chunk); err != nil {
				return
			}

			if len(chunk.Candidates) > 0 && len(chunk.Candidates[0].Content.Parts) > 0 {
				select {
				case out <- chunk.Candidates[0].Content.Parts[0].Text:
				case <-ctx.Done():
					return
				}
			}
		}

		// Consume closing ']'
		_, _ = decoder.Token()
	}()

	return out, nil
}

// Close closes any connections.
func (p *GoogleProvider) Close() error {
	p.client.CloseIdleConnections()
	return nil
}

// Google API types

type googleContentRequest struct {
	Contents         []googleContent        `json:"contents"`
	GenerationConfig googleGenerationConfig `json:"generationConfig,omitempty"`
}

type googleContent struct {
	Parts []googlePart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type googlePart struct {
	Text string `json:"text"`
}

type googleGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type googleContentResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
}
