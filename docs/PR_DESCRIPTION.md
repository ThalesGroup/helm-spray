# PR: Add test suite and CI pipeline

## Summary

This PR adds the first test suite to helm-spray (previously 0 test files) and wires it into a new CI workflow that runs on every push and PR.

---

## Changes

### 50 new tests across 4 packages

| Package | Tests | What's covered |
|---|---|---|
| `internal/dependencies` | 16 | Weight parsing, alias resolution, target/exclude filtering, tag gating, release prefix |
| `internal/values` | 14 | `#! .Files.Get` include directives, `pick`, `indent`, missing file errors, map merging |
| `pkg/helmspray` | 12 | `maxWeight`, `checkTargetsAndExcludes` |
| `pkg/kubectl` | 6 | `generateTemplate` output structure |

### New CI workflow (`.github/workflows/ci.yaml`)

Runs `go vet` + `go test ./...` on every push and pull request.

### Modernized release workflow

- `actions/checkout` and `actions/setup-go` updated from `v2` to `v4`/`v5`
- Hardcoded Go `1.22` replaced with `go-version-file: go.mod` (currently `1.24`)

### Bug fix: `internal/dependencies/dependencies.go`

`go vet` was failing on a double-format call (`fmt.Sprintf` result passed as format string to `log.Info`). Fixed as part of unblocking test compilation.

---

## How to verify

```bash
go test ./...
```

All 50 tests pass.
