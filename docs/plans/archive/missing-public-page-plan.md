# Missing public page plan

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Reproduce missing-route retries and hidden errors | reproduced four requests per missing route | done |
| 1 | Preserve HTTP status and handle missing pages explicitly | all 54 App tests pass, including focus/reconnect and transient errors | done |
| 2 | Qualify the repository | `make check` (83 React tests, 18 Playwright journeys) and `make build` pass | done |
