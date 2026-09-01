# Bean v0.7 capability matrix

`complete` means the v0.7 compiler/runtime pair has direct executable evidence. Bean remains alpha software and the limits below are part of the contract.

| Area | Capability | Status | Direct evidence |
| --- | --- | --- | --- |
| Definitions/AppIR | Strict typed definitions, canonical compilation, immutable versioned activation | complete | compiler, definition, AppIR compatibility tests |
| Agent CLI | Versioned JSON envelope, stable exits/codes/paths/candidates, schema/capability discovery | complete | CLI unit contracts and real-binary repair benchmark |
| Agent release loop | Init, validate, inspect, side-effect-free plan/diff, exact-source publish, isolated lifecycle test | complete | CLI/release/non-mutation tests and `make test-blackbox` |
| Canonical schemas | Draft 2020-12 manifest and per-kind schemas generated from compiler specification types | complete | maintained-example coverage and checked-in schema drift tests |
| DBAL | Parameterized CRUD, predicates, joins, groups, aggregates, transactions, inspection, migrations | complete | reusable SQLite/PostgreSQL contract |
| PostgreSQL | pgx backend selection, numbered parameters, SQLSTATE errors, Admin/Action/View HTTP parity | complete | `make test-postgres` against PostgreSQL 17 |
| Entity/relations | Typed native tables, four relation cardinalities, owner/tenant/soft-delete/version | complete | migration, Action, View, policy contracts |
| Local identity | Opt-in signup Action, bcrypt passwords, fixed role, safe output, independent throttle | complete | compiler, auth, HTTP, SQLite/PostgreSQL blog tests |
| Bound blocks | Compiler-checked immutable Page/Block values for Views, Webforms, and scoped AdminResource lists | complete | binding/filter diagnostics, tamper tests, React tests, blog browser journey |
| Content rendering | Generic list/detail links, named content Filters, safe Markdown, metadata fields, legacy rich text, empty states, cursor controls | complete | Filter/View/React XSS tests and blog browser journey |
| Operational presentation | Compiler-validated enum boards and expandable self-relation trees | complete | compiler/React contracts and Asana Lite browser journey |
| Demo presentation | Typed themes plus compiler-validated Metric, Timeline, and declared public View Search | complete | schema/compiler/HTTP/React contracts and ATS browser journey |
| Demo fixtures | Typed relation-aware deterministic generation through Actions with View-based replay verification | complete | scalar/relation/cycle/replay/refusal tests and ATS package evidence |
| Application patterns | Ten inspectable, byte-stable ordinary-definition bundles | complete | independent catalog compilation and CLI inspection tests |
| Local package | Staged SQLite package with executable, populated database, versioned manifest, checksums, verification, and source-independent startup | complete | CLI tamper/failure-atomicity tests and packaged-binary browser journey |
| Small file attachments | 5 MiB multipart `file` fields, transactional metadata/content, policy-checked download, replacement/deletion cleanup | complete | field/Action/HTTP/React contracts and Asana Lite browser journey |
| UI system | Source-owned shadcn primitives, shared Bean tokens, accessible confirmations, responsive Shell/Public/Admin/System/Studio surfaces | complete | frontend lint/unit/build, `ui.spec.ts`, and full Playwright gate |
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
| Blog vertical slice | Draft/publish, category/tag relations, signup/login, comments, moderation, RSS | complete | `make test-blog` and PostgreSQL browser parity |
| Asana Lite vertical slice | Anonymous local projects, root-task board, arbitrary-depth tree, route-bound subtasks, and multiple attachments | complete | `asana.spec.ts` browser journey |
| Applicant tracker vertical slice | Jobs, candidates, notes, activities, pipeline transitions, detail, search, metric, timeline, theme, and generated data | complete | `ats.spec.ts` and `package.spec.ts` browser journeys |

## Explicit limits

- One Bean process and one active application per database are qualified. Clustering, multi-process writers, replicas, HA, and failover are not.
- Migrations are additive. Destructive schema changes, data transformations, and automated rollback are rejected or deferred.
- Outbox delivery is at-least-once. A crash after delivery but before acknowledgement can duplicate an effect.
- v0.7 retains the v0.5 crash qualification assumption of a functioning filesystem or PostgreSQL service. Corruption, host loss, backup/restore, and point-in-time recovery are outside scope.
- The visual builder covers the core operational definition path; non-core page composition kinds use advanced JSON in this slice.
- Tree presentation is bounded by the View maximum of 200 rows. File fields are bounded small attachments stored as base64 metadata content; external object storage, resumable transfer, scanning, and media processing are outside this slice.
- SQLite/PostgreSQL parity is contract and workflow parity, not identical query plans or operational characteristics.
- `bean package` deliberately targets local SQLite only. It does not create containers, installers, hosted previews, signatures, or a distribution channel.
- External security review, load envelopes, SLOs, supply-chain signing, and production release certification remain future work.
