---
name: go-context
description: >
  Correct usage of context.Context in Go: propagation, cancellation, timeouts,
  deadlines, values, and common anti-patterns.
  Use when: "context usage", "context.Context", "context cancellation", "timeout",
  "context.WithTimeout", "context.WithCancel", "context values", "context propagation".
  Do NOT use for: concurrency patterns beyond context (use go-concurrency-review),
  HTTP middleware context (use go-api-design), or
  error handling (use go-error-handling).
---

# Go Context

`context.Context` controls cancellation, deadlines, and request-scoped values
across API boundaries. Misusing it causes goroutine leaks, orphaned work,
and subtle production bugs.

## 1. Core Rules

### Context is always the first parameter:

```go
// ✅ Good — context is first
func GetUser(ctx context.Context, id string) (*User, error)
func (s *Service) Process(ctx context.Context, req Request) error

// ❌ Bad — context buried in the middle or end
func GetUser(id string, ctx context.Context) (*User, error)
func Process(req Request, ctx context.Context) error
```

### NEVER store context in a struct:

```go
// ❌ Bad — context stored in struct
type Server struct {
    ctx    context.Context // NEVER do this
    cancel context.CancelFunc
}

// ✅ Good — pass context through method parameters
func (s *Server) Shutdown(ctx context.Context) error {
    return s.httpServer.Shutdown(ctx)
}
```

Context represents the lifetime of a single operation, not the lifetime of an object.

### NEVER pass nil context:

```go
// ❌ Bad
doSomething(nil, data)

// ✅ Good — use context.TODO() if unsure which context to use
doSomething(context.TODO(), data)

// ✅ Good — use context.Background() for top-level/main
doSomething(context.Background(), data)
```

## 2. Cancellation

### Always defer cancel:

```go
// ✅ Good — cancel called even if operation succeeds
ctx, cancel := context.WithCancel(parentCtx)
defer cancel()

result, err := longOperation(ctx)
```

Failing to call cancel leaks resources (timers, goroutines) until the parent
context is cancelled.

### Use WithCancel for manual cancellation:

```go
func (s *Supervisor) Run(ctx context.Context) error {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    g, ctx := errgroup.WithContext(ctx)

    g.Go(func() error { return s.runWorkerA(ctx) })
    g.Go(func() error { return s.runWorkerB(ctx) })

    // If any worker returns an error, errgroup cancels ctx,
    // which signals all other workers to stop.
    return g.Wait()
}
```

### Check context cancellation in loops:

```go
// ✅ Good — respects cancellation
func processItems(ctx context.Context, items []Item) error {
    for _, item := range items {
        if err := ctx.Err(); err != nil {
            return fmt.Errorf("processing cancelled: %w", err)
        }
        if err := process(ctx, item); err != nil {
            return fmt.Errorf("process item %s: %w", item.ID, err)
        }
    }
    return nil
}

// ❌ Bad — runs to completion even if cancelled
func processItems(ctx context.Context, items []Item) error {
    for _, item := range items {
        process(ctx, item) // ignores ctx cancellation between items
    }
    return nil
}
```

## 3. Timeouts and Deadlines

### WithTimeout for duration-based limits:

```go
func (c *Client) FetchUser(ctx context.Context, id string) (*User, error) {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+"/users/"+id, nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("fetch user %s: %w", id, err)
    }
    defer resp.Body.Close()

    // ...
}
```

### WithDeadline for absolute time limits:

```go
// Use when coordinating with external deadlines (SLAs, cron windows)
deadline := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
ctx, cancel := context.WithDeadline(ctx, deadline)
defer cancel()
```

### Timeout budgets — don't exceed parent timeout:

```go
// ✅ Good — child timeout shorter than parent
func handler(ctx context.Context) error {
    // Parent has 30s timeout (from HTTP server)

    // Give DB query 5s of the 30s budget
    dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    data, err := db.QueryContext(dbCtx, query)

    // Give external API 10s of the remaining budget
    apiCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    result, err := client.Call(apiCtx, data)

    return nil
}

// ❌ Bad — child timeout exceeds parent (silently capped anyway)
ctx, cancel := context.WithTimeout(parentCtx, 60*time.Second) // parent has 5s left
// This timeout is 60s but will actually fire at parent's deadline
```

### Check if deadline exists:

```go
if deadline, ok := ctx.Deadline(); ok {
    remaining := time.Until(deadline)
    if remaining < minRequired {
        return fmt.Errorf("insufficient time remaining: %v", remaining)
    }
}
```

## 4. Context Values

### Use sparingly — only for request-scoped metadata:

```go
// ✅ Appropriate uses:
// - Request ID
// - Trace/span ID
// - Authenticated user info
// - Request-scoped logger

// ❌ Bad uses:
// - Database connections (use dependency injection)
// - Configuration (use struct fields)
// - Function parameters (pass explicitly)
```

### Use unexported key types to prevent collisions:

```go
// ✅ Good — unexported type prevents key collisions
type contextKey struct{}

var requestIDKey = contextKey{}

func WithRequestID(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, requestIDKey, id)
}

func RequestID(ctx context.Context) string {
    id, _ := ctx.Value(requestIDKey).(string)
    return id
}
```

```go
// ❌ Bad — string keys risk collisions across packages
ctx = context.WithValue(ctx, "request_id", id) // any package could overwrite this
```

### Always provide accessor functions — never expose the key:

```go
// ✅ Good — clean API with accessors
rid := middleware.RequestID(ctx)

// ❌ Bad — exposes internal key type
rid := ctx.Value(requestIDKey).(string) // caller needs key, risks panic on nil
```

## 5. Context in HTTP Handlers

### Use r.Context() for the request context:

```go
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context() // carries cancellation when client disconnects

    user, err := h.service.GetUser(ctx, id)
    if err != nil {
        if errors.Is(err, context.Canceled) {
            return // client disconnected, no point writing response
        }
        // handle error...
    }
    // ...
}
```

### Attach values via middleware:

```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        user, err := authenticate(r)
        if err != nil {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        ctx := WithUser(r.Context(), user)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

## 6. Context in Testing

### Use context with timeout in tests to prevent hangs:

```go
func TestSlowOperation(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    result, err := slowOperation(ctx)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    // assert result...
}
```

### Test cancellation behavior:

```go
func TestCancellation(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    cancel() // cancel immediately

    _, err := operation(ctx)
    if !errors.Is(err, context.Canceled) {
        t.Errorf("expected context.Canceled, got %v", err)
    }
}
```

## 7. context.Background() vs context.TODO()

| Function | When to use |
|---|---|
| `context.Background()` | Top-level: `main()`, `init()`, test setup. Intentional root context. |
| `context.TODO()` | Placeholder when you don't know which context to use yet. Signals "this needs to be fixed". |

`context.TODO()` is a code smell in production code — replace it before shipping.

## Verification Checklist

1. `context.Context` is the first parameter in all functions that accept it
2. No context stored in struct fields
3. `defer cancel()` called immediately after `WithCancel`, `WithTimeout`, `WithDeadline`
4. Long loops check `ctx.Err()` between iterations
5. Child timeouts don't exceed parent timeout budget
6. Context values use unexported key types with accessor functions
7. Only request-scoped metadata stored in context values (not configs, connections)
8. HTTP handlers use `r.Context()` and pass it downstream
9. No `nil` context passed — use `context.TODO()` or `context.Background()`
10. Tests use `context.WithTimeout` to prevent hanging
