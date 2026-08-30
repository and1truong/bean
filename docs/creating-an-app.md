# Creating an application

Start with an Entity and fields, then expose reads through Views and writes through Actions. A Webform submits to an Action. Public screens are Pages whose Panel regions contain Blocks. Add Policies to all non-public reads and writes.

Import with `bean app import`, run `bean validate`, review diagnostics/migration output in Studio, and run `bean publish`. Machine names are permanent after publication; additive fields, indexes, relations, labels, and presentation changes are supported. Destructive or incompatible changes are rejected.

The Studio core path has typed editors for Entity, View, Action, Policy, and AdminResource. Reference controls are populated from the current draft. Advanced JSON edits the same canonical definition and preserves fields not represented by the visual slice.

Each Entity gets a default administration resource. Define an AdminResource when operators need curated columns, search fields, filters, form ordering, or domain Actions. Its reads still use a View and its writes still use Actions; see `docs/definitions.md` for the contract.

For user-authored Markdown, store the source in a textual Entity field, define a named `Filter` with a `markdown` step, and reference it from the public View through `fieldFilters`. Keep the Admin View unfiltered so editors read and update source Markdown rather than generated HTML.

Use a SQLite path for an embedded deployment or pass a PostgreSQL URL through `--database-url`/`BEAN_DATABASE_URL`. Definitions and AppIR are backend-neutral; do not put backend SQL in definitions.
