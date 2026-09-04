# Goal: Bean Design System v2

Status: complete

Redesign Bean as a coherent, compact professional tool for building, inspecting, querying, visualizing, and managing structured information.

## Contract

- Preserve definition → validation → migration → immutable AppIR → atomic activation, metadata-driven behavior, View reads, and Action writes.
- Keep React 19, Vite, Tailwind v4, source-owned primitives, React Query, and the existing routing/state architecture.
- Independently author all implementation under Bean's MIT license. Drupal, Metabase, Linear, GitHub, VS Code, and Grafana are references only; copy no source, styles, components, assets, or packages.
- Establish semantic light/dark tokens, compact type/spacing/control scales, border-led hierarchy, restrained radius, and overlay-only elevation.
- Replace rediscovered navigation with a stable application/workspace/module hierarchy for authenticated tools.
- Prove the system in Studio, Admin data tables, Explore analytics, and generated record forms without changing application logic.
- Target WCAG 2.2 AA where practical, desktop productivity first, narrow-laptop resilience, and graceful small-screen fallback.

## Evidence

- A repository-grounded audit and `docs/design-system.md` document decisions and remaining migration work.
- Focused React tests cover shell/theme, fields, table/filter behavior, and Studio structure.
- Existing browser journeys verify representative responsive screens and visual captures where supported.
- Completion requires `make check` and `make build`.
