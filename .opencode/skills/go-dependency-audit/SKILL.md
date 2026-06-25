---
name: go-dependency-audit
description: >
  Audit Go module dependencies: detect outdated packages, check for known
  vulnerabilities, review go.mod hygiene, identify unused or redundant deps,
  and evaluate dependency quality.
  Use when auditing dependencies, checking for CVEs, cleaning up go.mod,
  upgrading modules, or evaluating third-party packages.
  Trigger examples: "check dependencies", "audit deps", "go.mod review",
  "update modules", "vulnerability scan", "govulncheck".
  Do NOT use for code-level security issues (use go-security-audit) or
  architecture review (use go-architecture-review).
---

# Go Dependency Audit

Every dependency you add is code you don't control but are responsible for.
Audit ruthlessly.

## 1. Vulnerability Scanning

### govulncheck (official Go tool):

```bash
# Install
go install golang.org/x/vuln/cmd/govulncheck@latest

# Scan project
govulncheck ./...

# Scan binary
govulncheck -mode=binary ./cmd/api-server
```

`govulncheck` checks against the Go vulnerability database and reports
only vulnerabilities that actually affect your code paths — not just
transitive deps you never call.

Run this in CI. No exceptions.

### Additional scanning:

```bash
# Nancy (Sonatype OSS Index)
go list -json -deps ./... | nancy sleuth

# Trivy (container + deps)
trivy fs --scanners vuln .
```

## 2. go.mod Hygiene

### Check for unused dependencies:

```bash
go mod tidy
git diff go.mod go.sum  # any changes = deps were stale
```

`go mod tidy` MUST be run before every commit. Add to CI:

```bash
go mod tidy
git diff --exit-code go.mod go.sum
```

### No replace directives in committed code:

```go
// ❌ Bad — committed replace directive
replace github.com/foo/bar => ../local-bar

// ✅ Acceptable — in monorepos with workspace
// go.work handles this instead
```

Exception: temporary replace for bug fixes with a comment and linked issue:

```go
// TODO(#1234): remove after upstream merges fix
replace github.com/foo/bar => github.com/myorg/bar v0.0.0-fix
```

### Verify checksums:

```bash
go mod verify
```

This confirms that downloaded modules match their expected checksums.
Failures indicate supply-chain tampering.

## 3. Dependency Evaluation Criteria

Before adding any dependency, evaluate:

| Criterion | Check |
|---|---|
| **Maintenance** | Last commit < 6 months? Active issue responses? |
| **Popularity** | Stars/forks alone mean nothing. Usage in production projects matters. |
| **License** | Compatible with your project? MIT/Apache/BSD preferred. |
| **Size** | Does it pull in 50 transitive deps for one function? |
| **Alternatives** | Can you do this with stdlib in < 50 lines? |
| **API stability** | Is it v1+? Does it follow semver? Frequent breaking changes? |
| **Test coverage** | Does the project have meaningful tests? |

### The stdlib question:

Go's standard library is excellent. Before adding a dependency, ask:
"Can I solve this with `net/http`, `encoding/json`, `database/sql`,
`text/template`, `crypto/*`, `os/exec`, etc.?"

If the answer is yes and the code is < 100 lines, write it yourself.

## 4. Module Version Audit

### List all dependencies with versions:

```bash
go list -m all
```

### Check for available updates:

```bash
go list -m -u all  # shows available updates
```

### Upgrade strategy:

```bash
# Update specific module
go get github.com/foo/bar@latest

# Update all direct deps (minor/patch only)
go get -u ./...

# Update all deps including major versions (dangerous)
go get -u -t ./...
```

ALWAYS run full test suite after updates:

```bash
go get github.com/foo/bar@v1.5.0
go mod tidy
go test -race ./...
```

## 5. Transitive Dependency Analysis

```bash
# Why is this module in my dependency tree?
go mod why github.com/some/transitive-dep

# Full dependency graph
go mod graph

# Visual dependency graph (with modgraphviz)
go mod graph | modgraphviz | dot -Tpng -o deps.png
```

Watch for:
- 🔴 Transitive deps with known CVEs
- 🔴 Abandoned transitive deps (no commits in 2+ years)
- 🟡 Diamond dependency conflicts (two versions of same module)
- 🟡 Oversized transitive trees (a logging library pulling in gRPC)

## 6. Go Version Management

```go
// go.mod
module github.com/myorg/myproject

go 1.22  // minimum Go version required
```

Rules:
- Set `go` directive to the minimum version that supports features you use.
- `toolchain` directive (Go 1.21+) pins the exact toolchain version.
- Test against multiple Go versions in CI (at minimum: current and previous).

## 7. Recommended vs. Avoid

### Well-maintained, production-proven packages:

| Domain | Package |
|---|---|
| Logging | `go.uber.org/zap`, `log/slog` (stdlib 1.21+) |
| HTTP Router | `github.com/go-chi/chi`, `net/http` (1.22+ routing) |
| Config | `github.com/caarlos0/env`, `github.com/spf13/viper` |
| Testing | `github.com/stretchr/testify`, stdlib `testing` |
| Database | `github.com/jackc/pgx`, `github.com/jmoiron/sqlx` |
| Validation | `github.com/go-playground/validator` |
| UUID | `github.com/google/uuid` |
| Errors | `go.uber.org/multierr`, stdlib `errors` (1.20+) |

### Patterns to avoid:

- ❌ Frameworks that take over `main()` (Go is not Java Spring)
- ❌ ORMs that hide SQL (prefer `sqlx` or raw `database/sql`)
- ❌ Code generators you don't understand
- ❌ Packages with `v0.x` that have been v0 for 3+ years

## Audit Output Format

```
## Dependency Audit Report

**Module:** github.com/myorg/myproject
**Go version:** 1.22
**Direct deps:** N | **Indirect deps:** M

### 🔴 Vulnerabilities
- CVE-XXXX-YYYY in github.com/foo/bar@v1.2.3 — upgrade to v1.2.5

### 🟡 Outdated Dependencies
- github.com/foo/bar v1.2.3 → v1.5.0 available (minor)

### 🟢 Observations
- go.mod is clean, no replace directives
- All deps actively maintained
```
