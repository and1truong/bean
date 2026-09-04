# Remaining Panel and composition limitations

Status: idea inventory; all entries below are unscheduled.

Related: [`panel-bounded-sizing.md`](panel-bounded-sizing.md) and [`menu-navigation.md`](menu-navigation.md).

Panel provides flat, compile-time composition of named Regions containing ordered Blocks or inline semantic content. Its closed layout vocabulary already supports deterministic responsive presets, ordered Page layout bands, semantic Page section widths, and policy-aware empty-Region collapse. The remaining cases below expose boundaries that may belong to Panel or to another typed Module.

## 1. Nested Panels

A useful structural model might be:

```text
Panel
├── header
├── body
│   ├── main
│   └── sidebar
└── footer
```

Regions currently contain Blocks or inline content, not another Panel. Supporting nested Panels would require explicit contracts for Policy inheritance, context binding, responsive collapse, cycle detection, identity, inspection, and maximum depth. Ordered Page sections should remain the preferred solution for successive layout bands.

## 2. Unequal Block sizes and grid spans

Operational dashboards may need mixed spans:

```text
┌───────────────────────┬───────────┐
│ Revenue chart: 8 cols │ Metric: 4 │
├───────────┬───────────┴───────────┤
│ Metric: 3 │ Activity table: 9     │
└───────────┴───────────────────────┘
```

Panel cannot assign column or row spans to a Region or item. A future contract would need bounded integers and deterministic placement rather than arbitrary grid CSS. See [`panel-bounded-sizing.md`](panel-bounded-sizing.md).

## 3. Sticky sidebar or header

A long Page may keep its table of contents visible while the article scrolls. Panel cannot declare sticky positioning, offsets, or breakpoint behavior. A correct design must account for viewport height, nested overflow containers, sticky headers, and disabling sticky behavior on narrow screens.

## 4. Responsive interaction changes

Some mobile transformations are not grid collapse:

- a sidebar becomes a drawer;
- tabs become an accordion;
- navigation becomes a menu button;
- a table becomes cards;
- related content becomes a carousel.

These changes require state, keyboard semantics, and often a different Block renderer. They should not be treated as Panel sizing alone.

## 5. Empty, loading, and error states affecting reflow

If one dashboard Block has no rows or fails, should its cell remain stable, collapse, or allow adjacent Blocks to expand? Panel knows static composition and server-authorized Block presence, while result emptiness and errors belong to the Block or Display. Dynamic reflow can create disruptive layout shift, so this requires an explicit typed contract.

## 6. Multiple Blocks reading the same record

Authors may split one record into `post_title`, `post_teaser`, `post_metadata`, `post_content`, and `post_tags` Blocks to gain layout control. Each View Block may then execute the same detail View independently. Panel has no shared data scope or single-fetch/multiple-renderer contract.

This is more likely a View Display composition problem than a Panel problem.

## 7. Field layout inside a record

A detail or form screen may require:

```text
Title                    Status
Teaser                   Category
Content — full width
Tags                     Published date
```

Panel places Blocks; it does not place Entity fields. Turning every field into a Block would be the wrong abstraction. Ordered groups and field spans belong in typed detail/form renderer metadata.

## 8. Dynamic repeated composition

A CMS may persist an ordered list of text, image, quote, and product-grid sections whose count and types are known only at runtime. Panel is static metadata compiled into immutable AppIR; record data cannot generate an arbitrary Block tree.

If supported, this should be a bounded typed collection renderer, not runtime interpretation of arbitrary Panel definitions.

## 9. Tabs, accordions, and master-detail

These patterns require URL-addressable active state, lazy data loading, keyboard behavior, conditional visibility, and selection context shared between Blocks. Panel has no interaction state.

The three-level primary, optional secondary, and vertical navigation case is explored in [`menu-navigation.md`](menu-navigation.md). Its likely owner is the route-backed Menu Module rather than Panel.

## 10. Masonry and unpredictable heights

Masonry card layouts complicate source order, keyboard order, server rendering, hydration, print output, and layout stability. They should remain outside Panel core unless a maintained application demonstrates the need.

## 11. Overlay and layered composition

A hero with text and Actions layered over an image requires aspect ratio, positioning, stacking, contrast, and responsive crop behavior. Adding arbitrary positioning to Panel would move it toward a freeform page builder. A typed accessibility-aware hero renderer is a safer owner.

## 12. Print-specific layout

Print may need to move a sidebar below content, remove interactive controls, avoid page breaks after headings, repeat table headers, or start sections on new pages. Panel has no general print contract. Print presentation must never bypass Policy or change data visibility.

## 13. RTL and localization

Physical Region names such as `left` and `right` are ambiguous for right-to-left languages. Semantic names such as `main`, `sidebar`, `start`, and `end` may be more appropriate. Any migration must preserve source, reading, and focus order.

## 14. User-personalized dashboards

Users may want to hide, reorder, drag, or resize widgets and persist a personal layout. Panel is immutable application metadata shared by all users. Personalization would require a bounded preference layer constrained by compiled Panel composition and existing Policies.

## Responsibility map

| Problem | Likely owner |
| --- | --- |
| Responsive columns, spans, and gaps | Panel |
| Nested structural composition | Panel composition, only with demonstrated need |
| Field order, groups, and spans | Display/Form renderer |
| Hero overlays | Typed Block renderer |
| Route-backed primary/secondary/vertical navigation | Menu Module |
| In-Page tabs and accordions | Separate typed interactive Module, only with demonstrated need |
| Reusing one View result across presentation sections | View/Display composition |
| Dynamic database-backed sections | Typed collection renderer |
| User dashboard customization | Preference layer |
| Record and Block visibility | Policy and server render tree |
| Print presentation | Panel, Theme, and renderer contract |

## Potential follow-ups

1. Deepen Menu with typed static and record-backed targets for primary, optional secondary, and vertical route navigation.
2. Add bounded Panel track counts and spans only if a maintained example requires them.
3. Improve typed detail/form field layout instead of using one Block per field.
4. Define a shared-result View Display composition contract if repeated reads become measurable.

## Deliberate boundaries

- No arbitrary HTML, CSS, JavaScript, utility classes, or media queries in metadata.
- No freeform coordinates, layering, or general-purpose visual page builder.
- No client-side layout rules that alter authorization or data visibility.
- No responsive visual reordering that changes reading or focus order.
- No runtime parsing of source YAML outside the definition-to-AppIR lifecycle.
