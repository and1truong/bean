# Bean v0.2 semantic-correctness plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Honest capability baseline and fail-closed compiler | `docs/capabilities.md`; compiler contract tests | done |
| 1 | Strict definitions, canonical compilation, versioned AppIR | definition/compiler/compatibility tests | done |
| 2 | Complete Action executor, bindings, rollback, idempotency | Action operation and transaction contract tests | done |
| 3 | Complete View query plan, joins, aggregates and cursor paging | DBAL/View/renderer contract tests | done |
| 4 | Relations, policy enforcement matrix, Webforms and render context | boundary and UI contract tests | done |
| 5 | Deterministic releases, fuzz smoke, compatibility, black-box gates and docs | new Make targets plus clean build | done |

## Working rules

- Add a failing contract test before closing each capability gap.
- Unknown fields and unsupported semantics fail publication.
- Accepted source definitions compile to executable typed AppIR.
- Reads flow through Views; writes flow through Actions.
- Policy predicates are applied before data leaves the database boundary.
- Keep `docs/capabilities.md` and `docs/progress.md` synchronized with evidence.

## Terminal gates

```bash
make bootstrap
make check
make build
```
