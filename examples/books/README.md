# Books

This maintained example proves scoped, hierarchical, route-backed Menu navigation.

- `site_navigation` is a global typed Menu.
- `book_contents` derives one Menu instance per `book` record.
- `page` records are reusable typed targets and resolve through the `page_records/detail` Page display.
- The `/books/:id` Page mounts the scoped Menu as a normal Block; record destinations preserve the Menu context in the generated route.
- Page create/update Actions accept the reserved `_navigation` submission. Record fields and placements commit or roll back together.

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
