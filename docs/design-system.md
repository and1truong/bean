# Bean Design System

Bean is a professional tool for building, inspecting, querying, visualizing, and managing structured information. Its interface should feel predictable, compact, and durable: **tool first, decoration second**.

The system is independently authored for Bean under the MIT license. Drupal Admin, Metabase, Linear, GitHub, VS Code, and Grafana informed broad UX principles only. No source, styles, components, icons, templates, or assets were copied from those products.

## Principles

1. **Structure over decoration.** Establish hierarchy with alignment, typography, borders, and surface contrast before adding containers or effects.
2. **Comfortable-compact by default.** Preserve scanability while fitting real data and configuration on a laptop screen.
3. **One meaning per treatment.** Primary, selected, destructive, disabled, and elevated states should not compete.
4. **Stable application anatomy.** Keep workspace and module navigation in predictable locations. Keep page context and actions above content.
5. **Semantic and native first.** Use headings, landmarks, forms, labels, tables, links, and native controls unless custom behavior creates genuine value.
6. **Metadata remains presentation intent.** Shared runtime components interpret typed metadata; application-specific styles do not enter core React code.

## Architecture

The canonical foundations are in `web/src/style.css`. Source-owned control primitives remain under `web/src/components/ui`. Bean-level compositions live in `web/src/components/bean.tsx`:

- `Page` and `PageHeader`
- `SectionCard` (a flat, border-led section despite its compatibility name)
- `Field`
- `Toolbar` and `FilterBar`
- `ActiveFilters`
- `DataTable`
- `EmptyState`, `LoadingState`, `ErrorAlert`, and `StatusAlert`
- `StatusIndicator`, `Spinner`, and `Divider`

Do not add a second styling system or a generic abstraction that hides application semantics. Add a primitive only when it eliminates a repeated inconsistency.

## Tokens

Use semantic tokens instead of raw component colors.

### Color

| Role | CSS token | Purpose |
| --- | --- | --- |
| Canvas | `--canvas` | application background |
| Surface | `--surface` | primary content and controls |
| Subtle surface | `--surface-subtle` | grouped or secondary regions |
| Hover surface | `--surface-hover` | immediate hover feedback |
| Selected surface | `--surface-selected` | current navigation and rows |
| Elevated surface | `--surface-elevated` | dialogs, menus, popovers |
| Border | `--border-color` | ordinary structural edges |
| Subtle border | `--border-subtle` | row and section separation |
| Strong border | `--border-strong` | controls and emphatic structure |
| Text | `--text-color` | primary content |
| Secondary text | `--text-secondary` | descriptions |
| Muted text | `--text-muted` | metadata and helper copy |
| Disabled text | `--text-disabled` | unavailable controls |
| Accent | `--accent-color` | primary actions and current context |
| Accent hover | `--accent-hover` | primary action hover |
| Accent subtle | `--accent-subtle` | quiet accent background |
| Feedback | `--success`, `--warning`, `--danger`, `--info` | semantic status only |
| Focus | `--focus-ring` | keyboard focus visibility |

Existing shadcn-compatible variables are aliases to these values. New Bean code should name the semantic role when possible. Metadata-selected accents may alter only the accent family, not arbitrary component colors.

### Light and dark

Light and dark modes are separate palettes. Dark mode uses dark gray layered surfaces, brighter status colors, stronger borders, and a lighter accent; it is not an inversion. The shell applies `.dark` and `data-theme="dark"`, preserving Tailwind dark variants and semantic variables. The header theme control stores `bean_color_mode` locally.

Check every new component in both modes. In particular, never assume white surfaces, black text, or translucent primary colors will retain contrast.

### Spacing

The compositional scale is deliberately small:

| Token | Value |
| --- | --- |
| `--space-1` | 4px |
| `--space-2` | 8px |
| `--space-3` | 12px |
| `--space-4` | 16px |
| `--space-5` | 24px |
| `--space-6` | 32px |

Use 4–8px within a control or tightly related group, 12–16px within a section, and 24–32px between page regions. Avoid one-off spacing values.

### Radius and elevation

- small elements: 3–4px;
- controls and standard containers: 4–6px;
- large radius requires an explicit design-system reason;
- circular radius is reserved for true circles and status dots.

Normal layout uses no shadow. Borders and surface changes define sections, tables, panes, and toolbars. `--shadow-overlay` is reserved for dialogs, menus, popovers, and floating overlays.

## Typography

Geist Variable is the application family with system fallbacks.

| Role | Treatment |
| --- | --- |
| Page title | 24px / 1.25, 650 weight, tight tracking |
| Section heading | 16px / 1.4, 650 |
| Panel heading | 13px / 1.4, 650 |
| Body and table | 14px |
| Label | 13px, medium |
| Metadata/helper | 12px |
| Code/value | 13px monospace, tabular numerals |

Headings establish structure without consuming workspace. Presentation Sequences intentionally retain a separate large-display scale.

## Application shell

Authenticated tools use this hierarchy:

`Application → workspace → module → current view → contextual actions`

The top bar contains product/workspace identity and account/theme utilities. A stable sidebar contains Application, Admin, Explore, and Studio according to authorization, with a semantic current-page state. At narrow widths the same navigation becomes a horizontal module strip; routes are not duplicated in the DOM.

Public metadata-driven workspace menus remain semantic route navigation, not ARIA tabs. Use `aria-current` for the active destination.

## Page anatomy

A standard page contains:

1. breadcrumbs or context when needed;
2. `PageHeader` with compact title, optional description, primary action, and secondary actions;
3. a toolbar for search, filters, view controls, and bulk state;
4. content such as a table, editor, visualization, dashboard, or canvas;
5. an optional inspector or secondary pane;
6. optional paging/status below the local content region.

Do not nest every region in a card. Use `SectionCard` for one genuinely bounded section; use separators and headings inside long forms.

## Controls and interaction

Controls default to 32px high with restrained 4px radius. Small and icon controls are available through the shared `Button` API. Reuse the existing Button, Input, Textarea, NativeSelect, Checkbox, Label, Badge, Alert, RouteTabs, and AlertDialog primitives.

Every interactive component must define:

- hover, pressed, selected, disabled, loading, and `focus-visible` behavior as applicable;
- immediate feedback with transitions no longer than 100ms;
- no motion required to understand state;
- a usable accessible name, including icon-only controls;
- no disabled action that looks active.

Use Link or a link-backed Button for navigation and Button for operations. Do not make a non-interactive container clickable.

## Forms

`Field` establishes label, control, description, required marker, and validation order. It wires helper/error IDs to a single child control and sets `aria-invalid` for server validation errors.

- Keep labels directly above controls.
- Explain format or consequence in concise helper text.
- Use native `required`, `readonly`, and `disabled` behavior.
- Use sections such as General, Data, Display, Behavior, and Advanced for long forms.
- Prefer separators and `fieldset`/`legend` over nested cards.
- Advanced JSON or destructive operations must be visibly distinct, not visually dominant.
- Place destructive actions away from the primary save flow and require confirmation where loss is irreversible.

## Data tables

`DataTable` owns the bordered table region, result count, and selected-row status. The low-level semantic `Table` primitives own markup.

Defaults:

- 32px sticky column headers and 36px rows;
- subtle row separators and hover fill;
- selected rows use `surface-selected`;
- numbers align right with tabular numerals;
- text aligns left;
- long values truncate only when the full value is available via `title` or another reveal;
- horizontal overflow stays within the table region, never the page;
- sort controls remain buttons inside `<th>` and `<th aria-sort>` communicates state;
- selection uses labelled checkboxes and announces selected count;
- bulk actions appear next to the table, only after selection;
- loading, errors, and empty results stay localized to the table region;
- paging follows the table and keeps unavailable directions disabled.

Resizable columns are intentionally deferred until a persisted width model and pointer/keyboard behavior can be added cleanly. Do not fake resizing with CSS-only handles.

## Filters

`FilterBar` groups search, field controls, and the apply action. `ActiveFilters` states each active condition as **field + value**, provides an individually labelled remove action, exposes the count, and offers Clear all.

- Keep common filters inline.
- Do not open a large filter panel for one to three controls.
- A future operator builder should display field, operator, and value together.
- URL-backed page/View filters remain shareable and are authoritative after Apply.
- Saved views can later persist the same typed filter state; do not encode visual CSS in saved state.

## Studio

Studio uses professional creation-tool anatomy:

- **toolbar/header:** release status and validate/publish actions;
- **navigator:** typed definitions and current selection;
- **workspace:** identity controls plus the visual or advanced editor;
- **inspector:** release explanation, migration preview, diagnostics, and active release.

The center workspace receives the largest share of width. Navigator and inspector use subtle surface differentiation and separators rather than cards. Below 1120px the inspector moves below the workspace; below 768px the navigator becomes a bounded region above the editor so existing definitions remain reachable without consuming the whole screen. Actions are grouped by lifecycle: Save draft in the definition toolbar, Validate and Publish in release context.

## Analytics and Explore

Explore separates query definition from a persistent result preview on wide screens and stacks them in source order on narrow screens. This establishes room for future metrics, charts, pivots, dimensions, measures, drill-down, grouping, aggregation, date ranges, dashboards, and saved queries without creating decorative dashboard cards.

Metrics may be visually prominent because the value is the content. Charts use semantic colors and accessible names. Tables remain the detailed fallback for data inspection.

## States

- **Empty:** state the reason, a concise next action, and optional short explanation. Do not use giant illustrations.
- **Loading:** use localized skeletons or a small spinner. Do not block unrelated page regions.
- **Validation error:** connect text to the field and set `aria-invalid`.
- **Request/system error:** use an alert with actionable language and preserve the affected region.
- **Permission/missing data:** explain the condition without pretending it is an empty successful result.

## Accessibility

Target WCAG 2.2 AA where practical.

- Preserve visible 2px focus treatment and logical DOM/focus order.
- Use landmarks and a single contextual page `<h1>`.
- Keep native table, form, button, and link semantics.
- Radix AlertDialog provides modal semantics and focus handling; do not recreate it locally.
- Associate every field description and validation message.
- Use `aria-current` for routes, `aria-sort` for table sort state, and live/status regions for asynchronous state.
- Never convey status by color alone.
- Respect `prefers-reduced-motion`.
- Verify light/dark contrast and 200% zoom behavior.

## Responsive behavior

Desktop productivity is primary. The system must still support narrow laptops and small screens:

- tool navigation becomes horizontal before content;
- Studio panes collapse in source order;
- Explore moves preview below the query;
- grids become single column;
- tables scroll inside their own bounded region;
- dialogs fit within viewport height and width;
- the document itself must not gain accidental horizontal overflow.

## Do / don't

**Do**

- reuse semantic tokens and shared controls;
- prefer structure, source order, and borders;
- use compact, descriptive headings;
- align fields and columns precisely;
- keep secondary actions quiet;
- test both themes and keyboard interaction.

**Don't**

- introduce arbitrary colors or one-off spacing values;
- create local button styles;
- wrap everything in `Card`;
- use large radius without design-system justification;
- implement new form controls without reusing shared primitives;
- use gradients, glassmorphism, decorative shadows, floating pills, or landing-page scale in application screens;
- copy implementation or assets from design-reference products.

## Remaining migration notes

The core representative paths are migrated. Follow-up work should remain incremental:

- move remaining board and calendar card collections toward compact list/panel treatments where their metadata model permits;
- add typed status mappings instead of rendering all queue statuses as neutral outline badges;
- add a reusable operator/value filter builder when multiple operators become runtime-configurable;
- evaluate accessible persisted table column widths before adding resize behavior;
- consolidate repeated Studio collection-row markup after its authoring schemas stabilize;
- add visual-regression baselines only when the repository adopts a stable screenshot review workflow.
