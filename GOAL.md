# Goal: Mark required Entity form fields

Status: complete

Make required fields immediately visible in metadata-driven administration Entity forms.

## Outcome

- Every Entity form field whose immutable metadata has `Required: true` shows a red `*` directly after its label.
- Text, numeric, sensitive, enum, relation, boolean, textarea, and file controls share the same required-field indication.
- The marker is visual-only for assistive technology; native required semantics and accessible field names remain unchanged.
- Optional fields do not receive the marker; required read-only fields retain it.

## Acceptance criteria

- A required Entity form label renders as `Label *`, with the `*` using the destructive red theme token.
- Existing typed controls, metadata behavior, and form submission remain unchanged.
- Focused React evidence verifies the marker's position and styling.
- `make check` and `make build` pass.

## Non-goals

- Changing Entity validation or required-field semantics.
- Adding markers to unrelated login, Studio, filter, or system-administration forms.
