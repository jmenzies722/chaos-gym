#!/bin/bash
# Deploy everything to the k3s cluster. Copied to /opt/chaos-gym/ on the
# instance and run there over SSM — the cluster has no inbound port open, so
# there is no kubectl from the laptop.
#
# Re-run at the start of every session: the ECR auth token below is valid for
# 12 hours, so the pull secret is not a set-it-once thing.
set -euo pipefail

REGION=us-east-1
REGISTRY=755484097925.dkr.ecr.us-east-1.amazonaws.com
MANIFEST_DIR=/opt/chaos-gym

TOKEN=$(aws ecr get-login-password --region "$REGION")

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

k3s kubectl apply -f "$MANIFEST_DIR/app.yaml"
k3s kubectl apply -f "$MANIFEST_DIR/loadgen.yaml"
k3s kubectl apply -f "$MANIFEST_DIR/otel-agent.yaml"
k3s kubectl apply -f "$MANIFEST_DIR/otel-gateway.yaml"
k3s kubectl apply -f "$MANIFEST_DIR/monitoring.yaml"
k3s kubectl apply -f "$MANIFEST_DIR/dashboard.yaml"
k3s kubectl apply -f "$MANIFEST_DIR/chaos.yaml"

echo "=== rollout ==="
k3s kubectl rollout status deployment/chaos-app --timeout=90s
