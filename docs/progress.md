# Progress

> Completed work and its verification evidence are indexed in [`docs/plans/completed.md`](plans/completed.md) and stored under [`docs/plans/archive/`](plans/archive/). This file tracks only active, proposed, or deferred work.

## Configurable authentication

Status: in progress. Contract: [`GOAL.md`](../GOAL.md). Milestones 1–4 (account security, email recovery and verification) are complete.

- Login now has duplicate-submit protection, pending/error behavior, password-manager hints, and an accessible password visibility control.
- Optional `Authentication` metadata compiles into immutable AppIR v16. Registration defaults off for explicit `local`, `internal`, and `public` presets, while omitted configuration preserves legacy behavior. Disabled registration is blocked across UI, HTTP, Webform, direct Action execution, and OpenAPI.
- Signed-in users can change their password or revoke all sessions from the Account page. Trusted host operators have an email-independent password-reset CLI. Account mutations preserve transactional audit/session behavior and pass SQLite and PostgreSQL coverage.
- Opt-in `passwordRecovery` compiles into AppIR v17; enabled publication/startup requires host mail configuration. Known and unknown valid addresses use the same encrypted durable request path and generic response. A committed worker issues 15-minute tokens, stores only digests, and queues AES-GCM-encrypted SMTP delivery. STARTTLS and certificate verification are mandatory. The browser strips fragment credentials and redeems only on explicit POST; password replacement consumes tokens and revokes sessions atomically. Release replacement/disable cancels older intents and links. Retry safety retains consumed receipts; automatic retention cleanup is not yet implemented.
- Opt-in `emailVerification` uses AppIR v19, preserving v18 field layouts and all earlier feature boundaries. Existing/new accounts start unverified; enabling blocks unverified login and session use, including built-in Account Actions. Registration queues verification atomically; generic resend handles existing accounts without exposing their status. Confirmation requires purpose-bound token plus current password, records verification, consumes tokens, revokes sessions and audits atomically. Disabling returns to email-independent login; recovery does not implicitly verify ownership.
- Latest qualification: `make check` passes 108 frontend tests, 27 browser journeys and Go/race/contracts; `make build` passes. `make test-postgres` passes recovery/verification/account HTTP parity and the Blog journey. Verification tests cover current/wrong password, expiry/purpose/release separation, concurrent consumption, audit/enqueue rollback, old sessions, disabled APIs/direct Actions/outbox and real local STARTTLS for existing and newly registered accounts. The PostgreSQL fixture uses a separate application ID to avoid deleting unrelated test Entities.
- Next: metadata-driven onboarding/invitations (milestone 5); OIDC and MFA remain later evaluation slices. Unsupported switches continue to fail compilation rather than acting as inert flags.

Runtime and security details: [`docs/authentication.md`](authentication.md).

## Blog Admin browser investigation

Status: deferred.

Fresh-demo Category/Post create, edit, and publish flows were verified before work switched to ATS. Investigation of the older Blog instance's credentials remains deferred; no Blog runtime or authentication change was made.
