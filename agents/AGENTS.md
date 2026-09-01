# Building applications with Bean

Use Bean as a deterministic compiler and runtime. Keep application behavior in Bean definitions; do not add application-specific branches to Bean core.

1. Discover the available vocabulary with `bean.definition.capabilities` and `bean.definition.schema`.
2. Inspect only the definitions relevant to the requested change.
3. Model the smallest complete domain in ordinary Bean application files.
4. Validate and repair using diagnostic codes, paths, and candidates.
5. Preview both release plan and semantic diff before publication.
6. Publish through the Release Plane, then run the isolated release test.
7. Verify live behavior through named Views and Actions only.
8. Add presentation and demo data after the behavior is correct.

Declare a `Lifecycle` when an Entity has a business state machine. Put the canonical initial state and graph on Lifecycle, bind transition Actions with `lifecycle`, and use Action-local `transitions` only for a narrower Policy-specific subset. Do not expose the lifecycle field through generic update behavior.

Never request raw tables, SQL, arbitrary mutations, roles, tenant identity, or plane grants as tool input. The host configures Definition, Release, and Application Plane access plus runtime identity. A missing primitive is a design constraint to surface, not permission to generate arbitrary code inside metadata.

Prefer the generic `bean agent call` contract for scripts and the MCP stdio adapter for compatible hosts. Both expose the same ten `bean.agent/v1alpha1` operations described in [the protocol reference](../docs/agent-protocol.md).
