# Goal: Policy-aware collapsible Panel Regions

Status: complete

Allow authors to opt a Panel Region into collapsing when authorization removes every rendered child, preventing empty sidebar or column tracks without moving Policy decisions into the browser.

## Design

```yaml
kind: Panel
name: article
layout: sidebar-main
regions:
  - {name: sidebar, blocks: [editor_tools], collapseWhenEmpty: true}
  - {name: main, blocks: [article_body]}
```

`collapseWhenEmpty` is an optional Region boolean and defaults to `false` for full compatibility. A Region is empty only when its server-authorized render tree contains zero Block nodes. It does not collapse merely because a View returns no rows, is loading, or presents an error state.

The Panel renderer evaluates Blocks and their Policies in source order. An empty opted-in Region is omitted. If one Region remains after another collapses, the server marks it expanded and the runtime spans it across every Panel track. If every Region collapses, the Panel is unavailable; enclosing Pages and Sequences retain their existing visible-child behavior. Errors and unresolved references still fail rather than collapsing.

The field is stored in immutable AppIR v11. AppIR v10 Page sections and all earlier formats remain loadable; only v11 may contain `collapseWhenEmpty: true`. Bound View/Webform authorization remains based on declared composition and does not infer access from visual collapse.

## Acceptance criteria

- `collapseWhenEmpty: true` omits a Region with zero authorized children.
- The sole remaining Region spans the full Panel width at every responsive breakpoint.
- `collapseWhenEmpty` omitted or `false` preserves existing empty Region tracks.
- A Panel with all Regions collapsed is unavailable to Page and Sequence composition.
- Block errors are surfaced and never mistaken for empty authorization output.
- Source, DOM, keyboard, and screen-reader order of surviving Regions is unchanged.
- Schema, AppIR compatibility, compiler, semantic diff, server render, React/CSS, and browser coverage are deterministic.
- `make check` and `make build` pass.

## Non-goals

- Collapsing a Block because its View query returns zero rows.
- Client-side Policy evaluation or visibility decisions.
- Fallback Blocks, Region reordering, animation, sticky behavior, spans, arbitrary CSS, or custom breakpoints.
- Changes to reads, writes, SQL, SQLite, or migrations.
