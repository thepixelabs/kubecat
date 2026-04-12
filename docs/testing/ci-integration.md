# CI Integration (Testing)

This document explains how Kubecat runs tests in CI and how to reproduce CI locally.

## Workflow location

CI is configured in:

- `.github/workflows/ci.yaml`

## What CI runs

The workflow runs in parallel:

- **Frontend Tests** (Linux): installs `gui/frontend` dependencies and runs:
  - `npm run test`
  - `npm run test:coverage` (uploads `gui/frontend/coverage/` as an artifact)
- **Backend Tests** (Linux): runs:
  - `go test ./... -v -race -coverprofile=coverage.out` (uploads `coverage.out` as an artifact)
- **Build** (macOS): depends on both test jobs and runs `wails build` (uploads `build/bin/` as an artifact)
- **Version Preview** (PRs only): semantic-release dry run

## Reproducing CI locally

### Frontend (GUI)

```bash
cd gui/frontend
npm ci
npm run test
npm run test:coverage
```

### Backend (Go)

```bash
go test ./... -v -race -coverprofile=coverage.out
```

## Coverage artifacts

CI uploads:

- `frontend-coverage`: `gui/frontend/coverage/`
- `backend-coverage`: `coverage.out`

You can open the frontend HTML report locally after running coverage:

```bash
open gui/frontend/coverage/index.html
```

And the Go coverage report:

```bash
go tool cover -html=coverage.out
```

## Common CI failures

- **Node mismatch**: ensure you’re using Node 20 for `gui/frontend`
- **Flaky DOM tests**: avoid time-based assertions; prefer `waitFor` around stable UI states
- **Race detector failures**: fix shared mutable state; use locks or channels and rerun with `-race`


