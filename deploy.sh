#!/bin/bash
# Deploy everything to the k3s cluster. Copied to /opt/chaos-gym/ on the
# instance and run there over SSM — the cluster has no inbound port open, so
# there is no kubectl from a laptop.
#
# Re-run at the start of every session: the ECR auth token below is valid for
# 12 hours, so the pull secret is not a set-it-once thing.
set -euo pipefail

REGION="${AWS_REGION:-us-east-1}"
MANIFEST_DIR="${MANIFEST_DIR:-/opt/chaos-gym}"

# The account id is derived, never committed. Manifests carry the placeholder
# __ECR_REGISTRY__ and are rendered below, which keeps this repository free of
# account-specific identifiers and lets anyone run it against their own account
# without editing a single YAML file.
ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
REGISTRY="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"

TOKEN="$(aws ecr get-login-password --region "$REGION")"

# Secrets are namespaced. A pull secret in `default` does not exist in `chaos`,
# and the symptom of forgetting is ImagePullBackOff with a 401 — which reads
# like a credentials problem rather than a missing-object problem.
for ns in default chaos; do
  k3s kubectl create namespace "$ns" --dry-run=client -o yaml | k3s kubectl apply -f -
  k3s kubectl create secret docker-registry ecr-creds \
    --namespace "$ns" \
    --docker-server="$REGISTRY" \
    --docker-username=AWS \
    --docker-password="$TOKEN" \
    --dry-run=client -o yaml | k3s kubectl apply -f -
done

# Render into a temporary directory rather than editing the manifests in place,
# so a failed run cannot leave account-specific values written back into files
# that are about to be committed.
RENDERED="$(mktemp -d)"
trap 'rm -rf "$RENDERED"' EXIT

for f in "$MANIFEST_DIR"/*.yaml; do
  sed "s|__ECR_REGISTRY__|${REGISTRY}|g" "$f" > "$RENDERED/$(basename "$f")"
done

# Order matters on a first run: the namespace and the CRDs the monitoring chart
# installs have to exist before anything that references them.
for f in app.yaml loadgen.yaml otel-agent.yaml otel-gateway.yaml \
         monitoring.yaml dashboard.yaml podmonitors.yaml chaos.yaml; do
  [ -f "$RENDERED/$f" ] && k3s kubectl apply -f "$RENDERED/$f"
done

echo "=== rollout ==="
k3s kubectl rollout status deployment/chaos-app --timeout=90s
