# Goal: verify Community in the browser

Status: complete

Complete the Community example browser flow: expose the existing publish_post Action in Admin and render its existing public_feed View at `/`, so Application navigation and member login no longer land on a 404. Preserve private-post isolation and keep the change in example metadata. Show Body, Visibility, and Updated at in the Post Admin table, with Body linking to the record and no ID column.

## Contract

- Render the published application name in the shared header when no Theme display name is configured; verify manifest, frontend, and the live Community browser.

- Preserve existing Blog and ATS demos and unrelated edits.
- Use local test accounts and records; preserve the View-read/Action-write and publication lifecycle contracts.
- Use a page Display on public_feed for the homepage, with an empty state and bounded pagination.
- Fix any confirmed defect and complete `make check` and `make build`.

## Evidence

- Community runs at `http://127.0.0.1:8083/admin` using `tmp/community-browser.db`.
- Member A created a private Post; member B saw no records and received Record not found on its direct Admin URL. After A published through the newly exposed Action, B saw the public Post and created a like Reaction.
- Community metadata now exposes Publish post in Admin and a paginated public_feed page Display at `/`; README documents administrator/member setup and the public homepage.
- Community validation and semantic tests pass. The existing member-only API regression is preserved alongside separate member/editor identities for the new Admin browser regression; both pass.
- The homepage regression first reproduced HTTP 404, then verified HTTP 200, the empty feed, and public-only results for both anonymous visitors and the post owner. All three Community regressions pass.
- The local Community release is activated and the homepage is verified in the browser before and after sign-out.
- The Post table shows Body, Visibility, and Updated at without ID; clicking Body opens the correct record in the browser.
- Final `make check` passes, including 87 frontend tests and 20 Playwright journeys; `make build` passes.
- The header now defaults to the published app name, preserving explicit Theme branding. Community release 6 renders Community in the live Admin header; final checks pass with 90 frontend tests and 20 browser journeys, and the build passes.
