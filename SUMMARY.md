# Summary of Changes — helm-spray

**Branch:** `feature/AI-improvements_EC`
**Date:** June 25, 2026

---

## Phase 1: Bug Fixes (14 issues fixed)

| File | Fix |
|------|-----|
| `pkg/helm/helm.go` | Replaced `ioutil.TempDir` → `os.MkdirTemp`, removed shell injection (`sh -c`), added bounds check with `os.ReadDir`, parameter renamed `chart` → `chartRef` |
| `pkg/helmspray/helmspray.go` | Replaced `ioutil.TempDir`/`TempFile` → `os.MkdirTemp`/`os.CreateTemp`, removed duplicate `"os"` import, removed `== true` comparisons |
| `internal/dependencies/dependencies.go` | Replaced `reflect.TypeOf` → type switch, removed `reflect` import |
| `pkg/kubectl/kubectl.go` | Added `strconv.Atoi` error handling |
| `internal/log/log.go` | Fixed `WithNumberedLines` counter variable, added `numberOfLines == 0` guard |
| `cmd/root.go` | Removed `== true` comparison |
| `internal/values/values.go` | Removed `== false` and `== true` comparisons |
| `README.md` | Fixed 5 typos: "thei weigths", "primarilly", "temporarilly", "cwthis", "con be" |

## Phase 2: Wait Algorithm Fix (issues #60, #58)

| File | Fix |
|------|-----|
| `pkg/kubectl/kubectl.go` | Deployments: now check `unavailableReplicas > 0` in addition to `readyReplicas`. StatefulSets: removed `updatedReplicas` check (was causing infinite wait with `OnDelete` strategy) |

## Phase 3: Helm 4 Migration

| File | Change |
|------|--------|
| `go.mod` | `helm.sh/helm/v3 v3.18.5` → `helm.sh/helm/v4 v4.2.2` |
| `pkg/helmspray/helmspray.go` | Updated imports, handles `chart.Charter` interface |
| `internal/values/values.go` | Updated imports (`chart/common`, `chart/common/util`), function signature accepts `chart.Charter` |
| `internal/dependencies/dependencies.go` | Updated imports (`chart/v2`), type-asserts `chart.Charter` → `*chartv2.Chart` |

## Phase 4: Web GUI

| File | Description |
|------|-------------|
| `cmd/web.go` | New CLI command `helm spray web` |
| `internal/web/server.go` | HTTP server with API handlers |
| `internal/web/chart_scanner.go` | Chart scanner, release lister, helpers |
| `internal/web/websocket.go` | WebSocket hub for live log streaming |
| `internal/web/static/index.html` | Two-tab UI: Chart Explorer + Spray Execution |
| `internal/web/static/style.css` | Dark theme styling |
| `internal/web/static/app.js` | Frontend logic, WebSocket client |

## Phase 5: Unit Tests (39 test cases)

| File | Test Cases |
|------|------------|
| `pkg/util/util_test.go` | 9 test cases for `Duration()` |
| `internal/log/log_test.go` | 8 test cases for `Info()`, `WithNumberedLines()`, `Error()` |
| `internal/values/values_test.go` | 7 test cases for `mergeMaps()` |
| `internal/dependencies/dependencies_test.go` | 6 test cases for `Get()`, `tags()` |
| `internal/web/chart_scanner_test.go` | 9 test cases for `parseWeights()`, `computeExecutionOrder()`, `parseChartFile()`, `ExecCommand()` |

## Phase 6: Documentation + CI/CD

| File | Change |
|------|--------|
| `.github/workflows/ci.yml` | New GitHub Actions: build, test, lint, cross-compile |
| `CONTRIBUTING.md` | Expanded with dev setup, PR guidelines, code style |
| `AGENTS.md` | Updated with all changes, new files, conventions |

**Note:** `.travis.yml` still exists (stale, Go 1.14) — should be deleted separately.

---

## Files Modified (14)

1. `go.mod`
2. `main.go` (no change needed)
3. `cmd/root.go`
4. `cmd/web.go` (new)
5. `pkg/helm/helm.go`
6. `pkg/helmspray/helmspray.go`
7. `pkg/kubectl/kubectl.go`
8. `pkg/util/util_test.go` (new)
9. `internal/log/log.go`
10. `internal/log/log_test.go` (new)
11. `internal/values/values.go`
12. `internal/values/values_test.go` (new)
13. `internal/dependencies/dependencies.go`
14. `internal/dependencies/dependencies_test.go` (new)

## Files Created (11)

1. `cmd/web.go`
2. `internal/web/server.go`
3. `internal/web/chart_scanner.go`
4. `internal/web/chart_scanner_test.go`
5. `internal/web/websocket.go`
6. `internal/web/static/index.html`
7. `internal/web/static/style.css`
8. `internal/web/static/app.js`
9. `pkg/util/util_test.go`
10. `internal/log/log_test.go`
11. `.github/workflows/ci.yml`

## Files Updated (3)

1. `README.md` — typo fixes
2. `CONTRIBUTING.md` — expanded guidelines
3. `AGENTS.md` — updated documentation
