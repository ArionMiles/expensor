# Backend contract test harness

This suite runs the official Schemathesis container against the generated OpenAPI artifact and a live Expensor backend.

## What this harness covers

- request/response conformance for selected OpenAPI-covered routes
- status code, content type, response schema, and server-error checks
- drift between `api/openapi/expensor.openapi.yaml` and the running backend

## Initial scope

The first Phase 4 baseline intentionally covers read-only, parameter-free operations. This keeps the contract suite deterministic and avoids mixing schema conformance with stateful business behavior.

The allowlist lives in `allowlist.tsv`. Add endpoints there only after confirming they are stable enough for generated contract inputs.

## Local workflow

- `task test:be:contract`

Artifacts are written under `tests/contract/artifacts/`:

- `allowlist.tsv`
- one log file per checked operation
- `reports/` JUnit output emitted by Schemathesis

## Relationship to component tests

Component tests under `tests/component` assert seeded business behavior. Contract tests under `tests/contract` assert OpenAPI boundary conformance. Do not duplicate detailed business assertions here.
