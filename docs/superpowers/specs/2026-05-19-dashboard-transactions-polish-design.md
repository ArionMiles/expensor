# Dashboard And Transactions Polish Pass — Design Spec

**Date:** 2026-05-19
**Status:** Approved
**Scope:** Auto-apply correctness, dashboard/query consistency, dashboard information architecture, shortcuts, multi-select mute, and chart/layout polish

---

## Problem

Recent changes exposed a mixed set of correctness and UX gaps:

1. Merchant-wide label application does not affect future transactions, and the same may be true for category/bucket automation.
2. The weekday/hour heatmap can show a transaction count that does not match the transaction list opened from a clicked cell.
3. The `Spend by Label` panel is too narrow for the data it can represent and should expand to labels, categories, and buckets.
4. The app lacks core keyboard shortcuts for sidebar toggling and fast page navigation.
5. Muting transactions one-by-one is slow on larger review passes.
6. Dashboard widget heights are slightly misaligned in at least one shared row.
7. Dashboard metrics do not clearly separate current-month metrics from all-time metrics.
8. Spend chart Y-axis labels can clip because the number formatting is too wide.

These issues cross backend query correctness, ingest-time rule persistence, and frontend interaction/layout behavior, so treating them as one coordinated pass reduces rework.

---

## Goals

- Reproduce the auto-apply and heatmap mismatches with tests before fixing them.
- Make ingest-time merchant automation consistent for labels, categories, buckets, and mute rules.
- Make heatmap cell counts and drilldown transaction listings use the same filter semantics.
- Reorganize the dashboard so current-month widgets are clearly separated from all-time widgets.
- Generalize the spend breakdown panel to support `Labels`, `Categories`, and `Buckets`.
- Add app-level shortcuts for sidebar toggle and command palette navigation.
- Support multi-select mute from the transactions table.
- Fix the dashboard card-height mismatch and chart-axis clipping.

---

## Non-Goals

- Adding a dashboard-wide date filter.
- Adding command-palette actions beyond page navigation in this pass.
- Redesigning the overall visual language of the dashboard.
- Reworking transaction bulk actions beyond muting.
- Changing the meaning of existing muted-transaction behavior outside the new bulk selection flow.

---

## Solution Overview

The implementation should proceed backend-first, then layer frontend enhancements on top of stable data contracts.

1. Add failing backend tests for future merchant-rule auto-application and for heatmap/drilldown count consistency.
2. Extend the writer/store path so new transactions receive all persisted merchant automation before commit.
3. Unify heatmap aggregation and transaction drilldown around the same timezone-aware weekday/hour filter contract.
4. Expose current-month dashboard data using the timezone stored in `app_config` under `app.timezone`.
5. Reorganize the dashboard UI into `Current Month` and `All Time` sections and generalize the spend breakdown widget.
6. Add global shortcuts, transaction multi-select mute, and final dashboard layout/formatting polish.

---

## Backend Design

### 1. Future auto-apply rules

The source of truth for merchant automation remains in the database:

- Merchant label mappings for labels
- `merchant_categories` rows for category/bucket
- `muted_merchants` rows for mute behavior

The writer already applies muted-merchant rules after transaction insert and preserves extracted category/bucket on upsert. This pass should make the write path apply all merchant automation consistently for every newly written transaction.

Required behavior:

- A new transaction whose merchant matches a saved merchant-label mapping receives the mapped label(s) automatically.
- A new transaction whose merchant matches a persisted merchant category rule receives the mapped category/bucket automatically.
- Existing user-edited transaction fields remain protected during reprocessing.
- Reprocessing should still fill empty category/bucket fields from merchant automation when the stored value is blank.

Implementation shape:

- Keep the write transaction in `backend/pkg/writer/postgres/postgres.go`.
- After collecting inserted transaction IDs, apply:
  1. label mappings,
  2. merchant category/bucket rules,
  3. muted merchant rules,
  before commit.
- Reuse store/query semantics already used for bulk merchant actions where practical, but keep ingest-time automation in the writer layer so future transactions are covered regardless of frontend behavior.

### 2. Heatmap/drilldown consistency

The heatmap aggregate query and the clicked transaction list must use one canonical contract:

- same muted filtering,
- same timezone,
- same hour extraction,
- same date-range boundaries,
- same weekday interpretation.

The likely source of mismatch is that the heatmap groups directly on raw timestamps while `ListTransactions` already supports timezone-aware hour filtering. This pass should make both paths evaluate weekday/hour in the same configured timezone.

Required behavior:

- The count shown in a weekday/hour cell equals the number of transactions returned by the drilldown route for the same cell and same date range.
- The timezone comes from `app_config["app.timezone"]`, with UTC fallback only when unset.
- The weekday encoding remains explicit and documented: PostgreSQL DOW convention `0=Sunday` through `6=Saturday`, unless both paths are migrated together to a different convention.

Implementation shape:

- Extend `GetSpendingHeatmap` to evaluate weekday/hour using the configured timezone.
- Ensure dashboard click-through builds a `Transactions` URL with exactly the filters that `ListTransactions` expects.
- If weekday filtering does not yet exist in `ListTransactions`, add it rather than approximating with only hour/range.
- Keep all drilldown filtering in the backend/store contract, not in ad hoc frontend filtering.

### 3. Current-month dashboard data

The dashboard does not have a date filter in this pass. Current-month widgets should therefore always be based on the current calendar month in the configured timezone from `app_config`.

Required behavior:

- Current-month calculations use the configured timezone, not browser local time.
- The month label shown in the UI reflects that same timezone-derived current month.
- All-time widgets continue using the full unfiltered dataset.

Implementation shape:

- Add or extend store/API methods so dashboard data can be requested by period scope:
  - current calendar month in configured timezone,
  - all time.
- Keep month-boundary calculation in backend code so the frontend is rendering a defined result rather than reimplementing date logic.

---

## Frontend Design

### 1. Dashboard information architecture

The dashboard should be explicitly split into two sections:

1. `Current Month (<Month YYYY>)`
2. `All Time`

Current-month widgets should be ordered from most directly actionable spend insight to less critical context. Initial ordering:

1. Total spend
2. Total transactions
3. Spend breakdown
4. Category/bucket donuts
5. Needs/Wants/Savings
6. MoM comparisons

All-time widgets should hold broader history and longitudinal views such as long-range spend trends and annual heatmaps.

### 2. Spend breakdown widget

The current `Spend by Label` widget should become a generalized spend breakdown panel covering:

- `Labels`
- `Categories`
- `Buckets`

Behavior:

- The panel title is `Spend Breakdown`.
- A local toggle switches the displayed dimension.
- Empty states, legends, and click-through labels reflect the active dimension.
- Reuse the existing line-chart structure unless a small extraction is needed for the three-way toggle, and shorten Y-axis formatting to 1 decimal abbreviated values like `46.4K`.

### 3. Shortcuts

Add app-level keyboard shortcuts:

- `Cmd+.` to toggle the sidebar
- `Cmd+K` to open a command palette for page navigation

Behavior:

- Do not trigger shortcuts while focus is inside `input`, `textarea`, or `contenteditable` elements.
- The command palette only needs to support route navigation in this pass.
- The palette should feel consistent with the existing app styling and should live at the app layout level so it is accessible from all pages.

### 4. Multi-select mute

The transactions table should support row selection for bulk mute.

Behavior:

- Add a checkbox column immediately to the right of the mute button column.
- Support selecting individual rows and page-level select-all for the current page.
- Show a bulk mute action when one or more rows are selected.
- Reuse existing mute APIs, invalidation, and muted-transaction refresh behavior.
- Clear selection when the underlying page/filter dataset changes so selection does not leak across unrelated views.

### 5. Layout and chart polish

- Normalize heights and border alignment between the Needs/Wants/Savings widget and MoM comparison widgets.
- Adjust spacing and layout only as needed to keep the current visual language.
- Shorten Y-axis labels in the spend line chart to avoid clipping.

---

## Data Flow

### Future merchant automation

1. User creates or confirms a merchant-wide label/category/bucket rule.
2. Rule is persisted in the database.
3. A future transaction arrives through the writer.
4. The writer inserts or upserts the base transaction row.
5. Before commit, the writer applies all matching merchant automation for labels, categories/buckets, and mute rules.
6. The committed transaction is immediately consistent with prior merchant-wide actions.

### Heatmap drilldown

1. Dashboard heatmap fetches timezone-aware aggregate buckets.
2. User clicks a weekday/hour cell.
3. Dashboard navigates to `Transactions` with explicit weekday/hour/range filters derived from that cell.
4. `Transactions` calls the list endpoint with those same filters.
5. The returned transaction count matches the heatmap bucket count for that cell.

### Current-month dashboard

1. Backend reads `app.timezone` from `app_config`.
2. Backend derives the current month boundaries in that timezone.
3. Backend computes current-month dashboard aggregates using those boundaries.
4. Frontend renders the section header and widgets using the returned current-month dataset.

---

## Testing Strategy

### Required first step

Before any production fix:

- write a failing test for future merchant-label auto-application,
- verify whether category/bucket future auto-application already works or fails,
- write a failing test for heatmap/drilldown count consistency against the test DB.

### Backend tests

- Writer integration tests for new-transaction merchant label auto-apply.
- Writer or store integration tests for future category/bucket auto-apply and reprocessing behavior.
- Store integration tests that compare a heatmap bucket count with `ListTransactions` using the corresponding drilldown filters.
- Handler tests for any new API parameters or response shapes introduced for dashboard period scope or heatmap timezone handling.

### Frontend tests

- Add targeted interaction tests only where the repo already supports them or where lightweight additions are justified.
- Prefer backend/data-contract tests for the correctness issues and keep frontend testing focused on non-trivial interaction logic such as shortcut gating and command-palette navigation if test infrastructure is available.

---

## Risks And Mitigations

### Timezone drift

Risk:
heatmap grouping, current-month metrics, and transaction drilldowns can diverge if they compute local time in different places.

Mitigation:
derive timezone-sensitive boundaries and extraction semantics in backend/store code and pass only explicit filters or rendered results to the frontend.

### Reprocessing regressions

Risk:
future-transaction automation fixes could accidentally overwrite user-edited category/bucket fields or duplicate labels.

Mitigation:
cover both fresh insert and reprocessing paths with integration tests and preserve existing `ON CONFLICT` protections plus idempotent label insertion.

### UI state complexity

Risk:
global shortcuts, command palette state, sidebar state, and table row selection can conflict with existing page interactions.

Mitigation:
keep shortcut handling centralized, ignore editable targets, and clear selection on dataset changes.

---

## Open Decisions Resolved

- Command palette shortcut: `Cmd+K`
- Spend breakdown dimensions: `Labels`, `Categories`, `Buckets`
- Current-month period basis: current calendar month in the timezone saved in `app_config`
- Dashboard date filter: not part of this pass

---

## Expected Outcome

After this pass:

- future transactions honor merchant-wide label and category/bucket rules,
- heatmap counts and drilldown results are consistent,
- the dashboard clearly distinguishes current-month metrics from all-time views,
- the spend breakdown panel supports three dimensions,
- keyboard navigation and bulk muting reduce transaction-review friction,
- dashboard rows align cleanly and spend-chart labels no longer clip.
