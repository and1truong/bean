# Booking example source-split plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Capture the source-split contract and inspect the existing Booking definition | manifest/resource model and baseline validation | done |
| 1 | Move Booking definitions into explicit feature-oriented resources | `app.yaml` manifest and reviewable YAML resources | done |
| 2 | Document and qualify the unchanged application behavior | Booking validation/test, matching semantic checksum, `make check`, and `make build` | done |

## Working rules

- Preserve definition order and content while changing only source organization.
- Keep the manifest resource list explicit; do not introduce discovery or core behavior.
- Keep this plan, `GOAL.md`, and `docs/progress.md` current.
