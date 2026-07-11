# PR #74 Handoff: Phase 1 Cleanup And Refactor Follow-Ups

## Goal

Continue cleanup on the Phase 1 PR before moving on, focused on code organization, app composition, instrumentation, and error handling.

This is separate from Phase 2 SQLite implementation. The cleanup should make Phase 2 easier without adding SQLite itself.

## Current Branch Context

- Branch: `pr/sqlite-backend-prep`
- PR: `#74`
- Relevant local commits:
  - `94486ad Minor changes`
  - `d8b248f Simplify postgres startup health check`
  - `a083aac Prepare database backend composition`
  - `6f9b1b3 Fix local dev startup`

## Reference Material

Use the local LogChef clone as a reference, not a template to copy blindly:

```text
~/code/logchef/internal/store/store.go
~/code/logchef/internal/store/postgres/postgres.go
```

What to borrow conceptually:

- clear composition boundary
- backend-neutral application wiring outside `cmd`
- concrete backend ownership of database behavior
- store conformance-test pattern
- simple startup path

What not to borrow blindly:

- names that do not fit Expensor
- abstractions that are not needed yet
- broad rewrites unrelated to Phase 1 cleanup

## Current Architecture Shape

Phase 1 introduced:

- `backend/internal/app/store.go`
- `backend/internal/store/contracts.go`
- `backend/internal/store/instrumented`
- `backend/internal/store/storetest`
- concrete Postgres backend under `backend/internal/store/postgres`
- daemon transaction sink abstraction
- backend-owned seed method

The intended direction is:

- `internal/store` holds shared contracts and models.
- backend packages own database behavior.
- `internal/app` owns application composition.
- `cmd/server` should become thinner over time.
- instrumentation should wrap store contracts outside repository implementations.

## Cleanup Themes

1. Improve app composition.
   - Review what still lives in `cmd/server/main.go`, `bootstrap.go`, `daemon.go`, `scheduler.go`, and `content.go`.
   - Move composition logic into `internal/app` where it is not CLI-specific.
   - Keep `cmd/server` responsible for process entrypoint concerns: config load, signal handling, top-level startup/shutdown.
   - Do not turn `internal/app` into a grab bag. Prefer small files by responsibility.

2. Tighten store runtime naming and shape.
   - Revisit `app.Store` fields and names.
   - Ensure it exposes only what application layers need.
   - Avoid leaking backend-specific implementation types.
   - Keep `app.NewStore`, not `NewStoreRuntime`.

3. Improve instrumentation wrappers.
   - Keep instrumentation out of concrete repositories.
   - Keep delegated calls visible in wrapper methods.
   - Avoid callback helpers that hide the actual store call.
   - Review `internal/store/instrumented` for naming, grouping, and constructor clarity.
   - Consider splitting files by consumer/behavior boundaries if the single file remains too large.
   - Verify `TransactionBatchWriter` instrumentation is consistent with the rest of store instrumentation.

4. Continue error-builder cleanup.
   - Convert touched `fmt.Errorf` call sites to `backend/pkg/errors.E` where the error crosses a package or application boundary.
   - Do not mechanically convert every local helper error if it makes code worse.
   - Preserve useful operation names and kinds.
   - Avoid leaking sensitive data in error messages, logs, span status descriptions, metrics, or trace attrs.
   - Keep raw SQL and user data out of telemetry.

5. Revisit config organization.
   - Generic database settings should live under `config.Database`.
   - Backend-specific settings should live under `config.Database.Postgres` and `config.Database.SQLite`.
   - Do not reintroduce `BackendConfigured`; empty backend means not configured.
   - Remove dead config fields instead of keeping stale knobs.

6. Clean test support.
   - Do not add production `testing.go` helper files.
   - Test-only helpers should live in `_test.go` files.
   - Shared conformance tests under `storetest` are acceptable because production code does not import them.
   - Keep Postgres-specific helper APIs out of shared store contracts.

7. Review package boundaries.
   - `internal/store` contracts are intentionally centralized here despite the usual "interfaces at consumer boundaries" rule.
   - Do not duplicate the same narrow interface in every consumer if the result is worse.
   - Keep consumer-specific interfaces where they materially clarify a dependency, especially HTTP handler capability interfaces.

## Suggested Work Order

1. Read current PR #74 diff and note all current Phase 1 artifacts.
2. Compare Expensor composition to the LogChef reference files.
3. Propose specific moves before editing, especially for `cmd/server`.
4. Apply small, reviewable cleanup batches.
5. After each batch, run narrow tests first.
6. Run full backend validation before committing.

## Candidate Files To Inspect First

```text
backend/cmd/server/main.go
backend/cmd/server/bootstrap.go
backend/cmd/server/content.go
backend/cmd/server/daemon.go
backend/cmd/server/scheduler.go
backend/internal/app/store.go
backend/internal/store/contracts.go
backend/internal/store/instrumented/
backend/internal/store/postgres/
backend/internal/daemon/
backend/pkg/config/config.go
```

## Validation Expectations

Run at minimum:

```bash
task fmt:be
task test:be
task lint:be:prod
git diff --check
```

If startup behavior or `task dev` changes, verify:

```bash
task -n dev
task -n run:backend
```

Use Docker-backed tests when changing store behavior in a way that existing unit tests do not cover.

## Non-Goals

- Do not implement SQLite in this cleanup pass.
- Do not rewrite repository SQL just for style.
- Do not collapse backend-specific code into shared utilities.
- Do not add broad abstractions without a direct cleanup payoff.
- Do not reintroduce long startup polling for database readiness.
