# AI Integration

> **Status**: ✅ Implemented

Kubecat integrates AI capabilities for natural language queries, smart troubleshooting, and predictive insights.

## Vision

Ask questions in plain English and get actionable answers:

```
> Show me pods using more than 80% memory

Found 3 pods with high memory usage:

NAMESPACE    NAME              MEMORY    LIMIT     %
default      api-server-xyz    1.8Gi     2Gi      90%
web          cache-redis       950Mi     1Gi      95%
monitoring   prometheus-0      3.8Gi     4Gi      95%

Recommendation: Consider increasing memory limits or scaling horizontally.
```

## Features

### Natural Language Queries

Query your cluster using natural language:

```
> Why is my deployment failing?
> Show pods that restarted in the last hour
> What changed since yesterday?
> Find services without endpoints
> List pods scheduled on node-1
```

### Smart Troubleshooting

AI-assisted root cause analysis:

```
> Why is pod api-server-abc123 in CrashLoopBackOff?

Analyzing pod events and logs...

Root Cause Analysis:
1. Container exited with code 1
2. Last log: "Error: REDIS_URL environment variable not set"
3. ConfigMap 'api-config' missing REDIS_URL key

Suggested Fix:
Add REDIS_URL to configmap 'api-config':

  kubectl patch configmap api-config -p '{"data":{"REDIS_URL":"redis://..."}}'
```

### Anomaly Detection

Automatic detection of unusual patterns:

```
⚠ Anomaly Detected

Pod restart rate increased 500% in namespace 'production'
Affected pods: api-server-*, worker-*
Started: 10 minutes ago
Correlated event: ConfigMap 'app-config' updated
```

### Predictive Insights

```
📊 Resource Prediction

Based on current trends, namespace 'default' will exceed
memory limits in approximately 2 hours.

Current: 75% of quota
Trend: +5% per hour
Projected: 100% at 14:30 UTC

Recommendation: Scale api-server deployment or increase quota.
```

## AI Backends (All Implemented ✅)

### Local (Ollama)

Privacy-focused, runs entirely on your machine:

```yaml
# ~/.config/kubecat/config.yaml
kubecat:
  ai:
    enabled: true
    provider: ollama
    model: llama3.2
    endpoint: http://localhost:11434
```

### Cloud (OpenAI)

More powerful models, requires API key:

```yaml
kubecat:
  ai:
    enabled: true
    provider: openai
    model: gpt-4
    apiKey: ${OPENAI_API_KEY}
```

### Cloud (Anthropic)

Alternative cloud provider:

```yaml
kubecat:
  ai:
    enabled: true
    provider: anthropic
    model: claude-3-sonnet
    apiKey: ${ANTHROPIC_API_KEY}
```

## Architecture

### Query Flow

```
┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
│   User   │     │  Query   │     │    AI    │     │   K8s    │
│  Input   │────▶│  Parser  │────▶│ Backend  │────▶│  Client  │
└──────────┘     └──────────┘     └──────────┘     └──────────┘
                                        │                │
                                        ▼                ▼
                                  ┌──────────┐     ┌──────────┐
                                  │ Generate │◀────│ Resource │
                                  │ Response │     │   Data   │
                                  └──────────┘     └──────────┘
```

### Context Injection

The AI receives context about:
- Current cluster state
- Recent events
- Resource specifications
- Historical patterns

### Prompt Templates

```go
type QueryContext struct {
    Cluster     ClusterInfo
    Namespace   string
    Resources   []Resource
    Events      []Event
    Question    string
}

func BuildPrompt(ctx QueryContext) string {
    return fmt.Sprintf(`
You are a Kubernetes expert assistant.

Cluster: %s
Namespace: %s
Resources: %d pods, %d deployments, %d services

Recent Events:
%s

User Question: %s

Provide a helpful, actionable response.
`, ctx.Cluster.Name, ctx.Namespace, ...)
}
```

## Privacy Considerations

### Local-First
Ollama runs entirely locally - no data leaves your machine.

### Data Sanitization
Before sending to cloud providers:
- Secrets are never included
- Sensitive annotations are filtered
- Only metadata is shared, not full specs

### Opt-In
AI features are disabled by default:

```yaml
kubecat:
  ai:
    enabled: false  # Default
```

