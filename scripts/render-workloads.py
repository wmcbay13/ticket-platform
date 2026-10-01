#!/usr/bin/env python3
"""Generate the repetitive service resources; output is reviewed and committed."""
from pathlib import Path
import yaml

documents = []
for role in ['catalog', 'booking', 'worker', 'web']:
    labels = {'app': role, 'project': 'ticket-platform'}
    pod_labels = {**labels}
    if role != 'web':
        pod_labels['metrics'] = 'ticket'
    container = {
        'name': role, 'image': 'ticket-web' if role == 'web' else 'ticket-backend',
        'ports': [{'name': 'http', 'containerPort': 8080, 'protocol': 'TCP'}],
        'resources': {'requests': {'cpu': '50m', 'memory': '64Mi'}, 'limits': {'cpu': '500m', 'memory': '128Mi'}},
        'securityContext': {'runAsNonRoot': True, 'runAsUser': 101 if role == 'web' else 65532,
                            'allowPrivilegeEscalation': False, 'readOnlyRootFilesystem': True,
                            'capabilities': {'drop': ['ALL']}},
        'startupProbe': {'httpGet': {'path': '/healthz', 'port': 'http'}, 'periodSeconds': 2, 'failureThreshold': 30},
        'readinessProbe': {'httpGet': {'path': '/healthz' if role == 'web' else '/readyz', 'port': 'http'}, 'periodSeconds': 3},
        'livenessProbe': {'httpGet': {'path': '/healthz', 'port': 'http'}, 'periodSeconds': 10},
    }
    pod_spec = {
        'automountServiceAccountToken': False, 'terminationGracePeriodSeconds': 15,
        'securityContext': {'runAsNonRoot': True, 'seccompProfile': {'type': 'RuntimeDefault'}},
        'tolerations': [{'key': 'node.kubernetes.io/'+condition, 'operator': 'Exists', 'effect': 'NoExecute', 'tolerationSeconds': 30} for condition in ['not-ready','unreachable']],
        'topologySpreadConstraints': [{'maxSkew': 1, 'topologyKey': 'kubernetes.io/hostname',
                                      'whenUnsatisfiable': 'ScheduleAnyway', 'labelSelector': {'matchLabels': {'app': role}}}],
        'containers': [container],
    }
    if role == 'web':
        container['volumeMounts'] = [{'name': 'tmp', 'mountPath': '/tmp'}]
        pod_spec['volumes'] = [{'name': 'tmp', 'emptyDir': {'sizeLimit': '32Mi'}}]
    else:
        container['args'] = ['-mode', role]
        container['env'] = [{'name': 'DATABASE_URL', 'valueFrom': {'secretKeyRef': {'name': 'app-auth', 'key': role+'_url'}}},
                            {'name': 'APP_VERSION', 'value': 'bootstrap'}, {'name': 'DB_MAX_CONNS', 'value': '5'}]
    deployment = {'apiVersion': 'apps/v1', 'kind': 'Deployment', 'metadata': {'name': role, 'namespace': 'ticket', 'labels': labels},
                  'spec': {'replicas': 2, 'revisionHistoryLimit': 3, 'selector': {'matchLabels': {'app': role}},
                           'strategy': {'type': 'RollingUpdate', 'rollingUpdate': {'maxUnavailable': 0, 'maxSurge': 1}},
                           'template': {'metadata': {'labels': pod_labels}, 'spec': pod_spec}}}
    if role == 'booking':
        deployment['apiVersion'] = 'argoproj.io/v1alpha1'
        deployment['kind'] = 'Rollout'
        deployment['spec']['strategy'] = {'canary': {
            'stableService': 'booking-stable', 'canaryService': 'booking-canary',
            'maxSurge': 1, 'maxUnavailable': 0,
            'trafficRouting': {'traefik': {'weightedTraefikServiceName': 'booking-traffic'}},
            'steps': [{'setWeight': 10}, {'pause': {'duration': '2m'}},
                      {'analysis': {'templates': [{'templateName': 'booking-quality'}],
                                    'args': [{'name': 'canary-hash', 'valueFrom': {'podTemplateHashValue': 'Latest'}}]}},
                      {'setWeight': 50}, {'pause': {'duration': '2m'}},
                      {'analysis': {'templates': [{'templateName': 'booking-quality'}],
                                    'args': [{'name': 'canary-hash', 'valueFrom': {'podTemplateHashValue': 'Latest'}}]}},
                      {'setWeight': 100}],
        }}
    documents.append(deployment)
    for name in (['booking-stable', 'booking-canary'] if role == 'booking' else [role]):
        documents.append({'apiVersion': 'v1', 'kind': 'Service', 'metadata': {'name': name, 'namespace': 'ticket', 'labels': labels},
                          'spec': {'selector': {'app': role}, 'ports': [{'name': 'http', 'port': 8080, 'targetPort': 'http'}]}})
    documents.append({'apiVersion': 'policy/v1', 'kind': 'PodDisruptionBudget', 'metadata': {'name': role, 'namespace': 'ticket'},
                      'spec': {'minAvailable': 1, 'selector': {'matchLabels': {'app': role}}}})
    if role in ['catalog', 'booking']:
        documents.append({'apiVersion': 'autoscaling/v2', 'kind': 'HorizontalPodAutoscaler', 'metadata': {'name': role, 'namespace': 'ticket'},
                          'spec': {'scaleTargetRef': {'apiVersion': deployment['apiVersion'], 'kind': deployment['kind'], 'name': role},
                                   'minReplicas': 2, 'maxReplicas': 4,
                                   'metrics': [{'type': 'Resource', 'resource': {'name': 'cpu', 'target': {'type': 'Utilization', 'averageUtilization': 60}}}],
                                   'behavior': {'scaleDown': {'stabilizationWindowSeconds': 300}}}})
documents.append({
    'apiVersion': 'batch/v1', 'kind': 'Job',
    'metadata': {'name': 'ticket-migrate', 'namespace': 'ticket', 'annotations': {
        'argocd.argoproj.io/hook': 'Sync', 'argocd.argoproj.io/sync-wave': '-1',
        'argocd.argoproj.io/hook-delete-policy': 'BeforeHookCreation,HookSucceeded'}},
    'spec': {'backoffLimit': 3, 'activeDeadlineSeconds': 180, 'template': {
        'metadata': {'labels': {'app': 'migrate', 'project': 'ticket-platform'}},
        'spec': {'restartPolicy': 'Never', 'automountServiceAccountToken': False,
                 'securityContext': {'runAsNonRoot': True, 'runAsUser': 65532, 'seccompProfile': {'type': 'RuntimeDefault'}},
                 'containers': [{'name': 'migrate', 'image': 'ticket-backend', 'args': ['-mode', 'migrate'],
                                 'env': [{'name': 'DATABASE_URL', 'valueFrom': {'secretKeyRef': {'name': 'app-auth', 'key': 'admin_url'}}}] +
                                        [{'name': role.upper()+'_PASSWORD', 'valueFrom': {'secretKeyRef': {'name': 'app-auth', 'key': role+'_password'}}} for role in ['catalog','booking','worker']],
                                 'resources': {'requests': {'cpu': '50m', 'memory': '32Mi'}, 'limits': {'cpu': '300m', 'memory': '128Mi'}},
                                 'securityContext': {'allowPrivilegeEscalation': False, 'readOnlyRootFilesystem': True, 'capabilities': {'drop': ['ALL']}}}]}}}})
Path('deploy/app/workloads.yaml').write_text(yaml.safe_dump_all(documents, sort_keys=False))
