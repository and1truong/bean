# Reviewable application sources plan (completed)

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Manifest/resource grammar and source-aware loader | definition loader tests | done |
| 1 | CLI, embedded demo, and fixture integration | CLI and affected package tests | done |
| 2 | Feature-oriented example migration and authoring docs | all example validation and documentation review | done |
| 3 | Repository qualification | `make check` and `make build` | done |

## Working rules

- Optimize the source format for authors and reviewers; do not preserve the unused bundle syntax.
- Keep resource inclusion explicit, local, and non-overriding.
- Preserve source locations through compiler diagnostics.
- Flatten sources into the canonical Bundle before the existing release lifecycle.
