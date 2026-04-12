# Installation

## Requirements

- Go 1.23 or later (for building from source)
- Node.js 20+ and npm (for building from source)
- kubectl configured with cluster access

## From Source

```bash
# Clone the repository
git clone https://github.com/thepixelabs/kubecat.git
cd kubecat

# Build
wails build

# The binary will be in build/bin/
```

## Using Go Install

```bash
go install github.com/thepixelabs/kubecat@latest
```

## Homebrew (planned)

```bash
brew install kubecat/tap/kubecat
```

## Binary Releases (planned)

Download from [GitHub Releases](https://github.com/thepixelabs/kubecat/releases):

```bash
# Linux
curl -LO https://github.com/thepixelabs/kubecat/releases/latest/download/kubecat_linux_amd64.tar.gz
tar xzf kubecat_linux_amd64.tar.gz
sudo mv kubecat /usr/local/bin/

# macOS
curl -LO https://github.com/thepixelabs/kubecat/releases/latest/download/kubecat_darwin_arm64.tar.gz
tar xzf kubecat_darwin_arm64.tar.gz
sudo mv kubecat /usr/local/bin/

# Windows
# Download kubecat_windows_amd64.zip and add to PATH
```

## Docker (planned)

```bash
docker run --rm -it \
  -v ~/.kube/config:/root/.kube/config:ro \
  ghcr.io/thepixelabs/kubecat:latest
```

## Verifying Installation

```bash
kubecat --version

# Output:
# Kubecat - Next-Gen Kubernetes Terminal Command Center
#
# Version:    0.1.0
# Git Commit: abc1234
# Build Date: 2024-01-15T10:30:00Z
# Go Version: go1.23.0
# OS/Arch:    darwin/arm64
```

## Shell Completions (planned)

```bash
# Bash
kubecat completion bash > /etc/bash_completion.d/kubecat

# Zsh
kubecat completion zsh > "${fpath[1]}/_kubecat"

# Fish
kubecat completion fish > ~/.config/fish/completions/kubecat.fish
```

## Upgrading

```bash
# From source
git pull
wails build

# Using Go
go install github.com/thepixelabs/kubecat@latest
```

## Uninstalling

```bash
# Remove binary
rm $(which kubecat)

# Remove configuration (optional)
rm -rf ~/.config/kubecat
rm -rf ~/.local/share/kubecat
rm -rf ~/.local/state/kubecat
rm -rf ~/.cache/kubecat
```
