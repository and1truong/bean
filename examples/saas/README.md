# Multi-Tenant SaaS

A compact multi-tenant application definition. It demonstrates organisations, user memberships, tenant-scoped projects, tenant-aware policies, and a policy-preserving project API View.

## Highlights

- Organisation and membership model
- Member and owner tenant roles
- Tenant-scoped project records
- Tenant-aware read and write policy
- JSON display at `/api/projects`

## Run it

From the repository root:

```bash
./bin/bean app validate --file ./examples/saas/app.yaml
./bin/bean app publish --file ./examples/saas/app.yaml --db ./tmp/saas.db --json
./bin/bean serve --db ./tmp/saas.db --addr 127.0.0.1:8080
```

Use <http://127.0.0.1:8080/admin> to inspect the model. Requests to tenant-scoped data require an authenticated tenant context with an allowed role.
