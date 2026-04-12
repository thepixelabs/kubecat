# Keyboard Shortcuts

Complete reference of all keyboard shortcuts in Kubecat.

## Global Shortcuts

Available from any view:

| Key | Action |
|-----|--------|
| `c` | Open cluster configuration |
| `1` | Switch to Dashboard |
| `2` | Switch to Explorer |
| `3` | Switch to Timeline |
| `4` | Switch to GitOps |
| `5` | Switch to Security |
| `/` | Open Query/Search |
| `?` | Toggle help |
| `q` | Quit Kubecat |
| `Ctrl+C` | Quit Kubecat |

## Navigation

Vim-style navigation available in lists and tables:

| Key | Action |
|-----|--------|
| `j` or `↓` | Move down one row |
| `k` or `↑` | Move up one row |
| `h` or `←` | Move left / Previous |
| `l` or `→` | Move right / Next |
| `g` | Go to first item |
| `G` | Go to last item |
| `Ctrl+D` or `PgDn` | Page down |
| `Ctrl+U` or `PgUp` | Page up |
| `Enter` | Select / Confirm |
| `Esc` | Back / Cancel |

## Cluster Configuration View

| Key | Action |
|-----|--------|
| `↑/↓` | Navigate contexts |
| `Enter` | Connect to cluster |
| `a` | Set as active cluster |
| `d` | Disconnect from cluster |
| `/` | Filter contexts |
| `Esc` | Return to previous view |

## Resource Explorer View

| Key | Action |
|-----|--------|
| `←/→` | Switch resource type |
| `Tab` | Cycle namespace |
| `/` | Open filter |
| `r` | Refresh resources |
| `Enter` | View details (planned) |
| `d` | Describe resource (planned) |
| `e` | Edit resource (planned) |
| `l` | View logs (planned) |
| `s` | Shell into pod (planned) |
| `f` | Port forward (planned) |
| `y` | Copy YAML (planned) |
| `Ctrl+D` | Delete resource (planned) |

## Filter/Search Input

When filter is active:

| Key | Action |
|-----|--------|
| Type | Add to filter |
| `Backspace` | Delete character |
| `Ctrl+W` | Delete word |
| `Ctrl+U` | Clear filter |
| `Enter` | Apply filter |
| `Esc` | Cancel filter |

## Log Viewer (Planned)

| Key | Action |
|-----|--------|
| `f` | Toggle follow mode |
| `w` | Toggle line wrap |
| `t` | Toggle timestamps |
| `/` | Search in logs |
| `n` | Next match |
| `N` | Previous match |
| `g` | Go to start |
| `G` | Go to end |
| `y` | Copy selection |
| `Esc` | Exit log viewer |

## YAML Viewer (Planned)

| Key | Action |
|-----|--------|
| `e` | Edit in $EDITOR |
| `y` | Copy to clipboard |
| `/` | Search |
| `n` | Next match |
| `N` | Previous match |
| `j/k` | Scroll |
| `Esc` | Close viewer |

## Timeline View (Planned)

| Key | Action |
|-----|--------|
| `←/→` | Navigate time |
| `[` | Go back 1 hour |
| `]` | Go forward 1 hour |
| `t` | Jump to time |
| `n` | Go to now |
| `/` | Filter events |
| `Enter` | View event details |

## Customizing Keybindings (Planned)

```yaml
# ~/.config/kubecat/keybindings.yaml
keybindings:
  # Override defaults
  quit: "ctrl+q"
  explorer: "e"
  
  # Add custom bindings
  custom:
    - key: "ctrl+p"
      command: "pods"
    - key: "ctrl+d"
      command: "deployments"
```

## Key Notation

| Notation | Meaning |
|----------|---------|
| `a-z` | Lowercase letter |
| `A-Z` | Uppercase letter (Shift + letter) |
| `Ctrl+X` | Control + X |
| `Alt+X` | Alt/Option + X |
| `↑↓←→` | Arrow keys |
| `Enter` | Enter/Return key |
| `Esc` | Escape key |
| `Tab` | Tab key |
| `PgUp/PgDn` | Page Up/Down |
| `Home/End` | Home/End keys |

