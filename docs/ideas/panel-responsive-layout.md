# Idea: Responsive Panel layouts

Status: idea; not scheduled or implemented.

## Problem

A Panel currently describes semantic composition through a closed `layout` name and named regions:

- `single-column` → `main`
- `two-column` → `left`, `right`
- `sidebar-main` → `sidebar`, `main`
- `main-sidebar` → `main`, `sidebar`
- `grid` → `main`

The compiler validates these names, but the definition does not specify what happens at small, medium, or large screen sizes. It also cannot express grid column count, region span, gap, minimum width, or when a multi-column layout should collapse.

The public Page renderer currently preserves `data-layout` and `data-region` semantics but does not provide a complete responsive visual mapping for ordinary Panels. The presentation renderer has one concrete behavior for `two-column`: one column below `md`, two columns from `md` upward. This is an implementation detail rather than a complete Panel contract.

## Desired outcome

Panel metadata should produce predictable responsive behavior without exposing arbitrary CSS, Tailwind classes, media queries, or executable layout code.

An author should be able to answer:

- How many columns are shown on small, medium, and large screens?
- When do sidebar layouts stack?
- What proportion belongs to `main` and `sidebar`?
- How many cells does a `grid` show at each breakpoint?
- Does source and keyboard-navigation order remain stable when presentation changes?

## Suggested staged design

### Stage 1: freeze responsive preset semantics

First, make existing layout names useful without adding author-controlled sizing:

| Layout | Small | Medium | Large |
| --- | --- | --- | --- |
| `single-column` | one column | one column | one column |
| `two-column` | stacked | two equal columns | two equal columns |
| `sidebar-main` | stacked in source order | stacked | `1fr 2fr` |
| `main-sidebar` | stacked in source order | stacked | `2fr 1fr` |
| `grid` | one cell per row | two cells per row | three cells per row |

Exact breakpoint values should be part of the Theme/runtime contract rather than inferred from whichever CSS framework is in use. A likely initial vocabulary is `small`, `medium`, and `large`, with fixed runtime-owned thresholds.

This stage can remain source-compatible: existing definitions gain deterministic responsive rendering without schema changes.

### Stage 2: optional bounded sizing metadata

If applications demonstrate a need beyond presets, add typed metadata rather than arbitrary style strings. One possible direction for a sidebar layout is:

```yaml
kind: Panel
name: post_panel
layout: main-sidebar
responsive:
  small:
    columns: 1
  medium:
    columns: 1
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

Expected behavior:

- small: regions stack and use the full available width;
- medium: regions remain stacked;
- large: `main` occupies 8 of 12 tracks and `sidebar` occupies 4 of 12 tracks.

A grid of peer Blocks may need a separate, simpler contract:

```yaml
kind: Panel
name: post_cards
layout: grid
grid:
  columns:
    small: 1
    medium: 2
    large: 3
regions:
  - name: main
    blocks: [featured_post, recent_posts, popular_posts]
```

Here, `columns` applies to the ordered children of the `main` region. Per-item spans should be deferred until a concrete application requires them.

## Validation principles

Any advanced contract should remain closed and compiler-checkable:

- accept only known breakpoint names;
- accept bounded positive integer column and span values, likely 1–12;
- require every span to be less than or equal to its breakpoint's column count;
- allow span keys only for regions valid for the selected layout;
- reject unknown CSS units, selectors, classes, media queries, and style strings;
- preserve region and Block source order at every breakpoint;
- do not use visual reordering that changes reading, focus, or assistive-technology order;
- compile the result into immutable AppIR rather than interpreting source YAML in the browser.

Responsive metadata affects composition only. It must not alter View queries, Block bindings, Policy decisions, Actions, or record visibility.

## Rendering direction

The server should continue emitting the semantic Page → Panel → Region → Block tree. The frontend maps compiled Panel metadata to a bounded CSS Grid/Flex implementation.

Possible implementation rules:

- use `minmax(0, 1fr)` tracks to prevent wide Block content from overflowing its region;
- stack regions by default and enhance at the declared breakpoint;
- keep tables and boards responsible for their own local overflow behavior;
- let the application shell or Theme own maximum content width separately from Panel track sizing;
- test narrow widths, intermediate widths, wide widths, keyboard order, and long-content overflow.

Container queries may eventually be preferable to viewport media queries because a Panel can be embedded in a constrained parent. The first contract must explicitly choose one model; it should not accidentally depend on framework defaults.

## Open questions

1. Are deterministic presets sufficient, or is per-Panel `responsive` metadata justified by maintained examples?
2. Should breakpoint thresholds belong to Theme, be runtime constants, or use container sizes?
3. Does `grid` arrange Region children, or should it eventually support multiple named regions?
4. Should gap be a small semantic vocabulary such as `compact`, `normal`, and `spacious`, rather than a number?
5. Should sidebar width use integer spans, semantic sizes such as `narrow`/`wide`, or both?
6. How should missing breakpoint entries inherit: from the next smaller size, from the preset, or by explicit compiler normalization?
7. Should per-item grid spans ever be supported, and how would stable identity work for legacy `blocks` versus ordered `items`?

## Non-goals

- Arbitrary HTML, CSS, JavaScript, Tailwind classes, or freeform media queries in application metadata.
- Pixel-perfect coordinates or a freeform page builder.
- Breakpoint-specific content visibility or Policy behavior.
- Breakpoint-specific source/keyboard order.
- Changes to Entity, View, Action, database, or migration semantics.
