# Inline semantic Panel content plan

Status values: `pending`, `active`, `done`.

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Freeze ordered YAML, AppIR identity, compatibility, policy, and diagnostic contracts | root goal and design note | done |
| 1 | Compile and validate immutable ordered inline region items | AppIR/compiler/schema/diagnostic tests | done |
| 2 | Render and count mixed inline/named content through existing Panel/Sequence paths | Panel/Sequence/policy tests | done |
| 3 | Convert the presentation example and document canonical authoring | example validation and docs | done |
| 4 | Qualify the repository | `make check` and `make build` | done |

## Design decisions

- A region uses either unchanged `blocks: [name, ...]` or ordered `items`. Each ordered item has exactly one `block: name` or one non-empty `content: [...]`; optional `id` is valid only for inline content. Mixed content and references use `items`, whose list order is render order.
- Inline items lower during definition compilation into nested immutable AppIR region items. Their non-public identity is `@inline/<panel>/<region>/<id-or-ordinal>`; an explicit region-local `id` stabilizes identity across reordering, while an omitted ID deterministically uses the item ordinal.
- Generated identities never enter the global Block map and cannot be named in `block`/legacy `blocks` references. Rendering synthesizes the existing `type: content` Block representation from the compiled region item and dispatches it through the existing Block/content renderer.
- Legacy `regions[].blocks` remains represented and rendered unchanged. Declaring `blocks` and `items` together is rejected because it has no unambiguous interleaving order.
- Inline content has no policy field: Page/Sequence and Panel policy remain authoritative for it. Named referenced Blocks retain independent Block policy. Panel diagnostics use source-indexed paths (`spec.regions.<index>.items.<index>.content.<index>...`) and the shared content validator.

## Verification order

```bash
go test ./internal/appir ./internal/compiler
go test ./internal/panel ./internal/sequence ./internal/page ./internal/httpapi
bin/bean app validate --file examples/presentation/app.yaml --json
make check
make build
```
