# Community

A minimal social community definition focused on ownership and authorization. It models profiles, posts, comments, reactions, and follows while demonstrating owner-aware and public-record policies.

## Highlights

- Owner-scoped community records
- Private and public posts
- Member-only reaction policy
- Policy-preserving public feed View
- Controlled post publication Action

## Run it

From the repository root:

```bash
./bin/bean app validate --file ./examples/community/app.yaml
./bin/bean app publish --file ./examples/community/app.yaml --db ./tmp/community.db --json
./bin/bean serve --db ./tmp/community.db --addr 127.0.0.1:8080
```

Use <http://127.0.0.1:8080/admin> to inspect and populate the application.
