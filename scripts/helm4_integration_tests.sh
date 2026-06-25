#!/bin/sh
set -eu

CLUSTER_NAME="${CLUSTER_NAME:-spray-test}"
NAMESPACE="${NAMESPACE:-spray-itest-$$}"
USE_EXISTING_CLUSTER="${USE_EXISTING_CLUSTER:-1}"
CREATE_NAMESPACE="${CREATE_NAMESPACE:-}"
ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/helm-spray-itest.XXXXXX")"
HELM_PLUGINS_DIR="${WORK_DIR}/helm-plugins"
CREATED_NAMESPACE=0

# Usage:
#   scripts/helm4_integration_tests.sh
#   USE_EXISTING_CLUSTER=0 scripts/helm4_integration_tests.sh
#
# By default the script uses the current kubectl context, such as an AKS test
# cluster. Set USE_EXISTING_CLUSTER=0 to create or reuse a local kind cluster
# named by CLUSTER_NAME. On existing clusters, pass NAMESPACE for a namespace
# where your user can list/create/update/delete Helm release Secrets and create
# ConfigMaps. Set CREATE_NAMESPACE=1 only when your user may create namespaces.

cleanup() {
    if [ "${KEEP_NAMESPACE:-0}" != "1" ] && [ "${CREATED_NAMESPACE}" = "1" ]; then
        kubectl delete namespace "${NAMESPACE}" --ignore-not-found >/dev/null 2>&1 || true
    fi
    rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

require() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "Missing required command: $1" >&2
        exit 1
    fi
}

log() {
    printf '\n==> %s\n' "$1"
}

assert_contains() {
    haystack="$1"
    needle="$2"
    if ! printf '%s' "${haystack}" | grep -Fq -- "${needle}"; then
        echo "Expected output to contain: ${needle}" >&2
        exit 1
    fi
}

assert_release_exists() {
    release="$1"
    helm --namespace "${NAMESPACE}" status "${release}" >/dev/null
}

assert_release_missing() {
    release="$1"
    if helm --namespace "${NAMESPACE}" status "${release}" >/dev/null 2>&1; then
        echo "Expected release to be missing: ${release}" >&2
        exit 1
    fi
}

assert_configmap_exists() {
    name="$1"
    kubectl --namespace "${NAMESPACE}" get configmap "${name}" >/dev/null
}

expect_failure() {
    name="$1"
    shift
    log "${name}"
    set +e
    output="$("$@" 2>&1)"
    status=$?
    set -e
    if [ "${status}" -eq 0 ]; then
        echo "Expected command to fail: $*" >&2
        exit 1
    fi
    printf '%s\n' "${output}"
}

check_can_i() {
    verb="$1"
    resource="$2"
    if ! kubectl auth can-i "${verb}" "${resource}" --namespace "${NAMESPACE}" >/dev/null 2>&1; then
        echo "Missing Kubernetes permission in namespace \"${NAMESPACE}\": ${verb} ${resource}" >&2
        return 1
    fi
}

preflight_namespace_permissions() {
    if [ "${USE_EXISTING_CLUSTER}" != "1" ]; then
        return
    fi

    missing=0
    for verb in get list create update patch delete; do
        check_can_i "${verb}" secrets || missing=1
    done
    for verb in get list create update patch delete; do
        check_can_i "${verb}" configmaps || missing=1
    done

    if [ "${missing}" = "1" ]; then
        cat >&2 <<EOF

Helm stores release state in Kubernetes Secrets by default. Helm Spray calls
'helm list' and 'helm upgrade --install', so the test user needs Secret access
in namespace "${NAMESPACE}".

Ask a cluster admin for a Role/RoleBinding in that fixed namespace, or run this
test in a namespace where your user has Helm release permissions.
EOF
        exit 1
    fi
}

write_chart() {
    chart_dir="$1"
    mkdir -p "${chart_dir}/charts/app-a/templates" "${chart_dir}/charts/app-b/templates" "${chart_dir}/charts/app-c/templates"

    cat > "${chart_dir}/Chart.yaml" <<'EOF'
apiVersion: v2
name: helm-spray-itest
version: 0.1.0
dependencies:
  - name: app-a
    version: 0.1.0
    repository: file://charts/app-a
    condition: app-a.enabled
    tags:
      - core
  - name: app-b
    version: 0.1.0
    repository: file://charts/app-b
    condition: app-b.enabled
    tags:
      - optional
  - name: app-c
    version: 0.1.0
    repository: file://charts/app-c
    condition: app-c.enabled
EOF

    cat > "${chart_dir}/values.yaml" <<'EOF'
tags:
  core: true
  optional: true
app-a:
  enabled: true
  weight: 0
  marker: default-a
app-b:
  enabled: true
  weight: 1
  marker: default-b
app-c:
  enabled: true
  weight: 2
  marker: default-c
EOF

    for app in app-a app-b app-c; do
        cat > "${chart_dir}/charts/${app}/Chart.yaml" <<EOF
apiVersion: v2
name: ${app}
version: 0.1.0
appVersion: "1.0"
EOF

        cat > "${chart_dir}/charts/${app}/values.yaml" <<'EOF'
enabled: true
weight: 0
marker: subchart-default
EOF

        cat > "${chart_dir}/charts/${app}/templates/configmap.yaml" <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-smoke
data:
  app: ${app}
  marker: {{ .Values.marker | quote }}
EOF
    done
}

require go
require helm
require kubectl

if [ "${USE_EXISTING_CLUSTER}" = "0" ]; then
    require kind
fi

mkdir -p "${HELM_PLUGINS_DIR}/spray/bin"
cd "${ROOT_DIR}"
go build -o "${HELM_PLUGINS_DIR}/spray/bin/helm-spray" main.go
cp plugin.yaml README.md LICENSE "${HELM_PLUGINS_DIR}/spray/"
export HELM_PLUGINS="${HELM_PLUGINS_DIR}"

if [ "${USE_EXISTING_CLUSTER}" = "1" ]; then
    log "Using current kubectl context"
elif [ "${USE_EXISTING_CLUSTER}" = "0" ]; then
    if ! kind get clusters | grep -qx "${CLUSTER_NAME}"; then
        kind create cluster --name "${CLUSTER_NAME}"
    fi

    kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null
else
    echo "USE_EXISTING_CLUSTER must be 1 or 0" >&2
    exit 1
fi

if [ -z "${CREATE_NAMESPACE}" ]; then
    if [ "${USE_EXISTING_CLUSTER}" = "0" ]; then
        CREATE_NAMESPACE=1
    else
        CREATE_NAMESPACE=0
    fi
fi

if ! kubectl get namespace "${NAMESPACE}" >/dev/null 2>&1; then
    if [ "${CREATE_NAMESPACE}" = "1" ]; then
        kubectl create namespace "${NAMESPACE}" >/dev/null
        CREATED_NAMESPACE=1
    else
        log "Skipping namespace creation for ${NAMESPACE}"
    fi
fi

preflight_namespace_permissions

CHART_DIR="${WORK_DIR}/chart"
write_chart "${CHART_DIR}"
helm dependency build "${CHART_DIR}" >/dev/null

log "Plugin is registered as a Helm 4 legacy CLI plugin"
plugin_list="$(helm plugin list)"
printf '%s\n' "${plugin_list}"
assert_contains "${plugin_list}" "spray"
assert_contains "${plugin_list}" "legacy"

log "Case 1: baseline weighted install deploys all releases"
helm --namespace "${NAMESPACE}" spray "${CHART_DIR}" --verbose --timeout 60
assert_release_exists app-a
assert_release_exists app-b
assert_release_exists app-c
assert_configmap_exists app-a-smoke
assert_configmap_exists app-b-smoke
assert_configmap_exists app-c-smoke

log "Case 2: --target only upgrades the selected release"
before_a="$(helm --namespace "${NAMESPACE}" status app-a -o json)"
before_b="$(helm --namespace "${NAMESPACE}" status app-b -o json)"
helm --namespace "${NAMESPACE}" spray "${CHART_DIR}" --target app-a --verbose --timeout 60
after_a="$(helm --namespace "${NAMESPACE}" status app-a -o json)"
after_b="$(helm --namespace "${NAMESPACE}" status app-b -o json)"
if [ "${before_a}" = "${after_a}" ]; then
    echo "Expected app-a status to change after --target app-a" >&2
    exit 1
fi
if [ "${before_b}" != "${after_b}" ]; then
    echo "Expected app-b status to remain unchanged after --target app-a" >&2
    exit 1
fi

log "Case 3: --exclude skips the excluded release"
before_b="$(helm --namespace "${NAMESPACE}" status app-b -o json)"
helm --namespace "${NAMESPACE}" spray "${CHART_DIR}" --exclude app-b --verbose --timeout 60
after_b="$(helm --namespace "${NAMESPACE}" status app-b -o json)"
if [ "${before_b}" != "${after_b}" ]; then
    echo "Expected app-b status to remain unchanged after --exclude app-b" >&2
    exit 1
fi

log "Case 4: tag filtering can skip tagged dependencies"
helm --namespace "${NAMESPACE}" spray "${CHART_DIR}" --prefix-releases tagskip --set tags.optional=false --verbose --timeout 60
assert_release_exists tagskip-app-a
assert_release_missing tagskip-app-b
assert_release_exists tagskip-app-c

log "Case 5: explicit release prefix creates independent release names"
helm --namespace "${NAMESPACE}" spray "${CHART_DIR}" --prefix-releases pref --verbose --timeout 60
assert_release_exists pref-app-a
assert_release_exists pref-app-b
assert_release_exists pref-app-c
assert_configmap_exists pref-app-a-smoke

log "Case 6: namespace prefix creates independent release names"
helm --namespace "${NAMESPACE}" spray "${CHART_DIR}" --prefix-releases-with-namespace --target app-c --verbose --timeout 60
assert_release_exists "${NAMESPACE}-app-c"
assert_configmap_exists "${NAMESPACE}-app-c-smoke"

log "Case 7: values file overrides are passed through"
cat > "${WORK_DIR}/override-values.yaml" <<'EOF'
app-a:
  marker: from-values-file
EOF
helm --namespace "${NAMESPACE}" spray "${CHART_DIR}" --prefix-releases vals -f "${WORK_DIR}/override-values.yaml" --target app-a --verbose --timeout 60
marker="$(kubectl --namespace "${NAMESPACE}" get configmap vals-app-a-smoke -o jsonpath='{.data.marker}')"
if [ "${marker}" != "from-values-file" ]; then
    echo "Expected values file marker override, got: ${marker}" >&2
    exit 1
fi

log "Case 8: --reuse-values remains accepted on Helm 4"
helm --namespace "${NAMESPACE}" spray "${CHART_DIR}" --target app-a --reuse-values --verbose --timeout 60

log "Case 9: dry-run debug uses Helm 4 JSON output without mutating releases"
before_a="$(helm --namespace "${NAMESPACE}" status app-a -o json)"
dry_output="$(helm --namespace "${NAMESPACE}" spray "${CHART_DIR}" --dry-run --debug --timeout 60 2>&1)"
printf '%s\n' "${dry_output}"
assert_contains "${dry_output}" "--dry-run=client"
after_a="$(helm --namespace "${NAMESPACE}" status app-a -o json)"
if [ "${before_a}" != "${after_a}" ]; then
    echo "Expected dry-run not to mutate app-a" >&2
    exit 1
fi

invalid_target_output="$(expect_failure "Case 10: invalid target fails validation" \
    helm --namespace "${NAMESPACE}" spray "${CHART_DIR}" --target not-a-chart --timeout 60)"
assert_contains "${invalid_target_output}" "invalid targetted sub-chart"

conflicting_flags_output="$(expect_failure "Case 11: conflicting target/exclude fails validation" \
    helm --namespace "${NAMESPACE}" spray "${CHART_DIR}" --target app-a --exclude app-b --timeout 60)"
assert_contains "${conflicting_flags_output}" "cannot use both --target and --exclude together"

log "All Helm 4 integration tests passed in namespace ${NAMESPACE}"
