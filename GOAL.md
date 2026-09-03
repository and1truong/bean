# Goal: Deterministic responsive Panel presets

Status: complete

Make every existing Panel layout produce a documented, predictable responsive composition without adding author-controlled CSS or changing application metadata.

## Design

This source-compatible stage keeps the existing closed `layout` field and AppIR unchanged. The browser maps it to runtime-owned viewport thresholds:

- small: below `48rem`;
- medium: `48rem` through below `64rem`;
- large: `64rem` and above.

Preset behavior is fixed:

| Layout | Small | Medium | Large |
| --- | --- | --- | --- |
| `single-column` | 1 column | 1 column | 1 column |
| `two-column` | stacked | 2 equal columns | 2 equal columns |
| `sidebar-main` | stacked | stacked | `1fr 2fr` |
| `main-sidebar` | stacked | stacked | `2fr 1fr` |
| `grid` | 1 item per row | 2 items per row | 3 items per row |

Regions and Blocks retain source/DOM order at every width. Tracks use `minmax(0, …)` and Regions use `min-width: 0` so wide Block content remains locally responsible for overflow. `grid` lays out the ordered children of its existing `main` Region.

Viewport queries are deliberate for this first stage because Panels are route-level compositions in the current runtime. Container queries and metadata-controlled sizing remain deferred until nested or constrained Panels provide evidence.

## Acceptance criteria

- Ordinary Page Panels implement all five preset mappings at the fixed thresholds.
- Presentation `two-column` behavior remains compatible.
- Source, DOM, keyboard, and screen-reader order never changes across breakpoints.
- No definition schema, AppIR, Policy, View, Action, SQL, or migration behavior changes.
- Focused React/CSS and browser evidence covers responsive layout and overflow containment.
- Canonical Panel documentation states the runtime contract.
- `make check` and `make build` pass.

## Non-goals

- Per-Panel breakpoints, columns, spans, gaps, widths, or responsive metadata.
- Breakpoint-specific visibility or visual reordering.
- Nested Panels, multiple layout bands, drawers, tabs, accordions, carousels, or arbitrary CSS.
