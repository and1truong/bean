# Goal: Bean v0.16 Semantic Sequences

Status: complete

Implement [GitHub issue #6](https://github.com/and1truong/bean/issues/6) as an experimental vertical slice proving that an agent can describe a presentation semantically and Bean can validate, render, inspect, test, and package it deterministically.

The concrete benchmark is a slide deck. The reusable product primitive is a `Sequence`, not a PowerPoint clone:

```text
Prompt
  -> probabilistic agent
  -> ordinary Bean Sequence, Panel, Block, View, and Theme definitions
  -> deterministic compiler diagnostics
  -> immutable AppIR
  -> accessible HTML sequence
  -> inspect / repair / publish / package
```

## Product outcome

An agent or human author can build a coherent ten-frame technical presentation without writing React, CSS, HTML, JavaScript, SVG, PDF, or PPTX internals. Bean owns the structural contract, safe content vocabulary, layout constraints, navigation, visual consistency, and deterministic diagnostics.

The same `Sequence` composition seam may later support onboarding flows, walkthroughs, stories, proposals, reports, and kiosks. v0.16 implements only the `presentation` profile; it does not speculate about the behavior of those future profiles.

## Repository findings

- `Page`, `Panel`, `Region`, and `Block` already form the canonical composition tree. `View` Displays already render tables, metrics, charts, timelines, and other data-backed components.
- `Theme` already supplies closed presets and accent tokens. The React client already renders a server-produced, compiler-known node tree and explicitly rejects unknown components.
- The compiler registry already owns schema, AppIR storage, inspection, references, semantic diff, capabilities, and graph validation for Definition kinds.
- A Page owns one route and one Panel; it does not own an ordered multi-frame experience, keyboard navigation, progress, speaker notes, fixed-ratio composition, or print behavior.
- A text Block cannot express headings, bullets, quotes, code, callouts, accessible images, or simple diagrams without application-authored markup.
- Compiler diagnostics are fatal and machine-stable. There is no warning channel, so deterministic density limits must be publication-blocking constraints rather than advisory warnings.
- Bean has no PDF/PPTX engine, browser automation runtime dependency, asset provider, image generator, or web research capability. HTML is sufficient to prove the issue thesis.

## Product and architecture boundary

### New generic primitive: Sequence

`Sequence` owns only ordered experience semantics:

- one canonical route;
- static title and optional description;
- one closed profile, initially `presentation`;
- one closed aspect ratio, `wide` (16:9) or `standard` (4:3);
- an ordered non-empty list of named frames;
- each frame references one existing `Panel`, declares one semantic layout, and may include speaker notes.

A Sequence does not duplicate Page, Panel, Block, View, Action, Policy, or Theme. Panels still own regions and block placement. Blocks still own content/render capabilities. Views still own reads. Actions still own writes. Policy continues to gate referenced Blocks and Panels. Theme remains application-wide.

### Bounded semantic content

Add one generic `content` Block capability. It accepts an ordered list of compiler-known elements:

- `heading`
- `paragraph`
- `bullets`
- `quote`
- `code`
- `callout`
- `image`
- `diagram`

Elements carry only typed semantic fields. Images use an absolute application path or HTTPS URL and require alt text. Diagrams are an ordered labelled flow, not arbitrary SVG or graph layout. Content Blocks remain usable on ordinary Pages.

Data-backed charts, tables, metrics, timelines, and records continue to be ordinary View Blocks. Sequence rendering does not query storage directly.

### Presentation profile

The profile supplies deterministic client behavior:

- one visible frame at a time in screen mode;
- previous/next controls, frame picker, progress, Home/End and arrow-key navigation;
- addressable `?frame=<machine_name>` state;
- hidden-by-default speaker notes with an explicit toggle;
- correct headings and `aria-roledescription="slide"` semantics;
- responsive scaling without changing document order;
- print CSS that lays every frame on its own page and omits navigation/notes.

The server emits one `Sequence` render node containing ordinary Panel/Region/Block children. The frontend may render the declared semantics but may not invent content, query logic, authorization, or layout types.

## Definition contract

```yaml
apiVersion: bean/v1alpha1
kind: Sequence
metadata:
  name: review_process
spec:
  route: /presentations/review-process
  title: Repairing Code Review
  description: A deterministic Bean presentation
  profile: presentation
  aspectRatio: wide
  frames:
    - name: opening
      title: Repairing Code Review
      layout: title
      panel: review_opening
      notes: Establish the operational cost before proposing process changes.
    - name: comparison
      title: Current and proposed flow
      layout: comparison
      panel: review_comparison
```

Initial layouts are deliberately closed:

- `title`, `section`, `statement`, `bullets`, `quote`, and `closing` require a `single-column` Panel;
- `two-column` and `comparison` require a `two-column` Panel;
- `image-focus`, `chart-focus`, `table`, `timeline`, `process`, and `architecture` accept `single-column` or `two-column` Panels.

The compiler normalizes omitted profile/aspect ratio to `presentation`/`wide`. Frame order is source order and frame names are stable deep-link identities.

## Deterministic validation

Limits are intentionally conservative and shared by schema capabilities, compiler, runtime tests, and agent guidance:

- 1–50 frames per Sequence;
- unique machine-name frame identities;
- frame title at most 80 Unicode code points;
- speaker notes at most 4,000 bytes;
- 1–12 rendered Blocks per frame;
- at most 700 weighted content units per frame;
- at most 6 bullet items per element and 120 code lines per code element;
- image source and non-empty alt text are required;
- every referenced Panel/Block must exist and be visible through the ordinary composition graph;
- Sequence routes participate in the same duplicate/shadow/namespace validation as Page and View display routes.

Weighted content units are computed entirely from normalized metadata: visible text code points plus fixed costs for bullets, code lines, images, diagrams, and data-backed Blocks. Layouts may apply a documented deterministic budget multiplier. The compiler does not measure browser pixels or claim font-metric certainty.

Stable diagnostic families cover:

- unsupported profile, aspect ratio, layout, or content type;
- missing or duplicate frame identity;
- missing Panel/Block references;
- panel/layout incompatibility;
- title too long;
- frame too dense;
- table/chart/image placement incompatibility;
- image missing alt text or unsafe source;
- content element or resource bounds.

The diagnostics expose code, kind/name, exact path, message, source location, and candidates where relevant. An agent can repair a definition without reading Bean source.

## Security and determinism

- Content is rendered as React text nodes; no raw HTML, CSS, JavaScript, SVG, Markdown execution, template interpolation, or URL script scheme is accepted.
- HTTPS images are passive external resources supplied by the application author. Image generation, licensing, download, caching, and secrets remain provider concerns.
- View Blocks execute through the existing View service with existing Policy and field redaction. Sequence rendering adds no read bypass.
- Block and Panel Policy behavior is unchanged. An unavailable Block is omitted; an unavailable Panel makes its frame unavailable. The Sequence exposes neither hidden content nor hidden frame metadata to the actor.
- Sequence state is URL-only presentation state. No hidden agent state or user-specific persistence is introduced.
- Compilation, AppIR encoding, inspection, reference order, semantic diff, render tree, and diagnostic ordering remain deterministic.

## Reference application and agent benchmark

Create `examples/presentation/` as Bean's executable introduction. No current operational example can cleanly explain the runtime itself while also proving a ten-frame narrative.

The reference prompt is:

> Create a ten-slide presentation introducing Bean. Explain the product thesis, deterministic architecture, core definitions, agent workflow, Explore and observe-to-act loop, safety model, application lifecycle, reference applications, roadmap, and how to get started. Add speaker notes where useful.

The example must contain exactly ten Sequence frames and exercise title, statement, bullets, comparison, process, architecture, chart-focus, two-column, and closing layouts. A small deterministic Entity/View dataset summarizes Bean's shipped capability areas so one frame proves that presentation data still flows through View semantics. All explanatory narrative is ordinary `content` Blocks.

Automated evidence must prove:

- the prompt rubric maps to inspectable Sequence/Panel/Block/View definitions;
- identical definitions produce identical AppIR and render trees;
- a seeded invalid deck returns stable repair diagnostics, and the repaired deck validates;
- `/presentations/bean` renders ten accessible frames;
- keyboard, buttons, frame picker, deep links, notes, progress, responsive bounds, and print markers work;
- the example validates, publishes, restarts without source, and packages;
- no application-specific name appears in core packages.

## Milestones

### M0 — Contract and fixtures

Freeze the architecture above, archive v0.15, add failing compiler/AppIR/React/browser fixtures, and register the exact benchmark rubric.

### M1 — Canonical Sequence and content metadata

Add `bean/appir/v8`, `Sequence`, SequenceFrame, content elements, schema, capabilities, inspection, references, semantic diff, normalization, compatibility, and route ownership. Existing v1–v7 releases remain readable; only v8 may contain Sequence or semantic content fields.

### M2 — Deterministic diagnostics

Implement structural, reference, URL, layout compatibility, content safety, and weighted-density validation. Prove stable paths/codes, deterministic ordering, bounded inputs, and repair without browser measurement.

### M3 — HTML presentation runtime

Render Sequence -> Panel -> Region -> Block through the existing tree. Add accessible navigation, URL frame state, keyboard support, notes, progress, responsive aspect ratio, and print CSS. Keep ordinary Page rendering unchanged.

### M4 — Reference agent vertical slice

Add the ten-frame Bean introduction, a grouped capability View/chart, deterministic fixtures, prompt rubric, invalid-to-valid repair fixture, browser flow, package/restart evidence, and documentation for human/agent authors.

### M5 — Qualification

Cut `0.16.0-alpha`, regenerate schemas/frontend assets, validate every example, run compatibility and release tests, update roadmap/progress/docs, and pass all terminal gates.

## Acceptance criteria

- `Sequence` is an ordinary inspectable, diffable, testable, versionable Bean definition and is stored in immutable AppIR v8.
- A Sequence composes existing Panels and Blocks rather than embedding arbitrary layout or application code.
- Content Blocks are reusable on ordinary Pages and reject executable markup and unsafe images.
- The ten-frame reference application is coherent, keyboard-accessible, deep-linkable, print-ready, and fully metadata-driven.
- Invalid density, title, layout, image, route, and resource cases fail deterministically with exact repair paths.
- Existing Views, Actions, Pages, Displays, examples, AppIR formats, release restart, SQLite, and PostgreSQL behavior do not regress.
- Capabilities, schema, inspect, references, semantic diff, CLI, and Agent Protocol expose the same vocabulary.
- `make check`, `make test-crash`, `make test-postgres`, and `make build` pass.

## Explicit non-goals

- A PowerPoint clone, WYSIWYG canvas, freeform coordinates, arbitrary templates, animation timelines, transitions, collaborative editing, comments, or presentation analytics.
- Embedded LLMs, prompt execution inside Bean, web research, image generation, asset licensing, or provider-specific agent behavior.
- Raw HTML, Markdown execution, CSS, JavaScript, SVG, Mermaid, arbitrary diagram code, or arbitrary URLs.
- Pixel-perfect browser-layout prediction or font-metric validation. Density checks are deterministic metadata bounds, not claims about every renderer.
- Native PDF/PPTX generation, editable PPTX, chart image rasterization, email delivery, public sharing, or hosted presentation infrastructure.
- New onboarding/walkthrough/kiosk profiles before a separate example proves their semantics.
- A second Theme, View, Page, Panel, Block, Policy, Action, release, or agent runtime.

## Risks and decisions

- Fixed semantic budgets may reject usable content or accept visually awkward content. Keep the algorithm simple, documented, deterministic, and repairable; do not disguise heuristics as pixel measurement.
- Remote images can fail independently. The image element must preserve alt text and a stable broken-image state; offline packaging of remote assets is deferred.
- Data-backed View Blocks may load after the frame shell. Existing loading/error/Policy states remain authoritative and must fit within frame overflow bounds.
- Print output varies by browser. v0.16 owns print structure and page breaks, not byte-identical PDF output.

## Terminal gates

```bash
make check
make test-crash
make test-postgres
make build
```
