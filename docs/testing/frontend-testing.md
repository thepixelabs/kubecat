# Frontend Testing Guide

The Kubecat GUI uses **Vitest** with **React Testing Library** for testing React components and hooks.

## Setup

### Dependencies

```json
{
  "devDependencies": {
    "@testing-library/jest-dom": "^6.4.2",
    "@testing-library/react": "^14.2.1",
    "@testing-library/user-event": "^14.5.2",
    "@vitest/coverage-v8": "^1.3.1",
    "jsdom": "^24.0.0",
    "vitest": "^1.3.1"
  }
}
```

### Configuration

**vitest.config.ts:**
```typescript
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.{test,spec}.{js,ts,jsx,tsx}'],
    exclude: ['node_modules', 'dist', 'wailsjs'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
    },
  },
});
```

### Test Setup

**src/test/setup.ts:**
```typescript
import '@testing-library/jest-dom';
import { vi } from 'vitest';

// Mock browser APIs not available in jsdom
Object.defineProperty(window, 'matchMedia', {
  value: vi.fn().mockImplementation((query) => ({
    matches: false,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  })),
});

// Mock ResizeObserver (required for React Flow)
class ResizeObserverMock {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}
window.ResizeObserver = ResizeObserverMock;

// Mock Wails runtime
vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn(),
  EventsOff: vi.fn(),
  EventsEmit: vi.fn(),
}));
```

## Running Tests

```bash
# Run all tests
npm run test

# Watch mode (re-runs on changes)
npm run test:watch

# With coverage report
npm run test:coverage

# Run specific file
npx vitest run src/components/MyComponent.test.tsx

# Interactive UI
npx vitest --ui
```

## Writing Component Tests

### Basic Component Test

```typescript
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { TogglePanel } from './TogglePanel';

describe('TogglePanel', () => {
  const defaultProps = {
    toggles: { showPods: true, showServices: false },
    onToggle: vi.fn(),
    namespaces: ['default', 'kube-system'],
    selectedNamespace: 'default',
    onNamespaceChange: vi.fn(),
  };

  it('should render namespace selector', () => {
    render(<TogglePanel {...defaultProps} />);
    
    expect(screen.getByRole('combobox')).toBeInTheDocument();
    expect(screen.getByText('default')).toBeInTheDocument();
  });

  it('should call onToggle when button clicked', () => {
    const onToggle = vi.fn();
    render(<TogglePanel {...defaultProps} onToggle={onToggle} />);
    
    fireEvent.click(screen.getByText('Pods'));
    
    expect(onToggle).toHaveBeenCalledWith('showPods');
  });
});
```

### Testing with User Events

Use `@testing-library/user-event` for realistic interactions:

```typescript
import userEvent from '@testing-library/user-event';

it('should update namespace on selection', async () => {
  const user = userEvent.setup();
  const onNamespaceChange = vi.fn();
  
  render(<TogglePanel {...defaultProps} onNamespaceChange={onNamespaceChange} />);
  
  const select = screen.getByRole('combobox');
  await user.selectOptions(select, 'kube-system');
  
  expect(onNamespaceChange).toHaveBeenCalledWith('kube-system');
});
```

### Testing Async Components

```typescript
import { render, screen, waitFor } from '@testing-library/react';

it('should load and display data', async () => {
  mockListResources.mockResolvedValue([
    { name: 'pod-1', status: 'Running' },
  ]);
  
  render(<ClusterVisualizer isConnected={true} namespaces={[]} />);
  
  // Wait for loading to finish
  await waitFor(() => {
    expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
  });
  
  // Verify data is displayed
  expect(screen.getByTestId('react-flow')).toBeInTheDocument();
});
```

## Writing Hook Tests

Use `renderHook` from React Testing Library:

```typescript
import { renderHook, waitFor } from '@testing-library/react';
import { useClusterGraph } from './useClusterGraph';

describe('useClusterGraph', () => {
  it('should fetch data when connected', async () => {
    const { result } = renderHook(() => 
      useClusterGraph({ namespace: 'default', isConnected: true })
    );
    
    expect(result.current.loading).toBe(true);
    
    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
    
    expect(result.current.graphData.nodes.length).toBeGreaterThan(0);
  });

  it('should not fetch when disconnected', () => {
    const { result } = renderHook(() =>
      useClusterGraph({ namespace: 'default', isConnected: false })
    );
    
    expect(result.current.graphData.nodes).toHaveLength(0);
    expect(mockListResources).not.toHaveBeenCalled();
  });
});
```

### Testing Hook State Changes

```typescript
it('should refetch when namespace changes', async () => {
  const { result, rerender } = renderHook(
    ({ namespace }) => useClusterGraph({ namespace, isConnected: true }),
    { initialProps: { namespace: 'default' } }
  );
  
  await waitFor(() => expect(result.current.loading).toBe(false));
  
  // Change namespace
  rerender({ namespace: 'kube-system' });
  
  await waitFor(() => {
    expect(mockListResources).toHaveBeenCalledWith('pods', 'kube-system');
  });
});
```

## Mocking Strategies

### Mocking Wails Bindings

```typescript
// src/test/mocks/wails-app.ts
import { vi } from 'vitest';

export const createWailsAppMocks = () => ({
  ListResources: vi.fn().mockResolvedValue([]),
  GetClusterEdges: vi.fn().mockResolvedValue([]),
  Connect: vi.fn().mockResolvedValue(true),
  // ... other bindings
});

// In tests
vi.mock('../../../wailsjs/go/main/App', () => createWailsAppMocks());
```

### Mocking Complex Libraries

```typescript
// Mock React Flow (requires DOM measurements)
vi.mock('@xyflow/react', () => ({
  ReactFlow: ({ children }) => <div data-testid="react-flow">{children}</div>,
  Background: () => <div data-testid="background" />,
  Controls: () => <div data-testid="controls" />,
  useNodesState: () => [[], vi.fn(), vi.fn()],
  useEdgesState: () => [[], vi.fn(), vi.fn()],
}));

// Mock framer-motion (avoid animation timing issues)
vi.mock('framer-motion', () => ({
  motion: {
    div: ({ children, ...props }) => <div {...props}>{children}</div>,
  },
  AnimatePresence: ({ children }) => <>{children}</>,
}));
```

## Test Utilities

### Custom Render with Providers

```typescript
// src/test/test-utils.tsx
import { render, RenderOptions } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ThemeProvider } from 'next-themes';

const AllProviders = ({ children }) => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider defaultTheme="dark">
        {children}
      </ThemeProvider>
    </QueryClientProvider>
  );
};

const customRender = (ui, options) =>
  render(ui, { wrapper: AllProviders, ...options });

export * from '@testing-library/react';
export { customRender as render };
```

### Mock Data Factories

```typescript
// src/test/mocks/wails-app.ts
export const mockPodResource = (
  name: string,
  namespace: string,
  status = 'Running'
) => ({
  kind: 'Pod',
  name,
  namespace,
  status,
  age: '1d',
  restarts: 0,
  node: 'node-1',
});

export const mockClusterNode = (
  type: NodeType,
  name: string,
  namespace: string
): ClusterNode => ({
  id: `${type}/${namespace}/${name}`,
  type,
  name,
  namespace,
  status: 'Running',
});
```

## Common Patterns

### Testing Loading States

```typescript
it('should show loading indicator', () => {
  // Mock slow response
  mockListResources.mockImplementation(
    () => new Promise(resolve => setTimeout(() => resolve([]), 1000))
  );
  
  render(<ClusterVisualizer isConnected={true} namespaces={[]} />);
  
  expect(screen.getByText('Loading cluster data...')).toBeInTheDocument();
});
```

### Testing Error States

```typescript
it('should handle errors gracefully', async () => {
  mockListResources.mockRejectedValue(new Error('Network error'));
  
  render(<ClusterVisualizer isConnected={true} namespaces={[]} />);
  
  await waitFor(() => {
    expect(screen.getByText(/error/i)).toBeInTheDocument();
  });
});
```

### Testing Conditional Rendering

```typescript
it('should show disconnected state when not connected', () => {
  render(<ClusterVisualizer isConnected={false} namespaces={[]} />);
  
  expect(screen.getByText('No cluster connected')).toBeInTheDocument();
  expect(screen.queryByTestId('react-flow')).not.toBeInTheDocument();
});
```

## Debugging Tips

### Debug DOM Output

```typescript
import { screen } from '@testing-library/react';

it('debug test', () => {
  render(<MyComponent />);
  
  // Print DOM to console
  screen.debug();
  
  // Print specific element
  screen.debug(screen.getByRole('button'));
});
```

### Check Accessibility

```typescript
it('should be accessible', () => {
  render(<TogglePanel {...props} />);
  
  // Verify ARIA labels
  expect(screen.getByRole('combobox')).toHaveAccessibleName();
  
  // Verify buttons are focusable
  const buttons = screen.getAllByRole('button');
  buttons.forEach(btn => {
    expect(btn).not.toHaveAttribute('tabindex', '-1');
  });
});
```

## File Naming Convention

```
src/
└── components/
    └── cluster-visualizer/
        ├── ClusterVisualizer.tsx
        ├── ClusterVisualizer.test.tsx    # Component tests
        └── hooks/
            ├── useClusterGraph.ts
            └── useClusterGraph.test.ts   # Hook tests
```

Tests are colocated with the code they test using the `.test.tsx` or `.test.ts` suffix.

