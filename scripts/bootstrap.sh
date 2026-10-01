#!/usr/bin/env bash
source "$(dirname "$0")/common.sh"
./scripts/tools.sh
docker info >/dev/null
available_mb="$(awk '/MemAvailable/ {print int($2/1024)}' /proc/meminfo)"
if (( available_mb < 8192 )); then
  echo "Available memory: ${available_mb} MiB; recommended: 8192 MiB. Close other applications before load testing."
fi
mkdir -p .local/data .local/downloads
if ! kind get clusters 2>/dev/null | rg -qx ticket-platform; then
  python3 - <<'PY'
from pathlib import Path
import yaml
config=yaml.safe_load(Path('deploy/kind.yaml').read_text())
config['nodes'][1]['extraMounts'][0]['hostPath']=str(Path('.local/data').resolve())
Path('.local/kind.yaml').write_text(yaml.safe_dump(config))
PY
  kind create cluster --name ticket-platform --config .local/kind.yaml --image "kindest/node:v${KUBERNETES_VERSION}" --kubeconfig "$KUBECONFIG" --wait 0s
fi
kubectl config use-context kind-ticket-platform >/dev/null
# Bootstrap controllers are rendered from the same pinned values Argo later owns.
helm template cilium cilium --repo https://helm.cilium.io --version "$CILIUM_CHART" --namespace kube-system -f deploy/platform/values/cilium.yaml --include-crds > .local/cilium.yaml
k apply --server-side --force-conflicts -f .local/cilium.yaml
k rollout status daemonset/cilium -n kube-system --timeout=300s
k wait --for=condition=Ready nodes --all --timeout=300s
k create namespace argocd --dry-run=client -o yaml | k apply -f -
helm template argocd argo-cd --repo https://argoproj.github.io/argo-helm --version "$ARGOCD_CHART" --namespace argocd -f deploy/platform/values/argocd.yaml --include-crds > .local/argocd.yaml
k apply --server-side --force-conflicts -f .local/argocd.yaml
k rollout status deployment/argocd-server -n argocd --timeout=300s
k apply -f deploy/app/namespaces.yaml
k create namespace monitoring --dry-run=client -o yaml | k apply -f -
python3 scripts/secrets.py
k apply -f .local/secrets.yaml
k apply -f deploy/root.yaml
echo 'Waiting for Argo CD to install MetalLB (gitops-demo must have a published deployment snapshot).'
for attempt in $(seq 1 120); do
  if k get crd ipaddresspools.metallb.io >/dev/null 2>&1 && k get deployment/metallb-controller -n metallb-system >/dev/null 2>&1; then break; fi
  sleep 5
done
k rollout status deployment/metallb-controller -n metallb-system --timeout=300s
python3 scripts/metallb-pool.py
k apply -f .local/metallb-pool.yaml
./scripts/status.sh
