# Community

A minimal social community definition focused on ownership and authorization. It models profiles, posts, comments, reactions, and follows while demonstrating owner-aware and public-record policies.

## Highlights

- Owner-scoped community records
- Private and public posts
- Member-only reaction policy
- Policy-preserving public feed View
- Public homepage with a paginated feed and an empty state
- Controlled post publication Action

## Run it

From the repository root:

```bash
./bin/bean init --db ./tmp/community.db --admin-email admin@example.test --admin-password test-password
./bin/bean app validate --file ./examples/community/app.yaml
./bin/bean app publish --file ./examples/community/app.yaml --db ./tmp/community.db --json
./bin/bean serve --db ./tmp/community.db --addr 127.0.0.1:8080
```

Open <http://127.0.0.1:8080/> for public posts or <http://127.0.0.1:8080/admin> to inspect and populate the application.

For a local demo with an initialized administrator, you can also run `./bin/bean demo --app community --db ./tmp/community.db --addr 127.0.0.1:8080`.

To exercise ownership through Admin, create a local account with both `member` (Community writes) and `editor` (Admin access):

```bash
./bin/bean user create --db ./tmp/community.db --email member@example.test --password test-password --roles member,editor
```

Sign in as that member, create a private Post, then use **Actions → Publish post**, select `public`, and run the Action. Public posts appear on the homepage and in the JSON feed at `/api/feed`. The homepage uses the same public-only View for visitors and signed-in members; private posts remain in their owner's Admin list.
