#!/bin/sh
set -eu

CLUSTER_NAME="${CLUSTER_NAME:-spray-test}"
NAMESPACE="${NAMESPACE:-spray-smoke}"
ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/helm-spray-smoke.XXXXXX")"
HELM_PLUGINS_DIR="${WORK_DIR}/helm-plugins"

cleanup() {
    rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

require() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "Missing required command: $1" >&2
        exit 1
    fi
}

require go
require helm
require kind
require kubectl

mkdir -p "${HELM_PLUGINS_DIR}/spray/bin"
cd "${ROOT_DIR}"
go build -o "${HELM_PLUGINS_DIR}/spray/bin/helm-spray" main.go
cp plugin.yaml README.md LICENSE "${HELM_PLUGINS_DIR}/spray/"

export HELM_PLUGINS="${HELM_PLUGINS_DIR}"

if ! kind get clusters | grep -qx "${CLUSTER_NAME}"; then
    kind create cluster --name "${CLUSTER_NAME}"
fi

kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null
kubectl get namespace "${NAMESPACE}" >/dev/null 2>&1 || kubectl create namespace "${NAMESPACE}" >/dev/null

mkdir -p "${WORK_DIR}/chart/charts/app-a/templates" "${WORK_DIR}/chart/charts/app-b/templates"

cat > "${WORK_DIR}/chart/Chart.yaml" <<'EOF'
apiVersion: v2
name: helm-spray-smoke
version: 0.1.0
dependencies:
  - name: app-a
    version: 0.1.0
    repository: file://charts/app-a
    condition: app-a.enabled
  - name: app-b
    version: 0.1.0
    repository: file://charts/app-b
    condition: app-b.enabled
EOF

cat > "${WORK_DIR}/chart/values.yaml" <<'EOF'
spray:
  weights:
    app-a: 0
    app-b: 1
app-a:
  enabled: true
app-b:
  enabled: true
EOF

cat > "${WORK_DIR}/chart/charts/app-a/Chart.yaml" <<'EOF'
apiVersion: v2
name: app-a
version: 0.1.0
appVersion: "1.0"
EOF

cat > "${WORK_DIR}/chart/charts/app-a/values.yaml" <<'EOF'
enabled: true
EOF

cat > "${WORK_DIR}/chart/charts/app-a/templates/configmap.yaml" <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-a-smoke
data:
  app: app-a
EOF

cat > "${WORK_DIR}/chart/charts/app-b/Chart.yaml" <<'EOF'
apiVersion: v2
name: app-b
version: 0.1.0
appVersion: "1.0"
EOF

cat > "${WORK_DIR}/chart/charts/app-b/values.yaml" <<'EOF'
enabled: true
EOF

cat > "${WORK_DIR}/chart/charts/app-b/templates/configmap.yaml" <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-b-smoke
data:
  app: app-b
EOF

helm dependency build "${WORK_DIR}/chart"
helm --namespace "${NAMESPACE}" spray "${WORK_DIR}/chart" --verbose --timeout 60
helm --namespace "${NAMESPACE}" spray "${WORK_DIR}/chart" --target app-a --verbose --timeout 60
helm --namespace "${NAMESPACE}" spray "${WORK_DIR}/chart" --exclude app-b --verbose --timeout 60
helm --namespace "${NAMESPACE}" spray "${WORK_DIR}/chart" --dry-run --debug --timeout 60

helm --namespace "${NAMESPACE}" list
kubectl --namespace "${NAMESPACE}" get configmap app-a-smoke app-b-smoke
