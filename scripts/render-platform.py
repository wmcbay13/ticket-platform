#!/usr/bin/env python3
"""Keep Argo applications and the bootstrap lockfile in agreement."""
from pathlib import Path
import yaml
versions = dict(line.split('=',1) for line in Path('versions.env').read_text().splitlines() if line and not line.startswith('#'))
repo = 'https://github.com/wmcbay13/ticket-platform.git'
charts = [
    ('cilium', 'kube-system', 'https://helm.cilium.io', 'cilium', 'CILIUM_CHART', '-3'),
    ('argocd', 'argocd', 'https://argoproj.github.io/argo-helm', 'argo-cd', 'ARGOCD_CHART', '-3'),
    ('monitoring', 'monitoring', 'https://prometheus-community.github.io/helm-charts', 'kube-prometheus-stack', 'PROMETHEUS_CHART', '-2'),
    ('metallb', 'metallb-system', 'https://metallb.github.io/metallb', 'metallb', 'METALLB_CHART', '-2'),
    ('rollouts', 'argo-rollouts', 'https://argoproj.github.io/argo-helm', 'argo-rollouts', 'ROLLOUTS_CHART', '-2'),
    ('metrics-server', 'kube-system', 'https://kubernetes-sigs.github.io/metrics-server', 'metrics-server', 'METRICS_SERVER_CHART', '-2'),
    ('traefik', 'ingress', 'https://traefik.github.io/charts', 'traefik', 'TRAEFIK_CHART', '-1'),
]
docs=[]
for name,namespace,url,chart,key,wave in charts:
    docs.append({'apiVersion':'argoproj.io/v1alpha1','kind':'Application',
        'metadata':{'name':name,'namespace':'argocd','annotations':{'argocd.argoproj.io/sync-wave':wave}},
        'spec':{'project':'default','destination':{'server':'https://kubernetes.default.svc','namespace':namespace},
            'sources':[{'repoURL':url,'chart':chart,'targetRevision':versions[key],
                        'helm':{'releaseName':name,'valueFiles':['$values/deploy/platform/values/'+name+'.yaml']}},
                       {'repoURL':repo,'targetRevision':'gitops-demo','ref':'values'}],
            'syncPolicy':{'automated':{'prune':True,'selfHeal':True},'syncOptions':['CreateNamespace=true','ServerSideApply=true'],
                          'retry':{'limit':5,'backoff':{'duration':'10s','factor':2,'maxDuration':'3m'}}}}})
app={'apiVersion':'argoproj.io/v1alpha1','kind':'Application','metadata':{'name':'ticket-platform','namespace':'argocd','annotations':{'argocd.argoproj.io/sync-wave':'0'}},
     'spec':{'project':'default','destination':{'server':'https://kubernetes.default.svc','namespace':'ticket'},
             'source':{'repoURL':repo,'targetRevision':'gitops-demo','path':'deploy/app'},
             'syncPolicy':{'automated':{'prune':True,'selfHeal':True},'syncOptions':['CreateNamespace=true','ServerSideApply=true','RespectIgnoreDifferences=true'],
                           'retry':{'limit':5,'backoff':{'duration':'10s','factor':2,'maxDuration':'3m'}}},
             'ignoreDifferences':[
                 {'group':'argoproj.io','kind':'Rollout','name':'booking','jsonPointers':['/spec/replicas']},
                 {'group':'apps','kind':'Deployment','name':'catalog','jsonPointers':['/spec/replicas']},
                 {'group':'','kind':'Service','jqPathExpressions':['.spec.selector."rollouts-pod-template-hash"']},
                 {'group':'traefik.io','kind':'TraefikService','name':'booking-traffic','jqPathExpressions':['.spec.weighted.services[].weight']},
             ]}}
docs.append(app)
Path('deploy/platform/apps').mkdir(parents=True,exist_ok=True)
Path('deploy/platform/apps/applications.yaml').write_text(yaml.safe_dump_all(docs,sort_keys=False))
