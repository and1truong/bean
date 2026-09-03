# CRM

A sales CRM definition with companies, contacts, deals, and activities. It demonstrates role- and owner-aware policies, deal transitions, operational Views, charts, metrics, theming, and generated demo data.

## Highlights

- Salesperson and manager roles
- Owner-scoped contacts and deals
- Searchable deal table and pipeline View
- Pipeline value and stage summaries
- Deterministic `DemoSeed` data

## Run it

From the repository root:

```bash
./bin/bean app validate --file ./examples/crm/app.yaml
./bin/bean app publish --file ./examples/crm/app.yaml --db ./tmp/crm.db --json
./bin/bean demo seed --file ./examples/crm/app.yaml --db ./tmp/crm.db --seed 42 --json
./bin/bean serve --db ./tmp/crm.db --addr 127.0.0.1:8080
```

Open <http://127.0.0.1:8080/explore> to explore CRM entities or <http://127.0.0.1:8080/admin> to administer them.
