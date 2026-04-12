# Themes

Kubecat supports customizable themes to match your terminal aesthetic.

## Built-in Themes

### Default

Deep space aesthetic with violet accents.

```yaml
colors:
  primary:     "#7C3AED"  # Violet
  secondary:   "#06B6D4"  # Cyan
  background:  "#0F0F23"  # Deep space
  foreground:  "#E2E8F0"  # Light gray
  success:     "#10B981"  # Emerald
  warning:     "#F59E0B"  # Amber
  error:       "#EF4444"  # Red
```

### Dracula

Classic Dracula color scheme.

```yaml
colors:
  primary:     "#BD93F9"  # Purple
  secondary:   "#8BE9FD"  # Cyan
  background:  "#282A36"  # Background
  foreground:  "#F8F8F2"  # Foreground
  success:     "#50FA7B"  # Green
  warning:     "#FFB86C"  # Orange
  error:       "#FF5555"  # Red
```

## Switching Themes

### Via Configuration

```yaml
# ~/.config/kubecat/config.yaml
kubecat:
  theme: dracula
```

### Via Environment Variable

```bash
export KUBECAT_THEME=dracula
kubecat
```

## Creating Custom Themes

Create a theme file in your themes directory:

```yaml
# ~/.local/share/kubecat/themes/my-theme.yaml
name: my-theme

colors:
  # Primary accent color
  primary: "#FF6B6B"
  
  # Secondary accent color
  secondary: "#4ECDC4"
  
  # Main background
  background: "#1A1A2E"
  
  # Main text color
  foreground: "#EAEAEA"
  
  # Subdued text
  muted: "#6B7280"
  
  # Status colors
  success: "#10B981"
  warning: "#FBBF24"
  error: "#EF4444"
  info: "#3B82F6"
  
  # Border colors
  border: "#374151"
  borderFocus: "#FF6B6B"
```

Then activate it:

```yaml
# ~/.config/kubecat/config.yaml
kubecat:
  theme: my-theme
```

## Color Reference

### Core Colors

| Color | Usage |
|-------|-------|
| `primary` | Main accent, selected items, active elements |
| `secondary` | Secondary accent, labels, highlights |
| `background` | Main background color |
| `foreground` | Main text color |
| `muted` | Subdued text, hints, disabled items |

### Status Colors

| Color | Usage |
|-------|-------|
| `success` | Running, Active, Healthy, Bound |
| `warning` | Pending, Warning, ContainerCreating |
| `error` | Failed, Error, CrashLoopBackOff |
| `info` | Informational highlights |

### Border Colors

| Color | Usage |
|-------|-------|
| `border` | Default borders and separators |
| `borderFocus` | Focused element borders |

## Color Formats

Supported formats:

```yaml
# Hex (recommended)
primary: "#FF6B6B"
primary: "#F66"  # Short form

# Named colors (limited support)
primary: "red"
primary: "dodgerblue"

# RGB (planned)
primary: "rgb(255, 107, 107)"

# Special values
background: "default"  # Use terminal default
```

## Style Reference

Beyond colors, themes can define text styles:

```yaml
# ~/.local/share/kubecat/themes/my-theme.yaml
styles:
  title:
    bold: true
    underline: false
    italic: false
  
  subtitle:
    italic: true
  
  selected:
    bold: true
    reverse: true  # Swap fg/bg
  
  key:
    bold: true
```

## Terminal Compatibility

### 256 Color Terminals

Most modern terminals support 256 colors:

```bash
export TERM=xterm-256color
```

### True Color (24-bit)

For full hex color support:

```bash
# Check support
echo $COLORTERM
# Should be "truecolor" or "24bit"
```

### Fallback Colors

If your terminal has limited color support, Kubecat will approximate colors to the nearest available.

## Theme Gallery (Community)

Share your themes with the community:

1. Create your theme file
2. Test thoroughly in your terminal
3. Submit a PR to the themes repository
4. Include a screenshot

### Submitting a Theme

```bash
# Fork and clone the themes repo
git clone https://github.com/thepixelabs/kubecat-themes.git

# Add your theme
cp my-theme.yaml themes/

# Add a screenshot
# themes/screenshots/my-theme.png

# Submit PR
```

