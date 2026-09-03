# Idea: Ordered multiple-value Entity fields

Status: idea; not scheduled or implemented.

## Scope

Add first-class, typed, ordered multiple values for exactly these initial cases:

1. lists of strings and numbers;
2. multi-select enum fields;
3. multiple files in one field.

Every multiple-value field has compiler-validated minimum and maximum cardinality. Value order is part of persisted application data, is returned consistently by Views, and can be changed through accessible UI controls backed by Actions.

## Motivation

Today an Entity scalar field stores one value. To-many relations support multiple related record IDs, and `json` can technically contain an array, but neither is the right contract for simple repeatable scalar values or several files owned by one record.

Examples include:

- article aliases or alternative headings;
- ordered numeric thresholds or scores;
- categories selected from one closed enum vocabulary;
- a product image gallery;
- several documents attached directly to one record.

Using JSON would lose element typing, cardinality validation, generated form behavior, deterministic ordering semantics, and transactional file cleanup.

## Proposed authoring contract

A `multiple` object changes the field value from one scalar into an ordered list of that scalar type:

```yaml
kind: Entity
name: post
fields:
  - name: aliases
    label: Alternative titles
    type: string
    multiple:
      minValues: 0
      maxValues: 5

  - name: scores
    label: Editorial scores
    type: integer
    multiple:
      minValues: 1
      maxValues: 10

  - name: channels
    label: Channels
    type: enum
    options: [web, email, rss, social]
    multiple:
      minValues: 1
      maxValues: 3

  - name: attachments
    label: Attachments
    type: file
    multiple:
      minValues: 0
      maxValues: 8
```

The initial typed-list vocabulary should stay narrow:

- `string`;
- `integer`;
- `decimal`;
- `enum`;
- `file`.

Additional scalar types should require separate evidence rather than becoming multiple automatically.

## Cardinality contract

- `minValues` is an integer greater than or equal to zero.
- `maxValues` is a positive integer greater than or equal to `minValues`.
- The compiler enforces a runtime-owned hard maximum so definitions cannot create unbounded lists.
- Cardinality is checked after normal input decoding and before persistence.
- An empty list is valid only when `minValues: 0`.
- On create, an omitted optional list becomes an empty list; an omitted list with positive `minValues` fails validation.
- On update, omission means “leave unchanged”; an explicitly supplied empty list means “replace with empty” and is validated against `minValues`.
- `null` is not a list and should be rejected when explicitly supplied.

`required` and `minValues` must not create two contradictory meanings. A possible compatibility rule is:

- `required: true` with omitted `minValues` compiles to `minValues: 1`;
- `required: true` with `minValues: 0` is rejected;
- explicit `minValues` remains the canonical cardinality contract in AppIR.

The final design must freeze this behavior before implementation.

## Value semantics

### Typed string and number lists

Action input and View output use ordered JSON arrays:

```json
{
  "aliases": ["Bean Runtime", "Bean Platform"],
  "scores": [10, 8, 9],
  "weights": ["0.25", "0.50", "1.00"]
}
```

Values retain the existing scalar contract:

- strings remain strings;
- integers use the existing bounded integer behavior;
- decimals remain exact canonical decimal strings rather than binary floating-point values.

The whole list is validated element by element. An error should identify the field and element index, for example `scores.2`.

Whether duplicate string/number values are accepted is an open contract decision. If duplicates are accepted, each persisted item needs its own stable internal identity so reorder and removal do not depend on value equality.

### Multi-select enum

```yaml
- name: channels
  type: enum
  options: [web, email, rss, social]
  multiple:
    minValues: 1
    maxValues: 3
```

Input and output are ordered arrays of enum values:

```json
{"channels": ["web", "rss", "email"]}
```

Every item must belong to `options`. Duplicate enum values should be rejected because selecting the same option more than once has no useful meaning. Selection order remains persisted and visible even though membership is unique.

### Multiple files

```yaml
- name: attachments
  type: file
  multiple:
    minValues: 0
    maxValues: 8
```

Each file keeps the existing file guarantees:

- individual upload size remains bounded by the existing file limit;
- rows store immutable file references rather than binary content;
- download authorization continues through the owning Entity and its View/Policy path;
- removal deletes unreferenced blob content inside the same Action transaction;
- reorder changes position only and never reuploads or duplicates blob content.

The multipart/API contract must distinguish existing file references, newly uploaded files, removal, and final ordering. Existing blob identifiers can provide stable identity for persisted files; newly uploaded values need temporary request identities until the Action commits.

## Ordering contract

A multiple-value field is intrinsically ordered. It is not an unordered set.

- Input array order is authoritative on create and replacement update.
- Views return values in persisted position order.
- Generated Admin forms display that same order.
- Reordering is a write and must execute through an Action.
- Parent optimistic concurrency/version checks apply so concurrent reorder operations cannot silently overwrite each other.
- A successful value change or reorder increments the owning record version and appears in audit evidence as a change to the field.
- Removing an item compacts positions deterministically.

The system should use stable item identities internally even if ordinary View output exposes only scalar arrays. This allows reorder operations, duplicate scalar values if permitted, and file preservation without treating the scalar value itself as identity.

## UI behavior

### String and number lists

Generated forms should render an ordered field editor with:

- one typed control per value;
- Add value;
- Remove value;
- drag handle where pointer interaction is available;
- keyboard-accessible Move up and Move down controls;
- current count and minimum/maximum guidance;
- Add disabled when `maxValues` is reached;
- removal prevented or validated when it would violate `minValues`;
- element-level validation associated with the failing control.

Drag-and-drop cannot be the only reorder mechanism. DOM order must update to match persisted order after a move so keyboard and assistive-technology reading order remain correct.

### Multi-select enum

The UI should support selecting several unique options and separately expose their persisted order. A possible interaction is:

- searchable checkbox/select list for membership;
- ordered selected-value chips or rows;
- drag and keyboard controls for selected-value order;
- clear indication of `minValues` and `maxValues`.

A native `<select multiple>` may be an initial fallback, but by itself it does not provide a good explicit ordering interaction.

### Multiple files

The UI should support:

- selecting several files in one interaction;
- adding more files up to `maxValues`;
- showing filename, media type, and size;
- removing existing or newly selected files;
- reordering existing and newly selected files before submit;
- keyboard-accessible ordering controls;
- preserving existing files during reorder without downloading or reuploading them;
- showing per-file upload and validation errors without losing unaffected selections.

All form submission and reordering must invoke ordinary generated or explicit Actions. The browser must not mutate storage directly.

## Persistence direction

Do not store first-class multiple values as an opaque JSON array. A likely storage model is one generated collection table per Entity field, with logical columns for:

- owning Entity record ID;
- stable collection-item ID;
- zero-based or one-based integer position;
- one typed scalar value or file reference.

The storage contract needs uniqueness for owner plus position and backend-equivalent ordering. File collection rows reference the existing blob store. The migration and DBAL implementations remain confined to their existing packages and must work equivalently on SQLite and PostgreSQL.

Create, replacement, removal, and reorder must be atomic with the owning Action transaction. An implementation must avoid transient position uniqueness conflicts when swapping items; this is a DBAL concern and must not leak SQL into core packages.

## Read and write boundaries

- Entity metadata defines element type and cardinality.
- Views are the only public read path and return ordered arrays.
- Actions are the only write and reorder path.
- Policies continue to authorize the owning record; multiple values do not create a bypass.
- Generated create/update Actions accept the typed array shape.
- Explicit Actions may bind and replace the entire list using the same typed contract.
- Direct client control of collection table names, item ownership, positions outside the submitted list, or blob references is forbidden.

The first slice does not need element-level View filtering or aggregation. Views may select the whole ordered field value as one typed list result.

## Definition lifecycle and migration concerns

- Adding a multiple field creates additive storage without rewriting existing Entity rows.
- Existing records begin with an empty list; publication must reject or require an explicit migration strategy when new `minValues` would make those records invalid.
- Increasing `maxValues` is compatible.
- Decreasing `maxValues` must never truncate silently; activation requires preflight evidence that existing records comply or a prior Action-driven data migration.
- Increasing `minValues` has the same existing-data concern.
- Changing a field between scalar and multiple, changing its element type, or changing a file list to another type should not be treated as an automatic additive migration.
- Removing a multiple field must follow the repository's additive migration policy and file-retention rules rather than dropping data opportunistically.

The compiled AppIR must carry normalized multiple-value metadata so restart behavior does not depend on source YAML.

## Validation and test evidence

A future implementation should prove:

- schema and compiler acceptance/rejection for supported types and cardinality bounds;
- element-indexed validation diagnostics;
- create, read, replacement, append-through-full-replacement, removal, and reorder behavior;
- stable ordering after publication and restart;
- optimistic concurrency on simultaneous reorder;
- Policy enforcement on reads, writes, and file downloads;
- atomic rollback of values, positions, parent changes, audit, and blobs;
- no blob deletion during reorder;
- blob cleanup when a file is removed or its owner is hard-deleted;
- SQLite/PostgreSQL parity;
- accessible mouse and keyboard reorder UI;
- minimum/maximum UI and server validation;
- package and restart compatibility through immutable AppIR.

## Open questions

1. What hard maximum applies to scalar lists and file lists?
2. Are duplicate string and numeric values allowed?
3. Should ordinary View output expose scalar arrays only, or item identities as well for editable clients?
4. Is whole-list replacement sufficient for the first Action contract, or is a dedicated typed reorder operation needed?
5. How should multipart requests encode the final interleaved order of existing and newly uploaded files?
6. How should adding a positive `minValues` field handle existing records while preserving additive activation?
7. Should Admin UI use drag-and-drop immediately, or begin with accessible Move up/Move down controls and add dragging later?

## Non-goals

- Multiple relations; existing one-to-many and many-to-many relations remain separate.
- Arbitrary JSON arrays or nested repeatable objects.
- Lists of rich text, dates, URLs, emails, booleans, money, UUIDs, or relations in the initial slice.
- Unordered sets.
- Element-level Policy, ownership, Lifecycle, or audit records.
- Filtering, sorting, grouping, or aggregating by individual list elements in the initial slice.
- Per-file captions, roles, alt text, or other structured metadata; use a child Entity when each file needs fields of its own.
- Arbitrary client writes to collection storage or direct SQL access.
