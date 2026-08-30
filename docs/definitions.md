# Definitions

Definitions use `apiVersion: bean/v1alpha1`, a supported `kind`, lowercase namespaced metadata, and a typed specification. Supported kinds are Entity, View, Action, Webform, Policy, Block, Panel, Page, Role, Menu, Job, AdminResource, and LocalRegistration. Bundle files contain `name`, `definitions`, and optional seed rows.

Compilation validates envelopes, names, fields, references, relation kinds, limits, Action steps, Panel regions, and route uniqueness. Diagnostics identify kind, name, specification path, and a corrective message. Generated CRUD is emitted as Views and Actions inside AppIR.

## Admin resources

Every Entity receives a generated AdminResource backed by its generated list View and create/update/delete Actions. Add an explicit definition to control its presentation and domain operations:

```yaml
- apiVersion: bean/v1alpha1
  kind: AdminResource
  metadata: {name: article}
  spec:
    entity: article
    description: Editorial content
    labelField: title
    list:
      columns: [id, title, status, updated_at]
      search: [title, body]
      filters: [status]
      sort: [{field: updated_at, desc: true}]
      pageSize: 25
    form:
      fields: [title, body, status]
      readonly: [created_at, updated_at, version]
    actions: [publish_article]
```

`view`, `createAction`, `updateAction`, and `deleteAction` may override the generated names. Configured list fields must be selected by the View, and every Action must target the same Entity. Publication fails on invalid references. Application Admin requires editor or administrator access; System Admin and Studio remain administrator-only. Admin can only search, filter, or sort fields declared by metadata.

## Local registration and sensitive inputs

Local signup is disabled unless a `LocalRegistration` definition references a `register_local_user` Action. That Action declares a literal `defaultRole`; compilation requires the Role to exist and supplies fixed display-name, email, password, and password-confirmation inputs. Password inputs are sensitive/write-only and Action output is limited to safe identity fields. The client cannot provide roles, tenants, or system fields.

## Bound blocks and content presentation

A View or Webform Block may declare typed `inputs` and bind them to Page context. Compilation requires each bound name and type to match the target View filter or Action input. HTTP execution recomputes the binding from the matched Page route and rejects client collisions. View Blocks may configure generic `presentation` fields for list/detail mode, title/body/meta fields, route templates, rich-text fields, and empty state. Entity fields may use the `slug` type. Transaction Actions may bind `$now` once for deterministic timestamps.

## Runtime durability

An Action mutation, its audit row, idempotency result, jobs, and outbox intents share the Action transaction. Idempotency keys are scoped to an Action and include a canonical input fingerprint; reusing a key with different input is a conflict. Jobs and outbox records use persistent pending/claimed/completed-or-failed states with leases, bounded attempts, scheduled retry, stale-claim recovery, and administrator retry/cancel controls. Delivery is at-least-once, so external consumers must deduplicate.

Publication may commit an additive migration before committing the new active-release pointer. On restart, Bean validates the active AppIR against physical storage, permits harmless schema-ahead columns, and reconciles already-applied additive steps before retry. A missing table, column, relation table, or release target fails startup with a diagnostic instead of serving mixed state.
