# Dynamic hierarchical Menu plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze scoped instances, typed targets, placement lifecycle, bounds, and deferred scope | root goal and Menu idea | done |
| 1 | Compile hierarchical static targets and Entity navigation into AppIR v13 | AppIR/compiler/schema/inspect/diff tests | done |
| 2 | Add portable dynamic placement persistence and publication preflight | migration/SQLite/PostgreSQL/release tests | done |
| 3 | Execute typed placement create/update/delete atomically with Entity Actions | Action and idempotency tests | done |
| 4 | Resolve authorized Menu trees and active routes through Views | Menu/HTTP/Policy tests | done |
| 5 | Add generated record editing and responsive accessible navigation | React/Studio/browser tests | done |
| 6 | Add the maintained Book/Page acceptance slice and qualify the repository | package/restart/`make check`/`make build` | done |

## Working rules

- Keep static Page/View placements canonical in Menu definitions; contextual Studio controls edit the Menu draft.
- Scope dynamic Menu instances by owner record identity without persisting MenuInstance records.
- Keep record navigation destinations typed through same-Entity View page Displays; never persist expanded routes.
- Keep placement state outside AppIR and mutate it only within ordinary Action transactions.
- Reject parent deletion, cycles, depth overflow, unauthorized owners, duplicate targets, and unbounded trees.
- Render route navigation with `nav`/links/`aria-current`, never ARIA tab semantics.
- Preserve flat Menu source and AppIR compatibility.
