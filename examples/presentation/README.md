# Bean Introduction

A ten-frame introduction to Bean built entirely from semantic Sequence metadata. It demonstrates frame-local inline semantic Panel content, an explicitly named content Block, a live View-backed chart Block, closed presentation layouts, accessible browser navigation, and print structure.

## Definition layout

- `app.yaml` — application entry point
- `theme.yaml` — presentation theme
- `data.yaml` — capability data, View, and chart Block
- `content.yaml` — an intentionally named reusable semantic content Block
- `layout.yaml` — frame Panels with inline content and the ordered Sequence

## Run it

From the repository root:

```bash
./bin/bean demo --app presentation --db ./tmp/presentation.db --addr 127.0.0.1:8080
```

Open <http://127.0.0.1:8080/presentations/bean>.

To use the explicit publish flow instead:

```bash
./bin/bean app validate --file ./examples/presentation/app.yaml
./bin/bean app publish --file ./examples/presentation/app.yaml --db ./tmp/presentation.db --json
./bin/bean demo seed --file ./examples/presentation/app.yaml --db ./tmp/presentation.db --seed 42 --json
./bin/bean serve --db ./tmp/presentation.db --addr 127.0.0.1:8080
```
