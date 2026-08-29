# Progress

## Current

Bean v0.2 semantic correctness is implemented. Bean remains alpha software; the next production-readiness goals are crash durability/recovery, external security review, operational/load qualification, and signed release engineering.

## Milestone evidence

- Milestone 0: honest capability matrix and strict unsupported-metadata diagnostics.
- Milestone 1: typed deterministic bindings, `bean/appir/v1`, canonical compilation and compatibility fixture.
- Milestone 2: complete Actions, typed I/O, named results, rollback/concurrency, audit/outbox and idempotency.
- Milestone 3: one View plan for joins, filters, groups, five aggregates, offset/keyset pagination, policies and redaction.
- Milestone 4: constrained relations, policy matrix, complete Webforms, registered renderers and typed context.
- Milestone 5: deterministic migrations and contract, fuzz-smoke, compatibility, black-box, race and browser gates.

## Verification log

- `make bootstrap` — pass on 2026-08-30; frozen dependencies unchanged.
- `go test ./...` — pass.
- `make test-contract`, `make test-fuzz-smoke`, `make test-compatibility` — pass.
- `go test -race ./...` — pass.
- Frontend lint, typecheck, unit test and production build — pass.
- Playwright — 9/9 pass outside the filesystem/process sandbox.
- `make check` — pass, including race and Playwright 9/9.
- `make build` — pass; output is `bin/bean` (`bean 0.2.0-alpha`).
