# Testing Overview

## Philosophy

Kubecat follows a **testing pyramid** approach:

```
          ╱╲
         ╱  ╲
        ╱ E2E ╲         Few, slow, high-value
       ╱──────╲
      ╱  Integ  ╲       Some, medium speed
     ╱──────────╲
    ╱    Unit    ╲      Many, fast, isolated
   ╱──────────────╲
```

### Unit Tests

- Test individual functions, hooks, and components in isolation
- Mock all external dependencies
- Fast execution (< 100ms each)
- High coverage target (80%+)

### Integration Tests

- Test component interactions
- Test API handler chains
- Use realistic mocks
- Medium execution time

### E2E Tests (Planned)

- Test complete user workflows
- Real browser/app environment
- Slowest, but highest confidence

## Architecture-Specific Testing

### GUI Layer (React + Wails)

The GUI is a Wails desktop application with a React frontend that communicates with a Go backend through bindings.

```
┌─────────────────────────────────────────────────────┐
│                 React Frontend                       │
│  ┌─────────────┐  ┌─────────────┐  ┌────────────┐  │
│  │ Components  │  │   Hooks     │  │   Stores   │  │
│  │   (Vitest)  │  │  (Vitest)   │  │  (Vitest)  │  │
│  └──────┬──────┘  └──────┬──────┘  └─────┬──────┘  │
│         └────────────────┼───────────────┘          │
│                          │                          │
│                    ┌─────┴─────┐                    │
│                    │   Mocked  │                    │
│                    │   Wails   │                    │
│                    │  Bindings │                    │
│                    └───────────┘                    │
└─────────────────────────────────────────────────────┘
                          │
                    (Not tested together)
                          │
┌─────────────────────────────────────────────────────┐
│                  Go Backend                          │
│  ┌─────────────┐  ┌─────────────┐  ┌────────────┐  │
│  │  Handlers   │  │   Core      │  │  Storage   │  │
│  │ (go test)   │  │  (go test)  │  │ (go test)  │  │
│  └─────────────┘  └─────────────┘  └────────────┘  │
└─────────────────────────────────────────────────────┘
```

### Key Testing Boundaries

| Boundary       | What to Mock           | Why                                 |
| -------------- | ---------------------- | ----------------------------------- |
| Wails Bindings | `wailsjs/go/main/App`  | Frontend shouldn't need Go running  |
| Kubernetes API | `client-go` interfaces | Tests shouldn't need a real cluster |
| Database       | SQLite in-memory       | Fast, isolated tests                |
| File System    | `afero` or similar     | Portable, predictable               |

## Test Categories by Feature

### Cluster Visualizer

| Test Type   | What's Tested          | Location                     |
| ----------- | ---------------------- | ---------------------------- |
| Unit        | Node filtering logic   | `useClusterGraph.test.ts`    |
| Unit        | Path highlighting      | `usePathHighlight.test.ts`   |
| Integration | Full visualizer render | `ClusterVisualizer.test.tsx` |
| Integration | Toggle interactions    | `TogglePanel.test.tsx`       |

### Cluster Diff

| Test Type   | What's Tested    | Location                  |
| ----------- | ---------------- | ------------------------- |
| Unit        | Diff computation | Backend Go tests          |
| Integration | UI workflow      | `ClusterDiff.test.tsx`    |
| Integration | Source selection | `SourceSelector.test.tsx` |

### Time Travel

| Test Type   | What's Tested     | Location             |
| ----------- | ----------------- | -------------------- |
| Unit        | Snapshot storage  | `storage_test.go`    |
| Unit        | Event correlation | `correlator_test.go` |
| Integration | Timeline UI       | (Planned)            |

## Running Tests Locally

### Quick Commands

```bash
# Frontend - Run once
cd gui/frontend && npm run test

# Frontend - Watch mode (development)
cd gui/frontend && npm run test:watch

# Frontend - With coverage
cd gui/frontend && npm run test:coverage

# Backend - All tests
go test ./...

# Backend - Verbose
go test -v ./...

# Backend - Single package
go test -v ./internal/storage/...

# Backend - With race detection
go test -race ./...
```

### IDE Integration

**VS Code / Cursor:**

- Install "Vitest" extension for frontend tests
- Install "Go" extension for backend tests
- Tests appear inline with run buttons

**Terminal:**

- Vitest provides interactive watch mode
- Use `go test -v` for verbose output

## Test Data Management

### Frontend Test Data

Test data is defined in mock files:

```typescript
// src/test/mocks/wails-app.ts
export const mockPodResource = (name: string, namespace: string) => ({
  kind: "Pod",
  name,
  namespace,
  status: "Running",
  // ...
});
```

### Backend Test Data

Use table-driven tests with inline data:

```go
func TestAnalyze(t *testing.T) {
    tests := []struct {
        name     string
        input    Resource
        expected Analysis
    }{
        {"healthy pod", healthyPod, expectedAnalysis},
        {"failing pod", failingPod, failureAnalysis},
    }
    // ...
}
```

## Debugging Failed Tests

### Frontend

```bash
# Run single test file
npx vitest run src/components/cluster-visualizer/ClusterVisualizer.test.tsx

# Run with debug output
DEBUG=true npm run test

# Interactive UI
npx vitest --ui
```

### Backend

```bash
# Run single test
go test -v -run TestSpecificTest ./internal/storage/

# With debugger (Delve)
dlv test ./internal/storage/ -- -test.run TestSpecificTest
```
