# Unified Tasks 1-6 Summary

This branch integrates six related Helm Spray maintenance tasks on top of
`origin/master`. The combined goal is to make chart retrieval safer, preserve
Helm chart schema compatibility, improve deployment selection and readiness
behavior, produce portable release artifacts, and prove the legacy CLI plugin
path against Helm 4.

## Scope

1. Replace `helm fetch` with `helm pull` and remove shell copy logic.
2. Fix weight storage to avoid chart schema conflicts.
3. Rework or soften `enabled` and `condition` behavior.
4. Improve readiness handling, especially hook-only charts and StatefulSet
   `OnDelete`.
5. Modernize release builds for Linux compatibility.
6. Add Helm 4 compatibility testing.

## Upstream Issue Context

The work maps to long-running open issues in the original repository:

- [#93](https://github.com/ThalesGroup/helm-spray/issues/93): root-level
  `weight` is rejected by chart schema validation.
- [#83](https://github.com/ThalesGroup/helm-spray/issues/83): released Linux
  binaries can require a newer glibc than target hosts provide.
- [#75](https://github.com/ThalesGroup/helm-spray/issues/75) and
  [#34](https://github.com/ThalesGroup/helm-spray/issues/34): chart
  `enabled` and `condition` behavior needs to be less surprising.
- [#67](https://github.com/ThalesGroup/helm-spray/issues/67): chart reference
  handling needs to work with current Helm commands.
- [#60](https://github.com/ThalesGroup/helm-spray/issues/60) and
  [#58](https://github.com/ThalesGroup/helm-spray/issues/58): readiness waits
  can block incorrectly for StatefulSets and related resources.
- [#13](https://github.com/ThalesGroup/helm-spray/issues/13): hook-only charts
  should not fail readiness handling.

## Integrated Behavior

- Chart downloads now use `helm pull --untar` and avoid ad hoc shell copy
  commands.
- Sub-chart weights are read from `spray.weights.<chart-or-alias>` instead of
  root-level chart fields, keeping umbrella charts compatible with stricter
  schemas.
- Conditions and tags remain compatible with Helm Spray targeting while reducing
  unwanted installs from disabled dependencies.
- Readiness handling avoids waiting forever when charts only contain hooks and
  treats StatefulSet `OnDelete` behavior more carefully.
- Release builds are produced with `CGO_ENABLED=0` for Linux portability and now
  cover Linux, Darwin, and Windows artifacts.
- Helm 4 smoke and integration tests validate legacy `cli/v1` plugin behavior,
  targeting, excludes, tags, prefixes, values overrides, reuse-values, dry-run,
  and expected failure cases.

## Manual Verification

Run the normal Go tests:

```sh
make test
```

Build release artifacts and verify the Linux artifact in a minimal container:

```sh
make clean
make dist
docker run --rm -v "$PWD/_dist:/dist" alpine:3.20 sh -c \
  'tar -xzf /dist/helm-spray-linux-amd64.tar.gz -C /tmp && /tmp/bin/helm-spray --help'
```

Run the Helm 4 smoke test:

```sh
make helm4-smoke
```

The smoke script builds the local plugin into an isolated temporary Helm plugin
directory, generates a two-subchart umbrella chart, and checks normal,
`--target`, `--exclude`, and `--dry-run --debug` flows.

## Helm 4 Integration Procedure

The integration script asserts:

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

### iMac Local Test

Use the make target. It forces kind mode, creates a disposable namespace, and
cleans that namespace automatically:

```sh
make helm4-integration
```

Equivalent direct command:

```sh
USE_EXISTING_CLUSTER=0 scripts/helm4_integration_tests.sh
```

Local cleanup:

```sh
kind delete cluster --name spray-test
colima stop
```

### Azure Kubernetes Node Test

Use the fixed namespace assigned to your Azure identity. Do not rely on the
current kubectl namespace; pass `NAMESPACE` explicitly:

```sh
KEEP_NAMESPACE=1 NAMESPACE=customer-namespaces scripts/helm4_integration_tests.sh
```

Replace `customer-namespaces` with the fixed namespace you are allowed to use.
`KEEP_NAMESPACE=1` leaves resources behind for inspection.

Azure inspection:

```sh
helm -n customer-namespaces list
kubectl -n customer-namespaces get configmaps
```

Azure cleanup, preserving non-test releases such as `echo`:

```sh
helm -n customer-namespaces uninstall \
  app-a app-b app-c \
  tagskip-app-a tagskip-app-c \
  pref-app-a pref-app-b pref-app-c \
  customer-namespaces-app-c \
  vals-app-a
```

The Azure user running the test must be able to list/create/update/delete Helm
release Secrets and test ConfigMaps in that namespace.

## CI/CD Recommendation

The release workflow should use the same Go version declared by `go.mod`
(`go1.24.1`) so GitHub Actions does not fail before building. A future CI
workflow can split validation into three jobs:

- `go-test`: run `go test ./...` and `make build`.
- `release-build`: run `make dist` and verify the Linux archive in a minimal
  container.
- `helm4-integration`: install Helm 4, start a kind cluster, and run
  `make helm4-integration`.

Keep Azure fixed-namespace testing as a manual or protected-environment job
because it requires namespace-specific RBAC for Helm release Secrets and test
ConfigMaps.

## Suggested Commit Message

```text
Finalize integrated Helm Spray maintenance tasks

Integrate the six-task maintenance branch covering chart pull behavior, schema
safe weight storage, condition handling, readiness fixes, portable release
builds, and Helm 4 compatibility tests.

The change addresses upstream issue themes from ThalesGroup/helm-spray,
including #93, #83, #75, #34, #67, #60, #58, and #13. It also updates release
CI to use the Go toolchain required by go.mod.
```
