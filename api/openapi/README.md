# OpenAPI baseline

`expensor.openapi.yaml` is the generated OpenAPI document for the Expensor backend. It is committed to the repository on purpose, not treated as a throwaway build artifact. In Phase 2 it serves as the baseline for later contract testing work.

## Regeneration

Use the pinned Swaggo workflow from `backend/`:

```bash
go run github.com/swaggo/swag/cmd/swag@v1.16.4 init -g openapi.go -d ./cmd/server,./internal/api --output ../api/openapi --outputTypes yaml
mv ../api/openapi/swagger.yaml ../api/openapi/expensor.openapi.yaml
```

Prefer the Task targets instead of running the commands manually:

- `task openapi:generate`
- `task openapi:check`

Regenerate the file whenever the annotated backend API surface changes. Committed drift is not acceptable; `task openapi:check` exists to catch it locally and in CI.

## Deferred scope for this pass

The initial generated spec intentionally leaves these areas for later phases:

- auth and OAuth callback flows
- rules import/export
- dashboard and stats
- muted merchants and merchant categorization
