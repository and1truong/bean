# Ordered multi-Panel Page section plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze syntax, bounds, compatibility, policy, and binding semantics | root goal and design note | done |
| 1 | Compile ordered Page sections into versioned immutable AppIR | AppIR/compiler/schema/reference/diff tests | done |
| 2 | Render Policy-visible sections and authorize bound Blocks across them | Page/HTTP/generated-registration tests | done |
| 3 | Demonstrate multiple layout bands in tracker and document authoring | source validation, Studio/React/Playwright, docs | done |
| 4 | Qualify the repository | `make check` and `make build` | done |

## Design decisions

- Add `sections: [{panel: ...}]` as the ordered multi-band source form; keep legacy `panel` as-is and reject declaring both. Bound the new list to 1–32 entries.
- Store `PageSection` directly in immutable AppIR v10. A legacy Page keeps `Panel`; a sections Page keeps `Sections`, so compatibility input does not create noisy semantic rewrites.
- Resolve Page context once. Render each Panel in source order, omit Panels denied by their own Policy, and return unavailable only when no Panel is visible. The outer Page Policy remains unchanged.
- Compute Page filter membership over all declared Panels. Bound View/Webform requests must locate the named Block in at least one declared Panel whose Policy and Block Policy allow the request.
- Repeated Panel references are allowed because a reusable layout band may intentionally appear more than once; no nested composition or cycle graph is introduced.

## Verification order

```bash
go test ./internal/appir ./internal/compiler
go test ./internal/page ./internal/httpapi ./internal/generatedtest
bin/bean app validate --file examples/tracker/app.yaml --json
cd e2e && bunx playwright test tracker.spec.ts
make check
make build
```
