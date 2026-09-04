# Historical compiler fixtures for field-layout compatibility

These snapshots were emitted by **the compiler at each historical commit**, not constructed by changing the format of a v18 snapshot.

| Format | Commit | Snapshot SHA-256 |
| --- | --- | --- |
| v14 | `644455d` | `fa612ca556516b8a7132f332d7b55b3af92bcd386bd40d6cc2794c9a322f431c` |
| v15 | `b4749ff` (directional Sequence) | `b51f75ffe5abe80f1fec7f42f7d9059b52818083bde926416f210be0e50ec1f5` |
| v16 | `24295ae` (Authentication) | `dc4f6068f01ab4e923ffc6a6d09d1a0590a36067f6fcc13971717553503f6bd5` |
| v17 | `8b0129a` (Password Recovery) | `d34db58e077d29de654b6e7e66b72230b48cd7dcd5dd822f342ceb1c938dc65d` |

`field-layout-baseline-generator.go.txt` is the exact fixture generator. To reproduce, extract each commit with `git archive` into an isolated temporary checkout, copy the generator to `cmd/compat-snapshot/main.go` there, and run:

```sh
go run ./cmd/compat-snapshot > field-layout-baseline-vNN.json
go run ./cmd/compat-snapshot source > field-layout-baseline-vNN.source.json
```

The `.source.json` files hold the corresponding ordinary definitions. Every fixture contains one Entity, generated Admin/Actions, a role-based detail Display, and a two-frame Sequence. v14 frames omit direction; v15–v17 include `next` followed by `down`; v16/v17 have internal Authentication with registration disabled, and v17 additionally enables Password Recovery. None contains field layout.

AppIR tests prove exact decode/clone behavior and fail-closed feature boundaries. Release tests recompile the original definitions (allowing only the new format and v14's compiler-owned direction defaults), install the historical snapshot into an isolated release database, close/reopen, publish v18 layouts without physical migrations, and prove restart and rejected-publication isolation. Fixture release IDs are empty until the release test supplies a local ID; no business records or credentials are embedded.
