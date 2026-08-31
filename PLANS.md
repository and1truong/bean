# Bean v0.6 agent-readable compiler plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze the CLI envelope, exit-status, diagnostic-code, schema, capability, inspect, plan, diff, publish, and test contracts | contract document, fixtures, and failing black-box tests | done |
| 1 | Versioned JSON output and stable diagnostic taxonomy for existing validation/publication paths | CLI and compiler contract tests, including redaction and deterministic ordering | done |
| 2 | Canonical JSON Schemas plus capability discovery generated from or checked against compiler vocabulary | all maintained examples validate; schema drift tests pass | done |
| 3 | Source/application inspection and side-effect-free semantic plan/diff | compiler/release tests and database non-mutation evidence | done |
| 4 | Unified `app init`, `app publish`, and isolated lifecycle `app test` loop | JSON-only black-box harness and repair benchmark | done |
| 5 | Compatibility documentation and repository qualification | `make check` and `make build` | done |

## Working rules

- Treat codes, paths, envelope fields, ordering, exit statuses, and redaction as public API.
- Derive machine descriptions from compiler vocabulary where possible; do not maintain an unrelated second schema model.
- Keep source-only commands database-free and make planning/diffing explicitly side-effect-free.
- Preserve source-aware human diagnostics while adding structured output; do not regress existing CLI users.
- Reuse the compiler, release, migration, AppIR, View, and Action services; CLI and future MCP must not fork behavior.
- Do not add application primitives, agent-provider logic, MCP, hosting, or generated semantic tests in this goal.
- Add failing black-box evidence before changing a public command contract and run the nearest test after every milestone.
- Keep `GOAL.md`, `ROADMAP.md`, and `docs/progress.md` current as milestones move.

## Terminal gates

```bash
make check
make build
```

# Completed Asana Lite local application plan

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Goal, contracts, and metadata design | focused compiler/field tests and source validation | done |
| 1 | Transactional generic file field and multipart Webform path | field, Action, HTTP, and React tests | done |
| 2 | Generic board and arbitrary-depth tree presentations | compiler and React tests | done |
| 3 | Metadata-only anonymous Asana Lite application | source validation and browser journey | done |
| 4 | Documentation and repository qualification | `make check` and `make build` | done |

## Working rules

- Keep project/task/attachment behavior under `examples/asana`; core additions must compile and render any compatible metadata.
- Preserve View reads, Action writes, immutable route bindings, and atomic Action/audit/blob persistence.
- Never use a client filename as a path or expose file bytes in AppIR, manifests, logs, audit, or Action results.
- Board and tree field references are compiler-validated; state changes use declared Actions.
- Run the nearest focused test after every milestone and keep this plan plus `docs/progress.md` current.

# Reviewable application sources plan (completed)

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Manifest/resource grammar and source-aware loader | definition loader tests | done |
| 1 | CLI, embedded demo, and fixture integration | CLI and affected package tests | done |
| 2 | Feature-oriented example migration and authoring docs | all example validation and documentation review | done |
| 3 | Repository qualification | `make check` and `make build` | done |

## Working rules

- Optimize the source format for authors and reviewers; do not preserve the unused bundle syntax.
- Keep resource inclusion explicit, local, and non-overriding.
- Preserve source locations through compiler diagnostics.
- Flatten sources into the canonical Bundle before the existing release lifecycle.

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
