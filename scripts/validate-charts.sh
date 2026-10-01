#!/usr/bin/env bash
source "$(dirname "$0")/common.sh"
mkdir -p .local/charts
python3 - <<'PY' > .local/chart-list.tsv
import yaml
for app in yaml.safe_load_all(open('deploy/platform/apps/applications.yaml')):
 source=app['spec'].get('sources',[{}])[0]
 if 'chart' in source:
  print('\t'.join([app['metadata']['name'],app['spec']['destination']['namespace'],source['chart'],source['repoURL'],source['targetRevision']]))
PY
while IFS=$'\t' read -r name namespace chart repo version; do
  helm template "$name" "$chart" --repo "$repo" --version "$version" --namespace "$namespace" \
    --kube-version "$KUBERNETES_VERSION" --api-versions monitoring.coreos.com/v1 --api-versions monitoring.coreos.com/v1/ServiceMonitor \
    --include-crds -f "deploy/platform/values/$name.yaml" > ".local/charts/$name.yaml"
  echo "Chart values validated: $name $version"
done < .local/chart-list.tsv
