# Golang-Migrate Phase 2 Design

> **For agentic workers:** this is a design/spec document only. Do not implement before the spec is approved and turned into a plan.

**Goal:** Move the backend to `golang-migrate` while automatically cutting existing databases over from the `public` schema to a new `expensor` schema at startup.

**Context:** Phase 1 already renamed the custom bookkeeping table to `legacy_schema_migrations`, which frees the canonical `schema_migrations` table name for `golang-migrate`. Phase 2 introduces a one-time bridge for existing databases and makes `expensor` the permanent home for all application tables. Phase 3 will remove the bridge entirely.

**Tech Stack:** Go, PostgreSQL, `golang-migrate/migrate`, pgx, embedded SQL migrations, Task

---

## Requirements

- Create and use a new `expensor` schema for all application tables.
- Detect the cutover state automatically at startup.
- For existing databases, run a one-time migration bridge that moves the current `public` data into `expensor`.
- Keep runtime SQL unqualified and rely on `search_path=expensor,public`.
- Use `golang-migrate` for the permanent migration runner after the bridge completes.
- Preserve rollback only during the migration window. If the bridge fails before commit, the original `public` schema must remain usable.
- Remove the bridge in phase 3 without disturbing the permanent migration path.

---

## Architecture

The backend startup path should gain a dedicated migration bootstrap step in `backend/cmd/server` before the store and HTTP server are created.

That bootstrap step should:

1. Open a short-lived administrative connection.
2. Detect whether the `expensor` schema already exists.
3. If `expensor` is missing, run the one-time bridge.
4. Configure the long-lived application pool with `search_path=expensor,public`.
5. Hand off to `golang-migrate`, which owns `expensor.schema_migrations`.

The bridge must stay isolated from the permanent migration runner. It is temporary release-scaffolding, not a reusable runtime subsystem.

---

## Startup Bridge

### Fresh database

When the database is new and no application schema exists yet:

- Create the `expensor` schema.
- Run the normal `golang-migrate` startup path against `expensor`.
- Continue startup normally.

### Existing database

When `expensor` is missing but `public` already contains application tables:

- Create `expensor`.
- Recreate the current application tables in `expensor`.
- Copy the existing data from `public` into `expensor`.
- Validate the copy before cutover.
- Commit the bridge only after validation succeeds.

The bridge should be transactional or equivalent so a failure before commit leaves the old `public` layout unchanged. Rollback support is only required while this migration window is open; once the cutover commits, the old layout is no longer part of the supported runtime path.

---

## Migration Model

Phase 2 should replace the custom filename-tracking runner with `golang-migrate`.

- Migration files should use the normal `.up.sql` / `.down.sql` layout.
- The permanent migration source should continue to live in the backend repository and be embedded at build time.
- Application SQL should remain unqualified so the active schema can be selected by connection search path.
- The bridge should establish the initial `expensor` layout for existing databases, then baseline `golang-migrate` so future starts continue from the current schema version.

The custom `legacy_schema_migrations` table is retained only long enough to keep phase-1 databases functional during the transition. Phase 2 must not reintroduce the old custom runner as the permanent owner of migrations.

---

## Error Handling

- If the bridge cannot complete, startup must fail.
- If validation fails, the process must not continue into the main server.
- A failed bridge must not partially rewrite the database into an unsupported mixed state.
- Operators need enough logging to distinguish schema creation failures, copy failures, validation failures, and migration runner failures.

There should be no background retry loop and no silent best-effort fallback. The safe behavior is to stop and let the operator fix the database state before retrying startup.

---

## Testing

Phase 2 needs integration coverage against a real Postgres container.

- Verify that a database with only `public` tables is detected and migrated into `expensor`.
- Verify that a database already using `expensor` skips the bridge.
- Verify that a failed bridge leaves the original `public` layout intact.
- Verify that startup configures the runtime pool with `search_path=expensor,public`.
- Verify that `golang-migrate` owns `schema_migrations` after cutover.

Existing store and writer integration tests should continue to pass against the migrated schema. Test setup may need a small helper that runs the startup bridge before the suite opens a long-lived store connection.

---

## Phase 3 Follow-up

Phase 3 removes the temporary bridge and any other release-scoped compatibility code.

- Keep `golang-migrate` as the only migration mechanism.
- Keep `expensor` as the only application schema.
- Remove all code that exists solely to translate older `public` databases into the new layout.

---

## Out of Scope

- No dual-write layer.
- No long-lived compatibility reads from `public`.
- No manual operator-only migration workflow.
- No schema-qualified application SQL unless a specific query genuinely requires it later.
