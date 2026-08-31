# Goal: Bean v0.6 agent-readable compiler

Turn Bean's compiler and release lifecycle into a stable machine-facing contract that coding agents can use without parsing human terminal output or inspecting Bean source code.

## Primary outcome

A tool-agnostic client can initialize an application workspace, discover Bean's vocabulary, validate and repair definitions, inspect compiled meaning, preview changes, publish, and run lifecycle smoke tests using versioned JSON responses only.

```text
definitions -> validate -> inspect -> plan -> diff -> publish -> test
                     ^                                  |
                     +-------- structured repair -------+
```

## Command contract

The supported v0.6 loop is:

```bash
bean app init
bean capabilities
bean schema
bean app validate
bean app inspect
bean app plan
bean app diff
bean app publish
bean app test
```

Every command supports `--json`. Human output remains useful, but machine clients never need it.

- `app init` creates the smallest valid source workspace, not a product template.
- `capabilities` describes supported definition kinds, field types, Action operations, presentations, limits, and optional runtime capabilities.
- `schema` emits or locates canonical JSON Schemas for the bundle and each definition kind.
- `app validate` performs source loading, schema validation, compilation, and reference validation without database mutation.
- `app inspect` returns normalized compiled meaning and resolved references for the application or one named definition.
- `app plan` is side-effect-free and reports validation plus the migration/release plan against an optional target database.
- `app diff` reports semantic definition and AppIR changes between the candidate and active release, ignoring source formatting.
- `app publish` validates, plans, applies additive migration, and atomically activates the exact candidate while reporting its checksum and release identity.
- `app test` runs compile, migration, publication, and startup smoke contracts in an isolated SQLite database. Semantic/generated business tests remain a later roadmap phase.

## Machine interface

- JSON responses use one documented, versioned envelope with command identity, success state, result, and diagnostics.
- JSON stdout contains no logs or human prose. Logs go to stderr and are disabled or structured explicitly.
- Collections and suggestions have deterministic ordering; equivalent input and state produce equivalent semantic output.
- Exit statuses distinguish success, definition/validation refusal, command usage, and runtime failure.
- Every public diagnostic has a stable code such as `BEAN-E1001`, a canonical source-relative path, a human message, and source location when available.
- Diagnostics include the offending value only when it is safe and include deterministic candidate suggestions only when the compiler can derive them.
- Stable codes and structured fields follow an explicit compatibility policy; message wording is not an API.
- Sensitive definition inputs, secrets, passwords, file bytes, and database credentials never appear in diagnostics, inspection, plans, diffs, or test output.

## Schema and introspection contract

- Publish canonical schemas for the application manifest/bundle and every supported definition kind.
- Generate schemas and capability descriptions from the same typed vocabulary or verify them against it so documentation cannot silently drift from compilation.
- Represent cross-definition references and compiler-only semantic constraints through inspection/capabilities when JSON Schema cannot express them.
- Include schema/API version and Bean compatibility information in machine responses.
- Keep source-mode commands useful without a database; require a database only for comparisons or activation that depend on persisted state.

## Acceptance scenario

1. A black-box client creates a minimal workspace with `bean app init --json`.
2. It discovers the vocabulary through `bean capabilities --json` and `bean schema --json`.
3. It submits an intentionally invalid applicant-tracking definition.
4. It repairs unknown references, invalid field use, and invalid Action transitions using only diagnostic codes, paths, candidates, schemas, and inspection.
5. `app validate`, `app plan`, and `app diff` succeed without mutating the target database.
6. `app publish` activates the candidate through the existing migration and immutable AppIR lifecycle.
7. `app test` proves the candidate compiles, migrates, publishes, and starts in isolation.
8. Repeating read-only commands against unchanged source/state produces the same normalized payload.

## Measurable acceptance criteria

- Every command in the supported loop has human and JSON black-box tests.
- A JSON-only harness completes the loop without regular expressions over prose.
- Every emitted compiler/loader diagnostic belongs to a tested stable code family; unknown/unclassified errors fail the contract suite.
- Canonical schemas accept every maintained example and reject representative unknown fields and invalid shapes before compilation.
- Inspect output resolves Entity, relation, View, Action, Policy, Webform, Page, and presentation references used by maintained examples.
- Plan and diff are proven side-effect-free by database state checks.
- JSON output is parseable on success and failure and contains no mixed log lines.
- Existing definitions, human CLI workflows, source locations, AppIR compatibility, SQLite/PostgreSQL behavior, and atomic activation remain green.
- A recorded repair benchmark publishes the invalid acceptance fixture without Bean source inspection or human definition edits.

## Explicit non-goals

- Embedding an LLM, prompt UI, or provider-specific agent logic in Bean
- MCP or another network agent protocol; v0.6 first establishes the transport-neutral service and CLI contract
- New CRM, calendar, chat, realtime, OAuth, storage, or infrastructure surfaces
- Application patterns, generated realistic seed data, themes, or hosted sharing
- New lifecycle/ownership semantic primitives
- Generated semantic, policy, transition, or browser tests beyond lifecycle smoke checks
- Arbitrary code, JavaScript, SQL, React, plugin, or extension escape hatches
- Destructive migrations or broader production-readiness claims

## Terminal gates

```bash
make check
make build
```
