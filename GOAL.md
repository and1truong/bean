# Goal: Structural Contracts and Unified Execution Seams

Status: complete

Deepen Bean's machine-facing and security-sensitive modules so that diagnostics, reads, Action-step effects, value sources, rendering, writes, Definition kinds, and Agent Protocol operations each have one structural owner. Remove prose parsing, duplicated policy/read implementations, silent fallbacks, and mutable name-matched registration while preserving the definition -> validation -> migration -> immutable AppIR -> atomic activation lifecycle.

## Primary outcome

A caller crosses one narrow interface for each cross-cutting concept:

```text
validation rule -> coded Diagnostic + structured facts -> recovery/candidates
compiled View -> one read engine -> database or transaction adapter
Action step -> declared effects -> enforced read/write obligations -> handler
value source -> context-specific resolver/validator -> fail-closed result
render node -> typed client dispatcher -> explicit unknown-node failure
client write -> one encoder/caller -> shared field-error envelope
Definition kind -> compile/normalize/validate/inspect/schema ownership
Agent operation -> sealed metadata + handler -> authorized dispatch
```

The result remains a modular monolith. This goal creates neither a plugin platform nor application-specific behavior.

## Architecture constraints

- Diagnostics keep their public JSON shape, stable codes, wording, paths, candidates, locations, and ordering; internal structured facts are not serialized.
- Human diagnostic wording and Go standard-library error strings are never machine control flow.
- Views remain the only read primitive and Actions remain the only write primitive, including reads performed inside Action transactions.
- The shared View engine accepts only a minimal read adapter and typed expressions/bindings; callers cannot inject SQL or bypass Policy.
- Action-step effects describe enforceable obligations. Handlers cannot bypass the obligation seam with unrestricted read/write access.
- Closed value-source contexts retain explicit allowlists; sharing vocabulary must not make a source valid in an unrelated context. Unknown sources fail closed.
- TypeScript render dispatch includes structural Page/Panel/Region nodes and every server-emitted Block component. Unknown nodes and unsupported expression operators fail loudly.
- Definition normalization keeps explicit dependency phases; registry iteration order never becomes a hidden semantic dependency.
- Registries are immutable after explicit construction. No package `init`, mutable global registration, or runtime name matching.
- SQL and backend-specific behavior remain confined to `internal/dbal/sqlite`, `internal/dbal/postgres`, and `internal/migration`.
- Public definitions, AppIR, diagnostics, CLI/MCP envelopes, HTTP behavior, maintained examples, and activation semantics remain compatible except for explicit fixes to demonstrated policy, sanitization, or silent-failure bugs.

## Milestones and evidence

1. **Structural diagnostics**
   - Every emitted diagnostic receives its public code from its validation rule or a rule-specific constructor.
   - Missing references, missing fields, and unknown fields carry non-serialized structured facts.
   - Recovery suppression, source location, and candidate enrichment consume facts rather than message text.
   - A wording-mutation test proves machine behavior is unchanged when prose changes.

2. **One View read engine**
   - `RunPage` and Action `query` steps use one View-owned plan/materialization implementation through database and transaction read adapters.
   - Policy predicates, owner/tenant fallback, soft deletion, joins, aggregates, ordering, limits, decoding, to-many hydration, content filtering, rich-text sanitization, and redaction have one implementation.
   - Focused adapter-equivalence tests cover policy, rich text, deterministic ordering, limit clamping, and relations.

3. **Enforced Action-step effects**
   - One step-entity resolver is shared by compiler, runtime, and DemoSeed.
   - Declared read/mutation effects select mandatory authorization/predicate obligations.
   - Registry-wide tests prove every entity-reading/mutating handler crosses the required seam.

4. **One value-source catalog**
   - One module owns source names, literal sensitivity, and context-specific validation/resolution.
   - Compiler, expression, Action, Block, Page, and inspection consume that owner.
   - Unsupported sources return errors; registry-parity tests cover every context.

5. **Typed client rendering and expressions**
   - A typed render-node dispatcher owns Page, Panel, Region, and all Block component props.
   - Unknown server components render an explicit tested error.
   - Client expression evaluation implements the compiler-advertised closed operator set or rejects unsupported operators explicitly.

6. **One client write path**
   - Shared encoding and Action calling cover Admin save/delete, ActionRunner, board movement, ActionBlock, and Webform's write adapter.
   - Admin and ActionRunner inputs surface server field errors.
   - Focused tests cover JSON/multipart encoding and deterministic batch failure aggregation.

7. **Complete Definition-kind ownership**
   - The mutable `definition.Kinds` oracle is removed.
   - Registry entries own per-kind normalization and validation hooks; cross-kind phases remain explicit.
   - Completeness evidence compares registry-owned definition collections with the independently declared AppIR storage contract, not another registry-derived list.

8. **Sealed Agent Protocol operations and derived capabilities**
   - One immutable operation entry owns metadata and handler; construction rejects incomplete entries.
   - `Service.Register` and the reachable `BEAN-P5001` state disappear; tests use an explicit constructor seam.
   - Capability lists derive from their closed-algebra owners without introducing registries solely for name listing.

9. **Deletion cleanup and qualification**
   - Remove confirmed zero-caller/pass-through modules and exact helper/constant duplicates where deletion improves locality.
   - Do not centralize representation-specific helpers merely to reduce line count.
   - Documentation and progress records match the implementation.

## Explicit non-goals

- Public extension SDK, dynamic registration, code loading, or dependency-injection container
- New Definition kinds, Action operations/steps, Block types, presentations, field types, or application features
- A universal resolver that makes every value source legal in every context
- Raw SQL, raw Entity reads, or writes outside Actions
- Replacing explicit cross-kind compiler phases with registry ordering
- Application-specific branches in core Go or React code

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
