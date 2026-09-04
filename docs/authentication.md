# Authentication configuration

Bean includes local password login/logout, database-backed sessions, and opt-in fixed-role registration. No email provider or OAuth credentials are required to run an internal application.

## Explicit application configuration

```yaml
kind: Authentication
name: auth
preset: internal
registration: false
```

There is at most one `Authentication`, named `auth`. `preset` is required and accepts `local`, `internal`, or `public`. All three have the same conservative effective defaults: registration and password recovery are disabled. Preset names describe deployment intent; **`public` does not automatically enable email features or claim production readiness**.

To opt into registration, set `registration: true` and retain a valid `LocalRegistration` definition pointing to a `register_local_user` Action with a compiler-validated, non-privileged default role. The preset never creates a role, selects a tenant, or creates a workspace.

```yaml
kind: Authentication
name: auth
preset: public
registration: true
```

See `examples/blog/access.yaml` for the complete registration Action, role, form, and Page. With registration disabled you can keep those definitions in source for later activation: generated navigation, registration Blocks, and OpenAPI no longer advertise it. HTTP Action and Webform submissions and direct Action callers are denied. Authored explanatory text or custom static links are not rewritten automatically.

## Compatibility and activation

Without `Authentication`, existing behavior is preserved: `LocalRegistration` is the signup opt-in. With explicit `Authentication`, `registration` defaults to false, even when `LocalRegistration` exists. Login, logout, administrator provisioning, and current sessions are unaffected by toggling registration; existing accounts are not deleted.

Configuration is validated before publication and stored in immutable AppIR (v16 for the initial Authentication contract, v17 for password recovery). Failed publication does not change active capabilities. Earlier AppIR releases remain loadable, but cannot contain the new configuration. Republish definitions to activate a change; do not edit an active snapshot.

## Account security (no email required)

Every signed-in user can open **Account** in the header (`/admin/system/account`). This built-in identity page is available to ordinary members, not only administrators, and remains available when self-registration is disabled. It does not change application roles or expose other accounts.

- **Change password** requires the current password and matching new password/confirmation. New passwords are 10–72 bytes (bcrypt's upper bound). Success revokes every session, including the current browser, then returns to login.
- **Sign out all devices** asks for confirmation and revokes every session for the current account without changing the password.
- Both operations require a valid session and CSRF token and have a bounded throttle independent of login/signup. Bodies reject unknown fields; clients cannot select a user ID. Built-in Account Actions perform the mutation, session revocation, and success audit in one transaction. They never store password inputs or input fingerprints in generic Action idempotency records.
- Session creation and account mutations serialize on the user row; a login that checked the old password before a concurrent reset cannot create a valid session afterward. Account Actions revalidate the session inside the transaction. Revocation cannot cancel requests that were already authorized elsewhere before it committed.

The auth subsystem retains private credential reads; no password/hash is exposed through an application View, account response, or audit. Account operations require no application metadata or active release and do not add a schema migration.

### Host-operated password recovery

A trusted host operator with database access can recover an existing local account without email:

```sh
bean user reset-password --db ./bean.db --email member@example.test --password-stdin < /secure/path/new-password
```

Supply the password through a protected file or secret-manager pipe, **not a literal command argument or shell-history command**. Standard input is bounded and accepts one optional trailing LF/CRLF; spaces are preserved. The command refuses missing users, does not grant roles or change tenants, revokes all sessions atomically, and audits `system_password_reset` with actor `host_operator`. Database URL selection works as for `user create`. There is no HTTP endpoint for this host-only operation. Protect database credentials and the input source; remove any temporary secret file safely after use.

## Email password recovery (opt-in)

Set `passwordRecovery: true` on `Authentication`, independently of registration. No email dependency is introduced when it is false or omitted. The login page then offers **Forgot password?**. The compiler validates metadata without secrets; publication and normal startup additionally require configured host delivery. Read-only inspection does not require credentials.

Configure the host's `BEAN_AUTH_EMAIL` environment variable as a JSON object through a secret manager:

```json
{
  "address": "smtp.example.test:587",
  "username": "smtp-user",
  "password": "HOST_SECRET",
  "from": "accounts@example.test",
  "origin": "https://app.example.test",
  "key": "BASE64_ENCODED_RANDOM_32_BYTE_KEY"
}
```

Generate a random encryption key (for example `openssl rand -base64 32`) and retain it securely across restarts. SMTP requires STARTTLS, TLS 1.2+, and certificate/hostname verification; there is no plaintext fallback. For private SMTP CAs, an optional `rootCAFile` points to a PEM trust file. The public `origin` must be an HTTPS origin without credentials/path/query; HTTP is allowed only for loopback development. Links never trust request Host/forwarded headers. Host settings stay out of definitions/AppIR.

- `POST /api/auth/recovery/request` accepts only `{ "email": "..." }`. Every syntactically valid email follows the same encrypted enqueue path and receives the same 202 response, whether the account exists or not. IP and destination limits are bounded and independent from login. Per-destination suppression retains the generic response.
- The worker resolves the account and atomically records a token digest plus an AES-GCM encrypted delivery intent. SMTP is outside the transaction. Request receipts prevent duplicate token issuance on worker retries. Each stage has three attempts, 30-second retry delays, and SMTP has a 10-second deadline. Delivery is **at-least-once**; a retry can deliver the same link again. SMTP failures persist only a generic error in outbox status.
- Tokens use 256-bit randomness and expire 15 minutes after the request. The link uses `/login?recovery=reset#token=...`; the browser removes the fragment and retains the token only in component memory. Reloading afterward requires reopening the email link. GET/mount never redeems a token.
- `POST /api/auth/recovery/reset` requires `{ "token": "...", "password": "...", "confirmation": "..." }`. It atomically replaces the password, consumes all outstanding tokens for the account, revokes sessions and audits success. It does not auto-login, grant roles, or mutate an incidental browser cookie belonging to another account.
- Password changes and host recovery also invalidate issued tokens. Tokens and pending messages bind the active release; **republishing invalidates earlier links and discards earlier queued requests/deliveries**. Disabling recovery hides entry points and denies both HTTP and direct Actions.

Metadata startup adds `bean_auth_token` and its user index. Token rows store digests, not bearer tokens. Outbox requests/delivery payloads are authenticated-encrypted under the host key; do not treat database backups as containing usable plaintext links. Consumed token receipts and outbox history are retained for retry safety; automatic retention cleanup is not implemented. Drain pending mail before rotating the host encryption key, or expect old envelopes to fail decryption. Operators can inspect sanitized outbox status for delivery failures; there is no claim that accepted requests guarantee email delivery.

## Security and future slices

Password hashing, authorization, session protections, CSRF, and throttling are not optional switches. Existing Secure-cookie and trusted-proxy host settings still need correct deployment configuration.

Email verification, invitations, per-device session listing, OIDC, and MFA remain planned. Their configuration keys are currently rejected, not accepted as inert feature flags. Advanced features will only become available alongside working backend enforcement, delivery where needed, and negative tests. Local/internal applications will retain an email-independent administration path.
