# Goal: migrate the frontend class merger to `cn`

Status: complete

Replace Bean's direct `clsx` and `tailwind-merge` dependencies with the Tailwind CSS v4-compatible `cn` package while preserving the shared application-facing `cn()` API and existing component behavior.

## Contract

- Keep the migration small: retain `web/src/lib/utils.ts` and all existing component call sites.
- Keep `class-variance-authority`; do not alter component APIs, Tailwind configuration, or visual design.
- Cover conditional inputs and representative Tailwind conflicts, variants, arbitrary values, dark mode, important modifiers, arbitrary variants, and CVA output with semantic regression tests.
- Evaluate `cn build` and bundle impact without adding generated artifacts or CI complexity unless it clearly benefits Bean.
- Record a lightweight before/after benchmark as supporting evidence only.

## Evidence

- Frontend tests, lint, typecheck, build, and available UI tests pass.
- Repository searches show no direct `clsx`, `tailwind-merge`, or `twMerge` use remains outside unavoidable transitive dependencies.
- Completion requires `make check` and `make build`.
