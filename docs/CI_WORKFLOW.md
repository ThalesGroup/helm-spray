# CI Workflow Contribution

## What was done

Added a full CI pipeline for both GitHub and GitLab, covering build, test, formatting, and SonarQube quality gate. The project previously had no test gate of any kind.

**Branch:** `feat/test-suite`

---

## GitHub Actions (`.github/workflows/`)

### New workflow: `ci.yaml`

Runs on every push to any branch and on every pull request.

```yaml
name: CI
on:
  push:
    branches: ['**']
  pull_request:
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go vet ./...
      - run: go test ./...
```

### Updated workflow: `github-release.yaml`

| Field | Before | After |
|---|---|---|
| `actions/checkout` | `@v2` | `@v4` |
| `actions/setup-go` | `@v2` | `@v5` |
| Go version | `'1.22'` (hardcoded) | `go-version-file: go.mod` |

---

## GitLab CI (`.gitlab-ci.yml`)

Four-stage pipeline: `build → test → scan → report`.

### Stages

**`build`** — compiles the binary using `.golang:build` from the NextGen catalog step. Injects the version from `plugin.yaml` via `-ldflags`.

**`test`** — two parallel jobs:
- `go-test`: runs `go vet ./...` then the full test suite via `.golang:test`. Produces `coverage.txt`.
- `go-fmt`: checks formatting via `.golang:fmt`.

**`scan`** — SonarQube quality gate via the `nextgen-cicd/catalog/step/sonarqube/scan@6` component. Runs only after both test jobs pass.

```yaml
sonar-scan:
  needs: [build, go-test, go-fmt]
  variables:
    CICD_SONAR_CUSTOM_PARAM: >-
      -Dsonar.go.coverage.reportPaths=$CI_PROJECT_DIR/coverage.txt
      -Dsonar.coverage.exclusions=**/*_test.go
```

The `sonar.coverage.exclusions` flag is required — without it SonarQube counts test file lines in the denominator, which artificially lowers the reported coverage percentage.

**`report`** — posts SonarQube findings (bugs, vulnerabilities, code smells) back to the GitLab MR or branch via `sonar-helper-issues@6`.

---

## Why `sonar.coverage.exclusions` matters

SonarQube by default includes `*_test.go` files when computing coverage. Since test files contain many lines that are never instrumented by the Go coverage tool, they inflate the "lines to cover" count and suppress the percentage. Adding `-Dsonar.coverage.exclusions=**/*_test.go` excludes them, bringing the reported figure in line with `go tool cover`.