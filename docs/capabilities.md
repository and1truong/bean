# Bean v0.16 capability matrix

`complete` means the v0.16 compiler/runtime pair has direct executable evidence. Bean remains alpha software and the limits below are part of the contract.

| Area | Capability | Status | Direct evidence |
| --- | --- | --- | --- |
| Definitions/AppIR | Strict typed definitions, canonical compilation, immutable versioned activation | complete | compiler, definition, AppIR compatibility tests |
| Agent CLI | Versioned JSON envelope, stable exits/codes/paths/candidates, schema/capability discovery | complete | CLI unit contracts and real-binary repair benchmark |
| Agent release loop | Init, validate, inspect, side-effect-free plan/diff, exact-source publish, isolated lifecycle test | complete | CLI/release/non-mutation tests and `make test-blackbox` |
| Agent Protocol | Ten provider-neutral operations across independently authorized Definition, Release, and Application Planes | complete | registry, handler, transport-parity, and plane-boundary tests |
| MCP adapter | Current `2026-07-28` stdio discovery/tools plus maintained initialization-era compatibility | complete | framing, discovery, list/call, malformed request, EOF, and stdout-cleanliness tests |
| Application Plane | Policy-preserving View-only reads and Action-only writes with host-owned identity | complete | SQLite/PostgreSQL owner/tenant and bypass-refusal contracts |
| Canonical schemas | Draft 2020-12 manifest and per-kind schemas generated from compiler specification types | complete | maintained-example coverage and checked-in schema drift tests |
| Lifecycle semantics | Canonical initial state and reachable transition graph, optional Action subsets, protected generic updates, AppIR v2, semantic inspect/diff | complete | compiler, AppIR, Action, release, CLI/MCP parity, ATS, and commerce contracts |
| Deterministic Rules | Named typed canonical AST, closed sources/operators, compile/runtime bounds, AppIR v3, inspect/references/redacted semantic diff | complete | Rule, compiler, schema, AppIR, Agent Protocol, and release contracts |
| Rule consumers | Policy-ordered Action guards, simultaneous server-owned derives, final-candidate Entity invariants | complete | Action rollback/replay/context tests plus commerce, ATS, and booking slices |
| Semantic TestSuites | Typed Rule/Action targets, fixtures, explicit actor/tenant/time/ID/seed context, deterministic assertions, typed offline provider mocks, fresh case isolation | complete | compiler/schema/AppIR, isolated runner, CLI/Agent Protocol, release, Extension, and maintained defect contracts |
| Generated tests | Explicit-oracle replays, proven Policy/Lifecycle negatives, DemoSeed CRUD, structural/route evidence, eligible HTTP journeys | complete | generator, seeded-defect, production Action/View/HTTP, ordering, and CLI/Agent Protocol contracts |
| Typed Extensions | AppIR v5 HTTP boundary with typed I/O, transactional intents, stable idempotency identity, host bearer auth, bounded delivery, and offline interaction assertions | complete | compiler/schema/AppIR/release, Action atomicity/replay, outbox/HTTP failure, TestSuite, and commerce contracts |
| First-class View displays | AppIR v6 named page/block/JSON/CSV/RSS displays over one canonical View query, with legacy Block presentation normalization | complete | compiler/schema/AppIR/inspection/release, HTTP, React, blog, and ATS contracts |
| Bean Explore | Administrator Entity-to-View preview/save using the canonical compiler, Policy path, View service, and Studio draft | complete | compiler/HTTP/React/ATS contracts |
| Typed Explore queries | AppIR v7 View search, projection, filters, deterministic sort/cursor, scalar/date grouping, aggregates, and records/groups/metric result shapes | complete | compiler/View/DBAL SQLite/PostgreSQL contracts |
| Explore Displays | Compatible cards, bar chart, metric, grouped table, calendar, and existing record renderers with same-View switching | complete | compiler/React/ATS/Booking contracts |
| Operational dashboards | Explicit typed Page-filter fan-out, typed drill to exact record Views, and URL-reproducible state | complete | compiler/Page/HTTP/React/ATS/CRM/Asana contracts |
| Explore Actions | Table/board selection and bounded sequential record batches through ordinary Policy/Rule/Lifecycle/Action semantics | complete | compiler/Action/HTTP/React/ATS/CRM/Commerce contracts |
| Explore authoring parity | Explore, Studio, schemas, inspect/references/diff, and five agent prompt rubrics produce ordinary definitions | complete | React/compiler/Agent Protocol/agent fixture contracts |
| Semantic sequences | AppIR v8 ordered route/frame composition over existing Panels and Blocks; AppIR v15 adds `next` chapters and `down` detail frames with Policy-connected render trees | complete | compiler/AppIR/Agent Protocol/release/HTTP contracts |
| Semantic content | Closed heading, paragraph, bullets, quote, code, callout, image, and ordered-diagram content without executable markup | complete | schema/compiler/content/React safety contracts |
| Inline Panel content | AppIR v9 nested ordered region items interleave frame-local semantic content and named Blocks; legacy `blocks` remains compatible | complete | AppIR/compiler/schema/Panel/Sequence/policy contracts |
| Ordered Page sections | AppIR v10 composes 1–32 ordered Panel layout bands while preserving legacy `panel`, Page filters, and Page/Panel/Block Policy boundaries | complete | compiler/schema/Page/HTTP/release/React/tracker browser contracts |
| Policy-aware Region collapse | AppIR v11 opt-in `collapseWhenEmpty` omits zero-child authorized Regions and expands a sole survivor without client Policy logic | complete | AppIR/compiler/Panel/Page/Sequence/React/CSS/browser contracts |
| Semantic Page section widths | AppIR v12 `contained`, `wide`, and `full` Page placement widths preserve safe gutters, defaults, order, and reusable Panel independence | complete | compiler/schema/Page/restart/Studio/React/browser geometry contracts |
| Hierarchical Menu navigation | AppIR v13 plus Action-managed dynamic placements provide typed global/scoped three-level route navigation; AppIR v14 adds closed `default`/`line` shadcn-style visuals while preserving `nav`/link semantics | complete | compiler/publication/Action/View/HTTP/React/Studio/generated-editor/Books/restart/browser contracts |
| Responsive Panels | Fixed viewport-owned single/two/sidebar/grid presets preserve source order and bounded tracks without author CSS | complete | React semantic-hook and Playwright computed-layout contracts |
| HTML presentation profile | Wide/standard frames, horizontal/vertical navigation, URL state, keyboard/buttons/picker, progress, notes, responsive bounds, and print page breaks | complete | React and presentation Playwright journey |
| Presentation agent slice | Ten-frame, five-chapter Bean introduction with vertical depth, stable repair diagnostics, and a real grouped View/chart | complete | presentation source, agent rubric, restart, package, and browser evidence |
| Public tables and controls | Ordered labelled linked columns, typed exposed-filter operators/widgets/defaults, URL state, immutable binding separation, and cursor page sizes | complete | compiler/View/DBAL/HTTP/React/Studio and SQLite/PostgreSQL contracts |
| View page titles | Static or unique-bound result titles rendered as page headings and browser titles | complete | compiler/HTTP/React and blog contracts |
| DBAL | Parameterized CRUD, predicates, joins, groups, aggregates, transactions, inspection, migrations | complete | reusable SQLite/PostgreSQL contract |
| PostgreSQL | pgx backend selection, numbered parameters, SQLSTATE errors, Admin/Action/View HTTP parity | complete | `make test-postgres` against PostgreSQL 17 |
| Entity/relations | Typed native tables, four relation cardinalities, owner/tenant/soft-delete/version | complete | migration, Action, View, policy contracts |
| Local identity | Opt-in signup Action, bcrypt passwords, fixed role, safe output, independent throttle | complete | compiler, auth, HTTP, SQLite/PostgreSQL blog tests |
| Bound blocks | Compiler-checked immutable Page/Block values for Views, Webforms, and scoped AdminResource lists | complete | binding/filter diagnostics, tamper tests, React tests, blog browser journey |
| Content rendering | Generic list/detail links, named content Filters, safe Markdown, metadata fields, legacy rich text, empty states, cursor controls | complete | Filter/View/React XSS tests and blog browser journey |
| Operational presentation | Compiler-validated enum boards and expandable self-relation trees | complete | compiler/React contracts and Asana Lite browser journey |
| Demo presentation | Typed themes plus compiler-validated Metric, Timeline, and declared public View Search | complete | schema/compiler/HTTP/React contracts and ATS browser journey |
| Demo fixtures | Typed relation-aware deterministic generation through Actions, including server-derived fields, with View-based replay verification | complete | scalar/relation/derive/cycle/replay/refusal tests and ATS package evidence |
| Application patterns | Ten inspectable, byte-stable ordinary-definition bundles | complete | independent catalog compilation and CLI inspection tests |
| Local package | Staged SQLite package with executable, populated database, versioned manifest, checksums, verification, and source-independent startup | complete | CLI tamper/failure-atomicity tests and packaged-binary browser journey |
| Small file attachments | 5 MiB multipart `file` fields, transactional metadata/content, policy-checked download, replacement/deletion cleanup | complete | field/Action/HTTP/React contracts and Asana Lite browser journey |
| UI system | Source-owned shadcn primitives, shared Bean tokens, accessible confirmations, responsive Shell/Public/Admin/System/Studio surfaces | complete | frontend lint/unit/build, `ui.spec.ts`, and full Playwright gate |
| Actions | Typed I/O, full declared step set, Rule guards/derives/invariants, rollback, concurrency, audit, job/outbox intent | complete | Action integration and race tests |
| Idempotency | Atomic result persistence and canonical input fingerprint conflict | complete | replay and changed-input contracts |
| Views/policies | Projection, filters, joins, aggregates, keyset paging, named displays, tenant/owner/role/redaction | complete | View/policy/display contracts and browser apps |
| Releases | Additive plan, schema-ahead reconciliation, pointer integrity, startup storage validation | complete | release tests and process-crash gate |
| SQLite durability | WAL, foreign keys, synchronous FULL, crash/restart integrity/foreign-key checks | complete | `make test-crash` |
| Jobs | Claim token/lease, attempts, retry schedule, terminal failure, stale recovery | complete | job state-machine test |
| Outbox | Claim token/lease, attempts, retry schedule, terminal failure, stale recovery | complete | outbox state-machine test |
| Application Admin | Search/filter/sort/page, typed forms, relations, domain/bulk Actions, history | complete | HTTP, React, CMS and Studio browser tests |
| System Admin | Safe users/roles, release/migration/queue visibility, retry/cancel, CSRF and audit | complete | HTTP secret-exclusion/mutation tests and React tests |
| Studio | Typed Entity/View/Page/Action/Policy/AdminResource editors including common Explore query/display/filter/drill/action semantics | complete | React core-editor tests |
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
- v0.16 retains the v0.5 crash qualification assumption of a functioning filesystem or PostgreSQL service. Corruption, host loss, backup/restore, and point-in-time recovery are outside scope.
- The visual builder covers the core operational definition path, including Page filters; uncommon Block/Panel combinations use advanced JSON.
- Tree presentation is bounded by the View maximum of 200 rows. File fields are bounded small attachments stored as base64 metadata content; external object storage, resumable transfer, scanning, and media processing are outside this slice.
- SQLite/PostgreSQL parity is contract and workflow parity, not identical query plans or operational characteristics.
- `bean package` deliberately targets local SQLite only. It does not create containers, installers, hosted previews, signatures, or a distribution channel.
- Lifecycle owns only one Entity enum field, initial state, and transition graph. Rules own only bounded side-effect-free local predicates and scalar calculations. TestSuites target Rules and Actions in isolated SQLite only; generated cases exercise Policy/Lifecycle through Actions and never infer business expectations. Extensions provide only typed after-commit HTTP external writes; synchronous results, scripts, WASM, plugins, direct database/filesystem/process access, OAuth, arbitrary headers, compensations, and exactly-once effects remain unsupported. Direct Policy/Lifecycle/Extension TestSuite targets and production release gating remain future work. Legacy Action-local transition graphs remain supported; AppIR v1-v4 remain readable without Extension semantics.
- Explore does not include arbitrary SQL, pivots, to-many/distinct aggregates, average money, currency conversion, computed read buckets, user-private presets, external data federation, or provider schema introspection. View displays do not include numeric/full pagers, richer chart families, or arbitrary templates/application code. Group results fail rather than silently truncate beyond 200. Bulk Actions are bounded to 200 unique records, sequential, and non-atomic across records.
- Sequence v0.16 supports only the `presentation` profile and deterministic semantic density bounds. Freeform coordinates, arbitrary templates, animation, transitions, WYSIWYG editing, embedded agents, research/image generation, executable Markdown/HTML/CSS/JS/SVG, native PDF/PPTX, hosted sharing, asset caching, and pixel-perfect font/layout prediction are outside scope.
- MCP deliberately targets local stdio only. Streamable HTTP, hosted identity, OAuth, subscriptions, prompts, resources, sampling, and remote rate-limit infrastructure are outside v0.15.
- External security review, load envelopes, SLOs, supply-chain signing, and production release certification remain future work.
