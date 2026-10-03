# Goal: browser WASM playground

Status: in progress. Tracked as <https://github.com/and1truong/bean/issues/66>.

A static browser page can compile and preview real Bean definitions with zero backend: the same Go compiler runs as WebAssembly, and the same semantic-content renderers preview the result. Mèo and humans can paste or import sources, see diagnostics and the real render tree, and share a link — nothing is simulated.

## Architecture and scope

- `cmd/beanwasm` builds `GOOS=js GOARCH=wasm` and exposes one versioned JS global (`beanPlayground`) backed by `internal/playground`: a bounded virtual file map (≤64 files, ≤256 KiB/file, ≤1 MiB total, YAML/JSON only, no traversal), `compile` + `render` ops, stable `BEAN-P41xx/P42xx` error codes, and a session that keeps the last clean compile.
- Compilation reuses `definition.LoadFS → compiler.Compile` unchanged; rendering reuses `page/sequence` `Node` trees, with backend-dependent blocks (View/Entity/ResourceList reads, Webform/Action writes, owner-scoped Menu) rewritten to explicit `UnsupportedBlock` notices — never simulated.
- The worker layer (`web/public-playground/worker.js` + `web/src/playground/bridge.ts`) provides request identity, stale-result rejection, a 60 s call bound, and a fatal/recovery path; the UI keeps the last valid preview on failed edits.
- `web/src/registry.tsx` extracts the render registry so the playground and the app share `PageNode`/`StructuralNode`/menu chrome; missing components render an explicit alert.
- `make playground` emits a reproducible static bundle (`index.html` + hashed assets + `worker.js` + `bean.wasm` + matching `wasm_exec.js` + `examples/*.json`) servable from any host or subdirectory via `base: './'` and hash routing.
- Draft state is memory-only and documented as such; import/export via `.zip`/`.json` is the persistence boundary.
- Excluded: persistence of drafts, data-backed Views/Actions/Webforms, auth, jobs, Extensions, uploads, Studio/Explore backend, scenario execution.

## Slices

See `PLANS.md` and [`docs/playground.md`](docs/playground.md).

Previous completed goal: visual agentic browser testing (epic <https://github.com/and1truong/bean/issues/20>, slices #23–#36 shipped to `epic/browser-testing`).
