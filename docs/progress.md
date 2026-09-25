# Progress

> Completed work and its verification evidence are indexed in [`docs/plans/completed.md`](plans/completed.md) and stored under [`docs/plans/archive/`](plans/archive/). This file tracks only active, proposed, or deferred work.

## Visual agentic browser testing

Status: in progress (slice 1 of 14). Contract: [`GOAL.md`](../GOAL.md). Epic: <https://github.com/and1truong/bean/issues/20> — sub-issues #23–#36 ship as PRs into `epic/browser-testing`, which merges to main as one umbrella PR at the end.

- Slice 1 (ticket #23) added the `Scenario` definition kind: `internal/scenario` publishes the closed node-type, condition, assertion, and bound contract; the compiler validates source shape per node type, required fields, edge integrity (dangling `next`/`onFail`/`body`/`branches[].next` targets and unreachable nodes are `BEAN-E2891` diagnostics, never panics), and `api_call` Action references, then compiles into immutable `App.Scenarios` on AppIR v21 (`ScenarioFormat`).
- `fill` accepts `text` XOR `secret` (secret resolves at execution time, outside AppIR); `pause` reserves the takeover seam for slice 10.
- Generated `schemas/scenario.schema.json` via `go run ./cmd/bean schema --output schemas --json`; capabilities publish node types, conditions, assertions, and bounds. Format gate: scenarios require v21 exactly; pre-v21 snapshots reject them.
- Verification: `go test ./internal/compiler ./internal/appir ./cmd/bean` passes, including the v21 format compatibility matrix and canonical-schema drift checks.

## Extended semantic content and composition

Status: complete. Contract: [`GOAL.md`](../GOAL.md).

- Added the closed semantic vocabulary through source presence checks, deterministic compiler validation/defaults, AppIR v20, shared projection and React: heading levels, ordered lists, safe links, dividers, literal tables, compatible images, explicit-load audio/YouTube/playlist, and single-choice `choices`.
- Added `Block type: tabs` at the existing Block reference seam with typed bounded tab content, Policy/projection support, orientation-aware keyboard behavior, per-instance state, inactive-content reset, media unload, and complete print output. Named Blocks, inline Panel content, and tab content share the same closed element contract and schema.
- Generated schemas and CLI/protocol capabilities publish the new enums, bounds, source policy, and v20 format. Compiler/schema/AppIR/release tests cover source types and locations, URL/ID/table/tab/quiz rules, density, historical/current/future format gates, immutable round trips, and invalid-publication atomicity.
- Extended `/presentations/bean` to 17 metadata-only frames across eight chapters while retaining all prior stable frames, inline composition, `product_statement`, and the live chart. Local image/audio assets and intercepted YouTube fixtures keep browser tests deterministic.
- Focused verification passes: semantic compiler/content/block/release/AppIR Go tests; 70 semantic/App Vitest tests; and both presentation Playwright journeys. Generated schemas were refreshed with `go run ./cmd/bean schema --output schemas --json`.
- Final qualification passes: `make check` (all Go/unit/integration/contracts/fuzz-smoke/compatibility/black-box/race checks, 117 Vitest tests, and 28 Playwright journeys) and `make build`.

## Content Block authoring guide

Status: complete. Contract: [`GOAL.md`](../GOAL.md).

- Added [Content Blocks](content-blocks.md), a reader-oriented reference for closed-vocabulary content placement, fields, defaults, bounds, image safety rules, and examples.
- Linked it from the README, application guide, and definition reference; CLI schema/capabilities output remains the version-exact machine reference.
- `make check` and `make build` pass.

## Configurable authentication

Status: complete for the current scope. Milestones 1–4 (account security, email recovery and verification) are complete. The user deferred onboarding/invitations; no further auth implementation is scheduled.

- Login now has duplicate-submit protection, pending/error behavior, password-manager hints, and an accessible password visibility control.
- Optional `Authentication` metadata compiles into immutable AppIR v16. Registration defaults off for explicit `local`, `internal`, and `public` presets, while omitted configuration preserves legacy behavior. Disabled registration is blocked across UI, HTTP, Webform, direct Action execution, and OpenAPI.
- Signed-in users can change their password or revoke all sessions from the Account page. Trusted host operators have an email-independent password-reset CLI. Account mutations preserve transactional audit/session behavior and pass SQLite and PostgreSQL coverage.
- Opt-in `passwordRecovery` compiles into AppIR v17; enabled publication/startup requires host mail configuration. Known and unknown valid addresses use the same encrypted durable request path and generic response. A committed worker issues 15-minute tokens, stores only digests, and queues AES-GCM-encrypted SMTP delivery. STARTTLS and certificate verification are mandatory. The browser strips fragment credentials and redeems only on explicit POST; password replacement consumes tokens and revokes sessions atomically. Release replacement/disable cancels older intents and links. Retry safety retains consumed receipts; automatic retention cleanup is not yet implemented.
- Opt-in `emailVerification` uses AppIR v19, preserving v18 field layouts and all earlier feature boundaries. Existing/new accounts start unverified; enabling blocks unverified login and session use, including built-in Account Actions. Registration queues verification atomically; generic resend handles existing accounts without exposing their status. Confirmation requires purpose-bound token plus current password, records verification, consumes tokens, revokes sessions and audits atomically. Disabling returns to email-independent login; recovery does not implicitly verify ownership.
- Latest qualification: `make check` passes 108 frontend tests, 27 browser journeys and Go/race/contracts; `make build` passes. `make test-postgres` passes recovery/verification/account HTTP parity and the Blog journey. Verification tests cover current/wrong password, expiry/purpose/release separation, concurrent consumption, audit/enqueue rollback, old sessions, disabled APIs/direct Actions/outbox and real local STARTTLS for existing and newly registered accounts. The PostgreSQL fixture uses a separate application ID to avoid deleting unrelated test Entities.
- Deferred: onboarding/invitations (milestone 5), at the user's request. OIDC/MFA remain outside scope. Unsupported switches continue to fail compilation rather than acting as inert flags. This scope closure changes documentation only; the qualification above applies to the unchanged runtime.

Runtime and security details: [`docs/authentication.md`](authentication.md).

## Blog Admin browser investigation

Status: deferred.

Fresh-demo Category/Post create, edit, and publish flows were verified before work switched to ATS. Investigation of the older Blog instance's credentials remains deferred; no Blog runtime or authentication change was made.
