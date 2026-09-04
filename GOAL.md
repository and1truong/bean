# Goal: Dynamic hierarchical Menu navigation

Status: complete

Deepen Bean's existing Menu into bounded route-backed workspace navigation with static Page/View targets and owner-scoped Entity-record placements.

## Initial contract

- A Menu is either global or scoped by an owner Entity; scoped instances are derived from `(menu name, owner record ID)` and are not records.
- `workspace` menus have at most three levels: horizontal primary, optional horizontal secondary, and vertical tertiary navigation; tertiary navigation becomes a labelled native select on narrow viewports.
- Parent determines level. Siblings sort by ascending `(weight, placement ID)` with weight bounded to `-1000..1000`.
- A target appears at most once per Menu instance, may appear in several instances, and may have a placement-specific label override up to 120 characters.
- Static placements live canonically in global Menu definitions and target Page definitions or View page Displays.
- Navigation-enabled Entity records declare a label field, destination View page Display, and eligible scoped Menus.
- Record editors submit optional typed navigation placement state with create/update Actions. Record and placement mutations commit or roll back atomically.
- Removing a placement with children is rejected. Deleting a target or owner record removes affected placements atomically.
- Reads remain View-authorized and writes remain Action-owned. Client code never evaluates Policy.
- Existing flat Menu definitions and `menu` Blocks remain compatible.

## Bounds

- 32 Menu definitions.
- Depth 3.
- 200 placements per Menu instance.
- 32 authorized Menu instances per record editor.
- Weight `-1000..1000`.
- Label override length 120.

## Acceptance slice

Add a maintained metadata-only Book/Page example proving:

- two owner-scoped Menu instances;
- one Page record reused in both Books;
- a three-level hierarchy;
- parent/weight/label editing through generated record forms;
- typed route resolution;
- Policy filtering and active trail;
- desktop and mobile navigation;
- atomic target/owner cleanup;
- publication/restart/package compatibility.

## Compatibility and non-goals

- Raise immutable AppIR only for compiled navigation contracts; dynamic placements remain persisted application data.
- Reject publication that would orphan dynamic placements; never delete or rewrite them silently.
- Defer static templates in scoped Menus, duplicate targets in one instance, external/raw user routes, drag-and-drop, Theme menu slots, and true in-Page tabpanels.
- Preserve definition → validation → migration → immutable AppIR → atomic activation, generic core behavior, View reads, Action writes, and SQL confinement.
- Completion requires `make check` and `make build`.
