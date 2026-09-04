# ATS browser verification

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Start an isolated populated ATS demo | 37 definitions validate; demo runs at localhost:8082 | done |
| 1 | Verify dashboard, candidate movement, and Admin | search, Applied → Screen, and Admin login/list pass; required datetime is blank and blocks Save | done |
| 2 | Fix Admin datetime display and submission; qualify | 11 Admin tests, browser save/reload, 18 Playwright journeys, `make check`, `make build` pass | done |
