# Mocking Guide

This guide documents the main mocking boundaries in Kubecat and how we mock them in tests.

## Mocking boundaries

| Boundary | What to mock | Where |
|---|---|---|
| Wails bindings | `wailsjs/go/main/App` module | Frontend tests |
| Wails runtime | `wailsjs/runtime/runtime` module | Frontend test setup |
| DOM APIs | `ResizeObserver`, `matchMedia`, `IntersectionObserver` | `src/test/setup.ts` |
| Kubernetes API | interfaces around `client-go` usage | Go tests |
| Time | inject `now()` / clock | Go tests |
| Storage | in-memory sqlite / fakes | Go tests |

## Frontend: mocking Wails bindings

The React UI calls into generated Wails bindings (e.g. `ListResources`, `GetClusterEdges`). In tests, we mock the module so the UI can run without the Go backend.

### Option A: mock per test file

```ts
import { vi } from "vitest";

const mockListResources = vi.fn();
const mockGetClusterEdges = vi.fn();

vi.mock("../../../wailsjs/go/main/App", () => ({
  ListResources: (...args: unknown[]) => mockListResources(...args),
  GetClusterEdges: (...args: unknown[]) => mockGetClusterEdges(...args),
}));
```

Use this when a test suite needs special behavior per test.

### Option B: shared mock factory

Use a shared factory (e.g. `gui/frontend/src/test/mocks/wails-app.ts`) when many test files need consistent defaults and convenient overrides.

Guideline: keep the factory small and composable:

- `createWailsAppMocks(overrides)`
- `mockPodResource(...)`, `mockServiceResource(...)` etc.

## Frontend: mocking DOM APIs

Libraries like React Flow require browser APIs not present in jsdom. We set these globally in:

- `gui/frontend/src/test/setup.ts`

Mocks typically include:

- `ResizeObserver`
- `IntersectionObserver`
- `window.matchMedia`
- `Element.prototype.scrollIntoView`
- `Element.prototype.getBoundingClientRect`

Rule of thumb: keep these mocks minimal and predictable.

## Frontend: mocking UI-heavy libraries

### React Flow

Mock to a simple component and assert your UI renders around it:

```ts
vi.mock("@xyflow/react", () => ({
  ReactFlow: ({ children }: { children?: React.ReactNode }) => (
    <div data-testid="react-flow">{children}</div>
  ),
  Background: () => <div data-testid="react-flow-background" />,
  Controls: () => <div data-testid="react-flow-controls" />,
  MiniMap: () => <div data-testid="react-flow-minimap" />,
  Panel: ({ children, position }: any) => (
    <div data-testid={`react-flow-panel-${position}`}>{children}</div>
  ),
  useNodesState: () => [[], vi.fn(), vi.fn()],
  useEdgesState: () => [[], vi.fn(), vi.fn()],
}));
```

### framer-motion

Replace motion elements with plain DOM:

```ts
vi.mock("framer-motion", () => ({
  motion: {
    div: ({ children, ...props }: any) => <div {...props}>{children}</div>,
  },
  AnimatePresence: ({ children }: any) => <>{children}</>,
}));
```

### ELK layout

If a component uses ELK graph layout, stub it:

```ts
vi.mock("elkjs/lib/elk.bundled.js", () => ({
  default: class ELK {
    layout = vi.fn().mockResolvedValue({ children: [] });
  },
}));
```

## Backend: mocking Kubernetes clients

Prefer interfaces around what you use:

```go
type ResourceLister interface {
  ListResources(ctx context.Context, kind, namespace string) ([]Resource, error)
}
```

Then implement small fakes:

```go
type fakeLister struct {
  items []Resource
  err   error
}

func (f *fakeLister) ListResources(ctx context.Context, kind, namespace string) ([]Resource, error) {
  if f.err != nil {
    return nil, f.err
  }
  return f.items, nil
}
```

## Anti-patterns (avoid)

- **Mocking internal implementation details** (private helpers, internal state)
- **Over-mocking** (tests pass but don’t reflect real behavior)
- **Brittle snapshot tests** for complex UIs
- **Sleeping in tests** (`setTimeout`, `time.Sleep`) instead of waiting on conditions


