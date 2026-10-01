#!/usr/bin/env bash
source "$(dirname "$0")/common.sh"
require_cluster
command="${1:?demo command}"
mkdir -p .local/results
base="http://$(k get svc traefik -n ingress -o jsonpath='{.status.loadBalancer.ingress[0].ip}')"
case "$command" in
  smoke) python3 scripts/smoke.py "$base";;
  load|canary-traffic)
    script=spike; [[ "$command" == canary-traffic ]] && script=canary
    result="${script}-$(date -u +%Y%m%dT%H%M%SZ).json"
    docker run --rm --network host --user "$(id -u):$(id -g)" -e BASE_URL="$base" -e DURATION="${DURATION:-10m}" \
      -v "$PROJECT_ROOT/tests/load:/scripts:ro" -v "$PROJECT_ROOT/.local/results:/results" \
      "$K6_IMAGE" run --summary-export "/results/$result" "/scripts/$script.js"
    echo "Results: .local/results/$result";;
  failure-pod)
    before="$(k get pods -n ticket -l app=catalog -o jsonpath='{.items[0].metadata.uid}')"
    name="$(k get pods -n ticket -l app=catalog -o jsonpath='{.items[0].metadata.name}')"
    k delete pod "$name" -n ticket
    k rollout status deployment/catalog -n ticket --timeout=180s
    k get pods -n ticket -l app=catalog -o wide
    if k get pods -n ticket -l app=catalog -o jsonpath='{.items[*].metadata.uid}' | rg -q "$before"; then echo 'Pod was not replaced'; exit 1; fi
    python3 scripts/smoke.py "$base";;
  failure-worker-node)
    node=ticket-platform-worker2
    restore(){ docker start "$node" >/dev/null; }
    trap restore EXIT
    docker stop "$node"
    echo 'Waiting for node detection and the application’s 30-second eviction toleration.'
    sleep 90
    k rollout status deployment/catalog -n ticket --timeout=180s
    k rollout status deployment/worker -n ticket --timeout=180s
    k get pods -n ticket -o wide
    python3 scripts/smoke.py "$base"
    restore;trap - EXIT
    k wait --for=condition=Ready node/"$node" --timeout=180s;;
  policy-test)
    cleanup(){ k delete pod ticket-denied ticket-allowed -n ticket --ignore-not-found >/dev/null; }
    trap cleanup EXIT
    for role in denied allowed; do
      label=policy-probe; [[ "$role" == allowed ]] && label=migrate
      k run "ticket-$role" -n ticket --image=busybox:1.37.0 --labels="app=$label" \
        --overrides='{"spec":{"automountServiceAccountToken":false,"securityContext":{"runAsNonRoot":true,"runAsUser":65532,"seccompProfile":{"type":"RuntimeDefault"}},"containers":[{"name":"probe","image":"busybox:1.37.0","command":["sleep","120"],"securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}}]}}'
      k wait --for=condition=Ready "pod/ticket-$role" -n ticket --timeout=90s
    done
    if k exec -n ticket ticket-denied -- sh -c 'nc -w 3 postgres.ticket-data.svc 5432 </dev/null'; then echo 'Unexpected unauthorized database access'; exit 1; fi
    k exec -n ticket ticket-allowed -- sh -c 'nc -w 3 postgres.ticket-data.svc 5432 </dev/null'
    if k exec -n ticket ticket-denied -- wget -T 3 -qO- http://catalog:8080/healthz; then echo 'Unexpected unauthorized API access'; exit 1; fi
    echo 'Authorized database path works; unauthorized database and API paths are denied.';;
  drift-demo)
    k patch deployment catalog -n ticket --type=json -p='[{"op":"replace","path":"/spec/template/spec/containers/0/resources/requests/cpu","value":"500m"}]'
    for attempt in $(seq 1 60); do
      value="$(k get deployment catalog -n ticket -o jsonpath='{.spec.template.spec.containers[0].resources.requests.cpu}')"
      if [[ "$value" == 50m ]]; then echo 'Argo CD restored the Git-defined 50m CPU request.'; exit 0; fi
      sleep 3
    done
    echo 'Drift was not repaired within 180 seconds'; exit 1;;
  canary-good) ./scripts/demo-release.sh good;;
  canary-fail) ./scripts/demo-release.sh fail;;
  canary-restore) ./scripts/demo-release.sh restore;;
  *) echo "Unknown demo: $command"; exit 1;;
esac
