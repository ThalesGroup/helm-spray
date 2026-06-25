---
name: go-interface-design
description: >
  Go interface design patterns: implicit interfaces, consumer-side definition,
  interface compliance verification, composition, the accept-interfaces-return-structs
  principle, and common pitfalls.
  Use when designing interfaces, decoupling packages, defining contracts,
  reviewing interface usage, or refactoring for testability.
  Trigger examples: "design interface", "accept interfaces return structs",
  "interface compliance", "consumer-side interface", "interface composition".
  Do NOT use for HTTP handler patterns (use go-api-design) or
  general code review (use go-code-review).
---

# Go Interface Design

Go interfaces are implicit. This is the single most important design feature
of the language, and most people coming from Java or C# get it wrong at first.

## 1. The Cardinal Rule: Define Interfaces at the Consumer

The consumer of a behavior defines the interface, NOT the provider:

```go
// ❌ Wrong — producer defines interface (Java thinking)
// package store
type UserStore interface {      // defined alongside implementation
    GetByID(ctx context.Context, id string) (*User, error)
    Create(ctx context.Context, user *User) error
    // ... 15 more methods
}

type PostgresStore struct { ... }
func (s *PostgresStore) GetByID(...) { ... }
func (s *PostgresStore) Create(...) { ... }

// ✅ Right — consumer defines what it needs
// package service
type UserReader interface {     // only what THIS service needs
    GetByID(ctx context.Context, id string) (*domain.User, error)
}

type UserService struct {
    store UserReader  // depends on narrow interface
}

// package store (no interface defined here)
type PostgresStore struct { db *sql.DB }
func (s *PostgresStore) GetByID(ctx context.Context, id string) (*domain.User, error) { ... }
func (s *PostgresStore) Create(ctx context.Context, user *domain.User) error { ... }

// PostgresStore satisfies service.UserReader implicitly — no declaration needed
```

Why this matters:
- Consumer depends only on what it uses (Interface Segregation Principle).
- Producer can add methods without breaking consumers.
- Testing requires only the methods the consumer calls.
- No import cycle: consumer doesn't import producer's package.

## 2. Keep Interfaces Small

The bigger the interface, the weaker the abstraction.

```go
// ✅ Good — focused, composable
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}

type ReadWriter interface {
    Reader
    Writer
}

// ❌ Bad — kitchen sink interface
type FileManager interface {
    Read(path string) ([]byte, error)
    Write(path string, data []byte) error
    Delete(path string) error
    List(dir string) ([]string, error)
    Move(src, dst string) error
    Copy(src, dst string) error
    Stat(path string) (os.FileInfo, error)
    Watch(path string) (<-chan Event, error)
}
```

Guideline: 1-3 methods is ideal. If you need more, compose smaller interfaces.

## 3. Accept Interfaces, Return Structs

```go
// ✅ Good — accepts interface, returns concrete type
func NewUserService(store UserReader, logger Logger) *UserService {
    return &UserService{store: store, logger: logger}
}

// ❌ Bad — returns interface (hides the concrete type for no reason)
func NewUserService(store UserReader) UserServiceInterface {
    return &UserService{store: store}
}
```

Return a concrete type so callers get full access to the type's methods.
Returning an interface only makes sense when the function genuinely
returns different concrete types based on input (factory pattern).

## 4. Verify Interface Compliance at Compile Time

Use the blank identifier assignment to catch broken contracts early:

```go
// Verify *PostgresStore implements service.UserReader at compile time
var _ service.UserReader = (*PostgresStore)(nil)

// Verify LogHandler implements http.Handler
var _ http.Handler = (*LogHandler)(nil)

// For value receivers:
var _ fmt.Stringer = Status(0)
```

Place these immediately after the type declaration. They cost nothing
at runtime and prevent silent contract breakage.

## 5. Don't Use Pointers to Interfaces

```go
// ❌ Bad — pointer to interface is almost never correct
func process(r *io.Reader) { ... }

// ✅ Good — interface is already a pointer internally
func process(r io.Reader) { ... }
```

An interface value is internally two pointers (type + data).
A pointer to an interface is a pointer to a pointer — needless indirection.

The only exception: when you need to replace the interface value itself
(swap the implementation at runtime), which is extremely rare.

## 6. The Empty Interface

`interface{}` (or `any` in Go 1.18+) means you've given up on type safety.
Use it sparingly:

```go
// ✅ Acceptable — generic container before generics / stdlib compatibility
func Marshal(v any) ([]byte, error)

// ✅ Better (Go 1.18+) — use generics instead of any
func Map[T, U any](slice []T, fn func(T) U) []U { ... }

// ❌ Bad — lazy interface design
func Process(data any) any { ... } // what does this even do?
```

## 7. Functional Options Pattern

When a constructor needs optional configuration, use functional options
instead of a config struct with an interface:

```go
type Option func(*Server)

func WithTimeout(d time.Duration) Option {
    return func(s *Server) { s.timeout = d }
}

func WithLogger(l Logger) Option {
    return func(s *Server) { s.logger = l }
}

func NewServer(addr string, opts ...Option) *Server {
    s := &Server{
        addr:    addr,
        timeout: 30 * time.Second,  // sensible default
        logger:  slog.Default(),    // default stdlib logger
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage
srv := NewServer(":8080",
    WithTimeout(60 * time.Second),
    WithLogger(logger),
)
```

## 8. Common Interface Anti-Patterns

### Premature interfaces:

```go
// ❌ Bad — interface defined before second implementation exists
type Processor interface {
    Process(ctx context.Context, data []byte) error
}

type processor struct { ... }  // only one implementation ever

// ✅ Good — use concrete type until you need the abstraction
type Processor struct { ... }
// Add interface when you have 2+ implementations or need testing seam
```

"Don't design with interfaces, discover them." — Rob Pike

### Interface pollution:

```go
// ❌ Bad — wrapping every struct in an interface "for testability"
type UserServiceInterface interface { ... }
type OrderServiceInterface interface { ... }
type PaymentServiceInterface interface { ... }
// 50 more interfaces with exactly one implementation each

// ✅ Good — define interfaces where they're consumed
// Each consumer declares only the methods IT needs
```

### Misusing interfaces for enums:

```go
// ❌ Bad — interface used as enum/sum type
type Shape interface {
    isShape()
}
type Circle struct{}
func (Circle) isShape() {}

// ✅ Better — sealed interface pattern (if you need it)
// Or just use constants with a type
type ShapeKind int
const (
    ShapeCircle ShapeKind = iota
    ShapeRectangle
)
```

## Decision Checklist

1. **Do I need an interface here?** — Only if you have 2+ implementations,
   need a testing seam, or are crossing a package boundary.
2. **Where should it be defined?** — At the consumer, not the producer.
3. **How many methods?** — Fewer is better. 1-3 is ideal.
4. **Am I returning an interface?** — Probably shouldn't. Return concrete.
5. **Have I verified compliance?** — `var _ Interface = (*Type)(nil)`
