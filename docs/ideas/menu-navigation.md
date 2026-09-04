# Idea: Dynamic hierarchical Menu navigation

Status: idea; core direction and initial placement semantics agreed, implementation not scheduled.

Related: [`panel-limitation.md`](panel-limitation.md).

## Goal

Present dense applications as one continuous workspace through route-backed navigation with up to three visual levels:

1. primary navigation for major areas;
2. optional secondary navigation for sub-areas;
3. vertical navigation for focused children.

Navigation targets may be static Page definitions, routable View Displays, or individual Entity records. Applications may define multiple global or record-scoped Menus. The React application shell remains mounted while route content changes, so the experience can remain continuous without introducing an in-Page tabs runtime.

These controls are semantic route navigation. They use links, `nav`, and `aria-current`; tab-shaped styling must not mislabel them as an ARIA tablist.

## Existing capability

Bean already has a static `Menu` definition containing flat `{label, route, policy}` items and a `menu` Block that filters denied items on the server. It has no hierarchy, stable placement identity, active trail, contextual Menu instances, typed targets, record-owned placements, weight, or generated record editor.

Page and View Page Displays already own routes. Entity records do not inherently own routes, but an Entity can opt into a compiled navigation destination through a View Page Display. All target record reads remain View-owned and all placement writes remain Action-owned.

## Domain model

### Menu definition

A Menu definition owns presentation, depth, optional owner scope, and limits:

```yaml
kind: Menu
name: book_contents
profile: workspace
maxDepth: 3
owner:
  entity: book
```

An unscoped Menu has one global instance:

```yaml
kind: Menu
name: main_navigation
profile: workspace
maxDepth: 3
```

`profile: workspace` maps hierarchy depth to runtime-owned presentation:

- level 1: horizontal primary navigation;
- level 2: horizontal secondary navigation under the active primary item;
- level 3: vertical navigation under the active secondary item;
- narrow viewport: level 3 becomes one labelled native select followed by the active route content.

The profile must not expose CSS classes, arbitrary orientations, or author breakpoints.

### Logical Menu instance

A scoped Menu has one logical instance per owner record. Its stable key is:

```text
(menu definition name, owner record ID)
```

Examples:

```text
(book_contents, book-123)
(book_contents, book-456)
(main_navigation, empty-owner)
```

A Menu instance is derived and has no separately persisted record. Creating a Book implicitly makes its `book_contents` instance addressable. Deleting the owner removes all dynamic placements in that instance atomically.

### Navigation target

A navigation target is a closed typed union:

```text
Page
View Page Display
Entity record with a compiled destination
legacy internal route
```

The target, not the placement, owns the canonical route and default label. Raw user-entered URLs are not part of the first dynamic record contract.

### Menu placement

A placement puts one target at one position in one Menu instance:

```text
ID
Menu definition name
Owner record ID, when scoped
Typed target
Parent placement ID, optional
Weight
Label override, optional
```

A placement with no parent is level 1. Every child derives its level from its parent chain. Authors do not set `level` directly because level alone cannot identify the containing branch.

## Agreed placement semantics

The following decisions are accepted for the initial design:

1. A target may appear in multiple Menu instances but at most once in the same Menu instance.
2. The target supplies its default label; each placement may override that label.
3. Deleting a target Entity record removes all of its dynamic placements atomically. Broken links are not retained.
4. Deleting a Menu owner record removes every dynamic placement in that logical Menu instance atomically.
5. Menu instances are derived from `(Menu definition, owner record ID)` and are not separate records.

## Hierarchy and ordering

- Parent and child placements must belong to the same Menu instance.
- Maximum depth is compiler/runtime bounded, initially no more than three.
- A placement cannot parent itself or one of its ancestors.
- Siblings sort by ascending `(weight, placement ID)`; a heavier weight therefore sinks.
- Default weight is `0` and the accepted integer range must be bounded.
- Stable placement ID breaks weight ties and prevents label or route changes from reordering siblings.
- A Policy-denied parent suppresses its complete subtree without promoting descendants.
- The active trail is derived from the resolved current target and includes its visible ancestors.

Removing or reparenting a placement that currently has children still needs an explicit contract; silent orphaning is forbidden.

## Typed targets

### Static Page target

A Page may contribute one or more static placements:

```yaml
kind: Page
name: recruiting_activity
route: /recruiting/activity
navigation:
  - menu: main_navigation
    id: activity
    label: Activity
    parent: recruiting
    weight: 30
```

The Page editor may expose “Add to menu,” parent, weight, and label override controls. Compilation normalizes the contribution into the owning Menu's immutable static placement tree.

A Page with route parameters requires compiler-validated bindings before it can be a static target in a scoped Menu.

### Static View Display target

Placement belongs to a routable Display, not to the View as a whole:

```yaml
kind: View
name: candidates
displays:
  directory:
    type: page
    route: /candidates
    navigation:
      - menu: main_navigation
        id: candidates
        label: Candidates
        parent: recruiting
        weight: 20
```

Only `type: page` Displays are eligible. Block Displays have no independent route and cannot be navigation targets.

### Entity record target

An Entity must opt into record navigation and declare how the runtime obtains a label and destination:

```yaml
kind: Entity
name: book_page
label: Book page
navigation:
  labelField: title
  destination:
    view: book_pages
    display: detail
  menus: [book_contents]
```

The destination Display must be a View-owned Page Display for the same Entity and must have compiler-resolvable bindings for its route parameters. Record navigation resolution uses that View, preserving record Policy and redaction.

The record editor then exposes a Navigation section for each placement:

```text
Menu:           Book contents
Book:           Building Bean
Parent:         Chapter 2
Weight:         30
Label override: optional
```

One record may have placements in several Books. Parent, weight, and label override are independent in each Menu instance.

## Route resolution

“Arbitrary route” means the route may come from any supported typed target, not that a record editor accepts an unvalidated URL string.

- Page targets resolve the referenced Page route.
- View Display targets resolve the referenced Page Display route.
- Entity record targets resolve the destination declared by that Entity and bind route parameters from the authorized record result.
- Existing literal internal-route Menu items remain loadable for compatibility.

The runtime should resolve routes when producing the Menu tree rather than persisting expanded paths. A route or slug contract can then change without leaving stale placement data.

External URLs, protocol selection, new-window behavior, and user-supplied raw paths require a separate capability and security contract.

## Static and dynamic placement adapters

The Menu Module has one output Interface and at least two real Adapters:

1. a compiled Adapter for Page, View Display, and legacy static Menu items;
2. a record Adapter for Action-managed Entity record placements.

Both normalize into one authorized navigation tree:

```text
NavigationTree
├── item
│   ├── placement ID
│   ├── label
│   ├── resolved internal route
│   ├── active state
│   └── children
└── item
```

React does not need to know which Adapter produced an item. This seam concentrates hierarchy, sorting, active-trail, Policy, and responsive behavior in one deep Module.

## Menu rendering

The existing `menu` Block is the likely initial rendering seam. A scoped Menu Block binds its owner through ordinary typed Block inputs:

```yaml
kind: Block
name: book_contents_navigation
type: menu
menu: book_contents
inputs:
  owner_id: {type: uuid, required: true}
bindings:
  owner_id: {source: context, name: book_id, required: true}
```

A Page explicitly places the Block where navigation belongs. This supports several contextual Menus without introducing hidden shell behavior. A future Theme-level global Menu slot should be considered separately only if repeating one Menu Block across Pages becomes measurable friction.

The rendered navigation uses route links and ordinary browser history. It does not mount inactive Page trees, fetch inactive Blocks, remember hidden child state, or require tab-specific bound-request authorization.

## Policy behavior

- Menu owner visibility is checked before resolving a scoped Menu instance.
- Static target Page/View Policies remain authoritative.
- Entity record targets are resolved through their declared View and record Policy.
- A placement may add a stricter Policy but cannot weaken its target Policy.
- Denied targets and their descendant placements are omitted on the server.
- If the active target is denied or deleted, it cannot appear in the tree; ordinary route handling determines whether the current destination is unavailable.
- The client receives only authorized labels, routes, hierarchy, and active state.

Menu visibility never replaces target authorization. Knowing or constructing a route must not bypass Page, View, Entity, record, or Block Policy.

## Read and write lifecycle

### Reads

- Dynamic target records are read through their declared Views.
- Parent choices shown in an editor come from the authorized Menu tree for the selected instance.
- Menu rendering receives bounded server-produced trees rather than direct storage access.
- Target labels and route fields are redacted consistently with View output.

### Writes

- Add, remove, reparent, reorder, and label override operations execute through Actions.
- Updating an Entity record and its submitted navigation placements should commit atomically.
- Optimistic concurrency must prevent two edits from silently overwriting placement order or parent.
- Target deletion and owner deletion perform the agreed placement cleanup in the same transaction.
- Clients cannot choose storage table names, owner IDs outside authorized instances, target types not declared by metadata, or cyclic parent chains.

The implementation may require generic placement persistence, but SQL and backend details remain confined to `internal/dbal/sqlite`, the PostgreSQL Adapter, and `internal/migration` as appropriate.

## Immutable AppIR

AppIR should contain only immutable navigation behavior:

- Menu definitions, owner scope, profile, depth, and limits;
- normalized static placements;
- typed Page and View Display targets;
- Entity label/destination/menu eligibility;
- target and placement Policy metadata.

Dynamic Entity record placements remain application data and are never compiled into AppIR. Existing flat Menu AppIR and literal routes must remain loadable through explicit compatibility behavior.

## Studio and generated editor behavior

### Page and View Display definitions

Studio can present contextual “Add to menu” controls while still producing inspectable metadata:

- Menu;
- stable placement ID;
- parent static placement;
- bounded weight;
- optional label override.

### Entity record editor

A navigation-enabled Entity record form can:

- list eligible Menu definitions;
- select one or more authorized Menu instances;
- add at most one placement per instance;
- select a parent from that instance;
- edit bounded weight;
- set or clear a label override;
- remove an existing placement;
- submit record and placement changes atomically through an Action.

The interface must remain usable without drag-and-drop. Pointer reordering may be added later, but keyboard-accessible parent and weight controls are required first.

## Responsive and accessibility behavior

- Every Menu level is a labelled `nav`, not an ARIA tablist.
- The active destination uses `aria-current="page"`; active ancestors receive presentation state without claiming to be the current page.
- Source and DOM order follow normalized sibling order.
- Level 1 and 2 are horizontal and may scroll without reordering on narrow screens.
- Level 3 is vertical at the runtime-owned desktop threshold.
- On narrow screens, level 3 becomes one labelled native select that navigates to the selected route.
- Desktop and mobile controls must not remain simultaneously focusable.
- Focus and active route must remain predictable when the viewport mode changes.

## Required bounds

The first contract must bound:

- number of Menu definitions;
- maximum depth, initially three;
- static and dynamic placements per Menu instance;
- Menu instances returned to one record editor;
- weight range;
- label override length;
- route parameter bindings;
- target View query limits;
- cycle-detection work.

Concrete numbers should come from the maintained acceptance application and adversarial tests rather than arbitrary metadata freedom.

## Maintained proof

A Book/Page reference slice is now more representative than ATS because it proves the distinguishing requirement: an Entity record can be placed in the table of contents of one Book or another.

A useful slice should include:

- at least two Book records and their derived Menu instances;
- Page records placed independently in both Books;
- one Page record reused across two Books;
- a three-level chapter/section/page hierarchy;
- static Page or View Display targets in a global Menu;
- reparenting and heavier-weight sinking;
- label override and derived-label updates;
- target and owner deletion cleanup;
- Policy-hidden target and subtree behavior;
- desktop horizontal/vertical and mobile-select navigation;
- restart, package, SQLite, and PostgreSQL parity.

Application-specific Book behavior belongs under `examples/`; core Menu, target, placement, and rendering Modules remain generic.

## Remaining decisions

1. Should deleting a parent placement with children be rejected until children are moved, or delete the complete placement subtree?
2. Can a scoped Menu include one static target template in every instance, and if so how are owner fields bound to its route?
3. Does an Entity record edit use a dedicated navigation Action input or a generic nested placement operation?
4. What bounded placement and Menu-instance limits are justified by the Book/Page acceptance slice?
5. Should Page/View static placement metadata live on each target as illustrated, or remain canonically inside Menu while Studio edits it contextually?
6. How should publication handle removal of a Menu definition or Entity navigation destination while dynamic placements still exist?

## Non-goals

- In-Page tabpanels or a recursively nested interactive container.
- Treating visual tab styling as ARIA tab semantics.
- More than one placement for the same target in one Menu instance.
- Separately persisted MenuInstance records.
- Broken-link retention after target or owner deletion.
- Client-side Policy or hierarchy resolution.
- Raw user-entered URLs, external navigation, or executable route templates in the first slice.
- Arbitrary CSS, author breakpoints, responsive reordering, or drag-only interaction.
- Dynamic Panels or runtime interpretation of source definitions.
