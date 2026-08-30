# Bean shadcn/ui system plan (completed)

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Tailwind v4, source-owned shadcn primitives, Bean tokens, and lint guard | frontend lint, typecheck, and build | done |
| 1 | Shell/Auth and metadata-driven public UI migration | React public rendering tests | done |
| 2 | Application Admin and System Admin migration with accessible confirmations | React Admin and CMS/blog browser tests | done |
| 3 | Studio migration, responsive browser coverage, docs, and qualification | Studio tests and terminal gates | done |

## Working rules

- Keep shadcn components checked in under `web/src/components/ui`; do not require a frontend runtime service.
- Preserve routes, accessible names, stable test IDs, metadata behavior, and View/Action data boundaries.
- Keep native selects for dynamic and multi-select forms; do not introduce a second client validation schema.
- Keep application-specific presentation in metadata rather than branching in core React code.
- Run the nearest frontend test after each milestone and keep `GOAL.md` and `docs/progress.md` current.

## Completed scoped-resource-list plan

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Typed `resource-list` definition and security contract | compiler and HTTP tests | done |
| 1 | Generic table/filter/action renderer and blog metadata route | React and Playwright tests | done |
| 2 | Backend parity, docs, and qualification | terminal gates | done |

## Completed v0.5 plan

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
