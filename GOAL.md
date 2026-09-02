# Goal: Bean v0.14 First-class View Displays

Status: complete

Make `View` Bean's complete read-and-presentation primitive rather than exposing a query definition under a product name that implies a Drupal-like display system. One View keeps one canonical query contract and owns named displays for pages, blocks, and serialized endpoints. Displays add table rendering, interactive exposed filters, explicit pager behavior, and page titles without weakening Policy, immutable bindings, or the View-read/Action-write boundary.

## Primary outcome

```text
View query metadata
  -> compiler-validated query plan
  -> one or more named displays
       -> page or block renderer
       -> JSON / CSV / RSS endpoint
  -> Policy-preserving View execution
  -> deterministic render metadata and UI
```

`View` remains the public definition. Query planning is an internal responsibility, not a new public `Query` or `Querier` definition.

## Frozen v0.14 contract

### View ownership

- Existing View query fields remain canonical at the View top level: entity, projection, relationships, fixed and contextual predicates, exposed-filter inputs, field filters, sorting, grouping, aggregates, Policy, and result bounds.
- A View owns named displays. Initial display types are `page`, `block`, `json`, `csv`, and `rss`.
- `page` and `block` displays select one compiler-known renderer. Existing `list`, `detail`, `board`, `tree`, `metric`, and `timeline` behavior moves behind this display contract; `table` is the new renderer.
- A `page` display owns its route, immutable route/context bindings, title, optional description, renderer, controls, pager, and empty state. It is the simple single-View page path. Existing Page/Panel/Block composition remains the path for pages containing multiple independent components.
- A `block` display has no route and is mounted by a View Block through `view` plus `display`. Block bindings provide immutable context but do not own presentation metadata.
- JSON, CSV, and RSS displays continue to execute the same View query and Policy contract. Existing serializer display metadata remains source-compatible.

### Compatibility and AppIR

- Introduce one explicit AppIR format for first-class displays. Older AppIR formats remain readable with their existing semantics.
- Existing source using `Block.presentation` compiles through a deterministic compatibility normalizer into a private named block display. New or migrated source uses View-owned displays and a Block display reference.
- Existing `View.displays` entries using `type: json`, `csv`, or `rss` retain their routes and wire output.
- Inspection, semantic diff, references, capabilities, generated schemas, release restart, and Agent Protocol output expose the same normalized display contract.
- Compatibility output is deterministic and does not make generated private displays editable application definitions.

### Table renderer

- A table declares an ordered non-empty column list. Each column references a selected, non-redacted View field and may set a display label and a route template.
- Cell formatting is inferred from the compiled field type in v0.14; arbitrary formatter code, HTML templates, and per-cell scripts are unavailable.
- Route-template parameters must reference selected, non-redacted fields and are encoded by the renderer.
- Empty state and responsive horizontal overflow use the shared View display contract.

### Exposed controls

- View-level exposed filters define the typed query inputs and the database fields they may constrain. New source names the target `field` and optional `operator`; the compiler derives type, options, and relation metadata from the Entity. Existing field-shaped exposed-filter source remains compatibility input.
- The initial closed operator vocabulary is `eq`, `contains`, `gte`, and `lte`. Compiler validation restricts operators by field type; `contains` is textual and ordered comparisons require an ordered scalar type.
- A page or block display explicitly chooses which exposed filters become interactive controls. Each control owns its label, optional default, and one closed widget choice: `auto`, `text`, `select`, `checkbox`, `number`, or `date`.
- Defaults and submitted values use the same field validation as direct View execution. Unknown filters, unsupported operators, and invalid values fail closed.
- Route/context-bound inputs cannot also be interactive. The server recomputes bound values and rejects collisions exactly as it does for existing bound Blocks.
- Filter state is addressable in the URL and resets that display's pager without changing unrelated displays on a composed Page.

### Pager and title

- UI displays declare `none` or `cursor` paging and an explicit page size bounded by the View default and maximum limits.
- Cursor paging uses the existing opaque keyset contract and supports deterministic previous/next navigation. Existing offset API access remains compatible, but numeric/full paging and total counts are not part of v0.14.
- A page display title is either static or sourced from one selected, non-redacted result field with a static fallback. Result-field titles are valid only for a compiler-proven single-record detail display.
- The resolved page title is rendered as the page heading and browser document title. A display description remains static in v0.14.

### Reference metadata shape

```yaml
kind: View
name: articles
entity: article
fields: [id, title, status, published_at]
sort: [{field: published_at, desc: true}]
defaultLimit: 25
maxLimit: 100
exposedFilters:
  id: {field: id, operator: eq}
  status: {field: status, operator: eq}
  published_after: {field: published_at, operator: gte}
displays:
  index:
    type: page
    route: /articles
    title: {text: Articles}
    renderer:
      type: table
      fields:
        - {field: title, label: Article, linkRoute: '/articles/:id'}
        - {field: status, label: Status}
        - {field: published_at, label: Published}
    controls:
      - {filter: status, label: Publication status, widget: select}
      - {filter: published_after, label: Published after, widget: date}
    pager: {type: cursor, pageSize: 25}
    emptyState: No articles match.
  detail:
    type: page
    route: /articles/:id
    bindings:
      id: {source: route, name: id, required: true}
    title: {field: title, fallback: Article}
    renderer: {type: detail, titleField: title}
  recent:
    type: block
    title: {text: Recent articles}
    renderer: {type: list, titleField: title}
    pager: {type: none}
  feed: {type: rss, route: /articles.rss}
```

A composed Page mounts the reusable display without copying its renderer metadata:

```yaml
kind: Block
name: recent_articles
type: view
view: articles
display: recent
```

### Authoring and reference slices

- Studio can author the v0.14 common path without Advanced JSON: named page/block displays, routes, list/detail/table renderers, table columns, exposed controls, cursor pager, and static or detail-result title.
- Advanced JSON remains available for relationships, aggregate Views, and specialized renderer fields not covered by the focused visual editor.
- The blog migrates its public list/detail Blocks to named View displays and proves a result-derived detail title.
- The ATS adds a metadata-only table page with a labelled stage control and cursor pager. Core packages contain no blog or ATS branch.
- Existing AdminResource and `resource-list` behavior remains supported and reuses shared table, control, and pager UI primitives where their read-only behavior is equivalent. Admin Actions and selection remain Admin-owned.

## Architecture constraints

- Preserve definition -> validation -> migration -> immutable AppIR -> atomic activation.
- Preserve View reads, Action writes, Policy enforcement, field redaction, route-bound input recomputation, and the 200-row maximum.
- Keep application behavior and display choices in metadata under `examples/`; core packages remain generic.
- Keep SQL and SQLite/PostgreSQL dependencies confined to the existing DBAL and migration packages.
- Do not create a second query engine for display execution. HTTP, Page, Block, Admin, CLI, and Agent Protocol reads continue through the View service.
- Renderer and control vocabularies are closed, compiler-known contracts. No application-supplied React, CSS, JavaScript, SQL, or templates enter core execution.
- Add failing contract evidence before each public behavior and run the nearest focused test after each milestone.

## Measurable acceptance criteria

- A View can publish multiple named page, block, and serializer displays that all reuse one normalized query definition and independently validate routes and presentation metadata.
- Compiler tests reject duplicate routes, missing displays, incompatible display types, invalid renderer fields, redacted references, invalid table columns, unsupported exposed-filter operators/widgets, binding/control collisions, invalid pager bounds, and unsafe dynamic titles.
- Compatibility tests prove existing v0.13 View and Block sources compile with unchanged observable behavior and existing AppIR v1-v5 releases restart unchanged.
- HTTP and View tests prove filter coercion/operators, Policy and tenant/owner preservation, bound-input collision refusal, cursor invalidation after filter changes, result limits, and SQLite/PostgreSQL parity.
- React tests prove public table rendering, labels, controls, URL state, empty/error states, independent block paging, previous/next navigation, static/result-derived headings, and browser document titles.
- Studio tests prove the common display path can be authored without raw JSON and produces schema-valid definitions.
- Blog and ATS source, package/restart, generated checks, and browser journeys prove the metadata-only reference slices.
- `make check`, `make test-crash`, `make test-postgres`, and `make build` pass.

## Explicit non-goals

- A public `Query`/`Querier` definition, nested query DSL rewrite, raw SQL, arbitrary joins beyond the existing relationship contract, or a second read path.
- Numeric/full pagers, total-result counts, random access to arbitrary pages, infinite scrolling, saved user filter presets, or asynchronous autocomplete controls.
- Arbitrary templates, user-authored HTML/React/CSS/JavaScript, custom formatter plugins, runtime renderer registration, or application-specific core branches.
- A general Page replacement. Complex composed pages continue to use Page, Panel, and Block definitions.
- `sequence`, slides, presentation decks, speaker notes, density/overflow diagnostics, charts, diagrams, image generation, HTML/PDF/PPTX export, or implementation of [issue #6](https://github.com/and1truong/bean/issues/6). v0.14 establishes the reusable display seam those later experiments may consume.
- Reworking Admin Actions, forms, bulk selection, audit history, or System Admin behavior.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
