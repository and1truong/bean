# Progress

## Current

The shared header now uses the published application name when Theme.displayName is absent. Bundle publication snapshots the name in AppIR, definition-only republication preserves it, and legacy releases use the registered name. Regressions cover init's Bean placeholder, republication/reload, the app-name default, and explicit Theme branding. Preview includes the bundle name, while demo seed still compares operational definitions independently of branding. Final `make check` passes 90 frontend tests and 20 browser journeys; `make build` passes. Community release 6 is active and its Admin header renders Community in the browser. An earlier scheduler test failed intermittently, then passed in isolation and in the final full gate without unrelated changes.

Community Post table now explicitly shows Body, Visibility, and Updated at. The existing renderer links the first visible column when the default ID label field is omitted, so Body opens the record without a core change. The active local release is verified in the browser: no ID header and clicking Body opens the correct Post. `make check` passes all 20 browser journeys, and `make build` passes.

Community now has a homepage at `/` through a page Display on the existing public_feed View, preserving its public-only filter and Policy. Browser verification confirms signed-in and anonymous access on localhost:8083. The new regression first reproduced page API 404, then passed with an empty state, public posts, and private-post exclusion for both visitors and the owner. All three Community regressions pass; final `make check` passes 87 frontend tests and 20 Playwright journeys, and `make build` passes. The updated release is active in the local demo.

Community browser verification runs at localhost:8083 with two local member/editor accounts. A can create a private Post and B sees no matching records. The example omitted publish_post from Admin even though Visibility is protected after creation; an explicit Post AdminResource now exposes that existing Action. README setup now initializes an administrator and explains member/editor access plus the public feed. Browser verification confirms B cannot open A’s private Post, A can publish it, and B can then see it and create a like Reaction. Community validation/semantic tests and both API/browser regressions pass; member-only API identities remain unchanged, with separate member/editor identities for Admin testing. `make check` passes 87 frontend tests and all 19 Playwright journeys; `make build` passes. The ATS datetime fix was committed as `0bd5d4c`; the unrelated Books README remains separate.

ATS browser verification and the discovered Admin datetime fix are complete. The demo remains at localhost:8082 using `tmp/ats-admin-browser.db`, with 18 candidates. Search, Applied → Screen, and administrator login/list/detail work. Browser testing reproduced a blank required Applied control that blocked unrelated Summary edits: RFC3339 values were passed directly to datetime-local inputs. Admin now renders local datetimes, accepts fractional seconds, and converts edited local values to RFC3339 for create/update and selection Actions while leaving untouched timestamps out of update payloads. Browser Save/reload verifies the Summary persists and Applied retains its instant. All 11 Admin tests and 87 frontend tests pass; `make check` passes all 18 Playwright journeys including an ATS Save/reload regression, and `make build` passes.

Blog browser verification found that the fresh demo at localhost:8081 supports administrator login, Category creation, Post creation/editing, publication, and public listing. The existing localhost:8080 Blog denied the default demo credentials; its README omits administrator initialization, but the exact account state was not confirmed. The user then requested ATS verification; no Blog runtime or authentication changes were made.

Missing public page handling now preserves HTTP status in API errors, renders an explicit not-found state for 404, and suppresses 404 retries plus focus/reconnect refetches. Other errors render an alert and retain bounded retries. Regression tests reproduced four requests for a missing route before the fix; all 54 App tests now pass, including missing root/arbitrary routes, focus/reconnect, transient recovery, and exhausted server retries. `make check` passes all Go/race checks, 83 frontend tests, and 18 Playwright journeys; `make build` passes and refreshes the embedded frontend assets. Chromium required execution outside the sandbox after its initial launch was denied.

Booking example source splitting is complete. `examples/booking/app.yaml` is now an explicit manifest for `resources.yaml`, `bookings.yaml`, and `calendar.yaml`; the README documents each responsibility. The pre-split and split source sets produce the same checksum (`74c8773ce57034f09c4cd0b3f20a504ca596153e26c7dc6410531ecf7c796d8b`), and Booking validation/semantic tests, `make check`, and `make build` pass.

The `clsx` + `tailwind-merge` to `cn` migration is complete on `chore/migrate-to-cn`. The frontend has one shared merger at `web/src/lib/utils.ts`; no direct `twMerge()` calls or duplicate helpers exist, and every existing component call site remains unchanged. Direct dependencies now use `cn` 0.2.5 and retain `class-variance-authority`; `clsx` remains only as CVA's transitive dependency. Ten focused regressions cover conditional inputs, spacing/text/background conflicts, responsive/state/dark/arbitrary/important behavior, Tailwind v4 variable shorthand, and CVA output.

A lightweight Bun benchmark used 500,000 calls per run, 20,000 warmups, five isolated runs, and representative Bean class strings. Best observed old/new times were 82.6/16.9 ns for simple conditional calls, 99.8/10.6 ns for variant-heavy calls, 92.9/10.6 ns for conflicts, and 90.5/8.7 ns for repeated warm calls. These microbenchmarks support the dependency choice but do not measure end-to-end UI performance. The default production JS changed from 479,142 to 479,687 raw bytes and from 143,733 to 146,574 gzip bytes; generated CSS is byte-identical.

`cn build` was evaluated but not adopted. Version 0.2.5's fixed-path output was byte-deterministic and reduced the measured JS to 472,402 raw/143,911 gzip bytes, but it requires a generated table, a pre-build/dev hook, and scanner maintenance; placing the documented generated TypeScript under Bean's current Tailwind-scanned source also increased generated CSS. The package documentation recommends the default import for most projects, and the small wire-size tradeoff does not justify extra generated artifacts and CI complexity for this focused migration. Final frontend lint, typecheck, all 79 Vitest tests, and production build pass; `make check` passes all Go/race/black-box contracts and all 18 Playwright journeys; final `make build` passes.

Bean Design System v2 is complete on `ux/bean-design-system-v2`. `docs/ui-audit.md` records the repository-grounded audit, and `docs/design-system.md` defines the independently authored MIT-compatible visual language and remaining incremental migration notes. Semantic light/dark palettes, compact type/spacing/radius/control scales, border-led surfaces, a stable authenticated tool shell, accessible field errors, dense sticky-header tables, explicit active filters, normalized local states, a resource-directory Admin home, two-pane Explore, and navigator/workspace/inspector Studio are implemented without changing metadata, View, Action, Policy, or release behavior. React coverage passes 69 tests; Playwright passes all 18 application journeys, including responsive Admin and Studio. Representative 1440px Admin, table, Explore, Studio light, and Studio dark states were captured to temporary PR evidence paths and visually reviewed; screenshots are not committed per `docs/howto-pr.md`. Final `make check` and `make build` pass.

Shadcn-styled route tabs are complete. A source-owned `RouteTabs` adapter reuses the shadcn Tabs visual contract through semantic `nav`/`Link` components rather than adopting Radix `tablist`/`tab`/`tabpanel` behavior. AppIR v14 adds normalized, schema-visible `default` and `line` Menu variants, including capability discovery and Studio authoring. Legacy flat Menus, responsive workspace composition, active trails, and the mobile native `Section` select remain unchanged. React tests cover both variants and semantics; the Books example and Playwright coverage exercise the `line` variant across desktop/sidebar/mobile layouts. Visual review confirmed the underline and vertical indicators preserve layout and reading order. `make check` passed with 68 React tests and 18 Playwright tests.

Workspace Menu/content layout correction is complete. The generic renderer now composes a `workspace` Menu with following route content, including a Menu nested in a Panel Region: desktop levels 1–2 span the top, level 3 occupies a bounded left column, and content occupies the right column; narrow screens retain the labelled `Section` select above content. React regressions preserve View Page Display composition, Panel/Menu composition, flat Menus, no-tertiary flow, semantic navigation, and navigation-before-content DOM order. Books Playwright coverage measures both desktop compositions and mobile select-above-content geometry while changing routes and checking `aria-current`. The live `:8080` server now runs the rebuilt binary against the existing `tmp/books.db`; visual screenshots confirm both layouts and the persisted 27 Page records remain present. Final `make check` passes all Go, race, 68 React, black-box, and 18/18 Playwright tests; `make build` passes.

Dynamic hierarchical Menu navigation is complete. AppIR v13 and the compiler now cover global typed Page/View placements, derived owner-scoped Menu instances, Entity navigation destinations, three parent-derived levels, deterministic weight ordering, and explicit v12 compatibility. Portable `bean_menu_placement` data, publication orphan preflight, and strict `_navigation` replacement submissions now run through generic DBAL transactions: Entity create/update/delete and placement mutation either commit together or roll back together, while target/owner deletion cleans up and parent deletion with children is rejected. Server resolution authorizes owners and targets through Views, suppresses denied subtrees, resolves record routes without persisting expanded URLs, and emits active trails. React renders semantic horizontal levels, a desktop vertical third level, and one labelled mobile native select with links and `aria-current`, never ARIA tabs. The maintained `examples/books` slice declares global and scoped Menus, reusable Page targets, authenticated target filtering, and generated AdminResources. Generated record forms list up to 32 authorized owner instances, prioritize existing placements, and edit parent, weight, label override, and removal without drag-and-drop; Studio visually authors Menu and Entity navigation contracts. Focused browser evidence now covers two Books, one Page reused across both, a three-level active trail, mobile select behavior, generated-form reparenting, Policy filtering, and target/owner cleanup; release restart coverage preserves the compiled contracts. Final qualification is green: `make check` passes all Go, race, 67 React, black-box, and 18/18 Playwright tests; `make test-postgres` and `make build` pass.

Bean Blog composition modernization is complete. All taxonomy and approved-comment presentation now belongs to named View Displays, and all Blog Pages use explicit semantic-width sections. Article content renders at readable `contained` width; its `wide` discussion Panel stacks comments before the member form on narrow screens, becomes a `2:1` main/sidebar layout on large screens, and collapses the Policy-denied sidebar so anonymous readers receive an expanded comments Region. Public local narrative uses inline Panel content, while the member-only sidebar introduction remains a named Policy-bound content Block. The 66-definition Blog validates without diagnostics, and its browser journey preserves draft isolation, publication, safe Markdown, RSS/taxonomy reads, registration, scoped moderation, and approved-comment visibility while proving widths, responsive order, and authorization-aware collapse. Final `make check` passes all Go, race, 64 React, black-box, and 17/17 Playwright tests; `make build` passes.

Semantic Page section widths are complete. `sections[].width` uses only `contained` (`48rem`), `wide` (`72rem`), or `full` (available viewport), all with safe gutters; omitted and legacy widths preserve current `wide` geometry; width belongs to Page placement and does not affect Sequence, Policy, context, or accessible order. AppIR v12/compiler/schema/diff, Page/restart, Studio, React/CSS, and tracker browser geometry coverage pass. Final `make check` passes all Go, race, 64 React, black-box, and 17/17 Playwright tests; `make build` passes.

Asana Lite composition modernization is complete. All legacy Block presentation moved into named View Displays; one-off narrative Blocks became ordered inline Panel content; and the home, project, and task routes now compose focused responsive Panel sections. Project priority filters still fan out across chart, board, and tree sections, while route bindings, Actions, Policies, hierarchy, and attachment behavior remain unchanged. The Asana browser journey proves real one-to-two-column responsive behavior and ordered layout bands while retaining anonymous creation, board movement, deep subtasks, and file download. Final `make check` passes all Go, race, 64 React, black-box, and 17/17 Playwright tests; `make build` passes.

Policy-aware collapsible Panel Regions are complete. Opt-in `regions[].collapseWhenEmpty` only reacts to a zero-child server-authorized render tree, preserves existing Regions by default, expands a sole survivor across Panel tracks, makes an all-collapsed Panel unavailable, and stores the behavior in AppIR v11. AppIR/compiler/schema/diff, Panel/Page/Sequence, restart, React/CSS, and tracker browser coverage pass. Final `make check` passes all Go, race, 64 React, black-box, and 17/17 Playwright tests; `make build` passes.

Ordered multi-Panel Page sections are complete. `sections: [{id?, panel}]` provides 1–32 ordered layout bands with stable internal identities, remains mutually exclusive with legacy `panel`, stores directly in immutable AppIR v10, and retains Page filters plus Page/Panel/Block Policy and bound-request validation. Compiler/schema/reference/diff, Page/HTTP/LocalRegistration, restart, Studio, React, and tracker browser coverage pass. Final `make check` passes all Go, race, 63 React, black-box, and 17/17 Playwright tests; `make build` passes.

Deterministic responsive Panel presets are complete. Existing layout names now map to fixed runtime-owned small/medium/large viewport behavior at `48rem` and `64rem`, preserve source and accessibility order, bound tracks and Regions around wide content, and add no schema or AppIR fields. Focused React semantic-hook tests and Playwright computed-layout checks cover all five presets. `make check` passes all Go, race, 62 React, black-box, and 17/17 Playwright tests; `make build` passes.

Inline semantic Panel content is complete. Legacy `regions[].blocks` remains unchanged, while ordered `regions[].items` interleaves named Block references and inline semantic content. Inline items compile into nested immutable AppIR v9 identities, use the existing `ContentBlock` renderer and validation vocabulary, participate in Sequence Block/density/feature checks, and inherit enclosing Page/Sequence and Panel visibility; referenced named Blocks retain independent Policy checks. Focused AppIR compatibility, schema/compiler diagnostics and source locations, deterministic identity, Panel/Sequence render-tree, policy, legacy compatibility, presentation-agent, publication/restart, and complete Go tests pass. The presentation example now uses inline content for frame-local narrative, retains an intentionally named content Block, and preserves the live View-backed chart. Final `make check` passes all Go, race, 61 React, black-box, and 16/16 Playwright tests; `make build` passes.

The focused required Entity form marker is complete. Metadata-driven Admin controls now render a destructive-theme red `*` directly after labels when the Entity field has `Required: true`, including text, numeric, sensitive, enum, relation, boolean, textarea, file, and read-only controls. The marker is hidden from assistive technology, preserving accessible field names and existing native required semantics; optional fields remain unmarked. Focused Admin React evidence verifies order and styling. `make check` passes all Go, race, 61 React, black-box, and 16/16 Playwright tests; `make build` passes.

The post-v0.16 CSRF continuity fix is also complete. An HttpOnly session cookie can authenticate a direct or reloaded Admin tab while the Shell restores missing tab-local `bean_csrf` state from `/api/system/session`; the next Admin Action carries the restored token without weakening server validation.

## v0.16 completed state

Bean v0.16 Semantic Sequences is complete on branch `v0.16-presentations`, based on the completed v0.15 Explore commit. Milestones 0–5 are done.

Repository evidence led to one generic `Sequence` definition instead of a presentation-specific parallel runtime. AppIR v8 stores a route and ordered frames referencing existing Panels. Panels/Regions/Blocks remain the composition tree; View Displays remain the data rendering path; Theme remains the visual token contract; Policy remains authoritative. One generic `content` Block supplies bounded headings, paragraphs, bullets, quotes, code, callouts, accessible images, and ordered diagrams without arbitrary markup or scripts.

The first Sequence profile is an accessible HTML presentation with stable frame links, focus-safe keyboard/navigation controls, speaker notes, wide/standard aspect ratios, and print page breaks. Deterministic metadata-weighted density limits fail compilation with exact repair paths; Bean does not claim pixel-perfect browser measurement. The executable `examples/presentation` application is a ten-frame introduction to Bean and includes a real grouped View/chart over deterministic capability data.

Focused AppIR compatibility, compiler/schema/capabilities, stable invalid-to-valid agent repair, semantic diff/reference, release restart, server render/View authorization, React navigation/print, and presentation Playwright tests pass. Source-independent packaging retains the complete Sequence and seeded View chart. Final qualification is green: `make check` passes all Go/race/black-box contracts, 49 React tests, and 16/16 Playwright journeys; `make test-crash`, `make test-postgres`, and `make build` pass; the binary reports `bean 0.16.0-alpha`. Native PDF/PPTX, WYSIWYG/freeform layout, embedded LLMs, web research, image generation, arbitrary HTML/CSS/JS/SVG, and hosted sharing remain explicitly outside v0.16.

## v0.15 completed state

Bean v0.15 Explore is complete at local commit `05da909`; milestones 1–8 are done. The implemented boundary is “small core gaps plus first-party module”: administrator Entity exploration/save plus compiler-known query/result/interaction semantics, all materialized as ordinary View, Display, Page, Panel, Block, Action, Policy, and Lifecycle definitions. `bean/appir/v7` adds View-owned search, typed grouping/aggregation and result shapes, deterministic backend ordering, bounded group overflow, Policy-before-aggregate execution, compatible displays, Page filters, typed drill, bounded Action batches, Explore/Studio/agent parity, and executable ATS/CRM/Commerce/Booking/Asana/Tracker proofs. Final local `make check`, `make test-crash`, `make test-postgres`, and `make build` were green; the binary reports `bean 0.15.0-alpha`.

## v0.14 completed state

Bean v0.14 First-class View Displays is complete at merge `7feca74`; milestones 0–6 are done. `bean/appir/v6` keeps query metadata at View scope and adds named page/block/JSON/CSV/RSS displays, derived typed exposed-filter mappings, compiler-known render/control/pager vocabularies, Block display references, and deterministic legacy `Block.presentation` normalization. Schemas, capabilities, inspection references, semantic diff, AppIR compatibility guards, route/field/redaction/link/widget/pager/title validation, and focused compiler/AppIR/Agent Protocol tests pass.

Page and Block runtime execute displays through the existing Policy-preserving View service. Page routes and immutable bindings are server-resolved, client collisions and undeclared controls fail closed, filter state invalidates opaque cursors, and display limits stay within the View maximum. React evidence covers tables, labels, inferred values, safe links, empty/error states, URL controls, cursor previous/next, and static/result-derived headings and browser titles. Studio authors the common page/block, list/detail/table, column, control, pager, and title path without Advanced JSON.

Blog list/detail Blocks and ATS list/board/metric/timeline Blocks reference named displays; ATS also declares the metadata-only `/candidates` table page with a labelled stage control and cursor pager. Both sources compile without diagnostics. Local v0.14 qualification was green: `make check`, `make test-crash`, `make test-postgres`, and `make build` passed, including all 14 Playwright journeys. The release excludes a public Query/Querier definition, nested query rewrite, numeric/full paging, arbitrary templates or application code, sequence/slide rendering, and presentation export.

## v0.13 completed state

Bean v0.13 Typed Extension Boundary is complete at squash merge `8a8c199`. All milestones are done. `Extension` is a registry-owned Definition in immutable `bean/appir/v5`, with canonical schema, capabilities, stable `BEAN-E2871` diagnostics, inspection, semantic diff, metadata-only publication, restart, and explicit v1–v4 compatibility. Its closed contract accepts one HTTP transport, network permission, external-write effect, none-or-bearer authentication, bounded timeout/fixed retry, required idempotency, after-commit transaction semantics, and retry-then-fail behavior. Endpoints require HTTPS except for loopback HTTP; typed fields exclude secrets, relations, files, and storage semantics.

Transaction Actions now bind typed Extension inputs and persist one invocation in the existing outbox transaction. Policy denial creates no intent, rollback removes it, the success audit shares its commit, and Action idempotency replay does not enqueue twice. One invocation ID is the HTTP idempotency key across fixed-delay attempts. The HTTP provider sends a versioned bounded JSON POST, resolves bearer tokens only from `BEAN_EXTENSION_BEARER_TOKENS`, refuses redirects, validates typed output, and reports safe deterministic timeout, unavailable, authentication, redirect, response, and contract categories. Retryable failures retain existing lease/stale-claim behavior; permanent failures become terminal immediately. Focused compiler, Action, outbox, HTTP, bootstrap, and complete Go tests pass.

Semantic TestSuites now inject ordered typed provider responses or stable failures into the production delivery path after the Action transaction commits. Expected calls include Extension identity, immutable invocation/idempotency identity, and typed input; exhausted or unconsumed mocks fail through stable redacted `BEAN-T1001` evidence. Repeated execution is deterministic, provider values are redacted from inspection, generated negative cases cannot inherit irrelevant mocks, and ordinary event assertions exclude internal Extension topics.

Commerce now declares the generic `order_notification` Extension in metadata. `place_order` atomically creates its order and notification intent, then its maintained TestSuite delivers through a typed offline mock while continuing to prove inventory mutation, order creation, and the public `order_placed` event. Direct and generated replay suites pass without network access. Arbitrary scripts, synchronous provider results, WASM/plugins, OAuth, new queue infrastructure, and provider-specific core behavior remain excluded.

Local v0.13 qualification is green: `make check`, `make test-crash`, `make test-postgres`, and `make build` pass, including race detection, 30 frontend tests, 14/14 browser journeys, PostgreSQL application parity, crash/restart recovery, and `bean 0.13.0-alpha`. PR #15 reached the ten-review fallback; every final finding was fixed, answered, and resolved, final CI was green, and post-merge main workflow `33579291003` passed.

## v0.12 completed state

Bean v0.12 Generated Semantic and Rule Tests is complete at squash merge `fbaa54a`. Generated runtime checks materialize as ordinary TestSuite definitions from compiler-validated AppIR, explicit expectations, and relation-complete DemoSeed data. Explicit expectations remain the oracle; generated Policy/Lifecycle negatives and CRUD mutations use Actions, verification reads use Views, and eligible route journeys use the production HTTP handler. Structural evidence covers schema, Rule, Policy, Lifecycle, and route contracts with stable source identities.

Commerce, ATS, and booking pass generated replay, negative, CRUD, and journey checks. Repeated complete machine output is byte-identical; malformed routes produce structured failures rather than panic. `make check`, `make test-crash`, `make test-postgres`, and `make build` passed with `bean 0.12.0-alpha`. PR #14 reached the ten-review fallback; every final finding was fixed, answered, and resolved, final CI was green, and the post-merge main workflow `33570003770` passed.

## v0.11 completed state

Bean v0.11 First-class Semantic Test Suites is complete at merge `f224246`. TestSuite is registry-owned metadata with canonical schema, stable diagnostics, capability limits, inspection, references, semantic diff, immutable `bean/appir/v4` storage, and restart compatibility. Typed Rule and Action cases execute in fresh bounded SQLite runtimes through production paths with explicit context and deterministic result/error/mutation/event/audit evidence.

Commerce, ATS, and booking suites catch seeded calculation, Policy/guard, derivation, invariant, mutation, and event defects. `make check`, `make test-crash`, `make test-postgres`, and `make build` passed with `bean 0.11.0-alpha`. PR #13 merged after the documented ten-review fallback: every final-round finding was addressed and resolved, final validation/CI were green, and the post-merge main workflow passed.

## v0.10 completed state

Bean v0.10 Deterministic Rule Expressions is complete from merge commit `38a6095`. All milestones 0–5 are done. The frozen contract uses a named Rule definition with a canonical structured AST, explicit typed sources, a small fixed operator set, and compile/runtime resource bounds. The Rule core checks and evaluates every accepted operator and source, preserves exact integer and finite-decimal arithmetic, compares typed dates/datetimes, short-circuits deterministically, returns coded failures, and enforces 128-node, depth-16, 4 KiB literal, 16 KiB value, and normalized exact-number exponent limits. Replay, forbidden vocabulary, canonical shape, arity, typing, nullable boolean, missing-value, divide-by-zero, and resource-limit contracts pass with the repository-wide Go suite.

Rule is now a registry-owned Definition with canonical schema, `bean/appir/v3` storage, v1/v2 rejection of Rule-bearing releases, stable `BEAN-E2351` expression diagnostics, closed operator/source/limit capabilities, named inspection, typed references from Action and Entity consumers, and semantic diff. Inspect redacts literal values with a stable digest so threshold changes remain visible without exposing their contents. Compiler validation resolves Entity and input types and rejects mismatched guard, derive, and invariant consumers before publication.

Actions now reject client-supplied derived inputs, evaluate sibling derives simultaneously from the original input, inject one stable `context.now` across transaction retries, and preserve idempotent replay. Record Policy checks precede Rule guards; false guards deny before mutation and evaluator failures retain stable Rule causes while failing closed. Entity invariants evaluate the final typed create/update/transition candidate immediately before persistence, including create/update/decrement transaction steps, so a failed invariant rolls back the complete transaction. OpenAPI and Admin action forms omit server-owned inputs. Focused tests cover record-aware guard denial, Policy-before-failing-guard order, derivation, override refusal, unavailable context, candidate validation, rollback, and replay; the complete Go suite and 30 frontend tests pass.

Three maintained applications now carry unrelated metadata-only slices: commerce derives `order_item.line_total`; ATS guards pipeline movement against blank candidate names using the current record; booking derives `requested_at` from injected `context.now` and enforces `start_at < end_at`. Focused source execution and all three browser journeys pass. DemoSeed deterministically evaluates create derives for its expected dataset while omitting server-owned fields from Action calls. Rule publication is metadata-only, survives SQLite active-release reload, and executes after restart. The booking derive/invariant path passes the shared SQLite/PostgreSQL HTTP parity suite, `make test-postgres` passes including the PostgreSQL blog journey, and `make test-crash` passes both publication fault points.

Local terminal qualification passes on the v0.10 version cut: `make check` including race, black-box contracts, 30 React tests, and 14/14 Playwright journeys; `make test-crash`; `make test-postgres`; and `make build`. The binary reports `bean 0.10.0-alpha`; PR #11 merged after a clean latest-commit Codex review and green CI.

The first consumers remain Action guards, simultaneous server-owned derived Action inputs, and Entity invariant predicates; three unrelated metadata-only applications demonstrate computed/derived values, a record-aware guard, validation, and injected `context.now`.

Rules refine local behavior but do not replace Entity, Lifecycle, Policy, View, or Action structure. Policy remains authoritative authorization; evaluation is side-effect-free and has no I/O, implicit clock/randomness, environment, SQL, modules, or mutable state. Text/infix syntax, browser visibility, computed read columns, generated tests, and external effects are intentionally deferred.

## Structural contracts completed state

The structural-contract and unified-execution-seam goal is complete. All milestones 0–9 are done without adding a public plugin platform or application-specific core behavior.

Diagnostics now receive codes and non-serialized recovery facts structurally, with no production control flow that parses diagnostic or Go standard-library prose. Database and transactional reads share the View-owned engine, including Policy fallback, filtering, deterministic ordering, limit clamping, sanitization, redaction, and relation hydration. Action steps share one entity resolver and enforce declared read/write effects before handlers execute. A context-specific value-source owner now serves expressions, Actions, Blocks, Pages, compiler validation, and inspection and fails closed for unsupported sources.

The client has typed Page/Panel/Region/Block dispatch, explicit unknown-component and expression-operator failures, and one Action encoder/caller for JSON, multipart, Webform adaptation, and deterministic batches; Admin and ActionRunner expose field errors. Definition-kind entries now own compilation, explicit-phase normalization and per-kind validation, inspection, schema type, and AppIR storage metadata; completeness is checked against an independent AppIR storage contract. Agent Protocol metadata and handlers are sealed together, test overrides use construction, and the unreachable not-implemented state is removed. Confirmed zero-caller `internal/entity` and `internal/user` pass-through packages were deleted.

Qualification passes: `make check` including Go race tests, 30 frontend tests, black-box contracts, and 14/14 Playwright journeys; `make test-crash`; `make test-postgres` including the PostgreSQL blog journey; and `make build`. The definition -> validation -> migration -> immutable AppIR -> atomic activation lifecycle, View-read/Action-write boundary, backend confinement, maintained examples, and public envelopes remain intact.

## v0.9 completed state

Bean v0.9 Semantic Application Model is complete on top of merge commit `c51705d`. The scoped first slice adds exactly one first-class primitive, `Lifecycle`, derived from the maintained ATS candidate pipeline and commerce order flow. Milestones 0–5 are done.

Lifecycle owns one Entity enum state field, its initial state, and a reachable canonical transition graph. The compiler validates Action bindings and optional Policy-specific graph subsets with stable `BEAN-E2202`/`BEAN-E2201` diagnostics; immutable AppIR, capabilities, canonical schema, named inspection, references, and semantic diff expose the same model through the shared CLI/MCP Agent Protocol. Create Actions inject the initial state, generic and transaction updates cannot change it, and only a Lifecycle-bound transition path can follow the graph after existing Policy checks.

ATS candidates and commerce orders now use the shared primitive with no application-name branch. DemoSeed creates Lifecycle records at their initial state, preflights every generated transition path and typed Action input before writes, rejects transaction side effects or unmodeled field mutations, and reaches deterministic generated states through compatible Actions. Focused compiler, AppIR v2 compatibility, Action, release/restart, DemoSeed, Agent Protocol, CLI/MCP parity, HTTP, React, and complete Go tests pass; legacy Action-local transitions and AppIR v1 releases without Lifecycle semantics remain covered. `make check` passes including race, black-box contracts, 25 React tests, and 14/14 Playwright journeys; `make test-crash`, `make test-postgres`, and `make build` pass, and the binary reports `bean 0.9.0-alpha`. All actionable review findings are fixed, answered, and resolved. Ownership, auditability, soft deletion, terminal-state immutability, rules, generated tests, and extensions remain outside v0.9.

## v0.8 completed state

Bean v0.8 Agent Protocol is complete from merge commit `f281bae`. The frozen contract has ten provider-neutral operations across Definition, Release, and Application Planes, with CLI as the reference transport and a local MCP stdio adapter over the same dispatcher. Plane grants are host configuration; Application operations retain View-read and Action-write Policy boundaries.

Milestones 0–5 are done. Existing v0.6 commands and generic `agent call` invoke the shared dispatcher. MCP targets current `2026-07-28` stateless metadata plus maintained initialization compatibility, filters discovery by process grants, handles standard liveness and malformed-request behavior, and keeps stdout JSON-RPC-clean. All ten handlers, exact CLI/MCP result parity, independent transport authorization, strict authority inputs, non-initializing View reads, SQLite/PostgreSQL View/Action owner/tenant behavior, and raw Entity/mutation bypass refusal have focused evidence.

Local terminal qualification passes: `make check` including race, black-box CLI, and 14/14 Playwright journeys; `make test-crash`; `make test-postgres` including Agent Protocol parity and the PostgreSQL blog journey; and `make build`. The real binary reports `bean 0.8.0-alpha`; generic CLI validation and Definition-only MCP discovery produce clean versioned machine output. PR CI passes and the latest implementation review reports no major issues after all findings were fixed, answered, and resolved.

## v0.7 completed state

Bean v0.7 Demo Factory is complete on top of the v0.6 machine contract. Typed Theme/DemoSeed metadata, generic Metric/Timeline/Search presentation, ten inspectable ordinary-definition patterns, deterministic Action-based seeding, checksummed SQLite packaging, and the populated ATS reference slice have executable evidence. CRM and tracker also validate and package with deterministic datasets.

Milestones 0–5 are done. The repository gates pass and benchmark documentation records the honest qualification boundary; prepared-definition/package timings are not substituted for repeated external-agent p50/p90 North Star evidence. Provider-specific agents, MCP, hosted sharing, Lifecycle semantics, rules, generated semantic tests, and extensions were outside v0.7.

## v0.7 implementation evidence

- The ATS has 23 metadata definitions and seed `42` produces 76 coherent records with stable semantic source and dataset checksums.
- Focused compiler, HTTP, Action, seed, pattern, CLI package, React, ATS browser, and source-independent packaged-binary browser tests pass.
- The ten patterns compile independently and `pattern inspect` returns their ordinary definitions and declared capabilities.
- Package verification detects artifact tampering; a failed replacement leaves the prior package manifest untouched.
- `make check` passes vet/lint/typecheck, 25 React tests, all Go/race/compatibility/black-box contracts, and 14/14 Playwright journeys. `make test-crash`, `make test-postgres`, and `make build` also pass; `bin/bean version` reports `bean 0.7.0-alpha`.

## v0.6 completed state

The real binary supports the full init/capabilities/schema/validate/inspect/plan/diff/publish/test loop with human output or a versioned `bean.cli/v1alpha1` JSON envelope.

Stable diagnostic families, source-relative paths, compiler-derived candidates, credential redaction, canonical generated schemas, normalized inspection, read-only target planning, semantic AppIR diff, exact-source draft replacement, atomic release activation, and isolated SQLite restart smoke tests have focused evidence. Draft 2020-12 validation accepts all nine maintained applications and rejects invalid manifest/definition shapes. The black-box applicant-tracking harness repairs unknown Entity, unknown field, and invalid transition defects without parsing English messages, then publishes and proves a zero-change diff.

## v0.6 verification record

- `make check` passes vet, frontend lint/typecheck and 24 React tests, all Go tests, focused contracts, fuzz-smoke, AppIR compatibility, JSON-only repair black-box, race, and 12/12 Playwright workflows.
- `make test-crash` passes both supported publication crash/restart points with canonical application sources.
- `make test-postgres` passes reusable PostgreSQL DBAL/HTTP contracts and the complete blog browser journey on PostgreSQL 17.
- `make build` passes and `bin/bean version` reports `bean 0.6.0-alpha`.

## Roadmap baseline

- North Star: an agent publishes a credible prompt-defined demo with p50 under five minutes, p90 under ten minutes, no human modification after the initial prompt, and 100% of the required behavior rubric under a recorded benchmark protocol.
- v0.6 establishes the deterministic machine contract; it does not embed an LLM or add MCP.
- v0.7 builds the populated, themed, packageable Demo Factory after the machine contract is stable.
- v0.8 adds a provider-neutral Agent Protocol with separate Definition, Release, and Application Planes; v0.9 begins evidence-driven semantic primitives with first-class `Lifecycle`.
- v0.10 adds bounded, side-effect-free deterministic rules between first-class semantics and external effects.
- v0.11 adds first-class Semantic Test Suites for deterministic Rule and Action behavior; v0.12 generates tests through that contract; v0.13 adds the typed extension boundary.
- v1.0 qualifies one explicit envelope: a single Bean application process backed by managed PostgreSQL and external object storage.
- Realtime infrastructure, a functions platform, broad OAuth, Redis/messaging abstractions, arbitrary visual design, Kubernetes machinery, and embedded AI chat remain deferred.
- Bean owns application semantics; providers own infrastructure capabilities.

## Asana Lite (completed)

The accepted slice is a local anonymous project/task application with generic status-board, arbitrary-depth tree, and transactional small-file attachment primitives; application behavior remains metadata-only under `examples/asana`.

- The generic `file` field accepts only bounded multipart input, persists base64 content plus safe metadata in the Action transaction, cleans replacement/hard-delete blobs, and policy-checks live references before download.
- Compiler-validated `board` presentation groups enum states and invokes a same-Entity transition Action; `tree` presentation renders a selected many-to-one self relation with arbitrary-depth expand/collapse behavior.
- Anonymous-only compiled applications suppress authentication navigation without weakening protected application behavior.
- `examples/asana` contains 43 definitions grouped across access, projects, tasks, attachments, and pages. Root task creation is project-bound; subtask creation derives project identity from its immutable parent binding.
- Focused field, Action, HTTP, compiler, React, and all Go tests pass. The dedicated Playwright journey passes project creation, board movement, three nested task levels, multipart upload, and byte-identical download.
- `make check` passes, including race tests and all 12 Playwright workflows; `make build` passes with the updated embedded frontend. The Asana Lite goal is complete.

## Reviewable application sources (completed)

Applications use a small manifest plus optional explicit feature-oriented YAML resources; definition documents are flat and CLI diagnostics retain file, line, and column information.

## Reviewable application sources

- The loader supports inline multi-document definitions and explicit local resources, rejects unsafe or duplicate inclusion, and flattens source into the existing canonical Bundle.
- `bean app validate --file` and import report syntax, schema, duplicate, and compiler diagnostics before database mutation.
- All eight examples use the new format; blog's 67 definitions are grouped into navigation, access, taxonomy, posts, and comments resources.
- Focused loader, compiler, CLI, Action, and HTTP tests pass. `make check` passes with all 11 Playwright workflows, and `make build` passes.

## Previous completed work

The Bean-owned frontend is standardized on source-owned shadcn/ui components. Tailwind v4, shared Bean tokens, primitive wrappers, Shell/Auth, public metadata rendering, Application Admin, System Admin, and Studio are implemented and qualified. Accessible AlertDialog confirmations replace browser/custom confirmation dialogs, and lint prevents raw interactive-control regressions outside the primitive directory.

## shadcn/ui verification record

- Frontend lint, TypeScript, production build, and all 9 React tests pass.
- Playwright runs 11/11 workflows, including a mobile shadcn integration journey covering Admin cards, generated form controls, record confirmation, overflow, and Studio.
- `make check`, `make test-blog`, `make test-postgres`, and `make build` pass; the PostgreSQL gate includes the complete blog browser journey.
- The generated frontend remains embedded in `bin/bean`; the current assets are approximately 394 KB JavaScript and 48 KB CSS before compression.

Generic scoped resource lists remain complete. The metadata-driven `resource-list` Block reuses AdminResource presentation and Actions while enforcing immutable parent bindings and allowlisted interactive filters; the blog acceptance route is `/blog/:id/comments`.

## Scoped resource-list verification record

- Compiler and HTTP contracts cover resource references, typed defaults, immutable parent scope, filter allowlists, collision rejection, and member denial.
- React tests cover AdminResource reuse and default filter transport.
- The Playwright blog journey proves two posts cannot leak comments across scoped routes and exercises approval plus status filtering.
- The isolated staged tree passes `make check`; `make test-blog`, `make test-postgres`, and `make build` pass on the integrated worktree.

## v0.5 completed state

Bean v0.5 complete-blog vertical slice is implemented and qualified as `0.5.0-alpha`. The metadata-only `examples/blog` application covers draft/publish posts, categories, many-to-many tags, opt-in local-password signup/login, authenticated comments, editor approval/rejection, public list/detail/category/tag pages, and RSS.

The generic platform additions are a compiler-known sensitive registration Action boundary, server-recomputed route-bound View/Webform inputs, slug and transaction-time bindings, metadata-driven content presentation, conservative rich-text rendering, dependency-ordered relation migrations, and portable to-many View hydration. Core Go and React code contains no blog-name branches.

## v0.5 verification record

- Compiler/HTTP contracts prove fixed-role sensitive signup, safe output/idempotency, independent throttling, CSRF, draft and pending/rejected non-leakage, member denials, binding tamper rejection, publication/moderation audit, and rich-text sanitization.
- React unit tests cover public rendering, inert rich text, session-aware navigation, and Webform visibility; the frontend has 8 passing unit tests.
- `make test-blog` passes the complete SQLite browser journey.
- `make test-postgres` passes reusable DBAL/HTTP contracts plus the same complete browser journey on PostgreSQL 17.
- `make check` passes vet, frontend lint/typecheck/tests, all Go tests, focused contracts, fuzz-smoke, compatibility, black-box version, race, and Playwright 10/10.
- `make test-crash` passes both supported crash/restart points.
- `make build` passes and `bin/bean version` reports `bean 0.5.0-alpha`.

## v0.4 verification record

- `go test ./...` — pass on 2026-08-30.
- Frontend lint, typecheck, and 6 unit tests — pass.
- Focused no-JSON Studio Playwright acceptance — pass.
- `make test-crash` — pass for crash after migration and after activation commit.
- `make test-postgres` — pass against PostgreSQL 17 for reusable DBAL and Admin/Action/View HTTP parity.
- `make check` — pass, including vet, frontend gates, all Go tests, fuzz-smoke, compatibility, black-box, race, and Playwright 9/9.
- `make build` — pass; output is `bin/bean` (`bean 0.4.0-alpha`).
