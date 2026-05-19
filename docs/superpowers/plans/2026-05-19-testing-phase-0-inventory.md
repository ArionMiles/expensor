# Testing Phase 0 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce the authoritative test inventory and risk matrix that all later testing phases will use to select initial scope and avoid duplicate or low-value tests.

**Architecture:** This phase is documentation-first and deliberately non-invasive. It extracts the current backend API surface from the Go route table, inventories the frontend page/component interaction surface, classifies user-visible behaviors into the right test layers, and records which cases are intentionally deferred. The output is a set of repo docs under `docs/testing/` plus status updates in the testing spec.

**Tech Stack:** Markdown, Go route definitions, frontend source inventory, repo testing spec

---

## Status

- Status: Complete
- Last updated: 2026-05-19
- Owner: Unassigned

## Files

- Modify: `docs/superpowers/specs/2026-05-19-frontend-testing.md`
- Create: `docs/testing/test-inventory.md`
- Create: `docs/testing/backend-test-matrix.md`
- Create: `docs/testing/frontend-test-matrix.md`

## Planned File Responsibilities

- `docs/testing/test-inventory.md`
  Purpose: top-level overview, evaluation method, chosen initial cases, deferred cases, and phase recommendations.
- `docs/testing/backend-test-matrix.md`
  Purpose: backend route-group inventory with per-group candidate behaviors and recommended test layers.
- `docs/testing/frontend-test-matrix.md`
  Purpose: frontend page/component inventory with interaction risks, URL-state risks, and recommended test layers.
- `docs/superpowers/specs/2026-05-19-frontend-testing.md`
  Purpose: keep program-level status aligned with the fact that Phase 0 now has a detailed implementation plan and, once executed, completed inventory outputs.

### Task 1: Create the Testing Docs Directory and Inventory Shells

**Files:**
- Create: `docs/testing/test-inventory.md`
- Create: `docs/testing/backend-test-matrix.md`
- Create: `docs/testing/frontend-test-matrix.md`

- [ ] **Step 1: Write the failing structure check**

Run:

```bash
test -f docs/testing/test-inventory.md && test -f docs/testing/backend-test-matrix.md && test -f docs/testing/frontend-test-matrix.md
```

Expected: command exits non-zero because the files do not exist yet.

- [ ] **Step 2: Write the three markdown files with initial skeletons**

`docs/testing/test-inventory.md`

```md
# Expensor Test Inventory

## Purpose

This document records the initial test-case evaluation for the testing enhancement program.
It is the source of truth for which behaviors should be covered first, which test layer owns
them, and which cases are intentionally deferred.

## Evaluation Rules

1. Prefer high-value, low-brittleness cases first.
2. Avoid duplicate coverage across layers unless the layer serves a different purpose.
3. Treat URL persistence, floating UI behavior, and backend data correctness as first-class risks.
4. Mark deferred cases explicitly instead of leaving them implied.

## Initial Delivery Recommendations

## Deferred Cases
```

`docs/testing/backend-test-matrix.md`

```md
# Backend Test Matrix

| Domain | Routes | Critical Behaviors | Recommended Layer | Priority | Notes |
|---|---|---|---|---|---|
```

`docs/testing/frontend-test-matrix.md`

```md
# Frontend Test Matrix

| Surface | File | Critical Behaviors | Recommended Layer | Priority | Notes |
|---|---|---|---|---|---|
```

- [ ] **Step 3: Run a file existence check**

Run:

```bash
test -f docs/testing/test-inventory.md && test -f docs/testing/backend-test-matrix.md && test -f docs/testing/frontend-test-matrix.md
```

Expected: command exits 0.

- [ ] **Step 4: Commit**

```bash
git add docs/testing/test-inventory.md docs/testing/backend-test-matrix.md docs/testing/frontend-test-matrix.md
git commit --no-gpg-sign -m "docs: scaffold testing inventory docs"
```

### Task 2: Populate the Backend Route Inventory

**Files:**
- Modify: `docs/testing/backend-test-matrix.md`
- Reference: `backend/internal/api/server.go:85-197`

- [ ] **Step 1: Reproduce the backend route list from source**

Run:

```bash
sed -n '85,197p' backend/internal/api/server.go
```

Expected: route registrations grouped around health, readers, stats, config, rules, transactions, muted merchants, and merchant categorization.

- [ ] **Step 2: Write the initial backend domain matrix**

Replace the matrix with:

```md
# Backend Test Matrix

| Domain | Routes | Critical Behaviors | Recommended Layer | Priority | Notes |
|---|---|---|---|---|---|
| Health, daemon, and bootstrap status | `GET /api/health`, `GET /api/status`, `GET /api/version`, `POST /api/daemon/start`, `POST /api/daemon/rescan`, `GET /api/config/active-reader` | health availability, status shape, daemon state transitions, active-reader bootstrap state | Unit, Component, Contract | High | low schema complexity, high operational visibility |
| Plugin and reader discovery | `GET /api/plugins/readers`, `GET /api/plugins/writers`, `GET /api/readers/thunderbird/discover/profiles`, `GET /api/readers/thunderbird/discover/mailboxes`, `GET /api/readers/{name}/guide` | plugin listing, reader discovery, setup bootstrap shape | Unit, Contract | Medium | important for setup and onboarding entry points |
| Reader auth and configuration | `POST /api/readers/{name}/credentials`, `GET /api/readers/{name}/credentials/status`, `POST /api/readers/{name}/auth/start`, `GET /api/auth/callback`, `POST /api/readers/{name}/auth/exchange`, `GET /api/readers/{name}/auth/status`, `DELETE /api/readers/{name}/auth/token`, `GET /api/readers/{name}/config`, `POST /api/readers/{name}/config`, `GET /api/readers/{name}/status`, `DELETE /api/readers/{name}` | auth flow edges, readiness state, validation failures, config persistence | Unit, Contract, Deferred Playwright | Medium | operationally noisy; split happy-path docs from full auth complexity |
| Dashboard and stats | `GET /api/stats/dashboard`, `GET /api/stats/charts`, `GET /api/stats/labels/monthly`, `GET /api/stats/heatmap`, `GET /api/stats/heatmap/annual` | seeded aggregates, query validation, shape stability | Component, Contract | High | strong candidate for seeded correctness tests |
| Config and settings | `GET /api/config/banks`, `POST /api/config/sync`, `GET /api/config/sync/status`, `GET /api/config/base-currency`, `PUT /api/config/base-currency`, `GET /api/config/scan-interval`, `PUT /api/config/scan-interval`, `GET /api/config/lookback-days`, `PUT /api/config/lookback-days`, `GET /api/config/timezone`, `PUT /api/config/timezone`, `GET /api/config/time-format`, `PUT /api/config/time-format`, `GET /api/config/readers/{name}/checkpoint`, `DELETE /api/config/readers/{name}/checkpoint` | persistence, validation errors, default handling | Unit, Component, Contract | High | directly drives frontend settings behavior |
| Labels, categories, and buckets | `GET /api/config/labels/export`, `GET /api/config/labels/mappings`, `GET /api/config/labels`, `POST /api/config/labels`, `PUT /api/config/labels/{name}`, `DELETE /api/config/labels/{name}`, `POST /api/config/labels/{name}/apply`, `DELETE /api/config/labels/{name}/merchant`, `GET /api/config/categories/export`, `GET /api/config/categories`, `POST /api/config/categories`, `DELETE /api/config/categories/{name}`, `GET /api/config/buckets`, `POST /api/config/buckets`, `DELETE /api/config/buckets/{name}` | CRUD correctness, conflict behavior, export shape | Unit, Component, Contract | High | important for Transactions and settings flows |
| Rules | `GET /api/rules`, `GET /api/rules/export`, `POST /api/rules/import`, `POST /api/rules`, `PUT /api/rules/{id}`, `DELETE /api/rules/{id}` | regex validation, protected/default rule behavior, mutation success | Unit, Component, Contract | High | likely early frontend + backend test target |
| Transactions | `GET /api/transactions/search`, `GET /api/transactions/facets`, `GET /api/transactions`, `GET /api/transactions/{id}`, `PUT /api/transactions/{id}`, `POST /api/transactions/{id}/labels`, `DELETE /api/transactions/{id}/labels/{label}`, `PUT /api/transactions/{id}/mute`, `PUT /api/transactions/{id}/mute-reason` | filtering correctness, URL-driven query mapping, mutation effects | Unit, Component, Contract | Highest | central user value and highest regression risk |
| Muted merchants | `GET /api/muted-merchants`, `POST /api/muted-merchants`, `PUT /api/muted-merchants/{id}/reason`, `DELETE /api/muted-merchants/{id}` | merchant-level mute behavior and reason persistence | Unit, Component, Contract | Medium | complements transaction muting |
| Merchant categorization | `POST /api/merchants/categorize` | validation, category/bucket propagation behavior | Unit, Component, Contract | Medium | likely paired with seeded taxonomy cases |
```

- [ ] **Step 3: Verify the file contains every route group**

Run:

```bash
rg -n "Health and daemon status|Reader discovery and auth|Dashboard and stats|Config and settings|Labels, categories, buckets|Rules|Transactions|Muted merchants|Merchant categorization" docs/testing/backend-test-matrix.md
```

Expected: 10 matching rows or headers covering the backend surface.

- [ ] **Step 4: Commit**

```bash
git add docs/testing/backend-test-matrix.md
git commit --no-gpg-sign -m "docs: add backend testing matrix"
```

### Task 3: Populate the Frontend Interaction Inventory

**Files:**
- Modify: `docs/testing/frontend-test-matrix.md`
- Reference: `frontend/src/pages/*.tsx`
- Reference: `frontend/src/components/*.tsx`
- Reference: `frontend/src/hooks/useTooltip.tsx`

- [ ] **Step 1: Reproduce the candidate frontend surface**

Run:

```bash
find frontend/src -maxdepth 2 \( -type f -name '*.tsx' -o -type f -name '*.ts' \) | sort
```

Expected: files for pages such as `Transactions.tsx`, `Settings.tsx`, `Rules.tsx`, and components such as `InlineSelect.tsx`, `LabelCombobox.tsx`, `ConfirmModal.tsx`, `DateRangePicker.tsx`, `SlideNotification.tsx`, `FilterCombobox.tsx`.

- [ ] **Step 2: Write the initial frontend matrix**

Replace the matrix with:

```md
# Frontend Test Matrix

| Surface | File | Critical Behaviors | Recommended Layer | Priority | Notes |
|---|---|---|---|---|---|
| Transactions page | `frontend/src/pages/Transactions.tsx` | URL search param persistence, filter/query mapping, mutation feedback, search pagination | Frontend Unit, Playwright | Highest | page with the densest interaction surface |
| App shell and navigation | `frontend/src/components/AppLayout.tsx`, `frontend/src/components/Sidebar.tsx`, `frontend/src/components/CommandPalette.tsx` | global navigation behavior, sidebar persistence, keyboard shortcuts, portal/floating navigation UI | Frontend Unit, Playwright | High | shared interaction surface across the whole app |
| Dashboard page | `frontend/src/pages/Dashboard.tsx` | summary/date state persistence, drill-down navigation, floating UI interactions, chart and heatmap state | Frontend Unit, Playwright | High | high-value page with multiple stateful views |
| Settings page | `frontend/src/pages/Settings.tsx` | tab persistence in URL, save feedback, settings value validation surfaces | Frontend Unit, Playwright | High | directly tied to repo URL-state rule |
| Muted page | `frontend/src/pages/MutedPage.tsx` | tab persistence, inline editing, filtering, confirm flows, mutation entry points | Frontend Unit, Playwright | High | non-trivial management surface with URL state and mutations |
| Rules page | `frontend/src/pages/Rules.tsx` | CRUD entry points, inline editing affordances, confirmation flow | Frontend Unit, Playwright | High | pairs with backend rules workstream |
| Setup wizard | `frontend/src/pages/setup/Wizard.tsx` | branching setup flow, reader-specific transitions, OAuth/manual callback exchange fallback, completion path | Frontend Unit, Playwright | High | browser flow candidate after unit harness; step files are supporting detail |
| InlineSelect | `frontend/src/components/InlineSelect.tsx` | keyboard nav, outside click, avoid redundant commit, fixed-position dropdown | Frontend Unit | High | reusable primitive with interaction risk |
| LabelCombobox | `frontend/src/components/LabelCombobox.tsx` | label filtering, create/select/remove flows, portal dropdown and notification trigger | Frontend Unit | High | transaction labeling risk |
| FilterCombobox | `frontend/src/components/FilterCombobox.tsx` | input filtering, clear action, keyboard selection, outside click | Frontend Unit | Medium | reusable but simpler than transactions page |
| DateRangePicker | `frontend/src/components/DateRangePicker.tsx` | range selection, clear/apply, portal behavior, date/time carry-over | Frontend Unit | High | complex stateful input |
| ConfirmModal | `frontend/src/components/ConfirmModal.tsx` | confirm/cancel callback wiring, escape handling, overlay close | Frontend Unit | Medium | modal interaction primitive |
| SlideNotification | `frontend/src/components/SlideNotification.tsx` | timeout action, manual actions, dismissal timing | Frontend Unit | Medium | timer-driven UI |
| Tooltip hook | `frontend/src/hooks/useTooltip.tsx` | hover visibility, portal rendering, placement behavior | Frontend Unit | Medium | shared primitive |
```

- [ ] **Step 3: Verify the file contains the chosen initial targets**

Run:

```bash
rg -n "Transactions page|Settings page|Rules page|Setup wizard|InlineSelect|LabelCombobox|FilterCombobox|DateRangePicker|ConfirmModal|SlideNotification|Tooltip hook" docs/testing/frontend-test-matrix.md
```

Expected: matches for all initial frontend targets.

- [ ] **Step 4: Commit**

```bash
git add docs/testing/frontend-test-matrix.md
git commit --no-gpg-sign -m "docs: add frontend testing matrix"
```

### Task 4: Add Layering Rules, Initial Recommendations, and Deferred Cases

**Files:**
- Modify: `docs/testing/test-inventory.md`

- [ ] **Step 1: Write the inventory overview and layer-assignment rules**

Add:

```md
## Layer Assignment Rules

- Backend Unit: handler/package logic, validation branches, status-code rules, store mocking.
- Backend Component: seeded end-to-end backend correctness through HTTP against live services.
- Backend Contract: request/response conformance against generated OpenAPI.
- Frontend Unit: page/component interaction logic, URL-state persistence, floating UI behavior.
- Playwright: small number of cross-page user journeys and browser-only regressions.

## Initial Delivery Recommendations

### Phase 1

- Backend unit coverage reporting
- Frontend unit harness
- Initial frontend tests:
  - Transactions page URL persistence
  - Settings tab URL persistence
  - InlineSelect
  - LabelCombobox
  - DateRangePicker
  - ConfirmModal
  - SlideNotification

### Phase 2

- OpenAPI generation for:
  - health/status/version
  - transactions
  - config/settings endpoints used by the frontend
  - rules
  - dashboard/stats

### Phase 3

- Initial backend component suites:
  - transactions filtering and mutation correctness
  - taxonomy CRUD correctness
  - rules correctness
  - dashboard/heatmap seeded data correctness

### Phase 4

- Contract validation for the OpenAPI-covered route groups from Phase 2

### Phase 5

- Initial Playwright flows:
  - setup wizard happy path
  - transactions filter persistence on reload
  - rules CRUD happy path
  - settings tab persistence
```

- [ ] **Step 2: Add explicit deferred cases**

Append:

```md
## Deferred Cases

- Full OAuth browser automation against real providers
- Broad snapshot coverage of frontend pages
- Exhaustive browser coverage of taxonomy CRUD once frontend unit tests already cover the interactions
- Low-level duplicate API cases already covered by unit and component layers
- Playwright promotion to required CI status before flake data exists
```

- [ ] **Step 3: Verify the recommendations and deferred sections exist**

Run:

```bash
rg -n "Layer Assignment Rules|Initial Delivery Recommendations|Deferred Cases|Transactions page URL persistence|OpenAPI generation for|Initial backend component suites|Initial Playwright flows" docs/testing/test-inventory.md
```

Expected: matches for each required section and seeded recommendation.

- [ ] **Step 4: Commit**

```bash
git add docs/testing/test-inventory.md
git commit --no-gpg-sign -m "docs: document testing inventory priorities"
```

### Task 5: Reflect Phase 0 Completion in the Program Spec

**Files:**
- Modify: `docs/superpowers/specs/2026-05-19-frontend-testing.md`

- [ ] **Step 1: Mark the workstream complete once the inventory docs exist**

Change:

```md
| Phase 0 — Test inventory and risk evaluation | Plan Ready | `docs/superpowers/plans/2026-05-19-testing-phase-0-inventory.md` |
```

to:

```md
| Phase 0 — Test inventory and risk evaluation | Complete | `docs/superpowers/plans/2026-05-19-testing-phase-0-inventory.md` |
```

- [ ] **Step 2: Add a short completion note under `Plan Decomposition`**

Add:

```md
Phase 0 output docs:

- `docs/testing/test-inventory.md`
- `docs/testing/backend-test-matrix.md`
- `docs/testing/frontend-test-matrix.md`
```

- [ ] **Step 3: Verify the spec reflects completion**

Run:

```bash
rg -n "Phase 0 — Test inventory and risk evaluation \| Complete|docs/testing/test-inventory.md|docs/testing/backend-test-matrix.md|docs/testing/frontend-test-matrix.md" docs/superpowers/specs/2026-05-19-frontend-testing.md
```

Expected: matches for the completed status row and the three output docs.

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/specs/2026-05-19-frontend-testing.md
git commit --no-gpg-sign -m "docs: mark testing inventory phase complete"
```

### Task 6: Final Verification

**Files:**
- Verify: `docs/testing/test-inventory.md`
- Verify: `docs/testing/backend-test-matrix.md`
- Verify: `docs/testing/frontend-test-matrix.md`
- Verify: `docs/superpowers/specs/2026-05-19-frontend-testing.md`

- [ ] **Step 1: Run the inventory verification checks**

Run:

```bash
test -f docs/testing/test-inventory.md
test -f docs/testing/backend-test-matrix.md
test -f docs/testing/frontend-test-matrix.md
rg -n "^# Expensor Test Inventory$" docs/testing/test-inventory.md
rg -n "^# Backend Test Matrix$" docs/testing/backend-test-matrix.md
rg -n "^# Frontend Test Matrix$" docs/testing/frontend-test-matrix.md
```

Expected: all commands succeed.

- [ ] **Step 2: Review the staged changes**

Run:

```bash
git diff -- docs/testing docs/superpowers/specs/2026-05-19-frontend-testing.md docs/superpowers/plans/2026-05-19-testing-phase-0-inventory.md
```

Expected: diff shows only the inventory docs and matching spec/plan status updates.

- [ ] **Step 3: Commit the final Phase 0 work**

```bash
git add docs/testing docs/superpowers/specs/2026-05-19-frontend-testing.md docs/superpowers/plans/2026-05-19-testing-phase-0-inventory.md
git commit --no-gpg-sign -m "docs: complete testing inventory phase"
```

---

## Self-Review

### Spec Coverage

- Covers the Phase 0 requirement for a thorough test-case evaluation before implementation.
- Produces concrete inventory artifacts later phases can consume.
- Includes explicit status reflection back into the top-level spec.

### Placeholder Scan

- No `TODO` or `TBD` placeholders remain.
- All tasks include exact file paths and concrete commands.

### Type Consistency

- Uses the same Phase 0 naming across the spec, program index, and this plan.
- Uses the same output docs in tasks and exit criteria.

## Exit Criteria

- `docs/testing/test-inventory.md` exists with layer rules, recommendations, and deferred cases
- `docs/testing/backend-test-matrix.md` exists with grouped backend domains
- `docs/testing/frontend-test-matrix.md` exists with grouped frontend targets
- the testing spec marks Phase 0 as `Complete` after execution

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-19-testing-phase-0-inventory.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
