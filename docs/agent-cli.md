# Agent CLI contract

Bean v0.6 exposes the compiler and release lifecycle as a provider-neutral command contract. An agent does not need to inspect Bean source or parse human prose.

v0.11 retains the v0.8 generic one-to-one adapter over the shared `bean.agent/v1alpha1` dispatcher and adds first-class semantic TestSuites without changing the transport envelope:

```bash
bean agent call bean.definition.validate --input request.json --json
bean agent call bean.release.publish --input request.json --json
bean agent call bean.application.query --input request.json --json
```

The successful CLI `result` is identical to MCP `structuredContent`. Existing commands delegate to the same handlers and remain the preferred human-facing aliases. See [Agent Protocol](agent-protocol.md) for the ten operations, planes, and MCP transport.

v0.7 Demo Factory commands remain available:

```bash
bean pattern inspect workflow_resource --json
bean demo seed --file ./app.yaml --db ./demo.db --seed 42 --json
bean package --file ./app.yaml --output ./dist/demo --seed 42 --json
bean package verify --dir ./dist/demo --json
```

Pattern inspection returns required capabilities plus ordinary definitions; it does not install behavior. Demo seeding requires the same definitions to be active, writes through generated create Actions, and accepts a replay only when View reads match the generated dataset. Packaging is SQLite-only, stages and restart-checks the package before replacing an existing Bean package, and records source, dataset, executable, and database checksums in `bean.package/v1alpha1`.

## Supported loop

```bash
bean app init --dir ./ats --name "Applicant Tracking" --json
bean capabilities --json
bean schema --output ./schemas --json
bean app validate --file ./ats/app.yaml --json
bean app inspect --file ./ats/app.yaml Entity candidate --json
bean app plan --file ./ats/app.yaml --json
bean app diff --file ./ats/app.yaml --db ./ats/bean.db --json
bean app publish --file ./ats/app.yaml --db ./ats/bean.db --json
bean app test --file ./ats/app.yaml --json
```

`app init` refuses to overwrite an existing `app.yaml`. `app validate`, `app inspect`, and `app test` do not require a database. Without a target, `app plan` plans from an empty schema and `app diff` compares against an empty application. With `--db` or `--database-url`, plan and diff open an initialized Bean database without running metadata initialization, upgrades, migrations, or writes.

`app publish` validates and plans the source, replaces the draft with the exact source definition set, applies the existing additive migration lifecycle, and atomically activates immutable AppIR. It creates the selected SQLite database when needed. `app test` uses temporary SQLite databases to check source loading, compilation, migration planning, publication, restart activation, and every declared semantic TestSuite. It executes metadata-authored suites; generated suites remain deferred to v0.12.

## JSON envelope

Every supported command accepts `--json` and writes exactly one JSON document to stdout:

```json
{
  "apiVersion": "bean.cli/v1alpha1",
  "command": "app.validate",
  "ok": false,
  "diagnostics": [
    {
      "code": "BEAN-E2001",
      "kind": "View",
      "name": "candidates",
      "path": "spec.entity",
      "message": "references missing Entity canddate",
      "candidates": ["candidate"],
      "source": {"path": "views.yaml", "line": 4, "column": 1}
    }
  ]
}
```

`apiVersion`, `command`, `ok`, and `diagnostics` are always present. `result` is present on success. Diagnostics and result collections use deterministic ordering. JSON commands do not write ordinary logs or human summaries to stdout or stderr.

Exit statuses are:

| Status | Meaning |
| --- | --- |
| `0` | command succeeded |
| `1` | definitions, references, semantics, or migration contract were rejected |
| `2` | command usage was invalid |
| `3` | filesystem, database, or runtime operation failed |

## Diagnostic codes

Codes and structured fields are the compatibility interface. Human messages may improve without a compatibility change.

| Code | Family |
| --- | --- |
| `BEAN-E0002` | invalid command usage |
| `BEAN-E0003` | runtime or environment failure |
| `BEAN-E1001` | invalid source, manifest, resource, or YAML structure |
| `BEAN-E1002` | unknown source field |
| `BEAN-E1003` | missing required value |
| `BEAN-E1004` | duplicate field, resource, or definition |
| `BEAN-E1005` | incompatible definition API version |
| `BEAN-E1101` | unsupported definition kind |
| `BEAN-E1102` | invalid machine name |
| `BEAN-E2001` | missing definition reference |
| `BEAN-E2002` | incompatible definition reference |
| `BEAN-E2101` | missing or invalid Entity/View field reference |
| `BEAN-E2201` | invalid Action operation, state field, or transition edge |
| `BEAN-E2202` | invalid Lifecycle entity, state field, initial state, graph, or reachability |
| `BEAN-E2301` | invalid Policy or role contract |
| `BEAN-E2351` | invalid Rule expression, type, bound, or consumer contract |
| `BEAN-E2401` | invalid serializer, renderer, or presentation contract |
| `BEAN-E2501` | invalid context/input binding |
| `BEAN-E2601` | invalid Page route contract |
| `BEAN-E2701` | unsafe or incompatible migration contract |
| `BEAN-E2851` | invalid TestSuite target, case, fixture, context, assertion, or bound |
| `BEAN-E2900` | other typed definition semantic failure |
| `BEAN-T1001` | failed semantic TestSuite assertion |

Paths are canonical source paths such as `spec.fields` or `spec.transitions.screening`; source file paths are relative to the application manifest directory. `candidates` appear only when Bean can derive valid alternatives. Unknown fields use the decoded specification shape, missing definitions use the compiled symbol table, missing fields use the resolved Entity, and invalid transition edges use enum options.

Sensitive inputs, file bytes, secrets, passwords, and database credentials are excluded or redacted. Agents must not infer safety from a human message or echo supplied credentials into definitions.

## Schemas and capabilities

`bean capabilities --json` reports definition/API versions, definition kinds, semantic primitives, field types, Action operations and steps, Block types, presentations, serializers, layouts, database backends, and hard limits. In v0.11, `semanticPrimitives` contains `Lifecycle` and `Rule`; `ruleOperators`, `ruleSources`, and the node/depth/literal/value limits expose the closed evaluator contract. `testSuiteTargets` and the suite/case/fixture/encoded-byte limits expose the bounded TestSuite contract.

`bean schema [Kind] --json` returns canonical Draft 2020-12 JSON Schema. `bean schema --output ./schemas` writes `bean.schema.json` and one lower-case file per definition kind. The checked-in [schemas](../schemas) directory is generated from the same Go specification types used by compiler decoding; tests fail if those files drift or stop covering a maintained example.

JSON Schema describes document shape and rejects unknown properties. Cross-definition references and semantic constraints remain compiler responsibilities and are visible through diagnostics, capabilities, and `app inspect`.

v0.11 compilers emit `bean/appir/v4` so older runtimes reject TestSuite-bearing releases instead of silently discarding their semantics. The v0.11 runtime still loads `bean/appir/v1` releases without Lifecycle, Rule, or TestSuite semantics; `bean/appir/v2` releases with Lifecycle but without Rules or TestSuites; and `bean/appir/v3` releases with Rules but without TestSuites. Rule definitions and Rule-bound consumers remain invalid under v1/v2; TestSuites are invalid under v1/v2/v3.

## Inspection, plan, and diff

`app inspect` returns normalized compiled AppIR for the whole source or one `Kind name` pair. Named inspection also returns sorted, resolved definition references.

`app plan` returns the candidate checksum, AppIR format, ordered migration descriptions, and SQL statements. `app diff` recursively compares semantic AppIR and reports stable `add`, `remove`, and `change` operations using dot paths. Release IDs, release versions, generated OpenAPI, YAML formatting, and source layout do not create semantic changes.

Read-only commands produce equivalent normalized JSON for equivalent source and unchanged target state. Publication includes a generated release ID and is intentionally not byte-for-byte deterministic.

## Compatibility

`bean.cli/v1alpha1` permits additive result fields and new diagnostic codes. Existing field meanings, exit-status meanings, and diagnostic code meanings do not change within the API version. Removing or repurposing a field/code requires a new CLI API version. Message wording, whitespace in human output, and candidate ranking beyond deterministic ordering are not compatibility guarantees.
