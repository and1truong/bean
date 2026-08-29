# Security

Passwords use bcrypt and never leave the authentication module. Sessions are database-backed HttpOnly SameSite=Lax cookies with configurable Secure mode; cookie mutations require a per-session CSRF header. Login attempts are rate-limited and request bodies are capped at 1 MiB.

Policy checks are server-side. Tenant/owner constraints become DBAL predicates, field redaction occurs before rendering, write access defaults to deny, and administrator access is explicit. Queries parameterize values and validate identifiers. Rich text strips scripts and unsafe JavaScript URLs. View results cap at 200. Uniform errors include a request ID and omit database details.
