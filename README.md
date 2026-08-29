# Bean

Bean v0.1 is a metadata-driven web application runtime. YAML or Studio definitions compile into an immutable in-memory application, native SQLite tables, REST/OpenAPI operations, generated administration, and public React pages. The frontend and pure-Go SQLite driver ship in one executable.

```text
Definitions -> validation -> additive migration -> immutable AppIR -> atomic activation
HTTP -> policy -> View reads / Action writes -> typed DBAL -> SQLite
Page -> Panel -> Block -> typed render tree -> embedded React registry
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
./bin/bean app import --db ./bean.db --file ./examples/cms/app.yaml
./bin/bean publish --db ./bean.db
./bin/bean serve --db ./bean.db --addr 127.0.0.1:8080
```

Or run `./bin/bean demo --app cms --db ./tmp/cms.db --addr 127.0.0.1:8080`. Open `/`, `/admin`, `/studio`, `/docs`, or `/openapi.json`. Studio uses structured envelope fields and an advanced JSON specification editor; validate before publishing.

Core modules live under `internal/`: compilation/release, DBAL/SQLite/migrations, entity/field/View/Action, auth/policy, webform, render/page composition, OpenAPI/HTTP, audit/events/jobs, and embedded UI assets. Seven metadata-only examples live under `examples/`.

Known v0.1 limits match the product non-goals: SQLite and one active application per database, one process, local authentication, additive migrations only, deterministic fake integrations, REST only, and no visual drag-and-drop builder.
