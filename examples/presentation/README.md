# Bean Introduction

A seventeen-frame, eight-chapter introduction built entirely from Bean metadata. It preserves the original frame IDs and `/presentations/bean`, and adds focused frames for heading levels, ordered lists, safe links, dividers, literal tables, image/audio/YouTube/playlist media, horizontal and vertical Tabs Blocks, and direct/tab-contained choices. The original inline composition, named `product_statement`, speaker notes, and live View-backed capability chart remain.

## Definition layout

- `app.yaml` — application entry point
- `theme.yaml` — presentation theme
- `data.yaml` — deterministic capability data, View, and chart Block
- `content.yaml` — named content and Tabs Blocks
- `layout.yaml` — frame Panels, inline content/choices, and the ordered Sequence
- `web/public/assets/bean-release-flow.png` and `bean-intro.wav` — static demo assets served by the existing frontend asset path

## Run it

From the repository root:

```bash
./bin/bean demo --app presentation --db ./tmp/presentation.db --addr 127.0.0.1:8080
```

Open <http://127.0.0.1:8080/presentations/bean>.

To use the explicit publish flow:

```bash
./bin/bean app validate --file ./examples/presentation/app.yaml
./bin/bean app publish --file ./examples/presentation/app.yaml --db ./tmp/presentation.db --json
./bin/bean demo seed --file ./examples/presentation/app.yaml --db ./tmp/presentation.db --seed 42 --json
./bin/bean serve --db ./tmp/presentation.db --addr 127.0.0.1:8080
```

## Interaction and media

Outside controls, Left/Right changes chapter, Up/Down changes depth, Page Up/Page Down follows source order, and Home/End moves to the first/last frame. Horizontal tabs use Left/Right; vertical tabs use Up/Down; both use Home/End and wrap. Tab panels enter normal Tab order. Choices use native radio keyboard behavior, then **Check answer** and **Try again**.

Audio and YouTube start as transcript-first placeholders. Select **Load audio**, **Load YouTube video**, or **Load YouTube playlist** to create the external media element. Leaving the active frame or tab unloads it; returning requires Load again. YouTube uses `youtube-nocookie.com`, but loading still contacts the provider and is not a no-tracking guarantee. Captions are requested when the provider has them; the literal transcript is the always-available text alternative and Bean does not verify its accuracy. External images likewise expose normal browser connection information to their host.

Print keeps every frame and all tab content in source order, shows media title/transcript/fallback links, and omits audio controls and YouTube iframes.
