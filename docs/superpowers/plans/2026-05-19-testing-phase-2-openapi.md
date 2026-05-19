# Testing Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce generated OpenAPI under `api/openapi` with a committed artifact, documented generation workflow, and CI drift detection.

**Architecture:** Phase 2 should annotate the existing Go HTTP handlers incrementally instead of attempting full-schema perfection in one pass. The first generated spec should cover the highest-value route groups the frontend and later contract tests depend on, then expose a repeatable generation/check workflow through Task and CI.

**Tech Stack:** Go, Swaggo, Task, GitHub Actions

---

## Status

- Status: Complete
- Last updated: 2026-05-19
- Owner: Unassigned
- Execution note: OpenAPI baseline, Task targets, README, and CI drift checks are in place. Rules, dashboard/stats, auth/OAuth callback, muted merchants, and merchant categorization remain deferred.

## Files

- Create: `backend/cmd/server/openapi.go`
- Modify: `backend/internal/api/handlers.go`
- Modify: `Taskfile.yml`
- Modify: `.github/workflows/ci.yml`
- Create: `api/openapi/expensor.openapi.yaml`
- Create: `api/openapi/README.md`

## Planned File Responsibilities

- `backend/cmd/server/openapi.go`
  Purpose: hold top-level Swaggo metadata in package `main` so generation can start from the actual backend entrypoint without polluting runtime logic.
- `backend/internal/api/handlers.go`
  Purpose: add route-level annotations and any request/response doc comments needed for the initial OpenAPI surface.
- `Taskfile.yml`
  Purpose: add `openapi:generate` and `openapi:check` targets with explicit summaries/descriptions for local use and CI.
- `.github/workflows/ci.yml`
  Purpose: add an OpenAPI drift-check job or step that fails when generated output differs from the committed artifact.
- `api/openapi/expensor.openapi.yaml`
  Purpose: committed generated OpenAPI artifact used by humans now and later contract tooling.
- `api/openapi/README.md`
  Purpose: document scope, generation command, drift-check workflow, and known limitations of the first generated spec.

## Scope Boundary

The initial generated spec should cover these route groups first:

- health, version, daemon bootstrap, and active-reader status
- settings/config endpoints used by the frontend
- labels, categories, and buckets taxonomy endpoints
- transactions list/search/facets/get/update/mutation endpoints

The following groups may remain deferred if annotation cost becomes too high during Phase 2:

- OAuth callback intricacies and auth edge flows
- rule import/export details beyond basic schema coverage
- stats/dashboard routes with complex nested response bodies
- muted merchants and merchant categorization if they materially delay the initial drift-check path

Any deferred route groups must be documented in `api/openapi/README.md`.

---

### Task 1: Establish OpenAPI Generation Entry Point and Scope

**Files:**
- Create: `backend/cmd/server/openapi.go`
- Modify: `docs/superpowers/specs/2026-05-19-frontend-testing.md`
- Modify: `docs/superpowers/plans/2026-05-19-testing-program.md`
- Modify: `docs/superpowers/plans/2026-05-19-testing-phase-2-openapi.md`

- [x] **Step 1: Mark Phase 2 active before implementation starts**

Update the tracking docs to mark Phase 2 `In Progress` once execution begins.

Expected:
- spec `Program Status` table shows Phase 2 `In Progress`
- program index shows Phase 2 `In Progress`
- this plan status block is updated from `Plan Ready` to `In Progress`

- [x] **Step 2: Add top-level Swaggo metadata bootstrap**

Create `backend/cmd/server/openapi.go` in package `main` with:

- API title: `Expensor API`
- version placeholder: `dev`
- description explaining that this is the generated API contract for the Expensor backend
- `BasePath` `/api`
- `schemes` `http https`

Keep this file documentation-only; do not add runtime behavior.

- [x] **Step 3: Verify the bootstrap file is discoverable**

Run:

```bash
test -f backend/cmd/server/openapi.go
```

Expected: command exits 0.

---

### Task 2: Annotate Initial High-Value Route Groups

**Files:**
- Modify: `backend/internal/api/handlers.go`

- [x] **Step 1: Annotate bootstrap and config routes**

Add Swaggo annotations for:

- `HandleHealth`
- `HandleVersion`
- `HandleStatus`
- `HandleStartDaemon`
- `HandleRescan`
- `HandleGetActiveReader`
- `HandleGetBaseCurrency`
- `HandleSetBaseCurrency`
- `HandleGetScanInterval`
- `HandleSetScanInterval`
- `HandleGetLookbackDays`
- `HandleSetLookbackDays`
- `HandleGetTimezone`
- `HandleSetTimezone`
- `HandleGetTimeFormat`
- `HandleSetTimeFormat`
- `HandleGetReaderCheckpoint`
- `HandleClearReaderCheckpoint`
- `HandleTriggerSync`
- `HandleGetSyncStatus`

For each annotation block, define:

- summary
- tags
- path params/query params where applicable
- success response type
- relevant failure responses for validation/not found/disabled store cases

- [x] **Step 2: Annotate taxonomy routes**

Add annotations for:

- `HandleListLabels`
- `HandleCreateLabel`
- `HandleUpdateLabel`
- `HandleDeleteLabel`
- `HandleApplyLabel`
- `HandleGetLabelMappings`
- `HandleListCategories`
- `HandleCreateCategory`
- `HandleDeleteCategory`
- `HandleListBuckets`
- `HandleCreateBucket`
- `HandleDeleteBucket`
- `HandleListBanks`

- [x] **Step 3: Annotate transaction routes**

Add annotations for:

- `HandleListTransactions`
- `HandleSearchTransactions`
- `HandleGetFacets`
- `HandleGetTransaction`
- `HandleUpdateTransaction`
- `HandleAddLabels`
- `HandleRemoveLabel`
- `HandleMuteTransaction`
- `HandleUpdateMuteReason`

Make sure query params for `HandleListTransactions` and `HandleSearchTransactions` are documented with the actual URL keys used by the frontend.

- [x] **Step 4: Keep deferred endpoints explicitly deferred**

Do not force-open the whole route surface if it would introduce low-quality schemas. If stats/auth/rules remain out of scope, record them for README documentation in Task 4 rather than creating vague annotations now.

- [x] **Step 5: Run targeted backend type-check**

Run:

```bash
cd backend && go test ./internal/api ./cmd/server
```

Expected: command exits 0.

---

### Task 3: Generate and Commit the First OpenAPI Artifact

**Files:**
- Create: `api/openapi/expensor.openapi.yaml`

- [x] **Step 1: Add the output directory**

Create `api/openapi/` if it does not already exist.

- [x] **Step 2: Generate the first OpenAPI YAML**

Run a pinned Swaggo generation command from `backend/` that:

- uses `openapi.go` as the general-info entrypoint
- includes `./cmd/server` and `./internal/api` in the parse scope
- writes YAML output to `../api/openapi`
- then normalizes the generated filename to `../api/openapi/expensor.openapi.yaml`

Recommended shape:

```bash
cd backend && go run github.com/swaggo/swag/cmd/swag@v1.16.4 init \
  -g openapi.go \
  -d ./cmd/server,./internal/api \
  --output ../api/openapi \
  --outputTypes yaml
mv ../api/openapi/swagger.yaml ../api/openapi/expensor.openapi.yaml
```

Pick and keep one explicit Swaggo version in the command/Taskfile. Do not rely on an unpinned latest. Swaggo emits `swagger.yaml` by default, so the workflow must rename that artifact to the committed repo path.

- [x] **Step 3: Verify the generated artifact exists**

Run:

```bash
test -f api/openapi/expensor.openapi.yaml
```

Expected: command exits 0.

- [x] **Step 3a: Verify the generated artifact is tracked**

Run:

```bash
git ls-files --error-unmatch api/openapi/expensor.openapi.yaml
```

Expected: command exits 0.

- [x] **Step 4: Sanity-check the generated YAML**

Run:

```bash
sed -n '1,120p' api/openapi/expensor.openapi.yaml
```

Expected:
- top-level metadata is present
- `/api/health` is present
- `/api/config/base-currency` is present
- `/api/transactions` is present

---

### Task 4: Document Scope and Regeneration Workflow

**Files:**
- Create: `api/openapi/README.md`

- [x] **Step 1: Document the artifact purpose**

Add a short README section covering:

- what `expensor.openapi.yaml` is
- that it is generated and committed
- that it is the Phase 2 baseline for later contract testing

- [x] **Step 2: Document local generation**

Document:

- the `task openapi:generate` target to be added in Task 5
- when contributors should regenerate the file
- that committed drift is not acceptable

- [x] **Step 3: Document intentional scope limits**

List any route groups intentionally deferred from the initial generated spec.

- [x] **Step 4: Verify the README is concrete**

Run:

```bash
rg -n "generated and committed|task openapi:generate|deferred|contract" api/openapi/README.md
```

Expected: all four concepts are present.

---

### Task 5: Add Taskfile Targets for Generation and Drift Checks

**Files:**
- Modify: `Taskfile.yml`

- [x] **Step 1: Add `openapi:generate`**

Add a Task target that:

- runs from `backend/`
- invokes the pinned Swaggo generator
- writes to `../api/openapi`

The target must have a strong `summary` and `desc`.

- [x] **Step 2: Add `openapi:check`**

Add a Task target that:

- regenerates the spec
- fails if `git diff --exit-code -- api/openapi/expensor.openapi.yaml` reports drift

The target must explain in `desc` that it is intended for CI and contributor verification.

- [x] **Step 3: Verify Task metadata and parsing**

Run:

```bash
task --list | rg "openapi:generate|openapi:check"
```

Expected: both tasks appear with descriptive text.

- [x] **Step 4: Verify generation through Taskfile**

Run:

```bash
task openapi:generate
```

Expected: the YAML artifact regenerates without manual path juggling.

---

### Task 6: Add CI Drift Detection

**Files:**
- Modify: `.github/workflows/ci.yml`

- [x] **Step 1: Add an OpenAPI verification job or step**

Add CI wiring that:

- checks out the repo
- sets up Go using `backend/go.mod`
- installs Task
- runs `task openapi:check`

- [x] **Step 2: Keep the CI job readable**

Use a dedicated job name such as `openapi-check` unless folding into an existing backend verification lane is materially simpler.

- [x] **Step 3: Verify the workflow diff**

Run:

```bash
git diff -- .github/workflows/ci.yml
```

Expected: the drift-check step/job is easy to understand and isolated from unrelated CI work.

---

### Task 7: Final Verification and Status Reflection

**Files:**
- Verify: `backend/cmd/server/openapi.go`
- Verify: `backend/internal/api/handlers.go`
- Verify: `api/openapi/expensor.openapi.yaml`
- Verify: `api/openapi/README.md`
- Verify: `Taskfile.yml`
- Verify: `.github/workflows/ci.yml`
- Modify: `docs/superpowers/specs/2026-05-19-frontend-testing.md`
- Modify: `docs/superpowers/plans/2026-05-19-testing-program.md`
- Modify: `docs/superpowers/plans/2026-05-19-testing-phase-2-openapi.md`

- [x] **Step 1: Run OpenAPI generation**

Run:

```bash
task openapi:generate
```

Expected: `api/openapi/expensor.openapi.yaml` is regenerated successfully.

- [x] **Step 2: Run drift check**

Run:

```bash
task openapi:check
```

Expected: command exits 0 with no diff.

- [x] **Step 3: Run focused backend verification**

Run:

```bash
cd backend && go test ./internal/api ./cmd/server
```

Expected: command exits 0.

- [x] **Step 4: Mark Phase 2 complete in the tracking docs**

Update:

- spec `Program Status` table
- program index Phase 2 entry
- this plan’s status block

- [x] **Step 5: Record any deferred endpoints explicitly**

If some route groups were intentionally left out, list them in:

- `api/openapi/README.md`
- this plan’s execution notes

Do not silently treat partial coverage as complete coverage.

---

## Exit Criteria

- `api/openapi/expensor.openapi.yaml` is generated and committed
- `api/openapi/README.md` documents generation and deferred scope clearly
- `task openapi:generate` and `task openapi:check` exist with descriptive metadata
- CI fails if the generated OpenAPI artifact drifts from the committed file
- the initial spec covers the frontend-critical route groups selected for Phase 2
