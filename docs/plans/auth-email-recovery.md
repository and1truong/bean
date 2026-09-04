# Email-backed account recovery — milestone 4a

Status: complete. `make check` (102 frontend tests, 25 browser journeys), `make build`, and `make test-postgres` pass. Email verification remains milestone 4b, not an inert configuration switch.

## Contract

- `Authentication.passwordRecovery` is opt-in in every preset. Existing releases and local/internal apps need no email setup.
- A host-only SMTP configuration supplies a fixed public origin, sender, credentials and 32-byte encryption key. Require STARTTLS with certificate verification; never derive links from request Host or forwarded headers.
- Publication/startup of an enabled release requires configured delivery. Offline compilation/inspection stays independent of deployment secrets.
- Anonymous request Actions enqueue an encrypted request for every syntactically valid email, without looking up accounts or contacting SMTP. Respond identically for known/unknown accounts; independently bound IP and destination request rates. Do not put addresses/tokens in responses, logs, audits or unencrypted outbox payloads.
- A committed request worker generates 256-bit randomness, stores only a token digest, and atomically enqueues an authenticated-encrypted delivery intent. A durable request ID prevents reissuing on worker retries. SMTP runs only after that transaction commits, with bounded timeout/retries and sanitized failure categories. Delivery is at-least-once, not exactly-once.
- Reset links place the token in a URL fragment, not a query/path. GET never consumes a token. Explicit POST requires token plus matching new password/confirmation.
- Tokens expire after 15 minutes, bind user/purpose/active release, and are single-use under the same user-row lock as login and other account Actions. Reset atomically changes password, consumes outstanding tokens, revokes sessions and writes a secret-free audit. No automatic login or role/email/tenant change.
- Every password replacement (including host recovery) invalidates outstanding reset tokens. Disabled/replaced releases cannot request, deliver or redeem recovery. Already-authorized in-flight operations retain request-snapshot semantics.
- Production durability inherits the existing outbox runner. Pending/failed intents and consumed token rows retain only encrypted/digested secrets; retention/host-key rotation must be documented rather than claiming automatic cleanup.

## Qualification

Compiler/schema/format compatibility; host readiness before activation; migration/restart; request enumeration behavior; token expiry, replay, tamper, wrong release and password-change invalidation; rollback and retries; SMTP no-STARTTLS refusal and error redaction; UI/fragment handling; SQLite/PostgreSQL and browser journey; `make check` and `make build`.
