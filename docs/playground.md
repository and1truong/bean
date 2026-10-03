# Browser playground

The playground runs the real Bean compiler and the real semantic-content renderer inside a browser tab — no Bean backend, no `/api` calls. A static bundle under `dist/playground/` is served by any file server; the Go compiler executes as a WebAssembly module in a Web Worker.

```bash
make playground                     # builds dist/playground/
python3 -m http.server -d dist/playground 8080
# open http://localhost:8080/
```

Serving over HTTP is required: `file://` pages cannot fetch the WASM module. The bundle is subdirectory-safe (`base: './'` + hash routing), so `http://host/some/path/` works unchanged. The WASM response needs a `application/wasm` MIME type for streamed compilation — `python3 -m http.server` and common static hosts already send it; without it the worker falls back to buffered instantiation, which is slower but still works.

## Layout

```
dist/playground/
  index.html            entry (built from web/playground.html)
  assets/               JS/CSS + Geist fonts + deferred diagram chunk
  worker.js             Web Worker: loads wasm_exec.js + bean.wasm, dispatches calls
  wasm_exec.js          Go runtime support, copied from $(go env GOROOT)/lib/wasm
  bean.wasm             cmd/beanwasm — the Go compiler bridge (~10 MB)
  examples/index.json   manifest of checked-in examples
  examples/<name>.json  each example's files + manifest path
```

## What works

- Edit or paste multi-document YAML/JSON sources across file tabs, or load a checked-in example, or import a `.zip`/`.json` bundle/multi-select of files, then **Compile & preview**.
- The preview is the same React render tree the Bean server produces: literal semantic content, Content/Tabs/Lesson/Timeline/Mindmap/Flashcard Blocks, Pages, and interactive Sequences (frame navigation, speaker notes, print).
- Route selection mirrors the app's real routes; the browser URL hash holds the current route, so links can be shared within a session and back/forward works.
- Diagnostics surface with their stable `BEAN-*` codes, source file and line; clicking one opens that file. A failing compile keeps the last valid preview.
- **Export .zip / .json** downloads the current sources.
- Draft state lives in memory only — reloading resets to the starter draft (the UI says so below the editor). Import/export is the intended way to keep work.

## What is explicitly unavailable

Components that need a Bean backend are never simulated; the render tree marks them as *backend-dependent* (notice chips above the preview, plus an inline notice where the block would render):

- `View`, `Entity`, and `ResourceList` blocks — need a backend for data.
- `Webform` and `Action` blocks — need a backend for writes.
- Owner-scoped `Menu` blocks — workspace menus resolve dynamically; static menus still render.

Routes covered by authentication/policy are listed as *needs sign-in* in the route selector rather than executed.

## Bridge protocol

`beanPlayground(requestJSON) → responseJSON`, exposed by `bean.wasm`. Version `v: 1`.

- `{"v":1,"op":"compile","files":{path:content},"manifest":"app.yaml"}` → `{v,ok,diagnostics,app}` — `app` summarizes routes (page/sequence/display, title, protected, unsupported) and backend-dependent components. The session adopts a new app only when the compile has zero diagnostics, so a failed edit never breaks the preview.
- `{"v":1,"op":"render","path":"/route","query":"frame=x"}` → `{v,ok,tree,unsupported}` — the render tree with backend-dependent nodes rewritten to `UnsupportedBlock` entries `{name,component,reason}`.
- Errors return `{v,ok:false,error:{code,message}}` with codes `BEAN-P4100`–`P4103` (request/limits/path/manifest), `P4201` not found, `P4202` protected, `P4203` backend required, `P4204` missing render context, `P4205` internal, `P4206` no compiled app.

Source limits mirror a paste-friendly size: ≤64 files, ≤256 KiB per file, ≤1 MiB total, paths ≤256 bytes and ≤8 segments, `.yaml/.yml/.json` only, `..`/absolute paths rejected. The worker layer additionally bounds each call at 60 s, rejects stale responses by request id, and reports a fatal banner with a *Restart compiler* action if the WASM module fails to load.

The Go bridge has no filesystem, network, or database access — the virtual file map is its only input. The same `definition.LoadFS → compiler.Compile → render.Node` path serves `bean serve`, so diagnostics and trees match the backend (a Go test asserts byte parity against native composition).

## Bundle size and compile behavior

`bean.wasm` is ~10 MB (release flags: `-trimpath -ldflags "-s -w"`). It downloads once per tab; typical static-host caching keeps revisits fast. Inside the worker the Go runtime starts in ~1–3 s cold, then each compile of a small example completes in tens of milliseconds (measured ~40–70 ms for the starter and the presentation example). The deferred mermaid chunk (~5 MB) loads only when a diagram or mind map block renders.

## Agent interface (WebMCP + `window.bean`)

The page is an agent surface: a browser agent can compose a Bean app on it without any backend.

- **WebMCP** — when `document.modelContext` exists (the WICG draft; `navigator.modelContext` is the deprecated spell), the page registers six tools: `bean_state`, `bean_compile`, `bean_render`, `bean_navigate`, `bean_list_examples`, `bean_load_example`. Each carries a JSON input schema and descriptions; `bean_compile` takes `{files, manifest}` and returns `{ok, diagnostics, app}`.
- **`window.bean`** — always present: `bean.v` (1), `bean.webmcp` (registration succeeded), `bean.tools` (names), plus both the `bean_*` tool names and camelCase methods (`bean.compile(...)`, `bean.state()`, ...).

`/llms.txt` on the same host documents the tool contract, source limits, and error codes for agents that discover the site. A Playwright journey drives the full agent loop (`bean_compile` with new sources → `bean_navigate` → `bean_render` → `bean_state`) against the real WASM build.

## GitHub Pages

`.github/workflows/pages.yml` builds the same `make playground` bundle and deploys it to GitHub Pages on every push to `main` (or manually via *Actions → pages → Run workflow*). One-time repo setup: **Settings → Pages → Source: GitHub Actions**. The deployed URL is `https://<owner>.github.io/<repo>/` — a subdirectory, which the bundle handles via `base: './'` and hash routing.

## Tests

- `go test ./internal/playground` — compile/render round-trips, diagnostics and tree parity with native composition, file-map limits, render failures, last-valid-preview semantics.
- `cd web && bunx vitest run src/playground/zip.test.ts` — zip write/read round-trip including a deflated entry.
- `cd e2e && bunx playwright test playground.spec.ts` — real Chromium against the static dist: starter compile with zero `/api` requests, example loading, unsupported marking, broken-edit diagnostics + preview retention, hash-route back/forward, zip download, 390 px viewport. Runs under `make check` via `test-e2e`.
