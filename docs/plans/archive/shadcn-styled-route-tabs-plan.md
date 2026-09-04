# Shadcn-styled route tabs plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Preserve route semantics and freeze closed `default`/`line` variants | root goal and ADR 0010 | done |
| 1 | Compile normalized Menu variants into immutable AppIR | AppIR/compiler/schema/capability tests | done |
| 2 | Add a source-owned shadcn-style RouteTabs adapter | React semantic and variant tests | done |
| 3 | Apply and visually verify the Books navigation | Playwright geometry/active-state evidence | done |
| 4 | Qualify the repository | `make check` and `make build` | done |

## Working rules

- Reuse shadcn Tabs visual classes, not Radix Tabs behavior, for route-backed Menu links.
- Configure one closed variant per typed Menu; do not expose arbitrary CSS or per-placement styling.
- Keep legacy flat Menu rendering and mobile native selects unchanged.
