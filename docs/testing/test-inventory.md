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
  - labels/categories/buckets taxonomy endpoints
- Deferred from current OpenAPI baseline:
  - rules and rules import/export
  - dashboard/stats
  - auth/OAuth callback flows
  - muted merchants and merchant categorization

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

## Deferred Cases

- Full OAuth browser automation against real providers
- Broad snapshot coverage of frontend pages
- Exhaustive browser coverage of taxonomy CRUD once frontend unit tests already cover the interactions
- Low-level duplicate API cases already covered by unit and component layers
- Playwright promotion to required CI status before flake data exists
