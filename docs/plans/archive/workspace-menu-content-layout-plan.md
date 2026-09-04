# Workspace Menu/content layout plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze generic responsive composition and compatibility behavior | root goal and Menu/Panel design docs | done |
| 1 | Group a workspace Menu with following route content in the generic renderer | focused React tests | done |
| 2 | Implement desktop sidebar/content and mobile select/content geometry | CSS and Playwright measurements | done |
| 3 | Verify the existing Books demo with the rebuilt server | live visual inspection without data replacement | done |
| 4 | Qualify the repository | `make check` and `make build` | done |

## Working rules

- Compose only a `workspace` Menu with following siblings in the same render container; do not infer application names or IDs.
- Keep primary, secondary, tertiary, then content DOM order.
- Do not alter server Policy resolution, Menu hierarchy, routes, or Action persistence.
- Preserve ordinary rendering when content, tertiary navigation, or the workspace profile is absent.
