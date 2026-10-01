# Failure model

## Reservations

The booking API serializes matching idempotency keys with a PostgreSQL transaction-scoped advisory lock. It decrements inventory only when enough tickets remain, inserts the order, and inserts a payment job in the same transaction. A failed job insertion rolls back the inventory and order together. Different keys for the same event contend on PostgreSQL's row update; the nonnegative inventory constraint remains enforced.

## Payments

Workers atomically claim eligible jobs using `FOR UPDATE SKIP LOCKED`. A claim assigns a fresh UUID lease token and increments attempts. The lease lasts 30 seconds; normal simulated work lasts three seconds. A crashed or interrupted worker leaves a job eligible for reclaim after lease expiry.

Completion requires the current, unexpired token. It marks the job complete and transitions the pending order in one transaction. A declined or exhausted payment also releases inventory in that transaction. A stale worker and repeated completion cannot change inventory or order state. Retried jobs invalidate their old token immediately. Five attempts are allowed; the next reclaimed attempt fails the order and releases its tickets.

This is durable at-least-once processing with idempotent database effects. A future external payment provider would also need provider-side idempotency; this demo has no external payment side effects.

## Kubernetes recovery

Stateless workloads have two baseline replicas, readiness checks, preferred hostname spreading, and 30-second not-ready/unreachable tolerations. Preferred spreading permits replacements on the surviving worker. Node detection and eviction take time, so a node failure may briefly affect in-flight requests; clients can retry safely. PodDisruptionBudgets protect voluntary disruptions, not unexpected failures.

API HPAs scale between two and four replicas using CPU from Metrics Server. All backend replicas use at most five database connections, staying below PostgreSQL's 100-connection budget even with canary surge. The scheduler uses requests; container limits bound CPU/memory usage. Every kind node reports host resources, so the sum of apparent node capacity overstates physical capacity. Load testing remains bounded and measures the shared host.

PostgreSQL storage is retained and mounted from `.local/data` on the first worker. PostgreSQL pod restart preserves data. Losing its worker causes an outage until that node is restored; the volume does not fail over. All kind nodes run on one host, so host loss is a total outage. Argo CD/Prometheus are compact single-instance deployments and are not HA.

## GitOps ownership

Argo CD owns desired workloads and infrastructure. Rollouts owns booking traffic weights and revision-specific service selectors; HPA owns API replica counts. Bootstrap owns host-specific MetalLB addressing and locally generated secrets. These local resources must be recreated during bootstrap and are intentionally not committed to a public repository.

Canary abort restores stable routing, not Git history. Revert the bad desired-state change to make Git and the cluster agree again. Database migrations are additive and run before application rollout so old and new pods can coexist.
