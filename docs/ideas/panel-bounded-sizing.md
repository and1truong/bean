# Idea: Bounded Panel sizing metadata

Status: idea; not scheduled or implemented.

Related: [`panel-limitation.md`](panel-limitation.md).

## Existing baseline

Panel already has deterministic responsive behavior for its closed layout vocabulary:

| Layout | Small | Medium (`48rem`) | Large (`64rem`) |
| --- | --- | --- | --- |
| `single-column` | one column | one column | one column |
| `two-column` | stacked | two equal columns | two equal columns |
| `sidebar-main` | stacked | stacked | `1fr 2fr` |
| `main-sidebar` | stacked | stacked | `2fr 1fr` |
| `grid` | one cell per row | two cells per row | three cells per row |

The runtime owns these breakpoints and preserves source, DOM, keyboard, and screen-reader order. Page section widths separately provide `contained`, `wide`, and `full` placement.

The remaining gap is not basic responsiveness. It is bounded control over track counts and spans when a maintained application demonstrates that the existing presets are insufficient.

## Candidate scope

A future contract might permit fixed, compiler-known sizing metadata without exposing arbitrary CSS. For a sidebar layout:

```yaml
kind: Panel
name: post_panel
layout: main-sidebar
responsive:
  large:
    columns: 12
    spans:
      main: 8
      sidebar: 4
regions:
  - name: main
    blocks: [post_detail, post_comments]
  - name: sidebar
    blocks: [post_category, post_tags, related_posts]
```

For a grid of peer Blocks, a narrower contract may be sufficient:

```yaml
kind: Panel
name: post_cards
layout: grid
grid:
  columns:
    small: 1
    medium: 2
    large: 4
regions:
  - name: main
    blocks: [featured_post, recent_posts, popular_posts, editor_picks]
```

Per-item spans should remain deferred until a concrete application requires them.

## Validation principles

Any advanced contract should:

- accept only runtime-known breakpoint names;
- accept bounded positive integer column and span values, likely 1–12;
- require every span to fit within its track count;
- allow span keys only for Regions valid for the selected layout;
- preserve Region and Block source order at every breakpoint;
- compile into immutable AppIR;
- reject CSS units, selectors, classes, media queries, style strings, and visual reordering.

Sizing metadata must not alter View queries, Block bindings, Policy decisions, Actions, or record visibility.

## Rendering direction

The server should continue emitting the semantic Page → Panel → Region → Block tree. The frontend should map compiled values to bounded CSS Grid tracks while retaining `minmax(0, ...)`, `min-width: 0`, and local overflow ownership for tables and boards.

A first implementation must explicitly choose viewport or container sizing rather than accidentally inheriting framework defaults. Container queries may be preferable only if Panels later gain a demonstrated constrained-parent use case.

## Evidence required before scheduling

- A maintained application whose layout cannot be expressed adequately by current presets.
- A concrete reason that a new preset would be less suitable than author-controlled bounded spans.
- Narrow, medium, and wide browser geometry tests.
- Long-content overflow and accessibility-order tests.
- Schema, compiler, AppIR compatibility, inspection, diff, Studio, restart, and package coverage.

## Open questions

1. Should sidebar width use integer spans, semantic sizes such as `narrow`/`wide`, or a few new layout presets?
2. Does `grid` arrange ordered children of one Region, or should it support multiple named Regions?
3. How should omitted breakpoint entries inherit values?
4. Is a small gap vocabulary such as `compact`, `normal`, and `spacious` justified?
5. Should per-item spans ever be supported, and how would stable identity work for legacy `blocks` and ordered `items`?
6. Would container sizing provide enough leverage to justify changing the current viewport-owned model?

## Non-goals

- Arbitrary HTML, CSS, JavaScript, Tailwind classes, or media queries in metadata.
- Pixel coordinates or a freeform page builder.
- Breakpoint-specific visibility, Policy, reading order, or focus order.
- Changes to Entity, View, Action, database, or migration semantics.
