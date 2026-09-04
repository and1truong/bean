# Bean v0.14 First-class View Displays plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze View/display ownership, display types, renderer/control/pager/title contracts, compatibility normalization, reference slices, and exclusions | root goal plus compiler/runtime/browser fixture plan | done |
| 1 | Canonical display AppIR, schema, capabilities, inspect, references, semantic diff, route validation, and v0.13 source/AppIR compatibility | compiler/schema/Agent Protocol/AppIR/release tests | done |
| 2 | Named page/block displays with unified list/detail execution, static/result-derived titles, browser titles, and legacy Block-presentation normalization | HTTP/Page/Block/View/React compatibility tests | done |
| 3 | Public table renderer with typed ordered columns, labels, safe links, inferred formatting, responsive overflow, and empty states | compiler/React/HTTP tests plus ATS table-page journey | done |
| 4 | Display-owned exposed controls and cursor pager with typed operators/defaults/widgets, URL state, immutable binding separation, and independent block state | View/HTTP/security/React/PostgreSQL tests plus filter/pager journey | done |
| 5 | Move board/tree/metric/timeline presentation behind named displays, share read-only UI primitives with Admin, expand Studio's focused View editor, and migrate maintained metadata | renderer parity, Studio, source, package/restart, blog and ATS tests | done |
| 6 | Documentation, v0.14 version cut, terminal gates, and local review readiness | all gates, final diff checks, release-ready branch | done |

## Working rules

- `View` is the public read-and-presentation primitive; query planning remains internal and uses the existing View service.
- Keep existing query fields at the View top level. Do not spend v0.14 on a nested query DSL rewrite.
- Page displays cover simple single-View routes; Page/Panel/Block remains the composition model for multi-component pages.
- New metadata puts presentation in named View displays. Legacy `Block.presentation` is compatibility input only and normalizes into the same runtime contract.
- Exposed filters define typed query inputs; display controls own labels, widgets, defaults, and visibility.
- Prefer opaque cursor navigation and deterministic ordering; do not add total counts or numeric paging without a separate measured requirement.
- Reuse one closed renderer/control registry across compiler capabilities and React dispatch; do not add application-supplied code or dynamic runtime registration.
- Keep Admin mutations, selection, forms, and audit behavior Admin-owned while sharing equivalent read-only table/control/pager primitives.
- Add failing evidence before each public behavior, migrate one tracer-bullet display at a time, and run the nearest test after every milestone.
- Keep `GOAL.md`, `PLANS.md`, `ROADMAP.md`, `docs/capabilities.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
