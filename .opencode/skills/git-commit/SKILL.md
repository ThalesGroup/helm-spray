---
name: git-commit
description: >
  Structured git commit messages following Conventional Commits format
  for Go projects. Generates well-scoped, atomic commits with clear descriptions.
  Use when committing changes, writing commit messages, preparing PRs,
  or reviewing commit history quality.
  Trigger examples: "commit these changes", "create commit", "commit message",
  "prepare PR", "squash commits".
  Do NOT use for changelog generation (use changelog-generator) or
  code review (use go-code-review).
---

# Git Commit Standards

Commits tell the story of your codebase. A good commit history is worth
more than any amount of documentation — because it's always up to date.

## 1. Conventional Commits Format

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Types:

| Type | When |
|---|---|
| `feat` | New feature (correlates with MINOR in semver) |
| `fix` | Bug fix (correlates with PATCH in semver) |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `perf` | Performance improvement |
| `test` | Adding or correcting tests |
| `docs` | Documentation changes only |
| `chore` | Build process, tooling, dependencies |
| `ci` | CI/CD configuration changes |
| `style` | Formatting, whitespace (not CSS — code formatting) |

### Scope:

Use the package name or module area:

```
feat(auth): add JWT refresh token rotation
fix(store/postgres): handle connection pool exhaustion
refactor(service): extract validation into dedicated package
test(handler): add table-driven tests for user endpoints
chore(deps): bump go.uber.org/zap to v1.27.0
```

### Breaking changes:

```
feat(api)!: change pagination from offset to cursor-based

BREAKING CHANGE: The `offset` and `limit` query parameters are replaced
by `cursor` and `page_size`. All existing clients must migrate.
```

## 2. Commit Message Rules

### Subject line:
- Imperative mood: "add feature", NOT "added feature" or "adds feature"
- Lowercase after type prefix
- No period at the end
- Max 72 characters
- Must describe WHAT changed, not HOW

### Body (when needed):
- Blank line between subject and body
- Explain WHY the change was necessary
- Explain WHAT is different at a high level
- Wrap at 72 characters

### Footer:
- Reference issues: `Fixes #123`, `Closes #456`, `Refs #789`
- Co-authors: `Co-authored-by: Name <email>`
- Breaking changes: `BREAKING CHANGE: description`

## 3. Examples

### Simple change:

```
fix(handler): return 404 instead of 500 for missing user
```

### With body:

```
refactor(service): replace manual SQL with sqlx named queries

The raw SQL string concatenation for dynamic WHERE clauses was
error-prone and difficult to maintain. sqlx named queries provide
the same flexibility with automatic parameter binding.

No behavior change — all existing tests pass.
```

### Breaking change:

```
feat(config)!: migrate from YAML to environment variables

BREAKING CHANGE: Configuration is now loaded from environment
variables instead of config.yaml. See README.md for the full
list of supported variables.

Closes #234
```

### Dependency update:

```
chore(deps): upgrade pgx to v5.5.0

Picks up connection pool improvements and fixes for
COPY protocol handling. See release notes:
https://github.com/jackc/pgx/releases/tag/v5.5.0
```

## 4. Atomic Commits

Each commit should be ONE logical change that:
- Compiles on its own (`go build ./...` passes)
- Tests pass (`go test ./...` passes)
- Can be reverted independently without breaking other changes

### Split large changes:

```
# ❌ Bad — one commit doing everything
feat(user): add user management with CRUD, validation, auth, and tests

# ✅ Good — atomic, reviewable commits
feat(domain): add User entity and validation rules
feat(store): implement PostgreSQL user repository
feat(service): add user service with create and get operations
feat(handler): add REST endpoints for user management
test(service): add table-driven tests for user creation
docs(api): document user endpoints in OpenAPI spec
```

## 5. Pre-Commit Verification

Before committing, run:

```bash
# Format and lint
goimports -w .
golangci-lint run

# Build
go build ./...

# Test
go test -race ./...

# Tidy modules
go mod tidy
```

If any step fails, fix before committing. Never commit broken code
with "will fix later" — you won't.

## 6. Commit Workflow

```bash
# Stage specific files (not git add .)
git add internal/service/user.go
git add internal/service/user_test.go

# Review staged changes
git diff --staged

# Commit with message
git commit -m "feat(service): add user creation with email validation"

# Or use editor for longer messages
git commit  # opens $EDITOR
```

### Interactive rebase before PR:

```bash
# Clean up commit history before opening PR
git rebase -i main

# Squash fixup commits
# Reword unclear messages
# Reorder for logical flow
```

## 7. What NOT to Commit

- Generated files (`*.pb.go` unless required, `mocks/`)
- IDE configuration (`.idea/`, `.vscode/` — use global gitignore)
- OS files (`.DS_Store`, `Thumbs.db`)
- Binaries and build artifacts
- `.env` files with secrets
- `vendor/` (unless explicitly required by project policy)
