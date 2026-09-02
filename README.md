# Bean

Bean v0.16 alpha is a compiled application runtime designed for humans and AI agents to build operational web applications from high-level definitions. YAML, Explore, or typed visual Studio definitions compile into a versioned immutable application, native SQLite or PostgreSQL tables, REST/OpenAPI operations, a metadata-driven administration console, and public React pages. Views own Policy-preserving typed queries and named displays; Displays render records, groups, and metrics; Pages compose dashboards; Sequences compose ordered guided experiences from the same Panels and Blocks; Actions remain the mutation boundary. First-class Lifecycle metadata, deterministic Rules, semantic TestSuites, generated checks, and typed HTTP Extensions provide bounded application behavior without a script runtime. A provider-neutral Agent Protocol exposes the same compiler, release, View, and Action services through CLI or MCP stdio. The frontend and SQLite driver ship in one executable.

Bean's direction is to make probabilistic agents safe to build with through a deterministic compiler and runtime—not to become another general-purpose low-code or BI platform. Bean Explore turns an existing Entity model into an operational loop: explore, visualize, drill into exact records, and act through authorized Actions. Semantic Sequences add an ordered, inspectable experience without a parallel rendering runtime. See [ROADMAP.md](ROADMAP.md) for the product sequence and [GOAL.md](GOAL.md) for the current v0.16 execution contract.

```text
Definitions -> validation -> additive migration -> immutable AppIR -> atomic activation
HTTP -> policy -> View reads / Action writes -> typed DBAL -> SQLite/PostgreSQL
Page -> Panel -> Block -> typed render tree -> embedded React registry
Sequence -> ordered frame -> Panel -> bounded semantic content / View Display
View -> canonical query -> named page/block/serialized display
Explore -> candidate View -> preview -> Studio draft -> validate/diff/publish
Agent -> CLI or MCP -> Definition / Release / Application Plane -> shared services
Rule -> typed bounded evaluator -> Action guard / derived input / Entity invariant
Extension -> Action transaction -> durable intent -> bounded after-commit HTTP
```

## Prerequisites and build

Go 1.25 and Bun 1.4 are required at build time. Chromium is required only for browser tests.

```bash
make bootstrap
bunx --cwd e2e playwright install chromium
make check
make build
```

Runtime needs only `bin/bean` and one database:

```bash
./bin/bean init --db ./bean.db --admin-email admin@example.test --admin-password test-password
./bin/bean app validate --file ./examples/blog/app.yaml
./bin/bean app import --db ./bean.db --file ./examples/blog/app.yaml
./bin/bean publish --db ./bean.db
./bin/bean serve --db ./bean.db --addr 127.0.0.1:8080
```

The agent-native source loop is:

```bash
./bin/bean app init --dir ./my-app
./bin/bean capabilities --json
./bin/bean schema --output ./schemas
./bin/bean app validate --file ./my-app/app.yaml --json
./bin/bean app plan --file ./my-app/app.yaml --json
./bin/bean app publish --file ./my-app/app.yaml --db ./my-app/bean.db --json
./bin/bean app test --file ./my-app/app.yaml --json
```

Build a populated local demo from typed metadata, or inspect an ordinary-definition pattern before composing it:

```bash
./bin/bean pattern inspect workflow_resource --json
./bin/bean app publish --file ./examples/ats/app.yaml --db ./tmp/ats.db --json
./bin/bean demo seed --file ./examples/ats/app.yaml --db ./tmp/ats.db --seed 42 --json
./bin/bean package --file ./examples/ats/app.yaml --output ./dist/ats --seed 42 --json
./bin/bean package verify --dir ./dist/ats --json
./bin/bean demo --app presentation --db ./tmp/presentation.db --addr 127.0.0.1:8080
```

See [docs/agent-cli.md](docs/agent-cli.md) for the versioned envelope, diagnostic codes, exit statuses, inspection, semantic diff, and compatibility rules.

The provider-neutral protocol can also be called one-to-one or served to a local MCP host:

```bash
./bin/bean agent call bean.definition.validate --input ./request.json --json
./bin/bean mcp serve --allow-plane definition,release,application
```

See [docs/agent-protocol.md](docs/agent-protocol.md) and the shipped [agent guidance](agents/AGENTS.md) for operation schemas, plane authorization, host identity, and View/Action boundaries.

PostgreSQL uses the same commands with `--database-url` or `BEAN_DATABASE_URL`:

```bash
./bin/bean init --database-url 'postgres://bean:secret@db/bean?sslmode=require' --admin-email admin@example.test --admin-password change-this-password
./bin/bean serve --database-url 'postgres://bean:secret@db/bean?sslmode=require' --addr 127.0.0.1:8080
```

Run `./bin/bean demo --app presentation --db ./tmp/presentation.db --addr 127.0.0.1:8080` and open `/presentations/bean` for the ten-frame Bean introduction. The populated applicant tracker remains available with `--app ats`. Open `/explore` as an administrator to select any Entity, configure a record/group/metric View, preview it through the normal Policy path, and save it to the deterministic Studio draft. Open `/`, `/admin`, `/admin/system`, `/studio`, `/docs`, or `/openapi.json` for the other surfaces. Studio visually edits the common Explore query, Display, Page-filter, drill, and record-Action path, with Advanced JSON for uncommon combinations; validate and review semantic and migration changes before publishing.

Core modules live under `internal/`: compilation/release, deterministic Rule evaluation, DBAL/SQLite/PostgreSQL/migrations, field/View/Action, auth/policy, webform, render/page/sequence composition, demo seeding/patterns, OpenAPI/HTTP, audit/events/jobs, and embedded UI assets. Eleven metadata-only examples live under `examples/`, including Asana Lite, the populated ATS Explore slice, and the Bean introduction presentation.

Bean is still alpha, not a blanket production-readiness claim. v0.16 retains the qualified single-process crash/restart envelope on a functioning SQLite filesystem or PostgreSQL service, but excludes host loss, corruption, HA, multi-process writers, destructive migrations, backup/restore, external security certification, and SLO/load claims. Explore is not arbitrary SQL or a BI semantic layer. Sequences are not a PowerPoint clone: they use closed layouts and semantic content, accessible HTML navigation, and print structure, without freeform coordinates, executable markup, native PDF, or PPTX output. Bulk Actions are bounded, sequential, and non-atomic across records. Rules remain side-effect-free; Extensions are typed out-of-process HTTP calls, not a scripting or plugin platform. The exact accepted surface and evidence are listed in `docs/capabilities.md`.
