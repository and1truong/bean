# Bean roadmap

## Product thesis

Bean is the deterministic application runtime that agents can reliably build against.

```text
Intent
  -> probabilistic agent
  -> Bean definitions
  -> deterministic compiler
  -> immutable AppIR
  -> runtime
  -> working software
```

Bean is not trying to become another general-purpose low-code platform. Its advantage is that an agent can express application intent in a small typed definition set and receive deterministic validation, migration, release, runtime, and test behavior in return.

The target product description is:

> Bean is a compiled application runtime designed for humans and AI agents to build complete operational software from high-level definitions.

The application-logic boundary is:

> Primitives describe structure. Rules describe local deterministic logic. Extensions perform external effects.

Bean continues to prefer first-class semantics whenever the compiler, runtime, inspection, or test system benefits from understanding behavior structurally. Lifecycle transitions, ownership, and Policy relationships remain first-class Bean semantics. Rules may refine those semantics with local predicates and calculations; they do not replace semantic primitives with scripts.

## North Star

From an ordinary English prompt, a coding agent can produce and publish a credible demo application in less than five minutes, with no human modification after the initial prompt.

Example benchmark prompt:

```text
Build a lightweight applicant tracking system.
Candidates, jobs, interview stages, notes,
kanban pipeline, candidate detail, search.
```

The measured path is:

```text
prompt -> definitions -> validate -> plan -> publish -> test -> working demo
```

Each benchmark case declares its required entities, relationships, workflows, pages, searches, demo data, behavior rubric, and presentation rubric before a run. Feature count is not a success metric.

### Benchmark protocol

Record the agent, model, and version; tool and skill versions; Bean version; initial prompt; token and tool-call budgets; cold or warm run state; elapsed time; validation attempts; human interventions; behavior score; and presentation score for every run.

The benchmark target is:

- p50 elapsed time under five minutes
- p90 elapsed time under ten minutes
- zero human modification after the initial prompt
- 100% of the required behavior rubric

Presentation remains a separately reported score so a technically correct but unusable demo cannot hide behind the behavior result. Comparisons must disclose changed models, reasoning levels, budgets, prompts, tools, skills, and cache state.

## Product rules

- The agent may be probabilistic; compilation, planning, publication, and runtime behavior must be deterministic.
- Compiler diagnostics are a public agent API, not incidental prose.
- Every new core primitive must increase the set of applications an agent can describe.
- Application behavior stays in metadata; core packages stay generic.
- Bean owns application semantics. Providers own infrastructure capabilities.
- Prefer an existing semantic primitive, then a deterministic rule expression, then a typed extension—in that order.
- Definitions -> validation -> migration -> immutable AppIR -> atomic activation remains the lifecycle.
- Reads use Views and writes use Actions.
- Bean integrates with infrastructure through adapters instead of rebuilding infrastructure products.
- A phase starts only when its predecessor's exit criteria are met. Version labels express dependency order, not calendar commitments.

## Current engineering goal

Bean v0.13 Typed Extension Boundary is active. It adds one metadata-declared, out-of-process HTTP contract for typed external effects. Authorized Actions atomically persist after-commit extension intents; bounded outbox delivery preserves stable invocation identity, retry, failure, audit, and Action idempotency semantics. Semantic TestSuites replace the same provider path with typed offline mocks and ordered interaction assertions. The executable contract and milestones are in `GOAL.md` and `PLANS.md`.

## Roadmap

| Phase | Target | Outcome | Status |
| --- | --- | --- | --- |
| 0 | v0.5 | Compiled application runtime and complex metadata-only reference applications | complete |
| 1 | v0.6 | Agent-readable compiler and machine-stable CLI contract | complete |
| 2 | v0.7 | Demo Factory for fast, populated, presentable local applications | complete |
| 3 | v0.8 | Agent Protocol, MCP adapter, and shipped agent guidance | complete |
| 4 | v0.9 | First-class semantic model for common business structure | complete |
| 5 | v0.10 | Deterministic Rule Expressions for local application logic | complete |
| 6 | v0.11 | First-class Semantic Test Suites for Bean definitions | complete |
| 7 | v0.12 | Generated tests from semantic primitives and rules | complete |
| 8 | v0.13 | Typed extension boundary for external effects | active |
| 9 | v1.0 | Qualification of one explicit production envelope | planned |
| 10 | post-v1.0 | Bean Cloud preview environments | exploratory |
| 11 | post-v1.0 | Composable application-pattern ecosystem | exploratory |

### Phase 0 — Application runtime (complete)

Bean already compiles typed definitions into immutable AppIR, plans additive migrations, and atomically activates applications on SQLite and PostgreSQL. Entity, relation, View, Action, Policy, Webform, Page, Admin, Studio, board, tree, file, job, and outbox primitives exist. The metadata-only blog and Asana Lite applications prove that non-trivial application behavior does not need application-specific core branches.

This foundation is broad enough. v0.6 does not add another horizontal product surface.

### Phase 1 — v0.6: agent-readable compiler

Make the compiler and release lifecycle safe to drive without parsing terminal prose.

Required command loop:

```bash
bean app init
bean app validate
bean app inspect
bean app plan
bean app diff
bean app publish
bean app test
```

Every command supports `--json`, a versioned response envelope, documented exit statuses, deterministic ordering, and clean separation between machine output and logs. `app plan` is side-effect-free. `app diff` reports semantic definition/AppIR changes rather than YAML formatting changes. `app publish` reports the candidate checksum and applied release. `app test` runs compile, migration, publication, and startup smoke contracts in an isolated SQLite database; first-class semantic test suites are deferred to v0.11.

Self-description is part of the same contract:

```bash
bean capabilities --json
bean schema --json
bean app inspect --file ./app.yaml Entity candidate --json
```

Publish canonical bundle and per-kind JSON Schemas from the same typed vocabulary used by the compiler. Give every diagnostic a stable code, canonical path, source location when available, offending value when safe, and deterministic candidate suggestions when applicable. Human messages may improve without breaking clients; codes and structured fields follow the declared compatibility policy.

Exit criteria:

- A black-box client can execute the complete loop using JSON only.
- Invalid fixtures cover every public diagnostic family and assert stable codes and payload shape.
- A repair benchmark reaches a valid publication using only schemas, capabilities, inspection, and diagnostics—not Bean source inspection.
- Existing human CLI behavior, examples, release invariants, SQLite/PostgreSQL contracts, `make check`, and `make build` remain green.

The executable goal and milestones are in [GOAL.md](GOAL.md) and [PLANS.md](PLANS.md).

### Phase 2 — v0.7: Demo Factory

Optimize time-to-credible-demo using a deliberately bounded vocabulary: Entity, Relation, List, Detail, Form, Board, Tree, Dashboard, Metric, Timeline, Action, Search, and Attachment.

Add composable application patterns for CRUD resources, workflows, approvals, parent/child records, many-to-many tagging, ownership, assignment, comments, activity history, and dashboards. A v0.7 workflow composes an enum, Action, and transition; first-class `Lifecycle` semantics remain deferred to v0.9. Patterns begin as inspectable definition composition, not hidden runtime behavior.

Evolve today's literal seed fixtures into deterministic, realistic fixture generation. Add a small typed theme contract such as name, preset, and accent; agents should not generate CSS. Produce a portable local demo with:

```bash
bean demo seed
bean package
```

`bean package` is the v0.7 delivery target: one executable/application bundle plus SQLite data. `bean share` is deferred until the Cloud phase rather than introducing an early hosting dependency.

Exit criteria:

- A maintained prompt suite produces populated, coherent demos with no application-specific core code.
- The reference agent harness meets the North Star p50 and p90 targets under the declared benchmark protocol.
- Each demo passes its declared behavior rubric and opens with meaningful data rather than empty CRUD screens.
- No benchmark run receives human modification after the initial prompt.
- Adding patterns does not create a second DSL or bypass compiler validation.

### Phase 3 — v0.8: Agent Protocol

Formalize the v0.6 contract without embedding an LLM in Bean core.

Expose provider-neutral operations through three explicit planes:

```text
Agent Protocol
├── Definition Plane
│   schema / capabilities / validate / inspect
├── Release Plane
│   plan / diff / publish / test
└── Application Plane
    View reads / Action writes
```

CLI remains the reference transport; MCP is an adapter over the same service contracts rather than a parallel implementation. Authorization is defined independently for each plane. The Definition Plane grants access to definition and compiler information, the Release Plane controls preview and activation privileges, and only the Application Plane inherits the runtime rule that reads use Views and writes use Actions. In particular, `publish` is a privileged release operation, not an ordinary Action.

Ship repository guidance and examples:

```text
agents/
  AGENTS.md
  bean.skill.md
  examples/
```

The recommended loop is inspect capabilities, model the smallest domain, generate definitions, validate and repair, publish, smoke-test, then improve presentation.

Exit criteria:

- CLI and MCP pass the same protocol contract suite.
- Codex, Claude Code, Cursor, OpenCode, Pi, and custom clients can integrate without Bean knowing their identity.
- Definition, Release, and Application Plane authorization contracts are independently tested across CLI and MCP.
- Application Plane authorization preserves the same View-read and Action-write boundaries as HTTP and Admin.

### Phase 4 — v0.9: semantic application model

Add first-class business semantics only where reference applications prove repeated need. `Lifecycle` state machines are the first slice; later candidates include ownership, auditability, soft deletion, terminal-state immutability, and declarative invariants.

For example, a definition should directly express that managers may approve submitted invoices and approved invoices are immutable. The agent should not need to synthesize equivalent conditionals in application code.

Each semantic primitive must compile into AppIR, participate in validation and inspection, preserve backend parity, and supply negative as well as positive contract tests. Lifecycle is the first slice; later primitives require separate evidence rather than landing as one broad framework.

Exit criteria:

- At least two unrelated reference applications reuse each accepted semantic primitive.
- Illegal transitions and policy combinations fail at compile time where possible and deterministically at runtime otherwise.
- Semantics remain visible in schema, capabilities, inspect, and diff, with stable identifiers and evidence for later rule refinement and test generation.

### Phase 5 — v0.10: Deterministic Rule Expressions

Increase the local application logic agents can express without arbitrary application code or weakening Bean's deterministic, inspectable model. Rules sit between first-class semantic primitives and the later typed extension boundary:

```text
Semantic application model
        ↓
Deterministic rule expressions
        ↓
First-class Semantic Test Suites
        ↓
Generated semantic tests
        ↓
Typed extension boundary
```

Without rules, an agent often has to request another Bean primitive or escape into application code. With rules, the intended path is:

```text
agent intent
  ↓
typed Bean structure
  +
small local expressions
  ↓
compiler validation
  ↓
inspectable executable application
```

This should materially increase the applications that remain entirely inside Bean definitions. The selection order is: existing Bean semantic primitive, deterministic rule expression, then typed extension.

#### Semantic boundary

Do not replace structure with scripts. This loses the lifecycle meaning Bean needs to validate, inspect, diff, authorize, and test:

```yaml
approve:
  script: |
    if invoice.status == "submitted" {
      invoice.status = "approved"
    }
```

Keep the transition first-class and use a rule only for the local predicate:

```yaml
lifecycle:
  submitted:
    approve: approved

actions:
  approve:
    when: |
      user.role == "manager" &&
      this.total <= user.approval_limit
```

The same boundary applies to ownership and Policy relationships: Bean retains their structural meaning while rules refine declared behavior. Authorization still compiles through Bean Policy semantics; a rule does not create a parallel authorization system.

#### Rule-language contract

Prefer a deliberately narrow expression and data-transformation model with Bloblang-like properties over an unrestricted embedded language. Bloblang is not an implementation commitment; repository evidence and vertical slices must justify that choice separately.

Rules require:

- deterministic, side-effect-free, bounded evaluation
- bounded memory, result size, and execution complexity
- an explicitly typed result required by the consuming field or rule surface
- stable validation and runtime diagnostics
- visibility through schema, capabilities, inspect, and semantic diff
- stable semantics and evidence consumable by generated tests

Rules have no filesystem, network, process execution, direct database access, arbitrary SQL, environment-variable reads, module loading, or global mutable state. They have no implicit clock or random source and cannot call an ambient UUID generator; UUID values must be explicitly injected. Arbitrary JavaScript and Lua are not available.

Context is explicit and injected, for example:

```text
this
input
user
tenant
parent
context.now
context.request_id
```

The deterministic contract is:

```text
same rule
+ same input
+ same injected context
= same result
```

#### Initial rule surfaces

Rules do not become available arbitrarily throughout definitions. Start with:

- computed fields and values
- validation predicates
- Action guards through `when`
- derived Action inputs and values
- conditional requiredness
- conditional UI visibility where it does not affect authorization
- declarative invariant predicates
- deterministic mappings and transformations
- metric and calculated-value expressions

UI visibility controls presentation only. It never substitutes for server-side validation or authorization.

#### Reference examples

Approval threshold retains the lifecycle while refining the `approve` transition with a local business predicate:

```yaml
lifecycle:
  initial: draft
  states:
    draft:
      transitions:
        submit: submitted
    submitted:
      transitions:
        approve: approved
        reject: rejected

actions:
  approve:
    when: |
      user.role == "manager" &&
      this.total <= user.approval_limit
```

A tiered discount is computed business logic that does not justify a new Bean primitive:

```yaml
fields:
  discount_rate:
    computed: |
      if this.customer.type == "enterprise" {
        this.customer.negotiated_discount
      } else if this.customer.tier == "gold" && this.subtotal >= 100 {
        0.10
      } else {
        0
      }
```

Conditional validation expresses that purchases above $10,000 require a purchase-order number:

```yaml
validations:
  purchase_order_required:
    rule: |
      this.total <= 10000 ||
      this.purchase_order_number != null
```

A derived value captures the deterministic request timestamp when an order is submitted:

```yaml
actions:
  submit:
    derive:
      submitted_at: |
        context.now
```

`context.now` is injected by Bean. Rules cannot call a global `now()` function.

A computed SLA state uses that same explicit evaluation time:

```yaml
fields:
  overdue:
    computed: |
      this.status != "resolved" &&
      this.due_at < context.now
```

Conditional form presentation shows a rejection reason only for the rejected outcome:

```yaml
fields:
  rejection_reason:
    visible_when: |
      input.outcome == "rejected"
```

This last rule controls presentation only; server-side validation and authorization remain separate contracts.

#### External-effect boundary

Rules must not send email, call Stripe, fetch HTTP endpoints, write arbitrary database rows, publish Kafka events directly, read local files, execute shell commands, or invoke arbitrary JavaScript or Lua. Those behaviors belong behind a typed extension or provider boundary:

```text
Action
  ↓
Bean-controlled transaction / semantics
  ↓
declared typed external effect
  ↓
extension/provider
```

#### Exit criteria

- At least three unrelated reference applications use rules without application-specific core code.
- Together they demonstrate computed values, Action guards, validation, derived values, and one context-dependent rule using injected `context.now`.
- Rules are visible through schema, capabilities, inspect, semantic diff, and machine diagnostics.
- Validation catches unknown fields, invalid operators or functions, incompatible result types, and forbidden capabilities before publication where possible.
- Identical rules, inputs, and injected context evaluate deterministically.
- Resource limits are enforced and covered by black-box tests.
- Rules cannot perform I/O or obtain undeclared environment or runtime state.
- Existing semantic primitives remain structural and are not replaced by scripts.
- No arbitrary Lua, JavaScript, SQL, network, filesystem, or process execution enters Bean core.

v0.10 is not a general-purpose scripting runtime. Lua may be reconsidered later only behind the typed extension boundary or another strongly sandboxed provider abstraction when a concrete vertical slice proves the need.

### Phase 6 — v0.11: first-class Semantic Test Suites

Add the deterministic behavioral-test contract proposed in [issue #12](https://github.com/and1truong/bean/issues/12). A suite targets stable Bean definition identities and executes through the same engine used in production rather than a separate test interpreter.

Cases provide typed fixtures, input, and explicit actor, tenant, clock, ID, and randomness context. They assert externally meaningful results or errors, object mutations, emitted events, and audit records. Network access is disabled by default. Every case runs in isolation with ephemeral storage or transaction rollback and bounded CPU, memory, and execution time.

The initial contract supports Rules and Actions. Policy and Lifecycle targets may follow only where the same assertion vocabulary and execution path apply. The existing `bean app test --json` command returns stable, machine-readable diagnostics suitable for humans, CI, and repair loops. Agents may derive suites from definitions and requirements, but generated cases do not infer unstated intent or replace application-specific acceptance tests.

v0.11 does not invent a provider mock API before Bean has a production provider contract. Typed provider-call mocks and interaction assertions join the suite vocabulary with the v0.13 typed extension boundary so tests continue to exercise the production execution path.

Exit criteria:

- A canonical JSON/YAML suite contract targets stable Rule and Action identities and is visible through schema, capabilities, validation, inspection, and semantic diff.
- Suites provide fixtures plus explicit actor, tenant, and fixed-time context; they assert results or errors, mutations, and emitted events.
- Every case rolls back all state and produces the same deterministically ordered result and structured diagnostics for the same suite, definition digest, and context.
- The suite runner invokes production Rule and Action execution paths and enforces declared resource bounds.
- Maintained positive and negative suites catch seeded guard, invariant, permission, mutation, and event defects.

### Phase 7 — v0.12: generated semantic and rule tests

Use the semantic model, deterministic rules, and Semantic Test Suite contract to generate schema, policy, transition, route-binding, rule-evaluation, CRUD smoke, and browser-journey checks. `bean app test --json` reports stable check identifiers and evidence.

Generated checks cover rule type correctness, evaluation fixtures, Action guard allow/deny cases, invariant violations, representative boundary values, deterministic replay, forbidden capability access, and execution or resource-limit failures. They verify declared semantics and representative boundaries; they do not infer unstated requirements or prove arbitrary business correctness. Generated checks supplement application-specific acceptance tests.

Exit criteria:

- Generated negative transition and Policy cases catch seeded defects in reference definitions.
- Generated rule tests catch intentionally seeded guard, validation, calculation, context, and resource-limit defects.
- Generated checks materialize through the Semantic Test Suite contract and trace assertions to stable definition identities.
- Identical definitions and runtime state produce deterministically ordered results and evidence.

### Phase 8 — v0.13: typed extension boundary

Provide a narrow escape hatch without allowing arbitrary scripts throughout metadata:

```text
Bean core -> typed extension boundary -> custom implementation
```

An extension declares typed input/output, permissions, side effects, authentication requirements, timeout, retry behavior, idempotency expectations, transaction semantics, and failure behavior. Start with one portable out-of-process HTTP contract. WASM, Go services, or function providers are later options only if concrete vertical slices justify them.

Exit criteria:

- A reference application uses an extension without weakening Action transaction, authorization, audit, or idempotency contracts.
- Extension unavailability and retry behavior are deterministic and tested.
- Semantic Test Suites can replace typed provider calls with mocks and assert their interactions without contacting real infrastructure.
- Arbitrary inline JavaScript, SQL, and React remain unsupported.

### Phase 9 — v1.0: production qualification

Qualify Bean for one deliberately narrow production envelope only after the agent-to-demo loop is strong:

> Bean v1 supports a single Bean application process backed by managed PostgreSQL and external object storage.

Qualify backup/restore, secrets, migration and upgrade behavior, observability, security, rate limiting, release compatibility, and a declared load envelope within that topology. SQLite remains the local/demo strength. Other process, database, storage, and orchestration topologies are outside the v1.0 contract.

The phase must publish explicit supported topologies, failure models, SLO evidence, restore drills, compatibility windows, and known exclusions. Bean consumes managed databases, storage, identity, and orchestration; it does not become a Kubernetes platform, distributed database, Kafka clone, S3 clone, or identity provider.

### Phase 10 — Bean Cloud

Only after the local engine and production contract are proven, provide the narrow hosted loop:

```text
git or prompt -> Bean build -> preview environment -> shareable URL
```

The minimum platform supplies PostgreSQL, object storage, secrets, domains, logs, deployments, and expiring previews. It does not expand into a general backend-as-a-service or functions platform.

### Phase 11 — application-pattern ecosystem

Once the definition and extension compatibility contracts are stable, allow agents to discover and compose versioned packages such as approval workflows, kanban, comments, tenant ownership, and activity feeds.

Packages should prefer small patterns over whole opaque applications. They declare Bean compatibility, capabilities, dependencies, and test evidence; installation remains inspectable and compiles into ordinary definitions/AppIR.

## Explicitly deferred

During v0.6-v0.8, reject the following unless a vertical slice proves a required generic primitive:

- custom realtime infrastructure
- an Appwrite-style functions platform
- a broad OAuth/provider matrix
- Redis or messaging abstractions
- an email platform
- an S3 implementation
- a sophisticated arbitrary visual designer
- Kubernetes deployment machinery
- an AI chat UI embedded in Bean

Infrastructure complexity alone is not product progress. Bean owns Lifecycle, Policy, Action, View, Invariant, Page, and workflow semantics. Providers own capabilities such as SMTP, OAuth, S3, Kafka, Redis, LLMs, and container orchestration. Prefer an adapter or provider whenever a feature does not expand what an agent can describe.

## Decision checkpoints

At each release boundary, answer these questions before widening scope:

1. Did the release reduce agent ambiguity or increase the applications expressible in definitions?
2. Can the result be inspected, validated, diffed, and tested deterministically?
3. Did any application-specific behavior leak into core?
4. Does the North Star benchmark improve on elapsed time, validation attempts, human interventions, behavior score, or presentation score?
5. Is the next proposed primitive supported by a real vertical slice rather than infrastructure ambition?
6. Is proposed behavior structural semantics, a local deterministic rule, or an external effect?
