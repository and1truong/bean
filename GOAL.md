# Goal: Bean v0.8 Agent Protocol

Turn the v0.6 machine-readable CLI contract and v0.7 Demo Factory into one provider-neutral protocol that CLI and MCP clients use without embedding an LLM or duplicating Bean application logic.

## Primary outcome

Codex, Claude Code, Cursor, OpenCode, Pi, and custom clients can discover and invoke the same deterministic Bean operations through either the reference CLI transport or a standards-compliant MCP stdio adapter.

```text
agent provider
  -> CLI or MCP transport
  -> shared Agent Protocol dispatcher
     -> Definition Plane
     -> Release Plane
     -> Application Plane
  -> existing compiler / release / View / Action services
```

The protocol is versioned as `bean.agent/v1alpha1`. Transports may change framing, but operation names, input schemas, authorization decisions, structured results, diagnostics, and errors come from the shared dispatcher.

## Protocol operations

| Plane | Operation | Contract |
| --- | --- | --- |
| Definition | `bean.definition.capabilities` | Return compiler-owned vocabulary and protocol capabilities |
| Definition | `bean.definition.schema` | Return canonical manifest or per-kind JSON Schema |
| Definition | `bean.definition.validate` | Load and compile application source without database mutation |
| Definition | `bean.definition.inspect` | Return redacted AppIR or one definition plus references |
| Release | `bean.release.plan` | Produce a side-effect-free migration preview against a target |
| Release | `bean.release.diff` | Produce a semantic diff against the active release |
| Release | `bean.release.publish` | Compile, migrate, persist, and atomically activate a candidate |
| Release | `bean.release.test` | Run the isolated compile/migration/publish/restart smoke contract |
| Application | `bean.application.query` | Read only through a compiled View |
| Application | `bean.application.execute` | Write only through a compiled Action |

Definition and Release source inputs use normal Bean application files. MCP is a local coding-agent adapter, not a source-upload or remote hosting API.

## Shared dispatcher

The dispatcher owns:

- the operation registry, plane assignment, descriptions, and JSON input schemas
- deterministic input decoding and structured output
- stable protocol errors and compiler diagnostics
- authorization before opening a database or performing work
- credential redaction and deterministic ordering
- delegation to existing compiler, release, View, and Action services

It does not own YAML grammar, migration execution, policy evaluation, SQL, or runtime mutation logic. Those remain in their existing packages.

## CLI reference transport

Existing v0.6 commands remain compatible and delegate to the shared operations. Add a one-to-one machine entry point for contract testing and custom clients:

```bash
bean agent call bean.definition.validate --input request.json --json
bean agent call bean.release.publish --input request.json --json
bean agent call bean.application.query --input request.json --json
```

The CLI envelope remains `bean.cli/v1alpha1`; its `result` is the same structured protocol result returned through MCP. Human output remains a presentation concern of the CLI adapter.

## MCP adapter

The supported v0.8 MCP transport is stdio:

```bash
bean mcp serve --allow-plane definition,release,application
```

It emits only newline-delimited UTF-8 JSON-RPC messages on stdout and may log only to stderr. It supports the current MCP `2026-07-28` stateless request metadata model plus initialization-based compatibility for maintained clients, deterministic `tools/list`, and `tools/call`. Tool results include both structured content and equivalent JSON text.

Streamable HTTP, remote OAuth, subscriptions, prompts, resources, sampling, and elicitation are outside v0.8.

## Authorization contract

Plane access is process configuration, not model-controlled tool input.

- `definition` grants compiler vocabulary, schema, validation, and inspection only.
- `release` grants plan, diff, publish, and isolated test; `publish` remains a Release operation and never becomes an Action.
- `application` grants discovery of Application tools, but every query still passes through a View and every mutation through an Action.
- Missing plane access fails before source loading, target opening, or application execution.
- MCP `tools/list` exposes only operations allowed to that server process.
- The host supplies the runtime user, roles, and tenant context; tool arguments cannot self-assign roles or plane grants.

CLI and MCP tests independently prove each allow/deny boundary. Application tests also prove Entity/table names cannot substitute for Views and arbitrary writes cannot substitute for Actions.

## Agent guidance

Ship:

```text
agents/
  AGENTS.md
  bean.skill.md
  examples/
```

The guidance is provider-neutral and recommends: discover capabilities, inspect relevant definitions, model the smallest domain, validate and repair, preview plan/diff, publish, smoke-test, then improve presentation. Examples cover CLI and MCP configuration without provider-specific runtime branches.

## Measurable acceptance criteria

- The shared registry contains exactly the ten named operations with stable plane assignments and deterministic schemas.
- Existing v0.6 CLI commands and the generic `agent call` path execute shared handlers rather than parallel compiler/release implementations.
- MCP stdio passes official JSON-RPC framing, discovery, tool listing, tool call, malformed-request, unknown-tool, EOF, and stdout-cleanliness tests.
- The same protocol fixture produces semantically identical CLI and MCP structured results for every operation.
- Definition, Release, and Application plane authorization are independently denied and allowed in both transports.
- Release planning remains side-effect-free; publication preserves the immutable AppIR and atomic activation lifecycle.
- Application Plane query/execute tests preserve Policy behavior, View-only reads, Action-only writes, owner/tenant context, and backend parity.
- Tool input cannot select raw tables, arbitrary mutations, SQL, roles, tenants, or grants.
- Agent guidance is complete enough to configure at least one generic stdio MCP client without Bean knowing the provider identity.
- All maintained examples compile and existing v0.6/v0.7 machine contracts remain compatible.
- `make check`, `make test-crash`, `make test-postgres`, and `make build` pass.

## Explicit non-goals

- Embedding an LLM, provider SDK, autonomous loop, chat UI, or prompt orchestration
- MCP Streamable HTTP, hosted MCP, OAuth, API keys, remote identity, or rate-limit infrastructure
- Raw SQL, table CRUD, filesystem tools, shell execution, or arbitrary code tools
- Bean Cloud, preview URLs, sharing, domains, or remote deployment
- Lifecycle semantics, deterministic rules, generated semantic tests, or typed extensions
- Realtime, functions, messaging, Redis, object-storage implementation, containers, or Kubernetes

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
