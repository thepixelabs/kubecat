# Testing

> For the full testing documentation (including GUI testing), see: [`docs/testing/README.md`](../testing/README.md)

## Running Tests

```bash
# All tests
make test

# Verbose
go test -v ./...

# With coverage
go test -cover ./...

# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Frontend (GUI) Tests

The GUI lives in `gui/frontend` and uses **Vitest** + **React Testing Library**.

```bash
cd gui/frontend
npm ci

# Run tests once
npm run test

# Watch mode
npm run test:watch

# Coverage
npm run test:coverage
```

## Test Structure

```
internal/
├── client/
│   ├── cluster.go
│   └── cluster_test.go
├── config/
│   ├── config.go
│   └── config_test.go
```

## Writing Tests

```go
func TestManager_Add(t *testing.T) {
    tests := []struct {
        name    string
        context string
        wantErr bool
    }{
        {"valid context", "test-context", false},
        {"duplicate", "test-context", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            m := NewManager()
            err := m.Add(ctx, tt.context)
            if (err != nil) != tt.wantErr {
                t.Errorf("Add() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Mocking

Use interfaces for mockable dependencies:

```go
type ClusterClient interface {
    List(ctx context.Context, kind string) (*ResourceList, error)
}

// In tests
type mockClient struct {
    resources []Resource
    err       error
}

func (m *mockClient) List(ctx context.Context, kind string) (*ResourceList, error) {
    return &ResourceList{Items: m.resources}, m.err
}
```

