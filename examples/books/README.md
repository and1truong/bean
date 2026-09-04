# Books

This maintained example proves scoped, hierarchical, route-backed Menu navigation.

- `site_navigation` is a global typed Menu.
- `book_contents` derives one Menu instance per `book` record.
- `page` records are reusable typed targets and resolve through the `page_records/detail` Page display.
- The `/books/:id` Page mounts the scoped Menu as a normal Block; record destinations preserve the Menu context in the generated route.
- Page create/update Actions accept the reserved `_navigation` submission. Record fields and placements commit or roll back together.

## Run it

From the repository root:

```bash
./bin/bean init --db ./tmp/books.db --admin-email admin@example.test --admin-password test-password
./bin/bean demo --app books --db ./tmp/books.db --addr 127.0.0.1:8080
```

Open <http://127.0.0.1:8080/>. Sign in at <http://127.0.0.1:8080/login> with `admin@example.test` / `test-password`, then use <http://127.0.0.1:8080/admin> to create Books and Pages and assign each Page to one or more Book contents menus.

Opening a Book in Admin shows its authorized Book contents tree. **Add Page** opens `/admin/books/:bookID/create/pages?menu=book_contents`; this contextual form fixes the current Book and Menu while allowing an optional parent, bounded weight, and label override. It still runs `create_page`, and a successful create returns to the Book with the new Page visible in the tree. The ordinary `/admin/pages/new` form remains available when a Page should be placed through the target-side Navigation editor.

The Admin record and contents are composed within one server request. Its resolved Book snapshot is request-local and read-only; separate browser requests resolve independently. Creation never trusts that GET snapshot: `_navigation` re-reads and authorizes the Book inside the Action transaction before the Page and placement commit together.

A replacement submission contains a bounded `placements` array. Omit `_navigation` to preserve current placements; submit an empty `placements` array to remove the target from all eligible Menu instances.

```json
{
  "title": "Installation",
  "body": "Install the application.",
  "_navigation": {
    "placements": [
      {
        "menu": "book_contents",
        "ownerId": "<book-id>",
        "parentId": "<parent-placement-id>",
        "weight": 20,
        "labelOverride": "Install"
      }
    ]
  }
}
```

Parents must already exist in the same Menu instance. A target appears at most once per instance, hierarchy is limited to three levels, and deleting a placement with children is rejected. Deleting a Page target or Book owner cleans up its placements in the same Action transaction.
