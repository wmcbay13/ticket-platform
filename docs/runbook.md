# Demo runbook

Run commands from the repository root after `make bootstrap` finishes. Set `KUBECONFIG=$PWD/.local/kubeconfig` for direct kubectl commands and use context `kind-ticket-platform`.

## 1. Show the system

Run `make status`: three nodes, independent workloads, two baseline service replicas across workers, HPA targets, and Argo applications. Open the application, reserve two tickets, and watch pending become confirmed. Repeat with decline and retry. Run `make smoke` for an automated check.

Open Grafana with `make grafana-ui`, then **Ticket Platform — Operations** and **Kubernetes / Compute Resources / Pod**. Open Argo CD with `make argocd-ui` to show Git as desired state. Obtain local passwords only from the documented local files/command; do not include them in screenshots.

## 2. Traffic spike and limits

Run `make load` in one terminal and watch `kubectl -n ticket get hpa,pods -w` in another. The test ramps from 20 to 500 requests/sec, with a maximum of 100 virtual users. Compare CPU usage with requests/limits and inspect throttling, request latency, and desired replicas. Watch scale-down after the five-minute stabilization window.

The summary JSON is written to `.local/results/`. Record achieved request rate, p95, error fraction, replica range, and any dropped iterations. If the host cannot sustain the target, report the measured limit and free memory rather than claiming the target passed.

## 3. Recovery and policy

Run `make failure-pod` to replace a catalog pod while another replica serves traffic. Run `make failure-worker-node` to stop `ticket-platform-worker2`, wait for replacements, verify booking behavior, and restore the node automatically. Never use the storage worker for this demo.

Run `make policy-test`: a pod with the permitted migration identity can reach PostgreSQL; an unrelated pod cannot reach PostgreSQL or the catalog API. Actual ingress traffic continues through Traefik. Run `make drift-demo` and show Argo CD restoring the catalog's Git-defined CPU request.

## 4. Progressive release

Keep traffic running while changing the booking template:

```bash
# Terminal A
make canary-traffic

# Terminal B
make canary-good
kubectl --context kind-ticket-platform -n ticket get rollout booking -w
kubectl --context kind-ticket-platform -n ticket get analysisruns -w
```

The healthy revision progresses through 10%, 50%, and 100% after its quality checks. Use the Rollouts conditions and AnalysisRun metric results as evidence. Without sufficient traffic the release pauses; no-data does not count as success. If an analysis is inconclusive, keep traffic running and retry/promote through the Rollouts UI/CLI after inspecting its measurements; do not bypass failed quality checks.

Then run `make canary-fail` while `make canary-traffic` continues. The canary remains healthy at `/healthz` but deliberately returns 503 for API calls. Prometheus detects the canary's failures and Rollouts aborts to stable routing. Argo CD displays the failed desired revision. `make canary-restore` restores the canonical template through Git; keep traffic running until reconciliation completes.

Demo commands commit only to `gitops-demo`. The next normal pipeline promotion replaces these demo changes with its canonical configuration. Avoid running a demo release concurrently with an application release; conflicting pushes fail rather than force-overwrite another release.

## 5. Prepare merge

Disable preview deployments with `gh variable set PREVIEW_DEPLOY_ENABLED --body false`. Verify the feature PR's checks and local evidence, then review it for merge. After merging, pushes to `main` automatically publish and promote. No self-hosted GitHub runner or inbound access to the local cluster is needed.

## Cleanup

`make teardown` deletes only the named kind cluster. Database files and credentials remain under `.local/`; keep both to reuse the data on the next bootstrap. Metrics and Grafana runtime state are ephemeral and regenerated from Git. Delete the database directory and credentials explicitly only when intentionally resetting all demo data.
