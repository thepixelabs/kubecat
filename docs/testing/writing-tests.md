# Writing Tests

This guide explains how to write maintainable tests for Kubecat (Go backend + Wails/React frontend). It focuses on *patterns* you can repeat across features.

## What “good” looks like

- **Behavior-first**: test what users/consumers observe (rendered UI, returned values, side effects), not internal implementation details.
- **Deterministic**: avoid real time, randomness, network, real kube clusters, or real OS APIs in unit tests.
- **Fast**: tests should run in seconds locally and in CI.
- **Small units, realistic integration**: mock boundaries (Wails bindings, kube API, filesystem), but keep business logic real.

## Frontend (React) patterns

### 1) Prefer integration-style component tests

In the GUI, components often compose hooks + state + Wails calls. Prefer a test that renders the component and drives user behavior.

Checklist:

- Render the component with minimal props
- Mock the Wails binding module(s)
- Interact via buttons/selects
- Assert visible text / enabled state / callbacks

### 2) Test hooks when logic is non-trivial

Hooks like `useClusterGraph` and `usePathHighlight` have meaningful logic. Unit test them directly with `renderHook`.

Good asserts:

- `loading` transitions
- `graphData` content
- `refetch()` triggers additional calls
- edge cases (disconnected, empty input)

### 3) Keep DOM-heavy libraries mocked

Some libraries (e.g., React Flow, animations) are hard to run in jsdom. Mock them and focus on your own behavior:

- **React Flow**: stub `ReactFlow`, `Background`, `Controls`, `MiniMap`, and state hooks
- **framer-motion**: stub `motion.*` components to plain `<div>`
- **ELK**: stub `layout()` to avoid async graph layout complexity in tests

### 4) Use helpers for repeatable setup

If multiple tests need the same providers (React Query, themes), wrap them once in a `render` helper (already present in `gui/frontend/src/test/test-utils.tsx`).

## Backend (Go) patterns

### 1) Table-driven tests

Use table-driven tests for business logic:

```go
func TestSomething(t *testing.T) {
  tests := []struct {
    name    string
    input   string
    want    string
    wantErr bool
  }{
    {"happy path", "in", "out", false},
    {"bad input", "", "", true},
  }

  for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
      got, err := DoThing(tt.input)
      if (err != nil) != tt.wantErr {
        t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
      }
      if got != tt.want {
        t.Fatalf("got=%q want=%q", got, tt.want)
      }
    })
  }
}
```

### 2) Mock through interfaces, not global state

Define narrow interfaces around external dependencies (kube client, storage, AI provider). In tests, create small fakes/mocks that implement those interfaces.

### 3) Prefer “in-memory” dependencies for integration tests

- SQLite: `:memory:`
- FS: in-memory abstraction (or temp dirs via `t.TempDir()`)
- Time: inject a clock function if needed (e.g., `now func() time.Time`)

## Naming and file placement

### Frontend

- `*.test.ts` for hook/logic tests
- `*.test.tsx` for component tests
- Keep tests colocated near the code under test

### Backend

- `*_test.go` next to `*.go`
- Use `testdata/` folders for fixtures

## What to test next (practical roadmap)

For “all GUI app functionality”, start by converting the most important user workflows into test suites:

- **Cluster Visualizer**: disconnected state → connected render → toggles filter nodes/edges → refresh triggers refetch
- **Cluster Diff**: mode switch (cross-cluster/historical) → resource selection → compute diff → apply dry-run → export
- **Settings**: theme toggle (dark/light) → persisted preferences (if present)

Then add a small number of *feature-level* integration tests that cover multi-step flows end-to-end within the React app (still mocking Wails).


