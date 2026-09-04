# Completed v0.5 plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Role/policy, content, binding, sensitive-input, and browser contracts | compiler/HTTP/policy/browser tests | done |
| 1 | Metadata-only editorial blog with draft/publish, categories, tags, and public Views | Action/View contracts and hidden-draft browser test | done |
| 2 | Opt-in password signup, login/logout, fixed member role, and safe public auth UI | auth, HTTP, React, escalation, rate-limit, and CSRF tests | done |
| 3 | Route-bound comment submission and editor approval/rejection | binding-tamper, policy, audit, and browser moderation tests | done |
| 4 | Generic list/detail rendering, safe rich text, navigation, pagination, and RSS | React, XSS, responsive, and public-route browser tests | done |
| 5 | SQLite/PostgreSQL parity, regression qualification, docs, and v0.5 cut | all terminal gates | done |

## Working rules

- Build only generic primitives; keep all blog-specific behavior in `examples/blog` metadata.
- Add failing evidence before changing auth, policy, Action, View, or render boundaries.
- Self-registration grants only a compiler-fixed `member` role; editor/admin promotion remains protected System Admin behavior.
- Draft posts and pending/rejected comments must be impossible to retrieve publicly, not merely hidden in React.
- Route-bound values are server-validated and cannot be overridden by submitted form data.
- Passwords are sensitive inputs and never appear in AppIR output, logs, audit data, manifests, or idempotency results.
- Preserve definition → validation → migration → immutable AppIR → atomic activation.
- Preserve View reads and Action writes on public, Admin, SQLite, and PostgreSQL paths.
- Run the nearest focused test after each milestone and keep `docs/capabilities.md` and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-blog
make test-postgres
make build
```
