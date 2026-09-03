# Bean Blog

A metadata-only public blog with editorial administration, taxonomy, member registration, and moderated comments.

## Definition layout

- `app.yaml` — application entry point
- `navigation.yaml` — public navigation
- `access.yaml` — roles, registration, and signup
- `taxonomy.yaml` — categories and tags
- `posts.yaml` — post publishing, public Views, and Pages
- `comments.yaml` — comment submission and moderation

## Run it

From the repository root:

```bash
./bin/bean app validate --file ./examples/blog/app.yaml
./bin/bean app publish --file ./examples/blog/app.yaml --db ./tmp/blog.db --json
./bin/bean serve --db ./tmp/blog.db --addr 127.0.0.1:8080
```

Open <http://127.0.0.1:8080/> for the public site or <http://127.0.0.1:8080/admin> for administration.
