# Goal: visual agentic browser testing

Status: in progress. Epic tracked as <https://github.com/and1truong/bean/issues/20> with sub-issues #23–#36; per-ticket PRs merge into `epic/browser-testing`, never directly to main.

Authors can declare browser-test Scenarios, execute them against a running application through a sandboxed Playwright sidecar, watch live step/event streams, and debug failures in Studio. Actions remain the only domain mutation boundary; Scenarios are the orchestration boundary.

## Architecture and scope

- `Scenario` is a first-class definition kind that compiles into `App.Scenarios` on immutable AppIR v21. It defines the orchestration graph only: node types `navigate`, `click`, `fill`, `select`, `press`, `wait`, `assert`, `extract`, `branch`, `loop`, `script`, `api_call`, `pause`.
- Execution is session-oriented: Run → Session → StepExecution → Artifact/Event, durable outside domain transactions (no DB transaction spans a browser run).
- The browser adapter is an isolated Playwright sidecar child process behind a Semantic Browser API; never embedded in the app process or the domain write path.
- The event stream is real-time (SSE/WS) with a persisted log; artifacts (screenshots, DOM snapshots, traces) attach to step executions.
- `api_call` nodes delegate writes to declared Actions. `fill` accepts `secret` references resolved outside AppIR at execution time.
- Studio surfaces: live run view, graph editor, pause/takeover/resume, NL→scenario authoring, exploration→saved test, failure diagnosis/repair, app-driven generation.
- The browser workspace is a separate security boundary: isolated storage, network egress policy, and no shared session with the authoring Studio user by default.
- Excluded: in-process browser embedding, cross-run DB transactions, domain writes outside Actions, production-readiness of the sidecar before the security boundary ships.

## Slices

See `PLANS.md`. Slice order follows the dependency spine: scenario contract (#23) → run model (#24) + semantic browser API (#25) → Playwright adapter (#26) → event stream (#27) → HTTP API (#28) → Studio surfaces (#30–#32) → agentic authoring (#33–#36), with the security boundary (#29) required before the adapter is production-ready.

Previous completed goal: extended semantic content and composition.
