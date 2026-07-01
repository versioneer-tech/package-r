#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-package-r-e2e-s3}"
KIND_CONTEXT="${KIND_CONTEXT:-kind-${KIND_CLUSTER_NAME}}"
K8S_NAMESPACE="${K8S_NAMESPACE:-package-r}"
CSI_RCLONE_NAMESPACE="${CSI_RCLONE_NAMESPACE:-workspace}"
CSI_RCLONE_RELEASE="${CSI_RCLONE_RELEASE:-package-r-e2e-csi-rclone}"
CSI_RCLONE_CHART="${CSI_RCLONE_CHART:-oci://ghcr.io/eoepca/workspace/workspace-dependencies-csi-rclone}"
CSI_RCLONE_CHART_VERSION="${CSI_RCLONE_CHART_VERSION:-2.1.0}"

PACKAGE_R_IMAGE="${PACKAGE_R_IMAGE:-package-r:e2e-s3-k8s}"
PACKAGE_R_K8S_MANIFEST="${PACKAGE_R_K8S_MANIFEST:-$ROOT_DIR/docs/examples/kubernetes-csi-rclone.yaml}"
PACKAGE_R_K8S_KEEP_CLUSTER="${PACKAGE_R_K8S_KEEP_CLUSTER:-false}"
PACKAGE_R_K8S_KEEP_FORWARD="${PACKAGE_R_K8S_KEEP_FORWARD:-false}"
PACKAGE_R_K8S_SKIP_IMAGE_BUILD="${PACKAGE_R_K8S_SKIP_IMAGE_BUILD:-false}"
PACKAGE_R_K8S_SKIP_IMAGE_LOAD="${PACKAGE_R_K8S_SKIP_IMAGE_LOAD:-false}"
PACKAGE_R_K8S_ROLLOUT_TIMEOUT="${PACKAGE_R_K8S_ROLLOUT_TIMEOUT:-300s}"

PUBLIC_SHARE_HASH="${PUBLIC_SHARE_HASH:-public-share}"
ITEM_ID="${ITEM_ID:-67793f0b9478720001790586}"
BUCKET_NAME="${BUCKET_NAME:-package-r-e2e}"
AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-package-r-e2e}"
AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-package-r-e2e-secret}"
AWS_REGION="${AWS_REGION:-us-east-1}"
RCLONE_BIN="${RCLONE_BIN:-rclone}"
S3_FORWARD_PORT="${S3_FORWARD_PORT:-}"
PACKAGE_R_FORWARD_PORT="${PACKAGE_R_FORWARD_PORT:-}"
MINIO_IMAGE="${MINIO_IMAGE:-quay.io/minio/minio:latest}"

tmp_dir=""
created_cluster=false
s3_port_forward_pid=""
package_r_port_forward_pid=""
package_r_manifest=""

log() {
  printf '[e2e-s3-k8s] %s\n' "$*"
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log "missing required command: $1"
    exit 127
  fi
}

free_port() {
  python3 - <<'PY'
import socket

with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
    sock.bind(("127.0.0.1", 0))
    print(sock.getsockname()[1])
PY
}

kubectl_cmd() {
  kubectl --context "$KIND_CONTEXT" "$@"
}

helm_cmd() {
  helm --kube-context "$KIND_CONTEXT" "$@"
}

ensure_namespace() {
  namespace=$1
  phase="$(kubectl_cmd get namespace "$namespace" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
  if [ "$phase" = "Terminating" ]; then
    log "namespace $namespace is terminating; wait for cleanup or recreate the kind cluster"
    exit 1
  fi

  kubectl_cmd create namespace "$namespace" --dry-run=client -o yaml |
    kubectl_cmd apply -f -
}

cleanup_pid() {
  pid="${1:-}"
  if [ -n "$pid" ]; then
    kill "$pid" >/dev/null 2>&1 || true
    wait "$pid" >/dev/null 2>&1 || true
  fi
}

dump_k8s_debug() {
  if ! kubectl_cmd get nodes >/dev/null 2>&1; then
    return
  fi

  log "cluster debug summary"
  kubectl_cmd get pods -A >&2 || true
  kubectl_cmd get pvc,pv,storageclass -A >&2 || true
  if [ -n "$tmp_dir" ] && [ -f "$tmp_dir/minio-port-forward.log" ]; then
    log "MinIO port-forward log"
    cat "$tmp_dir/minio-port-forward.log" >&2 || true
  fi
  if [ -n "$tmp_dir" ] && [ -f "$tmp_dir/package-r-port-forward.log" ]; then
    log "packageR port-forward log"
    cat "$tmp_dir/package-r-port-forward.log" >&2 || true
  fi
  kubectl_cmd -n "$K8S_NAMESPACE" describe pods >&2 || true
  kubectl_cmd -n "$K8S_NAMESPACE" logs deployment/minio --all-containers --tail=160 >&2 || true
  kubectl_cmd -n "$K8S_NAMESPACE" logs deployment/package-r --all-containers --tail=160 >&2 || true
  kubectl_cmd -n "$CSI_RCLONE_NAMESPACE" logs daemonset/csi-rclone-nodeplugin --all-containers --tail=160 >&2 || true
  kubectl_cmd -n "$CSI_RCLONE_NAMESPACE" logs statefulset/csi-rclone-controller --all-containers --tail=160 >&2 || true
}

print_manual_access() {
  log "manual inspection details"
  cat <<EOF
Kind cluster:        $KIND_CLUSTER_NAME
Kubernetes context:  $KIND_CONTEXT
packageR namespace: $K8S_NAMESPACE
csi-rclone namespace: $CSI_RCLONE_NAMESPACE

Current port-forwards while this script is running:
  packageR: $BASE_URL
  MinIO S3: $AWS_ENDPOINT_URL

Useful checks:
  kubectl --context "$KIND_CONTEXT" -n "$K8S_NAMESPACE" get pods,svc,pvc
  kubectl --context "$KIND_CONTEXT" -n "$CSI_RCLONE_NAMESPACE" get pods
  curl -H 'X-Username: admin' "$BASE_URL/api/login"
  curl "$BASE_URL/api/public/catalog/$PUBLIC_SHARE_HASH"
EOF

  if [ "$PACKAGE_R_K8S_KEEP_CLUSTER" = "true" ]; then
    cat <<EOF
Reconnect later after the script exits:
  kubectl --context "$KIND_CONTEXT" -n "$K8S_NAMESPACE" port-forward service/package-r ${PACKAGE_R_FORWARD_PORT}:8888
  kubectl --context "$KIND_CONTEXT" -n "$K8S_NAMESPACE" port-forward service/minio ${S3_FORWARD_PORT}:9000
EOF
  else
    cat <<EOF
This run will clean up the cluster or e2e resources on exit.
Use make test-e2e-s3-k8s-keep to keep the deployment for manual inspection.
EOF
  fi
}

cleanup() {
  status=$?
  trap - EXIT

  cleanup_pid "$package_r_port_forward_pid"
  cleanup_pid "$s3_port_forward_pid"

  if [ "$status" -ne 0 ]; then
    dump_k8s_debug
  fi

  if [ "$PACKAGE_R_K8S_KEEP_CLUSTER" = "true" ]; then
    log "keeping kind cluster and e2e resources: $KIND_CLUSTER_NAME"
  else
    if [ "$created_cluster" = "true" ]; then
      kind delete cluster --name "$KIND_CLUSTER_NAME" >/dev/null 2>&1 || true
    else
      kubectl_cmd -n "$K8S_NAMESPACE" delete deployment/package-r service/package-r \
        secret/package-r-data secret/package-r-aws pvc/package-r-data \
        --ignore-not-found=true --wait=false >/dev/null 2>&1 || true
      kubectl_cmd -n "$K8S_NAMESPACE" delete deployment/minio service/minio --ignore-not-found=true --wait=false >/dev/null 2>&1 || true
      kubectl_cmd delete storageclass/package-r-csi-rclone --ignore-not-found=true --wait=false >/dev/null 2>&1 || true
      helm_cmd uninstall "$CSI_RCLONE_RELEASE" -n "$CSI_RCLONE_NAMESPACE" >/dev/null 2>&1 || true
    fi
  fi

  if [ -n "$tmp_dir" ]; then
    rm -rf "$tmp_dir" >/dev/null 2>&1 || true
  fi

  exit "$status"
}
trap cleanup EXIT

wait_for_http_any() {
  url=$1
  label=$2
  pid="${3:-}"

  for _ in $(seq 1 120); do
    code="$(curl --max-time 2 -sS -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)"
    if [ "$code" != "000" ]; then
      return 0
    fi
    if [ -n "$pid" ] && ! kill -0 "$pid" >/dev/null 2>&1; then
      log "$label port-forward exited before readiness"
      return 1
    fi
    sleep 0.25
  done

  log "timed out waiting for $label at $url"
  return 1
}

wait_for_http_ok() {
  url=$1
  label=$2
  pid="${3:-}"

  for _ in $(seq 1 120); do
    if curl --max-time 2 -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    if [ -n "$pid" ] && ! kill -0 "$pid" >/dev/null 2>&1; then
      log "$label port-forward exited before readiness"
      return 1
    fi
    sleep 0.25
  done

  log "timed out waiting for $label at $url"
  return 1
}

json_field() {
  field=$1
  python3 -c 'import json, sys; print(json.load(sys.stdin)[sys.argv[1]])' "$field"
}

fetch_and_compare() {
  url=$1
  expected=$2
  output=$3

  curl -fsSL "$url" --output "$output"
  cmp "$expected" "$output"
}

rclone_version_line() {
  "$RCLONE_BIN" version 2>/dev/null | sed -n '1p'
}

rclone_version_supports_query_presign() {
  line=$1
  version="${line#rclone v}"
  version="${version%% *}"
  version="${version%%-*}"
  major="${version%%.*}"
  rest="${version#*.}"
  minor="${rest%%.*}"

  case "$major" in
    '' | *[!0-9]*) return 1 ;;
  esac
  case "$minor" in
    '' | *[!0-9]*) return 1 ;;
  esac

  [ "$major" -gt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -ge 74 ]; }
}

require_compatible_rclone() {
  require_command "$RCLONE_BIN"

  version_line="$(rclone_version_line || true)"
  if ! rclone_version_supports_query_presign "$version_line"; then
    log "local ${version_line:-rclone} is too old; install rclone >= 1.74 or set RCLONE_BIN"
    exit 1
  fi
  log "using local ${version_line:-rclone}"
}

create_kind_cluster() {
  if kind get clusters | grep -Fxq "$KIND_CLUSTER_NAME"; then
    log "using existing kind cluster: $KIND_CLUSTER_NAME"
    created_cluster=false
    return
  fi

  log "creating kind cluster: $KIND_CLUSTER_NAME"
  if [ -e /dev/fuse ]; then
    cat >"$tmp_dir/kind.yaml" <<'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraMounts:
      - hostPath: /dev/fuse
        containerPath: /dev/fuse
EOF
  else
    log "warning: /dev/fuse is not present on the host; csi-rclone may not be able to mount"
    cat >"$tmp_dir/kind.yaml" <<'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
EOF
  fi

  kind create cluster --name "$KIND_CLUSTER_NAME" --config "$tmp_dir/kind.yaml"
  created_cluster=true
}

build_and_load_package_r_image() {
  if [ "$PACKAGE_R_K8S_SKIP_IMAGE_BUILD" != "true" ]; then
    log "building production packageR binary and image: $PACKAGE_R_IMAGE"
    (cd "$ROOT_DIR" && make build-backend)
    docker build -t "$PACKAGE_R_IMAGE" "$ROOT_DIR"
  else
    log "skipping packageR image build: $PACKAGE_R_IMAGE"
  fi

  if [ "$PACKAGE_R_K8S_SKIP_IMAGE_LOAD" != "true" ]; then
    log "loading packageR image into kind: $PACKAGE_R_IMAGE"
    kind load docker-image --name "$KIND_CLUSTER_NAME" "$PACKAGE_R_IMAGE"
  fi
}

install_csi_rclone() {
  log "installing csi-rclone from EOEPCA chart $CSI_RCLONE_CHART:$CSI_RCLONE_CHART_VERSION"
  ensure_namespace "$CSI_RCLONE_NAMESPACE"
  helm_cmd upgrade --install "$CSI_RCLONE_RELEASE" "$CSI_RCLONE_CHART" \
    --version "$CSI_RCLONE_CHART_VERSION" \
    --namespace "$CSI_RCLONE_NAMESPACE"

  kubectl_cmd -n "$CSI_RCLONE_NAMESPACE" rollout status daemonset/csi-rclone-nodeplugin --timeout="$PACKAGE_R_K8S_ROLLOUT_TIMEOUT"
  kubectl_cmd -n "$CSI_RCLONE_NAMESPACE" rollout status statefulset/csi-rclone-controller --timeout="$PACKAGE_R_K8S_ROLLOUT_TIMEOUT"
}

deploy_minio_s3() {
  log "deploying in-cluster MinIO S3 server"
  ensure_namespace "$K8S_NAMESPACE"

  cat >"$tmp_dir/minio.yaml" <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: minio
  namespace: $K8S_NAMESPACE
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: minio
  template:
    metadata:
      labels:
        app.kubernetes.io/name: minio
    spec:
      containers:
        - name: minio
          image: $MINIO_IMAGE
          args:
            - server
            - /data
          env:
            - name: MINIO_ROOT_USER
              value: $AWS_ACCESS_KEY_ID
            - name: MINIO_ROOT_PASSWORD
              value: $AWS_SECRET_ACCESS_KEY
          ports:
            - name: s3
              containerPort: 9000
          readinessProbe:
            httpGet:
              path: /minio/health/ready
              port: s3
            periodSeconds: 5
          livenessProbe:
            httpGet:
              path: /minio/health/live
              port: s3
            periodSeconds: 10
          volumeMounts:
            - name: data
              mountPath: /data
      volumes:
        - name: data
          emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: minio
  namespace: $K8S_NAMESPACE
spec:
  selector:
    app.kubernetes.io/name: minio
  ports:
    - name: s3
      port: 9000
      targetPort: s3
EOF

  kubectl_cmd apply -f "$tmp_dir/minio.yaml"
  kubectl_cmd -n "$K8S_NAMESPACE" rollout status deployment/minio --timeout="$PACKAGE_R_K8S_ROLLOUT_TIMEOUT"
}

start_s3_port_forward_and_upload_data() {
  S3_FORWARD_PORT="${S3_FORWARD_PORT:-$(free_port)}"
  AWS_ENDPOINT_URL="http://127.0.0.1:${S3_FORWARD_PORT}"
  export AWS_ENDPOINT_URL

  log "port-forwarding MinIO S3 to $AWS_ENDPOINT_URL"
  kubectl_cmd -n "$K8S_NAMESPACE" port-forward --address 127.0.0.1 service/minio "${S3_FORWARD_PORT}:9000" >"$tmp_dir/minio-port-forward.log" 2>&1 &
  s3_port_forward_pid=$!
  wait_for_http_any "$AWS_ENDPOINT_URL/minio/health/ready" "MinIO S3" "$s3_port_forward_pid"

  log "creating s3://$BUCKET_NAME"
  "$RCLONE_BIN" mkdir ":s3:${BUCKET_NAME}" \
    --s3-provider Minio \
    --s3-endpoint "$AWS_ENDPOINT_URL" \
    --s3-access-key-id "$AWS_ACCESS_KEY_ID" \
    --s3-secret-access-key "$AWS_SECRET_ACCESS_KEY" \
    --s3-region "$AWS_REGION"

  log "uploading tests/data to s3://$BUCKET_NAME/public"
  "$RCLONE_BIN" copy "$ROOT_DIR/tests/data" ":s3:${BUCKET_NAME}/public" \
    --s3-provider Minio \
    --s3-endpoint "$AWS_ENDPOINT_URL" \
    --s3-access-key-id "$AWS_ACCESS_KEY_ID" \
    --s3-secret-access-key "$AWS_SECRET_ACCESS_KEY" \
    --s3-region "$AWS_REGION" \
    --s3-no-check-bucket
}

render_package_r_manifest() {
  package_r_manifest="$tmp_dir/package-r-k8s.yaml"

  python3 - "$PACKAGE_R_K8S_MANIFEST" "$package_r_manifest" <<'PY'
import os
import sys
from pathlib import Path

source, target = sys.argv[1:]
text = Path(source).read_text(encoding="utf-8")
explicit_test_env = '''            - name: FB_SERVER_PORT
              value: "8888"
            - name: FB_AUTH_METHOD
              value: proxy
            - name: FB_AUTH_HEADER
              value: X-Username
            - name: FB_PASSWORD
              value: package-r-e2e
            - name: FB_ALLOW_SHARING
              value: "true"
            - name: FB_ALLOW_CHANGING
              value: "true"'''
replacements = {
    "            - name: FB_SERVER_PORT\n              value: \"8888\"": explicit_test_env,
    "remotePath: \"/my-bucket\"": f"remotePath: \"/{os.environ['BUCKET_NAME']}\"",
    "provider = Other": "provider = Minio",
    "endpoint = https://s3.example.invalid": f"endpoint = {os.environ['CLUSTER_S3_ENDPOINT']}",
    "region = auto": f"region = {os.environ['AWS_REGION']}",
    "access_key_id = <access-key>": f"access_key_id = {os.environ['AWS_ACCESS_KEY_ID']}",
    "secret_access_key = <secret-key>": f"secret_access_key = {os.environ['AWS_SECRET_ACCESS_KEY']}",
    "access_key_id: <access-key>": f"access_key_id: {os.environ['AWS_ACCESS_KEY_ID']}",
    "secret_access_key: <secret-key>": f"secret_access_key: {os.environ['AWS_SECRET_ACCESS_KEY']}",
    "image: ghcr.io/versioneer-tech/package-r:latest": f"image: {os.environ['PACKAGE_R_IMAGE']}",
    "imagePullPolicy: IfNotPresent": "imagePullPolicy: Never",
    "value: public=/public": f"value: {os.environ['PUBLIC_SHARE_HASH']}=/public",
    "value: https://s3.example.invalid": f"value: {os.environ['AWS_ENDPOINT_URL']}",
    "value: auto": f"value: {os.environ['AWS_REGION']}",
    "value: my-bucket": f"value: {os.environ['BUCKET_NAME']}",
}
for old, new in replacements.items():
    if old not in text:
        raise SystemExit(f"expected manifest text not found: {old}")
    text = text.replace(old, new)

text += f"""
---
apiVersion: v1
kind: Service
metadata:
  name: package-r
  namespace: {os.environ['K8S_NAMESPACE']}
spec:
  selector:
    app.kubernetes.io/name: package-r
  ports:
    - name: http
      port: 8888
      targetPort: http
"""
Path(target).write_text(text, encoding="utf-8")
PY
}

deploy_package_r() {
  export BUCKET_NAME AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_REGION
  export AWS_ENDPOINT_URL PACKAGE_R_IMAGE PUBLIC_SHARE_HASH K8S_NAMESPACE
  export CLUSTER_S3_ENDPOINT="http://minio.${K8S_NAMESPACE}.svc.cluster.local:9000"

  render_package_r_manifest
  log "deploying packageR with patched $PACKAGE_R_K8S_MANIFEST"
  kubectl_cmd apply -f "$package_r_manifest"
  kubectl_cmd -n "$K8S_NAMESPACE" rollout status deployment/package-r --timeout="$PACKAGE_R_K8S_ROLLOUT_TIMEOUT"
}

start_package_r_port_forward() {
  PACKAGE_R_FORWARD_PORT="${PACKAGE_R_FORWARD_PORT:-$(free_port)}"
  BASE_URL="http://127.0.0.1:${PACKAGE_R_FORWARD_PORT}"
  export BASE_URL

  log "port-forwarding packageR to $BASE_URL"
  kubectl_cmd -n "$K8S_NAMESPACE" port-forward --address 127.0.0.1 service/package-r "${PACKAGE_R_FORWARD_PORT}:8888" >"$tmp_dir/package-r-port-forward.log" 2>&1 &
  package_r_port_forward_pid=$!
  wait_for_http_ok "$BASE_URL/health" "packageR" "$package_r_port_forward_pid"
}

run_presign_checks() {
  expected_thumbnail="$ROOT_DIR/tests/data/openaerialmap-assets/$ITEM_ID/thumbnail.png"
  auth_resource_path="/public/openaerialmap-assets/$ITEM_ID/thumbnail.png"
  public_share_path="openaerialmap-assets/$ITEM_ID/thumbnail.png"
  catalog_url="$BASE_URL/api/public/catalog/$PUBLIC_SHARE_HASH"

  log "checking public catalog endpoint"
  curl -fsS "$catalog_url" | python3 -c 'import json, sys; data = json.load(sys.stdin); assert data["type"] == "FeatureCollection"; assert len(data["features"]) > 0'

  log "checking authenticated resource presign"
  token="$(curl -fsS -H 'X-Username: admin' "$BASE_URL/api/login")"
  auth_presigned_url="$(
    curl -fsS -H "X-Auth: $token" "$BASE_URL/api/resources${auth_resource_path}?presign=true" |
      json_field presignedURL
  )"
  fetch_and_compare "$auth_presigned_url" "$expected_thumbnail" "$tmp_dir/auth-thumbnail.png"

  log "checking public share presign"
  public_presigned_url="$(
    curl -fsS "$BASE_URL/api/public/share/$PUBLIC_SHARE_HASH/$public_share_path?presign=true" |
      json_field presignedURL
  )"
  fetch_and_compare "$public_presigned_url" "$expected_thumbnail" "$tmp_dir/public-thumbnail.png"

  log "checking public share presign redirect"
  fetch_and_compare \
    "$BASE_URL/api/public/share/$PUBLIC_SHARE_HASH/$public_share_path?presign=true&followRedirect=true" \
    "$expected_thumbnail" \
    "$tmp_dir/public-thumbnail-redirect.png"
}

require_command kind
require_command kubectl
require_command helm
require_command docker
require_command curl
require_command python3
require_compatible_rclone

tmp_dir="$(mktemp -d)"
create_kind_cluster
build_and_load_package_r_image
install_csi_rclone
deploy_minio_s3
start_s3_port_forward_and_upload_data
deploy_package_r
start_package_r_port_forward
run_presign_checks
log "Kubernetes S3 presign e2e checks passed"
print_manual_access

if [ "$PACKAGE_R_K8S_KEEP_FORWARD" = "true" ]; then
  log "keeping port-forwards open. Press Ctrl-C to stop port-forwards."
  wait
fi
