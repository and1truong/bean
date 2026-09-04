# Policy-aware collapsible Panel Region plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze opt-in empty, expansion, error, compatibility, and authorization semantics | root goal and Panel limitation note | done |
| 1 | Compile `collapseWhenEmpty` into AppIR v11 | AppIR/compiler/schema/diff tests | done |
| 2 | Omit authorized-empty Regions and expand a sole survivor | Panel/Page/Sequence and React/CSS tests | done |
| 3 | Document authoring and verify browser-visible full-track expansion | definitions/capabilities docs and Playwright | done |
| 4 | Qualify the repository | `make check` and `make build` | done |

## Design decisions

- `regions[].collapseWhenEmpty` is opt-in and defaults to false, preserving existing source and render trees.
- Empty means no server-authorized Block nodes after normal Block Policy evaluation. View result emptiness, loading, and errors are Block concerns and do not collapse the Region.
- Omit collapsed Regions. If one of multiple authored Regions survives, mark it expanded so runtime-owned CSS spans all Panel tracks. If none survive, the Panel is unavailable.
- Keep declared composition authoritative for compiler membership, inspection, generated checks, and bound HTTP requests; visual omission cannot grant or revoke Block access.
- Store the boolean in immutable AppIR v11 while preserving v10 Page sections and all earlier compatibility readers.

## Verification order

```bash
go test ./internal/appir ./internal/compiler ./internal/agentprotocol
go test ./internal/panel ./internal/page ./internal/sequence
cd web && bun run test -- --run
cd e2e && bunx playwright test tracker.spec.ts
make check
make build
```
