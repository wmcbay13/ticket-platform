#!/usr/bin/env bash
source "$(dirname "$0")/common.sh"
mode="${1:?good, fail, or restore}"
[[ "$mode" == good || "$mode" == fail || "$mode" == restore ]] || exit 1
snapshot="$(mktemp -d)"
trap 'git worktree remove --force "$snapshot" >/dev/null 2>&1 || true' EXIT
git fetch origin gitops-demo
git worktree add --detach "$snapshot" FETCH_HEAD
export SNAPSHOT_DIR="$snapshot" DEMO_MODE="$mode"
python3 - <<'PY'
import json,os,time
from pathlib import Path
import yaml
root=Path(os.environ['SNAPSHOT_DIR']);path=root/'deploy/app/workloads.yaml'
docs=list(yaml.safe_load_all(path.read_text()));release=json.loads((root/'release.json').read_text())
mode=os.environ['DEMO_MODE']
for d in docs:
 if d['kind']=='Rollout' and d['metadata']['name']=='booking':
  env=d['spec']['template']['spec']['containers'][0]['env'];env[:]=[e for e in env if e['name']!='DEMO_FAIL']
  for e in env:
   if e['name']=='APP_VERSION':e['value']=release['source_commit'] if mode=='restore' else release['source_commit']+'-demo-'+mode+'-'+str(int(time.time()))
  if mode=='fail':env.append({'name':'DEMO_FAIL','value':'true'})
path.write_text(yaml.safe_dump_all(docs,sort_keys=False))
PY
git -C "$snapshot" add deploy/app/workloads.yaml
if git -C "$snapshot" diff --cached --quiet; then
  echo 'The requested booking template is already the desired state.'
  exit 0
fi
git -C "$snapshot" commit -m "demo: $mode booking canary"
git -C "$snapshot" push origin HEAD:refs/heads/gitops-demo
echo 'Desired state published. Keep make canary-traffic running during promotion/abort.'
