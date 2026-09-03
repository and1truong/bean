# Issue Tracker

An issue-tracking application with projects, issues, and comments. It demonstrates filtered operational Views, cursor paging, multi-record selection, state transitions, board and chart renderers, drill-down, theming, and generated demo data.

## Highlights

- To-do, in-progress, and done workflow
- Filterable issue table and kanban board
- Ordered Page layout bands: single-column semantic introduction followed by two-column operations
- Opt-in Region collapse metadata so a policy-empty operational column can relinquish its track
- Issue-count chart with drill-down
- Project and assignee filtering
- Deterministic `DemoSeed` data

## Run it

From the repository root:

```bash
./bin/bean app validate --file ./examples/tracker/app.yaml
./bin/bean app publish --file ./examples/tracker/app.yaml --db ./tmp/tracker.db --json
./bin/bean demo seed --file ./examples/tracker/app.yaml --db ./tmp/tracker.db --seed 42 --json
./bin/bean serve --db ./tmp/tracker.db --addr 127.0.0.1:8080
```

Open <http://127.0.0.1:8080/> for the tracker dashboard.
