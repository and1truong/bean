# Bean v0.10 Deterministic Rule Expressions plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze named Rule AST, sources/operators, consumers, bounds, examples, and compatibility | root goal plus compiler/runtime fixture plan | done |
| 1 | Typed bounded deterministic Rule core | type/eval/replay/operator/source/resource-limit tests | active |
| 2 | Rule Definition, AppIR, schema, capabilities, inspect, references, diff, and compatibility | compiler/schema/Agent Protocol/AppIR tests | pending |
| 3 | Action guards, simultaneous derived inputs, and Entity invariants | Policy/order/rollback/idempotency/context tests | pending |
| 4 | Three metadata-only reference slices and backend/restart parity | source journeys plus SQLite/PostgreSQL/crash tests | pending |
| 5 | Documentation, v0.10 version cut, terminal gates, CI, clean review, and merge | all gates, CI, Codex review, merged PR | pending |

## Working rules

- Prefer an existing semantic primitive, then a Rule, then the later typed extension boundary.
- Rules are named canonical ASTs, not text scripts; resource bounds and type checking are compiler/runtime contracts.
- Policy authorizes; Rules can only further constrain or derive deterministic local values.
- Derived inputs are server-owned, simultaneous, and unavailable to sibling derives.
- Keep Rules free of I/O, implicit time/randomness, mutation, dynamic lookup, and environment state.
- Add failing contract evidence before each public behavior and run the nearest tests after each milestone.
- Keep `GOAL.md`, `PLANS.md`, `ROADMAP.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```

# Structural contracts and unified execution seams plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze compatibility, security fixes, and deep-module interfaces | root goal, ordered tracer bullets, and baseline focused tests | done |
| 1 | Structural diagnostic facts and rule-owned codes | wording-independence, recovery, candidate, and diagnostic contract tests | done |
| 2 | One View-owned read engine for database and transaction adapters | policy/rich-text/relation/order/limit equivalence tests | done |
| 3 | Enforced Action-step effects and one entity resolver | registry-wide read/write obligation and mismatch tests | done |
| 4 | Context-specific value-source catalog | resolver/compiler/redaction parity and fail-closed tests | done |
| 5 | Typed client render dispatch and expression parity | pure render/operator tests including explicit unknown failures | done |
| 6 | Shared client write encoder/caller and field errors | JSON/multipart/batch/Admin/Webform tests | done |
| 7 | Complete Definition-kind ownership and explicit phases | independent AppIR storage completeness and per-kind validation tests | done |
| 8 | Sealed Agent Protocol operation entries and owned capabilities | construction, authorization, discovery, and capability parity tests | done |
| 9 | Deletion cleanup, documentation, and qualification | focused tests plus all terminal gates | done |

## Working rules

- Land security and machine-contract tracer bullets before broader ownership cleanup.
- Preserve public behavior with before/after contract evidence; make demonstrated silent-failure and policy fixes explicit.
- Keep View reads and Action writes, immutable AppIR activation, and backend confinement intact.
- Prefer one deep module over shared pass-through helpers; retain explicit closed-algebra and compiler phase control flow.
- Run the nearest focused test after every milestone and keep `GOAL.md`, `PLANS.md`, `ROADMAP.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```

# Sealed internal capability registries plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Inventory repeated discriminators and freeze internal/external boundaries | root goal plus hotspot and retained-switch rationale | done |
| 1 | Immutable deterministic registry primitive | duplicate, lookup, ordering, and sealing tests | done |
| 2 | Definition-kind registry across compile/schema/inspect/reference paths | compiler/schema/Agent Protocol parity tests | done |
| 3 | Action-step registry with declared effects and runtime/compiler parity | Action/compiler/DemoSeed safety tests | done |
| 4 | Block-type registry and evidence-based operation/presentation decision | compiler/render/component parity tests and recorded rationale | done |
| 5 | Documentation, terminal gates, CI, and clean reviewed PR | all gates, CI, and Codex review | done |

## Working rules

- Preserve behavior and public contracts; this goal adds no application capability.
- Prefer registries only for repeated extension seams; retain explicit switches for closed algebras and security-sensitive orchestration.
- Keep registries immutable, explicitly constructed, deterministic, and free of `init()` registration.
- Add parity/failure evidence before replacing each dispatcher and run the nearest tests after every milestone.
- Keep `GOAL.md`, `PLANS.md`, `ROADMAP.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```

# Bean v0.9 Semantic Application Model plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Catalogue existing transition use, freeze the minimal Lifecycle and Action-binding contract, and define compatibility behavior | ATS/commerce fixtures plus schema and diagnostic test plan | done |
| 1 | Canonical Lifecycle schema, compiler validation, immutable AppIR, capabilities, inspect, and semantic diff | focused schema/compiler/AppIR/Agent Protocol tests | done |
| 2 | Policy-aware Lifecycle enforcement through Actions with safe publication and restart behavior | positive/negative Action, release, and crash contracts | done |
| 3 | Convert ATS candidate and commerce order flows to the shared primitive | source validation plus independent application journeys | done |
| 4 | Preserve legacy transition compatibility and SQLite/PostgreSQL parity | compatibility, CLI/MCP parity, backend, and bypass-refusal tests | done |
| 5 | Documentation, version cut, terminal gates, CI, and clean reviewed PR | all gates, CI, and Codex review | done |

## Working rules

- Add only Lifecycle in v0.9; later semantic candidates need their own evidence.
- Freeze the source and Action-binding contract before runtime implementation.
- Keep transition authorization in Policies and transition mutation in Actions.
- Normalize semantics once in the compiler; CLI and MCP consume shared Agent Protocol results.
- Maintain the declared legacy Action transition representation until compatibility tests prove the migration path.
- Add failing contract evidence before each public behavior and run the nearest test after every milestone.
- Keep `GOAL.md`, `ROADMAP.md`, `PLANS.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```

# Bean v0.8 Agent Protocol plan

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze protocol operations, planes, transport, authorization, and compatibility contracts | root goal plus registry/authorization test fixtures | done |
| 1 | Shared provider-neutral dispatcher for Definition, Release, and Application Planes | focused handler tests across all ten operations | done |
| 2 | Existing command delegation plus generic CLI protocol transport | CLI compatibility and plane allow/deny contract suite | done |
| 3 | MCP 2026-07-28 stdio adapter with maintained legacy initialization compatibility | framing/discovery/list/call/error/EOF tests | done |
| 4 | Cross-transport parity, runtime Policy boundaries, and backend qualification | CLI/MCP parity plus SQLite/PostgreSQL View/Action tests | done |
| 5 | Provider-neutral agent guidance, documentation, terminal gates, and clean reviewed PR | shipped `agents/`, docs, all gates, CI, and Codex review | done |

## Working rules

- The dispatcher delegates to compiler, release, View, and Action services; transports contain framing and presentation only.
- Plane grants are host configuration and are checked before source or database access.
- MCP tool arguments never grant roles, tenants, planes, raw tables, arbitrary writes, SQL, or shell access.
- Preserve the v0.6 CLI envelope and commands while making their structured results originate from shared handlers.
- Support MCP stdio only in v0.8; do not add remote transport or identity infrastructure.
- Add failing contract evidence before each public operation and run the nearest test after each milestone.
- Keep `GOAL.md`, `ROADMAP.md`, `PLANS.md`, and `docs/progress.md` current.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```

# Bean v0.7 Demo Factory plan (completed)

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze Demo Factory source, pattern, seed, theme, package, and benchmark contracts | root goal and frozen ATS/CRM/tracker protocol | done |
| 1 | Typed Theme plus generic Metric, Timeline, and public Search presentation | schema/capability/compiler, HTTP, React, and ATS browser tests | done |
| 2 | Deterministic relation-aware fixture generator and `bean demo seed` | scalar/relation/cycle/replay/refusal tests and populated ATS evidence | done |
| 3 | Inspectable catalog of ordinary-definition application patterns | catalog stability tests and independent compilation of every pattern | done |
| 4 | Atomic, checksummed SQLite `bean package` output | restart, source-independence, tamper, failure-atomicity, and packaged-browser tests | done |
| 5 | ATS/CRM/tracker prompt-suite qualification, documentation, and version cut | terminal gates plus documented benchmark qualification boundary | done |

## Working rules

- Patterns expose ordinary definitions and always pass through schema and compiler validation; they never become hidden runtime macros.
- Seed writes use Actions, verification reads use Views, and generated data never bypasses Policy or storage contracts.
- Theme values come from closed compiler-known vocabularies; do not accept CSS or arbitrary frontend tokens.
- Dashboard composes Page/Panel/Block; add only the missing Metric, Timeline, and Search presentation behavior.
- Package only the current executable plus an activated SQLite database and manifest; do not add cloud, container, or installer machinery.
- Treat JSON envelopes, diagnostics, ordering, checksums, seed output, and package manifests as machine contracts.
- Add failing evidence before each public contract and run the nearest test after every milestone.
- Keep `GOAL.md`, `ROADMAP.md`, and `docs/progress.md` current as milestones move.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
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
