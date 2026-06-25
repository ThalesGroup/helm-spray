---
name: go-code-review
description: >
  Comprehensive code review checklist for Go projects. Evaluates code quality,
  idiomatic patterns, error handling, naming, package structure, and test coverage.
  Use when reviewing Go code, PRs, or before merging changes.
  Trigger examples: "review this code", "check this PR", "code review", "review Go file".
  Do NOT use for security-specific audits (use go-security-audit) or
  performance-specific analysis (use go-performance-review).
---

# Go Code Review

Structured code review process for Go. Reviews should be constructive, specific,
and cite the relevant principle behind each finding.

## Review Process

Execute these steps in order. For each finding, classify severity:
- 🔴 **BLOCKER** — Must fix before merge. Correctness, data loss, security.
- 🟡 **WARNING** — Should fix. Maintainability, idiomatic Go, clarity.
- 🟢 **SUGGESTION** — Consider improving. Style, naming, documentation.

## 1. Correctness & Safety

### Error Handling
- Every error is checked. No blank identifier `_` discarding errors silently.
- Errors are wrapped with context: `fmt.Errorf("fetch user %d: %w", id, err)`.
- Error values compared with `errors.Is()` / `errors.As()`, never `==`.
- No `panic` outside of `init()` or truly unrecoverable situations.
- Errors handled exactly once — no log-and-return patterns.

### Nil Safety
- Pointer receivers checked before dereference when nil is a valid state.
- Map reads guarded or use comma-ok idiom.
- Channel operations consider closed/nil channels.
- Slice operations check bounds where relevant.

### Concurrency
- Shared mutable state protected by `sync.Mutex` or channels.
- No goroutine leaks — every goroutine has a clear termination path.
- Context propagation: all blocking calls accept and respect `context.Context`.
- `sync.WaitGroup` or `errgroup.Group` used for goroutine lifecycle.

## 2. API Design

- Exported functions have doc comments starting with the function name.
- Accept interfaces, return concrete types.
- Use functional options (`WithTimeout(d)`) over config structs for optional params.
- Context is always the first parameter: `func Foo(ctx context.Context, ...)`.
- Return `error` as the last return value.
- Avoid `bool` parameters — prefer named types or options.

## 3. Idiomatic Go

- Uses `:=` for local variables, `var` for zero-value intent.
- No unnecessary `else` after return/continue/break.
- Guard clauses and early returns reduce nesting.
- `defer` used for cleanup, placed right after resource acquisition.
- `range` used over manual index iteration where appropriate.
- Struct literals use field names.
- Interfaces defined at consumer, not producer.

## 4. Package Structure

- Package names are short, lowercase, singular nouns.
- No circular dependencies between packages.
- `internal/` used for non-public packages.
- `cmd/` contains main packages, one per binary.
- Clear separation of concerns — no god packages.

## 5. Testing

- Test functions follow `TestXxx` naming convention.
- Table-driven tests used for multiple input/output combinations.
- Test helpers use `t.Helper()` for clean stack traces.
- No test logic in `init()` — use `TestMain` when needed.
- Tests use `testify/assert` or `testify/require` consistently, or stdlib only.
- Edge cases covered: empty input, nil, zero values, max values.
- `t.Parallel()` used where safe.

## 6. Documentation

- All exported types, functions, and constants have doc comments.
- Doc comments start with the name of the entity.
- Package-level doc comment in `doc.go` for non-trivial packages.
- Complex algorithms or business logic have inline comments explaining *why*.

## 7. Dependencies

- `go.mod` has no replace directives in committed code (except monorepos).
- No unused dependencies.
- Dependencies are from well-maintained, reputable sources.
- Indirect dependencies are understood and acceptable.

## Review Output Format

```
## Code Review Summary

**Files reviewed:** <list>
**Overall assessment:** APPROVE | REQUEST CHANGES | COMMENT

### Findings

#### 🔴 BLOCKER: <title>
- **File:** `path/to/file.go:42`
- **Issue:** <what is wrong>
- **Why:** <which principle or guideline>
- **Fix:** <concrete suggestion>

#### 🟡 WARNING: <title>
...

#### 🟢 SUGGESTION: <title>
...

### What's Done Well
<genuine positive observations — always include at least one>
```
