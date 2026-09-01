# Testing

`make test` runs Go and React unit tests. `make test-integration` exercises SQLite release, Action concurrency, and tenant isolation. `make test-e2e` builds the embedded binary and runs sequential Chromium workflows with isolated temporary databases. Playwright keeps traces, screenshots, and video on failure.

`make test-blackbox` builds the real binary and runs the v0.6 JSON-only structured repair/publication benchmark plus human-output compatibility checks for every agent command. Agent Protocol tests cover all ten shared handlers, exact CLI/MCP structured-result parity including Lifecycle inspect results, authorization for each plane in each transport, current and legacy MCP framing, malformed requests, EOF, stdout cleanliness, and View/Action owner/tenant boundaries. `make test-postgres` repeats the Application Plane and Lifecycle transition boundary on PostgreSQL 17.

`make check` adds formatting, vet, lint, TypeScript, race detection, frontend build, the black-box agent contract, and every browser suite. `asana.spec.ts` covers the legacy Action-local board graph plus tree/file slice; `ats.spec.ts` covers Lifecycle-backed candidate movement and the populated Demo Factory vocabulary; `commerce.spec.ts` covers Lifecycle-backed order transitions; `package.spec.ts` starts the copied executable after deleting its source. `make build` produces `bin/bean`.
