# Progress

> Completed work and its verification evidence are indexed in [`docs/plans/completed.md`](plans/completed.md) and stored under [`docs/plans/archive/`](plans/archive/). This file tracks only active, proposed, or deferred work.

## Configurable authentication

Status: in progress. Contract: [`GOAL.md`](../GOAL.md). Milestones 1–3 are complete.

- Login now has duplicate-submit protection, pending/error behavior, password-manager hints, and an accessible password visibility control.
- Optional `Authentication` metadata compiles into immutable AppIR v16. Registration defaults off for explicit `local`, `internal`, and `public` presets, while omitted configuration preserves legacy behavior. Disabled registration is blocked across UI, HTTP, Webform, direct Action execution, and OpenAPI.
- Signed-in users can change their password or revoke all sessions from the Account page. Trusted host operators have an email-independent password-reset CLI. Account mutations preserve transactional audit/session behavior and pass SQLite and PostgreSQL coverage.
- Latest qualification for milestone 3: `make check` and `make build` pass; account HTTP parity and the Blog journey pass under `make test-postgres`.
- Next: delivery-backed email verification and expiring single-use recovery. Invitations, OIDC, and MFA remain later slices. Unsupported switches continue to fail compilation rather than acting as inert flags.

Runtime and security details: [`docs/authentication.md`](authentication.md).

## Typed detail/form field layout

Status: proposed. Contract: [`docs/plans/typed-field-layout.md`](plans/typed-field-layout.md).

The proposed first slice groups Blog Post Admin controls, then reuses the bounded layout shape for a read-only detail Display. Existing field membership, View projection, Action submission, derived/protected controls, and legacy rendering remain authoritative. Implementation awaits coordination with concurrent AppIR format work; no runtime change or qualification is claimed.

## Blog Admin browser investigation

Status: deferred.

Fresh-demo Category/Post create, edit, and publish flows were verified before work switched to ATS. Investigation of the older Blog instance's credentials remains deferred; no Blog runtime or authentication change was made.
