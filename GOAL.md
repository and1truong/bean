# Goal: Bean v0.15 Explore

Status: complete

Bean Explore turns an existing Bean data model into an operational application: explore, visualize, drill down, and act. It is a first-party authoring and runtime experience composed from Bean primitives, plus only the core semantics that the repository audit proves are missing.

> **Bean Explore turns any Bean data model into an operational application: explore, visualize, drill down, and act.**

The intended loop is:

```text
Model or connect data
  -> explore through typed View semantics
  -> save ordinary View and Display definitions
  -> compose them with Page, Panel, and Block
  -> drill into the contributing records
  -> execute Policy-checked Actions
  -> refresh the same Views
```

This goal starts only after v0.14 View Displays and must not reopen or destabilize that completed contract. The repository findings below record the pre-implementation baseline; completion is measured by the executable definition-of-done and terminal gates in this document.

## Product thesis

Metabase is a useful reference for the exploration loop, not a product template. It helps users connect data, explore it, save questions, build dashboards, drill down, and share. Bean's useful difference is that an observation can lead directly to an authorized, inspectable application Action.

Bean remains:

- a deterministic application runtime and execution target for agents;
- the owner of application semantics, while providers own external infrastructure;
- compiled from inspectable definitions through validation, migration, immutable AppIR, and atomic activation;
- based on View reads and Action writes, with Policy enforced by the runtime;
- free of opaque scripts, arbitrary SQL, hidden agent state, and application-specific core branches.

## Problem and target users

The current primitives can describe useful application screens, but creating an operational dashboard requires hand-authoring several independent definitions. Grouped aggregate Views exist but are not demonstrated by a runnable application. The UI cannot build or preview a typed query, switch compatible Displays, coordinate page filters, drill from an aggregate to its contributing records, or execute ordinary record Actions from most public View results.

The initial users are:

- application builders and agents turning an Entity model into saved operational Views and dashboards;
- operational managers observing a Policy-scoped process and drilling into exceptions;
- frontline operators acting on records exposed by those dashboards.

The job to be done is:

> Given an existing Bean Entity model, produce a useful, editable, Policy-safe operational application in minutes, then move from a signal to the exact contributing records and an authorized Action without leaving Bean semantics.

## Repository findings and product boundary

### What v0.14 already provides

- `View` is the canonical public read and presentation definition. Query fields remain at View scope and named Displays cover page, block, JSON, CSV, and RSS output.
- Typed exposed filters support `eq`, `contains`, `gte`, and `lte`; controls own labels, widgets, and defaults.
- Projection, one-hop declared relationships, fixed/context filters, sorting, grouping, `count`/`sum`/`min`/`max`/`avg`, cursor/offset bounds, and Policy-aware execution already exist.
- UI renderers are `list`, `detail`, `table`, `board`, `tree`, `metric`, and `timeline`.
- Page, Panel, and Block already own composition; Blocks bind immutable route/query context to Views.
- Application Admin already demonstrates search, filtering, sorting, row selection, sequential per-record batch Actions, audit history, and cursor paging.
- Every Entity receives a generated `<entity>_list` View and CRUD Actions. The generated View is a credible starting point for universal exploration.
- Studio drafts, validation, semantic diff, publish, immutable AppIR, Agent Protocol inspection, named View query, named Action execution, TestSuite, DemoSeed, SQLite, and PostgreSQL are established.
- View execution adds Policy, owner, tenant, and soft-delete predicates before the DBAL applies grouping and aggregation.

### Implemented but not demonstrated end to end

- No maintained example renders a grouped aggregate View.
- Relation grouping and all aggregate functions beyond `count` appear only in focused Go tests.
- No example proves that Policy is applied before an aggregate or that aggregate drill-down returns the exact contribution set.
- Public Displays do not demonstrate the Admin's reusable selection and batch Action path.
- Search and aggregation are present in separate paths but are not available in one universal Explore workflow.

### Semantic gaps

- Searchable fields are currently renderer metadata even though search changes the View query.
- The compiler has no explicit result-shape contract connecting a View to compatible renderers.
- Aggregate validation does not yet freeze numeric types, nulls, money, date buckets, relation cardinality, overflow, or deterministic group ordering across backends.
- A grouped View cannot use the metric renderer; chart, cards, and calendar renderers do not exist.
- Displays cannot declare typed drill-down, selection, or ordinary record Actions.
- Pages have immutable context bindings but no first-class page filter mapped to compatible Blocks.
- Public application UI has no universal Entity explorer, display switcher, or save-to-draft flow.
- Client-side Action metadata does not prove per-record availability under current Policy and Lifecycle state.
- CRM demo data does not provide multiple owners; Commerce has no DemoSeed; consequently security and observe-to-act dashboards cannot rely on current demo distributions.
- CSV data import, existing-table mapping, provider schema introspection, and external read providers do not exist.

### Chosen boundary

Bean Explore is a small number of missing core capabilities plus a first-party module:

1. a built-in Explore authoring surface over Entities and typed View plans;
2. compiler-known query, result-shape, interaction, and action-availability additions used by all clients;
3. ordinary View, Display, Page, Panel, Block, Action, Policy, and Lifecycle definitions as the saved output.

“Module” is a product surface, not a new Definition kind or plugin system. Explore does not introduce `Question`, `Report`, `Dataset`, `Visualization`, a second query engine, or a parallel persistence model.

## Architecture ownership

| Responsibility | Owner | Decision |
| --- | --- | --- |
| Typed read semantics | View | Projection, relationships, search, filters, sort, pagination, grouping, aggregation, and result bounds compile once into a canonical View plan. |
| Presentation | Display | Renderer, labels, columns, visual encoding, compatible switching, empty state, and typed interaction declarations. A Display never changes its View query shape. |
| Composition | Page / Panel / Block | Layout, block placement, immutable context, and page-filter fan-out to declared compatible View inputs. |
| Mutation and effects | Action | Record and bulk UI invokes existing Actions; Lifecycle, Rule, audit, idempotency, and Extension behavior remain unchanged. |
| Authorization | Policy | View rows are authorized before grouping; fields remain redacted; Actions are checked per record. |
| Workflow state | Lifecycle | Current state and permitted transitions are server-resolved and never inferred by a chart or client component. |
| Authoring coordination | Explore | Builds an ephemeral candidate View plan for preview and saves ordinary definitions into the existing draft lifecycle. It owns no application data or hidden artifacts. |

The one current mapping that must change is renderer-owned `searchFields`: search affects returned rows, so new source declares searchable fields on the View. Legacy renderer metadata normalizes deterministically for compatibility.

## Typed query contract

### One plan, two sources

A compiled View and an unsaved Explore preview both produce the same internal canonical View plan. The preview endpoint accepts a bounded candidate View specification, resolves it against the active Entity model, runs the same compiler/type checks, and executes it through `view.Service` under the current actor. It is not persisted and cannot name SQL, tables, Policies, actors, tenants, or providers.

Saving materializes an ordinary View definition in the current Studio draft. The user or agent then uses the existing validate -> inspect/diff -> test -> publish lifecycle. Published application reads remain named-View reads.

### Definition structure

The current View fields remain source-compatible. v0.15 adds View-owned search and typed group entries:

```yaml
kind: View
metadata: {name: candidates_by_stage}
spec:
  entity: candidate
  relationships:
    - {name: job, relationField: job_id, type: inner}
  fields: [stage]
  search: {fields: [name, email, summary]}
  exposedFilters:
    job: {field: job_id, operator: eq}
    department: {field: job.department, operator: eq}
    stage: {field: stage, operator: eq}
    applied_from: {field: applied_at, operator: gte}
    applied_to: {field: applied_at, operator: lte}
  groupBy:
    - {field: stage, as: stage}
  aggregates:
    - {function: count, field: id, alias: candidate_count}
  sort: [{field: candidate_count, desc: true}]
  defaultLimit: 50
  maxLimit: 200
```

Legacy string `groupBy` entries normalize to `{field: <name>, as: <name>}`. A date/datetime group may additionally declare one closed bucket: `day`, `week`, or `month`. Weeks start Monday and all datetime buckets use UTC in v0.15. The compiler emits the resolved group fields, aliases, types, and buckets in AppIR.

### Validation and type checking

- Projection, search, filter, sort, group, aggregate, display, drill, and action references must resolve at compile time and must not use sensitive or redacted fields.
- Public search is case-insensitive text containment over an explicit non-empty list of selected textual fields. It is a View input; a Display only chooses whether to expose its control and label.
- The closed exposed-filter vocabulary remains `eq`, `contains`, `gte`, and `lte`. Page and drill filters reuse these declared inputs; they cannot introduce a field or operator.
- `count(field)` accepts any field and counts non-null values; `count(id)` is the canonical visible base-record count. `sum` and `avg` accept integer, decimal, or money. `min` and `max` accept ordered scalar types.
- `sum`, `min`, and `max` of money preserve the money field type and its integer minor-unit representation. `avg(money)` is deferred because Bean has no explicit fractional-money/currency contract. Currency conversion and mixed-currency aggregation are out of scope.
- `count` over an empty ungrouped input is `0`. Other ungrouped aggregates over no non-null input are `null`; metric Displays render their declared empty state rather than inventing zero.
- Null group keys form one typed null group displayed as `Unknown` unless the Display supplies another static label. Aggregates other than `count(id)` ignore null inputs.
- A View may traverse only one explicitly declared relationship hop. Aggregate/group/filter traversal is initially limited to to-one relationships so joining cannot multiply base records. To-many aggregate traversal and distinct aggregation are deferred.
- Computed read values are accepted only if a later milestone defines a compiler-known, side-effect-free View projection using existing Rule evaluation semantics. v0.15 examples do not depend on it.

### Ordering, limits, and pagination

- Record results keep opaque cursor paging and the existing offset API compatibility. The compiler appends a stable base-record ID tie-breaker.
- Group results are not cursor-paged. They require explicit ordering or receive deterministic group-key ascending order, including a backend-independent null position.
- The hard record and group result limit remains 200. Record results can page within that bound per request.
- A complete aggregate/chart request probes one row beyond its declared limit. If more groups exist, it returns a typed `result_limit_exceeded` error instead of silently presenting a partial chart as complete.
- Numeric/full pagination, total result counts, arbitrary-page access, and unbounded export remain outside v0.15.

### Authorization and pushdown order

The semantic order is mandatory on SQLite and PostgreSQL:

```text
resolve actor and immutable bindings
  -> validate/coerce declared search and filters
  -> add Policy + owner + tenant + soft-delete predicates
  -> apply permitted to-one relationships
  -> filter contributing base rows
  -> group and aggregate
  -> order
  -> enforce bounds
  -> redact/materialize the declared result shape
```

The DBAL may push down every equivalent operation, but pushdown cannot change this order. Hidden rows never contribute to counts, sums, group existence, drill targets, or action availability. A category with no visible contributing rows is omitted. Unauthorized and absent aggregate targets are not distinguishable through counts or empty-state wording.

## Result shapes and Display compatibility

The compiler derives one closed result shape for each View:

| Shape | Query form | Compatible v0.15 Displays |
| --- | --- | --- |
| `records` | selected fields, no aggregates | table, list, cards, board, timeline, calendar |
| `detail` | records with a required unique binding and at most one row | detail |
| `metric` | no group and exactly one selected aggregate | metric |
| `groups` | one or more group keys plus aggregate(s) | table, chart |

Renderer-specific contracts further narrow compatibility: board needs a selected enum group field and compatible transition Action; timeline/calendar need selected date/datetime fields; chart initially supports one categorical or date-bucket axis and one numeric aggregate series.

Switching Displays means selecting another named Display of the same View and result shape. A Display never rewrites a record View into a grouped query. Dashboards use multiple Views when they need record, metric, and grouped representations of the same Entity, and page filters keep those Views aligned.

Initial chart rendering is a compiler-known accessible bar chart. Line, area, pie, scatter, pivot, arbitrary Vega/JavaScript, and runtime renderer plugins are deferred. Calendar is a record Display with a required start/date field and optional end field; utilization math is not implied by calendar rendering.

## Interaction semantics

### Saved Views and dashboards

- Explore starts from an Entity's generated `<entity>_list` View contract without mutating the generated definition.
- “Save as View” creates a named ordinary View and its named Displays in the Studio draft. Name conflicts require an explicit replace choice; no silent overwrite.
- “Add to dashboard” creates or updates ordinary Block/Panel/Page definitions in the same draft.
- There are no per-user saved questions, presets, private dashboards, or database-only artifacts in v0.15.

### Local and page filters

Local Display controls remain URL-addressable and block-namespaced. A Page gains compiler-known filter declarations that map one typed page input to explicit target Block/View exposed-filter names. All targets must agree on input type and operator. A filter can affect only listed compatible Blocks; it cannot override an immutable route/context binding.

Changing a page filter resets affected cursors, preserves unrelated local state, and refreshes all target blocks from one canonical URL state. The frontend serializes values but does not decide field mapping, coercion, or authorization.

### Typed drill-down and cross-filtering

A metric or chart Display may declare a drill target consisting of a target View/Display and typed bindings from current page filters and result values. The compiler proves that each source type matches a declared exposed filter on the target.

- A chart group supplies its group key, for example `stage = interview`.
- A metric supplies only the current local/page filter context.
- A relation group supplies the related scalar value, for example `job.department = Engineering`.
- The server resolves the compiled interaction. URL parameters are a transport representation of that interaction, not query authority.
- The target executes independently through its own View and Policy. Its records must equal the aggregate's visible contribution set for the forwarded predicates.

Cross-filtering within a Page uses the same typed mapping but updates page-filter state instead of navigating. Initial acceptance requires navigation drill-down; cross-filter-in-place may land only after the same contract is proven.

### Selection and Actions

A records Display may declare single or multiple selection and references to Actions on the same Entity. The compiler verifies selected `id`, Action entity, required user inputs, Lifecycle binding, and all metadata references. The server provides per-record Action/Lifecycle availability; the client does not infer authorization from visible buttons or manifest transitions.

Bulk execution reuses ordinary Actions rather than introducing `BulkAction`:

- maximum 200 selected IDs, processed in stable selection order;
- one Action transaction per record, sequentially in v0.15;
- Policy, Rule, Lifecycle, validation, and optimistic version checks run independently for every record;
- successful records remain committed if a later record fails;
- the result reports ordered `succeeded` IDs and structured failures by ID/code/message/field;
- retries target failed records only; no rollback is implied across the batch;
- after completion, all affected public View queries are invalidated and refetched while filters remain; an empty current cursor returns to the nearest valid state.

Atomic multi-record Actions and parallel batch execution are deferred.

### UI states

Every Explore and published Display must define loading, empty, error, stale-refresh, and unauthorized behavior. Stale data remains visible with an updating indicator, Actions are disabled while their result is unknown, and an Action result reports partial success. A Policy-denied page uses the existing not-found behavior; a denied dashboard block is “Unavailable” and exposes no count, category, label, or reason tied to hidden records.

## Primary executable specification: Applicant Tracker

`examples/ats/app.yaml` is the end-to-end application.

### Definition changes

- Keep Entities `job`, `candidate`, `note`, and `activity`, Lifecycle `candidate_pipeline`, Action `move_candidate`, Rule `candidate_is_named`, and existing audit behavior.
- Extend the candidate record View with declared `job`, `department`, `stage`, `applied_from`, and `applied_to` inputs and named table/list/cards/board Displays.
- Replace the ambiguous `candidate_total` label with `active_candidate_total`, fixed to stages other than `hired` and `rejected`.
- Add grouped Views `candidates_by_stage`, `candidates_by_job`, and `candidates_by_department`; add `recent_candidates`; reuse `recent_activity` and the existing pipeline View.
- Add chart/metric/table drill declarations targeting the candidate record Display.
- Extend the existing home Page/Panel with Blocks for active total, stage chart, job/department summaries, recent candidates, recent activity, and pipeline board.
- Add page filters for job, department, stage, `applied_from`, and `applied_to`, each mapped only to compatible candidate Blocks. Recent activity is not targeted because reaching its job requires unsupported two-hop traversal.
- Enable `move_candidate` on the filtered record list and board. Single and bulk execution must enforce the existing Lifecycle and Rule.

### Data and behavior

The existing seed counts and cyclic enums/relations provide multiple jobs, departments, stages, named candidates, non-zero aggregates, and applicable transitions. Add exact fixture assertions for counts and drill sets; do not rely only on qualitative generated labels. Dates use the documented fixed demo clock so “recent” is deterministic.

Expected journey:

1. Open the recruiting overview and see non-empty, mutually consistent metrics, charts, recent lists, activity, and board.
2. Filter by job, department, stage, or applied range; every compatible block refreshes to the same contribution set.
3. Select `interview` in the stage chart and reach the candidate list with `stage = interview` through compiled drill metadata.
4. Drill from active total and see exactly its contributing active candidates.
5. Move one or more eligible candidates; named-candidate Rule and Lifecycle are enforced per record, audit entries are preserved, and the metric/chart/list/board refresh.

The maintained agent benchmark prompt is:

> Create a recruiting overview with candidate totals, candidates by stage, recent activity and a pipeline board. Add filters for job, department and applied date.

Its output must be ordinary, manually editable definitions and pass schema, validation, inspect, diff, TestSuite, publication, restart, and browser evidence.

## Security executable specification: CRM

Extend `examples/crm/app.yaml` with `deal_records`, `deals_by_stage`, `pipeline_amount_by_stage`, and `open_pipeline_value` Views; table, chart, metric, and board Displays; a `crm_operations` Page/Panel/Blocks; and typed drills back to `deal_records`. Reuse `company`, `contact`, `deal`, `activity`, `owned_records`, salesperson/manager roles, the existing `deal_pipeline` View, and `move_deal`.

The Page access Policy admits salesperson and manager roles; each View still uses `owned_records`. Add explicit test fixtures with at least two salespeople, disjoint deals, different statuses/amounts, and a manager. DemoSeed alone is insufficient because it assigns one owner.

Automated acceptance must prove:

1. a salesperson's count and amount include only that actor's visible deals;
2. each drill target returns exactly the IDs contributing to its metric or chart group;
3. a manager sees the broader permitted set and corresponding totals;
4. counts, sums, empty states, and chart categories reveal no hidden group or record;
5. `move_deal` availability and execution reflect the current actor, record Policy, and its existing Action-local transition graph;
6. SQLite and PostgreSQL apply authorization before grouping and aggregation.

## Observe-to-act executable specification: Commerce

Extend `examples/commerce/app.yaml` with deterministic DemoSeed data and:

- `low_inventory_product_records` and `low_inventory_product_count` using a documented fixed inventory threshold;
- `orders_by_status`, `paid_unfulfilled_orders`, and `paid_unfulfilled_count`;
- record, chart, metric, and pipeline Displays plus a `commerce_operations` Page/Panel/Blocks;
- typed drill from every metric/chart to its contributing records;
- authorized `advance_order` from the paid-unfulfilled result.

The journey is:

```text
observe paid-but-unfulfilled count
  -> drill into the exact paid orders
  -> run advance_order on an eligible order
  -> observe its Lifecycle change, audit, and dashboard refresh
```

Product inventory conditions expressed as computed buckets and order value through order items are deferred until Bean has a justified computed-read and to-many aggregation contract. v0.15 must not add complex relation aggregation merely to decorate a dashboard.

## Example Evolution Matrix

| Example | Existing capability | Proposed extension | Semantics proven | Milestone | Automated verification |
| --- | --- | --- | --- | --- | --- |
| ATS | Lifecycle, guarded move Action, list/table/board/metric/timeline Displays, Page, DemoSeed | Recruiting overview, typed aggregate Views, page filters, chart/metric drill, selected Actions | Complete explore-to-act loop and agent-authored ordinary definitions | M1–M7 | compiler/View/HTTP/React tests; `cd e2e && bunx playwright test ats.spec.ts`; package/restart journey |
| CRM | Owner Policy, manager bypass, Action-local deal transitions, money | Policy-scoped count/sum dashboard and exact drill sets | Authorization before aggregate and per-record Action availability | M2, M5, M6 | SQLite/PostgreSQL View contracts and CRM E2E with explicit actors |
| Commerce | Inventory, money, order Lifecycle, Rules, Extension, Action suites | Low-inventory and paid-unfulfilled dashboards with `advance_order` | Observe -> inspect -> act -> refresh | M2, M5, M6 | semantic suites, HTTP/React tests, `cd e2e && bunx playwright test commerce.spec.ts` |
| Tracker | Issue Lifecycle/Action and deterministic seed | Status/project/assignee chart-to-board slice | Reuse outside ATS; direct relation filtering | M8 candidate | source validation and tracker browser journey |
| Asana | Split YAML, context-bound Pages, board/tree/detail, Actions | Project page filter and task status/priority counts | Split-source authoring and immutable context/page-filter separation | M4, M7 | loader/compiler tests and Asana browser journey |
| Booking | Interval Rule, calendar-shaped View, cancel Action | Calendar Display and date/resource drill | Record calendar compatibility and authorized cancellation | M3 | compiler/React/booking E2E |
| Blog | Named public Displays, relations, content, serializers | Regression fixture only | v0.14 compatibility and relation/display non-regression | all gates | blog SQLite/PostgreSQL/package journeys |
| CMS | JSON/CSV/RSS, AdminResource, Page | Regression fixture only | Serializer and generated Admin compatibility | all gates | source/schema and existing tests |
| Community | Owner/public Policies and social Actions | Deferred owner-scoped aggregate proof | Policy-safe grouping in another domain | post-v0.15 | future explicit multi-actor fixture |
| SaaS | Tenant Policy and tenant records | Deferred tenant-scoped aggregate dashboard | Tenant predicate before aggregate | post-v0.15 | future multi-tenant backend parity test |

No new example is required. ATS, CRM, Commerce, Asana, and optionally Booking cover the needed semantics more honestly than a new showcase.

## Agent integration

Human and agent authors use the same schema and definitions. The existing Definition/Release/Application planes remain sufficient: an agent edits a draft/source bundle, requests schemas/capabilities, validates, inspects, diffs, tests, and publishes; application reads and writes remain named View and Action calls.

Required prompt fixtures include:

- “Show candidates grouped by stage.”
- “Create a recruiting operations dashboard.”
- “Show pipeline value by deal stage.”
- “Show paid orders that have not been fulfilled.”
- “Add an action for moving selected candidates to interview.”

The benchmark captures the produced definition diff and proves deterministic recompilation. Explore preview is an authoring convenience, not an agent-only operation and not a way to bypass named application reads.

## Time to value and onboarding

The v0.15 five-minute path is deliberately narrow:

```text
existing Bean Entities
  -> generated <entity>_list baseline
  -> deterministic DemoSeed or explicit fixtures
  -> Explore preview
  -> save View/Displays/Page to draft
  -> validate/diff/test/publish
```

CSV row import is the next credible onboarding addition: a typed mapping and dry run should write through generated/entity Actions. It is not required for initial Explore semantics. Mapping an existing PostgreSQL table, provider schema introspection, and external read providers require new ownership, migration, Policy, and drift contracts and are deferred to a separate goal.

## Milestone sequence

1. **Universal Entity Explorer and saved Views** — preview record-shaped projection/search/filter/sort/cursor plans from any Entity and save an ordinary View/Display draft; ATS is the tracer bullet.
2. **Typed grouping, aggregation, and result shapes** — freeze scalar/date grouping, numeric/null/money rules, relationship bounds, ordering, limits, pushdown parity, and Policy-before-aggregate; ATS and CRM prove it.
3. **Compatible Displays and switching** — add cards, chart, grouped table, improved metric, and calendar contracts without letting Displays rewrite queries; ATS and Booking prove them.
4. **Page filters and dashboard composition** — typed filter fan-out across existing Blocks and one URL state; ATS overview and Asana split/context fixture.
5. **Typed drill-down** — metric/chart contribution mappings to record Displays; ATS and CRM exact-set evidence.
6. **Record and bulk Actions** — server-resolved availability, stable per-record batches, partial results, concurrency checks, and refresh; ATS, CRM, and Commerce.
7. **Agent/Studio authoring parity** — common Explore artifacts editable without hidden state, with schema/diagnostic/inspect/diff/TestSuite coverage and the recruiting prompt benchmark.
8. **Qualification and five-minute existing-Entity onboarding** — deterministic example data, all maintained regressions, documentation, version cut, and terminal gates.

Detailed implementation slices, files, tests, demo flows, and milestone non-goals are in `PLANS.md`.

## Definition of done

- An authorized builder can open every non-system Entity, safely preview a record View, and save ordinary inspectable definitions.
- The ATS recruiting overview demonstrates search/filter/sort, grouping/aggregation, compatible Displays, page filters, exact drill-down, single/bulk candidate transitions, audit, and refresh.
- CRM proves counts, sums, categories, drill records, and Action availability are calculated after Policy scoping for salesperson and manager actors on SQLite and PostgreSQL.
- Commerce proves observe -> inspect -> authorized Lifecycle Action -> refreshed dashboard without to-many analytic shortcuts.
- An agent produces the same ATS artifacts through existing schema/validate/inspect/diff/test/publish contracts with no hidden state.
- Every new public semantic has compiler diagnostics, canonical schema/AppIR, compatibility, restart, Agent Protocol inspection, and focused automated evidence.
- All maintained examples validate and package; relevant browser journeys pass.
- `make check`, `make test-crash`, `make test-postgres`, and `make build` pass.

## Explicit non-goals

- A generic BI platform or Metabase replacement.
- `Question`, `Report`, `Dataset`, `Visualization`, `Querier`, or a separate semantic/query runtime.
- Arbitrary SQL, notebooks, warehouse connector ecosystems, user scripts, custom React/Vega, or runtime renderer plugins.
- Complex pivots, to-many/distinct relation aggregation, computed inventory buckets, order-item rollups, currency conversion, or arbitrary relation depth.
- Scheduled reports, email subscriptions, alerts, public dashboard sharing, pixel-perfect reports, slides, presentation export, or issue #6 implementation.
- Numeric/full paging, total counts, unbounded results, or silent aggregate truncation.
- Atomic multi-record Actions or client-side authorization inference.
- CSV data import, existing PostgreSQL table mapping, or provider introspection in the initial goal.

## Risks and mitigations

| Risk | Mitigation |
| --- | --- |
| Preview becomes a second query engine | Compile preview and persisted Views into one canonical plan and execute both through `view.Service`. |
| Aggregate data leaks through groups or empty states | Apply Policy predicates first and test exact contribution sets with multiple actors/backends. |
| Display metadata starts owning query behavior | Freeze result shapes; move search to View; make Display switches shape-preserving. |
| Relation joins inflate counts | Allow aggregate traversal only across declared to-one relationships in v0.15. |
| Dashboards show plausible but partial aggregates | Probe group limit + 1 and return a typed overflow error. |
| Bulk UI weakens Action guarantees | Invoke ordinary Actions sequentially with per-record Policy/Lifecycle/version checks and structured partial results. |
| Demo charts are empty or accidental | Use deterministic seed distributions plus exact fixed fixtures for security and contribution assertions. |
| Explore scope delays production qualification | Keep v0.15 examples and closed vocabularies bounded; revisit v1.0 ordering explicitly in ROADMAP. |

## Deferred capabilities

- richer charts, cross-filter-in-place if navigation drill is sufficient, utilization formulas, computed read projections, to-many/distinct aggregates, average money, time zones other than UTC;
- per-user saved presets, private dashboards, public sharing, scheduled delivery, alerting, exports, and presentations;
- CSV import, existing-table mapping, provider schema introspection, and external data federation.

## Frozen product decisions and remaining question

- v0.15 precedes the previously planned v1.0 production-envelope qualification.
- Explore is served at administrator-only `/explore` and writes through the existing Studio draft lifecycle.
- The initial chart vocabulary is one accessible bar chart; richer chart types are deferred.
- Bulk execution is sequential and non-atomic across records, matching the existing Admin batch semantics.
- Calendar ships in v0.15 and Booking is its required executable proof.
- No owner decision blocks M1. After v0.15, the owner must choose whether CSV Action-based import or v1.0 production qualification is the next goal; existing-table/provider introspection should not be bundled into that choice.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
