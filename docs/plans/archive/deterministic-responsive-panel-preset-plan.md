# Deterministic responsive Panel preset plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze preset behavior, thresholds, order, and compatibility | root goal and design note | done |
| 1 | Give ordinary Panels semantic runtime classes and all five responsive mappings | focused React/CSS tests | done |
| 2 | Prove narrow/wide behavior and overflow containment in the maintained tracker journey | Playwright computed-style assertions | done |
| 3 | Document the canonical contract and update the idea inventory | definitions/capabilities/idea docs | done |
| 4 | Qualify the repository | `make check` and `make build` | done |

## Design decisions

- Keep source and AppIR unchanged: the existing closed `Panel.layout` value selects one runtime-owned responsive preset.
- Use fixed viewport thresholds: medium starts at `48rem`; large starts at `64rem`. Container queries remain deferred until Panels can be nested or embedded in constrained parents.
- Preserve source/DOM order at all widths. `two-column` becomes equal columns at medium; sidebars remain stacked until large and then use `1:2` or `2:1`; `grid` changes the ordered children of its `main` Region from one to two to three columns.
- Use `minmax(0, ...)` tracks and `min-width: 0` Regions. Blocks and Displays remain responsible for local table, board, code, and media overflow.
- Presentation selectors may increase spacing but must retain the same medium `two-column` breakpoint and ordering contract.

## Verification order

```bash
cd web && bun run test -- App
cd e2e && bunx playwright test tracker.spec.ts
make check
make build
```
