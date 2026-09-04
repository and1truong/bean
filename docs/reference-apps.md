# Reference applications

- `cms`: draft/publish news with page, JSON, CSV, RSS, and scheduled publishing metadata.
- `crm`: owned contacts/deals, manager bypass, Policy-scoped count/money dashboards, exact aggregate drill, and audited transitions.
- `commerce`: inventory, checkout Webform, deterministic payment, order Lifecycle, typed order-notification Extension, and a paid-unfulfilled observe → drill → `advance_order` dashboard.
- `tracker`: issues/comments, status chart-to-record drill, operational board, selected `move_issue`, and audit.
- `booking`: overlap-safe booking, cancellation, table/calendar Displays, and reminders.
- `saas`: provisioned team workspaces, tenant-scoped project lifecycle, dashboard/filter/drill, owner/member Actions, and isolated JSON/CSV exports.
- `community`: profiles/posts/comments/reactions/follows with private-to-public transitions.
- `blog`: draft/publish posts, categories, many-to-many tags, opt-in member signup, bound comments, post-scoped moderation queues, safe Markdown content filtering, public rendering, and RSS.
- `asana`: split YAML, anonymous local projects, immutable project context, typed task page filters/status chart, board movement, arbitrary-depth trees, and file attachments.
- `ats`: the primary Explore application with recruiting metrics/charts, shared filters, exact candidate drill, table/cards/board/timeline/calendar-compatible patterns, Lifecycle Actions, Rules, TestSuites, and deterministic DemoSeed.
- `presentation`: a ten-frame, five-chapter introduction to Bean using horizontal `next` and vertical `down` Sequence navigation, semantic content Blocks, speaker notes, deterministic capability data, and a real grouped View/chart.

Every application uses the same manifest and flat YAML definition format. Small examples keep definitions inline; larger examples use explicit feature-oriented resources. Core Go and React code contains no application-name branches.
