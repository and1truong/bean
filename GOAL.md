# Goal: split the Booking example source

Status: complete

Split `examples/booking/app.yaml` into an explicit manifest and feature-oriented YAML resources without changing the Booking application's compiled behavior, validation, migration, immutable AppIR, or activation lifecycle.

## Contract

- Keep `app.yaml` as the `Booking` manifest and list every resource explicitly.
- Group resource model, booking workflow, and calendar presentation definitions into reviewable local files.
- Preserve all definition content and the View-read/Action-write boundary.
- Document the source layout in the example README.
- Verify with Booking validation and semantic tests, then `make check` and `make build`.

## Evidence

- The pre-split and split source sets compile to the identical checksum: `74c8773ce57034f09c4cd0b3f20a504ca596153e26c7dc6410531ecf7c796d8b`.
- `./bin/bean app validate --file ./examples/booking/app.yaml --json`, `./bin/bean app test --file ./examples/booking/app.yaml --json`, `make check`, and `make build` pass.
