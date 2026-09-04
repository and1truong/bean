# Goal: stop repeated missing-page requests

Status: complete

Show an explicit not-found state for an undefined public route, including `/` in the presentation example, without retrying its HTTP 404 response or automatically refetching it on focus/reconnect.

## Contract

- Preserve the presentation route `/presentations/bean` and metadata-owned application behavior.
- Retain HTTP status in API errors; distinguish missing pages from other failures.
- Preserve bounded retries for transient page failures and normal navigation/session resets.
- Verify regressions, then run `make check` and `make build`.

## Evidence

- Before the fix, both `/` and `/missing` produced four HTTP 404 requests; exhausted server failures were hidden by the generic Bean heading.
- All 83 frontend tests pass, including 404 request counts and focus/reconnect behavior, transient recovery, and bounded failure retries.
- `make check` passes, including Go/race checks and all 18 Playwright journeys; `make build` passes and refreshes embedded frontend assets.
- Browser qualification required execution outside the sandbox because Chromium launch was denied inside it.
