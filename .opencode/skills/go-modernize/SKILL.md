---
name: go-modernize
description: >
  Modernize Go code to use current language features and standard library additions.
  Covers generics, log/slog, errors.Join, slices/maps packages, range-over-func,
  and iterators introduced in Go 1.21-1.23+.
  Use when: "modernize", "update to modern Go", "use generics", "replace interface{}",
  "upgrade Go version", "slog", "errors.Join", "range over func", "iterators".
  Do NOT use for: general code style (use go-coding-standards),
  error handling philosophy (use go-error-handling), or
  logging architecture (use go-observability).
---

# Go Modernize

Go evolves. Code written for Go 1.16 should not look the same as code targeting
Go 1.22+. Modernize incrementally — update `go.mod`, then adopt new patterns.

## 1. Generics (Go 1.18+)

### Replace `interface{}` / `any` with type parameters where appropriate:

```go
// ❌ Before — loses type safety
func Contains(slice []interface{}, target interface{}) bool {
    for _, v := range slice {
        if v == target {
            return true
        }
    }
    return false
}

// ✅ After — type-safe generic
func Contains[T comparable](slice []T, target T) bool {
    for _, v := range slice {
        if v == target {
            return true
        }
    }
    return false
}
```

### Use type constraints effectively:

```go
// Built-in constraints
func Sum[T int | int64 | float64](values []T) T {
    var total T
    for _, v := range values {
        total += v
    }
    return total
}

// Or use golang.org/x/exp/constraints (or define your own)
type Number interface {
    ~int | ~int8 | ~int16 | ~int32 | ~int64 |
    ~float32 | ~float64
}

func Sum[T Number](values []T) T {
    var total T
    for _, v := range values {
        total += v
    }
    return total
}
```

### When NOT to use generics:

```go
// ❌ Don't use generics when a single concrete type works
func PrintUser[T User](u T) { fmt.Println(u.Name) }
// → Just use: func PrintUser(u User) { fmt.Println(u.Name) }

// ❌ Don't use generics to avoid interfaces for behavior polymorphism
// Interfaces are still the right tool for runtime polymorphism

// ✅ Use generics for:
// - Container types (Set[T], Stack[T], Result[T])
// - Utility functions operating on multiple types (Map, Filter, Reduce)
// - Type-safe wrappers (sync pool, atomic values)
```

### Generic container example:

```go
type Set[T comparable] struct {
    items map[T]struct{}
}

func NewSet[T comparable](items ...T) Set[T] {
    s := Set[T]{items: make(map[T]struct{}, len(items))}
    for _, item := range items {
        s.items[item] = struct{}{}
    }
    return s
}

func (s Set[T]) Contains(item T) bool {
    _, ok := s.items[item]
    return ok
}

func (s Set[T]) Add(item T) {
    s.items[item] = struct{}{}
}
```

## 2. Structured Logging with log/slog (Go 1.21+)

### Replace log/fmt.Printf with slog:

```go
// ❌ Before
log.Printf("processing order %s for user %s", orderID, userID)

// ✅ After
slog.Info("processing order",
    slog.String("order_id", orderID),
    slog.String("user_id", userID),
)
```

### Replace third-party loggers where slog is sufficient:

```go
// Before — zap
logger.Info("request completed",
    zap.String("method", method),
    zap.Int("status", status),
    zap.Duration("latency", elapsed),
)

// After — slog (if you don't need zap-specific features)
slog.Info("request completed",
    slog.String("method", method),
    slog.Int("status", status),
    slog.Duration("latency", elapsed),
)
```

Keep zap/zerolog if you need their performance characteristics for
high-throughput logging. For most services, slog is sufficient.

## 3. errors.Join (Go 1.20+)

### Combine multiple errors:

```go
// ❌ Before — manual error accumulation
var errMsgs []string
for _, item := range items {
    if err := validate(item); err != nil {
        errMsgs = append(errMsgs, err.Error())
    }
}
if len(errMsgs) > 0 {
    return fmt.Errorf("validation: %s", strings.Join(errMsgs, "; "))
}

// ✅ After — errors.Join preserves the error chain
var errs []error
for _, item := range items {
    if err := validate(item); err != nil {
        errs = append(errs, err)
    }
}
if err := errors.Join(errs...); err != nil {
    return fmt.Errorf("validation: %w", err)
}
```

`errors.Join` preserves the full error chain — `errors.Is` and `errors.As`
work on each individual error.

## 4. slices and maps Packages (Go 1.21+)

### Replace hand-written slice operations:

```go
// ❌ Before — manual sort
sort.Slice(users, func(i, j int) bool {
    return users[i].Name < users[j].Name
})

// ✅ After — slices.SortFunc
slices.SortFunc(users, func(a, b User) int {
    return cmp.Compare(a.Name, b.Name)
})
```

```go
// ❌ Before — manual contains check
found := false
for _, v := range items {
    if v == target {
        found = true
        break
    }
}

// ✅ After
found := slices.Contains(items, target)
```

```go
// ❌ Before — manual index search
idx := -1
for i, v := range items {
    if v.ID == targetID {
        idx = i
        break
    }
}

// ✅ After
idx := slices.IndexFunc(items, func(item Item) bool {
    return item.ID == targetID
})
```

### Replace hand-written map operations:

```go
// ❌ Before — manual key collection
keys := make([]string, 0, len(m))
for k := range m {
    keys = append(keys, k)
}

// ✅ After
keys := slices.Collect(maps.Keys(m))
```

```go
// ❌ Before — manual map clone
clone := make(map[string]int, len(m))
for k, v := range m {
    clone[k] = v
}

// ✅ After
clone := maps.Clone(m)
```

## 5. Range Over Integers (Go 1.22+)

```go
// ❌ Before
for i := 0; i < n; i++ {
    process(i)
}

// ✅ After
for i := range n {
    process(i)
}
```

## 6. Range Over Function / Iterators (Go 1.23+)

### Use iter.Seq for custom iteration:

```go
// ✅ Iterator that yields filtered results
func (db *DB) ActiveUsers(ctx context.Context) iter.Seq2[User, error] {
    return func(yield func(User, error) bool) {
        rows, err := db.QueryContext(ctx, "SELECT id, name FROM users WHERE active = true")
        if err != nil {
            yield(User{}, fmt.Errorf("query active users: %w", err))
            return
        }
        defer rows.Close()

        for rows.Next() {
            var u User
            if err := rows.Scan(&u.ID, &u.Name); err != nil {
                if !yield(User{}, fmt.Errorf("scan user: %w", err)) {
                    return
                }
                continue
            }
            if !yield(u, nil) {
                return
            }
        }
        if err := rows.Err(); err != nil {
            yield(User{}, fmt.Errorf("iterate users: %w", err))
        }
    }
}

// Usage — clean range loop
for user, err := range db.ActiveUsers(ctx) {
    if err != nil {
        return fmt.Errorf("active users: %w", err)
    }
    process(user)
}
```

### Standard library iterators — use them:

```go
// maps.Keys, maps.Values return iterators (Go 1.23+)
for key := range maps.Keys(m) {
    fmt.Println(key)
}

// slices.All, slices.Values, slices.Backward
for i, v := range slices.Backward(items) {
    fmt.Printf("%d: %v\n", i, v)
}
```

## 7. http.NewRequestWithContext (Go 1.13+, but often missed)

```go
// ❌ Before — request without context
req, err := http.NewRequest(http.MethodGet, url, nil)

// ✅ After — context propagated
req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
```

## 8. Modernization Checklist by Go Version

| Go Version | Feature | Action |
|---|---|---|
| 1.13+ | `errors.Is`, `errors.As` | Replace `==` error comparisons |
| 1.13+ | `http.NewRequestWithContext` | Replace `http.NewRequest` |
| 1.16+ | `embed` | Replace `go-bindata` / `packr` |
| 1.18+ | Generics | Replace `interface{}` utility functions |
| 1.20+ | `errors.Join` | Replace manual error accumulation |
| 1.21+ | `log/slog` | Replace `log` for structured logging |
| 1.21+ | `slices`, `maps` | Replace hand-written slice/map utilities |
| 1.21+ | `min`, `max` builtins | Replace `math.Min`/`math.Max` (float64-only) |
| 1.22+ | Range over int | Replace `for i := 0; i < n; i++` |
| 1.23+ | Range over func | Replace callback-based iteration |

## Verification Checklist

1. `go.mod` version matches the features used in the codebase
2. No `interface{}` where `any` or type parameters would be clearer
3. `log/slog` used instead of `log.Printf` for structured logging
4. `errors.Join` used instead of manual error string concatenation
5. `slices.Contains`, `slices.SortFunc`, `maps.Clone` replace hand-written loops
6. Range over int (`for i := range n`) used where applicable
7. `http.NewRequestWithContext` used instead of `http.NewRequest`
8. No `sort.Slice` — use `slices.SortFunc` with `cmp.Compare`
9. Generics used for type-safe containers and utilities, not overused for trivial cases
10. Third-party dependencies evaluated against stdlib alternatives added in recent Go versions
