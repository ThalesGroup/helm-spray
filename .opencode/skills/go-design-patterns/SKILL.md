---
name: go-design-patterns
description: >
  Idiomatic Go design patterns: functional options, builder, factory, strategy,
  middleware chain, pub/sub, and other patterns adapted for Go's type system.
  Use when: "design pattern", "functional options", "builder pattern",
  "factory pattern", "strategy pattern", "middleware chain", "option pattern",
  "how to structure this".
  Do NOT use for: interface design principles (use go-interface-design),
  package layout (use go-architecture-review), or
  concurrency patterns (use go-concurrency-review).
---

# Go Design Patterns

Go favors composition over inheritance and simplicity over abstraction.
These patterns are idiomatic Go — not Java patterns ported to Go.

## 1. Functional Options

The most idiomatic Go pattern for configurable constructors.
Use when a type has many optional settings.

```go
type Server struct {
    addr         string
    readTimeout  time.Duration
    writeTimeout time.Duration
    logger       *slog.Logger
}

type Option func(*Server)

func WithAddr(addr string) Option {
    return func(s *Server) {
        s.addr = addr
    }
}

func WithReadTimeout(d time.Duration) Option {
    return func(s *Server) {
        s.readTimeout = d
    }
}

func WithLogger(l *slog.Logger) Option {
    return func(s *Server) {
        s.logger = l
    }
}

func NewServer(opts ...Option) *Server {
    s := &Server{
        addr:         ":8080",       // sensible defaults
        readTimeout:  5 * time.Second,
        writeTimeout: 10 * time.Second,
        logger:       slog.Default(),
    }
    for _, opt := range opts {
        opt(s)
    }
    return s
}

// Usage:
srv := NewServer(
    WithAddr(":9090"),
    WithReadTimeout(10*time.Second),
)
```

### When to use functional options vs config struct:

```go
// Use functional options when:
// - Many optional parameters with sensible defaults
// - API evolves over time (new options don't break callers)
// - Options need validation or side effects

// Use config struct when:
// - Most fields are required
// - Configuration is loaded from file/env (easy to deserialize)
// - No need for default values
type Config struct {
    Addr     string        `yaml:"addr"`
    DBUrl    string        `yaml:"db_url"`
    LogLevel slog.Level    `yaml:"log_level"`
}
```

## 2. Constructor Pattern

Every exported type with invariants needs a constructor.

```go
// ✅ Good — constructor enforces invariants
func NewUserService(repo UserRepository, logger *slog.Logger) (*UserService, error) {
    if repo == nil {
        return nil, errors.New("user service: nil repository")
    }
    if logger == nil {
        return nil, errors.New("user service: nil logger")
    }
    return &UserService{repo: repo, logger: logger}, nil
}

// ❌ Bad — struct literal with no validation
svc := &UserService{} // nil dependencies → panic at runtime
```

### Return error from constructor when validation is needed:

```go
// ✅ Good — constructor returns error
func NewEmailAddress(raw string) (EmailAddress, error) {
    if !isValidEmail(raw) {
        return EmailAddress{}, fmt.Errorf("invalid email: %s", raw)
    }
    return EmailAddress{value: raw}, nil
}
```

## 3. Factory Pattern

Use when you need to create different implementations of an interface
based on runtime configuration.

```go
type Store interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key, value string) error
}

func NewStore(cfg Config) (Store, error) {
    switch cfg.StoreType {
    case "redis":
        return newRedisStore(cfg.RedisAddr)
    case "memory":
        return newMemoryStore(), nil
    case "postgres":
        return newPostgresStore(cfg.DatabaseURL)
    default:
        return nil, fmt.Errorf("unknown store type: %s", cfg.StoreType)
    }
}
```

Return the interface, not a concrete type. The factory is the only place
that knows about concrete implementations.

## 4. Strategy Pattern

Swap behavior at runtime by injecting function types or interfaces.

### With function types (simpler):

```go
type RetryStrategy func(attempt int) time.Duration

func ExponentialBackoff(base time.Duration) RetryStrategy {
    return func(attempt int) time.Duration {
        return base * time.Duration(1<<uint(attempt))
    }
}

func ConstantDelay(d time.Duration) RetryStrategy {
    return func(_ int) time.Duration {
        return d
    }
}

func Retry(ctx context.Context, maxAttempts int, strategy RetryStrategy, fn func() error) error {
    var err error
    for i := 0; i < maxAttempts; i++ {
        if err = fn(); err == nil {
            return nil
        }
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(strategy(i)):
        }
    }
    return fmt.Errorf("after %d attempts: %w", maxAttempts, err)
}
```

### With interfaces (when behavior is complex):

```go
type Notifier interface {
    Notify(ctx context.Context, event Event) error
}

type SlackNotifier struct { webhookURL string }
type EmailNotifier struct { smtpClient *smtp.Client }
type NoopNotifier  struct{}

// Each implements Notifier. Inject the right one at startup.
```

## 5. Middleware / Decorator Pattern

Wrap behavior around a core function or interface.

### HTTP middleware (standard pattern):

```go
type Middleware func(http.Handler) http.Handler

func Chain(handler http.Handler, middlewares ...Middleware) http.Handler {
    for i := len(middlewares) - 1; i >= 0; i-- {
        handler = middlewares[i](handler)
    }
    return handler
}

// Usage:
handler := Chain(appHandler, Recoverer, RequestID, Logger, Auth)
```

### Interface decorator:

```go
type UserRepository interface {
    GetByID(ctx context.Context, id string) (*User, error)
}

// Logging decorator
type loggingUserRepo struct {
    next   UserRepository
    logger *slog.Logger
}

func NewLoggingUserRepo(next UserRepository, logger *slog.Logger) UserRepository {
    return &loggingUserRepo{next: next, logger: logger}
}

func (r *loggingUserRepo) GetByID(ctx context.Context, id string) (*User, error) {
    start := time.Now()
    user, err := r.next.GetByID(ctx, id)
    r.logger.Info("GetByID",
        slog.String("id", id),
        slog.Duration("duration", time.Since(start)),
        slog.Any("error", err),
    )
    return user, err
}
```

Stack decorators: `cache → logging → metrics → actual repo`.

## 6. Result Type Pattern

For operations that can return a value or an error in concurrent pipelines:

```go
type Result[T any] struct {
    Value T
    Err   error
}

func fetchAll(ctx context.Context, ids []string) []Result[User] {
    results := make([]Result[User], len(ids))
    var wg sync.WaitGroup

    for i, id := range ids {
        wg.Add(1)
        go func(i int, id string) {
            defer wg.Done()
            user, err := fetchUser(ctx, id)
            results[i] = Result[User]{Value: user, Err: err}
        }(i, id)
    }

    wg.Wait()
    return results
}
```

## 7. Cleanup with defer

### Resource management pattern:

```go
func processFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("open %s: %w", path, err)
    }
    defer f.Close()

    // process file...
    return nil
}
```

### Multi-resource cleanup:

```go
func migrate(ctx context.Context, srcDSN, dstDSN string) error {
    src, err := sql.Open("postgres", srcDSN)
    if err != nil {
        return fmt.Errorf("open source: %w", err)
    }
    defer src.Close()

    dst, err := sql.Open("postgres", dstDSN)
    if err != nil {
        return fmt.Errorf("open dest: %w", err)
    }
    defer dst.Close()

    // defers execute LIFO: dst.Close() first, then src.Close()
    return doMigration(ctx, src, dst)
}
```

## 8. Sentinel Values vs Zero Values

### Use the zero value as a useful default when possible:

```go
// ✅ Good — sync.Mutex zero value is an unlocked mutex
var mu sync.Mutex

// ✅ Good — bytes.Buffer zero value is an empty buffer
var buf bytes.Buffer

// ✅ Good — slice zero value is a valid empty slice
var users []User // nil slice works with append, len, range
```

### Use sentinel values when zero value is ambiguous:

```go
// When zero value is a valid input, use pointer or custom type
type Temperature struct {
    Celsius float64
    IsSet   bool
}

// Or use a pointer
func SetThreshold(t *float64) { // nil means "not configured"
    if t != nil {
        applyThreshold(*t)
    }
}
```

## Anti-Patterns to Avoid

```go
// ❌ God interface — too many methods
type Service interface {
    GetUser(ctx context.Context, id string) (*User, error)
    CreateUser(ctx context.Context, u *User) error
    DeleteUser(ctx context.Context, id string) error
    ListOrders(ctx context.Context, userID string) ([]Order, error)
    // 20 more methods...
}
// → Split into focused interfaces: UserReader, UserWriter, OrderLister

// ❌ Premature abstraction — interface for one implementation
type UserCache interface {
    Get(key string) (*User, bool)
    Set(key string, user *User)
}
// If there's only ever one implementation, use the concrete type.
// Extract an interface when a second consumer or implementation appears.

// ❌ Java-style inheritance simulation
type BaseService struct { ... }
type UserService struct { BaseService }  // embedding is NOT inheritance
// → Use composition: UserService has a dependency, not a parent.
```

## Verification Checklist

1. Functional options used for types with optional configuration
2. Constructors validate required dependencies and return errors
3. Factory functions return interfaces, not concrete types
4. No god interfaces — each interface has 1-3 methods
5. Middleware follows `func(http.Handler) http.Handler` signature
6. Decorators wrap interfaces, not concrete types
7. `defer` used for all resource cleanup (files, connections, locks)
8. Zero values are meaningful — no unnecessary initialization
9. No premature abstractions — interfaces extracted only when needed
10. Composition used instead of embedding for code reuse
