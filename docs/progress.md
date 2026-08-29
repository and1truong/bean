# Progress

## Current

All Bean v0.1 milestones are complete.

## Verification log

Commands and outcomes are recorded here as milestones complete.

- `go test ./internal/dbal/...` — pass; DBAL compiler and SQLite contract tests.
- `go test ./...` — pass; unit and integration coverage across compilation, migrations, release safety, authentication, fields, Actions, Views, tenant isolation, Webforms, and OpenAPI.
- `go test -race ./...` — pass.
- `bun run lint`, `bun run typecheck`, `bun run test`, `bun run build` — pass.
- Playwright — 9/9 pass: Studio builder, seven reference applications, and OpenAPI/offline docs.
- `make check` — pass on 2026-08-29.
- `make build` — pass; output is `bin/bean`.
- `make bootstrap` — pass with both Bun frozen lockfiles unchanged.
