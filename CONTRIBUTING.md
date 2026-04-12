# Contributing to Kubecat

## Commit Message Format

This project uses [Conventional Commits](https://www.conventionalcommits.org/) for automated versioning.

### Format

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Types

| Type       | Description                            | Version Bump |
| ---------- | -------------------------------------- | ------------ |
| `feat`     | New feature                            | MINOR        |
| `fix`      | Bug fix                                | PATCH        |
| `docs`     | Documentation only                     | None         |
| `style`    | Formatting, no code change             | None         |
| `refactor` | Code change, no new feature or bug fix | None         |
| `perf`     | Performance improvement                | PATCH        |
| `test`     | Adding tests                           | None         |
| `chore`    | Maintenance tasks                      | None         |

### Breaking Changes

For breaking changes, add `!` after the type or include `BREAKING CHANGE:` in the footer:

```
feat!: redesign API endpoints

BREAKING CHANGE: All `/api/v1` endpoints moved to `/api/v2`
```

### Examples

```
feat(visualizer): add pod status indicators
fix(security): resolve RBAC permission check
docs: update installation instructions
chore(deps): bump go version to 1.21
```
