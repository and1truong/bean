# Goal: Bean v0.9 Semantic Application Model

Add one evidence-driven first-class semantic primitive: `Lifecycle`. Bean must understand an application's state machine structurally instead of requiring every agent and Action to repeat an untyped transition map.

## Primary outcome

An agent can declare lifecycle intent once and Bean can validate, compile, inspect, diff, authorize, and enforce it deterministically:

```text
Lifecycle definition
  -> compiler validation
  -> immutable AppIR
  -> Action transition boundary
  -> Policy-aware runtime enforcement
  -> inspectable Agent Protocol result
```

The maintained applicant-tracking candidate pipeline and commerce order flow are the two unrelated reference applications for this primitive. Both remain metadata-only and use the same generic compiler/runtime path.

## Scope

`Lifecycle` owns the canonical state field and allowed transition graph for one Entity. Milestone 0 freezes the smallest source shape and Action binding that can represent both reference flows, including cases where different Actions or Policies expose different subsets of the lifecycle.

The implementation must provide:

- canonical JSON Schema and compiler capabilities for `Lifecycle`
- stable diagnostics for missing Entity/field references, non-enum state fields, unknown states, invalid edges, and incompatible Action bindings
- immutable AppIR representation with deterministic ordering and compatibility handling
- semantic inspect and diff output through the v0.8 Agent Protocol
- runtime transition enforcement only through Actions, with existing Policy, owner, and tenant context preserved
- SQLite and PostgreSQL parity
- positive and negative contract evidence from at least two unrelated maintained applications

Lifecycle does not create a second mutation path. Views remain the read boundary; Actions remain the write boundary. The transition graph is semantic application metadata, while authorization remains Policy metadata.

## Architecture constraints

- Preserve definition -> validation -> migration -> immutable AppIR -> atomic activation.
- Keep application-specific states and transitions under `examples/`; core packages stay generic.
- Keep SQL and SQLite dependencies within their existing DBAL and migration boundaries.
- Do not duplicate lifecycle rules across compiler, CLI, MCP, and runtime adapters; all transports consume the same compiled semantics.
- Publication of an invalid or incompatible lifecycle must fail before activation.
- Lifecycle changes appear in semantic diff without exposing secrets or storage details.
- Existing Action transition definitions remain compatible for the declared v0.9 compatibility window; any normalization or migration path must be deterministic and tested.

## Measurable acceptance criteria

- Exactly one new semantic primitive, `Lifecycle`, is added in v0.9.
- ATS candidate and commerce order definitions use the primitive without core application-name branches.
- Both applications compile, publish, restart, and enforce every allowed and denied transition on SQLite and PostgreSQL where the existing gate applies.
- Compiler tests reject invalid Entity, state-field, state, edge, and Action/lifecycle combinations with stable structured diagnostics.
- AppIR compatibility tests cover the new format and the supported legacy Action transition representation.
- `bean capabilities`, canonical schemas, definition inspection, semantic diff, CLI, and MCP expose the same Lifecycle semantics through shared v0.8 contracts.
- Runtime tests prove Policy checks occur before mutation and direct Entity/table mutation cannot bypass the lifecycle Action.
- Existing examples and v0.6-v0.8 machine contracts remain compatible.
- `make check`, `make test-crash`, `make test-postgres`, and `make build` pass.
- The pull request is clean under a fresh Codex review of its latest commit and all actionable threads are resolved.

## Explicit non-goals

- Ownership, auditability, soft deletion, terminal-state immutability, or a general invariant framework
- Deterministic rule expressions, generated semantic tests, or typed extensions
- Arbitrary scripts, embedded JavaScript, raw SQL, or another mutation API
- New MCP transports, provider SDKs, embedded LLMs, or agent orchestration
- Destructive migration support or production-envelope qualification
- Application-specific workflow code or a new visual workflow designer

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
