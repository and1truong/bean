# Typed field layout

Field layout groups record fields without splitting a record into Blocks or changing its query. The same bounded shape is supported by `AdminResource.form.layout` and a page/block detail Display's `renderer.layout`.

## Admin form

```yaml
form:
  fields: [title, slug, body]
  layout:
    groups:
      - name: content
        label: Content
        columns: 2
        fields:
          - {field: title}
          - {field: slug}
          - {field: body, span: full}
```

A supplied form layout must reference every `form.fields` field exactly once. When `form.fields` is omitted, the compiler's existing generated field list applies. Unknown, duplicated, readonly, or missing layout references fail compilation. Layout does not add Action inputs: derived fields are still omitted, protected lifecycle fields remain protected, and empty groups after control filtering are not rendered. The readonly footer, contextual Navigation editor, Actions, and history remain separate and unchanged.

## Readonly detail Display

```yaml
displays:
  record:
    type: block
    title: {field: title, fallback: Record}
    renderer:
      type: detail
      layout:
        groups:
          - name: publication
            label: Publication
            columns: 2
            fields:
              - {field: author}
              - {field: published_at}
          - name: content
            label: Content
            fields:
              - {field: body, span: full}
```

Detail layout fields must be stable, non-sensitive, non-redacted base-record fields selected by the View. Projected dotted relationship fields are deliberately deferred; existing title/body/meta presentations remain available for those cases. The layout may omit projected fields such as IDs or route inputs. It cannot be combined with renderer `titleField`, `bodyField`, `metaFields`, `linkRoute`, or `linkField`; use `Display.title` for the record heading. Non-detail renderers and JSON/CSV/RSS displays cannot use layout.

The existing authorized View result supplies every group; no field-specific query or Block is created. Plain values remain text; sanitized rich text and authorized file download links reuse existing render paths. Missing fields do not render, and an explicitly empty field never falls back to another field's content. Decimal and money strings are not converted through binary floating point by the layout renderer.

## Bounds and accessibility

- 1–16 ordered groups; 1–64 fields per group; at most 128 fields in total.
- Group names match `^[a-z][a-z0-9_]{0,63}$` and are unique within the layout.
- Group labels are nonblank, at most 120 Unicode characters.
- `columns` is `1` or `2`; omission normalizes to `1`.
- `span` is `single` or `full`; omission normalizes to `single`.
- Explicit null, zero columns, empty span, arbitrary CSS, nesting, and duplicate fields are rejected.
- Small screens always use one column. Two columns begin at the existing `48rem` viewport breakpoint. Full-row fields span available tracks.
- Source, DOM, keyboard, and screen-reader order are the same at every width. No dense placement or visual reordering.
- Forms use `fieldset`/`legend`; readonly groups use labelled sections and definition lists. Existing control labels, required markers, and validation associations are preserved.

## Authoring and compatibility

Studio has a **Field layout** editor for Admin resources and detail Displays: enable groups, edit names/labels/columns, select fields/spans, and move fields or groups with ordinary buttons. Compiler validation remains authoritative after changing membership. Enabling a detail layout replaces legacy renderer field roles; **Use legacy layout** removes the layout, after which legacy presentation can be authored again. The editor does not maintain a hidden copy of replaced roles.

CLI schema, capabilities, inspection, semantic diff, publication, restart, and packaged applications use the same compiled metadata. Layout is optional and absent on legacy forms/Displays, preserving their rendering. Field layouts require **AppIR v18**. v1–v17 remain readable with their original feature boundaries but cannot carry layout metadata. Sequence direction stays v15, Authentication stays v16, and Password Recovery stays v17. The branch is reconciled with committed Recovery `8b0129a`, but not merged into main; see the [compatibility report](reports/field-layout-compatibility.md). The private v15/v17 layout prototypes are not supported historical formats; rebuild disposable prototype artifacts from source with v18 rather than relabelling persisted snapshots.

## Maintained example and evidence

`examples/blog/posts.yaml` groups the Post form into Content and Classification. The original article route remains unchanged; `/posts/:slug/record` presents a grouped readonly projection through the same `published_post` View and publication Policy.

Focused compiler/schema/compatibility/inspection/diff tests, Admin/detail/Studio React tests, Blog create/edit/publication/browser geometry and keyboard tests, database reopen coverage, and a source-independent ATS package fixture exercise the capability. Combined qualification against committed runtime `8b0129a` passes `make check` (107 frontend tests and 26 browser journeys) and `make build` on the isolated branch. Form light/dark/mobile and readonly detail screenshots were reviewed.
