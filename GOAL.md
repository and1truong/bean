# Goal: Bean v0.13 Typed Extension Boundary

Status: complete

Add one narrow, typed, out-of-process HTTP extension boundary for external effects. Applications declare the contract in metadata; Actions atomically persist extension invocations, and the existing outbox delivers them after commit without weakening Policy, audit, idempotency, or transaction guarantees. Semantic TestSuites replace delivery with typed mocks and assert interactions offline.

## Primary outcome

```text
validated Extension + Action metadata
  -> immutable AppIR
  -> authorized Action transaction
  -> durable typed extension intent
  -> bounded HTTP provider delivery
  -> deterministic delivery state and test evidence
```

The same Extension definition, typed input, invocation identity, and provider contract are used by production HTTP delivery and Semantic TestSuite mocks.

## Frozen v0.13 contract

### Extension definition

- Add one canonical `Extension` definition with named typed input and output fields.
- The initial transport is `http` with one absolute endpoint. No in-process code loading, dynamic registration, or provider-specific core branch is added.
- Permissions and side effects use closed vocabularies. v0.13 supports only the declared outbound-network permission and external-write effect.
- Authentication is declared as `none` or `bearer`. Bearer credentials are host configuration, never application metadata, AppIR inspection output, logs, audit payloads, outbox payloads, or test evidence.
- Timeout and retry are explicit bounded values. Retry uses a fixed delay, a bounded attempt count, and stable failure classes; it has no jitter or implicit provider policy.
- Idempotency is required. Each committed invocation has one immutable invocation ID used as the HTTP idempotency key across every retry.
- Transaction semantics are exactly `after_commit`: an Action writes the invocation intent in the same transaction as domain mutation, audit, and Action idempotency state. HTTP never runs inside the database transaction, and provider output cannot feed a later step in that transaction.
- Failure semantics are at-least-once delivery with persistent pending/delivering/delivered/failed state. Timeout, connection failure, and retryable HTTP status follow the declared retry bound; invalid output and non-retryable responses fail deterministically.

### Action and HTTP boundary

- A transaction Action may use an `extension` step that references one compiled Extension and binds only declared typed inputs from existing Action value sources.
- Policy and Rule guards run before the transaction. The extension intent commits or rolls back with the Action's domain changes and audit record; Action idempotency replay does not enqueue a second intent.
- The portable wire contract is a versioned JSON `POST` carrying extension name, invocation ID, idempotency key, and typed input. The provider returns a bounded JSON object matching the declared output schema.
- The HTTP client enforces the declared timeout, response-size bound, status classification, output typing, and redirect refusal. It sends bearer auth only when the active host configuration supplies the required credential.
- Provider errors expose stable categories and safe details. Secrets and arbitrary response bodies are never persisted as errors.

### Semantic TestSuite contract

- Action cases may declare typed provider mocks keyed by Extension name and an ordered response or stable failure.
- Expectations may assert ordered extension interactions including Extension identity, invocation identity, idempotency key, and typed input.
- The isolated runner executes the production Action intent and delivery paths with the mock; it performs no network access and retains the existing case timeout, fixture, and encoded-size bounds.
- Missing mocks, unexpected calls, invalid mock output, exhausted responses, or unconsumed expected interactions fail with stable TestSuite diagnostics.

### Reference slice

- Commerce declares a generic order-notification HTTP Extension and invokes it from `place_order` after the order intent is committed.
- Its metadata TestSuite mocks the provider, asserts the typed call, and continues to prove inventory mutation, order creation, event, audit, and Action idempotency behavior.
- Core packages contain no commerce or provider name branches.

## Architecture constraints

- Preserve definition -> validation -> migration -> immutable AppIR -> atomic activation.
- Extension definitions and Action bindings are metadata; core packages remain generic.
- Reads use Views and writes use Actions. Extensions cannot query or mutate Bean storage directly.
- External effects are after-commit outbox delivery and explicitly at-least-once, never described as exactly-once.
- AppIR compatibility is explicit: Extension semantics require the new format; older v1-v4 releases remain readable without Extensions.
- SQL and backend dependencies remain confined to existing DBAL/migration packages.

## Measurable acceptance criteria

- Schema, capabilities, validation, inspect, references, semantic diff, immutable AppIR, release/restart, and compatibility expose the same Extension contract.
- Compiler tests reject unknown Extensions, untyped or extra bindings, unsupported permissions/effects/auth/transport/transaction modes, unsafe endpoints, and values outside fixed timeout/retry bounds.
- Focused Action tests prove authorization-before-intent, transaction rollback, audit atomicity, Action idempotency replay, stable invocation keys, and no in-transaction HTTP.
- HTTP provider tests prove typed request/response, bearer handling without secret leakage, timeout, redirect refusal, response bounds, deterministic retry classes, fixed retry schedule, stale-claim recovery, and terminal failure.
- Semantic TestSuite tests prove offline typed mocks, ordered interaction assertions, stable failures, isolation, and byte-stable repeated evidence.
- The commerce reference source validates, publishes, restarts, runs its mocked extension suite, and preserves SQLite/PostgreSQL Action behavior.
- Arbitrary inline JavaScript, SQL, React, Lua, WASM, shell, filesystem, process, dynamic module, and direct database capabilities remain unsupported.
- The binary reports `bean 0.13.0-alpha`.
- `make check`, `make test-crash`, `make test-postgres`, and `make build` pass.
- The latest PR commit has a clean Codex review and green CI, or the ten-review limit is reached with every final finding addressed, all threads resolved, and final validation and CI green; the PR is squash merged.

## Explicit non-goals

- Synchronous extension results inside an Action transaction or using provider output for domain mutation.
- Exactly-once external effects, distributed transactions, provider-side deduplication guarantees, or compensating workflows.
- WASM, Go plugin loading, functions platforms, embedded scripts, generic webhooks unrelated to typed Actions, or a provider SDK ecosystem.
- OAuth flows, secret management products, arbitrary headers, mutual TLS, remote MCP transport, queues beyond the existing outbox, or new infrastructure services.
- Direct Extension TestSuite targets, generated provider mocks, snapshots, release gating, load testing, or production SLO claims.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
