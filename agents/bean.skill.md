---
name: build-with-bean
description: Build or modify operational applications using Bean definitions, deterministic validation and release planning, and View/Action runtime boundaries. Use when a repository contains a Bean application manifest or the user asks to build an application on Bean.
---

# Build with Bean

Treat Bean definitions as structured application intent and Bean diagnostics as the repair API.

## Workflow

- Discover capabilities and the relevant canonical schemas before authoring unfamiliar definition kinds.
- Inspect existing definitions before changing them. Preserve the smallest valid model and reuse ordinary definitions rather than inventing hidden behavior.
- Validate after each coherent source edit. Fix diagnostics by stable code and path; candidate strings are suggestions, not authorization to change the domain.
- Run release plan and semantic diff before publish. Stop if the migration contract is incompatible or the diff exceeds the requested change.
- Publish only when requested, then run the isolated release test.
- For live verification, query a declared View and execute a declared Action. Never substitute an Entity name, table, raw SQL, or arbitrary mutation.
- Model an Entity state machine once with Lifecycle. Bind transition Actions to it; add an Action transition map only when that Action intentionally exposes a strict subset of the canonical graph.

## Authority

Plane grants and runtime user, roles, and tenant are host configuration. Do not place them in operation input or infer elevated identity. Definition access does not imply Release or Application access. Publication is a Release operation, not an application Action.

Use [CLI examples](examples/cli.md) when invoking the binary directly and [MCP configuration](examples/mcp.json) when the host supports stdio MCP. Read [the Agent Protocol reference](../docs/agent-protocol.md) for exact operation inputs and compatibility rules.
