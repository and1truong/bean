# Bean

Bean v0.4 alpha is a compiled application runtime for developer-built operational web applications. YAML or typed visual Studio definitions compile into a versioned immutable application, native SQLite or PostgreSQL tables, REST/OpenAPI operations, a metadata-driven administration console, and public React pages. The frontend and SQLite driver ship in one executable.

```text
Definitions -> validation -> additive migration -> immutable AppIR -> atomic activation
HTTP -> policy -> View reads / Action writes -> typed DBAL -> SQLite/PostgreSQL
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

PostgreSQL uses the same commands with `--database-url` or `BEAN_DATABASE_URL`:

```bash
./bin/bean init --database-url 'postgres://bean:secret@db/bean?sslmode=require' --admin-email admin@example.test --admin-password change-this-password
./bin/bean serve --database-url 'postgres://bean:secret@db/bean?sslmode=require' --addr 127.0.0.1:8080
```

Or run `./bin/bean demo --app cms --db ./tmp/cms.db --addr 127.0.0.1:8080`. Open `/`, `/admin`, `/admin/system`, `/studio`, `/docs`, or `/openapi.json`. Admin provides application resources plus protected user, release, migration, job, outbox, and audit operations. Studio provides typed visual editors for the core definitions and a lossless advanced JSON escape hatch; validate and review the migration preview before publishing.

Core modules live under `internal/`: compilation/release, DBAL/SQLite/PostgreSQL/migrations, entity/field/View/Action, auth/policy, webform, render/page composition, OpenAPI/HTTP, audit/events/jobs, and embedded UI assets. Seven metadata-only examples live under `examples/`.

Bean is still alpha, not a blanket production-readiness claim. v0.4 qualifies single-process crash/restart on a functioning SQLite filesystem or PostgreSQL service, but excludes host loss, corruption, HA, multi-process writers, destructive migrations, backup/restore, external security certification, and SLO/load claims. Outbox delivery is at-least-once and consumers must be idempotent. The exact accepted surface and evidence are listed in `docs/capabilities.md`.
