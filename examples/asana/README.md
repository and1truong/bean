# Asana Lite

A metadata-only project and task management application. It demonstrates nested tasks, status-based task movement, project dashboards, attachments, forms, and public pages.

## Definition layout

- `app.yaml` — application entry point
- `access.yaml` — access policy
- `projects.yaml` — projects, project Views, and project forms
- `tasks.yaml` — task Actions, Views, forms, and dashboard Blocks
- `attachments.yaml` — task attachments and upload form
- `pages.yaml` — navigation, Panels, and Pages

## Run it

From the repository root:

```bash
./bin/bean app validate --file ./examples/asana/app.yaml
./bin/bean app publish --file ./examples/asana/app.yaml --db ./tmp/asana.db --json
./bin/bean serve --db ./tmp/asana.db --addr 127.0.0.1:8080
```

Open <http://127.0.0.1:8080/>. Create data through the application forms or administration console.
