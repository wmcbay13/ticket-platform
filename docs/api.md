# API

All browser traffic uses Traefik's same-origin `/api` paths. Responses are JSON. Each API response includes `X-Request-ID` and `X-App-Version`.

| Method | Path | Behavior |
| --- | --- | --- |
| GET | `/api/events` | Seeded events, prices in cents, and current availability |
| GET | `/api/events/{id}` | One event; 404 if unknown |
| POST | `/api/orders` | Atomically reserve tickets and create a pending payment job |
| GET | `/api/orders/{id}` | Current order status and reservation details |

```bash
curl -X POST "$BASE_URL/api/orders" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: example-reservation-1' \
  -d '{"event_id":"neon-nights","quantity":2,"payment_scenario":"success"}'
```

`quantity` is an integer from 1 through 10. `payment_scenario` is `success`, `decline`, or `retry`. The retry scenario fails transiently once and then succeeds. Each attempt simulates a three-second payment delay. No card data is accepted.

The idempotency key must be nonblank and at most 128 characters. The first successful request returns 201, `Location`, and the pending order. Identical repeated requests return 200 and the original order, including its current status. Reusing a key with different reservation details returns 409. Concurrent first requests with the same key also create only one order.

Validation errors return 400; incorrect Content-Type returns 415; unknown resources return 404; insufficient inventory or conflicting keys return 409. Database unavailability returns a generic 503. Request bodies are bounded to 4 KiB and may contain only the documented fields.

```json
{
  "id": "a3b8e560-6d67-417e-8560-f701d9d0fa17",
  "event_id": "neon-nights",
  "quantity": 2,
  "total_cents": 9000,
  "status": "pending",
  "payment_scenario": "success",
  "created_at": "2026-10-01T12:00:00Z",
  "updated_at": "2026-10-01T12:00:00Z"
}
```

`pending` transitions once to `confirmed` or `failed`. Failed orders release their quantity back to the event. A confirmed order retains its reserved inventory. Order IDs are random UUIDs and act as guest lookup links; this is a local demo, not a production authentication mechanism.

Each backend exposes `/healthz`, `/readyz`, and `/metrics` internally. Readiness checks database reachability; liveness checks the process independently so a database outage does not cause restart storms.
