# Persisted-session CSRF continuity plan

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 1 | Reproduce authenticated Admin writes with missing tab-local CSRF state | focused React regression | done |
| 2 | Rehydrate the current session token without weakening server validation | Shell session synchronization and React test | done |
| 3 | Prove the real Blog Admin create journey after storage loss | Playwright Blog journey | done |
| 4 | Qualify the focused fix | `make check` and `make build` | done |

## Working rules

- Keep the database-backed HttpOnly session cookie and exact server-side CSRF comparison authoritative.
- Restore only the token returned for the currently authenticated session; do not retry or bypass failed mutations.
- Preserve login/logout cache clearing and all existing Action/Webform call paths.
