# Bean v0.4 production-platform vertical-slice plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Cross-plane contract, write-path audit, and failing baseline tests | architecture notes and contract tests | done |
| 1 | Crash-safe SQLite semantics and deterministic fault harness | Action/release/queue tests; `make test-crash` | done |
| 2 | PostgreSQL DBAL, migration, runtime, and parity suite | real backend contracts; `make test-postgres` | done |
| 3 | Protected system administration and recovery controls | HTTP, React, and browser tests | done |
| 4 | Typed Studio editors and visual core-definition workflow | round-trip unit and no-JSON browser tests | done |
| 5 | Cross-plane qualification, docs, full gates, and goal archive | all terminal gates | done |

## Working rules

- Add failing contract evidence before changing each semantic boundary.
- Keep definitions/AppIR backend-neutral and keep SQL inside adapters.
- Reads flow through Views; writes flow through Actions on every UI and backend.
- Visual Studio edits the canonical definition format; JSON remains lossless.
- Admin system controls are authenticated, CSRF-protected, confirmed, and audited.
- Fast deterministic gates run routinely; repeated qualification is a separate target.
- Keep `docs/capabilities.md` and `docs/progress.md` synchronized with evidence.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
