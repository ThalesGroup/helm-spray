# Test Suite Contribution

## What was done

Added the first test suite to helm-spray, which previously had **zero test files** across 1,356 lines of Go code.

**Branch:** `feat/test-suite`  
**Commit:** `test: add comprehensive test suite covering all packages (50 tests)`

---

## Files added

| File | Tests |
|---|---|
| `internal/dependencies/dependencies_test.go` | 16 |
| `internal/values/values_test.go` | 14 |
| `pkg/helmspray/helmspray_test.go` | 12 |
| `pkg/kubectl/kubectl_test.go` | 6 |
| **Total** | **50** |

---

## Coverage by package

### `internal/dependencies` — `Get()`

The core function that parses chart dependencies, resolves weights, and builds the deployment list.

- Empty dependency list returns cleanly
- Single dependency with default weight 0
- Alias resolution: `UsedName` is set to `Alias` when present, `Name` otherwise
- Weight parsing for both `float64` (YAML default) and `json.Number` (alternate JSON decoder)
- Negative weight returns an error
- Unsupported weight type (e.g. string) returns an error
- `--target` flag: only matched deps are `Targeted=true`
- `--exclude` flag: matched deps are `Targeted=false`
- Target matching works against alias, not just chart name
- Release prefix is prepended to `CorrespondingReleaseName`
- `AppVersion` is resolved from the matching sub-chart in the dependency tree
- Tags: dep with no tags is always `AllowedByTags=true`
- Tags: dep with a tag enabled in values is `AllowedByTags=true`
- Tags: dep with a tag absent from values is `AllowedByTags=false`
- Multiple deps with different weights are all parsed correctly

### `internal/values` — `processIncludeInValuesFile()` and `mergeMaps()`

The include-directive processor expands `#! {{ .Files.Get }}` comments in `values.yaml` before passing values to Helm.

- Values file with no directives is returned unchanged
- `#! {{ .Files.Get file.yaml }}` is replaced with file contents
- `#! {{ .File.Get file.yaml }}` (without `s`) works as a backward-compatible alias
- `| indent N` indents the included content by N spaces
- `pick (.Files.Get file.yaml) subtag` extracts a specific YAML sub-table
- `pick` combined with `| indent` works together
- A directive referencing a missing file returns an error
- An empty values file returns an empty string without error
- Multiple directives in the same file are all resolved

`mergeMaps` deep-merges two `map[string]interface{}` trees:

- Non-overlapping keys from both maps appear in the result
- For conflicting scalar keys, `b` wins
- For conflicting nested maps, keys are merged recursively (`b` wins leaf conflicts)
- Keys present only in `a` survive when `b` is empty
- Neither input map is mutated

### `pkg/helmspray` — `maxWeight()` and `checkTargetsAndExcludes()`

Pure helper functions used in the core orchestration loop.

`maxWeight`:
- Returns 0 for nil or empty input
- Returns the single value for a one-element slice
- Returns the correct maximum across multiple weights
- Handles the case where all weights are 0
- Handles the case where the maximum is the first element

`checkTargetsAndExcludes`:
- Returns nil when neither targets nor excludes are specified
- Returns nil for a valid target name
- Returns an error for an unknown target name
- Returns nil for a valid exclude name
- Returns an error for an unknown exclude name
- Returns nil when all of multiple targets are valid
- Matches targets against `UsedName` (alias), not raw chart name
- Returns an error if any one of multiple targets is invalid

### `pkg/kubectl` — `generateTemplate()`

Builds a `kubectl` go-template string used to poll workload readiness.

- Single name produces `{{if eq "name" .metadata.name}}` (no `or`)
- Multiple names produce `{{if or (eq ...) (eq ...) }}`
- Three names all appear in the template
- Output always starts with `{{range .items}}` and ends with `{{end}}`
- The body string is embedded verbatim in the template
- Single name does not use the `or` combinator

---

## Bug fixed alongside tests

**`internal/dependencies/dependencies.go:133`** — Pre-existing `go vet` failure caused by passing an already-formatted `fmt.Sprintf(...)` result as the format string argument to `log.Info`, which internally calls `fmt.Sprintf(str, params...)`. Fixed by passing format args directly:

```go
// Before (triggers vet: non-constant format string)
log.Info(2, fmt.Sprintf("found tag \"%s: %s\"", k, fmt.Sprint(v)))

// After
log.Info(2, "found tag \"%s: %s\"", k, fmt.Sprint(v))
```

This was blocking compilation of the entire `dependencies` package under `go test`.

---

## How to run

```bash
go test ./...
```

Expected output:

```
ok  github.com/gemalto/helm-spray/v4/internal/dependencies
ok  github.com/gemalto/helm-spray/v4/internal/values
ok  github.com/gemalto/helm-spray/v4/pkg/helmspray
ok  github.com/gemalto/helm-spray/v4/pkg/kubectl
```
