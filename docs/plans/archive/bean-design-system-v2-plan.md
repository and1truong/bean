# Bean Design System v2 plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Repository audit, design constraints, and migration map | `docs/ui-audit.md`, root goal, actual component inventory | done |
| 1 | Semantic tokens, compact type/spacing scales, light/dark themes, and normalized controls | frontend lint/typecheck and focused primitive tests | done |
| 2 | Stable authenticated application shell and standard page anatomy | shell/theme React tests and responsive browser checks | done |
| 3 | First-class dense tables, filters, forms, and normalized states | Admin/public View/Explore React tests | done |
| 4 | Navigator/workspace/inspector Studio and query-oriented Explore layouts | Studio/Explore React tests and Playwright verification | done |
| 5 | Admin dashboard, generated form, and visual cleanup migration | Admin tests and representative screenshots/traces | done |
| 6 | Design-system documentation, migration notes, and repository qualification | `docs/design-system.md`, `make check`, `make build` | done |

## Working rules

- Independently author MIT-compatible Bean code; use named products only as design references and copy no implementation or assets.
- Preserve routes, accessible names, test IDs, metadata behavior, View reads, Action writes, and release lifecycle boundaries.
- Prefer semantic tokens, shared primitives, borders, compact spacing, and source order over local decorative CSS.
- Keep application behavior in metadata and core visual behavior generic.
- Run the nearest frontend test after each milestone and keep this plan plus `docs/progress.md` current.
