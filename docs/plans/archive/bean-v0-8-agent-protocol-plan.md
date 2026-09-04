# Bean v0.8 Agent Protocol plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze protocol operations, planes, transport, authorization, and compatibility contracts | root goal plus registry/authorization test fixtures | done |
| 1 | Shared provider-neutral dispatcher for Definition, Release, and Application Planes | focused handler tests across all ten operations | done |
| 2 | Existing command delegation plus generic CLI protocol transport | CLI compatibility and plane allow/deny contract suite | done |
| 3 | MCP 2026-07-28 stdio adapter with maintained legacy initialization compatibility | framing/discovery/list/call/error/EOF tests | done |
| 4 | Cross-transport parity, runtime Policy boundaries, and backend qualification | CLI/MCP parity plus SQLite/PostgreSQL View/Action tests | done |
| 5 | Provider-neutral agent guidance, documentation, terminal gates, and clean reviewed PR | shipped `agents/`, docs, all gates, CI, and Codex review | done |

## Working rules

- The dispatcher delegates to compiler, release, View, and Action services; transports contain framing and presentation only.
- Plane grants are host configuration and are checked before source or database access.
- MCP tool arguments never grant roles, tenants, planes, raw tables, arbitrary writes, SQL, or shell access.
- Preserve the v0.6 CLI envelope and commands while making their structured results originate from shared handlers.
- Support MCP stdio only in v0.8; do not add remote transport or identity infrastructure.
- Add failing contract evidence before each public operation and run the nearest test after each milestone.
- Keep `GOAL.md`, `ROADMAP.md`, `PLANS.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
