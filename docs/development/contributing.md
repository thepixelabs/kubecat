# Contributing

Thank you for your interest in contributing to Kubecat!

## Getting Started

1. Fork the repository
2. Clone your fork
3. Create a feature branch
4. Make your changes
5. Submit a pull request

## Development Setup

See [Development Setup](setup.md) for the full setup guide including platform-specific dependencies, Wails CLI installation, and the development workflow.

Quick start:

```bash
# Clone the repository
git clone https://github.com/thepixelabs/kubecat.git
cd kubecat

# Install dependencies
go mod download
cd frontend && npm ci && cd ..

# Install git hooks
lefthook install

# Start development mode (Go backend + React frontend with hot-reload)
wails dev

# Run all tests
go test -race ./...
cd frontend && npm run test && cd ..

# Run linter
golangci-lint run
```

## Project Structure

See [Project Structure](../architecture/project-structure.md) for detailed layout.

## Code Style

### Go Code

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` for formatting
- Use `golangci-lint` for linting
- Write tests for new functionality

### Comments

```go
// Package client provides multi-cluster Kubernetes client management.
package client

// ClusterClient is the interface for interacting with a single cluster.
// It provides methods for listing, getting, and watching resources.
type ClusterClient interface {
    // List returns resources of a given kind.
    List(ctx context.Context, kind string, opts ListOptions) (*ResourceList, error)
}
```

### Error Handling

```go
// Use descriptive error messages
if err != nil {
    return fmt.Errorf("failed to connect to cluster %s: %w", name, err)
}

// Define domain-specific errors
var ErrContextNotFound = errors.New("context not found")
```

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add RBAC visualization

fix: handle empty namespace in explorer

docs: update installation guide

refactor: extract table component

test: add unit tests for cluster manager
```

### Types

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation |
| `refactor` | Code restructuring |
| `test` | Adding tests |
| `chore` | Maintenance |

## Pull Request Process

1. **Create an issue first** - Discuss the change before implementing
2. **Keep PRs focused** - One feature/fix per PR
3. **Write tests** - Aim for good coverage
4. **Update docs** - If behavior changes
5. **Pass CI** - All checks must pass

### PR Template

```markdown
## Description
Brief description of the changes.

## Related Issue
Fixes #123

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation

## Checklist
- [ ] Tests pass locally
- [ ] Linter passes
- [ ] Documentation updated
- [ ] Commit messages follow convention
```

## Testing

### Running Tests

```bash
# All tests
make test

# Specific package
go test ./internal/client/...

# With coverage
go test -cover ./...

# Verbose
go test -v ./...
```

### Writing Tests

```go
func TestClusterManager_Add(t *testing.T) {
    // Arrange
    manager := NewManager()
    
    // Act
    err := manager.Add(ctx, "test-context")
    
    // Assert
    assert.NoError(t, err)
    assert.True(t, manager.IsConnected("test-context"))
}
```

## Documentation

- Update docs for user-facing changes
- Use clear, concise language
- Include examples
- Keep code samples up to date

## Release Process

1. Update version in `internal/version/version.go`
2. Update CHANGELOG.md
3. Create a git tag: `git tag v0.1.0`
4. Push: `git push origin v0.1.0`
5. CI builds and publishes releases

## Community

- Be respectful and inclusive
- Help others in issues and discussions
- Share your use cases and feedback

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.

