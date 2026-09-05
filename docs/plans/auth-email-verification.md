# Email verification — milestone 4b

Status: complete. `make check` (108 frontend tests, 27 browser journeys), `make build`, and PostgreSQL parity pass. AppIR v19 preserves the already-completed v18 field-layout feature.

- `Authentication.emailVerification` is opt-in, independent of signup/recovery, and requires the same configured host email transport before publication/startup.
- New and existing local accounts start unverified; enabling does not silently trust old addresses, including host-provisioned administrators. Provide generic resend so operators/users can verify existing accounts. Local/internal presets remain email-independent unless explicitly enabled.
- Registration atomically creates the account and encrypted verification request. Resend uses identical queued responses for unknown, verified and unverified addresses. Reuse bounded delivery, topic-separated encryption and durable receipts.
- Verification tokens are purpose- and release-bound, 15-minute, one-time credentials. Explicit confirmation requires both the email token and the account's current password, preventing an unsolicited verification click from enabling an attacker-created account. GET/mount never verifies.
- Confirmation marks the account's current email verified, consumes verification tokens, revokes all sessions and audits in one account-locked transaction. No automatic login, privilege changes or email changes. Password recovery does not implicitly mark an address verified.
- Login and session resolution enforce verification while enabled, including direct built-in Account Actions. Correct-password/unverified login gets a verification-required response; unknown/wrong-password attempts retain generic credential errors.
- Disabling gates API, direct Action and queued delivery, while removing login's verification link. Replacing a release invalidates its pending links. Existing issued-token invalidation on password replacement remains conservative: resend after resetting a password.
- Ship/compiler/migration/token/rollback/SQLite/PostgreSQL/React/browser negative tests, `make check`, `make build`, then commit this slice separately.
