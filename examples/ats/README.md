# Applicant Tracker

A populated recruiting application for exploring candidates by stage, job, department, and recent activity. It demonstrates lifecycle transitions, guarded Actions, semantic tests, search and drill-down Views, charts, metrics, and generated demo data.

## Highlights

- Jobs, candidates, notes, and activities
- Candidate pipeline lifecycle and movement Action
- Candidate search, board, charts, metrics, and detail page
- Deterministic `DemoSeed` data
- Action contract tests

## Run it

From the repository root:

```bash
./bin/bean app validate --file ./examples/ats/app.yaml
./bin/bean app publish --file ./examples/ats/app.yaml --db ./tmp/ats.db --json
./bin/bean demo seed --file ./examples/ats/app.yaml --db ./tmp/ats.db --seed 42 --json
./bin/bean serve --db ./tmp/ats.db --addr 127.0.0.1:8080
```

Open <http://127.0.0.1:8080/>. You can also run the semantic tests with:

```bash
./bin/bean app test --file ./examples/ats/app.yaml --json
```
