#!/usr/bin/env bash
set -euo pipefail

namespace="renovate-scheduler"
release="renovate-scheduler"

kubectl create namespace "$namespace"
kubectl -n "$namespace" create secret generic renovate-scheduler-secret \
  --from-literal=RENOVATE_TOKEN=test-token
kubectl -n "$namespace" create configmap renovate-config \
  --from-literal=config.json='{}'

helm upgrade --install "$release" helm/renovate-scheduler \
  --namespace "$namespace" \
  --set image.repository=renovate-scheduler \
  --set image.tag=e2e \
  --set rbac.create=true \
  --set serviceAccount.name=renovate-scheduler \
  --set scheduler.config.kubernetes.serviceAccountName=renovate-scheduler \
  --wait \
  --timeout 2m

kubectl -n "$namespace" rollout status deployment/renovate-scheduler-renovate-scheduler --timeout=2m
kubectl -n "$namespace" logs deployment/renovate-scheduler-renovate-scheduler | grep -F 'starting scheduler'
