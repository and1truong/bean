# Goal: Bean v0.10 Deterministic Rule Expressions

Status: active

Add a bounded, side-effect-free Rule primitive for small local business calculations and predicates while keeping Entity, Lifecycle, Policy, View, and Action semantics structural. A Rule is named, typed, inspectable, and compiled from a canonical structured expression AST; it is not a script or extension runtime.

## Primary outcome

```text
semantic primitive
  + named typed Rule
  -> compiler validation and canonical AppIR
  -> bounded deterministic evaluator
  -> Action guard / derived value / Entity invariant
  -> inspectable working software
```

Given the same Rule, input, record, user, tenant, and explicitly injected context, evaluation returns the same typed result or the same stable failure.

## Frozen v0.10 contract

### Rule definition

```yaml
apiVersion: bean/v1alpha1
kind: Rule
metadata:
  name: order_subtotal
spec:
  entity: order
  result: number
  input:
    quantity: {type: integer}
    unit_price: {type: money}
  expression:
    op: multiply
    args:
      - {source: input, path: quantity}
      - {source: input, path: unit_price}
```

- `entity` is optional; when present it types `this.<field>` and restricts consumers to that Entity.
- `input` declares the Bean scalar field types available through `input.<field>`; sensitive, file, relation, password, and arbitrary executable inputs are forbidden.
- `result` is required and uses the closed Rule types `boolean`, `integer`, `number`, `string`, `date`, `datetime`, or `strings`. Consumers check compatibility with their Bean field type; for example, `money` accepts `number`.
- An expression node is either a leaf (`source` plus `path`, or `literal`) or a known operator with ordered `args`; mixed/unknown node shapes fail compilation.
- AppIR stores the canonical AST, not source formatting, so inspect and semantic diff are deterministic.

### Sources

- `this.<field>`: current/candidate Entity record, including system fields
- `input.<field>`: declared Action input
- `user.id|email|display_name|roles`: authenticated request context, nullable when anonymous
- `tenant.id`: current tenant identifier
- `context.now|request_id`: Bean-injected values
- literal JSON scalars

There is no environment, filesystem, network, SQL, process, module, clock, random, UUID-generator, global-state, result-step, route, or query source.

### Operators

- Boolean: `and`, `or`, `not`
- Comparison: `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `is_null`, `is_not_null`
- Numeric: `add`, `subtract`, `multiply`, `divide`
- Selection: `if`
- String: `concat`, `lower`, `upper`, `trim`

Operators have fixed arity and result typing. There are no user functions, loops, recursion, dynamic property lookup, mutation, or implicit coercion.

### Initial consumers

- `Action.when: <Rule>` requires a boolean result and is evaluated inside the Action transaction after existing Policy checks and before mutation.
- `Action.derive.<input>: <Rule>` computes a declared Action input inside the transaction. Client supply of a derived input is rejected; derives are simultaneous and cannot reference other derived inputs.
- `Entity.validations.<name>: <Rule>` requires a boolean Rule for the same Entity and validates create/update/transition candidates before persistence.

Policy remains authorization. A Rule guard or invariant may further deny behavior but can never grant access or bypass Policy. Reads still use Views and writes still use Actions.

### Bounds

- maximum 128 expression nodes
- maximum depth 16
- maximum literal encoding 4 KiB
- maximum intermediate or final encoded value 16 KiB
- deterministic short-circuit evaluation with a maximum 128 evaluated nodes

The compiler rejects statically visible bound violations. Runtime enforces the same evaluation and result bounds.

## Reference slices

Three unrelated metadata-only applications must demonstrate the complete initial surface:

1. commerce: numeric computed/derived value
2. ATS or tracker: record-aware Action guard
3. booking or tracker: Entity invariant plus a derived `context.now` value

The exact examples may be adjusted only when their existing domain model proves a smaller credible slice.

## Architecture constraints

- Preserve definition -> validation -> migration -> immutable AppIR -> atomic activation.
- Rule evaluation has no I/O and receives only an explicit typed environment.
- Existing Policy, Lifecycle, transaction, idempotency, audit/outbox, Action-step effect, and View-read boundaries remain authoritative.
- Compiler validates unknown sources, paths, operators, arity, consumer Entity, result types, and forbidden result capabilities before publication where possible.
- Runtime fails closed on missing values, type mismatch, divide-by-zero, resource limits, and unavailable context.
- Rule Definition ownership includes compile, normalize, validate, storage completeness, schema, inspect, references, and semantic diff.
- AppIR compatibility accepts older formats without Rules and rejects Rules in formats that cannot represent them.
- Core remains generic; all domain behavior stays under `examples/`.
- SQL and backend-specific behavior remain in existing DBAL and migration boundaries.

## Measurable acceptance criteria

- Rule core tests prove type checking, evaluation replay determinism, short-circuiting, all operators/sources, and every resource limit.
- Compiler diagnostics cover unknown fields, sources, operators, invalid arity, incompatible results/consumers, forbidden result types, missing references, and derive cycles.
- Canonical schema, capabilities, inspect, references, and semantic diff expose Rule behavior with deterministic ordering.
- Action tests prove Policy-before-guard, guard denial, simultaneous derivation, client override refusal, Entity invariants, transaction rollback, idempotent replay, and explicit `context.now`.
- Three reference applications use Rules without application-name branches.
- SQLite/PostgreSQL and publication restart compatibility remain green.
- The binary reports `bean 0.10.0-alpha`.
- `make check`, `make test-crash`, `make test-postgres`, and `make build` pass.
- The latest PR commit has a clean Codex review, all actionable threads are resolved, CI passes, and the PR is merged.

## Explicit non-goals

- Text/infix script parser or Bloblang compatibility
- Computed read columns, SQL expressions, materialized fields, or Rule-backed View filtering
- Conditional browser visibility or requiredness; existing typed form expressions remain unchanged
- Public extension/plugin SDK, dynamic registration, JavaScript, Lua, WASM, shell, HTTP, SQL, or provider calls
- Generated semantic tests; that is v0.11
- External effects; that is v0.12
- New semantic primitives or application-specific core branches

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
