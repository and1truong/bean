# Bean v0.7 Demo Factory plan (completed)

Status values: `pending`, `active`, `done`. A milestone is `done` only when its listed evidence passes.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze Demo Factory source, pattern, seed, theme, package, and benchmark contracts | root goal and frozen ATS/CRM/tracker protocol | done |
| 1 | Typed Theme plus generic Metric, Timeline, and public Search presentation | schema/capability/compiler, HTTP, React, and ATS browser tests | done |
| 2 | Deterministic relation-aware fixture generator and `bean demo seed` | scalar/relation/cycle/replay/refusal tests and populated ATS evidence | done |
| 3 | Inspectable catalog of ordinary-definition application patterns | catalog stability tests and independent compilation of every pattern | done |
| 4 | Atomic, checksummed SQLite `bean package` output | restart, source-independence, tamper, failure-atomicity, and packaged-browser tests | done |
| 5 | ATS/CRM/tracker prompt-suite qualification, documentation, and version cut | terminal gates plus documented benchmark qualification boundary | done |

## Working rules

- Patterns expose ordinary definitions and always pass through schema and compiler validation; they never become hidden runtime macros.
- Seed writes use Actions, verification reads use Views, and generated data never bypasses Policy or storage contracts.
- Theme values come from closed compiler-known vocabularies; do not accept CSS or arbitrary frontend tokens.
- Dashboard composes Page/Panel/Block; add only the missing Metric, Timeline, and Search presentation behavior.
- Package only the current executable plus an activated SQLite database and manifest; do not add cloud, container, or installer machinery.
- Treat JSON envelopes, diagnostics, ordering, checksums, seed output, and package manifests as machine contracts.
- Add failing evidence before each public contract and run the nearest test after every milestone.
- Keep `GOAL.md`, `ROADMAP.md`, and `docs/progress.md` current as milestones move.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
