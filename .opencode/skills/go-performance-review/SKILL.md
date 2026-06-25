---
name: go-performance-review
description: >
  Detect performance anti-patterns and apply optimization techniques in Go.
  Covers allocations, string handling, slice/map preallocation, sync.Pool,
  benchmarking, and profiling with pprof.
  Use when checking performance, finding slow code, reducing allocations,
  profiling, or reviewing hot paths.
  Trigger examples: "check performance", "find slow code", "reduce allocations",
  "benchmark this", "profile", "optimize Go code".
  Do NOT use for concurrency correctness (use go-concurrency-review) or
  general code style (use go-coding-standards).
---

# Go Performance Review

Profile first, optimize second. Never optimize without a benchmark proving the problem.

## 1. Allocation Reduction

### Prefer `strconv` over `fmt` for primitive conversions:

```go
// ✅ Good — zero allocations for simple conversions
s := strconv.Itoa(42)
s := strconv.FormatFloat(3.14, 'f', 2, 64)

// ❌ Bad — fmt.Sprintf allocates
s := fmt.Sprintf("%d", 42)
```

### Avoid unnecessary string-to-byte conversions:

```go
// ✅ Good — use strings.Builder for concatenation
var b strings.Builder
for _, s := range parts {
    b.WriteString(s)
}
result := b.String()

// ❌ Bad — repeated concatenation allocates on every +
result := ""
for _, s := range parts {
    result += s
}
```

### Preallocate slices and maps when size is known:

```go
// ✅ Good — single allocation
users := make([]User, 0, len(ids))
for _, id := range ids {
    users = append(users, getUser(id))
}

// ✅ Good — map with capacity hint
lookup := make(map[string]User, len(users))

// ❌ Bad — repeated growing
var users []User // starts at 0, grows via doubling
```

### Use `sync.Pool` for frequently allocated, short-lived objects:

```go
var bufPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func process(data []byte) string {
    buf := bufPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufPool.Put(buf)
    }()

    buf.Write(data)
    return buf.String()
}
```

## 2. Hot Path Optimizations

### Avoid interface conversions in tight loops:

```go
// ✅ Good — concrete type in loop
func sum(vals []int64) int64 {
    var total int64
    for _, v := range vals {
        total += v
    }
    return total
}

// ❌ Bad — interface{} causes boxing/unboxing
func sum(vals []interface{}) int64 { ... }
```

### Avoid `reflect` in performance-critical paths:

If you need reflection-like behavior at scale, use code generation
(`go generate`, `stringer`, protocol buffers).

### Reduce pointer chasing:

```go
// ✅ Good — contiguous memory, cache-friendly
type Points struct {
    X []float64
    Y []float64
}

// ❌ Slower — pointer chasing per element
type Points []*Point
```

## 3. Map Performance

```go
// ✅ Use capacity hints
m := make(map[string]int, expectedSize)

// ✅ For read-heavy concurrent access, use sync.Map
// But ONLY when keys are stable — sync.Map has higher overhead
// for writes than a mutex-protected map.

// ✅ For fixed key sets, consider using a slice with index mapping
// instead of a map.
```

## 4. Benchmarking

ALWAYS write benchmarks before and after optimization:

```go
func BenchmarkFoo(b *testing.B) {
    // Setup outside the loop
    input := generateInput()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        result = Foo(input) // assign to package-level var to prevent elision
    }
}

// Package-level var prevents compiler from eliminating the call
var result string
```

Run benchmarks with memory profiling:

```bash
go test -bench=BenchmarkFoo -benchmem -count=5 ./...
```

Compare before/after with `benchstat`:

```bash
go test -bench=. -count=10 > old.txt
# make changes
go test -bench=. -count=10 > new.txt
benchstat old.txt new.txt
```

## 5. Profiling

### CPU profiling:

```bash
go test -cpuprofile=cpu.prof -bench=BenchmarkFoo .
go tool pprof cpu.prof
```

### Memory profiling:

```bash
go test -memprofile=mem.prof -bench=BenchmarkFoo .
go tool pprof -alloc_space mem.prof
```

### HTTP server profiling (import net/http/pprof):

```go
import _ "net/http/pprof"

// Access at http://localhost:6060/debug/pprof/
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

## 6. High-Throughput Logging

`log/slog` is the right default for most services. But when benchmarks show
logging is a bottleneck (high-frequency hot paths, >100k log lines/sec),
consider zero-allocation loggers.

### When slog is not enough:

```go
// slog allocates per log call — fine for most services
slog.Info("request handled",
    slog.String("method", method),
    slog.Int("status", status),
)

// In hot paths where benchmarks prove logging is a bottleneck,
// use zap's zero-allocation core:
logger, _ := zap.NewProduction()
logger.Info("request handled",
    zap.String("method", method),
    zap.Int("status", status),
)
// zap avoids allocations by using a field pool and typed fields
```

### Decision tree:

| Scenario | Logger |
|---|---|
| General service logging | `log/slog` (stdlib, zero dependencies) |
| High-frequency hot path (>100k lines/sec) | `go.uber.org/zap` (zero-alloc) |
| Extreme throughput with JSON | `github.com/rs/zerolog` (zero-alloc JSON) |

### Best of both worlds — use zap as slog backend:

```go
// Use slog API everywhere, backed by zap's performance
zapLogger, _ := zap.NewProduction()
slogHandler := zapslog.NewHandler(zapLogger.Core(), nil)
logger := slog.New(slogHandler)

// Code uses standard slog API — can swap backend without changing callers
logger.Info("request handled",
    slog.String("method", method),
    slog.Int("status", status),
)
```

### Logging anti-patterns in hot paths:

```go
// ❌ Bad — logging inside tight loop
for _, item := range millions {
    slog.Info("processing item", slog.String("id", item.ID))
    process(item)
}

// ✅ Good — sample or batch log
for i, item := range millions {
    process(item)
    if i%10000 == 0 {
        slog.Info("progress", slog.Int("processed", i), slog.Int("total", len(millions)))
    }
}

// ✅ Good — log summary after loop
slog.Info("batch complete", slog.Int("count", len(millions)))
```

NEVER switch loggers without a benchmark proving the need.
`slog` is fast enough for the vast majority of Go services.

## 7. Common Anti-Patterns

| Anti-Pattern | Fix |
|---|---|
| `fmt.Sprintf` for simple int→string | `strconv.Itoa` |
| String concatenation in loop | `strings.Builder` |
| Slice without preallocation | `make([]T, 0, n)` |
| Map without capacity hint | `make(map[K]V, n)` |
| `regexp.Compile` inside function | Compile once at package level |
| `json.Marshal` in hot path | Use code-gen (`easyjson`, `sonic`) |
| Logging in tight loop | Batch or sample |
| `defer` in very tight inner loop | Manual cleanup (rare, benchmark first) |

## Important Caveat

Most Go code is not performance-critical. Readability and correctness ALWAYS
take priority over micro-optimizations. Only apply these patterns when:

1. A benchmark proves this code path is a bottleneck
2. The optimization is significant (>10% improvement)
3. The resulting code remains readable and maintainable

Premature optimization is still the root of all evil, even in Go.
