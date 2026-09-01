# Creating an application

Start with an Entity and fields, then expose reads through Views and writes through Actions. A Webform submits to an Action. Public screens are Pages whose Panel regions contain Blocks. Add Policies to all non-public reads and writes.

Declare the application and its source files explicitly:

```yaml
# app.yaml
apiVersion: bean/v1alpha1
name: Example
resources:
  - access.yaml
  - projects.yaml
  - site.yaml
```

Resource files contain one or more flat definition documents separated by `---`. Group definitions by feature so a change to one workflow stays in one reviewable file. Small applications may put definitions directly after the manifest in `app.yaml`. Resources are local paths, are never discovered by glob, and cannot override another definition with the same kind and name.

Start a minimal workspace with `bean app init --dir ./my-app`. Inspect the exact compiler vocabulary with `bean capabilities` and `bean schema`, then run `bean app validate --file ./my-app/app.yaml`. Invalid YAML, unknown fields, missing resources, duplicate definitions, and compilation errors report stable codes plus source file, line, and column. `--json` exposes the machine contract documented in `docs/agent-cli.md`.

For a credible local demo, add one closed-vocabulary `Theme` and one `DemoSeed` definition, publish, then run `bean demo seed --file ./my-app/app.yaml --db ./my-app/bean.db --seed 42`. Use `bean pattern inspect NAME` to copy and adapt one of the maintained ordinary-definition patterns. `bean package --file ... --output ... --seed ...` produces a checked, source-independent local SQLite directory; it does not deploy or host the application.

Preview without mutation using `bean app plan --file ./my-app/app.yaml`; add `--db` or `--database-url` to plan against an initialized target. Use `bean app diff` for semantic AppIR changes. Publish the exact source set with `bean app publish --file ./my-app/app.yaml --db ./my-app/bean.db`, then run `bean app test --file ./my-app/app.yaml` for isolated compile/migrate/publish/restart smoke evidence. The older import plus draft `bean publish` workflow remains available for Studio. Machine names are permanent after publication; additive fields, indexes, relations, labels, and presentation changes are supported. Destructive or incompatible changes are rejected.

The Studio core path has typed editors for Entity, View, Action, Policy, and AdminResource. Reference controls are populated from the current draft. Advanced JSON edits the same canonical definition and preserves fields not represented by the visual slice.

Each Entity gets a default administration resource. Define an AdminResource when operators need curated columns, search fields, filters, form ordering, or domain Actions. Its reads still use a View and its writes still use Actions; see `docs/definitions.md` for the contract.

For user-authored Markdown, store the source in a textual Entity field, define a named `Filter` with a `markdown` step, and reference it from the public View through `fieldFilters`. Keep the Admin View unfiltered so editors read and update source Markdown rather than generated HTML.

Use a `file` field on a related attachment Entity when a record needs one or more uploads. A matching Webform `file` element sends multipart data; Bean limits each file to 5 MiB, stores its bytes and metadata in the Action transaction, and exposes downloads by generated identifier. Keep a separate human-readable label field because client filenames are never storage paths.

A View Block may use compiler-validated `board` or `tree` presentation. Boards group an enum field and invoke a declared transition Action. Trees consume a selected many-to-one self relation and build an expandable hierarchy from the bounded View result. See `examples/asana` for both patterns.

Use a SQLite path for an embedded deployment or pass a PostgreSQL URL through `--database-url`/`BEAN_DATABASE_URL`. Definitions and AppIR are backend-neutral; do not put backend SQL in definitions.
