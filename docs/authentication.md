# Authentication configuration

Bean includes local password login/logout, database-backed sessions, and opt-in fixed-role registration. No email provider or OAuth credentials are required to run an internal application.

## Explicit application configuration

```yaml
kind: Authentication
name: auth
preset: internal
registration: false
```

There is at most one `Authentication`, named `auth`. `preset` is required and accepts `local`, `internal`, or `public`. In this first configuration slice all three have the same conservative effective default: registration is disabled. Preset names describe deployment intent; **`public` does not yet enable email verification, recovery, or claim production readiness**.

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

Configuration is validated before publication and stored in immutable AppIR v16. Failed publication does not change active capabilities. Earlier AppIR releases remain loadable, but cannot contain the new configuration. Republish definitions to activate a change; do not edit an active snapshot.

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

## Security and future slices

Password hashing, authorization, session protections, CSRF, and throttling are not optional switches. Existing Secure-cookie and trusted-proxy host settings still need correct deployment configuration.

Email verification/recovery, invitations, per-device session listing, OIDC, and MFA remain planned. Their configuration keys are currently rejected, not accepted as inert feature flags. Advanced features will only become available alongside working backend enforcement, delivery where needed, and negative tests. Local/internal applications will retain an email-independent administration path.
