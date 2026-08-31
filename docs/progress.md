# Progress

## Current

Bean v0.6 agent-readable compiler implementation is complete through milestone 4; repository qualification is active. The real binary now supports the full init/capabilities/schema/validate/inspect/plan/diff/publish/test loop with human output or a versioned `bean.cli/v1alpha1` JSON envelope.

Stable diagnostic families, source-relative paths, compiler-derived candidates, credential redaction, canonical generated schemas, normalized inspection, read-only target planning, semantic AppIR diff, exact-source draft replacement, atomic release activation, and isolated SQLite restart smoke tests have focused evidence. The black-box applicant-tracking harness repairs unknown Entity, unknown field, and invalid transition defects without parsing English messages, then publishes and proves a zero-change diff.

## Roadmap baseline

- North Star: an agent publishes a credible prompt-defined demo with p50 under five minutes, p90 under ten minutes, no human modification after the initial prompt, and 100% of the required behavior rubric under a recorded benchmark protocol.
- v0.6 establishes the deterministic machine contract; it does not embed an LLM or add MCP.
- v0.7 builds the populated, themed, packageable Demo Factory after the machine contract is stable.
- v0.8 adds a provider-neutral Agent Protocol with separate Definition, Release, and Application Planes; v0.9 begins evidence-driven semantic primitives with first-class `Lifecycle`.
- v0.10 adds bounded, side-effect-free deterministic rules between first-class semantics and external effects.
- v0.11 generates tests from semantic primitives and rules; v0.12 adds the typed extension boundary.
- v1.0 qualifies one explicit envelope: a single Bean application process backed by managed PostgreSQL and external object storage.
- Realtime infrastructure, a functions platform, broad OAuth, Redis/messaging abstractions, arbitrary visual design, Kubernetes machinery, and embedded AI chat remain deferred.
- Bean owns application semantics; providers own infrastructure capabilities.

## Asana Lite (completed)

The accepted slice is a local anonymous project/task application with generic status-board, arbitrary-depth tree, and transactional small-file attachment primitives; application behavior remains metadata-only under `examples/asana`.

- The generic `file` field accepts only bounded multipart input, persists base64 content plus safe metadata in the Action transaction, cleans replacement/hard-delete blobs, and policy-checks live references before download.
- Compiler-validated `board` presentation groups enum states and invokes a same-Entity transition Action; `tree` presentation renders a selected many-to-one self relation with arbitrary-depth expand/collapse behavior.
- Anonymous-only compiled applications suppress authentication navigation without weakening protected application behavior.
- `examples/asana` contains 38 definitions grouped across access, projects, tasks, attachments, and pages. Root task creation is project-bound; subtask creation derives project identity from its immutable parent binding.
- Focused field, Action, HTTP, compiler, React, and all Go tests pass. The dedicated Playwright journey passes project creation, board movement, three nested task levels, multipart upload, and byte-identical download.
- `make check` passes, including race tests and all 12 Playwright workflows; `make build` passes with the updated embedded frontend. The Asana Lite goal is complete.

## Reviewable application sources (completed)

Applications use a small manifest plus optional explicit feature-oriented YAML resources; definition documents are flat and CLI diagnostics retain file, line, and column information.

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
