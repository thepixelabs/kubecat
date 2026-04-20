// SPDX-License-Identifier: Apache-2.0

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

// providerFactory builds a Provider pointed at the given endpoint.
type providerFactory func(endpoint, apiKey, model string) Provider

// providerFactories covers every Provider implementation reachable through
// NewProvider.  LiteLLM is implemented by OpenAIProvider and is routed the
// same way here — it behaves identically on the wire.
func providerFactories() map[string]providerFactory {
	return map[string]providerFactory{
		"openai": func(ep, k, m string) Provider {
			return NewOpenAIProvider(ProviderConfig{Endpoint: ep, APIKey: k, Model: m, Timeout: 5 * time.Second})
		},
		"anthropic": func(ep, k, m string) Provider {
			return NewAnthropicProvider(ProviderConfig{Endpoint: ep, APIKey: k, Model: m, Timeout: 5 * time.Second})
		},
		"google": func(ep, k, m string) Provider {
			return NewGoogleProvider(ProviderConfig{Endpoint: ep, APIKey: k, Model: m, Timeout: 5 * time.Second})
		},
		"ollama": func(ep, k, m string) Provider {
			return NewOllamaProvider(ProviderConfig{Endpoint: ep, APIKey: k, Model: m, Timeout: 5 * time.Second})
		},
	}
}

// newStubOpenAI responds to POST /chat/completions and GET /models.
func newStubOpenAI(t *testing.T, reply string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
			t.Errorf("OpenAI expects Authorization: Bearer <key>, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": reply}},
			},
		})
	})
	mux.HandleFunc("/models", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
	})
	return httptest.NewServer(mux)
}

// newStubAnthropic responds to POST /messages.
func newStubAnthropic(t *testing.T, reply string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got == "" {
			t.Error("Anthropic expects x-api-key header")
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Error("Anthropic expects anthropic-version header")
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]interface{}{{"text": reply}},
		})
	})
	return httptest.NewServer(mux)
}

// newStubGoogle responds to POST /models/{model}:generateContent.
func newStubGoogle(t *testing.T, reply string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Google encodes the API key in the query string; assert it's present.
		if k := r.URL.Query().Get("key"); k == "" {
			t.Error("Google expects ?key=<apiKey> query parameter")
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{{"text": reply}},
					},
				},
			},
		})
	}))
}

// newStubOllama responds to POST /api/generate and GET /api/tags.
func newStubOllama(t *testing.T, reply string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/generate", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"response": reply})
	})
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"models": []interface{}{}})
	})
	return httptest.NewServer(mux)
}

// stubServerFor maps provider id → httptest server factory.
func stubServerFor(t *testing.T, provider, reply string) *httptest.Server {
	t.Helper()
	switch provider {
	case "openai":
		return newStubOpenAI(t, reply)
	case "anthropic":
		return newStubAnthropic(t, reply)
	case "google":
		return newStubGoogle(t, reply)
	case "ollama":
		return newStubOllama(t, reply)
	}
	t.Fatalf("no stub for provider %q", provider)
	return nil
}

// ---------------------------------------------------------------------------
// Name()
// ---------------------------------------------------------------------------

func TestProvider_Name_ReflectsIdentity(t *testing.T) {
	for id, factory := range providerFactories() {
		t.Run(id, func(t *testing.T) {
			p := factory("", "", "")
			// LiteLLM uses OpenAIProvider under the hood; both report "openai".
			if name := p.Name(); name != id {
				t.Errorf("Name() = %q, want %q", name, id)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Query() happy path — exercises every provider's native request/response
// parsing against a local stub server.
// ---------------------------------------------------------------------------

func TestProvider_Query_SuccessParsesResponse(t *testing.T) {
	for id, factory := range providerFactories() {
		t.Run(id, func(t *testing.T) {
			expected := "hello from " + id
			srv := stubServerFor(t, id, expected)
			t.Cleanup(srv.Close)

			p := factory(srv.URL, "test-api-key", "test-model")
			t.Cleanup(func() { _ = p.Close() })

			resp, err := p.Query(context.Background(), "any prompt")
			if err != nil {
				t.Fatalf("Query: %v", err)
			}
			if resp != expected {
				t.Errorf("Query response = %q, want %q", resp, expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Query() — non-2xx response returns a provider-specific error but never
// panics, and the error text contains a usable status hint.
// ---------------------------------------------------------------------------

func TestProvider_Query_Non2xxReturnsError(t *testing.T) {
	for _, id := range []string{"openai", "anthropic", "google", "ollama"} {
		t.Run(id, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = fmt.Fprint(w, `{"error":"bad key"}`)
			}))
			t.Cleanup(srv.Close)

			p := providerFactories()[id](srv.URL, "bad-key", "m")
			t.Cleanup(func() { _ = p.Close() })

			_, err := p.Query(context.Background(), "prompt")
			if err == nil {
				t.Fatal("expected error on 401 response")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Query() — network failure returns an error.
// ---------------------------------------------------------------------------

func TestProvider_Query_NetworkErrorSurfacedAsError(t *testing.T) {
	for _, id := range []string{"openai", "anthropic", "google", "ollama"} {
		t.Run(id, func(t *testing.T) {
			// Address that is guaranteed not to accept connections.
			p := providerFactories()[id]("http://127.0.0.1:1", "k", "m")
			t.Cleanup(func() { _ = p.Close() })

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			t.Cleanup(cancel)
			_, err := p.Query(ctx, "prompt")
			if err == nil {
				t.Fatal("expected connection error")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Available() semantics per provider.
// ---------------------------------------------------------------------------

func TestOpenAI_Available_TrueWhenListModelsOK(t *testing.T) {
	srv := newStubOpenAI(t, "")
	t.Cleanup(srv.Close)
	p := NewOpenAIProvider(ProviderConfig{Endpoint: srv.URL, APIKey: "k", Timeout: time.Second})
	t.Cleanup(func() { _ = p.Close() })
	if !p.Available(context.Background()) {
		t.Error("Available should be true when /models returns 200")
	}
}

func TestOpenAI_Available_FalseWithoutAPIKey(t *testing.T) {
	p := NewOpenAIProvider(ProviderConfig{Endpoint: "http://127.0.0.1:1", Timeout: time.Second})
	t.Cleanup(func() { _ = p.Close() })
	if p.Available(context.Background()) {
		t.Error("Available should be false when APIKey is empty")
	}
}

func TestOpenAI_Available_FalseOn401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	p := NewOpenAIProvider(ProviderConfig{Endpoint: srv.URL, APIKey: "bad", Timeout: time.Second})
	t.Cleanup(func() { _ = p.Close() })
	if p.Available(context.Background()) {
		t.Error("Available should be false on 401")
	}
}

func TestAnthropic_Available_OnlyChecksAPIKey(t *testing.T) {
	// Anthropic.Available does NOT hit the network — it only checks the key.
	hasKey := NewAnthropicProvider(ProviderConfig{APIKey: "k"})
	t.Cleanup(func() { _ = hasKey.Close() })
	if !hasKey.Available(context.Background()) {
		t.Error("Anthropic.Available should be true when key is set")
	}

	noKey := NewAnthropicProvider(ProviderConfig{})
	t.Cleanup(func() { _ = noKey.Close() })
	if noKey.Available(context.Background()) {
		t.Error("Anthropic.Available should be false when key is empty")
	}
}

func TestGoogle_Available_FalseWithoutAPIKey(t *testing.T) {
	p := NewGoogleProvider(ProviderConfig{})
	t.Cleanup(func() { _ = p.Close() })
	if p.Available(context.Background()) {
		t.Error("Google.Available should be false without API key")
	}
}

func TestGoogle_Available_TrueWhenEndpointOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") == "" {
			t.Error("Google availability probe must include ?key=")
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	p := NewGoogleProvider(ProviderConfig{Endpoint: srv.URL, APIKey: "k", Timeout: time.Second})
	t.Cleanup(func() { _ = p.Close() })
	if !p.Available(context.Background()) {
		t.Error("Google.Available should be true on 200")
	}
}

func TestOllama_Available_TrueWhenTagsOK(t *testing.T) {
	srv := newStubOllama(t, "")
	t.Cleanup(srv.Close)
	p := NewOllamaProvider(ProviderConfig{Endpoint: srv.URL, Timeout: time.Second})
	t.Cleanup(func() { _ = p.Close() })
	if !p.Available(context.Background()) {
		t.Error("Ollama.Available should be true when /api/tags returns 200")
	}
}

func TestOllama_Available_FalseWhenServerDown(t *testing.T) {
	p := NewOllamaProvider(ProviderConfig{Endpoint: "http://127.0.0.1:1", Timeout: time.Second})
	t.Cleanup(func() { _ = p.Close() })
	if p.Available(context.Background()) {
		t.Error("Ollama.Available should be false when server unreachable")
	}
}

// ---------------------------------------------------------------------------
// StreamQuery — basic happy path per provider.
// ---------------------------------------------------------------------------

func TestOpenAI_StreamQuery_EmitsTokensInOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// SSE-style stream with three tokens then DONE.
		for _, tok := range []string{"hello ", "world", "!"} {
			chunk := map[string]interface{}{
				"choices": []map[string]interface{}{
					{"delta": map[string]interface{}{"content": tok}},
				},
			}
			b, _ := json.Marshal(chunk)
			_, _ = fmt.Fprintf(w, "data: %s\n", b)
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n")
	}))
	t.Cleanup(srv.Close)

	p := NewOpenAIProvider(ProviderConfig{Endpoint: srv.URL, APIKey: "k", Model: "m", Timeout: 5 * time.Second})
	t.Cleanup(func() { _ = p.Close() })

	ch, err := p.StreamQuery(context.Background(), "hi")
	if err != nil {
		t.Fatalf("StreamQuery: %v", err)
	}

	var got []string
	for tok := range ch {
		got = append(got, tok)
	}
	if strings.Join(got, "") != "hello world!" {
		t.Errorf("stream result = %q, want %q", strings.Join(got, ""), "hello world!")
	}
}

func TestAnthropic_StreamQuery_EmitsContentBlockDeltas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		events := []map[string]interface{}{
			{"type": "content_block_delta", "delta": map[string]interface{}{"text": "hi "}},
			{"type": "content_block_delta", "delta": map[string]interface{}{"text": "there"}},
			{"type": "message_stop"},
		}
		for _, e := range events {
			b, _ := json.Marshal(e)
			_, _ = fmt.Fprintf(w, "data: %s\n", b)
		}
	}))
	t.Cleanup(srv.Close)

	p := NewAnthropicProvider(ProviderConfig{Endpoint: srv.URL, APIKey: "k", Model: "m", Timeout: 5 * time.Second})
	t.Cleanup(func() { _ = p.Close() })

	ch, err := p.StreamQuery(context.Background(), "hi")
	if err != nil {
		t.Fatalf("StreamQuery: %v", err)
	}
	var got []string
	for tok := range ch {
		got = append(got, tok)
	}
	if strings.Join(got, "") != "hi there" {
		t.Errorf("stream result = %q", strings.Join(got, ""))
	}
}

func TestOllama_StreamQuery_EmitsNDJSONChunks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		chunks := []map[string]interface{}{
			{"response": "foo ", "done": false},
			{"response": "bar", "done": false},
			{"response": "", "done": true},
		}
		for _, c := range chunks {
			b, _ := json.Marshal(c)
			_, _ = w.Write(b)
			_, _ = w.Write([]byte("\n"))
		}
	}))
	t.Cleanup(srv.Close)

	p := NewOllamaProvider(ProviderConfig{Endpoint: srv.URL, Model: "m", Timeout: 5 * time.Second})
	t.Cleanup(func() { _ = p.Close() })

	ch, err := p.StreamQuery(context.Background(), "hi")
	if err != nil {
		t.Fatalf("StreamQuery: %v", err)
	}
	var got []string
	for tok := range ch {
		got = append(got, tok)
	}
	if strings.Join(got, "") != "foo bar" {
		t.Errorf("stream result = %q", strings.Join(got, ""))
	}
}

// ---------------------------------------------------------------------------
// StreamQuery — error on non-2xx upstream.
// ---------------------------------------------------------------------------

func TestProvider_StreamQuery_Non2xxReturnsError(t *testing.T) {
	for _, id := range []string{"openai", "anthropic", "google", "ollama"} {
		t.Run(id, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			}))
			t.Cleanup(srv.Close)

			p := providerFactories()[id](srv.URL, "k", "m")
			t.Cleanup(func() { _ = p.Close() })

			_, err := p.StreamQuery(context.Background(), "prompt")
			if err == nil {
				t.Error("StreamQuery should return error on non-2xx upstream")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Close() is idempotent.
// ---------------------------------------------------------------------------

func TestProvider_Close_Idempotent(t *testing.T) {
	for id, factory := range providerFactories() {
		t.Run(id, func(t *testing.T) {
			p := factory("http://127.0.0.1:1", "k", "m")
			if err := p.Close(); err != nil {
				t.Errorf("first Close() = %v, want nil", err)
			}
			if err := p.Close(); err != nil {
				t.Errorf("second Close() = %v, want nil (must be idempotent)", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Constructor defaults — verify each New<Provider> fills in sensible defaults.
// ---------------------------------------------------------------------------

func TestProviderConstructors_ApplyDefaults(t *testing.T) {
	cases := []struct {
		name         string
		build        func() Provider
		wantEndpoint string
		wantModel    string
	}{
		{
			name:         "openai",
			build:        func() Provider { return NewOpenAIProvider(ProviderConfig{}) },
			wantEndpoint: "https://api.openai.com/v1",
			wantModel:    "gpt-4",
		},
		{
			name:         "anthropic",
			build:        func() Provider { return NewAnthropicProvider(ProviderConfig{}) },
			wantEndpoint: "https://api.anthropic.com/v1",
			wantModel:    "claude-3-sonnet-20240229",
		},
		{
			name:         "google",
			build:        func() Provider { return NewGoogleProvider(ProviderConfig{}) },
			wantEndpoint: "https://generativelanguage.googleapis.com/v1beta",
			wantModel:    "gemini-pro",
		},
		{
			name:         "ollama",
			build:        func() Provider { return NewOllamaProvider(ProviderConfig{}) },
			wantEndpoint: "http://localhost:11434",
			wantModel:    "llama3.2",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.build()
			t.Cleanup(func() { _ = p.Close() })

			// We can't introspect private config fields from outside the type,
			// but we can verify the defaults are in effect by consulting the
			// concrete type assertions.
			switch v := p.(type) {
			case *OpenAIProvider:
				if v.config.Endpoint != tc.wantEndpoint {
					t.Errorf("endpoint = %q, want %q", v.config.Endpoint, tc.wantEndpoint)
				}
				if v.config.Model != tc.wantModel {
					t.Errorf("model = %q, want %q", v.config.Model, tc.wantModel)
				}
				if v.config.Timeout <= 0 {
					t.Errorf("timeout must default to non-zero, got %v", v.config.Timeout)
				}
			case *AnthropicProvider:
				if v.config.Endpoint != tc.wantEndpoint {
					t.Errorf("endpoint = %q, want %q", v.config.Endpoint, tc.wantEndpoint)
				}
				if v.config.Model != tc.wantModel {
					t.Errorf("model = %q, want %q", v.config.Model, tc.wantModel)
				}
			case *GoogleProvider:
				if v.config.Endpoint != tc.wantEndpoint {
					t.Errorf("endpoint = %q, want %q", v.config.Endpoint, tc.wantEndpoint)
				}
				if v.config.Model != tc.wantModel {
					t.Errorf("model = %q, want %q", v.config.Model, tc.wantModel)
				}
			case *OllamaProvider:
				if v.config.Endpoint != tc.wantEndpoint {
					t.Errorf("endpoint = %q, want %q", v.config.Endpoint, tc.wantEndpoint)
				}
				if v.config.Model != tc.wantModel {
					t.Errorf("model = %q, want %q", v.config.Model, tc.wantModel)
				}
			default:
				t.Fatalf("unexpected provider type %T", p)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Ollama ListModels — not part of the Provider interface but a public method.
// ---------------------------------------------------------------------------

func TestOllama_ListModels_ParsesTagsResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"models": []map[string]interface{}{
				{"name": "llama3.2:latest"},
				{"name": "mistral:7b"},
			},
		})
	}))
	t.Cleanup(srv.Close)

	p := NewOllamaProvider(ProviderConfig{Endpoint: srv.URL, Timeout: time.Second})
	t.Cleanup(func() { _ = p.Close() })

	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 2 || models[0] != "llama3.2:latest" || models[1] != "mistral:7b" {
		t.Errorf("models = %v", models)
	}
}

// ---------------------------------------------------------------------------
// OpenAI / Anthropic Query with empty Choices → surfaces as error.
// ---------------------------------------------------------------------------

func TestOpenAI_Query_EmptyChoicesIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"choices": []interface{}{}})
	}))
	t.Cleanup(srv.Close)

	p := NewOpenAIProvider(ProviderConfig{Endpoint: srv.URL, APIKey: "k", Model: "m", Timeout: time.Second})
	t.Cleanup(func() { _ = p.Close() })

	if _, err := p.Query(context.Background(), "hi"); err == nil {
		t.Error("OpenAI.Query should return error when choices is empty")
	}
}

func TestAnthropic_Query_EmptyContentIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"content": []interface{}{}})
	}))
	t.Cleanup(srv.Close)

	p := NewAnthropicProvider(ProviderConfig{Endpoint: srv.URL, APIKey: "k", Model: "m", Timeout: time.Second})
	t.Cleanup(func() { _ = p.Close() })

	if _, err := p.Query(context.Background(), "hi"); err == nil {
		t.Error("Anthropic.Query should return error when content is empty")
	}
}

// ---------------------------------------------------------------------------
// toolParameterSchema builds a valid JSON schema.
// ---------------------------------------------------------------------------

func TestToolParameterSchema_EmitsRequiredWhenPresent(t *testing.T) {
	params := []ParameterSchema{
		{Name: "kind", Type: "string", Description: "Resource kind", Required: true},
		{Name: "namespace", Type: "string", Description: "Namespace", Required: false},
	}
	schema := toolParameterSchema(params)

	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want object", schema["type"])
	}
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("schema.properties missing or wrong type")
	}
	if _, ok := props["kind"]; !ok {
		t.Error("schema.properties.kind missing")
	}
	if _, ok := props["namespace"]; !ok {
		t.Error("schema.properties.namespace missing")
	}
	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("schema.required missing or wrong type: %T", schema["required"])
	}
	if len(required) != 1 || required[0] != "kind" {
		t.Errorf("schema.required = %v, want [kind]", required)
	}
}

func TestToolParameterSchema_OmitsRequiredWhenEmpty(t *testing.T) {
	params := []ParameterSchema{
		{Name: "a", Type: "string", Description: "", Required: false},
	}
	schema := toolParameterSchema(params)
	if _, has := schema["required"]; has {
		t.Error("schema.required must be omitted when no parameter is required")
	}
}
