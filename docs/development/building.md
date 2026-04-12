# Building from Source

Kubecat is built using [Wails](https://wails.io), which combines a Go backend with a React frontend.

## Requirements

- Go 1.23 or later
- Node.js 20+ and npm
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0`)

## Components

The application consists of two parts that are compiled together:

1. **Frontend**: React/TypeScript application in `frontend/`
2. **Backend**: Go application in project root

## Quick Build

The easiest way to build is using `wails`:

```bash
# Build binary for your architecture
wails build

# Build with optimizations (smaller binary)
wails build -s
```

The output binary will be located in `build/bin/`.

## Development

To run the application in development mode with hot-reloading:

```bash
wails dev
```

This will:

- Start the frontend dev server (Vite)
- Compile and run the Go backend
- Watch for file changes in both

## Production Build & Versioning

To build a production release with correct version numbers, we inject the version at build time using Go `ldflags`.

### How Versioning Works

1.  **CI/CD (GitHub Actions)**: The release workflow calculates the version (e.g., from git tag).
2.  **Injection**: It runs `wails build` with `-ldflags` to overwrite variables in `internal/version/version.go`.
3.  **Runtime**: The `App` struct (in `app.go`) reads these variables and exposes them to the frontend via `GetAppVersion()`.

### Manual Release Build

To manually build a versioned binary locally:

```bash
VERSION="v1.2.3"
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')

wails build -s -platform darwin/universal \
  -ldflags "-X github.com/thepixelabs/kubecat/internal/version.Version=${VERSION} \
            -X github.com/thepixelabs/kubecat/internal/version.GitCommit=${COMMIT} \
            -X github.com/thepixelabs/kubecat/internal/version.BuildDate=${DATE}"
```

## Cross-Compilation

Wails supports cross-compilation. Common targets:

```bash
# macOS Universal (Intel + Apple Silicon)
wails build -platform darwin/universal

# Windows AMD64
wails build -platform windows/amd64

# Linux AMD64
wails build -platform linux/amd64
```
