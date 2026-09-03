# Goal: Ergonomic inline semantic Panel content

Status: complete

Allow frame-local headings, paragraphs, bullets, diagrams, and other semantic content to be authored directly in a Panel region without requiring a globally named Block, while preserving named Blocks for reuse and data-backed behavior.

## Design

A region may use one of two source forms:

- legacy `blocks: [name, ...]`, unchanged;
- canonical ordered `items`, where each item contains exactly one `block: name` reference or one non-empty `content: [...]` semantic-content list. An inline content item may have an optional region-local machine `id`.

`items` is the only form for interleaving inline content and named Blocks, so source order is render order. A region cannot declare both `blocks` and `items`.

The compiler lowers every inline item into an immutable nested AppIR region item with an internal identity `@inline/<panel>/<region>/<id-or-ordinal>`. Explicit `id` keeps identity stable when nearby items are reordered; otherwise the item ordinal is deterministic. Internal identities are not global Block definitions or valid authoring references.

Inline content has no independent policy. It is visible whenever its enclosing Panel (and Page or Sequence) is visible. Referenced named Blocks retain their own Policy checks. Diagnostics are owned by the Panel and use index-based source paths such as `spec.regions.0.items.1.content.0.alt`.

## Acceptance criteria

- Panel regions render inline semantic content through the existing `ContentBlock` renderer.
- Ordered `items` deterministically interleave inline content and named Block references.
- Inline content receives the same vocabulary, accessibility, count, code, diagram, and Sequence density validation as named content Blocks.
- Sequence Block counts, feature checks, and render visibility include inline items.
- Generated identities are deterministic, inspectable in AppIR, and unavailable as global references.
- Legacy `regions[].blocks` and named content Blocks remain compatible.
- `examples/presentation` uses inline content while retaining a reusable named content Block and the live View/chart Block.
- `make check` and `make build` pass.

## Non-goals

- A Presentation/Slide runtime or raw-YAML runtime parsing.
- Inline View, Webform, Action, or independently policy-bound Blocks.
- Changes to View reads, Action writes, SQL, SQLite, or migrations.
