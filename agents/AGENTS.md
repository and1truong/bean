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

Use a named `Rule` only for a bounded local predicate or calculation after checking that an existing semantic primitive does not own the behavior. Declare every `input` type and the closed Rule `result` type, use only capabilities-reported sources/operators, bind boolean guards with `Action.when`, server-owned derived inputs with `Action.derive`, and same-Entity invariants with `Entity.validations`. Policy authorizes before Rules; never encode authorization in a Rule. Do not invent text scripts, ambient clock/randomness, I/O, dynamic lookup, or dependencies between derived inputs.

Never request raw tables, SQL, arbitrary mutations, roles, tenant identity, or plane grants as tool input. The host configures Definition, Release, and Application Plane access plus runtime identity. A missing primitive is a design constraint to surface, not permission to generate arbitrary code inside metadata.

For ordered guided content, generate an ordinary `Sequence` whose frames reference Panels. Compose semantic `content` Blocks and existing View Blocks inside those Panels; do not invent Presentation, Slide, Report, HTML, CSS, JavaScript, SVG, or private query definitions. Use the capability response for supported frame layouts, content elements, aspect ratios, and resource limits. Repair only compiler-diagnosed paths, then inspect and diff the same definitions before publication.

Prefer the generic `bean agent call` contract for scripts and the MCP stdio adapter for compatible hosts. Both expose the same ten `bean.agent/v1alpha1` operations described in [the protocol reference](../docs/agent-protocol.md).
