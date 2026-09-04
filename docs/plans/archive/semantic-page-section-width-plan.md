# Semantic Page section width plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze vocabulary, dimensions, ownership, defaults, and compatibility | root goal and Panel limitation note | done |
| 1 | Compile normalized section widths into AppIR v12 | AppIR/compiler/schema/inspect/diff tests | done |
| 2 | Render Page-owned Panels and Page chrome at deterministic widths | Page and React/CSS tests | done |
| 3 | Demonstrate contained and wide bands in tracker | source validation and Playwright geometry | done |
| 4 | Qualify the repository | `make check` and `make build` | done |

## Design decisions

- Add `sections[].width` with only `contained`, `wide`, and `full`; omitted values normalize to `wide`.
- Keep width on PageSection, not Panel, so reusable Panels and Sequence Panels do not inherit Page placement concerns.
- Define `contained` as `48rem`, `wide` as the existing `72rem`, and `full` as available viewport width. Every mode retains runtime-owned `1rem`/`1.5rem` safe gutters.
- Treat legacy `Page.panel` and AppIR v11 sections without width as `wide` at runtime. AppIR v12 stores normalized widths.
- Keep Page title, description, and filters aligned to `wide`; section width affects only Page-owned Panel nodes.
- Do not add full-bleed backgrounds, arbitrary values, responsive metadata, or order changes.

## Verification order

```bash
go test ./internal/appir ./internal/compiler ./internal/agentprotocol
go test ./internal/page ./internal/sequence
cd web && bun run test -- --run
cd e2e && bunx playwright test tracker.spec.ts
make check
make build
```
