# Goal: Bean v0.12 Generated Semantic and Rule Tests

Status: active

Generate deterministic checks from Bean's canonical semantic model, Rules, declared TestSuite fixtures, and DemoSeed data. Generated runtime cases materialize as ordinary TestSuite definitions and execute through the v0.11 runner; `bean app test --json` reports stable identities and evidence for every generated check.

## Primary outcome

```text
validated definitions + immutable AppIR
  + explicit TestSuite expectations + deterministic DemoSeed values
  -> generated TestSuite definitions and structural checks
  -> isolated production Rule/Action/View/HTTP execution
  -> stable ordered identities and evidence
```

Given identical definitions and runtime state, Bean generates the same checks, executes them in the same order, and returns identical evidence without retaining test state.

## Frozen v0.12 contract

### Generation boundary

```text
explicit TestSuite cases
  -> generated replay and semantic-negative cases
canonical Entity/Action/Lifecycle/Policy metadata + DemoSeed records
  -> generated CRUD, transition, authorization, route, and journey cases
```

- Generation consumes only compiler-validated AppIR, explicit TestSuite cases, and deterministic DemoSeed records. It does not infer requirements or invent expected business results.
- Every generated runtime case is encoded as a canonical TestSuite definition, compiled into immutable AppIR, and executed by the existing isolated runner.
- Stable generated identities include the source definition kind/name and check family. Ordering never depends on source map order, filesystem order, or database row order.
- Explicit Rule cases generate replay evidence and retain their declared expectation as the oracle. This covers calculations, injected context, runtime errors, and Rule resource limits without evaluating an expectation in the generator.
- Explicit Action cases generate replay evidence. Where metadata proves a denial, the generator adds Policy-negative or invalid-Lifecycle-transition cases with `conflict`, `noChanges`, and `noEvents` assertions.
- DemoSeed supplies representative typed values and relation-complete records for generated CRUD smoke. Applications without sufficient declared data omit the affected runtime case and report no fabricated claim.
- Structural checks cover canonical schema/type validation, Policy and Lifecycle bindings, route bindings, forbidden Rule capability rejection, and Rule resource bounds already enforced by the compiler.
- Route and browser-journey checks exercise published pages through Bean's production HTTP handler; reads remain Views and mutations remain Actions.

### Evidence and bounds

- Existing v0.11 isolation and resource bounds remain authoritative for explicit and generated suites.
- Generated definitions must fit the same suite/case/fixture/encoded-size limits. When source scale would exceed a bound, generation returns a stable diagnostic rather than silently dropping checks.
- Each machine check includes a stable ID, status, source definition identity, check family, and deterministic evidence. Assertion diagnostics retain the v0.11 safe-digest contract.
- Generated checks supplement explicit semantic and browser acceptance tests; passing checks do not claim arbitrary business correctness.

### Machine result

`bean app test --json` retains its versioned envelope, lifecycle smoke checks, and explicit suites, then includes generated evidence:

```json
{
  "checks": [{"id":"generated/schema/Entity/order","status":"passed","source":{"kind":"Entity","name":"order"}}],
  "suites": [{
    "id":"TestSuite/generated__Rule__order_subtotal",
    "status":"passed",
    "cases":[{"id":"TestSuite/generated__Rule__order_subtotal/replay__order_rules__computes_a_line_total","status":"passed"}]
  }]
}
```

- The candidate checksum continues to bind all evidence to one definition digest.
- Failed structural or runtime checks make the command unsuccessful and preserve complete ordered evidence for the checks that ran.

## Reference defect slices

Maintained metadata-only generated-check fixtures must prove:

1. commerce calculation, Rule limit, CRUD, transition, and event defects;
2. ATS Action guard and Policy allow/deny defects;
3. booking context derivation, invariant rollback, route binding, and deterministic replay defects.

The exact suite placement may follow the existing feature-oriented resource layout. Core packages must not branch on application names.

## Architecture constraints

- Preserve definition -> validation -> migration -> immutable AppIR -> atomic activation.
- Generation is a generic compiler/test concern; application-specific expectations stay in metadata under `examples/`.
- Generated reads use Views, generated writes use Actions, and journeys use the production HTTP surface.
- Policy remains authoritative; generated cases never bypass authorization.
- SQL and SQLite dependencies remain confined to the existing backend packages.
- No provider mocks, external network access, arbitrary code, or new scripting surface enters v0.12.

## Measurable acceptance criteria

- Generated negative transition and Policy cases catch seeded defects in maintained reference definitions.
- Generated Rule replays catch seeded guard, validation, calculation, context, and resource-limit defects using explicit expectations as their oracle.
- Generated checks materialize through TestSuite definitions and trace every assertion to stable definition identities.
- Schema, binding, CRUD, route, and browser-journey evidence uses stable IDs and deterministic evidence fields.
- Identical definitions and runtime state produce byte-stable ordered check/suite evidence.
- Existing explicit suites, smoke behavior, examples, SQLite/PostgreSQL parity, and restart compatibility remain green.
- The binary reports `bean 0.12.0-alpha`.
- `make check`, `make test-crash`, `make test-postgres`, and `make build` pass.
- The latest PR commit has a clean Codex review, all actionable threads are resolved, CI passes, and the PR is merged.

## Explicit non-goals

- Inferring unstated business requirements or generating an expected business result by evaluating the implementation under test.
- Replacing explicit application acceptance suites or claiming exhaustive correctness.
- Provider mocks or interaction assertions; those arrive with v0.13.
- Direct Policy/Lifecycle interpreters, PostgreSQL-backed suite execution, watch mode, release gating, load testing, or snapshots.
- External effects, network access, arbitrary scripts, SQL, filesystem, process, or environment capabilities.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
