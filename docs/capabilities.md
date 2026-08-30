# Bean v0.4 capability matrix

`complete` means the v0.4 compiler/runtime pair has direct executable evidence. Bean remains alpha software and the limits below are part of the contract.

| Area | Capability | Status | Direct evidence |
| --- | --- | --- | --- |
| Definitions/AppIR | Strict typed definitions, canonical compilation, immutable versioned activation | complete | compiler, definition, AppIR compatibility tests |
| DBAL | Parameterized CRUD, predicates, joins, groups, aggregates, transactions, inspection, migrations | complete | reusable SQLite/PostgreSQL contract |
| PostgreSQL | pgx backend selection, numbered parameters, SQLSTATE errors, Admin/Action/View HTTP parity | complete | `make test-postgres` against PostgreSQL 17 |
| Entity/relations | Typed native tables, four relation cardinalities, owner/tenant/soft-delete/version | complete | migration, Action, View, policy contracts |
| Actions | Typed I/O, full declared step set, rollback, concurrency, audit, job/outbox intent | complete | Action integration and race tests |
| Idempotency | Atomic result persistence and canonical input fingerprint conflict | complete | replay and changed-input contracts |
| Views/policies | Projection, filters, joins, aggregates, keyset paging, tenant/owner/role/redaction | complete | View/policy contracts and browser apps |
| Releases | Additive plan, schema-ahead reconciliation, pointer integrity, startup storage validation | complete | release tests and process-crash gate |
| SQLite durability | WAL, foreign keys, synchronous FULL, crash/restart integrity/foreign-key checks | complete | `make test-crash` |
| Jobs | Claim token/lease, attempts, retry schedule, terminal failure, stale recovery | complete | job state-machine test |
| Outbox | Claim token/lease, attempts, retry schedule, terminal failure, stale recovery | complete | outbox state-machine test |
| Application Admin | Search/filter/sort/page, typed forms, relations, domain/bulk Actions, history | complete | HTTP, React, CMS and Studio browser tests |
| System Admin | Safe users/roles, release/migration/queue visibility, retry/cancel, CSRF and audit | complete | HTTP secret-exclusion/mutation tests and React tests |
| Studio | Typed Entity/View/Action/Policy/AdminResource editors and draft references | complete | React core-editor tests |
| Studio | Lossless advanced JSON, diagnostics, schema/migration preview, release preview | complete | round-trip unit and release handler tests |
| Visual acceptance | Core application authored and published without specification JSON | complete | `studio-builder.spec.ts` |
| Qualification | contract, fuzz-smoke, compatibility, race, black-box, crash, PostgreSQL and Playwright | complete | terminal make targets |

## Explicit limits

- One Bean process and one active application per database are qualified. Clustering, multi-process writers, replicas, HA, and failover are not.
- Migrations are additive. Destructive schema changes, data transformations, and automated rollback are rejected or deferred.
- Outbox delivery is at-least-once. A crash after delivery but before acknowledgement can duplicate an effect.
- v0.4 crash qualification assumes a functioning filesystem or PostgreSQL service. Corruption, host loss, backup/restore, and point-in-time recovery are outside scope.
- The visual builder covers the core operational definition path; non-core page composition kinds use advanced JSON in this slice.
- SQLite/PostgreSQL parity is contract and workflow parity, not identical query plans or operational characteristics.
- External security review, load envelopes, SLOs, supply-chain signing, and production release certification remain future work.
