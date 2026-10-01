#!/usr/bin/env python3
"""Generate credentials once, never print them, and preserve them on reruns."""
import base64,json,os,secrets
from pathlib import Path
import yaml
directory=Path('.local');directory.mkdir(exist_ok=True)
path=directory/'credentials.json'
if path.exists():
    credentials=json.loads(path.read_text())
else:
    credentials={role:secrets.token_hex(24) for role in ['postgres','catalog','booking','worker','grafana']}
    path.touch(mode=0o600,exist_ok=False)
    path.write_text(json.dumps(credentials,indent=2)+'\n')
os.chmod(path,0o600)
app={role+'_url':f'postgres://{role}:{credentials[role]}@postgres.ticket-data.svc:5432/tickets?sslmode=disable' for role in ['catalog','booking','worker']}
app['admin_url']=f'postgres://postgres:{credentials["postgres"]}@postgres.ticket-data.svc:5432/tickets?sslmode=disable'
app.update({role+'_password':credentials[role] for role in ['catalog','booking','worker']})
def secret(name,namespace,data):
    return {'apiVersion':'v1','kind':'Secret','metadata':{'name':name,'namespace':namespace},'type':'Opaque',
            'data':{k:base64.b64encode(v.encode()).decode() for k,v in data.items()}}
docs=[secret('postgres-auth','ticket-data',{'password':credentials['postgres']}),secret('app-auth','ticket',app),secret('grafana-admin','monitoring',{'username':'admin','password':credentials['grafana']})]
target=directory/'secrets.yaml';target.touch(mode=0o600,exist_ok=True);os.chmod(target,0o600)
target.write_text(yaml.safe_dump_all(docs,sort_keys=False))
print('Generated/preserved credentials in .local/credentials.json (mode 0600).')
