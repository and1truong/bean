# Admin dark-theme controls

| Milestone | Deliverable | Evidence | Status |
| --- | --- | --- | --- |
| 0 | Reproduce inherited light colors in dark Admin | regression failed with light Delete color; screenshot also shows the light Actions background and unreadable current breadcrumb | done |
| 1 | Apply theme at the document scope and verify both modes | Chromium colors/screenshots pass in both modes; `make check` (90 frontend, 21 browser tests) and `make build` pass; live Community reloaded | done |
