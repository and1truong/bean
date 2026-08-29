# Bean v0.2 capability matrix

This matrix describes the accepted v0.2 metadata contract. `complete` means the compiler/runtime pair has direct executable evidence. Metadata outside this contract is rejected during publication. Bean remains alpha software.

| Area | Capability | Status | Direct evidence |
| --- | --- | --- | --- |
| Definitions | Strict envelopes, machine names, unknown-field rejection and defaults | complete | compiler and definition tests |
| Definitions | Typed per-kind decoding and typed Action bindings | complete | compiler contracts; AppIR has no semantic `map[string]any` |
| Compiler | Required fields, references, routes, operators and executor coverage | complete | compiler negative contracts |
| AppIR | Deterministic canonical output, explicit format version and immutable activation | complete | compiler, AppIR compatibility and release tests |
| DBAL | Parameterized CRUD, predicates, joins, groups and aggregates | complete | reusable SQLite contract tests |
| DBAL | Deterministic keyset cursor predicates | complete | View cursor integration contract |
| Entity | Native typed tables, validation, owner/tenant/soft-delete/version columns | complete | field, Action and tenant integration tests |
| Relations | Four cardinalities, constrained storage, Action mutation and View traversal | complete | migration and relation integration contracts |
| Migrations | Deterministic additive plans and destructive-change rejection | complete | migration deterministic/safety tests |
| Actions | Complete operation/step set, typed I/O, named results and bindings | complete | all-step compiler/runtime contract |
| Actions | Transaction, optimistic checks, rollback, audit, outbox and idempotency | complete | concurrency, rollback/outbox and replay contracts |
| Views | Projection, joins, filters, sorting, grouping and five aggregates | complete | DBAL and View query-plan contracts |
| Views | Context/exposed filters, offset/keyset pagination and bounded limits | complete | View integration and malformed-cursor tests |
| Views | Policy injection, redaction and JSON/CSV/RSS equivalence | complete | tenant, render-redaction and CMS E2E tests |
| Policies | Identity, role, owner, tenant, condition and logical rules | complete | table-driven enforcement matrix |
| Policies | Page/Panel/Block/View/row/field/Action/Webform/relation boundaries | complete | policy, render, data and E2E contracts |
| Webforms | Validation, conditional rules, groups and multi-step mapping | complete | Webform/compiler contracts |
| Webforms | Accessible widgets, draft state, field errors and confirmation | complete | frontend unit/type gates and E2E |
| Render | Registered Blocks, Panels, Pages, Menus and typed context | complete | block/page/render tests and Studio E2E |
| Auth | Password sessions, CSRF, tenants and non-leaking denial | complete | auth, data and E2E tests |
| Releases | Failure safety, restart, checksums, compatibility and OpenAPI | complete | release/AppIR/OpenAPI tests |
| Testing | Contract, fuzz-smoke, compatibility, race, black-box and Playwright | complete | `make check` |
| Studio | Draft, validate, publish and advanced JSON editing | complete | Studio E2E |
| Studio | Per-kind visual semantic editors | not in v0.2 | deliberately not advertised |

## Deliberately rejected or deferred

- Unsupported fields, operators, Action steps, serializers, widgets, Blocks and context sources fail publication.
- Adding constrained fields to an existing SQLite table is rejected instead of attempting an unsafe table rewrite.
- Postgres, GraphQL, clustering, external integrations, marketplace features and visual polish are outside v0.2.
- Production readiness remains deferred pending crash-recovery qualification, external security review, operational SLO/load work and signed release engineering.
