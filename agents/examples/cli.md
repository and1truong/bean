# CLI protocol example

Each input file is one JSON object. The CLI envelope remains `bean.cli/v1alpha1`; its successful `result` is the same structured value returned by MCP.

```bash
printf '%s\n' '{"file":"./app.yaml"}' > /tmp/bean-validate.json
bean agent call bean.definition.validate --input /tmp/bean-validate.json --json

printf '%s\n' '{"file":"./app.yaml","target":"./bean.db"}' > /tmp/bean-publish.json
bean agent call bean.release.publish --input /tmp/bean-publish.json --json

printf '%s\n' '{"target":"./bean.db","view":"candidates","params":{"limit":20}}' > /tmp/bean-query.json
bean agent call bean.application.query --input /tmp/bean-query.json --json
```

The CLI defaults to all three planes because the caller launches the local process. Use `--allow-plane definition` or another comma-separated subset when the invocation must be constrained. Runtime identity flags are host configuration and are intentionally absent from the JSON request.
