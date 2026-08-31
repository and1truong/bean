# Definitions

An application source starts with an `app.yaml` manifest containing `apiVersion: bean/v1alpha1`, the application name, and optional explicit local `resources`. Definition documents follow the manifest after `---`, or live in the listed resource files. Each definition has a top-level `kind`, machine `name`, optional `namespace`, and its kind-specific fields. Supported kinds are Entity, View, Action, Webform, Policy, Filter, Block, Panel, Page, Role, Menu, Job, AdminResource, and LocalRegistration.

Compilation validates envelopes, names, fields, references, relation kinds, limits, Action steps, Panel regions, and route uniqueness. Diagnostics identify the source file, line, column, kind, name, field path, and a corrective message. Generated CRUD is emitted as Views and Actions inside AppIR.

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

A View or Webform Block may declare typed `inputs` and bind them to Page context. Compilation requires each bound name and type to match the target View filter or Action input, and a Webform Block cannot bind a field that its Webform also renders. HTTP execution recomputes the binding from the matched Page route and rejects client collisions. View Blocks may configure generic `presentation` fields for list/detail mode, title/body/meta fields, route templates, rich-text fields, and empty state. Entity fields may use the `slug` type. Transaction Actions may bind `$now` once for deterministic timestamps.

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

## Board and tree presentations

A View Block can render an operational board when its presentation names a selected enum `groupField`, explicit enum `columns`, and a `moveAction`. Compilation requires the Action to be a transition for the same Entity and state field. Reads remain on the bound View and moves remain on the Action:

```yaml
presentation:
  mode: board
  titleField: title
  groupField: status
  orderField: position
  columns: [todo, in_progress, done]
  moveAction: move_task
```

A tree presentation requires a selected many-to-one self relation. It groups a flat View result by `parentField`, orders siblings by an optional integer `orderField`, renders arbitrary nesting with expand/collapse controls, and links nodes through the normal route template. The existing View maximum bounds one tree at 200 rows.

```yaml
presentation:
  mode: tree
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

An Action mutation, its audit row, idempotency result, jobs, and outbox intents share the Action transaction. Idempotency keys are scoped to an Action and include a canonical input fingerprint; reusing a key with different input is a conflict. Jobs and outbox records use persistent pending/claimed/completed-or-failed states with leases, bounded attempts, scheduled retry, stale-claim recovery, and administrator retry/cancel controls. Delivery is at-least-once, so external consumers must deduplicate.

Publication may commit an additive migration before committing the new active-release pointer. On restart, Bean validates the active AppIR against physical storage, permits harmless schema-ahead columns, and reconciles already-applied additive steps before retry. A missing table, column, relation table, or release target fails startup with a diagnostic instead of serving mixed state.
