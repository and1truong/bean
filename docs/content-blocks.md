# Content Blocks

Use semantic content for small, ordered, static interface or presentation material. Bean renders a closed vocabulary as React elements. Text is literal and escaped; metadata never executes Markdown, HTML, CSS, JavaScript, SVG, Mermaid, scripts, SQL, or arbitrary embeds.

The same `ContentElement` contract is available in a named content Block, an inline Panel item, and each tab:

```yaml
kind: Block
name: release_path
type: content
content:
  - {type: heading, level: 3, text: "Release path"}
  - {type: ordered_list, items: [Define, Validate, Publish]}
---
kind: Panel
name: release_panel
layout: single-column
regions:
  - name: main
    items:
      - content:
          - {type: paragraph, text: "Inline content keeps Panel-local identity."}
      - block: release_path
```

Every content list has 1–12 elements. A Panel `items` entry contains exactly one `content` list or one `block` reference. Named Blocks may have Policy; inline and tab content inherit their enclosing Policy boundary.

## Element contract

Required means present, non-null, and of the declared type. Defaults apply only when a field is omitted. Enums are closed, new nested objects reject unknown fields, and fields belonging to another variant are rejected even when empty. “Non-blank” means the value has a non-whitespace character. Text limits count Unicode code points and validated source text is preserved.

| Type | Fields | Contract and HTML |
| --- | --- | --- |
| `heading` | required `text`; optional `level` | `level` is `2`, `3`, or `4`; default `2`. Renders the matching `<h2>`–`<h4>`. |
| `paragraph` | required `text` | Non-blank; renders `<p>`. |
| `bullets` | required `items` | 1–6 strings; renders `<ul><li>`. |
| `ordered_list` | required `items` | 1–6 non-blank strings, each at most 240 code points; renders `<ol><li>`. |
| `quote` | required `text`; optional `attribution` | Non-blank text; renders `<blockquote>`. |
| `code` | required `text`; optional `language` | Non-blank text, at most 120 lines; preformatted literal text, without code execution. |
| `callout` | required `text`; optional `tone` | Non-blank text. `tone` is `info` (default), `success`, or `warning`. |
| `image` | required `source`, `alt` | Keeps the historical image source contract. Native lazy `<img>` with async decoding, responsive layout, an `alt`-based caption, and readable fallback. |
| `diagram` | required `items`; optional `direction` | 2–8 string nodes. `direction` is `horizontal` (default) or `vertical`; renders a semantic ordered flow, without SVG. |
| `link` | required `label`, `target`; optional `openIn` | Label is non-blank and at most 120 code points. `openIn` is `same_tab` (default) or `new_tab`; renders an anchor/router link. New-tab links include `noopener noreferrer` and an accessible indication. |
| `divider` | only required `type` | Renders `<hr>`. |
| `table` | required `caption`, `columns`, `rows`; optional `rowHeader` | Literal strings only; see [Static tables](#static-tables). |
| `audio` | required `source`, `title`, `transcript` | Explicit loading with native audio controls; see [Media](#media). |
| `youtube` | required `videoId`, `title`, `transcript` | Explicit privacy-enhanced embed loading from a validated ID. |
| `youtube_playlist` | required `playlistId`, `title`, `transcript` | Explicit privacy-enhanced playlist loading from a validated ID. |
| `choices` | required `question`, `choices`, `answer`; optional `explanation` | Browser-local single-choice quiz; see [Choices](#choices). |
| `formula` | required `expr`, `text` | Bounded mathematical expression rendered as MathML with an always-visible text fallback; see [Formulas](#formulas). |

## Safe links and image sources

`link.target` is 1–2048 code points and accepts an absolute application path beginning with one `/`, or an absolute HTTPS URL with a hostname. Query strings and fragments are allowed. Bean rejects protocol-relative URLs, credentials, leading/trailing whitespace, backslashes, control characters, malformed percent encoding, encoded controls/backslashes, decoded `.` or `..` path segments, HTTP, and every other scheme.

`image.source` keeps its existing acceptance: an absolute application path or HTTPS URL, without credentials, query, or fragment; application paths cannot begin with `//`, contain `..`, or contain backslashes. External images are fetched by the browser and therefore disclose connection information to the image host.

## Static tables

```yaml
- type: table
  caption: "Read and write boundaries"
  columns:
    - {id: primitive, label: Primitive}
    - {id: responsibility, label: Responsibility}
  rows:
    - [View, Read]
    - [Action, Write]
  rowHeader: first
```

The caption is non-blank and at most 120 code points. A table has 1–6 `{id,label}` columns and 1–12 rows. Column IDs are unique machine IDs within the table; labels are non-blank and at most 80 code points. Every row is an array with exactly one string cell per column. Cells may be empty and are at most 240 code points. With `rowHeader: first`, the first cell in every row must be non-blank. The default is `none`.

Bean renders `<table>`, `<caption>`, `<thead>`, `<tbody>`, column headers with `scope="col"`, and optional row headers with `scope="row"`. Wide tables have their own named, keyboard-focusable scroll region so they do not widen the page. Static tables have no query, expression, storage binding, or client fetch path; use a View table Display for live data.

## Media

```yaml
- {type: audio, source: "/assets/intro.wav", title: "Bean introduction", transcript: "A text equivalent of the recording."}
- {type: youtube, videoId: "M7lc1UVf-VE", title: "Player demonstration", transcript: "A text equivalent of the selected video."}
- {type: youtube_playlist, playlistId: "PLBCF2DAC6FFB574DE", title: "Learning playlist", transcript: "A text equivalent of the selected playlist material."}
```

Media titles are non-blank and at most 120 code points; literal transcripts are non-blank and at most 4,000 code points. `audio.source` uses the safe link policy but forbids query and fragment delimiters, including empty `?` or `#`. A video ID matches `^[A-Za-z0-9_-]{11}$`; a playlist ID matches `^[A-Za-z0-9_-]{10,80}$`. Media validation makes no network, HEAD, oEmbed, proxy, or provider request.

Audio starts with its title, transcript, disclosure, source fallback, and **Load audio** button. The browser receives no audio `src` until Load. Bean then mounts `<audio controls preload="none">`, without autoplay, looping, or custom controls, and reports loading/error status. The transcript and safe direct-source fallback remain available on error.

YouTube starts without an iframe, thumbnail, preconnect, or provider request. **Load YouTube video** and **Load YouTube playlist** create only these renderer-owned URLs:

- `https://www.youtube-nocookie.com/embed/{videoId}`
- `https://www.youtube-nocookie.com/embed/videoseries?listType=playlist&list={playlistId}`

Bean adds only `autoplay=0`, `controls=1`, `playsinline=1`, and `cc_load_policy=1`, a strict-origin-when-cross-origin referrer policy, responsive sizing, fullscreen, and playback permissions. Metadata cannot add iframe HTML, parameters, provider URLs, autoplay, camera, microphone, geolocation, or clipboard permission. The iframe load event means the frame loaded, not that playback succeeded, so the transcript and canonical **Open on YouTube** fallback remain visible.

`cc_load_policy=1` asks YouTube to show captions when the provider has them; it does not guarantee captions. See [YouTube player parameters](https://developers.google.com/youtube/player_parameters). The privacy-enhanced host does not mean tracking is absent after Load; see [privacy-enhanced embeds](https://support.google.com/youtube/answer/171780?hl=en). Bean retains the origin referrer required for embedded-player identity; see [embedded-player identity](https://developers.google.com/youtube/terms/required-minimum-functionality#embedded-player-api-client-identity).

Authors are responsible for transcript equivalence, including the selected playlist material. The compiler cannot verify transcript accuracy or provider captions. Leaving a Sequence frame or active tab pauses by unmounting audio/iframe state; returning requires Load again. Print includes titles, transcripts, and fallback URLs, without controls or iframes.

## Choices

```yaml
- type: choices
  question: "Which primitive owns writes?"
  choices:
    - {id: view, text: View}
    - {id: action, text: Action}
  answer: action
  explanation: "Views read; Actions write."
```

The question is non-blank and at most 240 code points. There are 2–6 `{id,text}` choices in source order. IDs match `^[a-z][a-z0-9_]*$`, contain 1–64 characters, and are unique within the quiz. Text is non-blank and at most 120 code points. `answer` references one ID. Explanation is at most 400 source code points; omitted, empty, or whitespace-only values normalize to `""`, while other text is preserved.

Choices use native radios in `fieldset`/`legend`. Selection does not grade until **Check answer**. Checking locks the radios, emits text feedback through a polite atomic live region, and shows non-empty explanation; **Try again** resets and focuses the first radio. State is isolated per mounted instance, has no fetch/storage/cookie/URL/backend path, and resets after leaving its frame/tab or replacing quiz content. The correct answer is intentionally present in the client payload.

## Formulas

```yaml
- type: formula
  text: "x equals (−b ± √(b² − 4ac)) / 2a"
  expr:
    kind: frac
    numerator:
      kind: group
      parts:
        - {kind: literal, text: "−b"}
        - {kind: literal, text: "±"}
        - kind: sqrt
          inner:
            kind: group
            parts:
              - {kind: sup, base: {kind: literal, text: "b"}, exponent: {kind: literal, text: "2"}}
              - {kind: literal, text: "−4ac"}
    denominator: {kind: literal, text: "2a"}
```

`expr` is a bounded, typed node tree — not TeX, markup, or a command language. `kind` selects the closed node vocabulary and its allowed fields; unknown kinds, foreign fields, and missing required children are rejected at compile time:

| Kind | Required fields | Optional fields | Renders |
| --- | --- | --- | --- |
| `literal` | `text` | — | One number, operator, or identifier token (`mn`, `mo`, `mi`). |
| `group` | `parts` | — | An `mrow` sequence of 1–24 nodes. |
| `paren` | `inner` | `style` | Stretchy delimiters: `round` `()`, `square` `[]`, `brace` `{}`, `abs` `\|\|` (default `round`). |
| `frac` | `numerator`, `denominator` | — | A fraction bar. |
| `sqrt` | `inner` | — | Radical. |
| `root` | `inner`, `index` | — | Indexed radical. |
| `sup` | `base`, `exponent` | — | Superscript. |
| `sub` | `base`, `subscript` | — | Subscript. |
| `func` | `name` | `argument` | A closed function set (`sin`–`tan`, `ln`, `log`, `exp`, `lim`, `min`, `max`, and related) rendered upright, optionally followed by an argument node. |
| `sum` | `lower`, `upper`, `body` | — | Summation with `∑` under/over scripts. |

An expression holds at most 48 nodes nested at most 8 deep, a `literal` is at most 40 code points, and `text` is a non-blank readable fallback of at most 400 code points. The fallback is always rendered visibly as the formula caption — it is the accessible name, the print representation, and the rendering on clients without MathML. The visual `<math>` tree is `aria-hidden`; node text is literal element content, so markup inside a literal stays text. Keep the fallback equivalent to the expression; the compiler cannot verify prose accuracy.

## Lessons

A `lesson` Block renders a worked example or guided explanation: a title plus 1–8 ordered `sections`, each a `{id, heading?, content}` step of the same closed element contract (including `formula`, `image`, `diagram`, `table`, and `choices`). All section content lists together hold at most 48 elements, and section IDs are unique machine IDs. `sections` and `title` are rejected on every other Block type, and other Block payload fields are rejected on `lesson`.

```yaml
kind: Block
name: blackboard_lesson
type: lesson
title: "Worked example: the quadratic formula"
sections:
  - {id: idea, heading: The idea, content: [{type: paragraph, text: "A quadratic has at most two roots."}]}
  - {id: formula, heading: The formula, content: [{type: formula, text: "x equals …", expr: {kind: literal, text: "x"}}]}
```

The block renders a blackboard-styled surface: dark board, chalk-colored ink, numbered sections in reading order, and content reusing the standard element renderers — so literal-text escaping, image fallback, table keyboard scrolling, and quiz behavior keep their existing contracts. The lesson inherits the enclosing Policy boundary and counts toward Sequence frame density like any other Block.

## Exact versioned reference

Run these commands against the Bean binary you deploy:

```bash
bean capabilities --json
bean schema Block --json
bean schema Panel --json
bean app validate --file ./app.yaml
```

The compiler enforces duplicate IDs, answer references, row widths, the 24-element total across a Tabs Block, section ID uniqueness, the 48-element total across a lesson, and formula node bounds because standard JSON Schema cannot express those relationships directly. See [Definitions](definitions.md#sequences-and-semantic-content) for Tabs and Sequence composition and [the presentation example](../examples/presentation/) for executable metadata, including a blackboard lesson frame.
