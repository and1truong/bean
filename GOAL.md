# Goal: Ordered multi-Panel Page sections

Status: complete

Allow one Page to compose multiple ordered Panels so authors can build full-width, sidebar, grid, and other layout bands without nested Panels or arbitrary layout code.

## Design

A Page may use either the legacy single-Panel field:

```yaml
panel: article_body
```

or ordered sections:

```yaml
sections:
  - {id: hero, panel: hero}
  - {id: body, panel: article_body}
  - {id: related, panel: related_grid}
  - {id: comments, panel: comments}
```

`panel` and `sections` are mutually exclusive. `sections` contains 1–32 Panel references and source order is render order. An optional machine `id` stabilizes section identity across nearby reorderings; otherwise identity derives from the Page and ordinal. Reusing a Panel in multiple sections is allowed. Legacy `panel` remains stored and rendered unchanged rather than being rewritten.

AppIR v10 stores ordered `PageSection` values. The compiler validates every reference and treats Blocks across all sections as Page members for filter targets. Rendering resolves Page context once, applies Page filters to every rendered section, checks each Panel Policy independently, omits denied sections, and hides the Page only when no section is visible. Page Policy remains the outer authority.

Bound View/Webform requests may resolve only Blocks belonging to the Page's declared Panels and must pass the containing Panel's Policy. Generated tests and LocalRegistration inspect all declared sections conservatively. No runtime YAML parsing is added.

## Acceptance criteria

- A Page renders 1–32 Panels in deterministic source order with stable internal section identities.
- Different responsive Panel presets can form successive layout bands.
- Legacy `panel` source and runtime behavior remain unchanged.
- Compiler schema, diagnostics, references, inspect/diff, and AppIR compatibility cover `sections`.
- Page filters can target named View Blocks in any declared section.
- Page and per-Panel Policy behavior fails closed, including bound Block requests.
- The tracker example demonstrates a single-column introduction followed by its existing two-column operational band.
- `make check` and `make build` pass.

## Non-goals

- Nested Panels, arbitrary depth, or cycle detection.
- Per-section CSS, widths, gaps, responsive overrides, conditional visibility, or interaction state.
- Shared View-result scopes across Blocks.
- Changes to View reads, Action writes, SQL, SQLite, or migrations.
