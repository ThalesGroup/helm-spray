# Tasks 5-6: Release Portability and Helm 4 Compatibility

This file tracks the work owned for items 5 and 6:

- Modernize release builds for Linux compatibility.
- Validate Helm 4 compatibility.

## Task 5: Portable Release Builds

### Goal

Produce release artifacts that run across common Linux distributions without depending on the build host's glibc version, and add Apple Silicon support.

### Current Problems

- Linux release binaries are built without `CGO_ENABLED=0`, so they can inherit glibc requirements from the build environment.
- The Makefile only produces `amd64` artifacts.
- Release targets run `go get -t -v ./...`, which can mutate module state during builds.
- Build artifacts are written into `bin/`, but the directory is not ignored by git.

### Implementation Checklist

- Add a reusable Makefile build target for platform-specific archives.
- Build releases with `CGO_ENABLED=0`.
- Support these archives:
  - `helm-spray-linux-amd64.tar.gz`
  - `helm-spray-linux-arm64.tar.gz`
  - `helm-spray-darwin-amd64.tar.gz`
  - `helm-spray-darwin-arm64.tar.gz`
  - `helm-spray-windows-amd64.tar.gz`
- Replace `go get -t -v ./...` with build-safe commands.
- Add `bin/` and `_dist/` to `.gitignore`.
- Verify Linux artifacts in a minimal container.

### Suggested Verification

```sh
make clean
make test
make dist
docker run --rm -v "$PWD/_dist:/dist" alpine:3.20 sh -c \
  'tar -xzf /dist/helm-spray-linux-amd64.tar.gz -C /tmp && /tmp/bin/helm-spray --help'
```

## Task 6: Helm 4 Compatibility Testing

### Goal

Provide repeatable checks proving the existing legacy CLI plugin path works with Helm 4, then document any behavior differences.

### Current Helm 4 Observations

- `helm spray --help` works under Helm `v4.2.2`.
- `helm plugin list` reports the plugin as `cli/v1 legacy`.
- A local kind smoke test can deploy a two-subchart umbrella chart with weighted releases.

### Compatibility Checklist

- Verify `helm list -o json` still matches the plugin's release parsing.
- Verify `helm upgrade --install -o json` still matches the plugin's upgrade parsing.
- Verify local chart install.
- Verify `--target`.
- Verify `--exclude`.
- Verify `--dry-run --debug`.
- Verify `--reuse-values`.
- Check whether Helm 4 emits deprecation warnings for `--force`.
- Keep the plugin as a legacy CLI plugin for the first Helm 4 compatibility pass.

### Suggested Smoke Test

```sh
make helm4-smoke
```

The smoke script builds the local plugin into an isolated temporary Helm plugin
directory, creates or reuses a kind cluster, generates a two-subchart umbrella
chart, and checks normal, `--target`, `--exclude`, and `--dry-run --debug`
flows.

### Suggested Integration Test

```sh
make helm4-integration
```

The integration script uses a fresh namespace by default and adds assertions for:

- Helm 4 legacy plugin registration.
- Baseline weighted install.
- `--target`.
- `--exclude`.
- tag filtering.
- explicit release prefixes.
- namespace-based release prefixes.
- values file overrides.
- `--reuse-values`.
- `--dry-run --debug` without release mutation.
- invalid target failure.
- conflicting `--target` and `--exclude` failure.

By default the script uses the current Kubernetes context, such as AKS, and does
not create namespaces. To run against a fixed AKS namespace and keep resources
for inspection:

```sh
KEEP_NAMESPACE=1 NAMESPACE=default scripts/helm4_integration_tests.sh
```

The user running the test must be able to list/create/update/delete Helm release
Secrets and test ConfigMaps in that namespace.

To force a local kind cluster:

```sh
make helm4-integration
```

### Cleanup

```sh
kind delete cluster --name spray-test
colima stop
```
