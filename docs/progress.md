# Progress

> Completed work and its verification evidence are indexed in [`docs/plans/completed.md`](plans/completed.md) and stored under [`docs/plans/archive/`](plans/archive/). This file tracks only active, proposed, or deferred work.

## Configurable authentication

Status: in progress. Contract: [`GOAL.md`](../GOAL.md). Milestones 1–3 and 4a (email password recovery) are complete.

- Login now has duplicate-submit protection, pending/error behavior, password-manager hints, and an accessible password visibility control.
- Optional `Authentication` metadata compiles into immutable AppIR v16. Registration defaults off for explicit `local`, `internal`, and `public` presets, while omitted configuration preserves legacy behavior. Disabled registration is blocked across UI, HTTP, Webform, direct Action execution, and OpenAPI.
- Signed-in users can change their password or revoke all sessions from the Account page. Trusted host operators have an email-independent password-reset CLI. Account mutations preserve transactional audit/session behavior and pass SQLite and PostgreSQL coverage.
- Opt-in `passwordRecovery` compiles into AppIR v17; enabled publication/startup requires host mail configuration. Known and unknown valid addresses use the same encrypted durable request path and generic response. A committed worker issues 15-minute tokens, stores only digests, and queues AES-GCM-encrypted SMTP delivery. STARTTLS and certificate verification are mandatory. The browser strips fragment credentials and redeems only on explicit POST; password replacement consumes tokens and revokes sessions atomically. Release replacement/disable cancels older intents and links. Retry safety retains consumed receipts; automatic retention cleanup is not yet implemented.
- Latest qualification: `make check` passes 102 frontend tests, 25 browser journeys and Go/race/contracts; `make build` passes. `make test-postgres` passes recovery/account HTTP parity and the Blog journey. Coverage includes expired/wrong-release/replayed tokens, concurrent redemption, audit rollback, delivery error redaction/retry, host readiness and a real local STARTTLS browser journey.
- Next: email ownership verification with enforced registration/login/session state (milestone 4b). Invitations, OIDC, and MFA remain later slices. Unsupported switches continue to fail compilation rather than acting as inert flags.

Runtime and security details: [`docs/authentication.md`](authentication.md).

## Typed detail/form field layout

Status: implemented, reconciled and qualified; awaiting the main merge intentionally withheld by the user. Contract: [`docs/plans/typed-field-layout.md`](plans/typed-field-layout.md).

`feat/typed-field-layout` is based on committed Recovery `8b0129a`, not the temporary snapshot used during development. AppIR v18 field layouts coexist with v15 Sequence, v16 Authentication and v17 Recovery. All 608 compatibility combinations and actual v14–v17 fixture upgrades pass, including database reopen, zero physical migrations and failed-publication isolation. `make check` passes 107 frontend tests and 26 browser journeys; explicit `make build` passes. The combined Recovery journey resets a password and opens grouped Blog controls in the same application. Main's runtime, index, databases and `GOAL.md` are unchanged; documentation archival is preserved. See [`compatibility report`](reports/field-layout-compatibility.md).

## Blog Admin browser investigation

Status: deferred.

Fresh-demo Category/Post create, edit, and publish flows were verified before work switched to ATS. Investigation of the older Blog instance's credentials remains deferred; no Blog runtime or authentication change was made.
