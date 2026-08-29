# Bean engineering contract

- `docs/goals/002.md` is the completed v0.2 product and acceptance-test record; new work must define a root `GOAL.md`.
- Keep `PLANS.md` and `docs/progress.md` current as milestones move.
- Preserve the definition -> validation -> migration -> immutable AppIR -> atomic activation lifecycle.
- Keep application behavior in metadata under `examples/`; core packages must remain generic.
- Confine SQL and SQLite dependencies to `internal/dbal/sqlite` and `internal/migration`.
- Route reads through Views and writes through Actions.
- Prefer focused, additive changes; run the nearest test after each milestone.
- Completion requires `make check` and `make build`.
