# Progress

## Current

Bean v0.5 complete-blog vertical slice is implemented and qualified as `0.5.0-alpha`. The metadata-only `examples/blog` application covers draft/publish posts, categories, many-to-many tags, opt-in local-password signup/login, authenticated comments, editor approval/rejection, public list/detail/category/tag pages, and RSS.

The generic platform additions are a compiler-known sensitive registration Action boundary, server-recomputed route-bound View/Webform inputs, slug and transaction-time bindings, metadata-driven content presentation, conservative rich-text rendering, dependency-ordered relation migrations, and portable to-many View hydration. Core Go and React code contains no blog-name branches.

## v0.5 verification record

- Compiler/HTTP contracts prove fixed-role sensitive signup, safe output/idempotency, independent throttling, CSRF, draft and pending/rejected non-leakage, member denials, binding tamper rejection, publication/moderation audit, and rich-text sanitization.
- React unit tests cover public rendering, inert rich text, session-aware navigation, and Webform visibility; the frontend has 8 passing unit tests.
- `make test-blog` passes the complete SQLite browser journey.
- `make test-postgres` passes reusable DBAL/HTTP contracts plus the same complete browser journey on PostgreSQL 17.
- `make check` passes vet, frontend lint/typecheck/tests, all Go tests, focused contracts, fuzz-smoke, compatibility, black-box version, race, and Playwright 10/10.
- `make test-crash` passes both supported crash/restart points.
- `make build` passes and `bin/bean version` reports `bean 0.5.0-alpha`.

## v0.4 verification record

- `go test ./...` — pass on 2026-08-30.
- Frontend lint, typecheck, and 6 unit tests — pass.
- Focused no-JSON Studio Playwright acceptance — pass.
- `make test-crash` — pass for crash after migration and after activation commit.
- `make test-postgres` — pass against PostgreSQL 17 for reusable DBAL and Admin/Action/View HTTP parity.
- `make check` — pass, including vet, frontend gates, all Go tests, fuzz-smoke, compatibility, black-box, race, and Playwright 9/9.
- `make build` — pass; output is `bin/bean` (`bean 0.4.0-alpha`).
