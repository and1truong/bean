# Goal: Restore CSRF continuity for persisted browser sessions

Status: complete

Fix the administration-console failure where an authenticated browser can load protected pages from its HttpOnly session cookie but every mutation fails with `CSRF validation failed.` because the tab does not contain the matching CSRF token.

## Problem

Bean stores the session identity in a database-backed HttpOnly cookie and returns the per-session CSRF token from `/api/system/session`. The React client previously copied that token into `sessionStorage` only after an interactive login. A new tab, cleared tab storage, or stale tab token could therefore remain authenticated for reads while omitting or sending the wrong `X-CSRF-Token` header on writes.

## Outcome

- An authenticated Shell synchronizes the CSRF token returned by `/api/system/session` into the current tab before normal user interaction.
- Login and logout behavior remains unchanged.
- The server continues to require exact per-session CSRF tokens for authenticated mutations; no CSRF check is weakened or bypassed.
- React and browser evidence reproduce an authenticated session with missing tab storage and prove that the next Admin Action carries the restored token.

## Acceptance criteria

- Opening or reloading an authenticated Admin route with empty `sessionStorage` restores `bean_csrf` from the server session response.
- Creating a Blog category after that restoration succeeds without signing in again.
- Existing authentication, logout, Action, Webform, Admin, and public-page tests do not regress.
- `make check` and `make build` pass.

## Non-goals

- Changing cookie attributes, session expiry, server-side token generation, or the CSRF validation algorithm.
- Automatically retrying rejected mutations.
- Moving session identity or CSRF authority into application metadata.
