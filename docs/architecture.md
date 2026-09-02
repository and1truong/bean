# Architecture

Bean is a modular Go monolith with an embedded React application and a selected SQLite or PostgreSQL database. Every Bean-owned React surface composes checked-in shadcn/ui primitives with Tailwind and shared Bean tokens; the optimized result remains an embedded asset bundle rather than a runtime frontend service.

```text
YAML / typed visual Studio -> canonical definitions -> validation
                           -> additive migration preview
                           -> immutable AppIR -> active release

Lifecycle -> canonical state graph -> Policy-aware Action transition
Rule -> canonical typed AST -> bounded I/O-free evaluation
                         -> Action guard / derived input / Entity invariant
Extension -> typed Action binding -> transactional outbox intent
                                -> bounded after-commit HTTP provider

HTTP -> Context/Policy -> View reads / Action writes -> typed DBAL
                                                   -> SQLite adapter
                                                   -> PostgreSQL adapter
Agent -> CLI/MCP -> shared Agent Protocol dispatcher -> compiler/release/View/Action
UI   <- typed manifest/render tree
Admin <- AdminResource data plane + protected System control plane
```

Definitions, revisions, release AppIR, and the active pointer are persisted. Normal requests use one validated immutable AppIR snapshot from the kernel. Publication compiles the complete draft, applies a deterministic additive migration transaction, then commits the release record, migration journal, and active pointer together. A crash between those commits leaves the previous release active with storage potentially ahead; retry inspects physical columns, skips already-applied additive work, and activates a complete new release. Startup resolves the pointer and validates every active Entity column and relation table before kernel activation.

SQLite uses foreign keys, WAL, `synchronous=FULL`, and a bounded busy timeout. PostgreSQL uses pgx through `database/sql`, numbered parameters, information-schema inspection, transactional migrations, and SQLSTATE-based portable errors. Backend-specific SQL is confined to DBAL and migration adapters. The supported v0.4 deployment is one Bean process; clustering and simultaneous application writers are not qualified.

Actions own validation, authorization, domain writes, optimistic checks, audit, outbox intents, jobs, and idempotency in one database transaction. A Lifecycle owns the initial state and canonical transition graph for one Entity enum field. Create Actions inject the initial state, generic updates cannot write the field, and bound transition Actions may expose the whole graph or a compiler-checked subset. Policies are evaluated before any Rule guard. Derived Action inputs are server-owned, evaluate simultaneously from the original input, and use one injected context across transaction retries; Entity invariants evaluate the final typed candidate immediately before persistence. Rules can deny or derive but cannot authorize, query, mutate, perform I/O, read ambient time, or bypass the Action boundary. An idempotency record stores the canonical client-input hash and result. Jobs and outbox records are claimed with persisted tokens and timestamps, executed outside the claim transaction, and finalized only by the current token owner. Expired claims return to pending. A crash after an external delivery but before finalization can duplicate it, so delivery is explicitly at-least-once.

An Extension is immutable typed metadata for one HTTP endpoint, not an in-process plugin. Its Action step validates and stores input, one stable invocation ID, and the originating canonical Extension contract in the same transaction as domain mutation and audit. The existing outbox delivers that pinned contract after commit with bounded timeout, response size, retry count, and fixed delay, even if a newer release becomes active first. Host bearer configuration is resolved at delivery time and never enters AppIR. TestSuites replace only the provider implementation: they still execute the production Action and outbox paths, then compare ordered typed calls offline.

The application Admin is metadata-driven rather than a raw-table editor: AdminResources select a compiled View, CRUD Actions, list/form fields, and domain Actions. Its shadcn Table and form components preserve server-driven query and mutation semantics rather than introducing a client-side data model. The separate System section exposes only curated operational columns. User-role changes and eligible queue retry/cancel mutations require administrator authentication, CSRF, confirmation in the UI, affected-row checks, and audit records; password hashes and session secrets are never selected.

Studio visual editors mutate the normal definition `spec` for Entity, View, Action, Policy, and AdminResource. Reference choices come from current draft definitions. The advanced JSON view reads and writes the same object, so there is no second visual metadata format and supported fields survive round trips. Validate returns compiler diagnostics, target schema, and migration preview before publish.

Board and tree rendering are typed View presentations: the compiler verifies selected grouping, parent, order, title, and transition references before activation. The browser resolves a board Action's canonical Lifecycle graph from normalized manifest metadata, reads rows through the declared View, and writes movement through the declared Action.

The Agent Protocol is an adapter boundary, not a second runtime. Its registry assigns ten versioned operations to Definition, Release, and Application Planes and checks process grants before invoking a handler. CLI translates existing flags into operation inputs; MCP translates JSON-RPC tool calls into the same inputs. Definition handlers call the compiler, Release handlers call the release lifecycle, and Application handlers call the active View or Action service with host-supplied request context. Neither transport owns application semantics or database access.

Internal capability registries are sealed composition points inside this monolith, not runtime plugin APIs. Definition kinds own their compiler and inspection projection; Action steps own compiler requirements, declared effects, and runtime handlers; Block types own validation metadata, render properties, and component names. Rule operators are intentionally a closed algebra rather than a plugin registry: their type checker and evaluator remain exhaustive together, and capabilities expose the exact vocabulary. Each registry is constructed explicitly, rejects duplicate names, exposes deterministic copied metadata, and has parity tests across its consumers. There is no global registration function or `init()` discovery, so a build contains one reviewable capability set and fails closed when compiler/runtime support drifts.

Action operations deliberately remain explicit orchestration in the Action service. Their branches share ordered authorization, idempotency, transaction, audit, outbox, and Lifecycle checks; splitting those branches into handlers would obscure rather than isolate the security boundary. View presentation modes also remain a closed cross-language algebra: Go validates their field invariants while the embedded React client renders them. A Go-only registry would falsely imply one owner without removing the TypeScript dispatch, and code generation or a public presentation SDK is outside this refactor. Both decisions should be revisited only when a new operation or presentation supplies concrete evidence that the current exhaustive control flow no longer keeps the invariant visible.

Small `file` values enter only through bounded multipart requests. Action execution inserts immutable blob metadata/content and the referencing Entity value in one database transaction, while replacement and hard deletion remove the old blob in that transaction. Downloads first find a live compiled Entity reference and apply its read policy; client filenames are response metadata only and never storage paths.

## Qualified failure boundary

v0.4 tests unexpected Bean process termination and restart while the local filesystem or PostgreSQL service remains functional. It does not claim recovery from corrupted media, dishonest fsync, host loss, database failover, network partitions, backup loss, or concurrent Bean writers. SQLite integrity/foreign-key checks and active-release/schema checks provide deterministic failure evidence, not a substitute for backup and disaster recovery.
