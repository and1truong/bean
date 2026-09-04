# Bean v0.9 Semantic Application Model plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Catalogue existing transition use, freeze the minimal Lifecycle and Action-binding contract, and define compatibility behavior | ATS/commerce fixtures plus schema and diagnostic test plan | done |
| 1 | Canonical Lifecycle schema, compiler validation, immutable AppIR, capabilities, inspect, and semantic diff | focused schema/compiler/AppIR/Agent Protocol tests | done |
| 2 | Policy-aware Lifecycle enforcement through Actions with safe publication and restart behavior | positive/negative Action, release, and crash contracts | done |
| 3 | Convert ATS candidate and commerce order flows to the shared primitive | source validation plus independent application journeys | done |
| 4 | Preserve legacy transition compatibility and SQLite/PostgreSQL parity | compatibility, CLI/MCP parity, backend, and bypass-refusal tests | done |
| 5 | Documentation, version cut, terminal gates, CI, and clean reviewed PR | all gates, CI, and Codex review | done |

## Working rules

- Add only Lifecycle in v0.9; later semantic candidates need their own evidence.
- Freeze the source and Action-binding contract before runtime implementation.
- Keep transition authorization in Policies and transition mutation in Actions.
- Normalize semantics once in the compiler; CLI and MCP consume shared Agent Protocol results.
- Maintain the declared legacy Action transition representation until compatibility tests prove the migration path.
- Add failing contract evidence before each public behavior and run the nearest test after every milestone.
- Keep `GOAL.md`, `ROADMAP.md`, `PLANS.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
