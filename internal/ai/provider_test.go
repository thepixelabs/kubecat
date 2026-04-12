package ai

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// IsCloudProvider
// ---------------------------------------------------------------------------

func TestIsCloudProvider(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		want     bool
	}{
		{"ollama is local", "ollama", false},
		{"OLLAMA uppercase treated as cloud", "OLLAMA", true},
		{"openai is cloud", "openai", true},
		{"anthropic is cloud", "anthropic", true},
		{"google is cloud", "google", true},
		{"gemini is cloud", "gemini", true},
		{"azure is cloud", "azure", true},
		{"empty string is cloud", "", true},
		{"Ollama mixed case is cloud", "Ollama", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsCloudProvider(tc.provider)
			if got != tc.want {
				t.Errorf("IsCloudProvider(%q) = %v, want %v", tc.provider, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SanitizeForCloud
// ---------------------------------------------------------------------------

func TestSanitizeForCloud_BearerToken(t *testing.T) {
	input := "Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.payload.signature"
	got := SanitizeForCloud(input)

	if strings.Contains(got, "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9") {
		t.Errorf("SanitizeForCloud did not redact bearer token: %q", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("SanitizeForCloud missing [REDACTED] marker: %q", got)
	}
	// Key prefix must survive
	if !strings.Contains(got, "Authorization:") {
		t.Errorf("SanitizeForCloud removed the key prefix: %q", got)
	}
}

func TestSanitizeForCloud_BearerToken_CaseInsensitive(t *testing.T) {
	input := "authorization: bearer supersecrettoken123"
	got := SanitizeForCloud(input)
	if strings.Contains(got, "supersecrettoken123") {
		t.Errorf("SanitizeForCloud did not redact lowercase bearer token")
	}
}

func TestSanitizeForCloud_SecretKeyAssignment(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"password colon", "password: my-secret-password"},
		{"token equals", "TOKEN=supersecret"},
		{"secret colon", "secret: abc123"},
		{"API_KEY assignment", "API_KEY=longkeyvalue"},
		{"CREDENTIAL mixed", "credential: verysecret"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeForCloud(tc.input)
			// The value part must be gone
			parts := strings.SplitN(tc.input, "=", 2)
			if len(parts) == 1 {
				parts = strings.SplitN(tc.input, ":", 2)
			}
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])
				if strings.Contains(got, value) {
					t.Errorf("SanitizeForCloud(%q) still contains value %q: %q", tc.input, value, got)
				}
			}
			if !strings.Contains(got, "[REDACTED]") {
				t.Errorf("SanitizeForCloud(%q) missing [REDACTED]: %q", tc.input, got)
			}
		})
	}
}

func TestSanitizeForCloud_Base64Blob(t *testing.T) {
	// 64+ character base64 string — should be caught
	blob := "dGhpcyBpcyBhIHZlcnkgbG9uZyBzZWNyZXQgdmFsdWUgdGhhdCBzaG91bGQgYmUgcmVkYWN0ZWQ="
	input := "data: " + blob
	got := SanitizeForCloud(input)
	if strings.Contains(got, blob) {
		t.Errorf("SanitizeForCloud did not redact base64 blob")
	}
	if !strings.Contains(got, "[REDACTED-BASE64]") {
		t.Errorf("SanitizeForCloud missing [REDACTED-BASE64] marker: %q", got)
	}
}

func TestSanitizeForCloud_ShortBase64NotRedacted(t *testing.T) {
	// 63 chars — below threshold, must NOT be redacted
	short := strings.Repeat("A", 63)
	got := SanitizeForCloud(short)
	if strings.Contains(got, "[REDACTED-BASE64]") {
		t.Errorf("SanitizeForCloud incorrectly redacted 63-char string: %q", got)
	}
}

func TestSanitizeForCloud_MultilineInput(t *testing.T) {
	input := "Cluster: prod\nAuthorization: Bearer secret123\nVersion: 1.28"
	got := SanitizeForCloud(input)
	if strings.Contains(got, "secret123") {
		t.Errorf("SanitizeForCloud did not redact token in multiline input")
	}
	// Non-sensitive lines must survive
	if !strings.Contains(got, "Cluster: prod") {
		t.Errorf("SanitizeForCloud removed non-sensitive line")
	}
	if !strings.Contains(got, "Version: 1.28") {
		t.Errorf("SanitizeForCloud removed version line")
	}
}

func TestSanitizeForCloud_EmptyInput(t *testing.T) {
	got := SanitizeForCloud("")
	if got != "" {
		t.Errorf("SanitizeForCloud(\"\") = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// SanitizeResourceObject
// ---------------------------------------------------------------------------

func TestSanitizeResourceObject_SecretData(t *testing.T) {
	obj := map[string]interface{}{
		"kind": "Secret",
		"data": map[string]interface{}{
			"password": "c3VwZXJzZWNyZXQ=",
			"apikey":   "YWJjMTIz",
		},
	}
	SanitizeResourceObject(obj)

	data := obj["data"].(map[string]interface{})
	for k, v := range data {
		if v != "[REDACTED]" {
			t.Errorf("Secret .data[%q] = %q, want [REDACTED]", k, v)
		}
	}
}

func TestSanitizeResourceObject_SecretStringData(t *testing.T) {
	obj := map[string]interface{}{
		"kind": "Secret",
		"stringData": map[string]interface{}{
			"username": "admin",
			"password": "hunter2",
		},
	}
	SanitizeResourceObject(obj)

	sd := obj["stringData"].(map[string]interface{})
	for k, v := range sd {
		if v != "[REDACTED]" {
			t.Errorf("Secret .stringData[%q] = %q, want [REDACTED]", k, v)
		}
	}
}

func TestSanitizeResourceObject_SecretKeysPreserved(t *testing.T) {
	obj := map[string]interface{}{
		"kind": "Secret",
		"data": map[string]interface{}{
			"my-key": "sensitivevalue",
		},
	}
	SanitizeResourceObject(obj)

	data := obj["data"].(map[string]interface{})
	if _, exists := data["my-key"]; !exists {
		t.Error("SanitizeResourceObject removed Secret data key, expected key to be preserved")
	}
}

func TestSanitizeResourceObject_NonSecretKindNotRedacted(t *testing.T) {
	obj := map[string]interface{}{
		"kind": "ConfigMap",
		"data": map[string]interface{}{
			"app.conf": "value=harmless",
		},
	}
	SanitizeResourceObject(obj)

	data := obj["data"].(map[string]interface{})
	if data["app.conf"] == "[REDACTED]" {
		t.Error("SanitizeResourceObject incorrectly redacted ConfigMap data")
	}
}

func TestSanitizeResourceObject_EnvVarSensitiveKey(t *testing.T) {
	obj := map[string]interface{}{
		"kind": "Pod",
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"name": "app",
					"env": []interface{}{
						map[string]interface{}{
							"name":  "DB_PASSWORD",
							"value": "hunter2",
						},
						map[string]interface{}{
							"name":  "APP_PORT",
							"value": "8080",
						},
					},
				},
			},
		},
	}
	SanitizeResourceObject(obj)

	spec := obj["spec"].(map[string]interface{})
	containers := spec["containers"].([]interface{})
	env := containers[0].(map[string]interface{})["env"].([]interface{})

	for _, e := range env {
		envVar := e.(map[string]interface{})
		name := envVar["name"].(string)
		val := envVar["value"]

		if name == "DB_PASSWORD" && val != "[REDACTED]" {
			t.Errorf("env DB_PASSWORD not redacted, got %q", val)
		}
		if name == "APP_PORT" && val == "[REDACTED]" {
			t.Errorf("env APP_PORT was incorrectly redacted")
		}
	}
}

func TestSanitizeResourceObject_InitContainersRedacted(t *testing.T) {
	obj := map[string]interface{}{
		"kind": "Pod",
		"spec": map[string]interface{}{
			"initContainers": []interface{}{
				map[string]interface{}{
					"name": "init",
					"env": []interface{}{
						map[string]interface{}{
							"name":  "SECRET_TOKEN",
							"value": "topsecret",
						},
					},
				},
			},
		},
	}
	SanitizeResourceObject(obj)

	spec := obj["spec"].(map[string]interface{})
	inits := spec["initContainers"].([]interface{})
	env := inits[0].(map[string]interface{})["env"].([]interface{})
	envVar := env[0].(map[string]interface{})
	if envVar["value"] != "[REDACTED]" {
		t.Errorf("initContainer SECRET_TOKEN not redacted, got %q", envVar["value"])
	}
}

func TestSanitizeResourceObject_NilInput(t *testing.T) {
	// Should not panic
	SanitizeResourceObject(nil)
}

// ---------------------------------------------------------------------------
// inferResourceTypes (via ContextBuilder with nil manager — only testing type
// inference logic without network calls)
// ---------------------------------------------------------------------------

func newTestContextBuilder() *ContextBuilder {
	return &ContextBuilder{manager: nil, events: nil}
}

func TestInferResourceTypes_PodKeywords(t *testing.T) {
	cb := newTestContextBuilder()
	cases := []struct {
		question string
		wantKind string
	}{
		{"why is my pod crashing?", "pods"},
		{"container OOMKilled", "pods"},
		{"high CPU usage", "pods"},
		{"memory pressure", "pods"},
		{"deployment restart loop", "pods"},
	}
	for _, tc := range cases {
		t.Run(tc.question, func(t *testing.T) {
			types := cb.inferResourceTypes(tc.question, "ollama")
			if !containsStr(types, tc.wantKind) {
				t.Errorf("inferResourceTypes(%q) = %v, want to contain %q", tc.question, types, tc.wantKind)
			}
		})
	}
}

func TestInferResourceTypes_SecretsBlockedForCloudProviders(t *testing.T) {
	cb := newTestContextBuilder()
	cloudProviders := []string{"openai", "anthropic", "google", "gemini"}
	for _, provider := range cloudProviders {
		t.Run(provider, func(t *testing.T) {
			types := cb.inferResourceTypes("show me secrets and credentials", provider)
			if containsStr(types, "secrets") {
				t.Errorf("inferResourceTypes with cloud provider %q returned secrets — must be blocked", provider)
			}
		})
	}
}

func TestInferResourceTypes_SecretsAllowedForOllama(t *testing.T) {
	cb := newTestContextBuilder()
	types := cb.inferResourceTypes("what secrets exist?", "ollama")
	if !containsStr(types, "secrets") {
		t.Errorf("inferResourceTypes with ollama should include secrets, got %v", types)
	}
}

func TestInferResourceTypes_CredentialsBlockedForCloud(t *testing.T) {
	cb := newTestContextBuilder()
	types := cb.inferResourceTypes("list my credentials", "openai")
	if containsStr(types, "secrets") {
		t.Errorf("inferResourceTypes: credentials keyword should not yield secrets for cloud providers")
	}
}

func TestInferResourceTypes_DefaultsToPods(t *testing.T) {
	cb := newTestContextBuilder()
	types := cb.inferResourceTypes("what is the meaning of life?", "openai")
	if !containsStr(types, "pods") {
		t.Errorf("inferResourceTypes with no specific keywords should default to pods, got %v", types)
	}
}

// ---------------------------------------------------------------------------
// BuildPrompt
// ---------------------------------------------------------------------------

func TestBuildPrompt_ContainsClusterName(t *testing.T) {
	qctx := &QueryContext{
		ClusterName:    "prod-us-east-1",
		ClusterVersion: "1.28",
		Question:       "Are any pods failing?",
	}
	prompt := BuildPrompt(qctx)
	if !strings.Contains(prompt, "prod-us-east-1") {
		t.Errorf("BuildPrompt missing cluster name")
	}
	if !strings.Contains(prompt, "1.28") {
		t.Errorf("BuildPrompt missing cluster version")
	}
}

func TestBuildPrompt_ContainsQuestion(t *testing.T) {
	qctx := &QueryContext{
		Question: "Why is the nginx pod crashing?",
	}
	prompt := BuildPrompt(qctx)
	if !strings.Contains(prompt, qctx.Question) {
		t.Errorf("BuildPrompt does not contain user question")
	}
}

func TestBuildPrompt_ContainsResourceContext(t *testing.T) {
	qctx := &QueryContext{
		Question: "check pods",
		Resources: []ResourceContext{
			{Kind: "Pod", Name: "nginx-abc", Namespace: "default", Status: "Running", Age: "2d"},
		},
	}
	prompt := BuildPrompt(qctx)
	if !strings.Contains(prompt, "nginx-abc") {
		t.Errorf("BuildPrompt missing resource name")
	}
}

func TestBuildPrompt_ContainsEventContext(t *testing.T) {
	qctx := &QueryContext{
		Question: "what happened?",
		Events: []EventContext{
			{Kind: "Pod", Name: "crashed-pod", Type: "Warning", Reason: "OOMKilled", Message: "Out of memory", Time: time.Now()},
		},
	}
	prompt := BuildPrompt(qctx)
	if !strings.Contains(prompt, "OOMKilled") {
		t.Errorf("BuildPrompt missing event reason")
	}
}

func TestBuildPrompt_HTMLStructureHint(t *testing.T) {
	qctx := &QueryContext{Question: "test"}
	prompt := BuildPrompt(qctx)
	if !strings.Contains(prompt, "ai-summary") {
		t.Errorf("BuildPrompt missing ai-summary HTML structure hint")
	}
}

// ---------------------------------------------------------------------------
// formatAge
// ---------------------------------------------------------------------------

func TestFormatAge(t *testing.T) {
	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "0s"},
		{"30 seconds", 30 * time.Second, "30s"},
		{"59 seconds", 59 * time.Second, "59s"},
		{"1 minute", 60 * time.Second, "1m"},
		{"90 minutes", 90 * time.Minute, "1h"},
		{"2 hours", 2 * time.Hour, "2h"},
		{"23 hours", 23 * time.Hour, "23h"},
		{"1 day", 24 * time.Hour, "1d"},
		{"6 days", 6 * 24 * time.Hour, "6d"},
		{"7 days", 7 * 24 * time.Hour, "1w"},
		{"14 days", 14 * 24 * time.Hour, "2w"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatAge(tc.d)
			if got != tc.want {
				t.Errorf("formatAge(%v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func containsStr(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
