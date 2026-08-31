# Testing

`make test` runs Go and React unit tests. `make test-integration` exercises SQLite release, Action concurrency, and tenant isolation. `make test-e2e` builds the embedded binary and runs sequential Chromium workflows with isolated temporary databases. Playwright keeps traces, screenshots, and video on failure.

`make check` adds formatting, vet, lint, TypeScript, race detection, frontend build, and every browser suite. `asana.spec.ts` is the anonymous local acceptance journey for board movement, three-level task nesting, multipart upload, and downloaded file integrity. `make build` produces `bin/bean`.
