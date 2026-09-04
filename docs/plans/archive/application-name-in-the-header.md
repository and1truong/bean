# Application name in the header

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Expose published app name and use it as the header default | regressions pass for bundle name after init, republish/reload, default branding, and explicit Theme branding | done |
| 1 | Qualify and verify the live Community header | `make check` (90 frontend tests, 20 browser journeys) and `make build` pass; live Admin header renders Community | done |
