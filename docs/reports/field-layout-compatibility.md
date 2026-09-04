# Field-layout compatibility reconciliation

Status: complete and qualified on `feat/typed-field-layout`, based on committed main **`8b0129a`**. **Not merged into main.**

## Final version ownership

| Serialized feature | v14 | v15 | v16 | v17 | v18 |
| --- | --- | --- | --- | --- | --- |
| Menu visual variants | yes | yes | yes | yes | yes |
| Directional Sequence frames | reject | yes | yes | yes | yes |
| Authentication metadata | reject | reject | yes | yes | yes |
| Password Recovery | reject | reject | reject | yes | yes |
| Admin form/detail field layouts | reject | reject | reject | reject | yes |

The field-layout branch now uses **AppIR v18**. Recovery keeps v17; no version collision remains. Historical snapshots keep their original format and values when loaded. Unknown future versions fail closed.

## What was reconciled

1. Initially rebase field layout onto Sequence v15 (`b4749ff`) and Authentication v16 (`24295ae`), preserving both implementations.
2. Remove the prototype shortcut that treated v14 as the current format: after combining Sequence, that shortcut would skip the directional-frame version boundary.
3. Capture in-progress Recovery consistently into an isolated temporary branch, rather than modifying or depending on files changing underneath a test run. Test the combined implementation and compare source hashes after qualification.
4. Recovery was committed as **`8b0129a`** during qualification. Its runtime matches the captured source; differences were documentation and generated assets. Rebase the feature commits onto this real commit, removing the temporary snapshot commit from feature ancestry.
5. Allocate v18 to field layout; retain independent gates for v15 Sequence, v16 Authentication, and v17 Recovery. Only v16–v18 delegate to the older validator after their own feature checks, through a local copy; v14 is never promoted.
6. Preserve the completed-plan/progress archival in `24027b0` and `7edaa87` instead of restoring obsolete history.
7. Regenerate canonical schemas and embedded frontend assets from combined sources, not either branch's stale JS/HTML.

CLI/compiler version assertions now expect v18 while retaining Sequence and authentication capabilities. Authentication registration/Recovery compilation and host-readiness checks remain intact. Login, Account and Recovery UI/routes coexist with grouped detail rendering and Admin/Studio controls. No authentication, action, mail, HTTP, migration or release implementation is changed relative to `8b0129a`; only the AppIR feature boundary and field-layout additions differ.

## Compatibility evidence

- **608 combinations**: v1–v19 × independent Sequence, Authentication, Recovery, Admin-layout and detail-layout flags. Includes negative gates, unknown versions and immutable validation.
- Actual compiler-emitted **v14/v15/v16/v17 snapshots** plus corresponding sources, generator and hashes under `internal/appir/testdata/field-layout-baselines.md`. The v17 fixture is byte-identical when generated from the final Recovery commit, not merely a relabelled newer snapshot.
- Original historical definitions recompile equivalently except for format and the v14 compiler-owned default frame direction. JSON numeric representations are compared canonically.
- Every historical release loads after database close/reopen and upgrades to v18 with **zero physical migration statements/descriptions** for field layout. The fixtures install historical AppIR into initialized current metadata storage; this does not claim that Recovery's earlier system-table migrations disappear. Existing Recovery migration/host-readiness tests remain part of the combined gates.
- Upgrades retain Authentication/Recovery and directional Sequence metadata, leave the old snapshot untouched, survive restart, and reject invalid layout publication without replacing the active release.
- Recovery's existing release-bound-token behavior remains unchanged: publishing a new release does not promise to keep old reset links valid.
- A combined browser journey uses actual local STARTTLS SMTP delivery, resets a password, logs in, opens the **grouped Blog form in the same application**, and rejects token replay.
- `make check` passes **107 frontend tests, 26 browser journeys**, and the Go/race/contract/compatibility/black-box gates. Explicit `make build` passes.

## Preserved work and integration

Main's branch, runtime, databases, index and `GOAL.md` are not modified by this work. The feature branch is based on actual Recovery commit `8b0129a`, not a synthetic Recovery snapshot. The original field-layout prototypes remain available at `backup/typed-field-layout-v15` and `backup/typed-field-layout-v17` for audit only.

Those private prototype formats are not supported historical formats: v15 belongs to Sequence and v17 belongs to Recovery. Rebuild disposable prototype packages/databases from source with v18 rather than changing persisted version headers to bypass validation.

Reconciliation and qualification are complete; the only remaining integration step is the main merge, intentionally withheld at the user's request. Any future unrelated metadata feature should allocate a version after v18 rather than reusing this format.
