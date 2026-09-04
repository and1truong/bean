# Bean v0.16 Semantic Sequences implementation plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its public flow and listed evidence pass.

| Milestone | User-visible deliverable | Example slice | Evidence | Status |
| --- | --- | --- | --- | --- |
| 0 | Freeze the generic Sequence boundary and Bean-introduction benchmark | `examples/presentation` contract | root goal, exact fixture/rubric, failing tests | done |
| 1 | Compile inspectable Sequence and semantic content metadata | minimal Sequence fixture | AppIR/schema/capabilities/inspect/diff/reference/compatibility tests | done |
| 2 | Receive deterministic layout, safety, and density repair diagnostics | broken Bean deck fixture | compiler diagnostic and repair-loop tests | done |
| 3 | Navigate an accessible, responsive, print-ready HTML sequence | three-frame browser fixture | render tree, React, accessibility, URL/keyboard/print tests | done |
| 4 | Present the ten-frame Bean introduction from ordinary definitions | complete Bean introduction | source, agent rubric, View/chart, E2E, package/restart tests | done |
| 5 | Ship the v0.16 compatibility/version/documentation cut | all maintained examples | terminal gates and clean diff | done |

## Dependency order

```text
M0 contract
  -> M1 canonical metadata/AppIR
     -> M2 deterministic validation
        -> M3 runtime rendering
           -> M4 complete reference vertical slice
              -> M5 qualification
```

## M0 — Contract and executable fixtures

- Archive the completed v0.15 goal as `docs/goals/015.md`.
- Define `Sequence` as an ordered route-level composition of existing Panels; do not introduce a parallel `Presentation`, `Slide`, `Report`, or rendering runtime.
- Define one initial `presentation` profile, `wide|standard` aspect ratios, closed frame layout vocabulary, content vocabulary, safety constraints, and deterministic resource budgets.
- Define the exact ten-frame Bean introduction rubric and a seeded invalid-to-valid repair case.
- Add failing focused tests before implementation.

Verification:

```bash
go test ./internal/compiler ./internal/appir ./internal/page
cd web && bun run test -- App
```

Non-goals: PDF/PPTX, embedded agents, WYSIWYG, warnings framework, arbitrary HTML/CSS/JS/SVG, or external asset tooling.

## M1 — AppIR v8 Sequence and semantic content

- Add `Sequence`, `SequenceFrame`, and `ContentElement` to AppIR v8.
- Add `content` as a generic Block capability that remains usable by ordinary Pages.
- Register Sequence with compiler storage, normalization, lookup, references, semantic diff, schema generation, and route matching.
- Expose sequence profiles/aspect ratios/layouts, content types/tones/directions, and all resource limits through capabilities.
- Extend AppIR compatibility so v1–v7 remain readable and reject v8-only fields.
- Keep the application definition lifecycle and release persistence unchanged.

Verification:

```bash
go test ./internal/appir ./internal/compiler ./internal/release ./internal/agentprotocol
```

## M2 — Stable validation and agent repair loop

- Validate canonical/unique routes and frame names, bounds, Panel references, layout/Panel compatibility, Block count, semantic content shape, image alt/source safety, diagram bounds, code lines, and weighted density.
- Use stable Bean diagnostic codes and exact source paths. Candidate lists should cover mistyped Panels and closed vocabulary values where the existing diagnostic machinery supports them.
- Prove identical broken input yields identical ordered diagnostics.
- Prove the reference agent fixture can change only diagnosed fields and reach a valid compiled AppIR without source-code knowledge.

Verification:

```bash
go test ./internal/compiler ./internal/agenttest
```

## M3 — Accessible HTML and print runtime

- Match Sequence routes alongside Page and View-display routes without ambiguity.
- Build one `Sequence` render node with a child Panel tree per Policy-visible frame.
- Render one active frame at a time with deep-linked machine-name state, buttons, picker, progress, arrow/Home/End keys, focus-safe semantics, notes toggle, and stable empty/error behavior.
- Add responsive 16:9/4:3 canvas sizing and print styles that render all frames, one per page, while hiding navigation and notes.
- Do not change ordinary Page/Panel/Block rendering semantics.

Verification:

```bash
go test ./internal/page ./internal/httpapi ./internal/block
cd web && bun run lint && bun run typecheck && bun run test
```

## M4 — Executable Bean introduction

Create `examples/presentation/` with exact files:

- `app.yaml`: manifest and explicit resource list;
- `content.yaml`: reusable semantic narrative Blocks;
- `data.yaml`: deterministic DemoSeed, `capability` Entity, grouped View and chart Display;
- `layout.yaml`: Panels and the `bean_introduction` Sequence.

The ten frames are:

1. Bean title and product statement.
2. Why probabilistic agents need a deterministic target.
3. Definition -> compiler -> AppIR -> runtime architecture.
4. Core definitions and ownership boundaries.
5. Agent validate/inspect/diff/test/publish loop.
6. Explore: model -> visualize -> drill -> act.
7. Shipped capability areas as a real grouped View/chart.
8. Safety, Policy, Lifecycle, Rule, Action, and Extension boundaries.
9. Reference applications, current status, and near roadmap.
10. Five-minute getting-started close.

Expected behavior:

- `/presentations/bean` opens frame 1 and reports `1 / 10`.
- keyboard/button/picker navigation reaches every frame and updates `?frame=`.
- a direct URL opens the requested frame; an unknown frame falls back to the first.
- speaker notes are hidden initially and visible only after the explicit toggle.
- frame 7 loads the grouped chart through the named View and has meaningful seeded groups.
- print mode has ten page-break frames and no interactive chrome.
- packaging removes source and retains the complete route and seeded chart.

Verification:

```bash
bin/bean app validate --file examples/presentation/app.yaml --json
cd e2e && bunx playwright test presentation.spec.ts package.spec.ts
go test ./internal/agenttest ./internal/release
```

## M5 — Qualification

- Update authoring, architecture, definition, capability, security, testing, reference-app, and agent guidance.
- Set `0.16.0-alpha`, regenerate JSON schemas and embedded frontend assets.
- Validate all examples and run package/restart evidence.
- Record the honest benchmark: deterministic prepared-definition validation/render time is not an external LLM generation benchmark.
- Pass terminal gates and leave a clean local commit on `v0.16-presentations`.

```bash
make check
make test-crash
make test-postgres
make build
```
