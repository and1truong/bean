# Bean v0.1 implementation plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Verification | Status |
| --- | --- | --- | --- |
| 0 | Repository contract, docs skeleton, Go/React shells, locked tooling | backend and frontend build | done |
| 1 | Typed DBAL, SQLite adapter, transactions, schema inspection, error mapping | `go test ./internal/dbal/...` | done |
| 2 | Typed definitions, compiler, AppIR, additive migrations, releases | compiler/release integration tests | done |
| 3 | Entities, fields, authentication, policy, Actions, audit/outbox/jobs | runtime integration tests | done |
| 4 | Views, JSON/CSV/RSS, OpenAPI, HTTP API | renderer and API tests | done |
| 5 | Webforms, render tree, Blocks/Panels/Pages, embedded React runtime/admin | frontend and server tests | done |
| 6 | Studio editors, diagnostics, migration preview, publish flow | Studio Playwright suite | done |
| 7 | Seven metadata-only reference applications and workflows | reference Playwright + concurrency tests | done |
| 8 | Security/ops review, CI, documentation, clean binary | `make check && make build` | done |

## Decisions

- Use `modernc.org/sqlite`, a maintained pure-Go driver.
- Store definition envelopes as YAML and their typed specifications as normal YAML maps decoded into Go structs at compile time.
- Use an immutable `appir.App` behind an atomic pointer for the request hot path.
- Use a small generic React component registry. Studio exposes structured envelope fields plus an advanced JSON spec editor.
- Run one application and one embedded worker per process; SQLite remains the transactional authority.
