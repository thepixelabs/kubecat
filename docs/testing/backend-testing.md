# Backend Testing Guide

The Kubecat Go backend uses the standard `testing` package with table-driven tests and interface-based mocking.

## Running Tests

```bash
# All tests
go test ./...

# Verbose output
go test -v ./...

# With race detection
go test -race ./...

# With coverage
go test -cover ./...

# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Single package
go test -v ./internal/storage/...

# Single test
go test -v -run TestSnapshotManager ./internal/storage/
```

## Test Structure

Tests are colocated with the code they test:

```
internal/
├── client/
│   ├── cluster.go
│   └── cluster_test.go
├── storage/
│   ├── snapshot.go
│   └── snapshot_test.go
├── analyzer/
│   ├── resource.go
│   └── resource_test.go
└── ai/
    ├── provider.go
    └── provider_test.go
```

## Writing Table-Driven Tests

```go
func TestResourceAnalyzer_Analyze(t *testing.T) {
    tests := []struct {
        name     string
        resource Resource
        want     *Analysis
        wantErr  bool
    }{
        {
            name: "healthy pod",
            resource: Resource{
                Kind:   "Pod",
                Name:   "web-1",
                Status: "Running",
            },
            want: &Analysis{
                Health:     HealthGood,
                Suggestion: "",
            },
            wantErr: false,
        },
        {
            name: "crashing pod",
            resource: Resource{
                Kind:     "Pod",
                Name:     "web-1",
                Status:   "CrashLoopBackOff",
                Restarts: 10,
            },
            want: &Analysis{
                Health:     HealthCritical,
                Suggestion: "Check container logs for crash reason",
            },
            wantErr: false,
        },
        {
            name:     "nil resource",
            resource: Resource{},
            want:     nil,
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            a := NewResourceAnalyzer()
            got, err := a.Analyze(context.Background(), tt.resource)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("Analyze() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("Analyze() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Interface-Based Mocking

### Define Interfaces

```go
// client.go
type ClusterClient interface {
    ListResources(ctx context.Context, kind, namespace string) ([]Resource, error)
    GetResource(ctx context.Context, kind, namespace, name string) (*Resource, error)
    WatchResources(ctx context.Context, kind, namespace string) (<-chan ResourceEvent, error)
}
```

### Create Mock Implementation

```go
// client_test.go
type mockClusterClient struct {
    resources    []Resource
    err          error
    watchEvents  []ResourceEvent
}

func (m *mockClusterClient) ListResources(ctx context.Context, kind, namespace string) ([]Resource, error) {
    if m.err != nil {
        return nil, m.err
    }
    return m.resources, nil
}

func (m *mockClusterClient) GetResource(ctx context.Context, kind, namespace, name string) (*Resource, error) {
    if m.err != nil {
        return nil, m.err
    }
    for _, r := range m.resources {
        if r.Name == name {
            return &r, nil
        }
    }
    return nil, ErrNotFound
}

func (m *mockClusterClient) WatchResources(ctx context.Context, kind, namespace string) (<-chan ResourceEvent, error) {
    ch := make(chan ResourceEvent)
    go func() {
        defer close(ch)
        for _, e := range m.watchEvents {
            ch <- e
        }
    }()
    return ch, nil
}
```

### Use Mock in Tests

```go
func TestExplorer_ListPods(t *testing.T) {
    mockClient := &mockClusterClient{
        resources: []Resource{
            {Kind: "Pod", Name: "web-1", Status: "Running"},
            {Kind: "Pod", Name: "web-2", Status: "Running"},
        },
    }
    
    explorer := NewExplorer(mockClient)
    pods, err := explorer.ListPods(context.Background(), "default")
    
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    if len(pods) != 2 {
        t.Errorf("expected 2 pods, got %d", len(pods))
    }
}
```

## Testing HTTP Handlers

```go
func TestHandler_GetResource(t *testing.T) {
    // Setup
    mockClient := &mockClusterClient{
        resources: []Resource{
            {Kind: "Pod", Name: "test-pod", Namespace: "default"},
        },
    }
    handler := NewHandler(mockClient)
    
    // Create request
    req := httptest.NewRequest("GET", "/api/pods/default/test-pod", nil)
    w := httptest.NewRecorder()
    
    // Execute
    handler.GetResource(w, req)
    
    // Assert
    res := w.Result()
    defer res.Body.Close()
    
    if res.StatusCode != http.StatusOK {
        t.Errorf("expected status 200, got %d", res.StatusCode)
    }
    
    var pod Resource
    if err := json.NewDecoder(res.Body).Decode(&pod); err != nil {
        t.Fatalf("failed to decode response: %v", err)
    }
    
    if pod.Name != "test-pod" {
        t.Errorf("expected name 'test-pod', got '%s'", pod.Name)
    }
}
```

## Testing with Context and Timeouts

```go
func TestClient_SlowOperation(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()
    
    client := &slowClient{delay: 500 * time.Millisecond}
    
    _, err := client.DoSomething(ctx)
    
    if !errors.Is(err, context.DeadlineExceeded) {
        t.Errorf("expected timeout error, got: %v", err)
    }
}
```

## Testing Concurrent Code

```go
func TestCache_ConcurrentAccess(t *testing.T) {
    cache := NewCache()
    
    // Use WaitGroup to synchronize goroutines
    var wg sync.WaitGroup
    
    // Concurrent writes
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            cache.Set(fmt.Sprintf("key-%d", i), i)
        }(i)
    }
    
    // Concurrent reads
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            cache.Get(fmt.Sprintf("key-%d", i))
        }(i)
    }
    
    wg.Wait()
    
    // Verify no races occurred (run with -race flag)
    if cache.Len() != 100 {
        t.Errorf("expected 100 items, got %d", cache.Len())
    }
}
```

## Test Fixtures

### Using testdata Directory

```
internal/analyzer/
├── analyzer.go
├── analyzer_test.go
└── testdata/
    ├── pod_healthy.yaml
    ├── pod_crashing.yaml
    └── deployment_scaled.yaml
```

```go
func TestAnalyzer_FromYAML(t *testing.T) {
    data, err := os.ReadFile("testdata/pod_healthy.yaml")
    if err != nil {
        t.Fatalf("failed to read test data: %v", err)
    }
    
    var pod Resource
    if err := yaml.Unmarshal(data, &pod); err != nil {
        t.Fatalf("failed to parse YAML: %v", err)
    }
    
    result, err := analyzer.Analyze(context.Background(), pod)
    // ...
}
```

## Testing Database Operations

```go
func TestStorage_SaveSnapshot(t *testing.T) {
    // Use in-memory SQLite for tests
    db, err := sql.Open("sqlite3", ":memory:")
    if err != nil {
        t.Fatalf("failed to open db: %v", err)
    }
    defer db.Close()
    
    // Run migrations
    if err := runMigrations(db); err != nil {
        t.Fatalf("failed to run migrations: %v", err)
    }
    
    storage := NewStorage(db)
    
    snapshot := &Snapshot{
        ID:        "snap-1",
        Timestamp: time.Now(),
        Data:      []byte(`{"pods": []}`),
    }
    
    if err := storage.SaveSnapshot(context.Background(), snapshot); err != nil {
        t.Fatalf("failed to save snapshot: %v", err)
    }
    
    // Verify
    loaded, err := storage.GetSnapshot(context.Background(), "snap-1")
    if err != nil {
        t.Fatalf("failed to get snapshot: %v", err)
    }
    
    if loaded.ID != snapshot.ID {
        t.Errorf("expected ID %s, got %s", snapshot.ID, loaded.ID)
    }
}
```

## Subtests and Cleanup

```go
func TestManager(t *testing.T) {
    // Setup that runs once
    db := setupTestDB(t)
    
    t.Run("Create", func(t *testing.T) {
        // Test create functionality
    })
    
    t.Run("Update", func(t *testing.T) {
        // Test update functionality
    })
    
    t.Run("Delete", func(t *testing.T) {
        // Test delete functionality
    })
    
    // Cleanup runs after all subtests
    t.Cleanup(func() {
        db.Close()
    })
}
```

## Benchmarks

```go
func BenchmarkCache_Get(b *testing.B) {
    cache := NewCache()
    
    // Setup
    for i := 0; i < 1000; i++ {
        cache.Set(fmt.Sprintf("key-%d", i), i)
    }
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        cache.Get(fmt.Sprintf("key-%d", i%1000))
    }
}
```

Run benchmarks:
```bash
go test -bench=. -benchmem ./internal/cache/
```

## Best Practices

1. **Use `t.Helper()`** for test helper functions
2. **Use `t.Parallel()`** for independent tests
3. **Clean up resources** with `t.Cleanup()`
4. **Use meaningful test names** that describe the scenario
5. **Test error cases** not just happy paths
6. **Use `-race` flag** to detect race conditions
7. **Keep tests fast** - mock slow dependencies

