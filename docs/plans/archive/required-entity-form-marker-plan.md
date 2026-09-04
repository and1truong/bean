# Required Entity form marker plan

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 1 | Render a red `*` after labels for required Entity fields across typed controls | focused Admin React test | done |
| 2 | Preserve accessibility and qualify the repository | `make check` and `make build` | done |

## Working rules

- Derive the marker only from immutable Entity field metadata.
- Keep the marker visual-only so accessible field names and native required semantics remain unchanged.
- Do not alter validation, View reads, Action writes, or application metadata.
