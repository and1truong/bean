# Goal: Bean v0.7 Demo Factory

Turn the v0.6 machine-facing compiler loop into a fast path from ordinary application intent to a populated, presentable, portable local demo without adding an LLM or application-specific runtime code.

## Primary outcome

A coding agent can compose ordinary Bean definitions for a maintained prompt, validate and publish them through the v0.6 contract, populate coherent demo data deterministically, and produce a self-contained local package that opens as a credible working product rather than an empty CRUD shell.

```text
prompt
  -> agent
  -> ordinary Bean definitions + inspectable patterns
  -> validate / plan / publish
  -> deterministic seed
  -> themed working demo
  -> portable package
```

Bean remains deterministic and provider-neutral. The benchmark records the external agent and model as inputs; Bean does not call or embed either one.

## Demo vocabulary

v0.7 qualifies the deliberately bounded application vocabulary from the roadmap:

- Entity and Relation for business structure
- List, Detail, Form, Board, and Tree for operational interaction
- Dashboard, Metric, and Timeline for overview and activity
- Search for navigation within declared View fields
- Action for mutation and enum transitions for workflows
- Attachment for bounded local files

Dashboard remains composition of ordinary Page, Panel, and Block definitions. Metric and Timeline are compiler-validated View presentations, not separate query or storage systems. Search remains a View read and cannot bypass View or Policy constraints.

## Typed demo metadata

Theme and seed configuration are typed, schema-visible, inspectable Bean definitions.

- One application Theme selects a display name, a maintained preset, and an accent token. It does not accept CSS, JavaScript, arbitrary class names, URLs, or file paths.
- Demo seed metadata declares an explicit count per Entity and an optional maintained data profile. It cannot contain executable generators or bypass Actions.
- The same source and seed value produce the same generated field values, relation choices, and insertion order.
- Generated records are created through compiled create Actions. Reads used for verification go through Views.
- Required relations are dependency-ordered. Cyclic required relations are rejected with a stable diagnostic rather than partially populated.
- Generated values satisfy field types, requiredness, enum options, uniqueness, and declared bounds. Sensitive/password/file fields are never synthesized.
- Seeding is safe by default: a non-empty target is rejected unless it already contains the exact idempotent generated dataset.

The supported command is:

```bash
bean demo seed --file ./app.yaml --db ./demo.db --seed 42 --json
```

## Inspectable patterns

Ship a maintained catalog for CRUD resource, workflow resource, approval workflow, parent/child resource, many-to-many tagging, ownership, assignment, comments, activity history, and dashboard composition.

Patterns are valid, ordinary Bean definition bundles. `bean pattern inspect` returns their source definitions and required capabilities; it does not install hidden runtime behavior, introduce another DSL, or skip normal schema/compiler validation. Agents may copy, rename, and compose the definitions using the same v0.6 validate/inspect/diff loop.

## Portable package

The supported delivery command is:

```bash
bean package --file ./app.yaml --output ./dist/acme-demo --seed 42 --json
```

For v0.7, a package is a directory containing the Bean executable, an activated SQLite database with deterministic demo data, and a machine-readable manifest with checksums, Bean version, source checksum, release identity, and start command. Packaging is local and reproducible; it does not download dependencies, create a container, or publish a URL.

The packaged executable and database must start without the original source tree. Package creation uses a temporary staging directory and only replaces the destination after all validation, publication, seeding, checksum, and restart checks pass.

## Acceptance scenario

The primary benchmark prompt is a lightweight applicant tracking system with jobs, candidates, interview stages, notes, a kanban pipeline, candidate detail, search, dashboard metrics, and activity timeline.

1. The agent discovers capabilities and relevant patterns.
2. It produces only ordinary Bean definitions plus typed Theme and DemoSeed definitions.
3. The v0.6 JSON loop validates, repairs, plans, and publishes the candidate.
4. `bean demo seed` creates coherent jobs, candidates, notes, and activities deterministically through Actions.
5. The application opens with meaningful records, working pipeline transitions, search, candidate detail, dashboard metrics, timeline, and the declared theme.
6. `bean package` creates a portable local directory and its restart smoke test passes.
7. Repeating the run with the same inputs produces the same semantic definitions and seed dataset checksums.

CRM and issue-tracker prompts provide two independent secondary cases for pattern reuse and presentation coverage.

## Measurable acceptance criteria

- Theme, DemoSeed, Metric, Timeline, and Search contracts are represented in canonical schemas, capabilities, inspection, semantic diff, and compiler diagnostics.
- All maintained examples continue to compile; the applicant-tracking reference application contains no application-specific core branch.
- Every catalog pattern compiles independently and its returned definitions are byte-stable and visible to `app inspect` after composition.
- Deterministic seed tests cover all supported scalar types, uniqueness, optional and required relations, stable ordering, idempotent replay, unsafe-target refusal, and unsupported cycles/sensitive/file inputs.
- SQLite and PostgreSQL retain compile/runtime parity; v0.7 packaging itself intentionally targets SQLite only.
- Package tests verify atomic destination replacement, checksums, no dependency on the source directory, activated release loading, populated View reads, and executable startup.
- A recorded benchmark report includes prompt, agent/model/tool versions, budgets, elapsed time, validation attempts, human interventions, behavior score, presentation score, and package checksum.
- Maintained benchmark runs meet p50 under five minutes, p90 under ten minutes, zero human edits after the prompt, and the complete declared behavior rubric. Results are not compared across changed models, prompts, budgets, or cache state without disclosure.
- `make check`, `make test-crash`, `make test-postgres`, and `make build` pass.

## Explicit non-goals

- Embedding an LLM, agent loop, provider SDK, MCP server, or AI chat UI
- `bean share`, hosting, preview URLs, domains, cloud databases, or Bean Cloud
- First-class Lifecycle semantics; v0.7 workflows continue to compose enum fields, Actions, and transitions
- Deterministic rule expressions, generated semantic tests, or typed external extensions
- Arbitrary CSS, JavaScript, templates, SQL, scripts, or executable seed hooks
- Realtime, calendar, chat, email, OAuth expansion, Redis, object-storage infrastructure, containers, or Kubernetes
- A general package registry or post-v1.0 package ecosystem

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
