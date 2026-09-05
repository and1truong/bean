# Typed detail/form field layout

Status: complete, reconciled, qualified and merged into `main` by fast-forward at `40ab41c`.

Worktree: `/Users/htruong/dirs/bean-field-layout`, branch `feat/typed-field-layout`, based on committed **`8b0129a`** (Sequence v15, Authentication v16, Password Recovery v17). Field layout owns **AppIR v18**. Root `GOAL.md` is unchanged. Authoring reference: [field layout](../../field-layout.md); reconciliation evidence: [compatibility report](../../reports/field-layout-compatibility.md).

## Outcome and boundaries

Authors group record fields through Admin Form and detail Display metadata, not Panel or field-per-Block composition. One typed `FieldLayout` owns ordered groups, labels, bounded columns and spans. Existing View projection, Policy, Actions, readonly/derived/protected controls, release activation and legacy rendering remain authoritative.

Blog Post Content pairs Title/Slug and gives Excerpt/Body full rows; Classification pairs Author/Category above Tags. `/posts/:slug/record` presents readonly groups through the existing publication-scoped View; the original article/taxonomy/comments route remains unchanged.

## Decisions

- `form.fields` remains authoritative; explicit layout covers every editable field once and excludes readonly fields.
- Detail layout may omit projected fields but only presents stable, non-sensitive, non-redacted base-record fields. Dotted relationship presentation stays on existing role-based renderers.
- Explicit detail layout excludes renderer title/body/meta/link roles; `Display.title` owns the heading.
- Omitted columns/span normalize to `1`/`single`; explicit null, zero columns, empty span and unsupported values fail compilation.
- Bounds: 16 groups, 64 fields/group, 128 total fields, unique bounded names and nonblank labels up to 120 characters.
- Small screens have one column; two-column groups begin at `48rem`. Full spans and DOM/keyboard/screen-reader order agree.
- Editable fieldsets and readonly labelled sections/definition lists share geometry, not read/write semantics.
- Studio authors ordinary metadata with keyboard-operable ordering buttons. No new business persistence, arbitrary CSS, nesting, visibility rules, tabs or Webform layout API.

## Milestones

| Milestone | Deliverable | Verification | Status |
| --- | --- | --- | --- |
| 0 | Contract and isolated ownership | Blog/Admin/compiler audit; dedicated worktree | done |
| 1 | Typed normalized AppIR, strict validation, schemas and discovery | schema/compiler/clone/inspect/diff tests | done |
| 2 | Grouped Admin/detail rendering and Studio | React membership, Actions, errors, roles, data boundaries and authoring | done |
| 3 | Maintained Blog and portable release behavior | create/edit, keyboard order, 390/800/1100px, publication, restart/package | done |
| 4 | Preserve Sequence/Auth and reconcile Recovery without merging main | actual v14–v17 fixtures; 608-case matrix; metadata-only upgrade; real Recovery commit in ancestry | done |
| 5 | Qualify combined runtime and document | `make check` (107 React, 26 browser journeys), `make build`; light/dark/mobile review | done |
| 6 | Merge into main and requalify | fast-forward `40ab41c`; `make check` and `make build` on main | done |

## Verification

```sh
go test ./internal/appir ./internal/compiler ./internal/agentprotocol ./internal/release
cd web && bun run test -- Admin App Studio FieldLayout Recovery Account
cd e2e && bunx playwright test blog.spec.ts recovery.spec.ts field-layout-package.spec.ts presentation.spec.ts
make check
make build
```

The historical fixtures prove exact load behavior and unchanged recompilation apart from format/default direction; release tests prove v14/v15/v16/v17 → v18 activation without physical migrations and with preserved auth/Recovery/Sequence metadata. The Recovery browser journey now also opens grouped Blog controls after password reset in the same application.

Temporary screenshots and snapshot worktrees are test evidence, not deployment assets. The user subsequently authorized the main fast-forward. No live demo or business database was changed, and `GOAL.md` remains untouched. Private prototype v15/v17 field-layout packages are obsolete; rebuild those disposable artifacts from source rather than relabelling stored releases.
