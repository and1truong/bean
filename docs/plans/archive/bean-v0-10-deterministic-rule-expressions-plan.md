# Bean v0.10 Deterministic Rule Expressions plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze named Rule AST, sources/operators, consumers, bounds, examples, and compatibility | root goal plus compiler/runtime fixture plan | done |
| 1 | Typed bounded deterministic Rule core | type/eval/replay/operator/source/resource-limit tests | done |
| 2 | Rule Definition, AppIR, schema, capabilities, inspect, references, diff, and compatibility | compiler/schema/Agent Protocol/AppIR tests | done |
| 3 | Action guards, simultaneous derived inputs, and Entity invariants | Policy/order/rollback/idempotency/context tests | done |
| 4 | Three metadata-only reference slices and backend/restart parity | source journeys plus SQLite/PostgreSQL/crash tests | done |
| 5 | Documentation, v0.10 version cut, terminal gates, CI, clean review, and merge | all gates, CI, Codex review, merged PR | done |

## Working rules

- Prefer an existing semantic primitive, then a Rule, then the later typed extension boundary.
- Rules are named canonical ASTs, not text scripts; resource bounds and type checking are compiler/runtime contracts.
- Policy authorizes; Rules can only further constrain or derive deterministic local values.
- Derived inputs are server-owned, simultaneous, and unavailable to sibling derives.
- Keep Rules free of I/O, implicit time/randomness, mutation, dynamic lookup, and environment state.
- Add failing contract evidence before each public behavior and run the nearest tests after each milestone.
- Keep `GOAL.md`, `PLANS.md`, `ROADMAP.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
