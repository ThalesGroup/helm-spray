# AGENTS.md — helm-spray

## What This Is

Helm plugin that deploys umbrella chart sub-charts sequentially by weight.
Each sub-chart becomes its own Helm release. Written in Go, shells out to
`helm` and `kubectl` CLI binaries (not Go client libraries).

Module: `github.com/gemalto/helm-spray/v4` — Go 1.24.0, Helm v4.

## Build & Verify

```bash
go build ./...              # compile check
go vet ./...                # static analysis
go test ./...               # unit tests
go test -race ./...         # tests with race detection
make dist                   # cross-compile for linux/darwin/windows
make dist_linux             # single platform
```

**Linting:** `.github/workflows/ci.yml` runs `golangci-lint` in CI.
Run locally with `golangci-lint run` if installed.

## Key Files

| Path | Role |
|------|------|
| `main.go` | Entry point — calls `cmd.NewRootCmd().Execute()` |
| `cmd/root.go` | Cobra CLI, flag binding, chart fetch, calls `s.Spray()` |
| `cmd/web.go` | Web GUI command — starts HTTP server for chart browser + spray execution |
| `pkg/helmspray/helmspray.go` | Core orchestration — `Spray()`, `upgrade()`, `wait()` |
| `pkg/helm/helm.go` | Wraps `helm` CLI via `exec.Command` |
| `pkg/kubectl/kubectl.go` | Wraps `kubectl` CLI via `exec.Command` |
| `pkg/util/util.go` | Duration formatting utility |
| `internal/values/values.go` | Values merge + `#! {{ .Files.Get }}` include processing |
| `internal/dependencies/dependencies.go` | Weight, tags, targeting computation |
| `internal/log/log.go` | Simple `[spray]` prefixed logging |
| `internal/web/server.go` | Web GUI HTTP server, API handlers |
| `internal/web/chart_scanner.go` | Chart directory scanner, release lister |
| `internal/web/websocket.go` | WebSocket hub for live log streaming |
| `internal/web/static/` | Frontend HTML/CSS/JS for web GUI |

## Architecture Gotcha

The `Spray` struct mixes config and runtime state. `deployments`, `statefulSets`,
`jobs` are set during `upgrade()` and read during `wait()` — all in the same
struct. When modifying deployment logic, be aware this shared mutable state
spans both methods.

## Known Issues (Fixed)

All of these were fixed in the feature/AI-improvements_EC branch:

- **Deprecated `ioutil`** — replaced with `os.MkdirTemp` / `os.CreateTemp`
- **Shell injection in `Fetch`** — replaced shell commands with `os.ReadDir`
- **No bounds check** — added bounds checking after `os.ReadDir`
- **`reflect.TypeOf` for type switching** — replaced with `switch v.(type)`
- **Silent error ignore** — now handles `strconv.Atoi` error properly
- **Infinite loop in `WithNumberedLines`** — fixed with separate counter variable
- **Redundant booleans** — removed `== true` / `== false` comparisons
- **README typos** — fixed "thei weigths", "primarilly", "temporarilly", "cwthis", "con be"
- **Wait algorithm issues (#60, #58)** — separate checks for Deployments vs StatefulSets

## Conventions

- **Logging**: All output uses `internal/log` — `Info(level, fmt, args...)`.
  Levels: 1=brief, 2=detail, 3=trace. Never use `fmt.Println` directly.
- **Error wrapping**: `fmt.Errorf("context: %w", err)` pattern.
- **No interfaces on helm/kubectl wrappers** — they're concrete functions,
  making unit testing difficult. If adding tests, mock at the command exec
  level or refactor to interfaces.
- **Proprietary `#!` directives** in `values.yaml` — not standard Helm or YAML.
  See `internal/values/values.go` for the regex parser.
- **Web GUI**: Go `net/http` + `gorilla/websocket` + static HTML/CSS/JS.
  No build step required. Run with `helm spray web`.

## Testing

Test files exist for:

- `pkg/util/util_test.go` — Duration function (9 test cases)
- `internal/log/log_test.go` — Info, WithNumberedLines, Error functions (8 test cases)
- `internal/values/values_test.go` — mergeMaps function (7 test cases)
- `internal/dependencies/dependencies_test.go` — Get, tags functions (6 test cases)
- `internal/web/chart_scanner_test.go` — parseWeights, computeExecutionOrder, parseChartFile, ExecCommand (9 test cases)

No test framework required — standard `testing` package.

## CI

`.github/workflows/ci.yml` provides:

- Build and test on Go 1.24 and 1.25
- Race detection
- Coverage reporting
- golangci-lint
- Cross-compilation (Linux, macOS, Windows)
- Artifact upload

`.travis.yml` is stale (Go 1.14, deprecated for open-source) and should be deleted.

## Branches

- `master` — main branch
- `feature/AI-improvements_EC` — current feature branch with all improvements
- `remotes/origin/AU_Thales-Against-The-Machine_2026June` — hackathon branch

## Web GUI

Run the web GUI with:

```bash
helm spray web --addr :8080 --chart-dir . --namespace default
```

Features:
- **Chart Explorer**: Browse umbrella charts, view dependencies and execution order
- **Spray Execution**: Configure and run spray operations with live WebSocket log streaming

Endpoints:
- `GET /` — Web GUI
- `GET /api/charts` — List umbrella charts
- `GET /api/chart?name=<name>` — Get chart details
- `GET /api/releases` — List Helm releases
- `POST /api/spray` — Start spray execution
- `GET /ws` — WebSocket for live logs
