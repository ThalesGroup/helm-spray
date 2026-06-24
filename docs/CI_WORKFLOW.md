# CI Workflow Contribution

## What was done

Added a continuous integration workflow and modernized the existing release workflow, which previously only ran on version-tag pushes with no test gate of any kind.

**Branch:** `feat/test-suite`  
**Commit:** `ci: add test workflow and modernize action versions to v4/v5`

---

## Files changed

| File | Change |
|---|---|
| `.github/workflows/ci.yaml` | Created — new CI pipeline |
| `.github/workflows/github-release.yaml` | Updated — modernized Go and action versions |

---

## New workflow: `ci.yaml`

```yaml
name: CI

on:
  push:
    branches:
      - '**'
  pull_request:

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Vet
        run: go vet ./...

      - name: Test
        run: go test ./...
```

### Trigger

Runs on every push to any branch and on every pull request. This means all future contributions get test feedback before merge.

### Steps

1. **Checkout** — full source at the pushed ref.
2. **Setup Go** — reads the version from `go.mod` (`go 1.24.0`). No hardcoded version to keep in sync.
3. **Vet** — runs `go vet ./...` to catch misuse of format strings, unreachable code, and other static issues.
4. **Test** — runs `go test ./...` against all 50 tests across the four packages that now have coverage.

---

## Updated workflow: `github-release.yaml`

### Changes

| Field | Before | After |
|---|---|---|
| `actions/checkout` | `@v2` | `@v4` |
| `actions/setup-go` | `@v2` | `@v5` |
| Go version | `'1.22'` (hardcoded) | `go-version-file: go.mod` |

The release workflow now uses the same Go version as the rest of the project (1.24) and modern action versions that receive security updates. Switching to `go-version-file` means the release and CI workflows stay in sync with `go.mod` automatically.

---

## Why this matters

Before this change, helm-spray had:
- No test files
- No CI pipeline
- A release workflow running on a Go version behind the declared minimum

After this change, every push and pull request runs the full test suite automatically, giving contributors immediate feedback and preventing regressions from being merged.
