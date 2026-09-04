# Definitions

An application source starts with an `app.yaml` manifest containing `apiVersion: bean/v1alpha1`, the application name, and optional explicit local `resources`. Definition documents follow the manifest after `---`, or live in the listed resource files. Each definition has a top-level `kind`, machine `name`, optional `namespace`, and its kind-specific fields. Supported kinds include Entity, View, Action, Lifecycle, Rule, Extension, TestSuite, Webform, Policy, Filter, Block, Panel, Page, Sequence, Role, Menu, Job, AdminResource, Authentication, LocalRegistration, and DemoSeed.

Compilation validates envelopes, names, fields, references, relation kinds, limits, Action steps, Panel regions, and route uniqueness. Diagnostics identify the source file, line, column, kind, name, field path, and a corrective message. Generated CRUD is emitted as Views and Actions inside AppIR.

## Authentication

An optional `Authentication` named `auth` declares `preset: local|internal|public` and `registration: true|false` (default false). Enabling registration requires the existing fixed-role `LocalRegistration` contract. Without this new definition legacy behavior is unchanged. See [Authentication configuration](authentication.md) for enforcement, compatibility, and currently unsupported advanced features. Initial Authentication configuration requires AppIR v16. Optional `passwordRecovery` (default false) requires AppIR v17 and host-configured email delivery before publication/startup.

## Panel layouts

Panel uses a closed responsive preset vocabulary; authors do not supply CSS, utility classes, media queries, or breakpoint-specific order. Existing metadata maps to this runtime contract:

| Layout | Below `48rem` | `48rem`–below `64rem` | `64rem` and above |
| --- | --- | --- | --- |
| `single-column` | one column | one column | one column |
| `two-column` | Regions stack | two equal columns | two equal columns |
| `sidebar-main` | Regions stack | Regions stack | sidebar one-third, main two-thirds |
| `main-sidebar` | Regions stack | Regions stack | main two-thirds, sidebar one-third |
| `grid` | one item per row | two items per row | three items per row |

For `grid`, the columns apply to the ordered Blocks or inline content inside its `main` Region. Every preset preserves definition, DOM, keyboard, and screen-reader order at all widths. Tracks and Regions may shrink to contain wide children; individual Block and Display renderers remain responsible for local overflow. These fixed viewport thresholds are runtime-owned and require no schema or AppIR fields.

A Region may opt into server-authorized empty collapse:

```yaml
kind: Panel
name: article
layout: sidebar-main
regions:
  - {name: sidebar, blocks: [editor_tools], collapseWhenEmpty: true}
  - {name: main, blocks: [article_body]}
```

After normal Block Policy evaluation, an opted-in Region with no rendered Block nodes is omitted. If it leaves one Region in a multi-Region Panel, that survivor spans every layout track; if all Regions collapse, the Panel is unavailable to its Page or Sequence. Omitted or false `collapseWhenEmpty` preserves the existing empty track. This behavior does not react to a View returning zero rows, loading, or rendering an error, and unresolved Blocks or render errors still fail. Declared composition remains authoritative for inspection, Page-filter membership, generated checks, and bound HTTP authorization. Collapsible Regions require AppIR v11.

## Page composition

A Page may reference one legacy `panel`, or use `sections` to compose 1–32 Panels as successive layout bands. The forms are mutually exclusive. Section source order is render, DOM, keyboard, and screen-reader order; repeated Panel references are allowed. An optional machine `id` stabilizes internal section identity across nearby reordering, while omitted IDs derive deterministically from the Page name and section ordinal. This supports full-width → sidebar → grid compositions without nested Panels or arbitrary layout metadata:

```yaml
kind: Page
name: article
route: /articles/:id
sections:
  - {id: hero, panel: article_hero, width: full}
  - {id: body, panel: article_with_sidebar, width: contained}
  - {panel: related_articles, width: wide}
  - {panel: comments}
```

A section's optional `width` uses the closed `contained`, `wide`, or `full` vocabulary. `contained` is a runtime-owned readable `48rem`, `wide` is the existing `72rem` application width, and `full` fills the available viewport; all three retain standard `1rem`/`1.5rem` safe gutters. Omitted width and legacy `Page.panel` mean `wide`. Width is Page placement metadata, so the same Panel may be reused at different widths and Sequence Panels are unaffected. Section widths require AppIR v12 and do not provide edge-to-edge bleed, backgrounds, arbitrary lengths, or breakpoint overrides.

Page context is resolved once and supplied to every section. Page filters may target named View Blocks in any declared Panel. The Page Policy remains the outer authorization boundary; each Panel and named Block then applies its own Policy. A denied Panel is omitted without changing the relative order of visible sections, and a Page with no visible sections is unavailable. Bound View and Webform requests must name a Block in a declared section and pass at least one containing Panel's Policy. Legacy `panel: name` remains unchanged in source and AppIR; ordered sections require AppIR v10.

## Menus and record navigation

A `workspace` Menu owns a bounded, ordered hierarchy of typed Page or View Page Display targets. Its optional `variant` is limited to `default` or `line` and normalizes to `default`; it controls only the source-owned shadcn-style route-navigation visual treatment. Global static placements have stable IDs, optional parents, weight from `-1000` through `1000`, and an optional 120-character label override. Scoped Menus declare an owner Entity and derive one logical instance per owner record; their Entity-record placements remain application data rather than AppIR.

A navigation-enabled Entity declares its visible label field, same-Entity View Page Display destination, and eligible scoped Menus. Create and update Actions may receive optional `_navigation: {placements: [...]}` state. Omission preserves placement state, while submission replaces it in the same transaction as record fields. Parent/cycle/depth, duplicate target, Policy, owner, count, and label bounds are server validated. Target and owner deletion clean up placements atomically; definition publication rejects contracts that would orphan live placements.

The Menu runtime resolves labels and routes through Views, filters denied targets and descendants on the server, and returns only an authorized tree. `workspace` presentation uses horizontal levels one and two, vertical level three on wider screens, and one labelled native select for level three on narrow screens. It uses labelled `nav` elements, links, `aria-current`, and active presentation state, not `tablist`, `tab`, or `tabpanel` semantics. Legacy flat literal-route Menus remain compatible. Hierarchical navigation requires AppIR v13; Menu visual variants require AppIR v14. See `examples/books` for global and owner-scoped definitions and generated record editing.

## Sequences and semantic content

A `Sequence` is a route-level ordered experience composed from existing Panels. The initial `presentation` profile supports `wide` and `standard` aspect ratios, stable frame identities, speaker notes, URL-addressed navigation, keyboard controls, progress, responsive HTML, and one-frame-per-page print structure. Sequence adds no data or mutation path: View Blocks still read through Views and Actions remain the only write boundary.

Each frame has an optional `direction`: `next` starts a horizontal chapter and `down` adds a vertical detail frame to the current chapter. Omission defaults to `next`, preserving flat Sequences. The first frame must be `next`. Left/right navigation moves between chapter roots; up/down moves within one chapter; Page Up/Page Down follows the complete source order. If Policy filtering removes a chapter root, its first visible detail frame is promoted to a horizontal root so the authorized navigation remains connected. Directional frames require AppIR v15.

```yaml
kind: Block
name: capability_chart
type: view
view: capabilities_by_area
display: chart
---
kind: Panel
name: capabilities_frame
layout: single-column
regions:
  - name: main
    items:
      - id: introduction
        content:
          - {type: heading, text: "Bean is a deterministic application runtime"}
          - {type: paragraph, text: "The chart follows the normal View path."}
      - block: capability_chart
      - content:
          - {type: callout, tone: success, text: "Definitions remain inspectable."}
---
kind: Sequence
name: bean_introduction
route: /presentations/bean
title: Introducing Bean
profile: presentation
aspectRatio: wide
frames:
  - {name: capabilities, title: Product capabilities, direction: next, layout: chart-focus, panel: capabilities_frame, notes: "Show the live data."}
  - {name: details, title: Capability detail, direction: down, layout: bullets, panel: capability_details}
```

A Panel region has two compatible source forms. Existing `blocks: [name, ...]` remains unchanged. Use `items` when content is local to the Panel or when inline content and named Blocks must be interleaved: every item contains exactly one `block` reference or one non-empty `content` list, and list order is render order. A region cannot declare both `blocks` and `items`. An inline item may declare a region-local machine `id` when its identity should survive nearby reordering; otherwise compilation derives identity from the Panel, region, and item ordinal. Generated identities are nested AppIR details, not global Block names or authoring references. A named `type: content` Block remains appropriate for reuse, independent Block Policy, or deliberate public identity.

Inline content uses the same `ContentBlock` renderer and validation as named content Blocks. It has no independent policy: it is visible when its enclosing Page or Sequence and Panel are visible. A referenced named Block still applies its own Policy, so hiding that Block does not hide neighboring inline content. Compiler diagnostics are owned by the Panel and use source-indexed paths such as `spec.regions.0.items.1.content.0.alt`.

Frame layouts are closed and compiler-checked against their Panel: `title`, `section`, `statement`, `bullets`, `quote`, `closing`, `two-column`, `comparison`, `image-focus`, `chart-focus`, `table`, `timeline`, `process`, and `architecture`. Content elements are `heading`, `paragraph`, `bullets`, `quote`, `code`, `callout`, `image`, and `diagram`. Images require alt text and an absolute application path or HTTPS URL; content is rendered as text nodes, never executable markup. `bean capabilities --json` reports the exact vocabularies and bounds. Current limits include 1–50 frames, 1–12 rendered Blocks (including inline content items) per frame, 80-code-point titles, 4,000-byte notes, 12 elements per named or inline content Block, six bullets, eight diagram nodes, 120 code lines, and deterministic layout density budgets.

## Admin resources

Every Entity receives a generated AdminResource backed by its generated list View and create/update/delete Actions. Add an explicit definition to control its presentation and domain operations:

```yaml
kind: AdminResource
name: article
entity: article
description: Editorial content
labelField: title
list:
  columns: [id, title, status, updated_at]
  search: [title, body]
  filters: [status]
  sort: [{field: updated_at, desc: true}]
  pageSize: 25
form:
  fields: [title, body, status]
  readonly: [created_at, updated_at, version]
actions: [publish_article]
```

`view`, `createAction`, `updateAction`, and `deleteAction` may override the generated names. Configured list fields must be selected by the View, and every Action must target the same Entity. Publication fails on invalid references. Application Admin requires editor or administrator access; System Admin and Studio remain administrator-only. Admin can only search, filter, or sort fields declared by metadata.

When an AdminResource's Entity owns a scoped Menu, its record response also contains the authorized Menu tree and contextual create targets derived from existing metadata. A target is eligible when its Entity declares that Menu, an AdminResource exposes the Entity, and that resource has an authorized create Action. Menu and resource names are sorted deterministically and bounded by the existing navigation limits; absent or unauthorized targets are omitted. This adds no AdminResource fields or AppIR version.

The canonical contextual route is `/admin/:ownerResource/:ownerID/create/:targetResource?menu=:menu`. Direct loads re-read the owner response and require the compiled owner/Menu/target triple. The form fixes the owner and Menu, offers only authorized tree placements as parents, and submits exactly one `_navigation` placement through the target resource's existing create Action. The owner is re-authorized inside that Action transaction; client route state and prior GET snapshots are not write authority. `/admin/:resource/new` and target-side Navigation editing remain available.

## Local registration and sensitive inputs

Local signup is disabled unless a `LocalRegistration` definition references a `register_local_user` Action. An optional static `route` advertises browser signup and must reference an anonymously accessible Page and Panel containing an anonymously accessible Webform Block for that Action. The full Page must render with anonymous identity and no query parameters, with its Page policy checked before context resolution exactly as it is at request time, and the Webform must expose every required registration input unconditionally without duplicating Block-bound fields; omit `route` for action-only registration. That Action declares a literal `defaultRole`; compilation requires the Role to exist and rejects the privileged `editor` and `administrator` roles. Registration compilation supplies fixed display-name, email, password, and password-confirmation inputs. Password inputs are sensitive/write-only and Action output is limited to safe identity fields. The client cannot provide roles, tenants, or system fields.

## Bound blocks and content presentation

A View owns one query contract and any number of named displays. `page` and `block` displays use the closed `list`, `detail`, `table`, `cards`, `board`, `tree`, `chart`, `metric`, `timeline`, or `calendar` renderer vocabulary; `json`, `csv`, and `rss` displays serialize the same Policy-preserving result. A page display owns its route, bindings, title, description, controls, pager, drill, selection, Actions, and empty state. A View Block mounts a named `block` display without copying presentation metadata. Legacy `Block.presentation` source remains accepted and compiles to a private compatibility display.

```yaml
kind: View
name: articles
entity: article
fields: [id, title, status, published_at]
exposedFilters:
  status: {field: status, operator: eq}
displays:
  index:
    type: page
    route: /articles
    title: {text: Articles}
    renderer:
      type: table
      fields:
        - {field: title, label: Article, linkRoute: /articles/:id}
        - {field: status, label: Status}
    controls:
      - {filter: status, label: Publication status, widget: select}
    pager: {type: cursor, pageSize: 25}
  recent:
    type: block
    renderer: {type: list, titleField: title}
    pager: {type: none}
---
kind: Block
name: recent_articles
type: view
view: articles
display: recent
```

Exposed filters map public input names to selected fields and accept `eq`, textual `contains`, or ordered `gte`/`lte`. Displays choose interactive controls and own their labels, defaults, and `auto`, `text`, `select`, `checkbox`, `number`, or `date` widget. Submitted values are validated using the Entity field contract. Page bindings and Block bindings are recomputed from trusted request context; bound inputs cannot also be controls and client collisions fail closed. Cursor filters are URL-addressable, use opaque previous/next state, and cannot exceed the View maximum of 200 rows.

## Explore query and result shapes

`View` is the typed query model. `search.fields` names selected textual fields. `groupBy` entries use `{field, as, bucket}`; `bucket` is optional and accepts UTC `day`, `week`, or `month` for date/datetime fields. `aggregates` use `{function, field, alias}` with `count`, `sum`, `min`, `max`, or `average`; money may be summed but not averaged. The compiler rejects alias collisions, incompatible types, redacted inputs, to-many aggregate traversal, and unsupported buckets. It derives one result shape:

- `records`: no aggregate; cursor paging and deterministic record ordering apply;
- `metric`: exactly one aggregate and no group; one row, no pager;
- `groups`: at least one group and aggregate; bounded to the View maximum and fails with `result_limit_exceeded` rather than truncating.

Policy predicates and fixed/exposed filters apply before grouping. `count` over an empty contribution set is zero; other empty aggregates are null. Money sums preserve minor units. Decimal `sum`, `min`, and `max` remain exact; decimal averages round to 16 fractional digits before canonical trailing-zero removal. Storage adapters push down grouping and aggregation and produce backend-equivalent values and ordering.

```yaml
kind: View
name: candidates_by_stage
entity: candidate
fields: [stage]
groupBy: [{field: stage}]
aggregates: [{function: count, field: id, alias: candidate_count}]
displays:
  chart:
    type: block
    renderer: {type: chart, groupField: stage, metricField: candidate_count}
    pager: {type: none}
```

Displays must match the compiled shape: charts consume grouped scalar + numeric outputs; metrics consume a metric output; record renderers consume records; tables accept records or groups. Calendar requires selected date/datetime start and optional end fields. Switching named Displays never changes the View query.

## Page filters, drill-down, and selected Actions

A Page filter explicitly targets one or more View Blocks and exposed filter names. Types/options are derived and checked across targets; Page URL state cannot override immutable route/context bindings. There is no implicit same-name fan-out.

```yaml
filters:
  stage:
    label: Stage
    widget: select
    targets:
      - {block: candidate_metric, filter: stage}
      - {block: candidate_stage_chart, filter: stage}
```

A chart or metric drill names a target record View/Display and maps only compiler-known `group` or active `filter` values to target exposed filters. The compiler derives the target route; definitions cannot supply a private query or URL template as authority. The target executes under its own Policy.

Record table/board Displays may declare `selection: single|multiple` and same-Entity `actions`. The public batch endpoint accepts 1–200 unique record IDs and shared typed values, executes the ordinary Action once per record in order, and reports ordered success/failure entries. Execution is sequential and non-atomic across records: a later failure does not roll back an earlier success. Each call independently enforces Policy, Rule, Lifecycle, version checks, audit, and transaction semantics.

Table columns are ordered, selected, non-redacted View fields with optional labels and safe application route templates. A page title may be static or sourced from a result field when a unique route-bound filter proves a single detail record; it becomes both the page heading and browser title.

A View or Webform Block may declare typed `inputs` and bind them to Page context. Compilation requires each bound name and type to match the target View filter or Action input, and a Webform Block cannot bind a field that its Webform also renders. HTTP execution recomputes the binding from the matched Page route and rejects client collisions. Entity fields may use the `slug` type. Transaction Actions may bind `$now` once for deterministic timestamps.

A `resource-list` Block embeds an AdminResource table and its Actions in a Page and must use a Policy whose readers are limited to `editor` and `administrator`. Its bound inputs scope the backing View immutably; `filters` is the allowlist of values a user may change, and `defaultFilters` only controls initial presentation:

```yaml
kind: Block
name: project_tasks
type: resource-list
resource: task
inputs: {project_id: {type: uuid, required: true}}
bindings: {project_id: {source: context, name: id, required: true}}
filters: [status]
defaultFilters: {status: open}
```

Compilation requires bound and interactive fields to be exposed by the AdminResource View, requires interactive fields to be configured by the AdminResource, and rejects overlap between the two sets. Page, Block, View, and Action policies continue to authorize reads and writes independently.

## Board and tree renderers

A View block display can render an operational board when its renderer names a selected enum `groupField`, explicit enum `columns`, and a `moveAction`. Compilation requires the Action to be a transition for the same Entity and state field. Reads remain on the View and moves remain on the Action:

```yaml
renderer:
  type: board
  titleField: title
  groupField: status
  orderField: position
  columns: [todo, in_progress, done]
  moveAction: move_task
```

A tree renderer requires a selected many-to-one self relation. It groups a flat View result by `parentField`, orders siblings by an optional integer `orderField`, renders arbitrary nesting with expand/collapse controls, and links nodes through the normal route template. The existing View maximum bounds one tree at 200 rows.

```yaml
renderer:
  type: tree
  titleField: title
  parentField: parent_id
  orderField: position
  linkRoute: /tasks/:id
```

## File fields

`file` is a bounded immutable upload reference. It is accepted only from multipart Action or Webform requests, is limited to 5 MiB, and is persisted with generated identity, safe display filename, media type, size, and base64 content in `bean_blob` within the application Action transaction. Entity rows store only the generated identifier. Replacing or hard-deleting the row removes the previous blob in the same transaction; soft deletion retains it. Downloads require a live referencing row and apply that Entity's read policy before returning attachment content with `nosniff` and attachment response headers.

For multiple files, define an attachment Entity with one `file` field plus a many-to-one relation to the owning record. This keeps cardinality and application labels in metadata rather than introducing a collection-valued binary column.

## Content filters

A `Filter` is a named, immutable output-formatting pipeline. Views opt individual selected textual fields into a Filter; source data remains unchanged and another View, such as an Admin View, can return the source for editing.

```yaml
kind: Filter
name: markdown
steps: [{type: markdown}]

---
kind: View
name: published_article
entity: article
fields: [id, title, body]
fieldFilters: {body: markdown}
```

`markdown` is the first supported step. It produces sanitized HTML: raw HTML, executable content, images, event attributes, and unsafe URL schemes are removed. Filtering occurs after policy redaction and is part of the View output contract for JSON, CSV, RSS, and Page rendering. Action input and output remain unformatted. Content Filters are distinct from View predicates and `exposedFilters`, which constrain database queries.

## Runtime durability

An `Extension` declares one typed out-of-process HTTP effect. Transaction Actions bind its input from typed Action inputs, literals, or statically typed entity-row step results in an `extension` step; required Extension fields require non-nullable sources, and source enum options must be a subset of the Extension field options. Bean stores the invocation with domain writes and delivers it only after commit. v0.13 accepts HTTPS endpoints, plus loopback HTTP for local development, with closed `network` permission and `external_write` effect vocabularies:

```yaml
kind: Extension
name: order_notification
transport: http
endpoint: https://provider.example/orders
input: {order_id: {type: uuid, required: true}}
output: {accepted: {type: boolean, required: true}}
permissions: [network]
sideEffects: [external_write]
authentication: bearer
timeoutSeconds: 5
retry: {maxAttempts: 3, delaySeconds: 60}
idempotency: required
transaction: after_commit
failure: retry_then_fail
```

Bearer tokens are host configuration in `BEAN_EXTENSION_BEARER_TOKENS`, encoded as a JSON object from Extension name to token. They never belong in definitions. Redirects are refused, responses are limited to 1 MiB and validated against `output`, and one immutable invocation ID is sent as the idempotency key on every attempt. Each intent pins its originating canonical Extension contract, so activating a newer release cannot retarget or reinterpret pending delivery. Provider output cannot feed a later Action step because delivery occurs outside the transaction. The `bean.extension/` event topic prefix is reserved for these compiled intents and is rejected on ordinary `emit` steps.

An Action mutation, its audit row, idempotency result, jobs, and outbox intents share the Action transaction. Idempotency keys are scoped to an Action and include a canonical input fingerprint; reusing a key with different input is a conflict. Jobs and outbox records use persistent pending/claimed/completed-or-failed states with leases, bounded attempts, scheduled retry, stale-claim recovery, and administrator retry/cancel controls. Delivery is at-least-once, so external consumers must deduplicate.

Publication may commit an additive migration before committing the new active-release pointer. On restart, Bean validates the active AppIR against physical storage, permits harmless schema-ahead columns, and reconciles already-applied additive steps before retry. A missing table, column, relation table, or release target fails startup with a diagnostic instead of serving mixed state.
