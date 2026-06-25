---
name: go-test-quality
description: >
  Go testing patterns for production-grade code: subtests, test helpers, fixtures,
  golden files, httptest, testcontainers, property-based testing, and fuzz testing.
  Covers mocking strategies, test isolation, coverage analysis, and test design philosophy.
  Use when writing tests, improving coverage, reviewing test quality,
  setting up test infrastructure, or choosing a testing approach.
  Trigger examples: "add tests", "improve coverage", "write tests for this",
  "test helpers", "mock this dependency", "integration test", "fuzz test".
  Do NOT use for performance benchmarking methodology (use go-performance-review),
  security testing (use go-security-audit), or table-driven test patterns
  specifically (use go-test-table-driven).
---

# Go Test Quality

Tests are production code. They run in CI on every commit, they document behavior,
and they're the first thing you read when a function breaks at 3am.
Write them with the same care you'd give to code that handles money.

## 1. Test Design Philosophy

### Test behavior, not implementation

```go
// ✅ Good — tests what the function DOES
func TestTransferFunds_InsufficientBalance(t *testing.T) {
    from := NewAccount("alice", 100)
    to := NewAccount("bob", 0)

    err := TransferFunds(from, to, 150)

    require.ErrorIs(t, err, ErrInsufficientFunds)
    assert.Equal(t, 100, from.Balance(), "sender balance should be unchanged")
    assert.Equal(t, 0, to.Balance(), "receiver balance should be unchanged")
}

// ❌ Bad — tests HOW the function does it
func TestTransferFunds_InsufficientBalance(t *testing.T) {
    // asserts that debit() was called before credit()
    // asserts that rollback() was called
    // asserts internal mutex was locked
}
```

### One assertion per logical concept

A test should verify one behavior. Multiple `assert` calls are fine when they
verify different facets of the SAME behavior (e.g., both accounts after a transfer).
But a test that checks creation AND update AND deletion is three tests pretending
to be one.

### Name tests like bug reports

The test name should describe the scenario so clearly that when it fails,
you already know what broke without reading the test body:

```go
// ✅ Good — reads like a sentence
func TestOrderService_Cancel_RefundsPartiallyShippedItems(t *testing.T) { ... }
func TestParseConfig_ReturnsErrorOnMissingRequiredField(t *testing.T) { ... }
func TestRateLimiter_AllowsBurstAfterCooldown(t *testing.T) { ... }

// ❌ Bad — says nothing useful
func TestCancel(t *testing.T) { ... }
func TestParseConfig2(t *testing.T) { ... }
func TestRateLimiter_Success(t *testing.T) { ... }
```

## 2. Subtests for Organized Scenarios

Use `t.Run` to group related scenarios under a parent test. Each subtest
gets its own setup, its own failure, and its own name in the output:

```go
func TestUserService_Create(t *testing.T) {
    svc := setupUserService(t)

    t.Run("succeeds with valid input", func(t *testing.T) {
        user, err := svc.Create(ctx, CreateUserInput{
            Name:  "Alice",
            Email: "alice@example.com",
        })

        require.NoError(t, err)
        assert.NotEmpty(t, user.ID)
        assert.Equal(t, "Alice", user.Name)
    })

    t.Run("rejects duplicate email", func(t *testing.T) {
        _, _ = svc.Create(ctx, CreateUserInput{
            Name: "Alice", Email: "taken@example.com",
        })

        _, err := svc.Create(ctx, CreateUserInput{
            Name: "Bob", Email: "taken@example.com",
        })

        require.ErrorIs(t, err, ErrDuplicateEmail)
    })

    t.Run("rejects empty name", func(t *testing.T) {
        _, err := svc.Create(ctx, CreateUserInput{
            Name: "", Email: "valid@example.com",
        })

        var valErr *ValidationError
        require.ErrorAs(t, err, &valErr)
        assert.Equal(t, "name", valErr.Field)
    })
}
```

Each subtest is independent, readable, and debuggable. When `rejects duplicate email`
fails, you see exactly that name in CI output — not `TestUserService_Create/case_3`.

## 3. Test Helpers Done Right

### Always call `t.Helper()`

This makes failure messages point to the caller, not the helper:

```go
func createTestUser(t *testing.T, svc *UserService, name string) *User {
    t.Helper()
    user, err := svc.Create(context.Background(), CreateUserInput{
        Name:  name,
        Email: name + "@test.com",
    })
    require.NoError(t, err)
    return user
}
```

### Factory functions with functional options

For complex test objects, avoid a constructor with 15 parameters.
Use defaults with overrides:

```go
func newTestOrder(t *testing.T, opts ...func(*Order)) *Order {
    t.Helper()
    o := &Order{
        ID:        uuid.New(),
        UserID:    uuid.New(),
        Status:    OrderStatusPending,
        Total:     9999, // $99.99
        CreatedAt: time.Now(),
    }
    for _, opt := range opts {
        opt(o)
    }
    return o
}

// Usage — only override what matters for THIS test
func TestOrder_Cancel_RejectsShippedOrders(t *testing.T) {
    order := newTestOrder(t, func(o *Order) {
        o.Status = OrderStatusShipped
    })

    err := order.Cancel()
    require.ErrorIs(t, err, ErrCannotCancelShipped)
}
```

### Cleanup with `t.Cleanup`

Prefer `t.Cleanup` over `defer` — it runs even if the test calls `t.FailNow()`,
and it's scoped to the test, not the function:

```go
func setupTestDB(t *testing.T) *sql.DB {
    t.Helper()
    db, err := sql.Open("postgres", testDSN)
    require.NoError(t, err)

    t.Cleanup(func() {
        db.Close()
    })
    return db
}
```

## 4. Golden File Testing

For complex outputs (JSON responses, HTML, SQL queries, protobuf), comparing
against golden files is more maintainable than inline assertions:

```go
var update = flag.Bool("update", false, "update golden files")

func TestRenderInvoice(t *testing.T) {
    invoice := buildTestInvoice()

    got, err := RenderInvoice(invoice)
    require.NoError(t, err)

    golden := filepath.Join("testdata", t.Name()+".golden")

    if *update {
        // Run: go test -update  to regenerate golden files
        require.NoError(t, os.WriteFile(golden, got, 0644))
    }

    want, err := os.ReadFile(golden)
    require.NoError(t, err)
    assert.Equal(t, string(want), string(got))
}
```

Golden files live in `testdata/` directories (which `go build` ignores).
Commit them to git — they ARE the expected output. Review diffs in PRs.

## 5. HTTP Handler Testing with `httptest`

Use `httptest.NewRecorder` for unit-style handler tests:

```go
func TestUserHandler_GetByID(t *testing.T) {
    store := &mockUserStore{
        getByIDFunc: func(ctx context.Context, id string) (*User, error) {
            if id == "123" {
                return &User{ID: "123", Name: "Alice"}, nil
            }
            return nil, ErrNotFound
        },
    }
    handler := NewUserHandler(store, slog.New(slog.NewTextHandler(io.Discard, nil)))

    t.Run("returns user as JSON", func(t *testing.T) {
        req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
        req.SetPathValue("id", "123")

        rec := httptest.NewRecorder()
        handler.HandleGet(rec, req)

        assert.Equal(t, http.StatusOK, rec.Code)
        assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")

        var body map[string]string
        require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
        assert.Equal(t, "Alice", body["name"])
    })

    t.Run("returns 404 for unknown user", func(t *testing.T) {
        req := httptest.NewRequest(http.MethodGet, "/users/unknown", nil)
        req.SetPathValue("id", "unknown")

        rec := httptest.NewRecorder()
        handler.HandleGet(rec, req)

        assert.Equal(t, http.StatusNotFound, rec.Code)
    })
}
```

### Full server test with `httptest.NewServer`:

```go
func TestAPI_CreateUser_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    app := setupApp(t)
    srv := httptest.NewServer(app.Router())
    t.Cleanup(srv.Close)

    resp, err := http.Post(srv.URL+"/api/v1/users",
        "application/json",
        strings.NewReader(`{"name":"Alice","email":"alice@test.com"}`))
    require.NoError(t, err)
    defer resp.Body.Close()

    assert.Equal(t, http.StatusCreated, resp.StatusCode)
}
```

## 6. Mocking Strategies

### Interface-based mocks (preferred for ≤3 methods):

```go
type mockNotifier struct {
    sendFunc func(ctx context.Context, to, msg string) error
    sent     []string
}

func (m *mockNotifier) Send(ctx context.Context, to, msg string) error {
    m.sent = append(m.sent, to)
    if m.sendFunc != nil {
        return m.sendFunc(ctx, to, msg)
    }
    return nil
}
```

### Function injection for simple seams:

```go
type Service struct {
    now    func() time.Time
    randID func() string
}

// Production: svc := &Service{now: time.Now, randID: uuid.NewString}
// Test:       svc := &Service{now: fixedTime, randID: func() string { return "abc" }}
```

### What NOT to mock:

- Value objects and pure functions — just call them
- The standard library — test the real `json.Marshal`, not a mock
- Your own code in the same package — test the real thing

## 7. Integration Tests with Testcontainers

```go
func TestPostgresUserStore(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    ctx := context.Background()
    pg, err := postgres.Run(ctx,
        "postgres:16-alpine",
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready").
                WithOccurrence(2).
                WithStartupTimeout(30*time.Second),
        ),
    )
    require.NoError(t, err)
    t.Cleanup(func() { pg.Terminate(ctx) })

    connStr, err := pg.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)

    store, err := NewPostgresStore(connStr)
    require.NoError(t, err)

    t.Run("create and retrieve user", func(t *testing.T) {
        created, err := store.Create(ctx, &User{Name: "Alice"})
        require.NoError(t, err)

        fetched, err := store.GetByID(ctx, created.ID)
        require.NoError(t, err)
        assert.Equal(t, "Alice", fetched.Name)
    })
}
```

Separate with build tags: `//go:build integration`

Run with: `go test -tags=integration -count=1 ./...`

## 8. Fuzz Testing (Go 1.18+)

Fuzz tests discover edge cases you'd never think of. Use for parsers,
validators, serializers — anything that takes arbitrary input:

```go
func FuzzParseEmail(f *testing.F) {
    f.Add("alice@example.com")
    f.Add("")
    f.Add("@")

    f.Fuzz(func(t *testing.T, input string) {
        result, err := ParseEmail(input)
        if err != nil {
            return // invalid input is fine, just don't panic
        }

        // Round-trip: parsing the output should give the same result
        reparsed, err := ParseEmail(result.String())
        require.NoError(t, err)
        assert.Equal(t, result, reparsed)
    })
}
```

Run with: `go test -fuzz=FuzzParseEmail -fuzztime=30s`

If your function can receive untrusted input, fuzz it.

## 9. Parallel Tests

```go
func TestSlugify(t *testing.T) {
    t.Parallel()

    t.Run("lowercases input", func(t *testing.T) {
        t.Parallel()
        assert.Equal(t, "hello-world", Slugify("Hello World"))
    })

    t.Run("strips special characters", func(t *testing.T) {
        t.Parallel()
        assert.Equal(t, "caf", Slugify("café!"))
    })
}
```

Do NOT use `t.Parallel()` when tests share mutable state,
databases, files, or process-level state (`os.Setenv`).

## 10. TestMain for Shared Setup

Use when ALL tests in a package need expensive one-time setup:

```go
var testDB *sql.DB

func TestMain(m *testing.M) {
    var teardown func()
    testDB, teardown = setupTestDatabase()

    code := m.Run()

    teardown()
    os.Exit(code)
}
```

Use sparingly — most tests don't need it.

## 11. Coverage

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
go tool cover -func=coverage.out
```

**Targets:** business logic 80%+, critical paths (auth, payments) 95%+,
handlers 70%+. Don't chase 100% on generated code and simple getters.

## Anti-Patterns

- 🔴 Test with no assertions — always passes, proves nothing
- 🔴 `time.Sleep` for synchronization — use channels or polling
- 🔴 Test depends on execution order — each test must stand alone
- 🔴 Mocking everything — you end up testing your mocks, not your code
- 🟡 Test names like `Test1`, `TestSuccess` — name the scenario
- 🟡 Reaching into private fields — test through the public API
- 🟡 No edge cases: empty, nil, zero, max values, unicode
- 🟡 Giant shared setup — each test should set up only what it needs
- 🟢 Fuzz anything that takes untrusted input
- 🟢 Golden files for complex output comparisons

## Verification Checklist

1. Every test has meaningful assertions (no empty test bodies)
2. Test names describe the scenario, not the method
3. `t.Helper()` called in every test utility function
4. `t.Cleanup()` used for resource teardown
5. `t.Parallel()` used where safe, avoided where not
6. Integration tests guarded with `testing.Short()` or build tags
7. Mocks are minimal — only mock external dependencies
8. Edge cases covered: empty, nil, zero, boundary values
9. `go test -race ./...` passes
10. Coverage is meaningful, not just high numbers
