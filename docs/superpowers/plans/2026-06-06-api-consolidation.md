# API Consolidation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidate redundant and action-oriented HTTP routes while preserving current application behavior.

**Architecture:** Keep existing store operations intact and change only the HTTP resource model, handler composition, frontend client contract, and generated API documentation. Implement each route family independently with public behavior tests before moving to the next family.

**Tech Stack:** Go `net/http`, React/TypeScript Axios client, Vitest/MSW, Swag/OpenAPI generation, Go handler and integration tests.

---

### Task 1: Consolidate Transaction List And Search

**Files:**
- Modify: `backend/internal/api/handlers_transactions.go`
- Modify: `backend/internal/api/server.go`
- Modify: `backend/internal/api/handlers_test.go`
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/api/client.test.ts`
- Modify: `frontend/src/mocks/handlers.ts`
- Modify: `frontend/playwright/readme-screenshot.spec.ts`
- Modify: `tests/contract/allowlist.tsv`
- Modify: `docs/testing/backend-test-matrix.md`
- Regenerate: `api/openapi/expensor.openapi.yaml`

- [ ] Add handler and frontend client tests proving `q` uses `/transactions` and invokes search behavior.
- [ ] Run the narrow backend and frontend tests and confirm they fail against the existing split routes.
- [ ] Merge `SearchTransactions` HTTP behavior into `ListTransactions`, remove the `/transactions/search` route, and update the frontend client and mocks.
- [ ] Regenerate OpenAPI and update contract/testing inventories.
- [ ] Run narrow tests and commit the transaction consolidation.

### Task 2: Consolidate Partial Resource Updates

**Files:**
- Modify: `backend/internal/api/handlers_transactions.go`
- Modify: `backend/internal/api/handlers_diagnostics.go`
- Modify: `backend/internal/api/server.go`
- Modify: `backend/internal/api/openapi_types.go`
- Modify: `backend/internal/api/handlers_test.go`
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/mocks/handlers.ts`
- Modify: affected frontend tests
- Regenerate: `api/openapi/expensor.openapi.yaml`

- [ ] Add handler tests for `PATCH /transactions/{id}` with transaction fields, mute state, and mute reason.
- [ ] Add tests for diagnostic and muted-merchant PATCH resources.
- [ ] Run narrow tests and confirm the new routes fail.
- [ ] Implement the PATCH handlers using existing store operations and remove the superseded routes.
- [ ] Update frontend calls, mocks, and OpenAPI.
- [ ] Run narrow tests and commit the partial-update consolidation.

### Task 3: Consolidate Application Preferences

**Files:**
- Modify: `backend/internal/api/handlers_config.go`
- Modify: `backend/internal/api/server.go`
- Modify: `backend/internal/api/openapi_types.go`
- Modify: `backend/internal/api/handlers_test.go`
- Modify: `frontend/src/api/client.ts`
- Modify: settings and onboarding consumers/tests
- Modify: `frontend/src/mocks/handlers.ts`
- Regenerate: `api/openapi/expensor.openapi.yaml`

- [ ] Add tests for complete preference reads and subset PATCH updates.
- [ ] Confirm tests fail with the current per-preference routes.
- [ ] Implement `GET` and `PATCH /config/preferences`, validating the complete supplied patch before persistence.
- [ ] Replace frontend preference calls and remove the ten old routes.
- [ ] Regenerate OpenAPI, run narrow tests, and commit.

### Task 4: Consolidate Reader Config And Annual Heatmap

**Files:**
- Modify: `backend/internal/api/server.go`
- Modify: `backend/internal/api/handlers_readers.go`
- Modify: `backend/internal/api/handlers_stats.go`
- Modify: `backend/internal/api/handlers_test.go`
- Modify: `frontend/src/api/client.ts`
- Modify: frontend mocks/tests
- Regenerate: `api/openapi/expensor.openapi.yaml`

- [ ] Add tests for `PUT /readers/{name}/config` and annual `GET /stats/heatmap?year=`.
- [ ] Confirm tests fail against current routes.
- [ ] Change reader config to PUT and merge annual heatmap routing into the heatmap handler, rejecting mixed year/range modes.
- [ ] Update frontend consumers and remove the annual subroute.
- [ ] Regenerate OpenAPI, run narrow tests, and commit.

### Task 5: Consolidate Taxonomy Merchant Mappings

**Files:**
- Modify: `backend/internal/api/handlers_taxonomy.go`
- Modify: `backend/internal/api/server.go`
- Modify: `backend/internal/api/handlers_test.go`
- Modify: `frontend/src/api/client.ts`
- Modify: affected frontend tests and mocks
- Regenerate: `api/openapi/expensor.openapi.yaml`

- [ ] Add tests for label, category, and bucket mapping PUT/DELETE routes with URL-decoded names and patterns.
- [ ] Confirm tests fail against action routes.
- [ ] Implement shared merchant-mapping handlers over existing store operations and remove request bodies from DELETE behavior.
- [ ] Update frontend consumers and mocks.
- [ ] Regenerate OpenAPI, run narrow tests, and commit.

### Task 6: Full Verification And PR

**Files:**
- Modify: `.github/PULL_REQUEST_TEMPLATE.md` only as a body template input; do not commit changes to the template.

- [ ] Run `task fmt`.
- [ ] Run `task test`.
- [ ] Run `task lint:be:prod`.
- [ ] Run `task lint:fe`.
- [ ] Run `task openapi:check`.
- [ ] Run `task test:be:contract`.
- [ ] Run relevant Playwright tests for transactions, settings, diagnostics, and expense groups.
- [ ] Review route inventory for removed paths.
- [ ] Push `pr/api-validation-candidates` and open a PR using the repository template.
