# Completed Asana Lite local application plan

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Goal, contracts, and metadata design | focused compiler/field tests and source validation | done |
| 1 | Transactional generic file field and multipart Webform path | field, Action, HTTP, and React tests | done |
| 2 | Generic board and arbitrary-depth tree presentations | compiler and React tests | done |
| 3 | Metadata-only anonymous Asana Lite application | source validation and browser journey | done |
| 4 | Documentation and repository qualification | `make check` and `make build` | done |

## Working rules

- Keep project/task/attachment behavior under `examples/asana`; core additions must compile and render any compatible metadata.
- Preserve View reads, Action writes, immutable route bindings, and atomic Action/audit/blob persistence.
- Never use a client filename as a path or expose file bytes in AppIR, manifests, logs, audit, or Action results.
- Board and tree field references are compiler-validated; state changes use declared Actions.
- Run the nearest focused test after every milestone and keep this plan plus `docs/progress.md` current.
