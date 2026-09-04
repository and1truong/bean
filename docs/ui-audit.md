# Bean UI audit

This audit describes the frontend before the Bean Design System v2 migration. It is intentionally concise and repository-specific.

## Architecture

- `web/src/App.tsx` owns routing, the public metadata renderer, the global shell, View displays, generated Webforms, menus, charts, boards, trees, and public tables.
- `web/src/Admin.tsx`, `Explore.tsx`, `Studio.tsx`, and `Sequence.tsx` own the four first-party application modes.
- React Router 7 handles routes. TanStack React Query owns server state. Zustand stores only the Studio editor draft. No architecture replacement is needed.
- Tailwind CSS v4 and source-owned shadcn-style primitives live under `web/src/components/ui`. Radix supplies accessible dialog and checkbox behavior. Dynamic and multi-value choices intentionally use native selects.
- Lucide is installed, but application chrome is almost entirely text. Charts are lightweight semantic HTML bars; there is no charting library.

## Findings

### Foundations

- `style.css` exposes the shadcn compatibility variables but not a complete semantic vocabulary for canvas, surfaces, border strengths, text levels, feedback, and elevation.
- Accent and preset rules set raw colors independently. Dark utility variants exist in primitives, but no dark palette or theme activation exists.
- Radius is generous (`0.625rem`) and cards commonly use `rounded-xl`; borders, rings, muted fills, and spacing vary by component.
- Type hierarchy is local (`text-2xl`, `text-3xl`, `text-4xl`) rather than a documented application scale. Page titles are large relative to dense tool screens.

### Shell and navigation

- `Shell` is one full-width accent header. Tool routes have no persistent module navigation, active-route indication, workspace context, or compact utility area.
- Public workspace menus are semantically strong (`nav`, links, `aria-current`) and should be retained, but their styling is a separate tab-like system.
- Pages repeatedly recreate context with local breadcrumbs and headers.

### Pages and forms

- `Page`, `PageHeader`, `SectionCard`, and `Field` are useful beginnings, but most substantial content is wrapped in cards.
- Forms use native labels and controls, yet error text is not consistently connected with `aria-describedby`, and controls do not consistently receive `aria-invalid`.
- Long Studio and generated forms rely on repeated rounded bordered boxes. Destructive controls are often adjacent to ordinary configuration instead of structurally separated.
- Spacing uses many local `gap-*`, `p-*`, and `mt-*` choices rather than a small compositional scale.

### Tables and filters

- `components/ui/table.tsx` provides semantic low-level markup and horizontal overflow, but Admin, public Views, Explore previews, and System Admin each assemble their own table state.
- Headers are not sticky. Numeric values do not receive a standard alignment. Long-value truncation, result counts, empty/loading/error states, selection, sorting, and bulk actions are inconsistent.
- Filter forms repeat similar markup in `App.tsx`, `Admin.tsx`, and `Explore.tsx`. Active conditions and clear-all behavior are not visible as a consistent model.

### Studio and analytics

- Studio is a long vertical sequence of Definition editor, Release preview, and Definitions cards. It does not preserve workspace area or expose a navigator/editor/inspector mental model.
- Explore puts a large query form over a preview card. Query intent is sound, but visual grouping and preview persistence do not resemble a professional data workspace.
- Metrics and charts function, but styling is locally embedded and does not yet define an analytics surface language.

### States, accessibility, and responsiveness

- Loading is localized and dialogs use Radix focus management—both are good foundations.
- Empty and request-error states are ad hoc. Empty states frequently use centered whitespace or cards and do not consistently explain reason and next action.
- Focus rings are present. Tests cover semantic route navigation, native labels, dialogs, table headers, and mobile overflow.
- Narrow layouts generally stack safely, but the current shell consumes excess vertical space and Studio has no deliberate pane collapse model.

## Migration risks

- `App.tsx` contains multiple generic metadata renderers; visual changes must not alter request construction, route binding, Policy behavior, or Action invocation.
- Existing tests rely on accessible names and stable `data-testid` hooks. Preserve them.
- Application-specific presentation remains metadata under `examples/`; the design system must stay generic.
- Source-owned components are already the correct maintenance boundary. Add focused primitives rather than introducing another framework.
