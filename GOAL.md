# Goal: contextual Admin and request-scoped route records

Status: complete

Adopt the safe part of Drupal-style route upcasting: resolve an authorized record through a View once for one composed server request and make an immutable snapshot available to declared downstream handlers. Prove the capability with a generic Book → Add Page Admin journey.

Detailed contract and milestones: `docs/plans/contextual-admin-route-records.md`.

## Product outcome

From `/admin/books/:id`, an editor can inspect the authorized contents tree and create a Page in that Book. The contextual form fixes the Book/Menu scope, supports parent, weight, and label override, invokes the existing `create_page` Action with `_navigation`, and returns to the Book.

## Contract

- Runtime records are `ResolvedRecord`/`RecordSnapshot` values, not mutable Entity objects.
- Reads remain View-backed and projection/Policy scoped; incompatible read contracts are never collapsed into one authorization proof.
- Cache lifetime is one request and one immutable release, authorization context, and reader/transaction scope.
- Writes re-resolve and authorize inside the Action transaction; GET context is not write authority.
- Derive the first owner-side affordance from existing Menu, Entity navigation, and AdminResource AppIR.
- Add no `book_id`, global hooks, process-wide cache, cross-Entity Admin Action, raw placement endpoint, speculative metadata, or new AppIR format.
- Keep application behavior under `examples/books`; core behavior remains generic.

## Completion

- Focused View/Menu/HTTP, Admin React, and Books browser contracts pass milestone by milestone.
- Existing generic Page creation and target-side NavigationEditor remain compatible.
- `make check` and `make build` pass.
