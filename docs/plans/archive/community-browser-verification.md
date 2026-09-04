# Community browser verification

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Prepare isolated Community and test identities | localhost:8083, separate DB, two member/editor accounts | done |
| 1 | Verify Admin, ownership, publication, and reactions | A publishes through Admin; B is denied private access, sees public posts, and creates a like | done |
| 2 | Resolve confirmed defects and qualify | semantic tests, both Community regressions, `make check` (19 browser journeys), and `make build` pass | done |
