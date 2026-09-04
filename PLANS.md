> Completed plans are archived in [`docs/plans/completed.md`](docs/plans/completed.md).

# Configurable authentication

Contract: `GOAL.md`. Status: in progress. Existing compiler/AppIR/sequence working-tree edits are unrelated and must be preserved.

| Milestone | Deliverable | Verification | Status |
| --- | --- | --- | --- |
| 0 | Freeze optional-feature, security, and compatibility contract | repository review | done |
| 1 | Login pending/error UX, duplicate-submit guard, password hints and visibility | 96 frontend tests; full make check and make build pass | done |
| 2 | Presets and registration override through compiler, immutable AppIR v16, UI and backend | format/legacy compatibility, off/on/off activation, persisted reload, invalid publish, HTTP/Webform/direct Action denial, OpenAPI and rendered form tests; full gates | done |
| 3 | Password change, session revocation, email-independent host recovery | built-in Account Actions, CSRF/unknown-field/isolation/throttle tests, audit-failure rollback, stale-login/reset regression, CLI stdin and ordinary-member browser journey; make check, make build and PostgreSQL parity pass | done |
| 4 | Email verification and expiring single-use recovery | delivery, replay, enumeration and throttle tests | pending |
| 5 | Application onboarding/invitations; evaluate OIDC/MFA | metadata-driven vertical journeys | pending |

Milestone 2 deliberately exposes only the supported registration override. All presets currently share the conservative signup-off baseline; email verification/recovery, MFA, and other unimplemented switches are rejected. `public` is deployment intent, not a production-readiness claim. Runtime details: `docs/authentication.md`.

Do not expose configuration switches for unimplemented security mechanisms. Local/internal operation must remain email-independent; self-registration stays opt-in for every preset.

# Blog Admin browser investigation

Status: deferred. The user switched to ATS after successful fresh-demo Category/Post create/edit/publish verification. Investigation of the existing Blog credentials remains deferred.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Inspect the running demo and prepare Blog testing | server and database identity, setup | deferred |
| 1 | Reproduce Admin behavior through login and editorial navigation | browser observations | pending |
| 2 | Fix confirmed defects and qualify | focused tests, browser verification, `make check`, `make build` | pending |
