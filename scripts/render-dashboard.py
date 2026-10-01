#!/usr/bin/env python3
from pathlib import Path
import json,yaml
panels=[
 ('HTTP requests / second','sum by(service) (rate(ticket_http_requests_total[1m]))','reqps'),
 ('HTTP p95 latency','histogram_quantile(0.95,sum by(service,le)(rate(ticket_http_duration_seconds_bucket[2m])))','s'),
 ('HTTP 5xx fraction','sum by(service)(rate(ticket_http_requests_total{status=~"5.."}[2m])) / sum by(service)(rate(ticket_http_requests_total[2m]))','percentunit'),
 ('Container CPU cores','sum by(pod)(rate(container_cpu_usage_seconds_total{namespace="ticket",container!="",container!="POD"}[1m]))','cores'),
 ('Container working set','sum by(pod)(container_memory_working_set_bytes{namespace="ticket",container!="",container!="POD"})','bytes'),
 ('CPU throttling fraction','sum by(pod)(rate(container_cpu_cfs_throttled_periods_total{namespace="ticket"}[1m])) / sum by(pod)(rate(container_cpu_cfs_periods_total{namespace="ticket"}[1m]))','percentunit'),
 ('API desired replicas','kube_horizontalpodautoscaler_status_desired_replicas{namespace="ticket"}','short'),
 ('Payment backlog','max(ticket_payment_backlog)','short'),
 ('Oldest pending payment','max(ticket_payment_oldest_seconds)','s'),
 ('Simulated payment outcomes','sum by(outcome)(rate(ticket_payments_total[2m]))','ops'),
]
dashboard={'title':'Ticket Platform — Operations','uid':'ticket-platform','schemaVersion':39,'version':1,'tags':['ticket','devops'],
 'timezone':'browser','refresh':'10s','time':{'from':'now-15m','to':'now'},'panels':[]}
for i,(title,query,unit) in enumerate(panels):
 dashboard['panels'].append({'id':i+1,'title':title,'type':'timeseries','gridPos':{'x':(i%2)*12,'y':(i//2)*8,'w':12,'h':8},
   'datasource':{'type':'prometheus','uid':'prometheus'},'targets':[{'refId':'A','expr':query,'legendFormat':'{{service}}{{pod}}{{outcome}}{{horizontalpodautoscaler}}'}],
   'fieldConfig':{'defaults':{'unit':unit},'overrides':[]}})
doc={'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':'ticket-dashboard','namespace':'monitoring','labels':{'grafana_dashboard':'1'}},
     'data':{'ticket-platform.json':json.dumps(dashboard,indent=2)}}
Path('deploy/app/dashboard.yaml').write_text(yaml.safe_dump(doc,sort_keys=False))
