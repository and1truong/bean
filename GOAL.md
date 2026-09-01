# Goal: Bean v0.11 First-class Semantic Test Suites

Status: active

Add a canonical `TestSuite` definition that verifies Rule and Action behavior through Bean's production execution paths. Every case is isolated, deterministic, bounded, and reported through the existing machine-readable `bean app test --json` contract.

## Primary outcome

```text
typed TestSuite metadata
  + explicit fixtures and context
  -> compiler validation and immutable AppIR
  -> isolated production Rule/Action execution
  -> deterministic result, mutation, event, audit, and error assertions
  -> stable app test evidence
```

Given the same suite, definition digest, fixtures, input, and explicit context, Bean returns the same ordered case results and diagnostics without retaining any case mutation.

## Frozen v0.11 contract

### Definition

```yaml
kind: TestSuite
name: order_rules
target: {kind: Rule, name: order_subtotal}
tests:
  - name: computes_a_line_total
    input: {quantity: 3, unit_price: 12.5}
    expect: {result: 37.5}
```

- A suite targets exactly one stable `Rule/<name>` or `Action/<name>` identity.
- Case names are non-empty machine names and unique within a suite. Suite and case order is canonical by name.
- Rule cases supply `input`, optional `this`, and explicit context, then assert exactly one of `result` or `error`.
- Action cases supply typed entity fixtures, Action `input`, and explicit context. They may assert `result`, `error`, selected field mutations, emitted outbox events, audit records, `noChanges`, or `noEvents`.
- Fixtures are keyed by Entity name and contain explicit records. They pass Bean field validation and relation checks before case execution.
- Context may provide an actor (`id`, `email`, `displayName`, and roles), tenant ID, fixed RFC3339 time, request ID, deterministic IDs, and a deterministic integer seed. Explicit IDs are consumed first; a seed derives any remaining Action-created UUIDs. Omitted values remain explicitly unavailable; there is no ambient clock, randomness, environment, or network input.
- Expected errors use Bean's stable runtime error code vocabulary rather than matching prose.
- Assertion values use canonical JSON-compatible Bean scalar values. Ordering never depends on source map order or database row order.

### Isolation and bounds

- `bean app test` compiles, migrates, and publishes the candidate into an ephemeral SQLite database, then runs every compiled suite.
- Each case executes against fresh fixtures and cannot observe mutations from another case. State is discarded after the case, including entity writes, audit, idempotency, jobs, and outbox rows.
- The suite runner calls the production Rule evaluator and Action service. It does not duplicate their semantics.
- Maximum 64 suites per application, 128 cases per suite, 1,024 fixtures per case, and 1 MiB canonical encoded suite data per application.
- Each case has a five-second context deadline. Existing Rule bounds remain authoritative.
- Network access and provider mocks do not exist in the v0.11 execution vocabulary.

### Machine result

`bean app test --json` retains its versioned envelope and existing lifecycle smoke checks, then includes canonical semantic-suite evidence:

```json
{
  "checks": [{"id":"definition.load","status":"passed"}],
  "suites": [{
    "id":"TestSuite/order_rules",
    "status":"passed",
    "cases":[{"id":"TestSuite/order_rules/computes_a_line_total","status":"passed"}]
  }]
}
```

- A failed assertion makes the command unsuccessful and reports a stable diagnostic with suite/case/assertion paths plus safe expected and actual values.
- Results and diagnostics are deterministically ordered by suite name, case name, and assertion path.
- The result includes the candidate definition checksum already returned by `app test`; test evidence is therefore tied to one source digest.

## Reference slices

Maintained metadata-only suites must cover:

1. commerce Rule calculation plus Action mutation/event behavior and seeded defects;
2. ATS Action guard allow/deny behavior;
3. booking Action derivation, invariant rollback, mutation, and no-event behavior.

The exact suite placement may follow the existing feature-oriented resource layout. Core packages must not branch on application names.

## Architecture constraints

- Preserve definition -> validation -> migration -> immutable AppIR -> atomic activation.
- `TestSuite` owns compile, normalize, validate, storage completeness, schema, capabilities, inspection, references, and semantic diff.
- AppIR compatibility accepts older formats without suites and rejects suites in formats that cannot represent them.
- Rule cases call the existing typed bounded evaluator with an explicit environment.
- Action cases call the existing Action service with injected clock and deterministic ID providers.
- Policy remains authorization; tests describe behavior but cannot bypass it.
- Fixture setup is runner infrastructure, not an application write API. Assertions observe application records through compiled identities and system event/audit tables through the runner only.
- SQL and SQLite dependencies remain confined to `internal/dbal/sqlite` and `internal/migration`; the runner depends on DBAL interfaces.
- Application behavior remains under `examples/`; core remains generic.

## Measurable acceptance criteria

- Compiler tests cover target existence/kind, case uniqueness, typed input/fixtures/context, assertion compatibility, bounds, and stable diagnostics.
- Canonical schema, capabilities, inspect, references, AppIR storage completeness, compatibility, and semantic diff expose TestSuite behavior deterministically.
- Rule suite tests prove production evaluator reuse, explicit context, result/error assertions, replay determinism, and Rule-bound failures.
- Action suite tests prove production Policy/guard/derive/invariant execution, expected result/error, selected mutation/event/audit assertions, deterministic IDs/time, and no cross-case or post-run persistence.
- Maintained positive and negative suites catch seeded guard, invariant, permission, mutation, event, and Rule-result defects.
- `bean app test --json` returns stable ordered suite/case evidence and diagnostics while retaining compile/migration/publication/restart smoke behavior.
- SQLite execution, PostgreSQL application parity, publication restart compatibility, and existing examples remain green.
- The binary reports `bean 0.11.0-alpha`.
- `make check`, `make test-crash`, `make test-postgres`, and `make build` pass.
- The latest PR commit has a clean Codex review, all actionable threads are resolved, CI passes, and the PR is merged.

## Explicit non-goals

- Generated tests; that is v0.12.
- Provider mocks or interaction assertions; those arrive with the v0.13 typed extension boundary.
- A standalone `bean test` command, watch mode, release gating, performance/concurrency testing, or full database snapshots.
- Direct Policy or Lifecycle suite targets beyond behavior exercised by Actions.
- PostgreSQL-backed semantic-suite execution; v0.11 uses the existing isolated SQLite `app test` environment.
- Network access, external effects, arbitrary scripts, Lua, JavaScript, SQL, filesystem, process, or environment capabilities.
- Inferring unstated application intent or replacing application-specific browser acceptance tests.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
