# Bean Blog composition modernization plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze the metadata-only Display, width, responsive, and Policy-collapse scope | root goal and repository review | done |
| 1 | Move remaining taxonomy/comment presentation into named View Displays | Blog validation and existing browser journey | done |
| 2 | Compose Blog routes with semantic section widths and inline content | Blog validation and render-tree behavior | done |
| 3 | Prove responsive authorized discussion and anonymous Region collapse | focused Blog Playwright journey | done |
| 4 | Qualify the repository | `make check` and `make build` | done |

## Working rules

- Keep application behavior in `examples/blog` and use only existing generic capabilities.
- Preserve every route/context binding and all public/editor/member Policy semantics.
- Keep the article body `contained`, discussion `wide`, and moderation tables `wide`.
- Preserve main-before-sidebar source and accessibility order at every viewport width.
- Keep Policy-sensitive sidebar narrative in a named content Block; public local narrative may be inline.
- Named Displays own presentation; Blocks own selection, bindings, Policy, and composition.
