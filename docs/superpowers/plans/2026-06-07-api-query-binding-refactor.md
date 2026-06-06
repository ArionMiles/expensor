# API Query Binding Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace manual candidate-API query parsing with typed form decoding while preserving the existing validation contract.

**Architecture:** Add one `go-playground/form` decoder to `Handlers`. A shared binding helper decodes `url.Values` into typed DTOs and translates conversion failures into the existing structured query-validation response; `validator/v10` continues to handle semantic constraints after binding.

**Tech Stack:** Go, `github.com/go-playground/form/v4`, `github.com/go-playground/validator/v10`, `net/http`.

---

### Task 1: Add the typed query binding boundary

**Files:**
- Create: `backend/internal/api/query_binding.go`
- Create: `backend/internal/api/query_binding_test.go`
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/validation.go`
- Modify: `backend/go.mod`
- Modify: `backend/go.sum`

- [x] **Step 1: Write failing tests**

Add focused tests proving that binding populates pointer-backed integers and timestamps, leaves absent values nil, and maps malformed values to `ValidationErrorDetail` entries with query field names.

- [x] **Step 2: Run tests to verify failure**

Run: `task test:be -- ./internal/api`

Expected: FAIL because the shared query-binding API does not exist.

- [x] **Step 3: Implement the shared binder**

Initialize one form decoder in `NewHandlers`, decode into typed DTOs, and convert decoder failures into the current 422 detail schema. Register `form` field names with the request validator.

- [x] **Step 4: Run tests to verify success**

Run: `task test:be -- ./internal/api`

Expected: PASS.

### Task 2: Migrate candidate handlers

**Files:**
- Modify: `backend/internal/api/handlers_transactions.go`
- Modify: `backend/internal/api/handlers_diagnostics.go`
- Delete: `backend/internal/api/query_validation.go`
- Test: `backend/internal/api/handlers_test.go`

- [x] **Step 1: Migrate transaction-list query binding**

Replace `decodeTransactionListQuery`, `appendQueryInt`, and `appendQueryTime` with the shared typed binder. Use `form` tags and retain the existing DTO-to-store-filter conversion.

- [x] **Step 2: Migrate diagnostic-list query binding**

Decode `diagnosticListQuery` through the same binder, then apply the existing default status before semantic validation.

- [x] **Step 3: Remove obsolete parsing helpers**

Delete the manual integer/time parsing file after both candidate handlers no longer reference it.

- [x] **Step 4: Run candidate handler tests**

Run: `task test:be -- ./internal/api`

Expected: PASS with unchanged response semantics.

### Task 3: Verify and publish

**Files:**
- Modify: `docs/superpowers/plans/2026-06-07-api-query-binding-refactor.md`

- [x] **Step 1: Format and verify**

Run:

```bash
task fmt:be
task test:be
task lint:be:prod
task openapi:check
```

Expected: all commands pass and strict lint reports `0 issues`.

- [x] **Step 2: Commit and push**

Commit the refactor in imperative mood and push `pr/api-validation-candidates` so PR #23 updates.
