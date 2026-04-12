# Configuration

Kubecat uses XDG Base Directory specification for configuration files.

## Configuration Locations

| Platform | Config Path |
|----------|-------------|
| Linux | `~/.config/kubecat/config.yaml` |
| macOS | `~/Library/Application Support/kubecat/config.yaml` |
| Windows | `%LOCALAPPDATA%\kubecat\config.yaml` |

Override with environment variable:
```bash
export KUBECAT_CONFIG_DIR=/custom/path
```

## Full Configuration Example

```yaml
# ~/.config/kubecat/config.yaml
kubecat:
  # UI refresh interval in seconds (minimum 2)
  refreshRate: 2
  
  # Active theme name
  theme: default
  
  # Disable all write operations
  readOnly: false
  
  # User interface settings
  ui:
    # Enable mouse support
    enableMouse: false
    # Hide the header
    headless: false
    # Disable icons (for terminals without Unicode)
    noIcons: false
  
  # AI/LLM configuration
  ai:
    enabled: false
    provider: ollama  # ollama, openai, anthropic
    model: llama3.2
    endpoint: http://localhost:11434
    # apiKey: ${OPENAI_API_KEY}  # For cloud providers
  
  # Pre-configured clusters
  clusters:
    - name: production
      context: prod-cluster
      namespace: default
      readOnly: true
    - name: staging
      context: staging-cluster
  
  # Log viewer settings
  logger:
    # Lines to fetch initially
    tail: 100
    # Maximum buffer size
    buffer: 5000
    # Show timestamps
    showTime: false
    # Wrap long lines
    textWrap: false
```

## Configuration Options

### Core Settings

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `refreshRate` | int | 2 | UI refresh interval (seconds) |
| `theme` | string | "default" | Active theme name |
| `readOnly` | bool | false | Disable all modifications |

### UI Settings

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `ui.enableMouse` | bool | false | Enable mouse support |
| `ui.headless` | bool | false | Hide the header |
| `ui.noIcons` | bool | false | Disable Unicode icons |

### AI Settings

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `ai.enabled` | bool | false | Enable AI features |
| `ai.provider` | string | "ollama" | AI backend |
| `ai.model` | string | "llama3.2" | Model name |
| `ai.endpoint` | string | "http://localhost:11434" | API endpoint |
| `ai.apiKey` | string | "" | API key for cloud providers |

### Logger Settings

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `logger.tail` | int | 100 | Initial log lines |
| `logger.buffer` | int | 5000 | Max buffer size |
| `logger.showTime` | bool | false | Show timestamps |
| `logger.textWrap` | bool | false | Wrap long lines |

## Environment Variables

Override any setting with environment variables:

```bash
# General
export KUBECAT_READ_ONLY=true
export KUBECAT_THEME=dracula

# AI
export KUBECAT_AI_ENABLED=true
export KUBECAT_AI_PROVIDER=openai
export OPENAI_API_KEY=sk-...
```

## Data Directories

Kubecat stores data in XDG-compliant locations:

| Purpose | Path |
|---------|------|
| Config | `~/.config/kubecat/` |
| Data | `~/.local/share/kubecat/` |
| State | `~/.local/state/kubecat/` |
| Cache | `~/.cache/kubecat/` |

### Data Directory Contents

```
~/.local/share/kubecat/
├── clusters/           # Per-cluster settings
│   └── prod-cluster/
│       └── config.yaml
├── themes/             # Custom themes
│   └── my-theme.yaml
└── plugins/            # Installed plugins
```

### State Directory Contents

```
~/.local/state/kubecat/
├── kubecat.log        # Application logs
├── history.db         # Event history (SQLite)
└── screen-dumps/      # Saved screenshots
```

## Viewing Active Configuration

```bash
kubecat info

# Output:
# Kubecat - Next-Gen Kubernetes Terminal Command Center
#
# Version:     0.1.0
# Config:      ~/.config/kubecat/config.yaml
# Data:        ~/.local/share/kubecat
# State:       ~/.local/state/kubecat
# Cache:       ~/.cache/kubecat
# Theme:       default
# AI:          disabled
```

## Resetting Configuration

```bash
# Backup and reset
mv ~/.config/kubecat ~/.config/kubecat.bak
kubecat  # Will create default config
```
