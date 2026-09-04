# Sealed internal capability registries plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Inventory repeated discriminators and freeze internal/external boundaries | root goal plus hotspot and retained-switch rationale | done |
| 1 | Immutable deterministic registry primitive | duplicate, lookup, ordering, and sealing tests | done |
| 2 | Definition-kind registry across compile/schema/inspect/reference paths | compiler/schema/Agent Protocol parity tests | done |
| 3 | Action-step registry with declared effects and runtime/compiler parity | Action/compiler/DemoSeed safety tests | done |
| 4 | Block-type registry and evidence-based operation/presentation decision | compiler/render/component parity tests and recorded rationale | done |
| 5 | Documentation, terminal gates, CI, and clean reviewed PR | all gates, CI, and Codex review | done |

## Working rules

- Preserve behavior and public contracts; this goal adds no application capability.
- Prefer registries only for repeated extension seams; retain explicit switches for closed algebras and security-sensitive orchestration.
- Keep registries immutable, explicitly constructed, deterministic, and free of `init()` registration.
- Add parity/failure evidence before replacing each dispatcher and run the nearest tests after every milestone.
- Keep `GOAL.md`, `PLANS.md`, `ROADMAP.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
