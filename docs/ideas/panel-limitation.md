# Panel limitations and difficult composition cases

Status: idea inventory; not scheduled or implemented.

Related: [`panel-responsive-layout.md`](panel-responsive-layout.md).

Panel currently provides a flat, compile-time composition of named Regions containing ordered Blocks or inline semantic content. Its layout vocabulary is closed and intentionally avoids arbitrary CSS. The following cases expose boundaries in that model. They are not all necessarily Panel responsibilities.

## 1. Multiple layout bands on one Page

A blog page may need several different layouts:

```text
┌──────────────────────────────────────┐
│ Hero — full width                    │
├──────────────────────────┬───────────┤
│ Article                   │ Sidebar   │
├──────────────────────────────────────┤
│ Related posts — 3-column grid        │
├──────────────────────────────────────┤
│ Comments — full width                │
└──────────────────────────────────────┘
```

One Panel currently has one layout. It cannot directly express a full-width hero, a main/sidebar body, a peer-card grid, and another full-width section in one composition.

## 2. Nested Panels

A useful structural model might be:

```text
Panel
├── header
├── body
│   ├── main
│   └── sidebar
└── footer
```

Regions currently contain Blocks or inline content, not another Panel. Supporting nested Panels would require explicit contracts for Policy inheritance, context binding, responsive collapse, cycle detection, identity, inspection, and maximum depth.

## 3. Unequal Block sizes and grid spans

Operational dashboards often need mixed spans:

```text
┌───────────────────────┬───────────┐
│ Revenue chart: 8 cols │ Metric: 4 │
├───────────┬───────────┴───────────┤
│ Metric: 3 │ Activity table: 9     │
└───────────┴───────────────────────┘
```

Panel metadata cannot currently assign column or row spans to a Region or item. A future contract would need bounded integers and deterministic placement rather than arbitrary grid CSS.

## 4. Contained, wide, and full-bleed content

A Page may need a constrained article body, a wider quotation or code sample, and an edge-to-edge hero image. Panel currently has no semantic width vocabulary such as:

```yaml
width: contained
width: wide
width: full-bleed
```

Maximum application-shell width should remain separate from Panel track sizing.

## 5. Sticky sidebar or header

A long post may keep its table of contents visible while the article scrolls. Panel cannot declare sticky positioning, offsets, or breakpoint behavior. A correct design must account for viewport height, nested overflow containers, sticky headers, and disabling sticky behavior on narrow screens.

## 6. Responsive interaction changes

Some mobile transformations are not grid collapse:

- a sidebar becomes a drawer;
- tabs become an accordion;
- navigation becomes a menu button;
- a table becomes cards;
- related content becomes a carousel.

These changes require state, keyboard semantics, and often a different Block renderer. They should not be treated as Panel sizing alone.

## 7. Breakpoint-specific visual order

Desktop and mobile designs sometimes request different ordering or splitting one Region across multiple positions. Visual order that differs from DOM order can break keyboard navigation, screen-reader reading order, and heading hierarchy. Arbitrary responsive reordering should probably remain unsupported; source order should stay authoritative.

## 8. Regions emptied by Policy

A sidebar may contain only editor-restricted Blocks. For an anonymous actor, every child can be removed by server-side Policy evaluation. The layout contract does not say whether the empty Region should retain its track, collapse, allow `main` to expand, or render a fallback.

A possible future semantic is:

```yaml
collapseWhenEmpty: true
```

Any decision must use the authorized server render tree, not client-only hiding.

## 9. Empty, loading, and error states affecting reflow

If one dashboard Block has no rows or fails, should its cell remain stable, collapse, or allow adjacent Blocks to expand? Panel currently knows composition, while empty/error behavior belongs to the Block or Display. Dynamic reflow can also create disruptive layout shift, so this boundary needs an explicit decision.

## 10. Multiple Blocks reading the same record

Authors may try to split one post into `post_title`, `post_teaser`, `post_metadata`, `post_content`, and `post_tags` Blocks to gain layout control. Each View Block may then execute the same detail View independently. Panel has no shared data scope or single-fetch/multiple-renderer contract.

This is more likely a View Display composition problem than a Panel problem.

## 11. Field layout inside a record

A detail or form screen may require:

```text
Title                    Status
Teaser                   Category
Content — full width
Tags                     Published date
```

Panel places Blocks; it does not place Entity fields. Turning every field into a Block would be the wrong abstraction. Ordered sections, field groups, and field spans belong in typed detail/form renderer metadata.

## 12. Dynamic repeated composition

A CMS may persist an ordered list of text, image, quote, and product-grid sections whose count and types are known only at runtime. Panel is static metadata compiled into immutable AppIR; record data cannot generate an arbitrary Block tree.

If supported, this should be a bounded typed collection/component renderer, not runtime interpretation of arbitrary Panel definitions.

## 13. Tabs, accordions, and master-detail

These patterns require more than spatial placement:

- URL-addressable active state;
- lazy data loading;
- keyboard behavior;
- conditional visibility;
- selection context shared between Blocks.

Panel currently has no interaction state. These may justify a typed interactive container or Block rather than expanding base Panel semantics.

## 14. Masonry and unpredictable heights

Masonry card layouts complicate source order, keyboard order, server rendering, hydration, print output, and layout stability. They should remain outside Panel core unless a strong maintained application demonstrates the need.

## 15. Overlay and layered composition

A hero with text and actions layered over an image requires aspect ratio, positioning, stacking, contrast, and responsive crop behavior. Adding arbitrary positioning to Panel would move it toward a freeform page builder. A typed, accessibility-aware `hero` Block renderer is a safer boundary.

## 16. Print-specific layout

Print may need to move a sidebar below content, remove interactive controls, avoid page breaks after headings, repeat table headers, or start sections on new pages. Panel currently has no print contract. Print presentation must never become a way to bypass Policy or change data visibility.

## 17. RTL and localization

Physical Region names such as `left` and `right` are ambiguous for right-to-left languages. Semantic names such as `main`, `sidebar`, `start`, and `end` may be more appropriate. Any migration must preserve source, reading, and focus order.

## 18. User-personalized dashboards

Users may want to hide, reorder, drag, or resize widgets and persist a personal layout. Panel is immutable application metadata shared by all users. Personalization would require a bounded preference layer constrained by the compiled Panel and existing Policies, not mutable or arbitrary definitions.

## Responsibility map

| Problem | Likely owner |
| --- | --- |
| Responsive columns, spans, and gaps | Panel |
| Multiple structural bands or nested sections | Panel composition |
| Field order, groups, and field spans | Display/Form renderer |
| Hero overlays | Typed Block renderer |
| Tabs and accordions | Typed interactive container or Block |
| Reusing one View result across presentation sections | View/Display composition |
| Dynamic database-backed sections | Typed collection renderer |
| User dashboard customization | Preference layer |
| Record and Block visibility | Policy and server render tree |
| Print presentation | Panel, Theme, and renderer contract |

## Candidate priorities

1. Complete deterministic responsive behavior for existing presets.
2. Support multiple layout bands on one Page without arbitrary nesting.
3. Add bounded main/sidebar spans and grid column counts if examples require them.
4. Define empty-Region collapse after Policy filtering.
5. Add semantic contained/wide/full-bleed widths.
6. Improve typed detail/form field layout instead of using one Block per field.

## Non-goals

- Arbitrary HTML, CSS, JavaScript, utility classes, or media queries in metadata.
- Freeform coordinates, layering, or a general-purpose visual page builder.
- Client-side layout rules that alter authorization or data visibility.
- Responsive visual reordering that changes accessible reading or focus order.
- Runtime parsing of source YAML outside the definition-to-AppIR lifecycle.
