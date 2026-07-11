# Phase 2 Handoff: SQLite Backend Support

## Goal

Add SQLite as a first-class supported database backend while keeping PostgreSQL support intact.

The end state is that Expensor can run as a single binary with SQLite by default, while PostgreSQL remains available when explicitly configured.

## Current Branch Context

- Phase 1 branch: `pr/sqlite-backend-prep`
- Relevant commits:
  - `a083aac Prepare database backend composition`
  - `6f9b1b3 Fix local dev startup`
- Phase 1 already introduced:
  - `backend/internal/app.NewStore`
  - backend-neutral store contracts in `backend/internal/store/contracts.go`
  - Postgres implementation under `backend/internal/store/postgres`
  - backend-owned seeding via `store.Seed`
  - backend-neutral conformance tests under `backend/internal/store/storetest`
  - TOML config fallback and env override flow
  - explicit DB backend config shape

## Non-Negotiable Decisions

- Do not infer DB backend from PostgreSQL env vars.
- If `EXPENSOR_DB_BACKEND` / `database.backend` is empty, app startup should log: `No DB backend configured. Defaulting to sqlite.`
- If PostgreSQL is configured, app startup should log: `PostgreSQL configured as the DB backend.`
- SQLite is the default for fresh single-binary use.
- PostgreSQL-to-SQLite migration is out of scope.
- SQLite support is fresh-install only.
- Use a pure-Go SQLite driver.
- Backend-specific SQL behavior belongs inside `internal/store/sqlite`, not shared utility code that handicaps PostgreSQL.
- Tests for SQLite must be at parity with PostgreSQL in the same PR. No phased or partial test coverage.

## Proposed Package Layout

```text
backend/internal/store/
  contracts.go
  models.go
  storetest/
    storetest.go
  postgres/
    *.go
    migrations/
      *.sql
  sqlite/
    *.go
    migrations/
      *.sql
```

## Driver Recommendation To Evaluate

Prefer `modernc.org/sqlite` if it satisfies the needs below:

- pure Go, no CGO requirement
- works for linux/darwin amd64 and arm64
- supports the SQL features and transaction behavior needed by Expensor
- acceptable migration and test runtime behavior
- acceptable locking/concurrency behavior for daemon ingestion plus HTTP reads/writes

Evaluate driver behavior with real store tests, not only an isolated proof of concept.

## Implementation Tasks

1. Add SQLite config behavior.
   - Use existing `config.Database.SQLite`.
   - Decide and implement default SQLite file path for single-binary use, likely under XDG data/state rather than config.
   - Keep `EXPENSOR_CONFIG_FILE` and `XDG_CONFIG_HOME` behavior as implemented in Phase 1.
   - Preserve env-overrides-TOML behavior.

2. Implement `backend/internal/store/sqlite`.
   - Implement `store.Backend`.
   - Keep repository boundaries similar to Postgres where practical.
   - Put SQLite-specific SQL, locking, upsert differences, JSON behavior, and migration handling inside the SQLite package.
   - Do not add shared helper abstractions just because SQLite lacks a PostgreSQL feature.

3. Add SQLite migrations.
   - Keep migrations embedded in the SQLite package.
   - Translate current schema for SQLite fresh installs.
   - Use idempotent migrations where practical.
   - Be explicit about UUID generation, timestamps, JSON, partial indexes, conflict clauses, and case-insensitive matching differences.

4. Wire `app.NewStore`.
   - `database.backend == ""` and `database.backend == "sqlite"` should open SQLite.
   - `database.backend == "postgres"` should open Postgres.
   - Unsupported backend should return a structured invalid argument error.
   - Startup still performs one health check and fails immediately if unhealthy.

5. Seed via backend method.
   - Implement `Seed(ctx, store.SeedContent)` on SQLite backend.
   - Preserve v2 rule data end-to-end: `sender_emails`, `source.type`, `source.label`, `source.bank`.
   - Preserve category resolver behavior.

6. Implement transaction ingestion.
   - SQLite `Store` should implement `Write(ctx, store.IngestionBatch) error`.
   - Match Postgres behavior for duplicate message upserts, preserving user edits for category/bucket/description.
   - Keep backend-specific conflict/upsert handling in SQLite code.

7. Conformance and parity tests.
   - Reuse `storetest.Run(t, backend)` for SQLite.
   - Add SQLite-specific tests for driver/migration/locking behavior as needed.
   - Keep existing Postgres tests passing.
   - Add or adapt component tests if startup/config/docker behavior changes.

8. Docker and release packaging.
   - Create SQLite-friendly `deploy/docker-compose.yml` as the default deploy compose.
   - Move the existing PostgreSQL compose to `deploy/docker-compose.postgres.yml`.
   - Keep a Postgres path documented and supported.
   - Publish binaries for:
     - linux/amd64
     - linux/arm64
     - darwin/amd64
     - darwin/arm64

9. README and install script.
   - README quick start should include:
     - existing docker-compose method
     - single-binary quick install script
   - Install script should:
     - detect platform and architecture
     - download the matching binary
     - install a default TOML config into `$XDG_CONFIG_HOME/expensor/config.toml`
     - avoid overwriting existing user config without consent

## Test Expectations

Run at minimum:

```bash
task fmt:be
task test:be
task lint:be:prod
task build:binary
```

Add targeted SQLite tests and run them explicitly during development.

If deploy docs, OpenAPI, frontend, or component behavior changes, also run the relevant task targets from `task --list-all`.

## Known Watch Points

- SQLite concurrency under daemon ingestion plus UI reads/writes.
- Tenant scoping behavior must exactly match Postgres.
- Case-insensitive matching semantics differ from PostgreSQL `ILIKE`.
- Timestamp precision and timezone behavior can differ.
- JSON storage and comparison behavior can differ.
- Partial unique indexes and conflict targets need careful translation.
- Avoid creating production files that only exist for tests.
