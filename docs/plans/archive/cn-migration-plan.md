# `cn` migration plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Audit class-merging dependencies, call sites, Tailwind patterns, and build-tool fit | repository searches and package documentation | done |
| 1 | Preserve the shared `cn()` API using `cn` and update direct dependencies | focused utility tests and lockfile review | done |
| 2 | Verify semantic compatibility and record representative before/after measurements | regression matrix, benchmark, and bundle comparison | done |
| 3 | Qualify the frontend and repository | frontend gates, UI tests, `make check`, and `make build` | done |

## Working rules

- Keep all component call sites and `class-variance-authority` unchanged.
- Do not add `cn build`, generated tables, or CI steps unless the measured benefit justifies their maintenance cost.
- Treat microbenchmarks as supporting evidence, not a claim about end-to-end UI performance.
- Preserve the user's pre-existing `examples/books/README.md` change.
