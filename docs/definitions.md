# Definitions

An application source starts with an `app.yaml` manifest containing `apiVersion: bean/v1alpha1`, the application name, and optional explicit local `resources`. Definition documents follow the manifest after `---`, or live in the listed resource files. Each definition has a top-level `kind`, machine `name`, optional `namespace`, and its kind-specific fields. Supported kinds include Entity, View, Action, Lifecycle, Rule, Extension, TestSuite, Webform, Policy, Filter, Block, Panel, Page, Sequence, Role, Menu, Job, AdminResource, LocalRegistration, and DemoSeed.

Compilation validates envelopes, names, fields, references, relation kinds, limits, Action steps, Panel regions, and route uniqueness. Diagnostics identify the source file, line, column, kind, name, field path, and a corrective message. Generated CRUD is emitted as Views and Actions inside AppIR.

## Sequences and semantic content

A `Sequence` is a route-level ordered experience composed from existing Panels. The initial `presentation` profile supports `wide` and `standard` aspect ratios, stable frame identities, speaker notes, URL-addressed navigation, keyboard controls, progress, responsive HTML, and one-frame-per-page print structure. Sequence adds no data or mutation path: View Blocks still read through Views and Actions remain the only write boundary.

```yaml
kind: Block
name: product_thesis
type: content
content:
  - {type: heading, text: "Bean is a deterministic application runtime"}
  - {type: bullets, items: ["Definitions are inspectable", "Compilation is deterministic", "Runtime behavior is policy-bound"]}
---
kind: Panel
name: thesis_frame
layout: single-column
regions:
  - {name: main, blocks: [product_thesis]}
---
kind: Sequence
name: bean_introduction
route: /presentations/bean
title: Introducing Bean
profile: presentation
aspectRatio: wide
frames:
  - {name: thesis, title: Product thesis, layout: bullets, panel: thesis_frame, notes: "Establish the runtime boundary."}
```

Frame layouts are closed and compiler-checked against their Panel: `title`, `section`, `statement`, `bullets`, `quote`, `closing`, `two-column`, `comparison`, `image-focus`, `chart-focus`, `table`, `timeline`, `process`, and `architecture`. Content elements are `heading`, `paragraph`, `bullets`, `quote`, `code`, `callout`, `image`, and `diagram`. Images require alt text and an absolute application path or HTTPS URL; content is rendered as text nodes, never executable markup. `bean capabilities --json` reports the exact vocabularies and bounds. Current limits include 1–50 frames, 1–12 Blocks per frame, 80-code-point titles, 4,000-byte notes, 12 elements per content Block, six bullets, eight diagram nodes, 120 code lines, and deterministic layout density budgets.

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

Policy predicates and fixed/exposed filters apply before grouping. `count` over an empty contribution set is zero; other empty aggregates are null. Money sums preserve minor units. Storage adapters push down grouping and aggregation and produce backend-equivalent ordering.

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
