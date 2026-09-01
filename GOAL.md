# Goal: Sealed Internal Capability Registries

Refactor Bean's growing discriminator-based dispatch into explicit, immutable internal capability registries where multiple packages currently repeat the same vocabulary. Preserve every public definition, AppIR, diagnostic, protocol, runtime, storage, and UI behavior.

## Primary outcome

Adding an internal capability has one declared owner and fails closed if its compiler/runtime pieces are incomplete:

```text
capability module
  -> sealed registry entry
  -> compiler/schema/introspection contract
  -> declared effects and runtime handler
  -> deterministic application behavior
```

The result remains a modular monolith. This goal does not create a third-party plugin platform.

## Evidence-driven scope

The initial inventory found three duplicated extension seams:

1. Definition kinds are independently enumerated by compilation, schema generation, lookup, name listing, and reference inspection.
2. Action transaction steps are independently enumerated by compiler allowlists/value rules, runtime dispatch, and DemoSeed safety inference.
3. Block types are independently enumerated by compiler validation, runtime prop construction, and component selection.

Action operations and presentation modes will move behind registries only if the tracer bullets prove that ownership becomes clearer without hiding Policy, transaction, audit, or serialization invariants. Field types, expression operators, DB predicates, migration types, and other deliberately closed algebras retain explicit exhaustive switches.

## Architecture constraints

- Registries are immutable after construction, deterministic, reject duplicate names, and expose no mutable global registration API.
- Registration is explicit; package `init()` side effects and runtime mutation are forbidden.
- Core retains transaction ownership, Policy checks, idempotency, audit/outbox behavior, View-read/Action-write boundaries, migration planning, and atomic activation.
- Capability handlers receive only typed context required for their declared responsibility.
- Compiler and runtime support cannot silently drift; tests compare registered vocabulary and required handlers.
- Effects such as reads, entity mutation, event emission, and job scheduling are declared as data where consumers need safe introspection.
- Public definition syntax, canonical schemas, AppIR format, diagnostics, CLI/MCP envelopes, HTTP behavior, and maintained examples remain compatible.
- SQL and backend-specific behavior remain confined to existing DBAL and migration boundaries.

## Measurable acceptance criteria

- A focused immutable-registry contract proves deterministic names, duplicate rejection, lookup, and construction-time sealing.
- Definition compilation, schema generation, named inspection, name listing, and reference inspection consume one Definition-kind registry rather than parallel kind switches.
- Action-step compiler metadata and runtime handlers share one registered vocabulary; missing handlers or duplicate steps fail tests.
- DemoSeed classifies transaction safety from declared step effects instead of maintaining its own incomplete operation list.
- Block validation, runtime rendering, and component selection consume one Block-type registry.
- Any Action-operation or presentation refactor has equivalent parity tests; otherwise the inventory explicitly records why the exhaustive control flow remains.
- Existing AppIR/diagnostic/Agent Protocol golden behavior remains unchanged.
- `make check`, `make test-crash`, `make test-postgres`, and `make build` pass.
- The pull request is clean under a fresh Codex review of its latest commit and all actionable threads are resolved.

## Explicit non-goals

- Dynamic loading, Go `plugin`, WASM, subprocess, HTTP, or marketplace extensions
- Public plugin SDK or compatibility promise for internal registry interfaces
- New definition kinds, Action operations, transaction steps, presentations, or application features
- Changes to application metadata, migration semantics, storage layout, or AppIR version
- A general dependency-injection container or service locator
- Registry conversion whose only benefit is deleting a small exhaustive switch

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
