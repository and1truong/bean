# Team Workspace

A working multi-tenant project workspace built from Bean metadata. Two teams share the application while their workspace profiles, projects, charts, exports, and Actions remain isolated.

Owners and members plan projects, start work and mark it complete. Owners can also archive/reopen projects and update the workspace profile. Normal demo users do not need Admin access.

## Run the populated demo

From the repository root, with Python 3 available:

```bash
make build
python3 examples/saas/setup-demo.py --db ./tmp/saas-workspace.db
./bin/bean serve --db ./tmp/saas-workspace.db --addr 127.0.0.1:8084
```

Open <http://127.0.0.1:8084/>. Choose **Sign in**, then use one of these local demo accounts. All passwords are `test-password`.

| Workspace | Owner | Member | Sample projects |
| --- | --- | --- | --- |
| Northstar Studio | owner-a@example.test | member-a@example.test | 6 |
| Harbor Labs | owner-b@example.test | member-b@example.test | 3 |

The setup script refuses an existing database instead of overwriting it. To continue an existing demo, run only `bean serve`; to reproduce the initial dataset, choose a new database path. It provisions users with the CLI and creates/verifies business data through authenticated Actions and Views. The temporary setup server is stopped automatically.

The system account `admin@example.test` is reserved for system configuration. It has no tenant and cannot browse tenant-scoped lists. No admin account is needed for the user journeys below.

## A five-minute walkthrough

1. Sign in as `owner-a@example.test`. Overview shows **Northstar Studio**, six projects and their status distribution.
2. Filter by **active**. The metric changes to two. Click the active chart bar to open exactly those projects.
3. Return to Home using **Team Workspace** in the header. Open **New**, enter a name and description, and submit. The project starts in **planned**.
4. Open **Projects**, select its row, run **Start project**, then **Complete project**. Select projects from the appropriate stage; invalid lifecycle moves are rejected. Open the project name to read its details or rename it.
5. Return to Home to see the count and chart update. In **Manage**, an owner can archive the project and reopen it into planned. **Settings** lets the owner update the workspace name/description.
6. Sign in as `member-a@example.test`: the same workspace is available, but owner management and Settings links are absent and their Actions are denied.
7. Sign in as `owner-b@example.test`: Harbor has three different projects. Northstar records, totals, details and CSV rows are unavailable, even if their UUIDs are known.

The application header is the return path to Overview from standalone View pages. Search, filters, pagination and bulk Actions are ordinary View Displays.

## API and export

Authenticated requests use the same tenant policies as the UI:

- `/api/projects`: project JSON for the current tenant.
- `/projects.csv`: project CSV for the current tenant.
- `/api/views/project_total`: scoped count; `?status=active` narrows it.
- `/api/views/projects_by_status`: scoped counts grouped by status.

Changing query parameters cannot choose a different tenant. Tenant identity comes from the login session. Anonymous requests cannot read workspace profiles, projects, metrics or exports.

## Definition layout

- `app.yaml`: manifest.
- `access.yaml`: roles, explicit tenant policies, protected workspace profiles and owner Settings.
- `projects.yaml`: Project model, name validation, Lifecycle, role-specific Actions, record/aggregate Displays, forms and AdminResource.
- `workspace.yaml`: line-style workspace Menu, responsive Page sections, dashboard filters, drill and bound forms.
- `contracts.yaml`: executable owner/member/cross-tenant archive contract.
- `setup-demo.py`: fresh local database, four identities and two distinct datasets; no direct business-table SQL.

## Scope and upgrade notes

This is a provisioned-workspace demo, not self-service SaaS identity or billing. Accounts have one server-assigned tenant and roles. It does not implement invitations, changing a user's workspace, or granting access through a Membership record.

The previous example's Membership table did not control login permissions. The new source removes that misleading model and makes Organisation tenant-scoped. **Do not publish this source into an old `tmp/saas.db`.** Use a fresh demo database as above; destructive migration is intentionally not provided. Existing databases are untouched by setup.

The ordinary Admin surface remains available for tenant-scoped staff accounts if an operator explicitly provisions an `editor` role alongside the business role and tenant. System administrators are not a replacement for a tenant context.

## Verification

```bash
./bin/bean app validate --file examples/saas/app.yaml
./bin/bean app test --file examples/saas/app.yaml --json
cd e2e && bunx playwright test saas.spec.ts
```

Browser/API tests cover the seeded owner workflow, member permissions, filter/chart drill, profile changes, public-read denial, A/B record/Action/aggregate/export isolation, lifecycle bypass refusal, and light/dark/mobile presentation. `make check` and `make build` qualify the complete repository.
