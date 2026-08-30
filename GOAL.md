# Goal: Bean shadcn/ui system

Standardize every Bean-owned frontend surface on source-owned components from https://ui.shadcn.com/ without changing application semantics, metadata contracts, authorization, or the View-read/Action-write boundaries.

## Acceptance criteria

- Shell/Auth, public Page and Block rendering, Application Admin, System Admin, and Studio use checked-in shadcn/ui primitives and a shared Bean theme.
- Generated forms retain native labels, validation, single-select, and multi-select behavior; existing stable browser selectors remain valid.
- Admin tables retain server-driven search, allowlisted filters, sorting, cursor pagination, selection, and Action execution.
- Destructive and confirmed operations use accessible AlertDialog flows rather than browser or custom CSS confirmation dialogs.
- Interactive controls and tables outside `web/src/components/ui` cannot regress to raw elements.
- Primary surfaces remain usable without horizontal page overflow on a mobile viewport.
- No application-specific rendering branches, new metadata theme system, or backend API changes are introduced.
- Tailwind and shadcn remain build-time/source-owned frontend concerns; optimized assets still ship in the single Bean executable.

## Terminal gates

```bash
make check
make test-blog
make test-postgres
make build
```
