# Configuration Reference

Kubecat is configured via a YAML file at `~/.config/kubecat/config.yaml`.

The file is created with defaults on first launch. All settings are optional — omit any key to use the default.

---

## Full Example

```yaml
kubecat:
  refreshRate: 2
  theme: default
  readOnly: false
  checkForUpdates: false

  ui:
    enableMouse: false
    headless: false
    noIcons: false

  ai:
    enabled: true
    selectedProvider: ollama
    selectedModel: llama3.2
    providers:
      ollama:
        enabled: true
        endpoint: "http://localhost:11434"
        models:
          - llama3.2
          - llama3.1:8b
      openai:
        enabled: false
        apiKey: "sk-..."
        models:
          - gpt-4o
          - gpt-4o-mini
      anthropic:
        enabled: false
        apiKey: "sk-ant-..."
        models:
          - claude-3-5-sonnet-20241022
          - claude-3-haiku-20240307
      google:
        enabled: false
        apiKey: "AIza..."
        models:
          - gemini-2.0-flash
          - gemini-1.5-pro

  activeContext: my-cluster

  clusters:
    - context: my-cluster
      name: "Production"
      namespace: default
      readOnly: true
    - context: dev-local
      name: "Local Dev"
      namespace: default
      readOnly: false

  logger:
    tail: 100
    buffer: 5000
    showTime: false
    textWrap: false
    logLevel: info

  retention:
    retentionDays: 7
    maxDatabaseSizeMB: 500
```

---

## Setting Reference

### Top-level `kubecat`

| Key | Default | Description |
|-----|---------|-------------|
| `refreshRate` | `2` | UI polling interval in seconds |
| `theme` | `default` | UI theme name |
| `readOnly` | `false` | Global read-only mode — blocks all cluster mutations |
| `checkForUpdates` | `false` | Opt-in: check GitHub Releases daily for new Kubecat versions |
| `activeContext` | `""` | Last active kubeconfig context (auto-set by UI) |

### `ui`

| Key | Default | Description |
|-----|---------|-------------|
| `enableMouse` | `false` | Enable mouse click support |
| `headless` | `false` | Hide the header bar |
| `noIcons` | `false` | Disable icons in the UI |

### `ai`

| Key | Default | Description |
|-----|---------|-------------|
| `enabled` | `false` | Enable AI features globally |
| `selectedProvider` | `ollama` | Active provider ID |
| `selectedModel` | `llama3.2` | Active model name |
| `providers.<id>.enabled` | `false` | Enable this specific provider |
| `providers.<id>.apiKey` | `""` | API key (written `0600`, never logged) |
| `providers.<id>.endpoint` | provider default | Custom API endpoint (Ollama, self-hosted) |
| `providers.<id>.models` | `[]` | List of models to show in dropdown |

### `clusters[]`

| Key | Default | Description |
|-----|---------|-------------|
| `context` | required | kubeconfig context name |
| `name` | context name | Friendly display name |
| `namespace` | `default` | Default namespace for this cluster |
| `readOnly` | `false` | Read-only mode for this cluster only |

### `logger`

| Key | Default | Description |
|-----|---------|-------------|
| `tail` | `100` | Log lines to display in the log viewer |
| `buffer` | `5000` | Maximum in-memory log buffer size |
| `showTime` | `false` | Show timestamps in log view |
| `textWrap` | `false` | Wrap long log lines |
| `logLevel` | `info` | Application log level (`debug`/`info`/`warn`/`error`) |

### `retention`

| Key | Default | Description |
|-----|---------|-------------|
| `retentionDays` | `7` | Days to keep events and snapshots |
| `maxDatabaseSizeMB` | `500` | Warn and aggressively prune if database exceeds this |

---

## Environment Variables

| Variable | Effect |
|----------|--------|
| `KUBECAT_CONFIG_DIR` | Override the config directory (default: `~/.config/kubecat`) |
| `KUBECONFIG` | kubeconfig file path (standard kubectl convention) |
| `XDG_CONFIG_HOME` | XDG base directory for config (overridden by `KUBECAT_CONFIG_DIR`) |
| `XDG_DATA_HOME` | XDG base directory for app data |
| `XDG_STATE_HOME` | XDG base directory for logs and database |
| `XDG_CACHE_HOME` | XDG base directory for cache |
