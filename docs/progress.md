# Progress

## Current

Reviewable application sources are complete. Applications use a small manifest plus optional explicit feature-oriented YAML resources; definition documents are flat and CLI diagnostics retain file, line, and column information.

## Reviewable application sources

- The loader supports inline multi-document definitions and explicit local resources, rejects unsafe or duplicate inclusion, and flattens source into the existing canonical Bundle.
- `bean app validate --file` and import report syntax, schema, duplicate, and compiler diagnostics before database mutation.
- All eight examples use the new format; blog's 67 definitions are grouped into navigation, access, taxonomy, posts, and comments resources.
- Focused loader, compiler, CLI, Action, and HTTP tests pass. `make check` passes with all 11 Playwright workflows, and `make build` passes.

## Previous completed work

The Bean-owned frontend is standardized on source-owned shadcn/ui components. Tailwind v4, shared Bean tokens, primitive wrappers, Shell/Auth, public metadata rendering, Application Admin, System Admin, and Studio are implemented and qualified. Accessible AlertDialog confirmations replace browser/custom confirmation dialogs, and lint prevents raw interactive-control regressions outside the primitive directory.

## shadcn/ui verification record

- Frontend lint, TypeScript, production build, and all 9 React tests pass.
- Playwright runs 11/11 workflows, including a mobile shadcn integration journey covering Admin cards, generated form controls, record confirmation, overflow, and Studio.
- `make check`, `make test-blog`, `make test-postgres`, and `make build` pass; the PostgreSQL gate includes the complete blog browser journey.
- The generated frontend remains embedded in `bin/bean`; the current assets are approximately 394 KB JavaScript and 48 KB CSS before compression.

Generic scoped resource lists remain complete. The metadata-driven `resource-list` Block reuses AdminResource presentation and Actions while enforcing immutable parent bindings and allowlisted interactive filters; the blog acceptance route is `/blog/:id/comments`.

## Scoped resource-list verification record

- Compiler and HTTP contracts cover resource references, typed defaults, immutable parent scope, filter allowlists, collision rejection, and member denial.
- React tests cover AdminResource reuse and default filter transport.
- The Playwright blog journey proves two posts cannot leak comments across scoped routes and exercises approval plus status filtering.
- The isolated staged tree passes `make check`; `make test-blog`, `make test-postgres`, and `make build` pass on the integrated worktree.

## v0.5 completed state

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
