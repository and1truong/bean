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

## Blog Admin browser investigation

Status: deferred.

Fresh-demo Category/Post create, edit, and publish flows were verified before work switched to ATS. Investigation of the older Blog instance's credentials remains deferred; no Blog runtime or authentication change was made.
