# Progress

## Current

Bean v0.8 Agent Protocol is complete from merge commit `f281bae`. The frozen contract has ten provider-neutral operations across Definition, Release, and Application Planes, with CLI as the reference transport and a local MCP stdio adapter over the same dispatcher. Plane grants are host configuration; Application operations retain View-read and Action-write Policy boundaries.

Milestones 0–5 are done. Existing v0.6 commands and generic `agent call` invoke the shared dispatcher. MCP targets current `2026-07-28` stateless metadata plus maintained initialization compatibility, filters discovery by process grants, handles standard liveness and malformed-request behavior, and keeps stdout JSON-RPC-clean. All ten handlers, exact CLI/MCP result parity, independent transport authorization, strict authority inputs, non-initializing View reads, SQLite/PostgreSQL View/Action owner/tenant behavior, and raw Entity/mutation bypass refusal have focused evidence.

Local terminal qualification passes: `make check` including race, black-box CLI, and 14/14 Playwright journeys; `make test-crash`; `make test-postgres` including Agent Protocol parity and the PostgreSQL blog journey; and `make build`. The real binary reports `bean 0.8.0-alpha`; generic CLI validation and Definition-only MCP discovery produce clean versioned machine output. PR CI passes and the latest implementation review reports no major issues after all findings were fixed, answered, and resolved.

## v0.7 completed state

Bean v0.7 Demo Factory is complete on top of the v0.6 machine contract. Typed Theme/DemoSeed metadata, generic Metric/Timeline/Search presentation, ten inspectable ordinary-definition patterns, deterministic Action-based seeding, checksummed SQLite packaging, and the populated ATS reference slice have executable evidence. CRM and tracker also validate and package with deterministic datasets.

Milestones 0–5 are done. The repository gates pass and benchmark documentation records the honest qualification boundary; prepared-definition/package timings are not substituted for repeated external-agent p50/p90 North Star evidence. Provider-specific agents, MCP, hosted sharing, Lifecycle semantics, rules, generated semantic tests, and extensions were outside v0.7.

## v0.7 implementation evidence

- The ATS has 23 metadata definitions and seed `42` produces 76 coherent records with stable semantic source and dataset checksums.
- Focused compiler, HTTP, Action, seed, pattern, CLI package, React, ATS browser, and source-independent packaged-binary browser tests pass.
- The ten patterns compile independently and `pattern inspect` returns their ordinary definitions and declared capabilities.
- Package verification detects artifact tampering; a failed replacement leaves the prior package manifest untouched.
- `make check` passes vet/lint/typecheck, 25 React tests, all Go/race/compatibility/black-box contracts, and 14/14 Playwright journeys. `make test-crash`, `make test-postgres`, and `make build` also pass; `bin/bean version` reports `bean 0.7.0-alpha`.

## v0.6 completed state

The real binary supports the full init/capabilities/schema/validate/inspect/plan/diff/publish/test loop with human output or a versioned `bean.cli/v1alpha1` JSON envelope.

Stable diagnostic families, source-relative paths, compiler-derived candidates, credential redaction, canonical generated schemas, normalized inspection, read-only target planning, semantic AppIR diff, exact-source draft replacement, atomic release activation, and isolated SQLite restart smoke tests have focused evidence. Draft 2020-12 validation accepts all nine maintained applications and rejects invalid manifest/definition shapes. The black-box applicant-tracking harness repairs unknown Entity, unknown field, and invalid transition defects without parsing English messages, then publishes and proves a zero-change diff.

## v0.6 verification record

- `make check` passes vet, frontend lint/typecheck and 24 React tests, all Go tests, focused contracts, fuzz-smoke, AppIR compatibility, JSON-only repair black-box, race, and 12/12 Playwright workflows.
- `make test-crash` passes both supported publication crash/restart points with canonical application sources.
- `make test-postgres` passes reusable PostgreSQL DBAL/HTTP contracts and the complete blog browser journey on PostgreSQL 17.
- `make build` passes and `bin/bean version` reports `bean 0.6.0-alpha`.

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
