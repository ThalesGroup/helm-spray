---
name: go-concurrency-review
description: >
  Review and implement safe concurrency patterns in Go: goroutines, channels,
  sync primitives, context propagation, and goroutine lifecycle management.
  Use when writing concurrent code, reviewing async patterns, checking thread safety,
  debugging race conditions, or designing producer/consumer pipelines.
  Trigger examples: "check thread safety", "review goroutines", "race condition",
  "channel patterns", "sync.Mutex", "context cancellation", "goroutine leak".
  Do NOT use for general code style (use go-coding-standards) or
  HTTP handler patterns (use go-api-design).
---

# Go Concurrency Review

Concurrency in Go is powerful and deceptively easy to get wrong.
These patterns prevent goroutine leaks, data races, and deadlocks.

## 1. Goroutine Lifecycle Management

EVERY goroutine MUST have a clear termination path. No fire-and-forget.

### Use `errgroup` for coordinated goroutines:

```go
g, ctx := errgroup.WithContext(ctx)

g.Go(func() error {
    return fetchUsers(ctx)
})

g.Go(func() error {
    return fetchOrders(ctx)
})

if err := g.Wait(); err != nil {
    return fmt.Errorf("fetch data: %w", err)
}
```

### Long-running goroutines must respect context:

```go
func (w *Worker) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case job := <-w.jobs:
            if err := w.process(job); err != nil {
                w.logger.Error("process job", slog.Any("error", err))
            }
        }
    }
}
```

### Start goroutines in the owner, not the callee:

```go
// ✅ Good — caller controls lifecycle
go worker.Run(ctx)

// ❌ Bad — function secretly starts goroutine
func NewWorker() *Worker {
    w := &Worker{}
    go w.run() // hidden goroutine — caller has no control
    return w
}
```

## 2. Channel Patterns

### Channel size is one or none:

```go
// Unbuffered — synchronization point
ch := make(chan Result)

// Buffered with size 1 — single-item handoff
ch := make(chan Result, 1)

// Larger buffers need explicit justification with documented reasoning
ch := make(chan Result, 100) // requires comment explaining why
```

### Signal channels use empty struct:

```go
done := make(chan struct{})
close(done) // broadcast signal to all receivers
```

### Producer/consumer with clean shutdown:

```go
func produce(ctx context.Context) <-chan Item {
    ch := make(chan Item)
    go func() {
        defer close(ch)
        for {
            item, err := fetchNext(ctx)
            if err != nil {
                return
            }
            select {
            case ch <- item:
            case <-ctx.Done():
                return
            }
        }
    }()
    return ch
}
```

## 3. Mutex Patterns

### Zero-value mutexes are valid:

```go
// ✅ Good — zero value works
type Cache struct {
    mu    sync.RWMutex
    items map[string]Item
}

// ❌ Bad — unnecessary pointer
type Cache struct {
    mu    *sync.RWMutex // never do this
}
```

### Mutex placement in struct:

```go
type SafeMap struct {
    mu sync.RWMutex // mutex guards the fields below
    items map[string]string
    count int
}
```

The mutex should appear directly above the field(s) it protects,
with a comment indicating the relationship.

### Lock scope should be minimal:

```go
// ✅ Good — minimal lock scope
func (c *Cache) Get(key string) (Item, bool) {
    c.mu.RLock()
    item, ok := c.items[key]
    c.mu.RUnlock()
    return item, ok
}

// ✅ Also good — defer for methods that return early
func (c *Cache) GetOrCreate(key string) Item {
    c.mu.Lock()
    defer c.mu.Unlock()

    if item, ok := c.items[key]; ok {
        return item
    }
    item := newItem(key)
    c.items[key] = item
    return item
}
```

### Never copy mutexes:

```go
// ❌ BLOCKER — copying a mutex copies its lock state
cache2 := *cache1 // this copies the mutex!
```

## 4. Atomic Operations

Use `sync/atomic` or `go.uber.org/atomic` for simple counters and flags:

```go
// ✅ Good — type-safe atomics
import "go.uber.org/atomic"

type Server struct {
    running atomic.Bool
    reqCount atomic.Int64
}

func (s *Server) HandleRequest() {
    s.reqCount.Inc()
    // ...
}
```

## 5. Context Propagation

### Rules:
- Context is ALWAYS the first parameter.
- Never store context in a struct field.
- Derive child contexts for sub-operations:

```go
func (s *Service) Process(ctx context.Context, req Request) error {
    // Derive context with timeout for external call
    fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel() // ALWAYS defer cancel

    data, err := s.client.Fetch(fetchCtx, req.ID)
    if err != nil {
        return fmt.Errorf("fetch %s: %w", req.ID, err)
    }
    // ...
}
```

### NEVER ignore context cancellation in select:

```go
// ✅ Good
select {
case result := <-ch:
    return result, nil
case <-ctx.Done():
    return nil, ctx.Err()
}

// ❌ Bad — blocks forever if context cancelled
result := <-ch
```

## 6. Avoid Mutable Globals

```go
// ❌ Bad — mutable global, not safe for concurrent access
var db *sql.DB

// ✅ Good — pass as dependency
type Server struct {
    db *sql.DB
}
```

## 7. sync.Once for Lazy Initialization

```go
type Client struct {
    initOnce sync.Once
    conn     *grpc.ClientConn
}

func (c *Client) getConn() *grpc.ClientConn {
    c.initOnce.Do(func() {
        c.conn = dial()
    })
    return c.conn
}
```

## Race Detection

ALWAYS run tests with race detector during CI:

```bash
go test -race ./...
```

This is non-negotiable. A test suite that passes without `-race` proves nothing
about concurrent correctness.

## Red Flags Checklist

- 🔴 Goroutine started without shutdown path
- 🔴 Channel never closed (potential goroutine leak)
- 🔴 Mutex copied by value
- 🔴 Context stored in struct field
- 🔴 `context.Background()` used where parent context was available
- 🔴 `select` without `ctx.Done()` case in blocking operation
- 🔴 Shared map/slice accessed without synchronization
- 🟡 Buffered channel with arbitrary large size
- 🟡 `time.Sleep` used for synchronization instead of proper signaling
- 🟡 Goroutine starting inside `init()` or constructor without lifecycle control
