# Golang-Migrate Schema Shift Design

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the custom SQL migration runner with `golang-migrate` while moving all application tables from `public` into an `expensor` schema and preserving a safe, one-time startup bridge between the old and new layouts.

**Architecture:** The work is split into three release-separated phases. Phase 1 renames the current custom migration bookkeeping table so the canonical `schema_migrations` name is free for `golang-migrate`. Phase 2 introduces a startup-owned compatibility bridge that creates `expensor`, recreates the application schema there, copies data across from `public`, validates the result, and then hands off to `golang-migrate` using the default `schema_migrations` table in `expensor`. Phase 3 removes the bridge and the legacy runner, leaving only `golang-migrate` plus the `expensor` schema.

**Tech Stack:** Go, PostgreSQL, `golang-migrate/migrate`, embedded `io/fs` migrations, pgx, Task

---

## Phase 1: Free the `schema_migrations` Name

Phase 1 is a compatibility-only PR. Its purpose is to rename the current filename-tracking table used by the custom runner so the next release can adopt `golang-migrate` without colliding with the existing table name.

### Requirements

- Rename the current tracking table from `schema_migrations` to `legacy_schema_migrations`.
- Keep the current sequential migration runner working against the renamed table.
- Preserve the existing idempotent startup behavior for current databases.
- Ensure the rename is safe for already-migrated databases and fresh databases.

### Validation

- Existing backend tests continue to pass.
- Startup migration tests confirm the old runner still records applied filenames, but under the renamed table.
- The application no longer creates or reads a table named `schema_migrations` during phase 1.

---

## Phase 2: Adopt Golang-Migrate and Move Tables to `expensor`

Phase 2 is a new release after phase 1 ships. It introduces the `expensor` schema and a one-time startup bridge that migrates existing databases from the `public` layout to the new schema before the app uses `golang-migrate` normally.

### Requirements

- Create the `expensor` schema if it does not exist.
- Recreate the application tables in `expensor`.
- Copy existing data from `public` tables into the `expensor` tables.
- Keep the bridge startup-owned and automatic for existing databases.
- Treat the bridge as one-time migration code, not as part of the permanent migration runner.
- Use `golang-migrate` after the bridge completes successfully.
- Use the default `schema_migrations` table for `golang-migrate` in the new schema.
- Preserve a rollback path only during the migration window. If the bridge fails before it commits, the original `public` schema remains usable.

### Bridge Behavior

- The bridge should run before application code opens a long-lived store connection.
- The bridge should run in a transaction or equivalent atomic unit so a failure leaves the old layout intact.
- The bridge should validate that the new schema can see the copied data before committing.
- The bridge should not introduce dual-write behavior or ongoing synchronization between `public` and `expensor`.
- The bridge should be isolated behind a dedicated startup helper so phase 3 can remove it cleanly.

### Migration File Expectations

- Application migrations should follow `golang-migrate` conventions, including `.up.sql` and `.down.sql` files.
- Schema-qualified SQL should target `expensor` where needed.
- The compatibility bridge should be separate from the permanent migration files so its removal does not disturb normal schema evolution.

### Validation

- Fresh-install tests create `expensor`, apply migrations, and verify the app works against the new schema.
- Existing-database tests verify the bridge copies data forward correctly.
- Failure tests verify a bridge error leaves the original `public` schema intact.
- Startup integration tests confirm `golang-migrate` owns `schema_migrations` after the bridge.

---

## Phase 3: Remove the Bridge

Phase 3 is the cleanup release after phase 2 has shipped and the migration window has closed. The application should no longer carry any compatibility path for `public`.

### Requirements

- Remove the one-time schema bridge and any temporary startup logic that only exists for migration.
- Keep `golang-migrate` as the only migration mechanism.
- Keep the application operating solely against the `expensor` schema.
- Leave the permanent migration setup minimal and easy to understand.

### Validation

- No phase-2 bridge code remains in the runtime path.
- Startup still applies `golang-migrate` migrations and uses `schema_migrations`.
- Existing tests cover normal startup and schema ownership without the bridge.

---

## Risks and Constraints

- This is a multi-release migration program, not a single code change.
- The phase-1 rename must land before any `golang-migrate` work that expects to own `schema_migrations`.
- The phase-2 bridge must be deliberately one-time and removable; it must not become a permanent compatibility layer.
- The schema move should be staged so existing data survives the transition without manual database resets.

## Out of Scope

- No schema cleanup inside unrelated feature work.
- No permanent dual-write or shadow-read layer.
- No direct migration to a custom `golang-migrate` table name unless a future decision explicitly requires it.
