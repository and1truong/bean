# Asana Lite composition modernization plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze the metadata-only composition and Display migration scope | root goal and repository review | done |
| 1 | Move Asana presentation metadata into named View Displays | source validation and existing browser journey | done |
| 2 | Compose Asana Pages from responsive ordered Panel sections and inline content | source validation and focused browser assertions | done |
| 3 | Qualify the maintained reference application and repository | Asana Playwright journey, `make check`, `make build` | done |

## Working rules

- Keep Asana behavior under `examples/asana` and use only existing generic capabilities.
- Preserve route-context Block bindings and the project priority Page-filter fan-out across Panel sections.
- Keep board, tree, and attachment-list bands full width; use responsive two-column Panels only for naturally paired content.
- Preserve source and accessibility order at every viewport width.
- Named Displays own presentation; Blocks own selection, bindings, and composition.
- Inline content replaces only one-off narrative Blocks with no independent Policy or reuse requirement.
