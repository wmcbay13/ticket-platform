#!/usr/bin/env bash
source "$(dirname "$0")/common.sh"
mkdir -p .local
kubectl kustomize deploy/app > .local/rendered-app.yaml
python3 - <<'PY'
from pathlib import Path
import yaml
for path in [Path('.local/rendered-app.yaml'),*Path('deploy/platform').rglob('*.yaml'),Path('deploy/root.yaml')]:
    documents=list(yaml.safe_load_all(path.read_text()))
    for d in documents:
        assert isinstance(d,dict),f'Empty or invalid YAML: {path}'
    if path.name=='rendered-app.yaml':
        identities=set()
        for d in documents:
            identity=(d['kind'],d['metadata'].get('namespace'),d['metadata']['name'])
            assert identity not in identities,f'Duplicate {identity}'
            identities.add(identity)
        assert any(d['kind']=='Rollout' for d in documents)
print('Kustomize rendering and YAML checks passed.')
PY
bash -n scripts/*.sh
