# Contextual Admin and request-scoped route records plan

Status: complete. Detailed contract: `docs/plans/contextual-admin-route-records.md`.

| Milestone | Deliverable | Verification | Status |
| --- | --- | --- | --- |
| 0 | Freeze View-backed resolution, cache boundaries, response shape, route, and query budget | failing HTTP/Menu/View contracts and counting reader | done |
| 1 | Add immutable request-scoped resolved-record/proof reuse | View/Menu/HTTP isolation and duplicate-read tests | done |
| 2 | Derive owner-side Menu trees and eligible target AdminResources | deterministic and Policy-negative HTTP tests | done |
| 3 | Add generic contextual Admin create flow | Admin React tests, lint, and typecheck | done |
| 4 | Prove Book → Add Page atomically | Books browser, tamper, rollback, and validation evidence | done |
| 5 | Document and qualify | `make check` and `make build` | done |

## Working rules

- Resolve runtime records through Views; never turn route upcasting into direct table access.
- Keep record snapshots request-local and immutable; cache keys must preserve release, View, actor, tenant, and reader/transaction boundaries.
- Re-authorize every write inside its Action transaction; GET context is never write authority.
- Derive the first contextual affordance from existing Menu, Entity navigation, and AdminResource AppIR; add no speculative hook system or metadata.
- Keep Page reusable across Books and use `_navigation` as the only placement mutation path.
- Preserve the completed shared-chart work and all unrelated SaaS and Books edits.
