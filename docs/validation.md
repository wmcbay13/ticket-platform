# Validation evidence

This file records measured checks from implementation. Local cluster evidence will be added after the deployed stack is exercised.

- Backend unit tests and Go race checks passed.
- PostgreSQL integration tests passed: 80 concurrent reservations accepted exactly 20 tickets; 40 concurrent duplicate requests created one order; stale leases could not complete; repeated failures did not release inventory twice; transaction rollback restored inventory and removed the partial order.
- Frontend TypeScript build and API tests passed.
- Full npm dependency audit reports zero vulnerabilities after updating Vite/Vitest.
- Kustomize rendering and YAML syntax checks passed.

The integration suite uses an isolated, disposable PostgreSQL container. It does not run against the demo's persistent database.
