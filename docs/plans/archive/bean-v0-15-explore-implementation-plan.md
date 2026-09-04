# Bean v0.15 Explore implementation plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its user-visible flow and listed evidence pass.

| Milestone | User-visible deliverable | Example slice | Evidence | Status |
| --- | --- | --- | --- | --- |
| 1 | Open any Entity, explore records, and save an ordinary View/Display draft | ATS candidate explorer | compiler/preview/Studio/React/ATS tests | done |
| 2 | Group and aggregate with deterministic typed semantics | ATS recruiting groups; CRM owner-scoped deal aggregates | compiler/View/DBAL/SQLite/PostgreSQL policy tests | done |
| 3 | Switch compatible table/list/cards/board/chart/metric/timeline/calendar Displays | ATS record/group/metric Views; Booking calendar | renderer compatibility and browser tests | done |
| 4 | Compose a dashboard with typed page-level filters | ATS recruiting overview; Asana project context | compiler/Page/HTTP/React/browser tests | done |
| 5 | Drill from metrics and chart groups into exact contributing records | ATS stage/department/active drills; CRM deal drills | exact-ID and non-leakage tests | done |
| 6 | Execute record and bounded bulk Actions, report partial results, and refresh | ATS `move_candidate`; CRM `move_deal`; Commerce `advance_order` | Policy/Lifecycle/concurrency/audit/refresh tests | done |
| 7 | Author the same artifacts through Explore, Studio, and an agent | ATS recruiting prompt; Asana split-source edit | schema/diagnostic/inspect/diff/TestSuite/Agent Protocol tests | done |
| 8 | Qualify five-minute existing-Entity onboarding and maintained examples | Tracker status slice plus all reference regressions | benchmark, package/restart, terminal gates | done |

## Dependency order

```text
M1 canonical View plan + draft save
  -> M2 result shapes and aggregate truth
     -> M3 compatible rendering
        -> M4 shared page filter context
           -> M5 typed contribution mapping
              -> M6 Action availability and invalidation
                 -> M7 authoring parity
                    -> M8 qualification
```

The sequence is deliberately vertical. A milestone may add compiler/runtime/UI support only when the named example exposes it as a usable application behavior. `v0.14-view-displays` remains a compatibility baseline, not a refactor target.

## M1 — Universal Entity Explorer and saved Views

### Outcome and demo flow

An administrator opens `/explore`, selects `candidate`, chooses projection/search/filter/sort, previews Policy-scoped rows with cursor paging, switches among already compatible record Displays, and saves `candidate_records` plus a named table Display into the current Studio draft. The saved YAML validates and can be inspected/diffed before publication.

### Exact definition and example changes

- Modify `examples/ats/app.yaml`.
- Preserve Entity `candidate`, generated `candidate_list`, Lifecycle `candidate_pipeline`, and Action `move_candidate`.
- Add ordinary View `candidate_records` with fields `id`, `job_id`, `name`, `email`, `stage`, `applied_at`, and `summary`; relationship alias `job`; View-owned search fields `name`, `email`, `summary`; exposed filters `job`, `department`, `stage`, `applied_from`, and `applied_to`; deterministic `applied_at desc, id asc` ordering.
- Add named record Displays `table`, `list`, `cards`, and `board` as their renderer contracts become available. M1 needs table/list; later names may be present only when their renderer milestone lands.
- Keep current DemoSeed counts. Add an exact View fixture/test asserting stable projected fields, search results, filter coercion, order, and first/next cursor behavior.

### Runtime/compiler/UI changes

- `internal/appir/appir.go`: next AppIR format, View-owned search metadata, canonical result-shape placeholder, compatibility fields.
- `internal/compiler/compiler.go`, `internal/compiler/definition_kinds.go`, `internal/compiler/schema.go`: compile one canonical View plan, normalize legacy renderer `searchFields`, validate preview-equivalent source, expose schema/capabilities/inspect/references/diff.
- `internal/view/view.go` and preferably a focused new `internal/view/plan.go`: share planning/execution between named Views and ephemeral preview; do not duplicate filtering or Policy code.
- `internal/httpapi/server.go`: administrator-only Entity catalogue, bounded preview, and save-to-current-draft endpoints. Preview cannot accept Policy/actor/tenant/table/SQL input.
- Add `web/src/Explore.tsx`; update `web/src/main.tsx`, `web/src/App.tsx`, `web/src/api.ts`, and tests. Reuse Studio draft/version-conflict behavior and existing table/control/pager components.
- `web/src/Studio.tsx`: link to saved definitions; do not create an Explore-only persistence store.

### Dependencies and verification

- Depends only on v0.14 and generated `<entity>_list` behavior.
- Focused commands:

```bash
go test ./internal/compiler ./internal/view ./internal/httpapi ./internal/appir
cd web && bun run test -- Explore App Studio
cd e2e && bunx playwright test ats.spec.ts studio-builder.spec.ts
```

- Acceptance: unknown/sensitive/redacted fields, invalid operators/values, excessive limits, stale draft versions, and unauthorized access fail with structured errors; preview and the saved/published View return the same ordered rows for the same actor and inputs.
- Non-goals: grouping, charts, page filters, drill, Actions, per-user presets, and data import.

## M2 — Typed grouping, aggregation, and result shapes

### Outcome and demo flow

The ATS shows candidate counts by stage/job/department and active candidate count. CRM shows deal count and amount by stage for the current actor. A salesperson and manager receive different correct aggregates without hidden categories.

### Exact definition and example changes

- Modify `examples/ats/app.yaml`:
  - add `active_candidate_total` with fixed exclusion of `hired`/`rejected`;
  - add `candidates_by_stage`, `candidates_by_job`, and `candidates_by_department` grouped Views;
  - add `recent_candidates` for later dashboard composition;
  - reuse `candidate`, relationship `job`, Policy `public`, and existing DemoSeed.
- Modify `examples/crm/app.yaml`:
  - add `crm_page_access` for salesperson/manager Page access without replacing row Policy;
  - add `deal_records`, `deals_by_stage`, `pipeline_amount_by_stage`, and `open_pipeline_value`, all with `owned_records`;
  - keep `deal_pipeline` and `move_deal`;
  - add exact fixtures created as salesperson A, salesperson B, and manager, with distinct deal IDs/statuses/amounts. Do not treat one-owner DemoSeed as security evidence.

### Runtime/compiler/UI changes

- `internal/appir/appir.go`: typed group entry `{field, as, bucket}` and explicit `records|detail|metric|groups` result shape; legacy string group normalization.
- `internal/compiler/compiler.go` and `internal/compiler/view_display_test.go`: type-check aggregate inputs/aliases/sorts, reject money average, to-many aggregate traversal, redacted references, result-shape/display mismatches, alias collisions, and invalid date buckets.
- `internal/view/view.go`: normalize empty aggregates, stable group bounds, and contribution predicates while retaining Policy-first execution.
- `internal/dbal/dbal.go`, `internal/dbal/sqlite/compiler.go`, and `internal/dbal/postgres/compiler.go`: backend-equivalent UTC date buckets, explicit null ordering, group limit probe, and typed aggregate results.
- `internal/httpapi/server.go`: expose result-shape metadata and structured `result_limit_exceeded` without leaking hidden row/group counts.

### Dependencies and verification

- Depends on M1's canonical plan.
- Focused commands:

```bash
go test ./internal/compiler ./internal/view ./internal/dbal/sqlite ./internal/dbal/postgres ./internal/httpapi
cd e2e && bunx playwright test ats.spec.ts crm.spec.ts
make test-postgres
```

- Acceptance: `count(id)` empty is zero; other empty aggregates are null; money sums preserve minor units; null groups are stable; group order matches SQLite/PostgreSQL; group overflow fails rather than truncates; CRM exact expected counts/sums differ correctly by actor and manager.
- Non-goals: to-many/distinct aggregates, average money, currency conversion, computed fields, pivots, arbitrary time zones, and chart rendering.

## M3 — Compatible Displays and switching

### Outcome and demo flow

Users switch among named compatible Displays without changing the View query: ATS candidates switch table/list/cards/board; grouped stage data switches table/chart; active total renders metric. Booking records render on a calendar when that slice is accepted.

### Exact definition and example changes

- Modify `examples/ats/app.yaml`:
  - complete named `candidate_records` record Displays;
  - add table/chart Displays to `candidates_by_stage` and metric Display to `active_candidate_total`;
  - retain existing timeline and board behavior as compatibility evidence.
- Modify `examples/booking/app.yaml` as the required calendar proof:
  - add deterministic DemoSeed for `resource` and `booking` or fixed E2E fixtures;
  - give `resource_calendar` a page/block calendar Display using `start_at` and `end_at`;
  - add a table Display over the same record-shaped View.

### Runtime/compiler/UI changes

- `internal/appir/appir.go` and `internal/view/display.go`: closed `cards`, `chart`, and `calendar` renderer metadata; typed chart axis/series and calendar start/end fields.
- `internal/compiler/compiler.go`: shape compatibility table and renderer-specific field/type/Action checks. A switch cannot alter query/filter/group/aggregate metadata.
- `internal/render/render.go`, `internal/page/page.go`, and `internal/httpapi/server.go`: emit normalized renderer and named-switch metadata.
- `web/src/App.tsx` and `web/src/style.css`: accessible card, bar-chart, grouped-table, metric, and calendar rendering plus a named Display switcher. Use semantic HTML/SVG and keyboard-readable labels; no third-party visualization runtime unless separately justified.
- `web/src/App.test.tsx`: loading, empty, null, error, overflow, responsive, keyboard, and switch-state tests.

### Dependencies and verification

- Depends on M2 result shapes.
- Focused commands:

```bash
go test ./internal/compiler ./internal/view ./internal/render ./internal/page ./internal/httpapi
cd web && bun run test -- App
cd e2e && bunx playwright test ats.spec.ts booking.spec.ts
```

- Acceptance: only compatible named Displays appear; switching preserves filter/search state and resets only an incompatible cursor; chart bars expose exact labels/values; metric null uses empty state; calendar event selection identifies a record without authorizing it.
- Non-goals: line/pie/scatter/area, arbitrary visual specs, utilization percentages, pixel-perfect reports, and query-changing Display switches.

## M4 — Page-level filters and dashboard composition

### Outcome and demo flow

The ATS recruiting overview has job, department, stage, and applied-date controls. One change updates every explicitly compatible candidate metric/chart/list/board while preserving recent activity and unrelated local controls. Asana proves page context and page filters cannot override each other in split YAML.

### Exact definition and example changes

- Modify `examples/ats/app.yaml`:
  - extend existing home Page/Panel and Blocks for `active_candidate_total`, `candidates_by_stage`, `candidates_by_job`, `candidates_by_department`, `recent_candidates`, `recent_activity`, and `candidate_pipeline`;
  - declare Page filters `job`, `department`, `stage`, `applied_from`, `applied_to` with explicit target Block/filter mappings;
  - leave `recent_activity` untargeted where a two-hop relation would be required.
- Modify `examples/asana/pages.yaml` and `examples/asana/tasks.yaml`:
  - add task count by status or priority for one existing project Page;
  - map an optional page control only to task Blocks while retaining the immutable route-bound project input;
  - keep split resources; do not merge YAML files.

### Runtime/compiler/UI changes

- `internal/appir/appir.go`: Page filter and target mapping contract.
- `internal/compiler/compiler.go`: resolve Block -> View -> exposed filter types, reject target/type/operator conflicts, duplicate URL names, unknown Blocks, and immutable binding collisions.
- `internal/page/page.go`, `internal/render/render.go`, `internal/httpapi/server.go`: compile and emit one page-filter context; server still recomputes bindings.
- `web/src/App.tsx`: page controls, canonical URL state, target-specific cursor reset, parallel block refresh, and partial-block state handling.
- `web/src/api.ts`: serialize only compiled page/filter names.

### Dependencies and verification

- Depends on M3 rendering and current v0.14 namespaced block state.
- Focused commands:

```bash
go test ./internal/compiler ./internal/page ./internal/render ./internal/httpapi ./internal/view
cd web && bun run test -- App
cd e2e && bunx playwright test ats.spec.ts asana.spec.ts
```

- Acceptance: every compatible ATS block returns the exact same filtered contribution set; an untargeted block does not refetch semantically; Asana route project binding cannot be replaced by query state; refresh/back/forward reproduce the dashboard.
- Non-goals: global application filters, arbitrary client joins, private filter presets, and implicit field-name fan-out.

## M5 — Typed drill-down

### Outcome and demo flow

Clicking ATS `interview`, a department, or active-candidate metric opens `candidate_records` with compiler-approved filters. Clicking CRM count/amount segments opens exactly the Policy-visible deals that contributed to that value.

### Exact definition and example changes

- Modify `examples/ats/app.yaml`: add drill metadata to `candidates_by_stage`, `candidates_by_department`, and `active_candidate_total`; target the named `candidate_records.table` route and forward compatible page inputs.
- Modify `examples/crm/app.yaml`: add chart/metric Displays and drill metadata from `deals_by_stage`, `pipeline_amount_by_stage`, and `open_pipeline_value` to `deal_records.table`.
- Extend `e2e/ats.spec.ts` and `e2e/crm.spec.ts` with exact expected IDs, not only visible counts/text.

### Runtime/compiler/UI changes

- `internal/appir/appir.go` and `internal/view/display.go`: typed drill target and source binding (`group`, `pageFilter`, `localFilter`).
- `internal/compiler/compiler.go`: prove source alias/type, target View/Display shape, exposed-filter operator, route, redaction, and Policy references.
- `internal/view/view.go`: retain normalized contribution predicates for comparison/test evidence; do not create a drill-specific query engine.
- `internal/httpapi/server.go`: resolve compiled navigation/cross-filter payload under the target View's own Policy.
- `web/src/App.tsx`: accessible chart/metric links, URL update/navigation, focus, loading, empty, and denied target behavior.

### Dependencies and verification

- Depends on M4 shared filter context and M2 contribution semantics.
- Focused commands:

```bash
go test ./internal/compiler ./internal/view ./internal/httpapi
cd web && bun run test -- App
cd e2e && bunx playwright test ats.spec.ts crm.spec.ts
make test-postgres
```

- Acceptance: for each fixture, aggregate contribution IDs equal drill result IDs for salesperson and manager; forged/unknown drill fields fail closed; hidden-only categories and different empty wording do not appear.
- Non-goals: arbitrary URL templates as query authority, cross-application drill, multi-hop drill, and required in-place cross-filtering.

## M6 — Record and bulk Actions

### Outcome and demo flow

From an ATS filtered table/board, select eligible candidates and run `move_candidate`; from CRM run permitted `move_deal`; in Commerce observe paid-unfulfilled orders, drill, run `advance_order`, and watch all related displays refresh.

### Exact definition and example changes

- Modify `examples/ats/app.yaml`: declare single/multiple selection and `move_candidate` on `candidate_records` table/board; keep `candidate_pipeline` and `candidate_is_named` authoritative.
- Modify `examples/crm/app.yaml`: expose `move_deal` on `deal_records` and existing pipeline with server-resolved actor/record availability.
- Modify `examples/commerce/app.yaml`:
  - add DemoSeed counts for products/orders/order items sufficient for several statuses and non-zero inventory;
  - add `low_inventory_product_records`, `low_inventory_product_count`, `orders_by_status`, `paid_unfulfilled_orders`, and `paid_unfulfilled_count`;
  - add `commerce_operations` Page/Panel/Blocks and record/chart/metric/board Displays;
  - expose existing `advance_order` only on compatible order records; preserve Rules, Extension, Action contract suites, and audit.
- Extend `e2e/ats.spec.ts`, `e2e/crm.spec.ts`, and `e2e/commerce.spec.ts` with success, denial, stale-version, and partial-failure cases.

### Runtime/compiler/UI changes

- `internal/appir/appir.go` and `internal/compiler/compiler.go`: Display selection/action references and same-Entity/input/Lifecycle validation.
- `internal/action` service files: expose read-only per-record capability evaluation by reusing Policy/Lifecycle resolution; Action execution itself remains unchanged.
- `internal/httpapi/server.go`: bounded availability and ordered batch endpoint/result over ordinary Action calls; no cross-record transaction.
- `web/src/action-client.ts`: formalize current sequential batch result as ordered successes/failures; retain shared encoder.
- `web/src/App.tsx`: row selection, Action inputs, confirmation, partial result, retry-failed, stale state, and public-View invalidation.
- `internal/testsuite`: add semantic result/audit assertions only where the existing TestSuite contract supports them.

### Dependencies and verification

- Depends on M5 exact record targeting.
- Focused commands:

```bash
go test ./internal/action ./internal/compiler ./internal/httpapi ./internal/view
cd web && bun run test -- App action-client
cd e2e && bunx playwright test ats.spec.ts crm.spec.ts commerce.spec.ts
```

- Acceptance: maximum 200 IDs; stable sequential order; Policy/Rule/Lifecycle/version checks per ID; successes remain committed across later failures; ordered structured result; audit once per success; retry only failures; ATS/CRM/Commerce metrics/chart/list/board refetch and agree.
- Non-goals: atomic bulk mutation, parallel execution, new `BulkAction`, arbitrary workflow scripts, order-value relation aggregation, and computed inventory-condition buckets.

## M7 — Agent and Studio authoring parity

### Outcome and demo flow

Explore and Studio author the common query/display/page/drill/action path as ordinary source. An agent fulfills the recruiting prompt using the same schemas, diagnostics, validation, inspection, diff, TestSuite, and publish contract. Asana proves edits remain reviewable across split source files.

### Exact definition and example changes

- Finalize the ATS definitions from M1–M6 and add/extend `candidate_move_contract` plus exact Explore TestSuites where current assertion vocabulary permits.
- Modify `examples/asana/app.yaml`, `examples/asana/pages.yaml`, and `examples/asana/tasks.yaml` only as needed to keep the M4 split-source dashboard slice manually editable.
- Add an agent fixture under the existing `internal/agenttest` layout containing the five required prompts and expected semantic artifact rubric, not a hard-coded YAML string oracle.

### Runtime/compiler/UI changes

- `internal/compiler/schema.go`, generated schemas, capabilities, inspect/references/diff paths, and `internal/agentprotocol`: expose every v0.15 field deterministically without adding an Explore-only Agent Protocol operation.
- `web/src/Studio.tsx` and `web/src/Studio.test.tsx`: edit the common search/group/aggregate/display/page-filter/drill/action path without Advanced JSON; retain Advanced JSON for uncommon combinations.
- `web/src/Explore.tsx`: save into a selected split resource or a deterministic default resource; show validation and semantic diff before publish.
- `internal/definition/source.go`: change only if inserting definitions into split resources cannot use the existing exact-source draft representation.

### Dependencies and verification

- Depends on all public semantics M1–M6.
- Focused commands:

```bash
go test ./internal/compiler ./internal/agentprotocol ./internal/agenttest
cd web && bun run test -- Explore Studio
cd e2e && bunx playwright test studio-builder.spec.ts ats.spec.ts asana.spec.ts
make test-blackbox
```

- Acceptance: generated definitions are inspectable, validated, diffable, testable, versionable, reproducible, and manually editable; rebuilding from identical definitions yields identical AppIR/checksum; no agent-only state or raw SQL appears.
- Non-goals: embedding an LLM, prompt storage, autonomous publication authority, arbitrary prose-to-SQL, and a new agent plane/operation.

## M8 — Qualification and five-minute onboarding

### Outcome and demo flow

A builder with existing Entities and deterministic data reaches a useful published operational dashboard in approximately five minutes. Tracker adds one independent issue-status chart-to-board proof so qualification is not ATS-only.

### Exact definition and example changes

- Modify `examples/tracker/app.yaml`: add issue count by status, a chart Display, record/table/board target, and `move_issue` after drill; reuse existing project/issue/comment Entities and DemoSeed.
- Keep ATS, CRM, Commerce, Asana, and Booking changes as maintained executable specifications.
- Do not alter Blog/CMS/Community/SaaS behavior except compatibility migrations mechanically required by the final AppIR/source format.
- Update `README.md`, `docs/architecture.md`, `docs/capabilities.md`, `docs/definitions.md`, `docs/security.md`, `docs/agent-protocol.md`, `docs/testing.md`, `docs/reference-apps.md`, `docs/progress.md`, `ROADMAP.md`, and version references with actual shipped behavior and evidence.

### Qualification and verification

- Record cold and warm existing-Entity flows: select Entity, configure Explore, save, validate, diff, test, publish, and open dashboard. Report elapsed time and human intervention; do not count pre-authored definitions as generation time.
- Run exact source validation for all ten applications and package/restart evidence for primary examples.
- Commands:

```bash
go test ./...
cd web && bun run lint
cd web && bun run typecheck
cd web && bun run test
cd e2e && bunx playwright test
make check
make test-crash
make test-postgres
make build
```

- Acceptance: all GOAL definition-of-done clauses pass; AppIR v1–v6 releases restart unchanged; no maintained example produces an empty required chart; the existing-Entity/DemoSeed path is measured near five minutes; final version and docs match runtime.
- Non-goals: CSV data import, existing PostgreSQL table mapping, provider introspection, external read federation, public sharing, reporting/subscription features, and presentation export.

## Working rules

- Add failing contract evidence before each public semantic and run the nearest test after each tracer bullet.
- Keep query semantics in View, visual encoding/interactions in Display, layout/filter fan-out in Page/Panel/Block, writes in Action, and authorization in Policy.
- Preview and named execution must share one canonical View plan and one `view.Service` path.
- Preserve the 200-row bound, parameterized DBAL pushdown, SQL backend confinement, immutable AppIR activation, and current source/AppIR compatibility policy.
- Extend examples before inventing abstractions. No example-specific branch may enter core.
- Keep `GOAL.md`, `PLANS.md`, `ROADMAP.md`, and `docs/progress.md` current as milestones move.

## Frozen decisions before M1

- Use administrator-only `/explore` backed by the existing Studio draft lifecycle.
- Insert v0.15 before v1.0 qualification.
- Ship a bar-only initial chart, sequential non-atomic batches, and the Booking calendar proof.
- No owner decision blocks implementation. The post-v0.15 choice between CSV Action-based import and v1.0 qualification remains outside this plan.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
