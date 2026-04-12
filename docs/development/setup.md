# Development Setup

This guide gets you from zero to a running Kubecat development environment.

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.24+ | https://go.dev/dl/ |
| Node.js | 20+ | https://nodejs.org or `brew install node@20` |
| Wails CLI | v2.11.0 | `go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0` |
| golangci-lint | v1.62+ | See below |
| lefthook | latest | `go install github.com/evilmartians/lefthook@latest` |

### macOS-specific

```bash
# Xcode Command Line Tools (required for WebKit)
xcode-select --install
```

### Install golangci-lint

```bash
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
  | sh -s -- -b $(go env GOPATH)/bin v1.62.2
```

---

## Clone and Bootstrap

```bash
git clone https://github.com/thepixelabs/kubecat.git
cd kubecat

# Install Go dependencies
go mod download

# Install frontend dependencies
cd frontend && npm ci && cd ..

# Install git hooks (pre-commit lint, pre-push tests)
lefthook install
```

---

## Development Mode

```bash
wails dev
```

This starts:
1. The Go backend with hot-reload (powered by `air`)
2. The Vite dev server on `localhost:5173` with HMR (Hot Module Replacement)
3. A Wails window connected to the dev server

Changes to Go files trigger a backend restart. Changes to `.tsx`/`.ts` files trigger instant HMR in the window — no restart needed.

The frontend can also be developed in a browser at `http://localhost:5173`. Wails runtime calls will fail in the browser, but component layout/styling work normally.

---

## Building for Production

```bash
# Standard build (current platform)
wails build

# macOS universal binary (Intel + Apple Silicon)
wails build -platform darwin/universal

# With version injection
wails build -platform darwin/universal \
  -ldflags "-X github.com/thepixelabs/kubecat/internal/version.Version=v1.0.0 \
             -X github.com/thepixelabs/kubecat/internal/version.GitCommit=$(git rev-parse HEAD) \
             -X github.com/thepixelabs/kubecat/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
```

Output: `build/bin/kubecat.app` (macOS)

---

## Running Tests

### Go tests

```bash
# All tests with race detector
go test -race ./...

# Specific package
go test -race ./internal/storage/...

# With coverage
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Frontend tests

```bash
cd frontend

# Run once
npm run test

# Watch mode
npm run test:watch

# With coverage report
npm run test:coverage
```

---

## Linting

```bash
# Go lint
golangci-lint run

# Go lint (fast — staged files only, for pre-commit)
golangci-lint run --fast

# Frontend lint
cd frontend && npx eslint . --max-warnings=0
```

The pre-commit hook (installed by `lefthook install`) runs `gofmt` and `golangci-lint --fast` automatically.

---

## Project Structure

```
kubecat/
├── main.go                   # Wails app entry point
├── app.go                    # Wails bridge (will be split into app_*.go files)
├── app_cluster.go            # Cluster connect/disconnect methods
├── app_resources.go          # Resource CRUD methods
├── app_ai.go                 # AI query methods
├── ...                       # Other bridge files
├── wails.json                # Wails project configuration
├── build/
│   ├── appicon.png           # App icon
│   └── darwin/
│       ├── Info.plist        # macOS bundle info
│       └── entitlements.plist # Hardened Runtime entitlements
├── frontend/
│   ├── src/
│   │   ├── main.tsx          # React entry point
│   │   ├── App.tsx           # Root component (router)
│   │   ├── components/       # React components
│   │   ├── stores/           # Zustand state stores
│   │   ├── hooks/            # Custom React hooks
│   │   └── wailsjs/          # Auto-generated Wails bindings (do not edit)
│   ├── package.json
│   ├── vite.config.ts
│   └── tsconfig.json
└── internal/
    ├── ai/                   # AI provider implementations
    ├── analyzer/             # Resource health analysis
    ├── client/               # Kubernetes API client wrapper
    ├── config/               # Configuration management
    ├── core/                 # Core Kubernetes service layer
    ├── events/               # Wails event emitter
    ├── gitops/               # ArgoCD and Flux integration
    ├── history/              # Event collection and snapshotting
    ├── logging/              # Structured logging setup
    ├── security/             # Security scanning
    ├── storage/              # SQLite storage layer
    ├── terminal/             # PTY terminal manager
    └── version/              # Build version info
```

---

## Wails Bindings

When you add or modify Go methods on the `App` struct that you want to call from the frontend, run:

```bash
wails generate module
```

This regenerates `frontend/src/wailsjs/go/main/App.js` and the associated TypeScript definitions. These generated files are committed to the repository.

---

## Database

During development, the SQLite history database is at:
```
~/.local/state/kubecat/history.db
```

To inspect it:
```bash
sqlite3 ~/.local/state/kubecat/history.db
sqlite> .tables
sqlite> SELECT COUNT(*) FROM events;
```

To reset (clear all history):
```bash
rm ~/.local/state/kubecat/history.db
```

---

## Dev Container (Alternative)

If you prefer a containerized environment, a `.devcontainer/` configuration is included. Open the project in VS Code with the Dev Containers extension installed and click "Reopen in Container". The `post-create.sh` script installs all dependencies automatically.

Note: The Wails GUI window does not run inside a container. Dev container mode is suitable for backend development and tests only.

---

## CI Pipeline

The CI pipeline (`.github/workflows/ci.yaml`) runs on every pull request to `main`:

1. **Frontend Build & Test** — `npm ci`, `vitest`, `vite build`
2. **Backend Tests** — `go test -race ./...` (requires frontend `dist/` from step 1)
3. **Build Application** — `wails build -s` on macOS
4. **Version Preview** — semantic release dry-run

PRs are gated on all jobs passing. The Release workflow runs on merge to `main` and produces the signed macOS artifacts.
