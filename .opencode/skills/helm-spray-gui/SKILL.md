---
name: helm-spray-gui
description: >
  Web GUI development guide for helm-spray. Covers the Go HTTP server,
  WebSocket real-time log streaming, chart browser/explorer, and spray
  execution panel. Use when building or modifying the web interface.
  Trigger examples: "web gui", "frontend", "chart explorer", "web interface",
  "http server", "websocket", "spray ui".
---

# Helm Spray Web GUI

## Architecture

```
Browser (HTML/JS)  ←——WebSocket——→  Go HTTP Server  ——exec——→  helm spray
       ↑                                ↑
       └── REST API ────────────────────┘
```

## Tech Stack

- **Backend**: Go `net/http` (standard library) + `gorilla/websocket`
- **Frontend**: Single `index.html` with embedded CSS + vanilla JS (no build step)
- **No frameworks**: no React, no Vue, no npm

## Directory Structure

```
internal/web/
├── static/
│   ├── index.html          # Single-page app
│   ├── style.css           # Styles
│   └── app.js              # Frontend logic
├── server.go               # HTTP server + routes
├── handlers.go             # API endpoint handlers
├── websocket.go            # WebSocket hub + broadcast
└── chart_scanner.go        # Local chart directory scanner
```

## Backend Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/` | Serve `index.html` |
| `GET` | `/api/charts?dir=<path>` | Scan directory for Helm charts |
| `GET` | `/api/chart?path=<path>` | Chart metadata + dependencies |
| `GET` | `/api/releases?ns=<namespace>` | List helm releases |
| `POST` | `/api/spray` | Start spray execution |
| `POST` | `/api/spray/stop` | Cancel running spray |
| `GET` | `/ws` | WebSocket for real-time logs |

## API Shapes

### GET /api/charts?dir=/path/to/charts

```json
{
  "charts": [
    {
      "name": "my-umbrella",
      "version": "0.1.0",
      "path": "/path/to/my-umbrella",
      "hasRequirements": true
    }
  ]
}
```

### GET /api/chart?path=/path/to/my-umbrella

```json
{
  "name": "my-umbrella",
  "version": "0.1.0",
  "description": "Umbrella chart for microservices",
  "dependencies": [
    {
      "name": "database",
      "alias": "",
      "usedName": "database",
      "weight": 0,
      "targeted": true,
      "hasTags": false,
      "allowedByTags": true,
      "correspondingReleaseName": "database"
    }
  ],
  "maxWeight": 2,
  "executionOrder": [
    {"weight": 0, "charts": ["database"]},
    {"weight": 1, "charts": ["api-server", "cache"]},
    {"weight": 2, "charts": ["frontend"]}
  ]
}
```

### POST /api/spray

Request:
```json
{
  "chartPath": "/path/to/chart",
  "namespace": "default",
  "dryRun": false,
  "verbose": true,
  "debug": false,
  "force": false,
  "timeout": 300,
  "targets": [],
  "excludes": [],
  "valuesFiles": [],
  "setValues": {}
}
```

Response:
```json
{
  "jobId": "spray-1719320000",
  "status": "started"
}
```

### WebSocket Messages

```json
// Log line
{"type": "log", "level": 1, "message": "[spray] deploying solution chart..."}

// Status update
{"type": "status", "status": "running", "currentWeight": 0, "totalWeights": 3}

// Completion
{"type": "complete", "status": "success", "duration": "2m30s"}

// Error
{"type": "error", "message": "timed out waiting for readiness"}
```

## Frontend Layout

### Tab 1: Chart Explorer

```
┌─────────────────────────────────────────────────────┐
│  Chart Directory: [/path/to/charts]    [Browse]     │
├─────────────────────────────────────────────────────┤
│  ┌──────────┐  ┌──────────┐  ┌──────────┐          │
│  │ umbrella │  │ service  │  │ infra    │          │
│  │ 0.1.0    │  │ 1.2.0    │  │ 0.3.0    │          │
│  └──────────┘  └──────────┘  └──────────┘          │
├─────────────────────────────────────────────────────┤
│  Selected: umbrella-chart                           │
│  Dependencies:                                      │
│  ┌────────────┬───────┬────────┬──────────┐         │
│  │ Name       │ Alias │ Weight │ Targeted │         │
│  ├────────────┼───────┼────────┼──────────┤         │
│  │ database   │ -     │ 0      │ true     │         │
│  │ api-server │ -     │ 1      │ true     │         │
│  │ frontend   │ web   │ 2      │ true     │         │
│  └────────────┴───────┴────────┴──────────┘         │
│                                                     │
│  Execution Order:                                   │
│  [database] → [api-server] → [frontend]             │
└─────────────────────────────────────────────────────┘
```

### Tab 2: Spray Execution

```
┌─────────────────────────────────────────────────────┐
│  Chart: [umbrella-chart]  Namespace: [default]      │
│                                                     │
│  [x] Dry Run   [ ] Verbose   [ ] Debug   [ ] Force │
│  Timeout: [300]s                                    │
│  Targets: [                  ] (comma-separated)    │
│  Excludes: [                 ] (comma-separated)    │
│  Values Files: [Browse]                             │
│                                                     │
│  [🚀 Execute Spray]                                 │
├─────────────────────────────────────────────────────┤
│  Status: Running (weight 1/3)    Duration: 0:45     │
│                                                     │
│  ┌─ Live Output ──────────────────────────────────┐ │
│  │ [spray] deploying solution chart "umbrella"    │ │
│  │ [spray]   > processing sub-charts of weight 0  │ │
│  │ [spray]     o upgrading release "database"...  │ │
│  │ [spray]   > waiting for liveness and readiness │ │
│  │ [spray]   > processing sub-charts of weight 1  │ │
│  │ [spray]     o upgrading release "api-server"...│ │
│  └────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

## Implementation Notes

### Gorilla WebSocket

Already in `go.sum` as transitive dependency. Add to `go.mod`:

```go
import "github.com/gorilla/websocket"
```

### Chart Scanner

Use `helm.sh/helm/v3/pkg/chart/loader` (already imported) to load charts:

```go
func scanCharts(dir string) ([]ChartInfo, error) {
    entries, err := os.ReadDir(dir)
    // ...
    for _, entry := range entries {
        if entry.IsDir() {
            chart, err := loader.Load(filepath.Join(dir, entry.Name()))
            // extract metadata
        }
    }
}
```

### Spray Execution

Use `os/exec` to run `helm spray` as subprocess, pipe output to WebSocket:

```go
cmd := exec.Command("helm", "spray", "--dry-run", chartPath)
stdout, _ := cmd.StdoutPipe()
scanner := bufio.NewScanner(stdout)
go func() {
    for scanner.Scan() {
        broadcast(LogMessage{Text: scanner.Text()})
    }
}()
cmd.Run()
```

### CORS

Since frontend and backend run on same origin (`localhost:8080`), no CORS needed.

### Port

Default: `8080`. Configurable via `--web-port` flag or `HELM_SPRAY_PORT` env var.

## File List

When creating the GUI, these files need to be created:

1. `internal/web/server.go` — HTTP server, routes, static file serving
2. `internal/web/handlers.go` — REST API handlers
3. `internal/web/websocket.go` — WebSocket hub, broadcast, client management
4. `internal/web/chart_scanner.go` — Chart directory scanning
5. `internal/web/static/index.html` — Single-page app HTML
6. `internal/web/static/style.css` — Styles
7. `internal/web/static/app.js` — Frontend JavaScript
8. `cmd/root.go` — Add `--web-port` flag and web server startup
