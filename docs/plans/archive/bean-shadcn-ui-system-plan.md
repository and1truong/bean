# Bean shadcn/ui system plan (completed)

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Tailwind v4, source-owned shadcn primitives, Bean tokens, and lint guard | frontend lint, typecheck, and build | done |
| 1 | Shell/Auth and metadata-driven public UI migration | React public rendering tests | done |
| 2 | Application Admin and System Admin migration with accessible confirmations | React Admin and CMS/blog browser tests | done |
| 3 | Studio migration, responsive browser coverage, docs, and qualification | Studio tests and terminal gates | done |

## Working rules

- Keep shadcn components checked in under `web/src/components/ui`; do not require a frontend runtime service.
- Preserve routes, accessible names, stable test IDs, metadata behavior, and View/Action data boundaries.
- Keep native selects for dynamic and multi-select forms; do not introduce a second client validation schema.
- Keep application-specific presentation in metadata rather than branching in core React code.
- Run the nearest frontend test after each milestone and keep `GOAL.md` and `docs/progress.md` current.
