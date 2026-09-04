# Goal: configurable, DX-friendly authentication

Status: in progress. Milestones 1–3 complete and qualified with `make check` and `make build`; Account Actions also pass PostgreSQL parity. Delivery-backed verification/recovery and onboarding slices remain pending.

Provide built-in account lifecycle features without requiring public-app onboarding or email infrastructure for local/internal applications. Preserve existing applications when no new auth configuration is declared.

## Contract

- Preserve definition → validation → migration → immutable AppIR → atomic activation.
- Keep application choices and presentation in metadata; core auth mechanisms remain generic.
- Offer `local`, `internal`, and `public` presets with explicit per-feature overrides. Presets do not silently enable self-registration or grant roles.
- Advanced features are optional: email verification, email recovery, invitations, OIDC, and MFA. Disabling a feature closes its backend execution paths as well as hiding UI entry points.
- Password hashing, authorization, CSRF, safe sessions, and abuse protection are not optional feature flags.
- Local/internal applications need no email provider. Preserve administrator provisioning; add a secure email-independent recovery path.
- Reject unsupported features and inconsistent configuration; never advertise verification/recovery without working delivery. Host secrets never enter metadata or AppIR.
- Reads use Views and account mutations use typed Actions. Preserve sensitive-input handling, transaction boundaries, and session/cache isolation.

## Milestones

1. Improve existing login UX with pending/error states, duplicate-submit protection, password-manager hints, and accessible password visibility controls. No backend or configuration changes in this slice.
2. Specify and implement metadata presets, overrides, compiled effective capabilities, compatibility, and UI/backend enforcement for supported features. Do not add inert security flags.
3. Add authenticated password changes and session revocation, then administrator recovery without email.
4. Add delivery-backed verification and single-use, expiring password recovery with enumeration/abuse defenses. Require an explicit delivery contract before enabling these features.
5. Add application onboarding/invitations; evaluate OIDC and MFA as separate vertical slices.

## Completion

Each milestone needs focused tests, including negative enforcement tests where relevant. Run `make check` and `make build` for each delivered slice; keep `PLANS.md` and `docs/progress.md` current. The entire goal is not complete until the agreed core lifecycle and configuration slices are implemented.

Previous completed goal: `docs/plans/contextual-admin-route-records.md`.
