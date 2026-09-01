# Bean

Bean v0.12 alpha is a compiled application runtime designed for humans and AI agents to build operational web applications from high-level definitions. YAML or typed visual Studio definitions compile into a versioned immutable application, native SQLite or PostgreSQL tables, REST/OpenAPI operations, a metadata-driven administration console, and public React pages. First-class Lifecycle metadata gives the compiler and runtime one canonical state-machine model. Named typed Rules add bounded deterministic guards, derived Action inputs, and Entity invariants without adding a script runtime. Metadata TestSuites exercise Rules and Actions through their production paths in fresh SQLite state; generated replays, semantic negatives, CRUD smoke cases, route checks, and HTTP journeys add evidence for behavior Bean can prove from declarations. Actions remain the mutation boundary and Policies remain the authorization boundary. A provider-neutral Agent Protocol exposes the same compiler, release, View, and Action services through CLI or MCP stdio. The frontend and SQLite driver ship in one executable.

Bean's direction is to make probabilistic agents safe to build with through a deterministic compiler and runtime—not to become another general-purpose low-code platform. See [ROADMAP.md](ROADMAP.md) for the product sequence and [GOAL.md](GOAL.md) for the current v0.12 execution contract.

```text
Definitions -> validation -> additive migration -> immutable AppIR -> atomic activation
HTTP -> policy -> View reads / Action writes -> typed DBAL -> SQLite/PostgreSQL
Page -> Panel -> Block -> typed render tree -> embedded React registry
Agent -> CLI or MCP -> Definition / Release / Application Plane -> shared services
Rule -> typed bounded evaluator -> Action guard / derived input / Entity invariant
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

Or run `./bin/bean demo --app ats --db ./tmp/ats.db --addr 127.0.0.1:8080` for the populated applicant tracker; `blog` and `asana` remain available. Open `/`, `/admin`, `/admin/system`, `/studio`, `/docs`, or `/openapi.json`. Admin provides application resources plus protected user, release, migration, job, outbox, and audit operations. Studio provides typed visual editors for the core definitions and a lossless advanced JSON escape hatch; validate and review the migration preview before publishing.

Core modules live under `internal/`: compilation/release, deterministic Rule evaluation, DBAL/SQLite/PostgreSQL/migrations, field/View/Action, auth/policy, webform, render/page composition, demo seeding/patterns, OpenAPI/HTTP, audit/events/jobs, and embedded UI assets. Ten metadata-only examples live under `examples/`, including Asana Lite and the populated ATS Demo Factory slice.

Bean is still alpha, not a blanket production-readiness claim. v0.12 retains the qualified single-process crash/restart envelope on a functioning SQLite filesystem or PostgreSQL service, but excludes host loss, corruption, HA, multi-process writers, destructive migrations, backup/restore, external security certification, and SLO/load claims. Rules are deliberately side-effect-free and are not an extension or scripting platform. Semantic suites and generated checks are bounded local evidence, not proofs of unstated business intent, production release gates, or external-effect tests. MCP is a local stdio adapter, not a hosted authorization or deployment surface. Outbox delivery is at-least-once and consumers must be idempotent. The exact accepted surface and evidence are listed in `docs/capabilities.md`.
