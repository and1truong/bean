> Completed plans are archived in [`docs/plans/completed.md`](docs/plans/completed.md).

# Visual agentic browser testing

Contract: `GOAL.md`. Status: in progress. Epic: <https://github.com/and1truong/bean/issues/20>; slices ship as PRs into `epic/browser-testing`.

| Slice | Deliverable | Ticket | Verification | Status |
| --- | --- | --- | --- | --- |
| 1 | Scenario definition kind + compiler → `App.Scenarios` (AppIR v21) | #23 | compiler/schema/AppIR tests | done |
| 2 | Durable run model (Run → Session → StepExecution → Artifact/Event) | #24 | migration/runtime tests | done |
| 3 | Semantic Browser API | #25 | API contract tests | done |
| 4 | Playwright adapter + session lifecycle | #26 | adapter/e2e tests | done |
| 5 | Run event stream + artifacts | #27 | stream/persistence tests | done |
| 6 | Scenario & run HTTP API | #28 | HTTP/contract tests | done |
| 7 | Browser security boundary | #29 | isolation/egress tests | done |
| 8 | Studio live run view | #30 | vitest UI tests | done |
| 9 | Studio scenario graph editor | #31 | vitest UI tests | done |
| 10 | Pause / takeover / resume | #32 | run-control tests | done |
| 11 | NL → scenario generation | #33 | generation tests | done |
| 12 | Exploration → saved test | #34 | capture tests | done |
| 13 | Failure diagnosis + agent repair | #35 | diagnosis tests | pending |
| 14 | App-driven test generation | #36 | generation tests | done |

# Extended semantic content and composition

Contract: `GOAL.md`. Status: complete.

| Slice | Deliverable | Verification | Status |
| --- | --- | --- | --- |
| 1 | AppIR v20 scaffold; heading levels, ordered list, safe link, divider through all content seams | focused AppIR/compiler/schema/projection/Vitest tests | done |
| 2 | Literal static table and density integration | compiler/schema/render/accessibility tests | done |
| 3 | Compatible image plus audio, YouTube, playlist and visibility lifecycle | URL/compiler/render/network/lifecycle tests | done |
| 4 | Tabs Block compiler, projection, keyboard behavior, print and media reset | compiler/schema/projection/Vitest/Playwright tests | done |
| 5 | `choices` in named, inline and tab content with isolated/resettable state | compiler/schema/projection/Vitest/Playwright tests | done |
| 6 | Presentation examples, docs, compatibility/activation/browser matrix and full qualification | generated-schema parity, browser journeys, `make check`, `make build` | done |

# Content Block authoring guide

Contract: `GOAL.md`. Status: complete.

| Milestone | Deliverable | Verification | Status |
| --- | --- | --- | --- |
| 1 | A task-oriented Content Block reference covering placement, elements, fields, defaults, limits, and safe image sources | documentation/source review | done |
| 2 | Links from the application and definition guides; remove duplicated low-level detail | link review | done |
| 3 | Repository qualification | `make check` and `make build` | done |

# Configurable authentication

Status: complete for the current scope (milestones 1–4). Onboarding/invitations are deferred at the user's request; OIDC/MFA remain outside scope.

| Milestone | Deliverable | Verification | Status |
| --- | --- | --- | --- |
| 0 | Freeze optional-feature, security, and compatibility contract | repository review | done |
| 1 | Login pending/error UX, duplicate-submit guard, password hints and visibility | 96 frontend tests; full make check and make build pass | done |
| 2 | Presets and registration override through compiler, immutable AppIR v16, UI and backend | format/legacy compatibility, off/on/off activation, persisted reload, invalid publish, HTTP/Webform/direct Action denial, OpenAPI and rendered form tests; full gates | done |
| 3 | Password change, session revocation, email-independent host recovery | built-in Account Actions, CSRF/unknown-field/isolation/throttle tests, audit-failure rollback, stale-login/reset regression, CLI stdin and ordinary-member browser journey; make check, make build and PostgreSQL parity pass | done |
| 4a | Opt-in email password recovery with encrypted durable delivery | expiry/replay/concurrency/rollback/retry, non-enumerating requests, SMTP STARTTLS browser, make check/build and PostgreSQL parity | done |
| 4b | Email ownership verification with enforced account state | registration/login/session, rollback, token purpose/expiry/concurrency, SMTP browser and PostgreSQL parity; make check/build pass; `docs/plans/auth-email-verification.md` | done |
| 5 | Application onboarding/invitations | excluded from current scope at the user's request | deferred |

Presets keep signup/recovery/verification off by default. `passwordRecovery` (AppIR v17) and `emailVerification` (AppIR v19) require host delivery; MFA and other unimplemented switches remain rejected. Milestone 4a contract: `docs/plans/auth-email-recovery.md`. `public` is deployment intent, not a production-readiness claim. Runtime details: `docs/authentication.md`.

Do not expose configuration switches for unimplemented security mechanisms. Local/internal operation must remain email-independent; self-registration stays opt-in for every preset.

# Blog Admin browser investigation

Status: deferred. The user switched to ATS after successful fresh-demo Category/Post create/edit/publish verification. Investigation of the existing Blog credentials remains deferred.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Inspect the running demo and prepare Blog testing | server and database identity, setup | deferred |
| 1 | Reproduce Admin behavior through login and editorial navigation | browser observations | pending |
| 2 | Fix confirmed defects and qualify | focused tests, browser verification, `make check`, `make build` | pending |
