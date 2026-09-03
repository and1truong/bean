# Bean engineering contract

- Preserve the definition -> validation -> migration -> immutable AppIR -> atomic activation lifecycle.
- Keep application behavior in metadata under `examples/`; core packages must remain generic.
- Confine SQL and SQLite dependencies to `internal/dbal/sqlite` and `internal/migration`.
- Route reads through Views and writes through Actions.
- Prefer focused, additive changes; run the nearest test after each milestone.
- Follow `docs/howto-pr.md` when creating or updating pull requests.
- Completion requires `make check` and `make build`.

## with goal driven work

- you must define a root `GOAL.md`.
- Keep `PLANS.md` and `docs/progress.md` current as milestones move.
