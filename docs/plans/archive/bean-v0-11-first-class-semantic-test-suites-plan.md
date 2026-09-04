# Bean v0.11 First-class Semantic Test Suites plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze TestSuite targets, fixtures, context, assertions, isolation, bounds, and compatibility | root goal plus compiler/runner fixture plan | done |
| 1 | Canonical TestSuite Definition, AppIR, schema, capabilities, inspect, references, diff, and compatibility | compiler/schema/Agent Protocol/AppIR tests | done |
| 2 | Deterministic isolated Rule suite runner through the production evaluator | result/error/context/replay/bound tests | done |
| 3 | Deterministic isolated Action suite runner through the production Action service | Policy/guard/derive/invariant/mutation/event/rollback tests | done |
| 4 | Existing `app test` machine contract plus maintained metadata-only reference suites | CLI/Agent Protocol and commerce/ATS/booking defect tests | done |
| 5 | Documentation, v0.11 version cut, terminal gates, CI, clean review, and merge | all gates, CI, Codex review, merged PR | done |

## Working rules

- TestSuite is metadata and immutable AppIR, not repository-only scripting or a separate interpreter.
- Target only Rule and Action in v0.11; exercise Policy and Lifecycle through Actions.
- Use explicit context, fixed time, deterministic IDs/seed, bounded data, and fresh per-case state.
- Reuse the production Rule evaluator and Action service; assertions observe results, records, and outbox evidence.
- Keep provider mocks, generated tests, standalone commands, and PostgreSQL suite execution out of v0.11.
- Add failing contract evidence before each public behavior and run the nearest test after every milestone.
- Keep `GOAL.md`, `PLANS.md`, `ROADMAP.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
