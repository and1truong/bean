# Goal: Shadcn-styled route tabs

Status: complete

Adopt the visual language of shadcn Tabs for Bean workspace Menu levels without changing route-navigation semantics.

## Contract

- Menu levels remain semantic `<nav>` elements containing React Router links with `aria-current`; no `tablist`, `tab`, or `tabpanel` roles.
- Checked-in source-owned UI components follow ADR 0010 and reuse shadcn Tabs list/trigger styling over route links.
- Typed workspace Menus expose only the closed `default` and `line` variants; arbitrary classes, colors, dimensions, and per-item styles remain unavailable.
- Omitted variant normalizes to `default`; legacy flat Menus remain unchanged.
- Horizontal and vertical Menu levels preserve responsive layout, active trails, keyboard order, and mobile native-select behavior.
- AppIR compatibility remains explicit and immutable.

## Evidence

- Compiler/schema/AppIR/capability tests cover normalization, invalid variants, and compatibility.
- React tests cover both variants while asserting route links, `nav`, `aria-current`, and absence of ARIA tabs.
- Books browser coverage verifies styled active navigation without regressing desktop/mobile geometry or route changes.
- Completion requires visual verification, `make check`, and `make build`.
