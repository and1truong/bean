# Testing

`make test` runs Go and React unit tests. `make test-integration` exercises SQLite release, Action concurrency, and tenant isolation. `make test-e2e` builds the embedded binary and runs sequential Chromium workflows with isolated temporary databases. Playwright keeps traces, screenshots, and video on failure.

`make test-blackbox` builds the real binary and runs the v0.6 JSON-only structured repair/publication benchmark plus human-output compatibility checks for every agent command. `make check` adds formatting, vet, lint, TypeScript, race detection, frontend build, the black-box agent contract, and every browser suite. `asana.spec.ts` covers the anonymous board/tree/file slice; `ats.spec.ts` covers the populated Demo Factory vocabulary; `package.spec.ts` starts the copied executable after deleting its source. `make build` produces `bin/bean`.
