---
name: helm-spray-build
description: >
  Build, cross-compile, and package the helm-spray Helm plugin. Covers Go
  build, Makefile targets, plugin packaging, and release workflow. Use when
  building the binary, creating releases, or debugging build issues.
  Trigger examples: "build", "compile", "make dist", "release", "package plugin",
  "cross-compile", "build for linux".
---

# Helm Spray Build Guide

## Prerequisites

- Go 1.24.0+ (check with `go version`)
- Helm 3.x (for plugin testing)
- Make (for Makefile targets)
- Git (for plugin install from source)

## Build Commands

```bash
go build ./...              # compile check (all packages)
go build -o bin/helm-spray main.go  # build binary
go vet ./...                # static analysis
go test ./...               # run tests (currently none)
```

## Makefile Targets

```bash
make dist                   # cross-compile for all platforms
make dist_linux             # linux/amd64 only
make dist_darwin            # darwin/amd64 only
make dist_windows           # windows/amd64 only
```

Output goes to `_dist/`:
```
_dist/
├── helm-spray-linux-amd64.tar.gz
├── helm-spray-darwin-amd64.tar.gz
└── helm-spray-windows-amd64.tar.gz
```

Each tarball contains:
```
bin/helm-spray[.exe]
README.md
LICENSE
plugin.yaml
```

## Local Development

### Build and install locally

```bash
make dist_linux                    # or dist_windows / dist_darwin
helm plugin install .              # install from local directory
helm plugin list                   # verify installation
```

### Quick rebuild cycle

```bash
go build -o bin/helm-spray main.go && helm plugin install .
```

### Test the plugin

```bash
helm spray --help                  # verify it loads
helm spray --dry-run ./test-chart  # dry-run test
```

## Version Management

Version is read from `plugin.yaml`:

```yaml
name: "spray"
version: 4.0.13
```

Makefile extracts it:
```makefile
VERSION := $(shell sed -n -e 's/version:[ "]*\([^"]*\).*/\1/p' plugin.yaml)
LDFLAGS := "-X main.version=${VERSION}"
```

To bump version: edit `plugin.yaml` only — Makefile and `-ldflags` handle the rest.

## CI/CD

Travis CI (`.travis.yml`) is stale. If adding GitHub Actions:

```yaml
# .github/workflows/release.yml
name: Release
on:
  push:
    tags: ['v*']
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: make dist
      - uses: softprops/action-gh-release@v2
        with:
          files: _dist/*
```

## Plugin Install Flow

When a user runs `helm plugin install`:

1. `plugin.yaml` declares `command: "$HELM_PLUGIN_DIR/bin/helm-spray"`
2. `scripts/install_plugin.sh` runs as install hook
3. Script detects OS, downloads pre-built tarball from GitHub Releases
4. Extracts binary, README, LICENSE, plugin.yaml into plugin directory

## Common Build Issues

| Issue | Fix |
|-------|-----|
| `go: cannot find module` | Run `go mod tidy` |
| `helm: command not found` | Add Helm to PATH, or `export PATH=$PATH:/usr/local/bin` |
| `plugin not found` | Run `helm plugin install .` from repo root |
| Version shows `SNAPSHOT` | Build with `make dist` (uses `-ldflags`) not raw `go build` |
| Wrong binary name | Check `plugin.yaml` `command` field matches your OS |

## File Structure After Install

```
$HELM_PLUGIN_DIR/
├── bin/
│   └── helm-spray[.exe]
├── plugin.yaml
├── README.md
├── LICENSE
└── scripts/
    └── install_plugin.sh
```
