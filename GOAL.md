# Goal: Asana Lite local application vertical slice

Build and qualify a metadata-only Asana Lite reference application for one local user, with no application login or registration, while adding only generic Bean primitives.

## Primary outcome

A user can run one embedded SQLite executable, create projects, manage root tasks on a status board, open task details, create and render subtasks at arbitrary nesting depths, and upload/download multiple task attachments.

## Acceptance criteria

- `examples/asana/app.yaml` and feature-oriented resources define all application-specific entities, Views, Actions, Webforms, Pages, Panels, Blocks, and menus; core code contains no Asana-specific branch.
- The public application does not advertise sign-in or registration when its compiled metadata has no authenticated application surface.
- Projects and tasks are created through public Webforms; every task belongs to one project and has title, description, status, priority, due date, parent, and sibling position metadata.
- A generic board presentation groups root tasks into compiler-validated status columns and moves cards only through a declared transition Action.
- A generic tree presentation renders a flat, project-scoped View as an expandable arbitrary-depth task hierarchy using compiler-validated id, parent, title, link, and order fields.
- Subtasks are created through an immutable parent route binding; the Action derives the project from the parent so clients cannot cross projects or create hierarchy cycles through the accepted UI.
- A generic `file` field and Webform element accept bounded multipart uploads. File bytes and metadata are persisted transactionally with the Action, downloads use generated identifiers and safe response headers, replacement/deletion cleans up referenced blobs, and filenames never become storage paths.
- Tasks support multiple attachments through a metadata-defined attachment Entity related to task; no attachment-specific behavior is hard-coded in core.
- Compiler, field, Action, HTTP, React, and Playwright evidence covers invalid metadata, file limits, anonymous use, board movement, deep nesting, and upload/download.
- Existing examples and compatibility guarantees remain green.

## Explicit limits

- One Bean process, one local user, and SQLite are the acceptance deployment; existing PostgreSQL contracts must continue to compile and pass their reusable tests.
- A project tree is bounded by the existing View limit of 200 tasks in this slice; server-recursive queries and virtualized trees are deferred.
- Board movement is accessible without requiring drag-and-drop. Pointer drag-and-drop and sibling reordering are optional follow-up UX.
- Attachments are bounded small operational files stored in Bean metadata storage, not external object storage, resumable uploads, media processing, or antivirus scanning.
- Authentication, registration, assignment to users, notifications, dependencies, timeline/calendar views, and destructive schema migration are out of scope.

## Terminal gates

```bash
make check
make build
```
