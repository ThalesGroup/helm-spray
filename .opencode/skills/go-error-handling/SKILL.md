---
name: go-error-handling
description: >
  Go error handling patterns, wrapping, sentinel errors, custom error types,
  and the errors package. Grounded in Effective Go, Go Code Review Comments, and production-proven idioms.
  Use when implementing error handling, designing error types, debugging error chains,
  or reviewing error handling patterns.
  Trigger examples: "handle errors", "error wrapping", "custom error type",
  "sentinel errors", "errors.Is", "errors.As".
  Do NOT use for panic/recover patterns in middleware (use go-api-design)
  or test assertion errors (use go-test-quality).
---

# Go Error Handling

Go's explicit error handling is a feature, not a limitation.
These patterns ensure errors are informative, actionable, and properly propagated.

## 1. Error Decision Tree

When creating or returning an error, follow this tree:

1. **Simple, no extra context needed?** → `errors.New("message")`
2. **Need to add context to existing error?** → `fmt.Errorf("doing X: %w", err)`
3. **Caller needs to detect this error?** → Sentinel `var` or custom type
4. **Error carries structured data?** → Custom type implementing `error`
5. **Propagating from downstream?** → Wrap with `%w` and add context

## 2. Sentinel Errors

Use package-level `var` for errors that callers need to check:

```go
// ✅ Good — exported sentinel error
var (
    ErrNotFound     = errors.New("user: not found")
    ErrUnauthorized = errors.New("user: unauthorized")
)

// Naming convention: Err + Description
// Prefix with package context in the message
```

Callers check with `errors.Is`:

```go
if errors.Is(err, user.ErrNotFound) {
    // handle not found
}
```

NEVER compare errors with `==`. Always use `errors.Is()`.

## 3. Custom Error Types

When errors need to carry structured information:

```go
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation: field %s: %s", e.Field, e.Message)
}

// Callers extract with errors.As:
var valErr *ValidationError
if errors.As(err, &valErr) {
    log.Printf("invalid field: %s", valErr.Field)
}
```

## 4. Error Wrapping

ALWAYS add context when propagating errors up the stack.
Use `%w` to preserve the error chain:

```go
// ✅ Good — context added, chain preserved
func getUser(id int64) (*User, error) {
    row, err := db.QueryRow(ctx, query, id)
    if err != nil {
        return nil, fmt.Errorf("get user %d: %w", id, err)
    }
    // ...
}

// ❌ Bad — no context
return nil, err

// ❌ Bad — chain broken, callers can't errors.Is/As
return nil, fmt.Errorf("failed: %v", err)
```

### When NOT to use `%w`

Use `%v` instead of `%w` when you explicitly want to **break** the error chain,
preventing callers from depending on internal implementation errors:

```go
// Intentionally hiding internal DB error from public API
return nil, fmt.Errorf("user lookup failed: %v", err)
```

## 5. Handle Errors Exactly Once

An error should be either logged OR returned, never both:

```go
// ✅ Good — return the error, let caller decide
func loadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("load config %s: %w", path, err)
    }
    // ...
}

// ❌ Bad — log AND return (error handled twice)
func loadConfig(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        log.Printf("failed to read config: %v", err) // handled once
        return nil, err                                 // handled again
    }
    // ...
}
```

The rule: the component that *decides what to do* about the error is the one
that logs/metrics it. Everyone else wraps and returns.

## 6. Error Naming Conventions

```go
// Sentinel errors: Err prefix
var ErrNotFound = errors.New("not found")

// Error types: Error suffix
type NotFoundError struct { ... }
type ValidationError struct { ... }

// Error messages: lowercase, no punctuation, no "failed to" prefix
// Include context: "package: action: detail"
errors.New("auth: token expired")
fmt.Errorf("user: get by id %d: %w", id, err)
```

## 7. Panic Rules

Panic is NOT error handling. Use panic only when:
- Program initialization fails and cannot continue (`template.Must`, flag parsing)
- Programmer error that should never happen (violated invariant)
- Nil dereference that indicates a bug, not a runtime condition

In tests, use `t.Fatal` / `t.FailNow`, never `panic`.

In HTTP handlers and middleware, recover from panics at the boundary
to prevent one request from crashing the server.

## 8. Error Checking Patterns

```go
// Inline error check — preferred for simple cases
if err := doSomething(); err != nil {
    return fmt.Errorf("do something: %w", err)
}

// Multi-return with named result — acceptable for complex functions
func process() (result string, err error) {
    defer func() {
        if err != nil {
            err = fmt.Errorf("process: %w", err)
        }
    }()
    // ...
}

// errors.Join for multiple errors (Go 1.20+)
var errs []error
for _, item := range items {
    if err := validate(item); err != nil {
        errs = append(errs, err)
    }
}
return errors.Join(errs...)
```

## Verification Checklist

1. No `_` discarding errors (unless explicitly justified with comment)
2. Every `fmt.Errorf` wrapping uses `%w` (or `%v` with documented reason)
3. Sentinel errors use `var Err...` naming
4. Custom error types implement `error` interface
5. Callers use `errors.Is` / `errors.As`, never `==` or type assertion
6. No log-and-return patterns
7. Error messages are lowercase, contextual, chain-friendly
