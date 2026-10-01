#!/usr/bin/env bash
source "$(dirname "$0")/common.sh"
require_cluster
k get nodes -o wide
k get applications -n argocd
k get pods -n ticket -o wide
k get hpa -n ticket
ip="$(k get svc traefik -n ingress -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || true)"
if [[ -n "$ip" ]]; then echo "Application: http://$ip"; fi
echo 'Argo CD: make argocd-ui (http://localhost:8085)'
echo 'Grafana: make grafana-ui (http://localhost:3000)'
echo 'Credentials: .local/credentials.json; Argo CD password: make argocd-password'
