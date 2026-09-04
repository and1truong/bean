# Goal: Modernize Bean Blog composition

Status: complete

Update the maintained metadata-only Blog example to exercise named View Displays and the current responsive Page/Panel composition contract, with readable article widths and Policy-aware discussion layout, while preserving editorial, publication, identity, taxonomy, moderation, and content-safety behavior.

## Design

- Move the remaining legacy taxonomy and comment `Block.presentation` metadata into named View-owned Displays.
- Compose the post route from a `contained` article section followed by a `wide` responsive discussion section.
- Render discussion as `main-sidebar`: approved comments in `main`, member comment form in `sidebar`.
- Mark the member-only sidebar `collapseWhenEmpty`; anonymous users receive an expanded full-track discussion Region after server-side Policy filtering.
- Keep sidebar introduction in a named content Block with the same member Policy so unauthenticated narrative cannot prevent Region collapse.
- Use inline semantic content for public, Page-local headings and explanatory copy.
- Assign semantic section widths to simple public and administrative Pages without introducing arbitrary CSS or new runtime capabilities.
- Preserve all route/context bindings, View reads, Action writes, Policies, content filtering, and serialized displays.

## Acceptance criteria

- Category index/result, tag index/result, and approved-comment renderers are named Displays owned by their Views.
- No Blog View Block relies on legacy `presentation` metadata.
- The home, taxonomy, signup, and moderation routes use explicit ordered sections with appropriate `contained` or `wide` widths.
- The post route renders article content at `contained` width and discussion at `wide` width.
- At large widths an authorized member sees comments and the form in `2:1` main/sidebar tracks; at narrow widths they stack in source order.
- An anonymous reader receives no comment-form Region and the approved-comments Region expands across the discussion Panel.
- Existing draft isolation, publication, Markdown sanitization, RSS/JSON/taxonomy reads, registration, comment moderation, and anonymous approved-comment visibility remain green.
- Browser evidence covers semantic widths, responsive tracks, source order, and Policy-aware collapse.
- `make check` and `make build` pass.

## Non-goals

- New Panel, Page, Display, Policy, Lifecycle, Action, file, database, or AppIR capabilities.
- Homepage taxonomy sidebar, full-width hero/media, arbitrary widths, custom breakpoints, or visual reordering.
- Migrating post/comment transitions to Lifecycle or adding semantic TestSuites.
- Webform cache invalidation, comment editing/deletion, reactions, notifications, or search.
- Changes to SQL, SQLite, PostgreSQL, or migrations.
