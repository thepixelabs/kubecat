# Configuration Reference

Complete reference for all Kubecat configuration options.

## Configuration File

Default location: `~/.config/kubecat/config.yaml`

Override with: `KUBECAT_CONFIG_DIR=/path/to/dir`

## Root Options

```yaml
kubecat:
  # All options go under the 'kubecat' key
```

## Core Options

### refreshRate

UI refresh interval in seconds.

| Type | Default | Min | Max |
|------|---------|-----|-----|
| int | 2 | 2 | 60 |

```yaml
kubecat:
  refreshRate: 2
```

### theme

Active theme name. Must match a file in themes directory.

| Type | Default |
|------|---------|
| string | "default" |

```yaml
kubecat:
  theme: dracula
```

Built-in themes: `default`, `dracula`

### readOnly

Disable all modification operations (delete, edit, scale).

| Type | Default |
|------|---------|
| bool | false |

```yaml
kubecat:
  readOnly: true
```

## UI Options

```yaml
kubecat:
  ui:
    enableMouse: false
    headless: false
    noIcons: false
```

### ui.enableMouse

Enable mouse support for clicking and scrolling.

| Type | Default |
|------|---------|
| bool | false |

### ui.headless

Hide the header bar (logo, cluster name, view indicator).

| Type | Default |
|------|---------|
| bool | false |

### ui.noIcons

Disable Unicode icons for terminals without support.

| Type | Default |
|------|---------|
| bool | false |

## AI Options

```yaml
kubecat:
  ai:
    enabled: false
    provider: ollama
    model: llama3.2
    endpoint: http://localhost:11434
    apiKey: ""
```

### ai.enabled

Enable AI-powered features.

| Type | Default |
|------|---------|
| bool | false |

### ai.provider

AI backend provider.

| Type | Default | Options |
|------|---------|---------|
| string | "ollama" | ollama, openai, anthropic |

### ai.model

Model name to use.

| Type | Default |
|------|---------|
| string | "llama3.2" |

Common models:
- Ollama: `llama3.2`, `mistral`, `codellama`
- OpenAI: `gpt-4`, `gpt-3.5-turbo`
- Anthropic: `claude-3-sonnet`, `claude-3-haiku`

### ai.endpoint

API endpoint URL.

| Type | Default |
|------|---------|
| string | "http://localhost:11434" |

### ai.apiKey

API key for cloud providers. Supports environment variable expansion.

| Type | Default |
|------|---------|
| string | "" |

```yaml
kubecat:
  ai:
    apiKey: ${OPENAI_API_KEY}
```

## Cluster Options

```yaml
kubecat:
  # Optional: last active cluster context used by Kubecat (separate from kubeconfig's current-context)
  activeContext: prod-cluster
  clusters:
    - name: production
      context: prod-cluster
      namespace: default
      readOnly: true
```

### activeContext

Last active kubeconfig context used by Kubecat. Kubecat uses this to restore your last selected cluster on startup.

| Type | Default |
|------|---------|
| string | "" |

### clusters[].name

Friendly display name for the cluster.

| Type | Required |
|------|----------|
| string | Yes |

### clusters[].context

Kubeconfig context name.

| Type | Required |
|------|----------|
| string | Yes |

### clusters[].namespace

Default namespace for this cluster.

| Type | Default |
|------|---------|
| string | "" (all namespaces) |

### clusters[].readOnly

Force read-only mode for this cluster.

| Type | Default |
|------|---------|
| bool | false |

## Logger Options

```yaml
kubecat:
  logger:
    tail: 100
    buffer: 5000
    showTime: false
    textWrap: false
```

### logger.tail

Number of log lines to fetch initially.

| Type | Default | Range |
|------|---------|-------|
| int | 100 | 1-10000 |

### logger.buffer

Maximum number of log lines to keep in buffer.

| Type | Default | Range |
|------|---------|-------|
| int | 5000 | 100-100000 |

### logger.showTime

Display timestamps in log output.

| Type | Default |
|------|---------|
| bool | false |

### logger.textWrap

Wrap long log lines instead of truncating.

| Type | Default |
|------|---------|
| bool | false |

## History Options (Planned)

```yaml
kubecat:
  history:
    enabled: true
    snapshotInterval: 5m
    retention: 7d
    maxSize: 500MB
```

### history.enabled

Enable time-travel debugging features.

| Type | Default |
|------|---------|
| bool | true |

### history.snapshotInterval

How often to take state snapshots.

| Type | Default | Format |
|------|---------|--------|
| duration | "5m" | Go duration |

### history.retention

How long to keep historical data.

| Type | Default | Format |
|------|---------|--------|
| duration | "7d" | Go duration |

### history.maxSize

Maximum database size before pruning.

| Type | Default |
|------|---------|
| string | "500MB" |

## Environment Variables

All options can be set via environment variables:

| Config Path | Environment Variable |
|-------------|---------------------|
| `kubecat.refreshRate` | `KUBECAT_REFRESH_RATE` |
| `kubecat.theme` | `KUBECAT_THEME` |
| `kubecat.readOnly` | `KUBECAT_READ_ONLY` |
| `kubecat.ui.enableMouse` | `KUBECAT_UI_ENABLE_MOUSE` |
| `kubecat.ai.enabled` | `KUBECAT_AI_ENABLED` |
| `kubecat.ai.provider` | `KUBECAT_AI_PROVIDER` |
| `kubecat.ai.apiKey` | `KUBECAT_AI_API_KEY` |

Environment variables take precedence over config file values.

## Example Configurations

### Minimal

```yaml
kubecat:
  theme: default
```

### Production Read-Only

```yaml
kubecat:
  readOnly: true
  clusters:
    - name: production
      context: prod-cluster
      readOnly: true
```

### AI-Enabled (Local)

```yaml
kubecat:
  ai:
    enabled: true
    provider: ollama
    model: llama3.2
```

### AI-Enabled (Cloud)

```yaml
kubecat:
  ai:
    enabled: true
    provider: openai
    model: gpt-4
    apiKey: ${OPENAI_API_KEY}
```

### Power User

```yaml
kubecat:
  refreshRate: 1
  theme: dracula
  ui:
    enableMouse: true
  logger:
    tail: 500
    buffer: 10000
    showTime: true
  history:
    enabled: true
    retention: 30d
```
