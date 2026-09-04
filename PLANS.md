# Workspace Menu/content layout plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze generic responsive composition and compatibility behavior | root goal and Menu/Panel design docs | done |
| 1 | Group a workspace Menu with following route content in the generic renderer | focused React tests | done |
| 2 | Implement desktop sidebar/content and mobile select/content geometry | CSS and Playwright measurements | done |
| 3 | Verify the existing Books demo with the rebuilt server | live visual inspection without data replacement | done |
| 4 | Qualify the repository | `make check` and `make build` | done |

## Working rules

- Compose only a `workspace` Menu with following siblings in the same render container; do not infer application names or IDs.
- Keep primary, secondary, tertiary, then content DOM order.
- Do not alter server Policy resolution, Menu hierarchy, routes, or Action persistence.
- Preserve ordinary rendering when content, tertiary navigation, or the workspace profile is absent.

# Bean Blog composition modernization plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze the metadata-only Display, width, responsive, and Policy-collapse scope | root goal and repository review | done |
| 1 | Move remaining taxonomy/comment presentation into named View Displays | Blog validation and existing browser journey | done |
| 2 | Compose Blog routes with semantic section widths and inline content | Blog validation and render-tree behavior | done |
| 3 | Prove responsive authorized discussion and anonymous Region collapse | focused Blog Playwright journey | done |
| 4 | Qualify the repository | `make check` and `make build` | done |

## Working rules

- Keep application behavior in `examples/blog` and use only existing generic capabilities.
- Preserve every route/context binding and all public/editor/member Policy semantics.
- Keep the article body `contained`, discussion `wide`, and moderation tables `wide`.
- Preserve main-before-sidebar source and accessibility order at every viewport width.
- Keep Policy-sensitive sidebar narrative in a named content Block; public local narrative may be inline.
- Named Displays own presentation; Blocks own selection, bindings, Policy, and composition.

# Dynamic hierarchical Menu plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze scoped instances, typed targets, placement lifecycle, bounds, and deferred scope | root goal and Menu idea | done |
| 1 | Compile hierarchical static targets and Entity navigation into AppIR v13 | AppIR/compiler/schema/inspect/diff tests | done |
| 2 | Add portable dynamic placement persistence and publication preflight | migration/SQLite/PostgreSQL/release tests | done |
| 3 | Execute typed placement create/update/delete atomically with Entity Actions | Action and idempotency tests | done |
| 4 | Resolve authorized Menu trees and active routes through Views | Menu/HTTP/Policy tests | done |
| 5 | Add generated record editing and responsive accessible navigation | React/Studio/browser tests | done |
| 6 | Add the maintained Book/Page acceptance slice and qualify the repository | package/restart/`make check`/`make build` | done |

## Working rules

- Keep static Page/View placements canonical in Menu definitions; contextual Studio controls edit the Menu draft.
- Scope dynamic Menu instances by owner record identity without persisting MenuInstance records.
- Keep record navigation destinations typed through same-Entity View page Displays; never persist expanded routes.
- Keep placement state outside AppIR and mutate it only within ordinary Action transactions.
- Reject parent deletion, cycles, depth overflow, unauthorized owners, duplicate targets, and unbounded trees.
- Render route navigation with `nav`/links/`aria-current`, never ARIA tab semantics.
- Preserve flat Menu source and AppIR compatibility.

# Asana Lite composition modernization plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze the metadata-only composition and Display migration scope | root goal and repository review | done |
| 1 | Move Asana presentation metadata into named View Displays | source validation and existing browser journey | done |
| 2 | Compose Asana Pages from responsive ordered Panel sections and inline content | source validation and focused browser assertions | done |
| 3 | Qualify the maintained reference application and repository | Asana Playwright journey, `make check`, `make build` | done |

## Working rules

- Keep Asana behavior under `examples/asana` and use only existing generic capabilities.
- Preserve route-context Block bindings and the project priority Page-filter fan-out across Panel sections.
- Keep board, tree, and attachment-list bands full width; use responsive two-column Panels only for naturally paired content.
- Preserve source and accessibility order at every viewport width.
- Named Displays own presentation; Blocks own selection, bindings, and composition.
- Inline content replaces only one-off narrative Blocks with no independent Policy or reuse requirement.

# Semantic Page section width plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze vocabulary, dimensions, ownership, defaults, and compatibility | root goal and Panel limitation note | done |
| 1 | Compile normalized section widths into AppIR v12 | AppIR/compiler/schema/inspect/diff tests | done |
| 2 | Render Page-owned Panels and Page chrome at deterministic widths | Page and React/CSS tests | done |
| 3 | Demonstrate contained and wide bands in tracker | source validation and Playwright geometry | done |
| 4 | Qualify the repository | `make check` and `make build` | done |

## Design decisions

- Add `sections[].width` with only `contained`, `wide`, and `full`; omitted values normalize to `wide`.
- Keep width on PageSection, not Panel, so reusable Panels and Sequence Panels do not inherit Page placement concerns.
- Define `contained` as `48rem`, `wide` as the existing `72rem`, and `full` as available viewport width. Every mode retains runtime-owned `1rem`/`1.5rem` safe gutters.
- Treat legacy `Page.panel` and AppIR v11 sections without width as `wide` at runtime. AppIR v12 stores normalized widths.
- Keep Page title, description, and filters aligned to `wide`; section width affects only Page-owned Panel nodes.
- Do not add full-bleed backgrounds, arbitrary values, responsive metadata, or order changes.

## Verification order

```bash
go test ./internal/appir ./internal/compiler ./internal/agentprotocol
go test ./internal/page ./internal/sequence
cd web && bun run test -- --run
cd e2e && bunx playwright test tracker.spec.ts
make check
make build
```

# Policy-aware collapsible Panel Region plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze opt-in empty, expansion, error, compatibility, and authorization semantics | root goal and Panel limitation note | done |
| 1 | Compile `collapseWhenEmpty` into AppIR v11 | AppIR/compiler/schema/diff tests | done |
| 2 | Omit authorized-empty Regions and expand a sole survivor | Panel/Page/Sequence and React/CSS tests | done |
| 3 | Document authoring and verify browser-visible full-track expansion | definitions/capabilities docs and Playwright | done |
| 4 | Qualify the repository | `make check` and `make build` | done |

## Design decisions

- `regions[].collapseWhenEmpty` is opt-in and defaults to false, preserving existing source and render trees.
- Empty means no server-authorized Block nodes after normal Block Policy evaluation. View result emptiness, loading, and errors are Block concerns and do not collapse the Region.
- Omit collapsed Regions. If one of multiple authored Regions survives, mark it expanded so runtime-owned CSS spans all Panel tracks. If none survive, the Panel is unavailable.
- Keep declared composition authoritative for compiler membership, inspection, generated checks, and bound HTTP requests; visual omission cannot grant or revoke Block access.
- Store the boolean in immutable AppIR v11 while preserving v10 Page sections and all earlier compatibility readers.

## Verification order

```bash
go test ./internal/appir ./internal/compiler ./internal/agentprotocol
go test ./internal/panel ./internal/page ./internal/sequence
cd web && bun run test -- --run
cd e2e && bunx playwright test tracker.spec.ts
make check
make build
```

# Ordered multi-Panel Page section plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze syntax, bounds, compatibility, policy, and binding semantics | root goal and design note | done |
| 1 | Compile ordered Page sections into versioned immutable AppIR | AppIR/compiler/schema/reference/diff tests | done |
| 2 | Render Policy-visible sections and authorize bound Blocks across them | Page/HTTP/generated-registration tests | done |
| 3 | Demonstrate multiple layout bands in tracker and document authoring | source validation, Studio/React/Playwright, docs | done |
| 4 | Qualify the repository | `make check` and `make build` | done |

## Design decisions

- Add `sections: [{panel: ...}]` as the ordered multi-band source form; keep legacy `panel` as-is and reject declaring both. Bound the new list to 1–32 entries.
- Store `PageSection` directly in immutable AppIR v10. A legacy Page keeps `Panel`; a sections Page keeps `Sections`, so compatibility input does not create noisy semantic rewrites.
- Resolve Page context once. Render each Panel in source order, omit Panels denied by their own Policy, and return unavailable only when no Panel is visible. The outer Page Policy remains unchanged.
- Compute Page filter membership over all declared Panels. Bound View/Webform requests must locate the named Block in at least one declared Panel whose Policy and Block Policy allow the request.
- Repeated Panel references are allowed because a reusable layout band may intentionally appear more than once; no nested composition or cycle graph is introduced.

## Verification order

```bash
go test ./internal/appir ./internal/compiler
go test ./internal/page ./internal/httpapi ./internal/generatedtest
bin/bean app validate --file examples/tracker/app.yaml --json
cd e2e && bunx playwright test tracker.spec.ts
make check
make build
```

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

# Inline semantic Panel content plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze ordered YAML, AppIR identity, compatibility, policy, and diagnostic contracts | root goal and design note | done |
| 1 | Compile and validate immutable ordered inline region items | AppIR/compiler/schema/diagnostic tests | done |
| 2 | Render and count mixed inline/named content through existing Panel/Sequence paths | Panel/Sequence/policy tests | done |
| 3 | Convert the presentation example and document canonical authoring | example validation and docs | done |
| 4 | Qualify the repository | `make check` and `make build` | done |

## Design decisions

- A region uses either unchanged `blocks: [name, ...]` or ordered `items`. Each ordered item has exactly one `block: name` or one non-empty `content: [...]`; optional `id` is valid only for inline content. Mixed content and references use `items`, whose list order is render order.
- Inline items lower during definition compilation into nested immutable AppIR region items. Their non-public identity is `@inline/<panel>/<region>/<id-or-ordinal>`; an explicit region-local `id` stabilizes identity across reordering, while an omitted ID deterministically uses the item ordinal.
- Generated identities never enter the global Block map and cannot be named in `block`/legacy `blocks` references. Rendering synthesizes the existing `type: content` Block representation from the compiled region item and dispatches it through the existing Block/content renderer.
- Legacy `regions[].blocks` remains represented and rendered unchanged. Declaring `blocks` and `items` together is rejected because it has no unambiguous interleaving order.
- Inline content has no policy field: Page/Sequence and Panel policy remain authoritative for it. Named referenced Blocks retain independent Block policy. Panel diagnostics use source-indexed paths (`spec.regions.<index>.items.<index>.content.<index>...`) and the shared content validator.

## Verification order

```bash
go test ./internal/appir ./internal/compiler
go test ./internal/panel ./internal/sequence ./internal/page ./internal/httpapi
bin/bean app validate --file examples/presentation/app.yaml --json
make check
make build
```

# Required Entity form marker plan

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 1 | Render a red `*` after labels for required Entity fields across typed controls | focused Admin React test | done |
| 2 | Preserve accessibility and qualify the repository | `make check` and `make build` | done |

## Working rules

- Derive the marker only from immutable Entity field metadata.
- Keep the marker visual-only so accessible field names and native required semantics remain unchanged.
- Do not alter validation, View reads, Action writes, or application metadata.

# Persisted-session CSRF continuity plan

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 1 | Reproduce authenticated Admin writes with missing tab-local CSRF state | focused React regression | done |
| 2 | Rehydrate the current session token without weakening server validation | Shell session synchronization and React test | done |
| 3 | Prove the real Blog Admin create journey after storage loss | Playwright Blog journey | done |
| 4 | Qualify the focused fix | `make check` and `make build` | done |

## Working rules

- Keep the database-backed HttpOnly session cookie and exact server-side CSRF comparison authoritative.
- Restore only the token returned for the currently authenticated session; do not retry or bypass failed mutations.
- Preserve login/logout cache clearing and all existing Action/Webform call paths.

# Bean v0.16 Semantic Sequences implementation plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its public flow and listed evidence pass.

| Milestone | User-visible deliverable | Example slice | Evidence | Status |
| --- | --- | --- | --- | --- |
| 0 | Freeze the generic Sequence boundary and Bean-introduction benchmark | `examples/presentation` contract | root goal, exact fixture/rubric, failing tests | done |
| 1 | Compile inspectable Sequence and semantic content metadata | minimal Sequence fixture | AppIR/schema/capabilities/inspect/diff/reference/compatibility tests | done |
| 2 | Receive deterministic layout, safety, and density repair diagnostics | broken Bean deck fixture | compiler diagnostic and repair-loop tests | done |
| 3 | Navigate an accessible, responsive, print-ready HTML sequence | three-frame browser fixture | render tree, React, accessibility, URL/keyboard/print tests | done |
| 4 | Present the ten-frame Bean introduction from ordinary definitions | complete Bean introduction | source, agent rubric, View/chart, E2E, package/restart tests | done |
| 5 | Ship the v0.16 compatibility/version/documentation cut | all maintained examples | terminal gates and clean diff | done |

## Dependency order

```text
M0 contract
  -> M1 canonical metadata/AppIR
     -> M2 deterministic validation
        -> M3 runtime rendering
           -> M4 complete reference vertical slice
              -> M5 qualification
```

## M0 — Contract and executable fixtures

- Archive the completed v0.15 goal as `docs/goals/015.md`.
- Define `Sequence` as an ordered route-level composition of existing Panels; do not introduce a parallel `Presentation`, `Slide`, `Report`, or rendering runtime.
- Define one initial `presentation` profile, `wide|standard` aspect ratios, closed frame layout vocabulary, content vocabulary, safety constraints, and deterministic resource budgets.
- Define the exact ten-frame Bean introduction rubric and a seeded invalid-to-valid repair case.
- Add failing focused tests before implementation.

Verification:

```bash
go test ./internal/compiler ./internal/appir ./internal/page
cd web && bun run test -- App
```

Non-goals: PDF/PPTX, embedded agents, WYSIWYG, warnings framework, arbitrary HTML/CSS/JS/SVG, or external asset tooling.

## M1 — AppIR v8 Sequence and semantic content

- Add `Sequence`, `SequenceFrame`, and `ContentElement` to AppIR v8.
- Add `content` as a generic Block capability that remains usable by ordinary Pages.
- Register Sequence with compiler storage, normalization, lookup, references, semantic diff, schema generation, and route matching.
- Expose sequence profiles/aspect ratios/layouts, content types/tones/directions, and all resource limits through capabilities.
- Extend AppIR compatibility so v1–v7 remain readable and reject v8-only fields.
- Keep the application definition lifecycle and release persistence unchanged.

Verification:

```bash
go test ./internal/appir ./internal/compiler ./internal/release ./internal/agentprotocol
```

## M2 — Stable validation and agent repair loop

- Validate canonical/unique routes and frame names, bounds, Panel references, layout/Panel compatibility, Block count, semantic content shape, image alt/source safety, diagram bounds, code lines, and weighted density.
- Use stable Bean diagnostic codes and exact source paths. Candidate lists should cover mistyped Panels and closed vocabulary values where the existing diagnostic machinery supports them.
- Prove identical broken input yields identical ordered diagnostics.
- Prove the reference agent fixture can change only diagnosed fields and reach a valid compiled AppIR without source-code knowledge.

Verification:

```bash
go test ./internal/compiler ./internal/agenttest
```

## M3 — Accessible HTML and print runtime

- Match Sequence routes alongside Page and View-display routes without ambiguity.
- Build one `Sequence` render node with a child Panel tree per Policy-visible frame.
- Render one active frame at a time with deep-linked machine-name state, buttons, picker, progress, arrow/Home/End keys, focus-safe semantics, notes toggle, and stable empty/error behavior.
- Add responsive 16:9/4:3 canvas sizing and print styles that render all frames, one per page, while hiding navigation and notes.
- Do not change ordinary Page/Panel/Block rendering semantics.

Verification:

```bash
go test ./internal/page ./internal/httpapi ./internal/block
cd web && bun run lint && bun run typecheck && bun run test
```

## M4 — Executable Bean introduction

Create `examples/presentation/` with exact files:

- `app.yaml`: manifest and explicit resource list;
- `content.yaml`: reusable semantic narrative Blocks;
- `data.yaml`: deterministic DemoSeed, `capability` Entity, grouped View and chart Display;
- `layout.yaml`: Panels and the `bean_introduction` Sequence.

The ten frames are:

1. Bean title and product statement.
2. Why probabilistic agents need a deterministic target.
3. Definition -> compiler -> AppIR -> runtime architecture.
4. Core definitions and ownership boundaries.
5. Agent validate/inspect/diff/test/publish loop.
6. Explore: model -> visualize -> drill -> act.
7. Shipped capability areas as a real grouped View/chart.
8. Safety, Policy, Lifecycle, Rule, Action, and Extension boundaries.
9. Reference applications, current status, and near roadmap.
10. Five-minute getting-started close.

Expected behavior:

- `/presentations/bean` opens frame 1 and reports `1 / 10`.
- keyboard/button/picker navigation reaches every frame and updates `?frame=`.
- a direct URL opens the requested frame; an unknown frame falls back to the first.
- speaker notes are hidden initially and visible only after the explicit toggle.
- frame 7 loads the grouped chart through the named View and has meaningful seeded groups.
- print mode has ten page-break frames and no interactive chrome.
- packaging removes source and retains the complete route and seeded chart.

Verification:

```bash
bin/bean app validate --file examples/presentation/app.yaml --json
cd e2e && bunx playwright test presentation.spec.ts package.spec.ts
go test ./internal/agenttest ./internal/release
```

## M5 — Qualification

- Update authoring, architecture, definition, capability, security, testing, reference-app, and agent guidance.
- Set `0.16.0-alpha`, regenerate JSON schemas and embedded frontend assets.
- Validate all examples and run package/restart evidence.
- Record the honest benchmark: deterministic prepared-definition validation/render time is not an external LLM generation benchmark.
- Pass terminal gates and leave a clean local commit on `v0.16-presentations`.

```bash
make check
make test-crash
make test-postgres
make build
```

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

# Bean v0.13 Typed Extension Boundary plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze Extension metadata, after-commit transaction boundary, HTTP wire contract, host auth, retry/idempotency/failure semantics, TestSuite mocks, and exclusions | root goal and reference-fixture plan | done |
| 1 | Canonical Extension Definition, AppIR format, schema, capabilities, inspect, references, diff, validation, and compatibility | compiler/schema/Agent Protocol/AppIR/release tests | done |
| 2 | Transactional Action extension intents and bounded HTTP delivery | Policy/rollback/audit/idempotency plus HTTP timeout/retry/auth/output tests | done |
| 3 | Offline typed provider mocks, interaction assertions, and metadata-only commerce slice | TestSuite runner and commerce source/restart/parity tests | done |
| 4 | Documentation, v0.13 version cut, terminal gates, CI, review, and squash merge | all gates, CI, Codex review, merged PR | done |

## Working rules

- Extension calls are typed after-commit intents; never hold a database transaction open across HTTP.
- Persist the intent with the Action transaction and reuse one invocation/idempotency identity for every at-least-once attempt.
- Keep credentials in host configuration and redact provider details; application metadata declares requirements, not secrets.
- Reuse the production intent/delivery path in Semantic TestSuites with an injected typed provider mock and no network.
- Add only the HTTP transport and commerce notification slice; defer provider SDKs, WASM, scripts, synchronous results, and infrastructure expansion.
- Keep Views for reads, Actions for writes, application behavior in metadata, and core packages generic.
- Add failing contract evidence before each public behavior and run its nearest tests.
- Keep `GOAL.md`, `PLANS.md`, `ROADMAP.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```

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

# Bean v0.11 First-class Semantic Test Suites plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze TestSuite targets, fixtures, context, assertions, isolation, bounds, and compatibility | root goal plus compiler/runner fixture plan | done |
| 1 | Canonical TestSuite Definition, AppIR, schema, capabilities, inspect, references, diff, and compatibility | compiler/schema/Agent Protocol/AppIR tests | done |
| 2 | Deterministic isolated Rule suite runner through the production evaluator | result/error/context/replay/bound tests | done |
| 3 | Deterministic isolated Action suite runner through the production Action service | Policy/guard/derive/invariant/mutation/event/rollback tests | done |
| 4 | Existing `app test` machine contract plus maintained metadata-only reference suites | CLI/Agent Protocol and commerce/ATS/booking defect tests | done |
| 5 | Documentation, v0.11 version cut, terminal gates, CI, clean review, and merge | all gates, CI, Codex review, merged PR | done |

## Working rules

- TestSuite is metadata and immutable AppIR, not repository-only scripting or a separate interpreter.
- Target only Rule and Action in v0.11; exercise Policy and Lifecycle through Actions.
- Use explicit context, fixed time, deterministic IDs/seed, bounded data, and fresh per-case state.
- Reuse the production Rule evaluator and Action service; assertions observe results, records, and outbox evidence.
- Keep provider mocks, generated tests, standalone commands, and PostgreSQL suite execution out of v0.11.
- Add failing contract evidence before each public behavior and run the nearest test after every milestone.
- Keep `GOAL.md`, `PLANS.md`, `ROADMAP.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```

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

# Bean v0.9 Semantic Application Model plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Catalogue existing transition use, freeze the minimal Lifecycle and Action-binding contract, and define compatibility behavior | ATS/commerce fixtures plus schema and diagnostic test plan | done |
| 1 | Canonical Lifecycle schema, compiler validation, immutable AppIR, capabilities, inspect, and semantic diff | focused schema/compiler/AppIR/Agent Protocol tests | done |
| 2 | Policy-aware Lifecycle enforcement through Actions with safe publication and restart behavior | positive/negative Action, release, and crash contracts | done |
| 3 | Convert ATS candidate and commerce order flows to the shared primitive | source validation plus independent application journeys | done |
| 4 | Preserve legacy transition compatibility and SQLite/PostgreSQL parity | compatibility, CLI/MCP parity, backend, and bypass-refusal tests | done |
| 5 | Documentation, version cut, terminal gates, CI, and clean reviewed PR | all gates, CI, and Codex review | done |

## Working rules

- Add only Lifecycle in v0.9; later semantic candidates need their own evidence.
- Freeze the source and Action-binding contract before runtime implementation.
- Keep transition authorization in Policies and transition mutation in Actions.
- Normalize semantics once in the compiler; CLI and MCP consume shared Agent Protocol results.
- Maintain the declared legacy Action transition representation until compatibility tests prove the migration path.
- Add failing contract evidence before each public behavior and run the nearest test after every milestone.
- Keep `GOAL.md`, `ROADMAP.md`, `PLANS.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```

# Bean v0.8 Agent Protocol plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze protocol operations, planes, transport, authorization, and compatibility contracts | root goal plus registry/authorization test fixtures | done |
| 1 | Shared provider-neutral dispatcher for Definition, Release, and Application Planes | focused handler tests across all ten operations | done |
| 2 | Existing command delegation plus generic CLI protocol transport | CLI compatibility and plane allow/deny contract suite | done |
| 3 | MCP 2026-07-28 stdio adapter with maintained legacy initialization compatibility | framing/discovery/list/call/error/EOF tests | done |
| 4 | Cross-transport parity, runtime Policy boundaries, and backend qualification | CLI/MCP parity plus SQLite/PostgreSQL View/Action tests | done |
| 5 | Provider-neutral agent guidance, documentation, terminal gates, and clean reviewed PR | shipped `agents/`, docs, all gates, CI, and Codex review | done |

## Working rules

- The dispatcher delegates to compiler, release, View, and Action services; transports contain framing and presentation only.
- Plane grants are host configuration and are checked before source or database access.
- MCP tool arguments never grant roles, tenants, planes, raw tables, arbitrary writes, SQL, or shell access.
- Preserve the v0.6 CLI envelope and commands while making their structured results originate from shared handlers.
- Support MCP stdio only in v0.8; do not add remote transport or identity infrastructure.
- Add failing contract evidence before each public operation and run the nearest test after each milestone.
- Keep `GOAL.md`, `ROADMAP.md`, `PLANS.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```

# Bean v0.7 Demo Factory plan (completed)

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze Demo Factory source, pattern, seed, theme, package, and benchmark contracts | root goal and frozen ATS/CRM/tracker protocol | done |
| 1 | Typed Theme plus generic Metric, Timeline, and public Search presentation | schema/capability/compiler, HTTP, React, and ATS browser tests | done |
| 2 | Deterministic relation-aware fixture generator and `bean demo seed` | scalar/relation/cycle/replay/refusal tests and populated ATS evidence | done |
| 3 | Inspectable catalog of ordinary-definition application patterns | catalog stability tests and independent compilation of every pattern | done |
| 4 | Atomic, checksummed SQLite `bean package` output | restart, source-independence, tamper, failure-atomicity, and packaged-browser tests | done |
| 5 | ATS/CRM/tracker prompt-suite qualification, documentation, and version cut | terminal gates plus documented benchmark qualification boundary | done |

## Working rules

- Patterns expose ordinary definitions and always pass through schema and compiler validation; they never become hidden runtime macros.
- Seed writes use Actions, verification reads use Views, and generated data never bypasses Policy or storage contracts.
- Theme values come from closed compiler-known vocabularies; do not accept CSS or arbitrary frontend tokens.
- Dashboard composes Page/Panel/Block; add only the missing Metric, Timeline, and Search presentation behavior.
- Package only the current executable plus an activated SQLite database and manifest; do not add cloud, container, or installer machinery.
- Treat JSON envelopes, diagnostics, ordering, checksums, seed output, and package manifests as machine contracts.
- Add failing evidence before each public contract and run the nearest test after every milestone.
- Keep `GOAL.md`, `ROADMAP.md`, and `docs/progress.md` current as milestones move.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```

# Completed Asana Lite local application plan

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Goal, contracts, and metadata design | focused compiler/field tests and source validation | done |
| 1 | Transactional generic file field and multipart Webform path | field, Action, HTTP, and React tests | done |
| 2 | Generic board and arbitrary-depth tree presentations | compiler and React tests | done |
| 3 | Metadata-only anonymous Asana Lite application | source validation and browser journey | done |
| 4 | Documentation and repository qualification | `make check` and `make build` | done |

## Working rules

- Keep project/task/attachment behavior under `examples/asana`; core additions must compile and render any compatible metadata.
- Preserve View reads, Action writes, immutable route bindings, and atomic Action/audit/blob persistence.
- Never use a client filename as a path or expose file bytes in AppIR, manifests, logs, audit, or Action results.
- Board and tree field references are compiler-validated; state changes use declared Actions.
- Run the nearest focused test after every milestone and keep this plan plus `docs/progress.md` current.

# Reviewable application sources plan (completed)

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Manifest/resource grammar and source-aware loader | definition loader tests | done |
| 1 | CLI, embedded demo, and fixture integration | CLI and affected package tests | done |
| 2 | Feature-oriented example migration and authoring docs | all example validation and documentation review | done |
| 3 | Repository qualification | `make check` and `make build` | done |

## Working rules

- Optimize the source format for authors and reviewers; do not preserve the unused bundle syntax.
- Keep resource inclusion explicit, local, and non-overriding.
- Preserve source locations through compiler diagnostics.
- Flatten sources into the canonical Bundle before the existing release lifecycle.

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

## Completed scoped-resource-list plan

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Typed `resource-list` definition and security contract | compiler and HTTP tests | done |
| 1 | Generic table/filter/action renderer and blog metadata route | React and Playwright tests | done |
| 2 | Backend parity, docs, and qualification | terminal gates | done |

## Completed v0.5 plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Role/policy, content, binding, sensitive-input, and browser contracts | compiler/HTTP/policy/browser tests | done |
| 1 | Metadata-only editorial blog with draft/publish, categories, tags, and public Views | Action/View contracts and hidden-draft browser test | done |
| 2 | Opt-in password signup, login/logout, fixed member role, and safe public auth UI | auth, HTTP, React, escalation, rate-limit, and CSRF tests | done |
| 3 | Route-bound comment submission and editor approval/rejection | binding-tamper, policy, audit, and browser moderation tests | done |
| 4 | Generic list/detail rendering, safe rich text, navigation, pagination, and RSS | React, XSS, responsive, and public-route browser tests | done |
| 5 | SQLite/PostgreSQL parity, regression qualification, docs, and v0.5 cut | all terminal gates | done |

## Working rules

- Build only generic primitives; keep all blog-specific behavior in `examples/blog` metadata.
- Add failing evidence before changing auth, policy, Action, View, or render boundaries.
- Self-registration grants only a compiler-fixed `member` role; editor/admin promotion remains protected System Admin behavior.
- Draft posts and pending/rejected comments must be impossible to retrieve publicly, not merely hidden in React.
- Route-bound values are server-validated and cannot be overridden by submitted form data.
- Passwords are sensitive inputs and never appear in AppIR output, logs, audit data, manifests, or idempotency results.
- Preserve definition → validation → migration → immutable AppIR → atomic activation.
- Preserve View reads and Action writes on public, Admin, SQLite, and PostgreSQL paths.
- Run the nearest focused test after each milestone and keep `docs/capabilities.md` and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-blog
make test-postgres
make build
```
