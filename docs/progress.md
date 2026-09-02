# Progress

## Current

Bean v0.15 Explore is complete; milestones 1–8 are done. The implemented boundary is “small core gaps plus first-party module”: administrator Entity exploration/save plus compiler-known query/result/interaction semantics, all materialized as ordinary View, Display, Page, Panel, Block, Action, Policy, and Lifecycle definitions. There is no Question, Report, Dataset, Visualization, Querier, arbitrary SQL, separate semantic layer, or parallel query runtime.

`bean/appir/v7` adds View-owned search, typed scalar/date grouping, checked aggregates, compiler-derived records/groups/metric shapes, deterministic backend ordering, bounded group overflow, and Policy-before-aggregate execution while v1–v6 compatibility remains explicit. Compatible table/list/cards/board/bar-chart/metric/timeline/calendar renderers switch without changing the query. Explicit Page filter targets preserve immutable context. Typed metric/chart drill resolves a target View/Display/filter contract and re-runs target Policy. Selected tables call bounded sequential non-atomic batches through the ordinary Action service and refresh affected Views.

The administrator-only `/explore` surface compiles previews through the canonical compiler/View service, creates record/group/metric Views, saves to the deterministic Studio draft, and shows validation plus semantic changes. Studio visually edits the common search/group/aggregate/renderer/page-filter/drill/action path. Capabilities, schemas, inspection references, diff, TestSuite, and the unchanged Agent Protocol expose the same semantics. Five prompt rubrics over ATS/CRM/Commerce verify ordinary inspectable artifacts and deterministic recompilation without a YAML string oracle.

ATS is the primary recruiting-operations application; CRM proves authorization before aggregation and exact Policy-visible contribution drills; Commerce proves observe → inspect → authorized Action → refreshed dashboard; Booking proves calendar compatibility; Asana proves split-source immutable context and page-filter composition; Tracker proves status chart-to-record drill outside ATS. All ten examples validate. Cold/warm existing-Entity browser flows create, preview, save, validate/diff, publish, and open ordinary record/chart Views in 860/653 ms with zero intervention on the qualification host; the measurement is automation evidence, not a human or external-agent p50 claim.

Final local qualification is green: `make check` passes vet, lint, typecheck, all Go and race tests, 47 React tests, agent black-box contracts, embedded build, package/restart, and 14/14 Playwright journeys; `make test-crash`, `make test-postgres`, and `make build` pass. PostgreSQL qualification includes DBAL, HTTP/Application Plane, Agent Protocol, and the blog browser journey. The binary reports `bean 0.15.0-alpha`. Existing Entities plus deterministic DemoSeed are the v0.15 onboarding path; CSV import and external/existing data mapping remain deferred.

## v0.14 completed state

Bean v0.14 First-class View Displays is complete at merge `7feca74`; milestones 0–6 are done. `bean/appir/v6` keeps query metadata at View scope and adds named page/block/JSON/CSV/RSS displays, derived typed exposed-filter mappings, compiler-known render/control/pager vocabularies, Block display references, and deterministic legacy `Block.presentation` normalization. Schemas, capabilities, inspection references, semantic diff, AppIR compatibility guards, route/field/redaction/link/widget/pager/title validation, and focused compiler/AppIR/Agent Protocol tests pass.

Page and Block runtime execute displays through the existing Policy-preserving View service. Page routes and immutable bindings are server-resolved, client collisions and undeclared controls fail closed, filter state invalidates opaque cursors, and display limits stay within the View maximum. React evidence covers tables, labels, inferred values, safe links, empty/error states, URL controls, cursor previous/next, and static/result-derived headings and browser titles. Studio authors the common page/block, list/detail/table, column, control, pager, and title path without Advanced JSON.

Blog list/detail Blocks and ATS list/board/metric/timeline Blocks reference named displays; ATS also declares the metadata-only `/candidates` table page with a labelled stage control and cursor pager. Both sources compile without diagnostics. Local v0.14 qualification was green: `make check`, `make test-crash`, `make test-postgres`, and `make build` passed, including all 14 Playwright journeys. The release excludes a public Query/Querier definition, nested query rewrite, numeric/full paging, arbitrary templates or application code, sequence/slide rendering, and presentation export.

## v0.13 completed state

Bean v0.13 Typed Extension Boundary is complete at squash merge `8a8c199`. All milestones are done. `Extension` is a registry-owned Definition in immutable `bean/appir/v5`, with canonical schema, capabilities, stable `BEAN-E2871` diagnostics, inspection, semantic diff, metadata-only publication, restart, and explicit v1–v4 compatibility. Its closed contract accepts one HTTP transport, network permission, external-write effect, none-or-bearer authentication, bounded timeout/fixed retry, required idempotency, after-commit transaction semantics, and retry-then-fail behavior. Endpoints require HTTPS except for loopback HTTP; typed fields exclude secrets, relations, files, and storage semantics.

Transaction Actions now bind typed Extension inputs and persist one invocation in the existing outbox transaction. Policy denial creates no intent, rollback removes it, the success audit shares its commit, and Action idempotency replay does not enqueue twice. One invocation ID is the HTTP idempotency key across fixed-delay attempts. The HTTP provider sends a versioned bounded JSON POST, resolves bearer tokens only from `BEAN_EXTENSION_BEARER_TOKENS`, refuses redirects, validates typed output, and reports safe deterministic timeout, unavailable, authentication, redirect, response, and contract categories. Retryable failures retain existing lease/stale-claim behavior; permanent failures become terminal immediately. Focused compiler, Action, outbox, HTTP, bootstrap, and complete Go tests pass.

Semantic TestSuites now inject ordered typed provider responses or stable failures into the production delivery path after the Action transaction commits. Expected calls include Extension identity, immutable invocation/idempotency identity, and typed input; exhausted or unconsumed mocks fail through stable redacted `BEAN-T1001` evidence. Repeated execution is deterministic, provider values are redacted from inspection, generated negative cases cannot inherit irrelevant mocks, and ordinary event assertions exclude internal Extension topics.

Commerce now declares the generic `order_notification` Extension in metadata. `place_order` atomically creates its order and notification intent, then its maintained TestSuite delivers through a typed offline mock while continuing to prove inventory mutation, order creation, and the public `order_placed` event. Direct and generated replay suites pass without network access. Arbitrary scripts, synchronous provider results, WASM/plugins, OAuth, new queue infrastructure, and provider-specific core behavior remain excluded.

Local v0.13 qualification is green: `make check`, `make test-crash`, `make test-postgres`, and `make build` pass, including race detection, 30 frontend tests, 14/14 browser journeys, PostgreSQL application parity, crash/restart recovery, and `bean 0.13.0-alpha`. PR #15 reached the ten-review fallback; every final finding was fixed, answered, and resolved, final CI was green, and post-merge main workflow `33579291003` passed.

## v0.12 completed state

Bean v0.12 Generated Semantic and Rule Tests is complete at squash merge `fbaa54a`. Generated runtime checks materialize as ordinary TestSuite definitions from compiler-validated AppIR, explicit expectations, and relation-complete DemoSeed data. Explicit expectations remain the oracle; generated Policy/Lifecycle negatives and CRUD mutations use Actions, verification reads use Views, and eligible route journeys use the production HTTP handler. Structural evidence covers schema, Rule, Policy, Lifecycle, and route contracts with stable source identities.

Commerce, ATS, and booking pass generated replay, negative, CRUD, and journey checks. Repeated complete machine output is byte-identical; malformed routes produce structured failures rather than panic. `make check`, `make test-crash`, `make test-postgres`, and `make build` passed with `bean 0.12.0-alpha`. PR #14 reached the ten-review fallback; every final finding was fixed, answered, and resolved, final CI was green, and the post-merge main workflow `33570003770` passed.

## v0.11 completed state

Bean v0.11 First-class Semantic Test Suites is complete at merge `f224246`. TestSuite is registry-owned metadata with canonical schema, stable diagnostics, capability limits, inspection, references, semantic diff, immutable `bean/appir/v4` storage, and restart compatibility. Typed Rule and Action cases execute in fresh bounded SQLite runtimes through production paths with explicit context and deterministic result/error/mutation/event/audit evidence.

Commerce, ATS, and booking suites catch seeded calculation, Policy/guard, derivation, invariant, mutation, and event defects. `make check`, `make test-crash`, `make test-postgres`, and `make build` passed with `bean 0.11.0-alpha`. PR #13 merged after the documented ten-review fallback: every final-round finding was addressed and resolved, final validation/CI were green, and the post-merge main workflow passed.

## v0.10 completed state

Bean v0.10 Deterministic Rule Expressions is complete from merge commit `38a6095`. All milestones 0–5 are done. The frozen contract uses a named Rule definition with a canonical structured AST, explicit typed sources, a small fixed operator set, and compile/runtime resource bounds. The Rule core checks and evaluates every accepted operator and source, preserves exact integer and finite-decimal arithmetic, compares typed dates/datetimes, short-circuits deterministically, returns coded failures, and enforces 128-node, depth-16, 4 KiB literal, 16 KiB value, and normalized exact-number exponent limits. Replay, forbidden vocabulary, canonical shape, arity, typing, nullable boolean, missing-value, divide-by-zero, and resource-limit contracts pass with the repository-wide Go suite.

Rule is now a registry-owned Definition with canonical schema, `bean/appir/v3` storage, v1/v2 rejection of Rule-bearing releases, stable `BEAN-E2351` expression diagnostics, closed operator/source/limit capabilities, named inspection, typed references from Action and Entity consumers, and semantic diff. Inspect redacts literal values with a stable digest so threshold changes remain visible without exposing their contents. Compiler validation resolves Entity and input types and rejects mismatched guard, derive, and invariant consumers before publication.

Actions now reject client-supplied derived inputs, evaluate sibling derives simultaneously from the original input, inject one stable `context.now` across transaction retries, and preserve idempotent replay. Record Policy checks precede Rule guards; false guards deny before mutation and evaluator failures retain stable Rule causes while failing closed. Entity invariants evaluate the final typed create/update/transition candidate immediately before persistence, including create/update/decrement transaction steps, so a failed invariant rolls back the complete transaction. OpenAPI and Admin action forms omit server-owned inputs. Focused tests cover record-aware guard denial, Policy-before-failing-guard order, derivation, override refusal, unavailable context, candidate validation, rollback, and replay; the complete Go suite and 30 frontend tests pass.

Three maintained applications now carry unrelated metadata-only slices: commerce derives `order_item.line_total`; ATS guards pipeline movement against blank candidate names using the current record; booking derives `requested_at` from injected `context.now` and enforces `start_at < end_at`. Focused source execution and all three browser journeys pass. DemoSeed deterministically evaluates create derives for its expected dataset while omitting server-owned fields from Action calls. Rule publication is metadata-only, survives SQLite active-release reload, and executes after restart. The booking derive/invariant path passes the shared SQLite/PostgreSQL HTTP parity suite, `make test-postgres` passes including the PostgreSQL blog journey, and `make test-crash` passes both publication fault points.

Local terminal qualification passes on the v0.10 version cut: `make check` including race, black-box contracts, 30 React tests, and 14/14 Playwright journeys; `make test-crash`; `make test-postgres`; and `make build`. The binary reports `bean 0.10.0-alpha`; PR #11 merged after a clean latest-commit Codex review and green CI.

The first consumers remain Action guards, simultaneous server-owned derived Action inputs, and Entity invariant predicates; three unrelated metadata-only applications demonstrate computed/derived values, a record-aware guard, validation, and injected `context.now`.

Rules refine local behavior but do not replace Entity, Lifecycle, Policy, View, or Action structure. Policy remains authoritative authorization; evaluation is side-effect-free and has no I/O, implicit clock/randomness, environment, SQL, modules, or mutable state. Text/infix syntax, browser visibility, computed read columns, generated tests, and external effects are intentionally deferred.

## Structural contracts completed state

The structural-contract and unified-execution-seam goal is complete. All milestones 0–9 are done without adding a public plugin platform or application-specific core behavior.

Diagnostics now receive codes and non-serialized recovery facts structurally, with no production control flow that parses diagnostic or Go standard-library prose. Database and transactional reads share the View-owned engine, including Policy fallback, filtering, deterministic ordering, limit clamping, sanitization, redaction, and relation hydration. Action steps share one entity resolver and enforce declared read/write effects before handlers execute. A context-specific value-source owner now serves expressions, Actions, Blocks, Pages, compiler validation, and inspection and fails closed for unsupported sources.

The client has typed Page/Panel/Region/Block dispatch, explicit unknown-component and expression-operator failures, and one Action encoder/caller for JSON, multipart, Webform adaptation, and deterministic batches; Admin and ActionRunner expose field errors. Definition-kind entries now own compilation, explicit-phase normalization and per-kind validation, inspection, schema type, and AppIR storage metadata; completeness is checked against an independent AppIR storage contract. Agent Protocol metadata and handlers are sealed together, test overrides use construction, and the unreachable not-implemented state is removed. Confirmed zero-caller `internal/entity` and `internal/user` pass-through packages were deleted.

Qualification passes: `make check` including Go race tests, 30 frontend tests, black-box contracts, and 14/14 Playwright journeys; `make test-crash`; `make test-postgres` including the PostgreSQL blog journey; and `make build`. The definition -> validation -> migration -> immutable AppIR -> atomic activation lifecycle, View-read/Action-write boundary, backend confinement, maintained examples, and public envelopes remain intact.

## v0.9 completed state

Bean v0.9 Semantic Application Model is complete on top of merge commit `c51705d`. The scoped first slice adds exactly one first-class primitive, `Lifecycle`, derived from the maintained ATS candidate pipeline and commerce order flow. Milestones 0–5 are done.

Lifecycle owns one Entity enum state field, its initial state, and a reachable canonical transition graph. The compiler validates Action bindings and optional Policy-specific graph subsets with stable `BEAN-E2202`/`BEAN-E2201` diagnostics; immutable AppIR, capabilities, canonical schema, named inspection, references, and semantic diff expose the same model through the shared CLI/MCP Agent Protocol. Create Actions inject the initial state, generic and transaction updates cannot change it, and only a Lifecycle-bound transition path can follow the graph after existing Policy checks.

ATS candidates and commerce orders now use the shared primitive with no application-name branch. DemoSeed creates Lifecycle records at their initial state, preflights every generated transition path and typed Action input before writes, rejects transaction side effects or unmodeled field mutations, and reaches deterministic generated states through compatible Actions. Focused compiler, AppIR v2 compatibility, Action, release/restart, DemoSeed, Agent Protocol, CLI/MCP parity, HTTP, React, and complete Go tests pass; legacy Action-local transitions and AppIR v1 releases without Lifecycle semantics remain covered. `make check` passes including race, black-box contracts, 25 React tests, and 14/14 Playwright journeys; `make test-crash`, `make test-postgres`, and `make build` pass, and the binary reports `bean 0.9.0-alpha`. All actionable review findings are fixed, answered, and resolved. Ownership, auditability, soft deletion, terminal-state immutability, rules, generated tests, and extensions remain outside v0.9.

## v0.8 completed state

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
- v0.11 adds first-class Semantic Test Suites for deterministic Rule and Action behavior; v0.12 generates tests through that contract; v0.13 adds the typed extension boundary.
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
