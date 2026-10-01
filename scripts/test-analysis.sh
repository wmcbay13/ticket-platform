#!/usr/bin/env bash
source "$(dirname "$0")/common.sh"
mkdir -p .local
python3 - <<'PY'
from pathlib import Path
import yaml
template=yaml.safe_load(Path('deploy/app/analysis.yaml').read_text())
query=next(m for m in template['spec']['metrics'] if m['name']=='success-rate')['provider']['prometheus']['query'].replace('{{args.canary-hash}}','probe')
tests=[]
for name,status,values,expected in [
 ('healthy','200','0+30x8',1),
 ('all responses fail','503','0+30x8',0),
 ('insufficient traffic','200','0+1x8',None),
 ('zero traffic','200','0+0x8',None),
]:
 tests.append({'name':name,'interval':'15s',
  'input_series':[{'series':f'ticket_http_requests_total{{service="booking",rollout_hash="probe",status="{status}"}}','values':values}],
  'promql_expr_test':[{'expr':query,'eval_time':'2m','exp_samples':[] if expected is None else [{'labels':'{}','value':expected}]}]})
tests.append({'name':'no samples','interval':'15s','input_series':[],
 'promql_expr_test':[{'expr':query,'eval_time':'2m','exp_samples':[]}]})
Path('.local/analysis-tests.yaml').write_text(yaml.safe_dump({'tests':tests},sort_keys=False))
PY
docker run --rm --entrypoint /bin/promtool -v "$PROJECT_ROOT/.local/analysis-tests.yaml:/tests.yaml:ro" \
  "$PROMETHEUS_TEST_IMAGE" test rules /tests.yaml
