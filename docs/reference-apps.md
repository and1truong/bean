# Reference applications

- `cms`: draft/publish news with page, JSON, CSV, RSS, and scheduled publishing metadata.
- `crm`: owned contacts, manager policy, activity, pipeline View, and audited transitions.
- `commerce`: inventory, checkout Webform, transactional decrement, deterministic payment, and order transitions.
- `tracker`: issues/comments, kanban View, transitions, and audit.
- `booking`: overlap-safe booking, cancellation, calendar, and reminders.
- `saas`: organisations, memberships, tenant-scoped projects, and automatic tenant predicates.
- `community`: profiles/posts/comments/reactions/follows with private-to-public transitions.
- `blog`: draft/publish posts, categories, many-to-many tags, opt-in member signup, bound comments, post-scoped moderation queues, safe Markdown content filtering, public rendering, and RSS.
- `asana`: anonymous local projects, status board movement, arbitrary-depth task trees, route-bound subtask creation, and multiple transactional file attachments.

Every application uses the same manifest and flat YAML definition format. Small examples keep definitions inline; the larger blog is split into explicit feature-oriented resources. Core Go and React code contains no application-name branches.
