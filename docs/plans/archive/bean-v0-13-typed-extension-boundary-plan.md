# Bean v0.13 Typed Extension Boundary plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze Extension metadata, after-commit transaction boundary, HTTP wire contract, host auth, retry/idempotency/failure semantics, TestSuite mocks, and exclusions | root goal and reference-fixture plan | done |
| 1 | Canonical Extension Definition, AppIR format, schema, capabilities, inspect, references, diff, validation, and compatibility | compiler/schema/Agent Protocol/AppIR/release tests | done |
| 2 | Transactional Action extension intents and bounded HTTP delivery | Policy/rollback/audit/idempotency plus HTTP timeout/retry/auth/output tests | done |
| 3 | Offline typed provider mocks, interaction assertions, and metadata-only commerce slice | TestSuite runner and commerce source/restart/parity tests | done |
| 4 | Documentation, v0.13 version cut, terminal gates, CI, review, and squash merge | all gates, CI, Codex review, merged PR | done |

## Working rules

- Extension calls are typed after-commit intents; never hold a database transaction open across HTTP.
- Persist the intent with the Action transaction and reuse one invocation/idempotency identity for every at-least-once attempt.
- Keep credentials in host configuration and redact provider details; application metadata declares requirements, not secrets.
- Reuse the production intent/delivery path in Semantic TestSuites with an injected typed provider mock and no network.
- Add only the HTTP transport and commerce notification slice; defer provider SDKs, WASM, scripts, synchronous results, and infrastructure expansion.
- Keep Views for reads, Actions for writes, application behavior in metadata, and core packages generic.
- Add failing contract evidence before each public behavior and run its nearest tests.
- Keep `GOAL.md`, `PLANS.md`, `ROADMAP.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
