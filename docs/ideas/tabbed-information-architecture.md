# Tabbed information architecture for dense Pages

Status: idea; no metadata or runtime contract is scheduled.

Related: [`panel-limitation.md`](panel-limitation.md).

## Problem

Information-dense applications often preserve the feeling of one Page by organizing content into three bounded navigation levels:

1. primary tabs for major areas;
2. optional secondary tabs for sub-areas;
3. vertical tabs for focused groups inside the selected area.

The selected leaf then contains the actual Page content. This is not only spatial layout: it introduces URL state, branch selection, lazy loading, Policy filtering, focus management, keyboard interaction, and responsive behavior.

## What Bean already has

### Flat route navigation

A `Menu` contains a flat ordered list of labelled routes with optional item Policies. A `menu` Block filters denied items on the server and renders links through the React application shell. Several Pages can therefore resemble primary navigation without a browser reload.

This remains route navigation between separate Page definitions. Menu has no hierarchy, stable item identity, orientation, active-item contract, or tab semantics.

### Page content composition

A Page can render 1–32 ordered `sections`, each referencing a reusable Panel. Sections preserve source, DOM, keyboard, and screen-reader order and support `contained`, `wide`, and `full` placement widths. Panels provide deterministic responsive presets; Regions contain ordered named Blocks and inline semantic content; opted-in policy-empty Regions can collapse.

These are strong primitives for the content of a selected tab pane. Today every Page section is rendered in order; sections cannot be grouped into selectable branches.

### Specialized URL-state precedents

View Blocks can switch among compatible Displays through a block-scoped URL parameter. This only changes presentation of the same View and currently uses a native select; it cannot select arbitrary Panels or section groups.

Sequences have URL-addressable active frames and keyboard navigation, but they are presentation-specific and include slide, aspect-ratio, notes, and print semantics. They must not become a parallel application-tabs runtime.

### Server-side authority

Page, Panel, Block, and Menu item Policies are evaluated on the server. Page context and filters are resolved centrally, while View reads and Action writes retain their existing paths. Any future tab contract can reuse these authorities without client-side authorization logic.

## Capability gap

Bean has the content-pane composition machinery but no typed interactive navigation Module.

| Desired behavior | Closest current capability | Missing contract |
| --- | --- | --- |
| Primary tabs | Flat Menu links to Page routes | In-Page active branch and tab identity |
| Optional secondary tabs | Another Menu Block | Parent-scoped child navigation |
| Vertical tabs | `sidebar-main` Panel layout | Selection, orientation, and tab semantics |
| Tab pane content | Ordered Page sections | Grouping sections under a selectable leaf |
| Deep links | Route, Page filter, View Display, and Sequence parameters | One normalized Page navigation state |
| Authorization | Server-side Page/Panel/Block/Menu Policy | Filtering branches and deterministic allowed fallback |
| Lazy loading | View queries are independently cached | Rendering and fetching only the active leaf |
| Accessibility | Stable source order and ordinary navigation | Tablist/tabpanel relationships, roving focus, and keyboard rules |
| Responsive behavior | Runtime-owned Panel breakpoints | Bounded transformation of horizontal and vertical navigation |
| Authoring | Studio Page section editor | Navigation hierarchy editor and repair diagnostics |

## Likely ownership

This behavior should not expand Panel. Panel owns spatial placement; tabs own interaction state and branch selection.

For the stated use case, the likely owner is a typed Page navigation Module whose leaf panes contain existing ordered Page sections. This creates one seam for:

- stable navigation identities;
- URL normalization;
- server-authoritative branch availability and fallback;
- active-leaf render-tree selection;
- accessible React behavior;
- deterministic responsive adaptation.

Panels, Regions, Blocks, Views, Actions, and Policies remain unchanged behind that seam.

## Illustrative shape

The following only demonstrates ownership and is not an accepted schema:

```yaml
kind: Page
name: customer_workspace
route: /customers/:id
workspace:
  primary:
    - id: overview
      label: Overview
      vertical:
        - id: summary
          label: Summary
          sections:
            - {panel: customer_summary, width: contained}
        - id: metrics
          label: Metrics
          sections:
            - {panel: customer_metrics, width: wide}

    - id: operations
      label: Operations
      secondary:
        - id: orders
          label: Orders
          vertical:
            - id: active
              label: Active
              sections:
                - {panel: active_orders, width: full}
            - id: history
              label: History
              sections:
                - {panel: order_history, width: full}
```

A final schema should avoid irregular optional nesting where possible. It must not turn Page into a recursively nestable page builder.

## Required behavior

### Stable and URL-addressable selection

Each navigation item needs a stable machine ID and author-facing label. The URL must identify the active primary, secondary, and vertical path so reload, sharing, browser history, and direct entry are deterministic.

The contract must define:

- default selection at every level;
- behavior for unknown IDs;
- whether changing a parent resets or restores child selection;
- whether navigation updates push or replace browser history;
- interaction between tab parameters and existing Page filters;
- parameter names that cannot collide with Block controls.

### Server-authoritative Policy handling

The server must omit unavailable branches before sending navigation descriptors. If the requested branch is denied or has no surviving Panel content, selection must fall back deterministically to the first allowed leaf. A Page with no allowed leaf remains unavailable.

The client must never decide authorization. It should receive the visible navigation model and active leaf, then manage only URL and interaction behavior.

### Active-leaf rendering

Rendering every authorized pane and hiding inactive panes would still mount their View Blocks, trigger unnecessary reads, increase payload size, and retain inactive forms. The preferred contract returns navigation descriptors plus only the selected leaf's render tree.

Declared composition should remain authoritative for inspection, generated checks, Page-filter validation, and bound View/Webform authorization even when a leaf is inactive.

### Accessibility

True in-Page tabs require more than tab-shaped links:

- `tablist`, `tab`, and `tabpanel` relationships;
- stable `aria-controls` and `aria-labelledby` identities;
- one tab stop per tablist through roving `tabIndex`;
- Left/Right navigation for horizontal levels;
- Up/Down navigation for vertical levels;
- Home/End behavior;
- defined automatic or manual activation;
- predictable focus after Policy-driven fallback or responsive transformation.

Route links visually styled as tabs should remain semantic navigation with `aria-current`, not be mislabeled as an ARIA tablist.

### Responsive behavior

Responsive behavior must be runtime-owned and must preserve logical order. Candidate defaults are horizontally scrollable primary/secondary navigation and a vertical list that becomes a select or accessible accordion on narrow screens. The exact transformation requires browser and assistive-technology evidence before becoming a contract.

No author-controlled breakpoints or responsive reordering should be introduced.

## Required bounds

A future compiler contract must bound complexity, including:

- a closed maximum depth of primary → optional secondary → vertical;
- maximum items per level;
- maximum total leaf panes;
- maximum ordered sections per leaf;
- unique stable IDs within defined scopes;
- no recursive tabs;
- no nested Panels;
- no database-generated tab definitions.

Concrete limits should be justified by a maintained executable example rather than selected speculatively.

## Delivery surface

A complete capability would require the normal lifecycle:

1. definition and schema vocabulary;
2. exact compiler validation and diagnostics;
3. immutable AppIR storage and backward compatibility;
4. inspection, references, semantic diff, and capabilities;
5. server-side visible-navigation and active-leaf resolution;
6. React primitives and URL state;
7. Studio authoring;
8. restart, package, Policy, accessibility, and browser coverage.

It should require no SQL, SQLite, migration, View-read, or Action-write changes.

## Open decisions

1. Are primary items true in-Page tabs, or route navigation styled as tabs under a persistent shell?
2. Does each leaf own one Panel or ordered Page sections?
3. Should URL changes push history for every level?
4. When returning to a parent tab, should its previous child selection be restored?
5. Should inactive but authorized Blocks remain valid bound-request targets?
6. What maintained application demonstrates all three levels without creating avoidable navigation overload?
7. What is the deterministic mobile representation for vertical tabs?

## Non-goals

- Arbitrary recursive containers or a freeform page builder.
- Client-side Policy or visibility evaluation.
- Repurposing Sequence as an application navigation runtime.
- Rendering every inactive pane merely to hide it with CSS.
- Arbitrary HTML, CSS, JavaScript, breakpoints, or visual reordering in metadata.
- Dynamic database-backed tab definitions.
- Changes to View reads, Action writes, SQL, or migrations.
