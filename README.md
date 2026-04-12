# Kubecat

A Kubernetes management desktop application built for engineers who want an intelligent cockpit view of their clusters.

Built with [Wails v2](https://wails.io/) (Go backend + WebKit frontend) and React.

---

## Features

- **Resource Explorer** — browse, filter, and manage any Kubernetes resource
- **Cluster Visualizer** — interactive graph showing ownership relationships and service routing
- **AI Queries** — ask questions about your cluster in plain English (Ollama, OpenAI, Anthropic, Google)
- **Security Scanning** — RBAC analysis, runtime security, network policy coverage
- **Time Travel** — periodic snapshots with event timeline for historical debugging
- **Cluster Diff** — compare two clusters or two points in time
- **GitOps Integration** — ArgoCD and Flux sync status
- **Terminal** — embedded shell with PTY support
- **Port Forwarding** — one-click pod port forwarding to localhost

## Privacy First

Kubecat is local-first. All data stays on your machine. No telemetry, no cloud sync, no backend server. AI queries only transmit the specific resource data you ask about, and only to the provider you configure. Run Ollama for fully air-gapped operation.

---

## Installation

### Homebrew

```bash
brew install --cask thepixelabs/thepixelabs/kubecat
```

### Manual

Download the latest `.dmg` from [GitHub Releases](https://github.com/thepixelabs/kubecat/releases).

---

## Prerequisites

- macOS 12 Monterey or later
- A valid `~/.kube/config` with at least one cluster context
- For AI features: [Ollama](https://ollama.ai) (local) or API key for a cloud provider

---

## Building from Source

### Requirements

- Go 1.24+
- Node.js 20+
- Wails CLI v2.11.0: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0`
- macOS: Xcode Command Line Tools (`xcode-select --install`)

### Development Mode

```bash
git clone https://github.com/thepixelabs/kubecat.git
cd kubecat
go mod download
cd frontend && npm ci && cd ..
wails dev
```

### Production Build

```bash
wails build -platform darwin/universal
```

Output: `build/bin/kubecat.app`

See [docs/development/setup.md](docs/development/setup.md) for the complete development guide.

---

## Architecture

```
kubecat/
├── main.go              # Wails application entry point
├── app.go               # Wails bridge — exposes Go functions to frontend
├── internal/
│   ├── ai/              # AI provider implementations (OpenAI, Anthropic, Google, Ollama)
│   ├── client/          # Kubernetes API client wrapper
│   ├── config/          # YAML configuration management
│   ├── core/            # Core Kubernetes service layer
│   ├── history/         # Event collection and cluster snapshotting
│   ├── security/        # RBAC, runtime, and network policy scanning
│   ├── storage/         # SQLite persistence layer
│   ├── terminal/        # PTY terminal manager
│   └── version/         # Build version info (injected at build time)
└── frontend/
    └── src/
        ├── components/  # React components
        ├── stores/      # Zustand state stores
        └── hooks/       # Custom React hooks
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.24, Wails v2 |
| Frontend | React 18, TypeScript, Tailwind CSS |
| State | Zustand, TanStack Query |
| Charts | @xyflow/react, elkjs |
| Terminal | xterm.js |
| Database | SQLite (modernc.org/sqlite, pure Go) |
| AI | OpenAI, Anthropic, Google Gemini, Ollama |

---

## Documentation

| Document | Description |
|----------|-------------|
| [User Guide](docs/user-guide/README.md) | End-user documentation |
| [Development Setup](docs/development/setup.md) | Zero-to-running guide for contributors |
| [Security Hardening](docs/security/hardening-checklist.md) | Security controls and verification |
| [RBAC Manifests](docs/rbac/README.md) | Kubernetes RBAC for service accounts |
| [Operations](docs/operations/) | Observability, data retention, troubleshooting |
| [Changelog](docs/CHANGELOG.md) | Release history |
| [Privacy Policy](PRIVACY.md) | Data handling and AI transmission details |

---

## License

See [LICENSE](LICENSE).
