# Contextual Admin and request-scoped route records

Status: complete

This plan turns the Books Admin friction into a generic Bean capability while adopting the useful part of Drupal-style route parameter upcasting: resolve a route identity once, preserve an authorized record snapshot for the lifetime of one server request, and make that context available to declared downstream handlers.

The first executable journey is:

```text
/admin/books/:bookID
  -> resolve Book through the books AdminResource View
  -> derive scoped Menus and eligible target AdminResources from active AppIR
  -> show Contents and Add Page
  -> /admin/books/:bookID/create/pages?menu=book_contents
  -> create_page with a fixed _navigation owner
  -> atomically create Page + placement
  -> return to the Book record
```

## Problem and repository evidence

- `Page.ResolveContext` currently resolves route and query values into scalar `map[string]any` values. It does not resolve a typed record.
- `beanctx.Request` can carry one Action `Entity` map, but it is not a multi-record route scope, a read cache, or a safe contract for Page/Admin handlers.
- `GET /api/admin/resources/{resource}/{id}` reads the record through its configured AdminResource View.
- `GET /api/admin/navigation/{entity}/{id}` is target-oriented: a Page editor discovers eligible Book/Menu instances. It does not derive target creates from a Book owner.
- `menu.DynamicTree` independently authorizes and loads the Menu owner. Downstream consumers receive only an owner ID, so a composed server operation can repeat the owner lookup.
- React Admin obtains record, navigation, and history through separate HTTP requests. A request-local resolver cannot deduplicate across those requests; reducing browser round trips requires an explicit composite response.
- The existing Page create Action already accepts `_navigation`, and `ReplaceTargetPlacements` creates the Page placement in the same transaction after re-authorizing the owner. No `book_id`, relation table, raw Menu write endpoint, or new transaction operation is needed.
- `AdminResource.actions` deliberately accepts only Actions for the same Entity. A Book Action must not be used as a disguised cross-Entity Page create shortcut.

## Product outcome

On a Book Admin record, an editor can see the authorized `book_contents` tree and choose **Add Page**. The contextual Page form identifies the Book, allows an optional parent, weight, and label override, and creates the Page through `create_page`. Success returns to the Book and the new target is immediately present in its contents.

The behavior must remain generic: any AdminResource whose Entity owns a scoped Menu can expose contextual creates for AdminResources whose Entities declare navigation eligibility for that Menu.

## Architecture decisions

### 1. Resolve records, not “full entities”

In Bean, `Entity` means immutable schema metadata. Runtime data will use an explicit name such as `ResolvedRecord` or `RecordSnapshot` and contain:

- Entity and View identity;
- record ID;
- the authorized projected row;
- an Entity-read authorization proof scoped to the active release, complete authorization context, reader, and request.

Snapshots are copied or otherwise treated as immutable. Consumers cannot mutate shared record maps.

### 2. Every resolution remains a View read

A route resolver does not query a table directly. It executes the declared View or reuses a result from that exact read contract. One scope is bound to one active release, database reader/transaction, and immutable authorization context; projection entries are additionally keyed by View and record ID.

A successful View read must not automatically become an Entity-read proof: an explicit View Policy takes precedence over the Entity Policy in the current runtime. Proof reuse is valid only for the exact read contract, or when runtime checks establish that the View uses the same effective Entity-read Policy and contextual visibility semantics. A stricter, looser, or merely different View causes an explicit second authorization read rather than an unsafe optimization. One View's fields, redaction, fixed predicates, relationships, or Policy can never satisfy another View projection.

Missing and unauthorized records remain indistinguishable to the client and return the existing not-found behavior.

### 3. Scope is one server request

No process-global or cross-request record cache is introduced. A cache cannot outlive its AppIR snapshot, authorization context, database reader, or transaction.

The Admin record endpoint will provide a bounded composite context so Book data and its owner-side Menu affordances can be assembled in one HTTP request. Separate browser requests may resolve the owner independently.

### 4. Writes re-resolve inside the Action transaction

A GET-time snapshot is never accepted as write authority. `create_page` continues to submit `_navigation`; `ReplaceTargetPlacements` re-reads and authorizes the Book using the Action transaction before inserting the placement. Optimistic concurrency, Policy, audit, idempotency, and rollback behavior remain unchanged.

### 5. Context dependencies stay explicit and closed

This work does not add Drupal-style ambient hooks or runtime plugin discovery. Internal handlers may receive a request scope/resolver, but each operation still derives its dependencies from immutable AppIR and closed capabilities. No consumer may discover arbitrary tables, fields, routes, or callbacks.

### 6. Derive the first affordance from existing AppIR

The initial slice adds no Definition field or AppIR version. Contextual creates are derived deterministically from:

```text
Menu.Owner.Entity
  <-> owner AdminResource.Entity
Entity.Navigation.Menus
  <-> scoped Menu name
AdminResource.Entity + CreateAction
```

Multiple eligible target AdminResources are sorted by stable resource name. Missing target AdminResources produce no create affordance. If later examples need custom visibility, labels, ordering, or non-Menu contextual operations, that evidence can justify explicit metadata through the normal schema -> compiler -> AppIR lifecycle.

## Proposed server contract

Extend the existing Admin record response additively:

```json
{
  "data": {
    "id": "book-id",
    "title": "Building Bean"
  },
  "context": {
    "menus": [
      {
        "name": "book_contents",
        "label": "Book contents",
        "items": [],
        "creates": [
          {
            "resource": "pages",
            "entity": "page",
            "label": "Page"
          }
        ]
      }
    ]
  }
}
```

The exact Go type names may change during M0, but the contract must preserve these properties:

- owner record and context come from one active AppIR snapshot;
- Menu and target order are deterministic;
- trees and target counts retain existing runtime bounds;
- only authorized owner/target data is returned;
- current clients that read only `data` remain compatible;
- the server returns semantic identities, not arbitrary client-authored URLs.

The canonical contextual client route is:

```text
/admin/:ownerResource/:ownerID/create/:targetResource?menu=:menu
```

The server-provided context validates that the owner resource, target resource, and Menu form a compiled eligible triple. Refreshing or directly opening the URL must work; React navigation state is not authoritative.

## Milestones

### M0 — Freeze semantics and executable failures

- Record the architecture decision for View-backed request resolution, cache lifetime, immutable snapshots, and transaction-time revalidation.
- Add failing contracts for deterministic reverse Menu discovery, unauthorized owners, ambiguous/missing targets, and contextual route validation.
- Add a counting reader fixture that demonstrates the duplicate owner lookup and fixes the expected per-request query budget.
- Freeze the additive Admin response and canonical contextual route shapes.

Verification:

```bash
go test ./internal/httpapi ./internal/menu ./internal/view
```

### M1 — Request-scoped resolved-record seam

- Introduce a small explicit request scope/resolver rather than extending `beanctx.Request.Entity` into an ambient bag.
- Resolve through View-owned read semantics and cache only exact projections plus separately scoped, contract-compatible Entity-read proofs.
- Let Menu owner authorization consume a valid same-request proof without repeating the owner read; force a second read when the Admin View and Entity visibility contracts differ.
- Prove isolation by release, View, complete authorization context, reader/transaction, and request.
- Prove returned rows cannot be mutated to alter another consumer's snapshot.
- Keep existing direct `DynamicTree` and View behavior compatible for callers without a seeded scope.

Verification:

```bash
go test ./internal/view ./internal/menu ./internal/httpapi
```

### M2 — Owner-side Admin context

- Deterministically derive scoped Menus and eligible target AdminResources from AppIR.
- Extend the Admin record response with bounded Menu trees and contextual create descriptors.
- Reuse the already resolved AdminResource record as owner context and as an authorization proof only when its effective visibility contract is compatible; otherwise perform the explicit owner check.
- Preserve not-found behavior and prevent field/projection leakage.
- Add HTTP tests for zero, one, and multiple eligible targets; empty and populated trees; denied owners and hidden targets; and stable ordering.

Verification:

```bash
go test ./internal/compiler ./internal/httpapi ./internal/menu
```

No new schema or compiler surface is expected in this milestone; compiler tests protect the existing Menu/AdminResource graph assumptions.

### M3 — Generic contextual Admin UI

- Add an owner-side Menu section to a record page only when the response contains eligible context.
- Render the authorized tree and one **Add {Target}** affordance per eligible target resource.
- Add the contextual create route while preserving `/admin/:resource/new`.
- Reuse the target AdminResource form and create Action.
- Render the owner as fixed context; allow parent, bounded weight, and label override; do not let the form silently switch owners.
- Submit one `_navigation` placement and derive the return route from validated resource identities rather than accepting an arbitrary redirect.
- Preserve loading, empty, field-error, CSRF, keyboard, mobile, and direct-refresh behavior.

Verification:

```bash
cd web && bun test src/Admin.test.tsx
cd web && bun run lint && bun run typecheck
```

### M4 — Books tracer bullet

- Keep all Book/Page behavior in `examples/books` and use the existing Menu, Entity navigation, AdminResource, View, and Action definitions.
- From `/admin/books/:id`, create a top-level Page and a child Page.
- Verify redirect back to the Book, immediate tree visibility, and public route navigation.
- Verify the same Page model remains reusable across Books; do not add `book_id`.
- Verify invalid Menu/resource combinations, tampered owner IDs, unauthorized owners, invalid parents, and failed placement validation create neither Page nor placement.
- Add a browser query/geometry assertion only where it proves user-visible behavior; keep lower-level query-count evidence in Go tests.

Verification:

```bash
cd e2e && bunx playwright test books.spec.ts
./bin/bean app validate --file examples/books/app.yaml
```

### M5 — Qualification and documentation

- Document contextual Admin behavior, request-scope limits, and transaction-time revalidation in `docs/architecture.md`, `docs/definitions.md`, and the Books README.
- Regenerate artifacts only if generated inputs actually changed.
- Run the nearest suites after every milestone and the repository gates at completion.

Terminal gates:

```bash
make check
make build
```

Run the reusable PostgreSQL HTTP/Menu contract when the implementation changes shared backend-facing behavior.

## Acceptance criteria

- In the Books contract, a Book is resolved once for the composed Admin record/context server request; Menu owner authorization does not issue a duplicate owner lookup.
- A View with a non-equivalent effective Policy or contextual predicate cannot seed an Entity-read proof and performs an explicit authorization read instead.
- Cache entries never cross request, release, authorization context, View, or database reader/transaction boundaries.
- No consumer receives fields outside its declared View projection.
- `/admin/books/:id` exposes Contents and Add Page without Books-specific core code.
- A contextual Page form survives direct load/refresh, fixes the current Book/Menu context, and offers only valid parents.
- Page creation and placement use the existing `create_page` Action and commit or roll back together.
- The Action transaction re-authorizes the owner regardless of GET-time context.
- Existing generic Page creation, Page-side NavigationEditor, Menu rendering, AdminResource APIs, and old clients remain compatible.
- No Entity relation, SQL outside DBAL/migration, raw Menu mutation endpoint, global hook, or process-wide cache is introduced.
- `make check` and `make build` pass.

## Completion evidence

- View scope tests prove exact-projection caching, deep-copy immutability, copied authorization context, request isolation, and explicit Entity re-reads for different Policies and contextual predicates.
- Menu counting-reader coverage proves the composed Admin request performs one Book lookup while retaining the compatible direct `DynamicTree` API.
- HTTP contracts cover empty and populated owner context, stable eligible targets, hidden and missing targets, projection compatibility, and missing-owner not-found behavior.
- Admin React contracts cover empty owner context, semantic Add routes, fixed placement submission, and invalid triple rejection. The Books browser journey creates top-level and child Pages, returns to the Book with immediate tree visibility, rejects tampered identities, and proves failed placement validation leaves no Page.
- `./bin/bean app validate --file examples/books/app.yaml`, `make check`, and `make test-postgres` pass. The final `make build` is recorded in `docs/progress.md`.

## Explicit non-goals

- A general Drupal-compatible hook/module system or mutable ambient Entity objects.
- Cross-request identity maps, process-global ORM sessions, relation auto-hydration, lazy proxies, or arbitrary recursive route converters.
- Public Page route-record metadata in the first slice; add it only when a separate public-page case proves the need.
- Arbitrary Admin detail layout/Blocks, inline editing of unrelated Entities, attach-existing-Page, owner-side reparent/reorder/delete, or drag-and-drop.
- A cross-Entity AdminResource Action or a new Action step for writing Menu placements.
- Trusting URL/query/client state as authorization or reusing a rendered record snapshot for writes.
- Changing the definition -> validation -> migration -> immutable AppIR -> atomic activation lifecycle.
