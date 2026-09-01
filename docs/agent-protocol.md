# Bean Agent Protocol

Bean v0.8 exposes one provider-neutral `bean.agent/v1alpha1` dispatcher. CLI and MCP change only framing; operation names, input schemas, plane authorization, compiler diagnostics, runtime results, and errors originate from the same service.

## Operations

| Plane | Operation | Input |
| --- | --- | --- |
| Definition | `bean.definition.capabilities` | `{}` |
| Definition | `bean.definition.schema` | `{"kind":"View"}`; kind is optional |
| Definition | `bean.definition.validate` | `{"file":"./app.yaml"}` |
| Definition | `bean.definition.inspect` | `{"file":"./app.yaml","kind":"Entity","name":"candidate"}`; kind/name are optional together |
| Release | `bean.release.plan` | `{"file":"./app.yaml","target":"./bean.db"}`; target is optional |
| Release | `bean.release.diff` | `{"file":"./app.yaml","target":"./bean.db"}`; target is optional |
| Release | `bean.release.publish` | `{"file":"./app.yaml","target":"./bean.db"}` |
| Release | `bean.release.test` | `{"file":"./app.yaml"}` |
| Application | `bean.application.query` | `{"target":"./bean.db","view":"candidates","params":{}}` |
| Application | `bean.application.execute` | `{"target":"./bean.db","action":"create_candidate","input":{...}}` |

Discovery returns canonical Draft 2020-12 input schemas. Inputs reject unknown top-level properties. Application input cannot select tables, SQL, arbitrary mutations, roles, tenants, or plane grants.

## CLI reference transport

```bash
bean agent call bean.definition.validate --input request.json --json
```

The local CLI defaults to all planes and supports `--allow-plane` for a constrained invocation. Successful protocol data is placed in the existing `bean.cli/v1alpha1` envelope's `result`. Protocol errors use stable codes:

| Code | Meaning |
| --- | --- |
| `BEAN-P1001` | unknown operation |
| `BEAN-P1002` | operation plane is not granted |
| `BEAN-P1003` | malformed or unknown operation input |
| `BEAN-P3001` | redacted target/runtime failure |
| `BEAN-P5001` | registered operation has no implementation |

Definition and migration failures retain their existing `BEAN-E*` diagnostics.

## MCP stdio adapter

```bash
bean mcp serve --allow-plane definition,release,application
```

The safe default is Definition Plane only. The process host may configure runtime identity with `--user-id`, `--user-email`, `--roles`, and `--tenant-id`; these values are never MCP tool arguments.

The adapter implements newline-delimited UTF-8 JSON-RPC over stdin/stdout, current MCP `2026-07-28` `server/discover`, `tools/list`, and `tools/call`, plus initialization compatibility for maintained `2024-11-05`, `2025-03-26`, `2025-06-18`, and `2025-11-25` clients. Modern results include `resultType`, private cache metadata, and server identity metadata. Tool calls return equivalent JSON text and `structuredContent`. Only MCP messages are written to stdout.

The current framing follows the official [MCP discovery](https://modelcontextprotocol.io/specification/2026-07-28/server/discover), [tools](https://modelcontextprotocol.io/specification/2026-07-28/server/tools), and [stdio](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/stdio) contracts. Streamable HTTP, OAuth, hosted identity, prompts, resources, sampling, and subscriptions are outside v0.8.

## Authorization and runtime boundaries

Plane checks run before source loading, target opening, or handler execution. MCP discovery hides tools outside the process grant. An unavailable tool returns an indistinguishable unknown/unavailable error rather than revealing its arguments or existence.

Application calls still execute normal runtime Policy logic with the host-supplied request identity. `query` invokes only `view.Service.RunPage`; `execute` invokes only `action.Service.Execute`. Owner, tenant, role, field-redaction, validation, transaction, audit, job, outbox, and idempotency behavior therefore remains identical to HTTP runtime behavior.
