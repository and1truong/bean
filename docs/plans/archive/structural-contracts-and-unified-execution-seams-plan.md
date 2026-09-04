# Structural contracts and unified execution seams plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze compatibility, security fixes, and deep-module interfaces | root goal, ordered tracer bullets, and baseline focused tests | done |
| 1 | Structural diagnostic facts and rule-owned codes | wording-independence, recovery, candidate, and diagnostic contract tests | done |
| 2 | One View-owned read engine for database and transaction adapters | policy/rich-text/relation/order/limit equivalence tests | done |
| 3 | Enforced Action-step effects and one entity resolver | registry-wide read/write obligation and mismatch tests | done |
| 4 | Context-specific value-source catalog | resolver/compiler/redaction parity and fail-closed tests | done |
| 5 | Typed client render dispatch and expression parity | pure render/operator tests including explicit unknown failures | done |
| 6 | Shared client write encoder/caller and field errors | JSON/multipart/batch/Admin/Webform tests | done |
| 7 | Complete Definition-kind ownership and explicit phases | independent AppIR storage completeness and per-kind validation tests | done |
| 8 | Sealed Agent Protocol operation entries and owned capabilities | construction, authorization, discovery, and capability parity tests | done |
| 9 | Deletion cleanup, documentation, and qualification | focused tests plus all terminal gates | done |

## Working rules

- Land security and machine-contract tracer bullets before broader ownership cleanup.
- Preserve public behavior with before/after contract evidence; make demonstrated silent-failure and policy fixes explicit.
- Keep View reads and Action writes, immutable AppIR activation, and backend confinement intact.
- Prefer one deep module over shared pass-through helpers; retain explicit closed-algebra and compiler phase control flow.
- Run the nearest focused test after every milestone and keep `GOAL.md`, `PLANS.md`, `ROADMAP.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
