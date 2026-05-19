# Backend component test harness

This suite runs black-box backend functional tests against a live Expensor backend and PostgreSQL instance started with Docker Compose.

## What this harness covers

- live HTTP behavior through the public API
- deterministic seeded data scenarios
- runtime backend coverage from the running binary

## What this harness does not cover

- OpenAPI contract conformance
- frontend/browser behavior
- reader OAuth and daemon happy-path scanning flows

## Local workflow

- `task test:be:component`

Artifacts are written under `tests/component/artifacts/`:

- `backend-coverage/` raw `GOCOVERDIR` files
- `backend-component.coverage.out` text coverage profile

## Seed data

The suite loads `fixtures/seed.sql` after migrations complete. Tests should assume the seeded rows described there instead of creating large ad hoc datasets inside each test.

## Seed source and privacy rule

The first version of `fixtures/seed.sql` should be derived from the realistic data already present in the local `expensor-dev-postgres` container rather than invented from scratch.

Before any row is copied into the fixture:

- replace message IDs, UUIDs, merchant fragments, descriptions, reasons, and free-text fields that could reveal personal information with anonymized placeholders
- normalize timestamps where exact real-world times are not required for the asserted behavior
- keep the business shape that matters for tests: currencies, categories, buckets, muted state, label relationships, and config values
- do not copy OAuth tokens, credentials, or any reader-specific secrets into fixtures

The committed fixture must be safe to publish.

## Test organization rule

All backend component tests in this phase must use table-driven test structure.

For each endpoint-oriented test function:

- define a `[]struct{...}` table of cases
- include the case name, request path/body, expected status, and the key assertion for the response
- iterate with `for _, tc := range cases { t.Run(tc.name, func(t *testing.T) { ... }) }`

Different behaviors for the same endpoint should be added as table entries, not as separate one-off test functions unless the setup shape is genuinely different.
