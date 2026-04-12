# Testing Guide

Kubecat uses a comprehensive testing strategy that covers both the Go backend and the React frontend. This ensures reliability across the entire application stack.

## Documentation Index

- **[Overview](./overview.md)** - Testing philosophy and strategy
- **[Frontend Testing](./frontend-testing.md)** - React/TypeScript testing with Vitest
- **[Backend Testing](./backend-testing.md)** - Go testing patterns
- **[Writing Tests](./writing-tests.md)** - Best practices and patterns
- **[CI Integration](./ci-integration.md)** - Continuous integration setup
- **[Mocking Guide](./mocking.md)** - How to mock dependencies

## Quick Start

### Run All Tests

```bash
# Frontend tests
cd frontend
npm run test

# Backend tests
go test ./...

# Or use make
make test
```

### Run with Coverage

```bash
# Frontend coverage
cd frontend
npm run test:coverage

# Backend coverage
go test -cover ./...
```

## Test Structure

```
kubecat/
├── frontend/
│   └── src/
│       ├── test/                    # Test utilities and setup
│       │   ├── setup.ts             # Global test setup
│       │   ├── test-utils.tsx       # Custom render functions
│       │   └── mocks/               # Wails/API mocks
│       └── components/
│           └── **/*.test.tsx        # Component tests (colocated)
│
└── internal/
    └── **/
        └── *_test.go                # Go tests (colocated)
```

## Testing Stack

| Layer | Framework | Coverage |
|-------|-----------|----------|
| Frontend Components | Vitest + React Testing Library | Unit/Integration |
| Frontend Hooks | Vitest + renderHook | Unit |
| Backend Logic | Go testing + testify | Unit |
| API Handlers | httptest | Integration |
| E2E | (Planned) Playwright | E2E |

## Coverage Goals

| Component | Target | Current |
|-----------|--------|---------|
| Hooks | 90%+ | ✅ 95%+ |
| Components | 70%+ | ✅ 70%+ |
| Backend Core | 80%+ | 🔄 In progress |

## Key Principles

1. **Test behavior, not implementation** - Focus on what users see and do
2. **Colocate tests with code** - Tests live next to the code they test
3. **Mock external dependencies** - Isolate units under test
4. **Fast feedback** - Tests should run quickly in development
5. **CI gates** - All tests must pass before merge

