---
name: go-test-table-driven
description: >
  Deep dive on table-driven tests in Go: when to use them, when to avoid them,
  struct design, subtest naming, advanced patterns like test matrices and
  shared setup, and refactoring bloated tables into clean ones.
  Use when writing table-driven tests, refactoring test tables, reviewing
  table test structure, or deciding whether table-driven is the right approach.
  Trigger examples: "table-driven test", "table test", "test cases struct",
  "test matrix", "parametrize tests", "data-driven test", "refactor test table".
  Do NOT use for general test strategy, mocking, golden files, or fuzz testing
  (use go-test-quality). Do NOT use for benchmarks (use go-performance-review).
---

# Go Table-Driven Tests

Table-driven tests are a powerful Go idiom — when used correctly. The problem
is that most codebases either underuse them (writing 10 copy-paste tests) or
overuse them (jamming complex branching logic into a 200-line struct).
This skill covers the sweet spot.

## 1. When Table-Driven Tests Shine

Use table tests when ALL of these are true:

- **Same function** under test across all cases
- **Same assertion pattern** — input goes in, output comes out, compare
- **Cases differ only in data**, not in setup or verification logic
- **3+ cases** — fewer than 3, explicit subtests are clearer

The canonical use case: pure functions, parsers, validators, formatters.

```go
func TestFormatCurrency(t *testing.T) {
    tests := []struct {
        name     string
        cents    int64
        currency string
        want     string
    }{
        {
            name:     "USD whole dollars",
            cents:    1000,
            currency: "USD",
            want:     "$10.00",
        },
        {
            name:     "USD with cents",
            cents:    1050,
            currency: "USD",
            want:     "$10.50",
        },
        {
            name:     "EUR formatting",
            cents:    999,
            currency: "EUR",
            want:     "€9.99",
        },
        {
            name:     "zero amount",
            cents:    0,
            currency: "USD",
            want:     "$0.00",
        },
        {
            name:     "negative amount",
            cents:    -500,
            currency: "USD",
            want:     "-$5.00",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := FormatCurrency(tt.cents, tt.currency)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

Why this works: every case has the same shape, the loop body is 2 lines,
and adding a new case is one struct literal. No branching, no conditionals.

## 2. When NOT to Use Table-Driven Tests

### Complex per-case setup

If each case needs different mocks, different state, or different dependencies:

```go
// ❌ Bad — table test with branching setup
tests := []struct {
    name        string
    setupMock   func(*mockStore)     // each case wires differently
    setupAuth   func(*mockAuth)      // more per-case wiring
    input       Request
    wantStatus  int
    shouldNotify bool                // branching assertion
}{...}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        store := &mockStore{}
        tt.setupMock(store)          // hiding logic inside functions
        auth := &mockAuth{}
        tt.setupAuth(auth)
        // ... 20 lines of conditional assertions
    })
}
```

This is a code smell. The table is just hiding complexity behind function
fields. Write separate subtests instead — they're longer but honest:

```go
// ✅ Good — explicit subtests for different scenarios
func TestOrderHandler_Create(t *testing.T) {
    t.Run("succeeds with valid order", func(t *testing.T) {
        store := &mockStore{createFunc: func(...) (*Order, error) {
            return &Order{ID: "1"}, nil
        }}
        handler := NewHandler(store)
        // ... clear, readable, self-contained
    })

    t.Run("returns 401 when unauthenticated", func(t *testing.T) {
        handler := NewHandler(&mockStore{})
        // ... different setup, different assertions
    })
}
```

### Fewer than 3 cases

Two cases don't need a table. The overhead of defining the struct
is more code than just writing two tests:

```go
// ❌ Overkill for 2 cases
tests := []struct {
    name    string
    input   string
    wantErr bool
}{
    {"valid", "hello", false},
    {"empty", "", true},
}

// ✅ Just write them
func TestValidate_AcceptsNonEmptyString(t *testing.T) {
    require.NoError(t, Validate("hello"))
}

func TestValidate_RejectsEmptyString(t *testing.T) {
    require.Error(t, Validate(""))
}
```

### Multiple branching paths

If your loop body has `if tt.shouldError` / `if tt.expectNotification` /
`if tt.wantRedirect` — you've outgrown the table. Each branch is a
different test pretending to share a structure.

## 3. Struct Design

### Keep fields minimal

Every field should change between at least 2 cases. If a field has the
same value in all cases, it's not a variable — it's setup:

```go
// ❌ Bad — userRole is "admin" in every case
tests := []struct {
    name     string
    userRole string  // always "admin"
    input    string
    want     string
}{
    {"case1", "admin", "a", "A"},
    {"case2", "admin", "b", "B"},
}

// ✅ Good — remove constants from the struct
func TestAdminFormatter(t *testing.T) {
    ctx := contextWithRole("admin") // shared setup, outside table

    tests := []struct {
        name  string
        input string
        want  string
    }{
        {"case1", "a", "A"},
        {"case2", "b", "B"},
    }
    // ...
}
```

### Name the `name` field well

The `name` field appears in test output. Make it a short sentence that
explains the scenario, not a label:

```go
// ✅ Good names
{name: "trims leading whitespace"},
{name: "returns error for negative amount"},
{name: "handles unicode characters"},

// ❌ Bad names
{name: "case1"},
{name: "success"},
{name: "test with special chars"},
```

### Use `wantErr` correctly

```go
// ✅ Good — simple boolean for "should it error?"
tests := []struct {
    name    string
    input   string
    want    int
    wantErr bool
}{
    {name: "valid number", input: "42", want: 42},
    {name: "empty string", input: "", wantErr: true},
    {name: "not a number", input: "abc", wantErr: true},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got, err := ParseInt(tt.input)
        if tt.wantErr {
            require.Error(t, err)
            return
        }
        require.NoError(t, err)
        assert.Equal(t, tt.want, got)
    })
}
```

### When you need to check specific errors

Use a `wantErrIs` field with a sentinel error, not just a boolean:

```go
tests := []struct {
    name      string
    id        string
    wantErrIs error // nil means no error expected
}{
    {name: "valid id", id: "123"},
    {name: "empty id", id: "", wantErrIs: ErrInvalidID},
    {name: "not found", id: "999", wantErrIs: ErrNotFound},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        _, err := store.GetByID(ctx, tt.id)
        if tt.wantErrIs != nil {
            require.ErrorIs(t, err, tt.wantErrIs)
            return
        }
        require.NoError(t, err)
    })
}
```

## 4. The Loop Body Must Be Trivial

The entire point of a table test is that the execution logic is
identical for every case. If your loop body exceeds ~10 lines,
something is wrong.

```go
// ✅ Good — loop body is 5 lines
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got, err := Process(tt.input)
        require.NoError(t, err)
        assert.Equal(t, tt.want, got)
    })
}

// ❌ Bad — loop body has become a mini-program
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        if tt.setupDB {
            db := setupDB(t)
            defer db.Close()
        }
        svc := NewService()
        if tt.withCache { svc.EnableCache() }
        got, err := svc.Process(tt.input)
        if tt.wantErr {
            require.Error(t, err)
            if tt.wantErrMsg != "" { assert.Contains(t, err.Error(), tt.wantErrMsg) }
            return
        }
        // ... more conditionals, more branches
    })
}
```

If you see this, split into separate subtests or separate test functions.

## 5. Parallel Table Tests

```go
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        got := Transform(tt.input)
        assert.Equal(t, tt.want, got)
    })
}
```

In Go 1.22+, the loop variable is scoped per iteration, so the old
`tt := tt` capture is unnecessary. For Go <1.22, you still need it:

```go
// Go <1.22 only
for _, tt := range tests {
    tt := tt // capture range variable
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        // ...
    })
}
```

Only use `t.Parallel()` in table tests when the function under test
has no side effects and no shared mutable state.

## 6. Readability Tricks

### Align struct literals for scanning

```go
tests := []struct {
    name  string
    input string
    want  string
}{
    {"lowercase",           "hello",       "hello"},
    {"uppercase",           "HELLO",       "hello"},
    {"mixed case",          "HeLLo",       "hello"},
    {"with spaces",         "Hello World", "hello world"},
    {"already lowercase",   "test",        "test"},
}
```

This works for simple cases. For complex structs, use the multi-line format:

```go
tests := []struct {
    name   string
    config Config
    want   string
}{
    {
        name: "default timeout",
        config: Config{
            Host:    "localhost",
            Timeout: 0, // should get default
        },
        want: "localhost:8080",
    },
    {
        name: "custom port",
        config: Config{
            Host: "localhost",
            Port: 9090,
        },
        want: "localhost:9090",
    },
}
```

### Map-based tables for ultra-simple cases

When the struct would just be `{name, input, want}`:

```go
func TestStatusText(t *testing.T) {
    cases := map[string]struct {
        code int
        want string
    }{
        "ok":          {200, "OK"},
        "not found":   {404, "Not Found"},
        "server error": {500, "Internal Server Error"},
    }

    for name, tc := range cases {
        t.Run(name, func(t *testing.T) {
            assert.Equal(t, tc.want, StatusText(tc.code))
        })
    }
}
```

Note: map iteration order is random, so this also stress-tests that
your cases are truly independent.

## 7. Error-Only Tables

When you're testing a validator and only care about which inputs fail:

```go
func TestValidateEmail(t *testing.T) {
    valid := []string{
        "user@example.com",
        "user+tag@example.com",
        "user@sub.domain.com",
    }
    for _, email := range valid {
        t.Run("valid/"+email, func(t *testing.T) {
            require.NoError(t, ValidateEmail(email))
        })
    }

    invalid := []string{
        "",
        "@",
        "user@",
        "@domain.com",
        "user space@example.com",
    }
    for _, email := range invalid {
        t.Run("invalid/"+email, func(t *testing.T) {
            require.Error(t, ValidateEmail(email))
        })
    }
}
```

Two simple slices. No struct needed. The test name includes the input
value, so failures are self-documenting.

## 8. Refactoring Bloated Tables

Signs your table test needs refactoring:

| Symptom | Fix |
|---|---|
| Struct has 8+ fields | Split into multiple test functions by scenario |
| `setupFunc` field in struct | Extract to separate subtests with explicit setup |
| `if tt.shouldX` in loop body | Each branch is a different test — split it |
| Same 3 fields are identical in every case | Move to shared setup outside the table |
| Test name is the only way to understand the case | The case is too complex for a table |
| Adding a case requires understanding all other cases | Table has grown beyond its useful life |

## Decision Flowchart

1. **Is the function pure (input → output, no side effects)?**
   Yes → table test is probably ideal. Go to 2.
   No → consider explicit subtests first.

2. **Do all cases share the exact same assertion pattern?**
   Yes → table test. Go to 3.
   No → explicit subtests.

3. **Can each case be expressed in ≤5 struct fields?**
   Yes → table test.
   No → split by scenario into separate test functions.

4. **Is the loop body ≤10 lines?**
   Yes → you're golden.
   No → the table is hiding complexity. Refactor.

## Verification Checklist

1. Table struct has only fields that vary between cases
2. Every case has a descriptive `name` field
3. Loop body is ≤10 lines with no branching
4. No `setupFunc` or `mockFunc` fields in the struct
5. `wantErr` is a simple bool or sentinel, not a string match
6. Cases cover: happy path, error path, edge cases (empty, nil, zero, max)
7. `t.Run` wraps each case for named subtests
8. `t.Parallel()` used only when function is side-effect-free
