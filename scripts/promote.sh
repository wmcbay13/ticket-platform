#!/usr/bin/env bash
# CI-only: publish an immutable deployment snapshot using checkout's Git credentials.
set -euo pipefail
cd "$(dirname "$0")/.."
: "${BACKEND_DIGEST:?}" "${WEB_DIGEST:?}" "${GITHUB_SHA:?}" "${GITHUB_REF_NAME:?}"
[[ "$BACKEND_DIGEST" =~ ^sha256:[a-f0-9]{64}$ && "$WEB_DIGEST" =~ ^sha256:[a-f0-9]{64}$ ]] || exit 1
git fetch origin "$GITHUB_REF_NAME"
if [[ "$(git rev-parse FETCH_HEAD)" != "$GITHUB_SHA" ]]; then echo 'Superseded build; leaving deployment unchanged.'; exit 0; fi
snapshot="$(mktemp -d)"
cleanup(){ git worktree remove --force "$snapshot" >/dev/null 2>&1 || true; }
trap cleanup EXIT
if git fetch origin gitops-demo; then git worktree add --detach "$snapshot" FETCH_HEAD; else git worktree add --detach "$snapshot" "$GITHUB_SHA"; fi
# Replace only this disposable worktree's tree with the deployment snapshot.
git -C "$snapshot" rm -r --ignore-unmatch . >/dev/null
git archive "$GITHUB_SHA" deploy versions.env | tar -x -C "$snapshot"
export SNAPSHOT_DIR="$snapshot"
python3 - <<'PY'
import json,os
from pathlib import Path
import yaml
root=Path(os.environ['SNAPSHOT_DIR']);path=root/'deploy/app/kustomization.yaml'
config=yaml.safe_load(path.read_text())
for image,env in zip(config['images'],['BACKEND_DIGEST','WEB_DIGEST']):
 image.pop('newTag',None);image['digest']=os.environ[env]
path.write_text(yaml.safe_dump(config,sort_keys=False))
path=root/'deploy/app/workloads.yaml';docs=list(yaml.safe_load_all(path.read_text()))
for d in docs:
 if d['kind'] in ['Deployment','Rollout']:
  for c in d['spec']['template']['spec']['containers']:
   for e in c.get('env',[]):
    if e['name']=='APP_VERSION':e['value']=os.environ['GITHUB_SHA']
path.write_text(yaml.safe_dump_all(docs,sort_keys=False))
(root/'release.json').write_text(json.dumps({'source_commit':os.environ['GITHUB_SHA'],'source_branch':os.environ['GITHUB_REF_NAME'],'backend':os.environ['BACKEND_DIGEST'],'web':os.environ['WEB_DIGEST']},indent=2)+'\n')
(root/'README.md').write_text('# Deployment snapshot\n\nManaged by GitHub Actions. Argo CD tracks this branch.\nSee release.json for immutable image and source provenance.\n')
PY
git -C "$snapshot" add -A
git -C "$snapshot" -c user.name='github-actions[bot]' -c user.email='41898282+github-actions[bot]@users.noreply.github.com' commit -m "deploy: ${GITHUB_SHA:0:12}"
git -C "$snapshot" push origin HEAD:refs/heads/gitops-demo
