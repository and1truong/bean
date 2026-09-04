# Goal: Workspace Menu/content layout

Status: complete

Make the generic three-level `workspace` Menu compose navigation and route content as one responsive workspace.

## Contract

- Desktop keeps levels 1 and 2 above a lower two-column area.
- Desktop places the active level-3 `nav` on the left and the route content on the right.
- Narrow viewports place one labelled native `Section` select above route content and hide the duplicate level-3 link list.
- Route navigation remains links, `nav`, and `aria-current`; no ARIA tablist/tabpanel behavior.
- The behavior applies generically to View Page Displays and workspace Menu Blocks mounted with sibling content in Page/Panel composition.
- Flat Menus and workspace Menus without level 3 retain their existing usable flow.
- Source, DOM, keyboard, and screen-reader order remains navigation before content.

## Evidence

- React regressions cover View Page Display composition, Menu Block composition, flat Menus, and workspace Menus without level 3.
- Playwright measures desktop left/right geometry and mobile select-above-content geometry, then changes routes and verifies active state.
- The existing `examples/books` data remains unchanged.
- Completion requires visual verification, `make check`, and `make build`.
