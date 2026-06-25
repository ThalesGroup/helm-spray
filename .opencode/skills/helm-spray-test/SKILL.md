---
name: helm-spray-test
description: >
  Guide for writing unit tests for the helm-spray project. Covers testable
  packages, mocking exec.Command for helm/kubectl wrappers, creating chart
  fixtures, and table-driven test patterns. Use when writing tests, adding
  test coverage, or setting up test infrastructure for helm-spray.
  Trigger examples: "write tests", "add test coverage", "test this package",
  "mock helm command", "test dependencies", "test values merge".
---

# Helm Spray Testing Guide

Zero test files exist in this project. This guide helps you add meaningful tests.

## Quick Start

```bash
go test ./...                    # runs all tests (currently none)
go test -v ./internal/values/... # test specific package
go test -run TestMerge ./internal/values/...  # run single test
```

## Package Testability Matrix

| Package | Difficulty | Approach |
|---------|------------|----------|
| `pkg/util` | Easy | Pure function, no deps |
| `internal/log` | Easy | Pure function, capture stdout/stderr |
| `internal/values` | Medium | Pure functions (`mergeMaps`), chart fixture needed for `Merge` |
| `internal/dependencies` | Medium | Needs `*chart.Chart` fixture with dependencies |
| `pkg/helmspray` | Hard | Mixes config + state, calls helm/kubectl |
| `pkg/helm` | Hard | Shells out to `helm` CLI via `exec.Command` |
| `pkg/kubectl` | Hard | Shells out to `kubectl` CLI via `exec.Command` |

## Priority Order

1. `pkg/util/util.go` — `Duration()` is trivial, easy first test
2. `internal/log/log.go` — `Info()`, `WithNumberedLines()`, `Error()`
3. `internal/values/values.go` — `mergeMaps()` (pure), `processIncludeInValuesFile()` (needs chart fixture)
4. `internal/dependencies/dependencies.go` — `Get()` needs chart + values fixtures

## Pure Function Tests (No Mocking)

### pkg/util/util.go

```go
func TestDuration(t *testing.T) {
    tests := []struct {
        name     string
        input    time.Duration
        expected string
    }{
        {"zero", 0, "0s"},
        {"seconds", 45 * time.Second, "45s"},
        {"minutes", 90 * time.Second, "1m30s"},
        {"hours", 3661 * time.Second, "1h1m1s"},
        {"truncates", 1500 * time.Millisecond, "1s"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Duration(tt.input)
            if got != tt.expected {
                t.Errorf("Duration(%v) = %q, want %q", tt.input, got, tt.expected)
            }
        })
    }
}
```

### internal/values/values.go — mergeMaps

```go
func TestMergeMaps(t *testing.T) {
    tests := []struct {
        name     string
        a, b     map[string]interface{}
        expected map[string]interface{}
    }{
        {"override", map[string]interface{}{"k": "old"}, map[string]interface{}{"k": "new"}, map[string]interface{}{"k": "new"}},
        {"add", map[string]interface{}{"a": 1}, map[string]interface{}{"b": 2}, map[string]interface{}{"a": 1, "b": 2}},
        {"deep merge", ...},
    }
    // Note: mergeMaps is unexported. Test via exported Merge() or use _test.go in same package.
}
```

## Testing helm/kubectl Wrappers

The `pkg/helm` and `pkg/kubectl` packages shell out to CLI binaries. Options:

### Option A: Refactor to Interface (Preferred for New Code)

```go
type CommandRunner interface {
    Run(name string, args ...string) ([]byte, error)
}

type HelmClient struct {
    runner CommandRunner
}

func (h *HelmClient) List(namespace string) (map[string]Release, error) {
    output, err := h.runner.Run("helm", "list", "--namespace", namespace, "-o", "json")
    // ...
}
```

Then test with a mock:

```go
type mockRunner struct {
    output []byte
    err    error
}

func (m *mockRunner) Run(name string, args ...string) ([]byte, error) {
    return m.output, m.err
}
```

### Option B: exec.Command Hook (Quick-and-Dirty)

```go
// In test setup
execCommand = func(cmd string, args ...string) *exec.Cmd {
    return exec.Command("echo", `{"releases": []}`)
}
defer func() { execCommand = exec.Command }()
```

## Chart Fixture for Dependencies/Values Tests

Create a minimal chart in test setup:

```go
func createTestChart() *chart.Chart {
    return &chart.Chart{
        Metadata: &chart.Metadata{
            Name:    "test-umbrella",
            Version: "0.1.0",
            Dependencies: []*chart.Dependency{
                {Name: "sub-a", Version: "0.1.0", Alias: "alpha"},
                {Name: "sub-b", Version: "0.2.0"},
            },
        },
        Values: map[string]interface{}{
            "alpha": map[string]interface{}{
                "weight": 0,
            },
            "sub-b": map[string]interface{}{
                "weight": 1,
            },
        },
        Raw: []*chart.File{
            {
                Name: "values.yaml",
                Data: []byte("alpha:\n  weight: 0\nsub-b:\n  weight: 1\n"),
            },
        },
    }
}
```

## Table-Driven Test Pattern

Use this project's standard pattern:

```go
func TestFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        want    OutputType
        wantErr bool
    }{
        {"case 1", input1, expected1, false},
        {"case 2", input2, zeroVal, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Function(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Function() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("Function() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Test File Locations

Place `_test.go` files next to the source:

```
pkg/util/util.go          → pkg/util/util_test.go
internal/log/log.go       → internal/log/log_test.go
internal/values/values.go → internal/values/values_test.go
internal/dependencies/dependencies.go → internal/dependencies/dependencies_test.go
```

## Gotchas

- `mergeMaps` is unexported — test from `package values` (not `package values_test`)
- `internal/log` writes to `os.Stdout`/`os.Stderr` — capture with `os.Pipe()` or `os.Setenv`
- `pkg/helm` and `pkg/kubectl` depend on external binaries — skip tests if `helm`/`kubectl` not available: `testutil.SkipIfNoHelm(t)`
- No test framework required — use standard `testing` + `reflect.DeepEqual`
