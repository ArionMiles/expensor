# OpenAPI baseline

`expensor.openapi.yaml` is the generated OpenAPI document for the Expensor backend. It is committed to the repository on purpose, not treated as a throwaway build artifact.

## Regeneration

Use the Taskfile targets to generate and validate the swagger files.

- `task openapi:generate`
- `task openapi:check`

Regenerate the file whenever the annotated backend API surface changes. Committed drift is not acceptable; `task openapi:check` exists to catch it locally and in CI.

