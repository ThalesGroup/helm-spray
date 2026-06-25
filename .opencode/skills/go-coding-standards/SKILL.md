---
name: go-coding-standards
description: >
  Go coding standards and style conventions grounded in Effective Go,
  Go Code Review Comments, and production-proven idioms.
  Use when writing or reviewing Go code, enforcing naming conventions, import ordering,
  variable declarations, struct initialization, or formatting rules.
  Trigger examples: "check Go style", "fix formatting", "review naming", "Go conventions".
  Do NOT use for architecture decisions, concurrency patterns, or performance tuning —
  use go-architecture-review, go-concurrency-review, or go-performance-review instead.
---

# Go Coding Standards

Idiomatic Go conventions grounded in Effective Go, Go Code Review Comments, and production-proven idioms.
All code MUST pass `goimports`, `golint`, and `go vet` without errors.

## 1. Import Ordering

Group imports in this order, separated by blank lines:

```go
import (
    // 1. Standard library
    "context"
    "fmt"
    "net/http"

    // 2. External packages
    "github.com/gorilla/mux"
    "log/slog"

    // 3. Internal/project packages
    "github.com/myorg/myproject/internal/service"
)
```

NEVER use dot imports. Use aliasing only to resolve conflicts.

## 2. Naming Conventions

### Packages
- Short, lowercase, single-word names. No underscores, no camelCase.
- Name should describe what the package *provides*, not what it *contains*.
- Avoid generic names: `util`, `common`, `helpers`, `misc`, `base`.

### Functions & Methods
- MixedCaps (exported) or mixedCaps (unexported). No underscores except in test files.
- Getters: use `Name()`, NOT `GetName()`. Setters: use `SetName()`.
- Constructors: `NewFoo()` returns `*Foo`. If only one type in package: `New()`.

### Variables
- Short names in tight scopes: `i`, `n`, `err`, `ctx`.
- Descriptive names for wider scopes: `userCount`, `retryTimeout`.
- Prefix unexported package-level globals with `_`: `var _defaultTimeout = 5 * time.Second`.
- Do NOT shadow built-in identifiers (`error`, `len`, `cap`, `new`, `make`, `close`).

### Interfaces
- Single-method interfaces: method name + `-er` suffix (`Reader`, `Writer`, `Closer`).
- Define interfaces where they are *consumed*, not where they are *implemented*.

## 3. Variable Declarations

### Top-level
Use `var` for top-level declarations. Do NOT specify type when it matches the expression:

```go
// ✅ Good
var _defaultPort = 8080
var _logger = slog.Default()

// ❌ Bad — redundant type
var _defaultPort int = 8080
```

### Local
- Prefer `:=` for local variables.
- Use `var` only when zero-value initialization is intentional and meaningful.

```go
// ✅ Good — zero value is meaningful
var buf bytes.Buffer

// ✅ Good — short declaration
name := getUserName()
```

## 4. Struct Initialization

ALWAYS use field names. Never rely on positional initialization:

```go
// ✅ Good
user := User{
    Name:  "Alice",
    Email: "alice@example.com",
    Age:   30,
}

// ❌ Bad — positional, breaks on field reordering
user := User{"Alice", "alice@example.com", 30}
```

Omit zero-value fields unless clarity requires them:

```go
// ✅ Good — zero values omitted
user := User{
    Name: "Alice",
}
```

## 5. Reduce Nesting

Handle errors and special cases first with early returns. Reduce indentation levels:

```go
// ✅ Good — early return
func process(data []Item) error {
    for _, v := range data {
        if !v.IsValid() {
            log.Printf("invalid item: %v", v)
            continue
        }

        if err := v.Process(); err != nil {
            return err
        }

        v.Send()
    }
    return nil
}
```

Eliminate unnecessary `else` blocks:

```go
// ✅ Good
a := 10
if condition {
    a = 20
}

// ❌ Bad
var a int
if condition {
    a = 20
} else {
    a = 10
}
```

## 6. Grouping and Ordering

Group related declarations:

```go
const (
    _defaultPort    = 8080
    _defaultTimeout = 30 * time.Second
)

var (
    _validTypes  = map[string]bool{"json": true, "xml": true}
    _defaultUser = User{Name: "guest"}
)
```

Function ordering within a file:
1. Constants and variables
2. `New()` / constructor functions
3. Exported methods (sorted by importance, not alphabetically)
4. Unexported methods
5. Helper functions

Receiver methods should appear immediately after the type declaration.

## 7. Line Length

Soft limit of 99 characters. Break long function signatures:

```go
func (s *Store) CreateUser(
    ctx context.Context,
    name string,
    email string,
    opts ...CreateOption,
) (*User, error) {
```

## 8. Defer Usage

Use `defer` for cleanup. It makes intent clear at the point of acquisition:

```go
mu.Lock()
defer mu.Unlock()

f, err := os.Open(path)
if err != nil {
    return err
}
defer f.Close()
```

## 9. Enums

Start enums at 1 (or use explicit sentinel) so zero-value signals "unset":

```go
type Status int

const (
    StatusUnknown Status = iota
    StatusActive
    StatusInactive
)
```

## 10. Use `time` Package Properly

- Use `time.Duration` for durations, NOT raw integers.
- Use `time.Time` for instants. Use `time.Since(start)` instead of `time.Now().Sub(start)`.
- External APIs: accept `int` or `float64` and convert internally.

```go
// ✅ Good
func poll(interval time.Duration) { ... }
poll(10 * time.Second)

// ❌ Bad
func poll(intervalSecs int) { ... }
poll(10)
```

## Verification Checklist

Before considering code complete:
1. `goimports` runs clean
2. `go vet ./...` passes
3. `golangci-lint run` passes (if configured)
4. No shadowed built-in identifiers
5. All imports properly grouped and ordered
6. Struct initializations use field names
7. No unnecessary nesting or else blocks
