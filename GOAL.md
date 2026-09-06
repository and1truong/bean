# Goal: extended semantic content and composition

Status: complete. All six vertical slices in `PLANS.md`, generated-schema parity, focused tests, `make check`, and `make build` pass.

Authors can declare ordered lists, safe links, dividers, heading levels, literal static tables, image/audio/YouTube/playlist media, Tabs Blocks, and single-choice `choices`. Every feature follows definition → compiler validation and normalization → migration → immutable AppIR → render projection → React → atomic activation.

## Architecture and scope

- `ordered_list`, `link`, `divider`, `table`, `audio`, `youtube`, `youtube_playlist`, and `choices` are ContentElement variants. `heading` gains `level`; `image` keeps its existing name, shape, URL acceptance, caption, and fallback behavior.
- Every ContentElement works in named `Block type: content`, inline `Panel.regions[].items[].content`, and tab content.
- Tabs are named `Block type: tabs` values referenced through existing Panel Block references. Each tab directly owns one bounded ContentElement list. Tabs are not ContentElements or definition kinds, cannot nest, and cannot contain Panel/Block references.
- Static tables contain literal strings only. View/Display remains the data-backed table path. Quiz state is browser-local and has no score, backend verification, persistence, Action, View, Entity, Webform, timer, navigation, shuffle, or multiple-choice mode.
- Excluded: executable HTML/Markdown/CSS/JS/SVG/Mermaid, arbitrary embeds/providers/iframes, scripts, SQL, generic recursive composition or quiz engines, file upload, media proxy/download/transcoding, provider APIs, analytics, and interaction persistence.

## Source and validation contract

- Enums are closed. Every new nested object rejects unknown fields. Required fields must be present with the correct type; explicit `null` is not omission. Defaults apply only to omitted fields, never empty enum values.
- Non-blank means `strings.TrimSpace(value) != ""`. Text limits count Unicode code points on source values. Validated text is preserved except `choices.explanation`: omitted, empty, or whitespace-only becomes `""`; other values remain unchanged.
- New machine IDs match `^[a-z][a-z0-9_]*$`, contain 1–64 code points, are not trimmed/lowercased/generated, and are unique only within their owning container. Mounted DOM IDs and radio group names include an instance namespace.
- Every content list contains 1–12 elements. Existing bounds and behavior for the original eight variants remain unchanged except heading levels.
- Fields are valid only on their owning variant, even when empty. Legacy fields outside this scope are not tightened. React renders markup characters as literal escaped text.
- Presence/type/forbidden-field checks inspect source before Go zero values erase omitted/null distinctions. Existing decode errors and deterministic `BEAN-E2881` semantic-content diagnostics keep exact kind/name/path and source location; no parser or diagnostic framework is added.

## Content contracts

- `heading`: required `type`, `text`; optional integer `level` in `2|3|4`, default `2` for new source. Historical omitted levels render as 2 without snapshot mutation.
- `ordered_list`: required `type`, `items`; 1–6 non-blank strings, each ≤240 runes.
- `link`: required `type`, non-blank `label` ≤120 runes, and safe `target` 1–2048 characters; optional `openIn` in `same_tab|new_tab`, default `same_tab`.
- `divider`: required `type` only.
- `table`: required non-blank `caption` ≤120 runes, 1–6 required `{id,label}` columns with unique machine IDs and non-blank labels ≤80 runes, and 1–12 string-array rows. Each row width equals column count; each cell ≤240 runes and may be empty. Optional `rowHeader` in `none|first`, default `none`; first cells must be non-blank when `first`.
- `audio`: required safe `source`, non-blank `title` ≤120 runes, and non-blank literal `transcript` ≤4000 runes.
- `youtube`: required 11-character `videoId` matching `^[A-Za-z0-9_-]{11}$`, title, and transcript. `youtube_playlist`: required `playlistId` matching `^[A-Za-z0-9_-]{10,80}$`, title, and transcript. Provider URL/embed/player-control fields are forbidden.
- `choices`: required non-blank `question` ≤240 runes, 2–6 required `{id,text}` choices with unique machine IDs and non-blank text ≤120 runes, plus an `answer` referencing one choice. Optional explanation is ≤400 source runes and normalizes as above.

Safe link targets are absolute application paths beginning with one `/` or absolute HTTPS URLs with a hostname. They reject whitespace around the source, protocol-relative URLs, credentials, backslashes, controls, malformed percent encoding, encoded controls/backslashes, and decoded `.`/`..` path segments. Query and fragment are allowed for links. Audio uses the same policy but forbids query/fragment, including empty `?`/`#` delimiters. Validation performs no network request.

## Tabs, rendering, and lifecycle

- Tabs Blocks require `type: tabs`, non-blank `label` ≤120 runes, and 2–6 `{id,label,content}` tabs. IDs are unique machine IDs; labels are non-blank and ≤80 runes. `orientation` defaults to `horizontal` and accepts `horizontal|vertical`; `variant` defaults to `underline` and accepts `underline|pills`. Total content across tabs is ≤24 elements. Only standard Block metadata and optional `policy` are accepted.
- Semantic HTML is required: native headings/list/link/hr/table, caption/column and optional row headers; a named keyboard-scrollable table overflow region; radios in fieldset/legend; WAI-ARIA tabs using native buttons, roving focus, automatic orientation-aware activation, Home/End/wrap, focusable selected panel, and hidden inactive panels.
- Media begins with title, transcript, disclosure, and an explicit Load button. Audio mounts native controls only after Load. YouTube uses only `youtube-nocookie.com` validated-ID URLs and fixed `autoplay=0`, `controls=1`, `playsinline=1`, `cc_load_policy=1` parameters, strict referrer policy, minimal playback/fullscreen permissions, loading status, transcript, and canonical fallback. No preload/provider request occurs before Load.
- Leaving an active tab or Sequence frame resets quiz/tabs and unloads media; returning starts fresh. Ordinary rerenders and speaker-note toggles preserve state. Frame return resets Tabs to their first tab. Instances never share state. Print includes all frames/tabs as title, transcript/content, and fallback URLs without player controls/iframes.
- Sequence ignores already-handled keys and events from descendants of interactive controls, tabs/panels, contenteditable nodes, and keyboard-scrollable content regions. It prevents default only when it owns navigation.

## Compatibility, schema, capability, and density contract

- Introduce one new format after `bean/appir/v19`, retaining the v19 constant. Historical snapshots remain byte/shape compatible. Old formats reject every new variant/field/Tabs Block at named, inline, and tab seams; future formats remain unsupported.
- AppIR uses typed structs and slices for columns, rows, choices, and tabs. Exported fields keep PascalCase JSON convention; machine IDs use `json:"id"`. New optional serialization does not add empty fields to legacy elements.
- One discriminated closed ContentElement schema is shared by Block, inline Panel, and tabs. Schemas express enums/types/required/exclusion/bounds; descriptions identify semantic-only uniqueness, answer reference, row width, and total tab element constraints. Generated JSON is updated only through the generator.
- Capabilities publish the new variants, Tabs Block, enums, source policy, and bounds from shared constants.
- Weight: ordered list item runes +20/item; link label runes +20; divider 20; table caption/header/cell runes +20/column +20/row; audio/YouTube/playlist title+transcript runes +180; choices question/options/explanation runes +20/choice; Tabs label/tab-label runes + all tab content weight +20/tab. Existing weights and Sequence budgets stay fixed.

## Completion

All YAML seams, compiler diagnostics/defaults, schemas/capabilities, AppIR v20 compatibility, projection, accessible React behavior/lifecycle/print, metadata-only presentation examples, documentation, focused Go/Vitest/Playwright coverage, atomic activation regressions, `make check`, and `make build` pass. No unrelated changes are introduced.

Previous completed goal: Content Block authoring guide.
