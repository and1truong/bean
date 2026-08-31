# Architecture

Bean is a modular Go monolith with an embedded React application and a selected SQLite or PostgreSQL database. Every Bean-owned React surface composes checked-in shadcn/ui primitives with Tailwind and shared Bean tokens; the optimized result remains an embedded asset bundle rather than a runtime frontend service.

```text
YAML / typed visual Studio -> canonical definitions -> validation
                           -> additive migration preview
                           -> immutable AppIR -> active release

HTTP -> Context/Policy -> View reads / Action writes -> typed DBAL
                                                   -> SQLite adapter
                                                   -> PostgreSQL adapter
UI   <- typed manifest/render tree
Admin <- AdminResource data plane + protected System control plane
```

Definitions, revisions, release AppIR, and the active pointer are persisted. Normal requests use one validated immutable AppIR snapshot from the kernel. Publication compiles the complete draft, applies a deterministic additive migration transaction, then commits the release record, migration journal, and active pointer together. A crash between those commits leaves the previous release active with storage potentially ahead; retry inspects physical columns, skips already-applied additive work, and activates a complete new release. Startup resolves the pointer and validates every active Entity column and relation table before kernel activation.

SQLite uses foreign keys, WAL, `synchronous=FULL`, and a bounded busy timeout. PostgreSQL uses pgx through `database/sql`, numbered parameters, information-schema inspection, transactional migrations, and SQLSTATE-based portable errors. Backend-specific SQL is confined to DBAL and migration adapters. The supported v0.4 deployment is one Bean process; clustering and simultaneous application writers are not qualified.

Actions own validation, authorization, domain writes, optimistic checks, audit, outbox intents, jobs, and idempotency in one database transaction. An idempotency record stores the canonical input hash and result. Jobs and outbox records are claimed with persisted tokens and timestamps, executed outside the claim transaction, and finalized only by the current token owner. Expired claims return to pending. A crash after an external delivery but before finalization can duplicate it, so delivery is explicitly at-least-once.

The application Admin is metadata-driven rather than a raw-table editor: AdminResources select a compiled View, CRUD Actions, list/form fields, and domain Actions. Its shadcn Table and form components preserve server-driven query and mutation semantics rather than introducing a client-side data model. The separate System section exposes only curated operational columns. User-role changes and eligible queue retry/cancel mutations require administrator authentication, CSRF, confirmation in the UI, affected-row checks, and audit records; password hashes and session secrets are never selected.

Studio visual editors mutate the normal definition `spec` for Entity, View, Action, Policy, and AdminResource. Reference choices come from current draft definitions. The advanced JSON view reads and writes the same object, so there is no second visual metadata format and supported fields survive round trips. Validate returns compiler diagnostics, target schema, and migration preview before publish.

Board and tree rendering are typed View presentations: the compiler verifies selected grouping, parent, order, title, and transition references before activation. The browser receives only normalized presentation metadata, reads rows through the declared View, and writes board movement through the declared Action.

Small `file` values enter only through bounded multipart requests. Action execution inserts immutable blob metadata/content and the referencing Entity value in one database transaction, while replacement and hard deletion remove the old blob in that transaction. Downloads first find a live compiled Entity reference and apply its read policy; client filenames are response metadata only and never storage paths.

## Qualified failure boundary

v0.4 tests unexpected Bean process termination and restart while the local filesystem or PostgreSQL service remains functional. It does not claim recovery from corrupted media, dishonest fsync, host loss, database failover, network partitions, backup loss, or concurrent Bean writers. SQLite integrity/foreign-key checks and active-release/schema checks provide deterministic failure evidence, not a substitute for backup and disaster recovery.
