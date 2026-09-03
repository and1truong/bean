# Goal: Modernize Asana Lite composition

Status: complete

Update the maintained metadata-only Asana Lite reference application to exercise the current Panel and View Display contracts without changing its accepted project, task, hierarchy, status, or attachment behavior.

## Design

- Replace each legacy single-Panel Page with ordered `sections` that compose focused responsive layout bands.
- Use `two-column` only where paired content benefits from it and keep the wide board/tree/list surfaces in `single-column` Panels.
- Preserve source, DOM, keyboard, and screen-reader order across responsive breakpoints.
- Move one-off Page narrative from globally named text Blocks into ordered inline Panel content.
- Move legacy `Block.presentation` metadata into named View-owned block Displays; Blocks retain only View/Display selection, route-context bindings, and Page composition.
- Preserve the project Page's explicit priority-filter fan-out across chart, board, and tree Blocks in separate sections.
- Keep all application behavior in `examples/asana`; use only already-shipped generic capabilities.

## Acceptance criteria

- Home composes a single-column introduction followed by a responsive two-column project list/create band.
- Project composes ordered header, overview, board, and hierarchy bands; overview becomes two columns at the existing medium breakpoint while board and tree remain full width.
- Task composes ordered header, responsive two-column action, and full-width attachment bands.
- One-off `home_intro`, `project_help`, and `task_help` text Blocks are removed in favor of inline semantic content.
- Project list/detail, task board/tree/detail, and attachment list presentations are named Displays owned by their Views.
- Existing anonymous creation, immutable route bindings, priority filtering, board movement, arbitrary-depth hierarchy, and upload/download behavior remain green.
- Browser evidence proves ordered sections and Asana's real responsive two-column collapse/expansion.
- `make check` and `make build` pass.

## Non-goals

- New Panel, Display, View, Action, Lifecycle, Rule, file, or database capabilities.
- Webform cache invalidation or removal of the existing refresh workflow.
- Direct-child task lists, sibling reorder, drag-and-drop, attachment deletion, authentication, or registration.
- Changes to SQL, SQLite, PostgreSQL, migrations, or immutable AppIR formats.
