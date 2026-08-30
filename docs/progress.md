# Progress

## Current

Bean v0.4 production-platform vertical slice is implemented. It advances the four requested planes together: crash-safe single-process runtime semantics, SQLite/PostgreSQL parity, protected System Admin operations, and typed visual Studio authoring. Bean remains alpha because the explicitly excluded operational and assurance work is still material.

## v0.4 milestone evidence

- Milestone 0 — contracts and baseline: failure model, portability boundary, System Admin boundary, visual/canonical definition invariant.
- Milestone 1 — crash-safe SQLite: input-bound idempotency, restart-safe additive publication, startup schema validation, job/outbox leases and deterministic process fault points.
- Milestone 2 — PostgreSQL: pgx adapter, URL selection, shared DBAL contract, SQLSTATE mapping, and Admin/Action/View HTTP parity against PostgreSQL 17.
- Milestone 3 — System Admin: safe user/role data, queue health, releases, migrations, retry/cancel controls, CSRF, affected-row checks, confirmation, and audit.
- Milestone 4 — visual Studio: typed core editors, draft references, lossless advanced JSON, inline diagnostics, migration/release preview, and no-JSON browser authoring.
- Milestone 5 — qualification: reference apps, browser workflows, real crash/restart, real PostgreSQL, compatibility, fuzz-smoke, race, and production build gates.

## Verification log

- `go test ./...` — pass on 2026-08-30.
- Frontend lint, typecheck, and 6 unit tests — pass.
- Focused no-JSON Studio Playwright acceptance — pass.
- `make test-crash` — pass for crash after migration and after activation commit.
- `make test-postgres` — pass against PostgreSQL 17 for reusable DBAL and Admin/Action/View HTTP parity.
- `make check` — pass, including vet, frontend gates, all Go tests, fuzz-smoke, compatibility, black-box, race, and Playwright 9/9.
- `make build` — pass; output is `bin/bean` (`bean 0.4.0-alpha`).
