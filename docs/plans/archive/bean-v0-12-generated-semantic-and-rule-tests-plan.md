# Bean v0.12 Generated Semantic and Rule Tests plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze generation inputs, identity/evidence contract, oracle boundary, bounds, and exclusions | root goal and reference-fixture plan | done |
| 1 | Deterministic generated-TestSuite materialization and stable machine evidence | generator/compiler/Agent Protocol replay and ordering tests | done |
| 2 | Rule replay, context, forbidden-capability, and resource-bound checks | seeded calculation/guard/validation/context/limit defect tests | done |
| 3 | Generated Policy denial and invalid Lifecycle transition cases | maintained ATS/commerce negative defect tests | done |
| 4 | DemoSeed-backed CRUD, route-binding, and browser-journey checks | deterministic SQLite HTTP/reference-application tests | done |
| 5 | Documentation, v0.12 version cut, terminal gates, CI, review, and merge | all gates, CI, Codex review, merged PR | done |

## Working rules

- Generate only claims proven by canonical AppIR, explicit TestSuite expectations, or deterministic DemoSeed data.
- Materialize runtime cases as ordinary TestSuite definitions and execute the production runner.
- Keep explicit expectations as the oracle; never derive a business expectation by evaluating the implementation under test.
- Trace stable generated IDs to source Definition kind/name and sort all evidence canonically.
- Keep Views for reads, Actions for writes, production HTTP for journeys, and application behavior in metadata.
- Add failing seeded-defect evidence before each generated family and run its nearest tests.
- Keep `GOAL.md`, `PLANS.md`, `ROADMAP.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
