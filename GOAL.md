# Goal: Semantic Page section widths

Status: complete

Allow each ordered Page section to choose a bounded semantic content width so full, wide, and readable bands can coexist without arbitrary CSS or nested Panels.

## Design

```yaml
kind: Page
name: article
route: /articles/:slug
sections:
  - {id: hero, panel: article_hero, width: full}
  - {id: body, panel: article_body, width: contained}
  - {id: related, panel: related_posts, width: wide}
```

`sections[].width` accepts only:

- `contained`: runtime-owned readable width of `48rem` including safe horizontal gutters;
- `wide`: current application width of `72rem` including safe gutters;
- `full`: all available viewport width with the same safe gutters, not edge-to-edge bleed.

Omitted width normalizes to `wide`. Legacy `Page.panel` also renders as implicit `wide`, preserving its geometry. Below a section's maximum all modes fill the available viewport, so no new breakpoint metadata is introduced.

The width belongs to Page placement rather than Panel because one reusable Panel may appear in different Page contexts, while Sequence layout remains independent. AppIR v12 stores normalized section widths. AppIR v11 and earlier remain loadable; missing width continues to mean `wide`.

The server annotates Page-owned Panel render nodes with the compiled width. The browser applies deterministic runtime classes and data attributes and keeps title, description, and Page filters at the existing `wide` width. Width never changes definition order, DOM order, keyboard order, screen-reader order, Policy, context, View reads, or Action writes.

## Acceptance criteria

- Page sections accept only `contained`, `wide`, or `full`.
- Omitted widths and legacy `panel` preserve the current `wide` layout.
- A reusable Panel can render at different widths in separate Page sections.
- Page title, description, and filters retain the existing wide alignment.
- Sequence Panels are unaffected.
- Safe gutters remain at every viewport width; `full` is not arbitrary full bleed.
- Schema, compiler diagnostics, AppIR compatibility, inspection/diff, restart, React, and browser geometry are covered.
- `make check` and `make build` pass.

## Non-goals

- Edge-to-edge media bleed, negative margins, background bands, or overlays.
- Arbitrary lengths, percentages, CSS classes, container queries, or author breakpoints.
- Width metadata on Panel or Sequence.
- Grid columns, item spans, nested Panels, or responsive reordering.
- Changes to reads, writes, SQL, SQLite, or migrations.
