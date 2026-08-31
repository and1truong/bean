# Goal: Reviewable application sources

Replace the single-file YAML bundle authoring format with explicit application manifests and human-readable resource documents without changing the canonical definition, validation, migration, AppIR, or activation lifecycle.

## Acceptance criteria

- An application manifest names the application, declares one API version, and explicitly lists local YAML resource files.
- Each resource file contains one or more flat, multi-document definitions with top-level `kind`, `name`, and kind-specific fields.
- Resource files are grouped by feature so application changes produce focused, readable diffs.
- The loader rejects unknown manifest fields, malformed YAML, missing or unsafe resources, unknown definition fields, and duplicate definitions.
- CLI diagnostics identify the source file, line, column, definition, and actionable cause; all independently discoverable errors are reported together.
- CLI import, source validation, embedded demos, tests, and documentation use the same loader.
- Loaded sources become the existing canonical `definition.Bundle` before validation; runtime semantics and persistence remain generic.

## Terminal gates

```bash
make check
make build
```
