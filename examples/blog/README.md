# Bean Blog

A metadata-only public blog with editorial administration, taxonomy, member registration, moderated comments, named View Displays, readable section widths, and Policy-aware responsive discussion layout.

## Definition layout

- `app.yaml` — application entry point
- `navigation.yaml` — public navigation
- `access.yaml` — roles, registration, and signup
- `taxonomy.yaml` — categories and tags
- `posts.yaml` — post publishing, public Views, readable Page sections, and responsive discussion composition
- `comments.yaml` — comment submission, named discussion Display, Policy-aware form content, and moderation

## Grouped record fields

The Post Admin form groups Content and Classification: Title/Slug and Author/Category pair at medium widths; Excerpt, Body, and Tags use full rows. Mobile keeps the same field order in one column. Layout is ordinary `AdminResource.form.layout` metadata and does not change draft creation or publication Actions.

`/posts/:slug/record` demonstrates the same bounded groups in a readonly detail Display over the existing publication-scoped `published_post` View. The original `/posts/:slug` article, taxonomy, and discussion remain unchanged. See [field layout](../../docs/field-layout.md) for authoring and deliberate limits.

## Run it

From the repository root:

```bash
./bin/bean app validate --file ./examples/blog/app.yaml
./bin/bean app publish --file ./examples/blog/app.yaml --db ./tmp/blog.db --json
./bin/bean serve --db ./tmp/blog.db --addr 127.0.0.1:8080
```

Open <http://127.0.0.1:8080/> for the public site or <http://127.0.0.1:8080/admin> for administration.
