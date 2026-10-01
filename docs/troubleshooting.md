# Troubleshooting

| Symptom | Check / remedy |
| --- | --- |
| Docker socket permission denied | Verify `docker info` in the terminal that runs bootstrap. After joining the Docker group, start a new login/session or use `newgrp docker`. |
| Bootstrap waits for MetalLB | Verify `gitops-demo` exists and inspect `kubectl -n argocd get applications`; check repository/chart connectivity and root application conditions. |
| ImagePullBackOff for GHCR | Public repo visibility is separate from package visibility. Make both container packages public in GitHub package settings, then retry. |
| kind nodes remain NotReady | Cilium must install successfully before node readiness. Inspect Cilium pods/logs and check host kernel/inotify limits. |
| App readiness fails | Inspect the migration hook, database pod, secret presence, and Cilium policies. Database credentials must match retained database data. |
| External IP stays pending | Confirm the bootstrap-created IPAddressPool/L2Advertisement and MetalLB controller/speakers. The pool must belong to the Docker kind network. |
| HPA shows unknown CPU | Verify Metrics Server is healthy, `kubectl top pods -n ticket` works, and container CPU requests exist. Prometheus is not HPA's resource metrics API. |
| Canary pauses | Inspect AnalysisRuns: no/insufficient samples are inconclusive. Run `make canary-traffic`; confirm the booking PodMonitor target and rollout_hash label are present. |
| Canary aborts | Inspect canary-specific 5xx and latency metrics. Revert the desired GitOps configuration; stable traffic is restored by Rollouts. |
| Pods fail or host swaps heavily during load | Close other applications, inspect working-set/limits, and reduce the load target. All kind nodes share physical RAM and CPU. |
| Recreated cluster cannot start PostgreSQL | Keep `.local/credentials.json` with `.local/data`; changing the initial password variable does not reset an existing PostgreSQL password. |
| Policy-test fails unexpectedly | Verify policy-probe pod labels, Cilium endpoint identities, and DNS. Allowed and denied probes must produce different outcomes. |

For local diagnostics, use the explicit `kind-ticket-platform` context and `.local/kubeconfig`. Never apply this demo's teardown or failure commands to another cluster. Monitor credentials, PostgreSQL passwords, and kubeconfigs are ignored and must not be copied into issue reports.
