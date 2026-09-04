# Goal: verify the ATS example in the browser

Status: complete

Run a separate local Applicant Tracker demo, verify its dashboard and Admin, and fix the reproduced datetime form defect: stored RFC3339 timestamps appear empty in datetime-local controls and block unrelated saves. Convert edited local times to RFC3339 while preserving untouched stored instants.

## Contract

- Preserve the running Blog databases and unrelated changes.
- Use the ATS metadata and generated demo data without application-specific core changes.
- Test candidate reads through Views and transitions through Actions.
- Record browser evidence and fix any confirmed defect; complete `make check` and `make build`.

## Evidence

- ATS runs on `http://127.0.0.1:8082` with `tmp/ats-admin-browser.db` and 18 demo candidates.
- Browser verification covers search, Applied → Screen, Admin login/list/detail, and a Summary edit surviving Save and reload while Applied retains its original instant.
- Admin datetime controls now display local time and accept fractional seconds; create/update and selection Action submissions convert edited local values to RFC3339 without rewriting untouched stored timestamps.
- All 11 Admin tests pass, including unchanged, edited, created, and selection Action datetime regressions.
- `make check` passes (87 frontend tests and 18 Playwright journeys, including the new ATS Save/reload regression); `make build` passes.
