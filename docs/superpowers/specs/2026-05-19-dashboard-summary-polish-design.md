# Dashboard Summary Polish Design

**Date:** 2026-05-19
**Status:** Complete
**Scope:** Dashboard summary hierarchy, clickthrough correctness, donut alignment, and heatmap consistency follow-up

## Goal

Refine the dashboard layout and interactions so the high-value summary widgets are easier to use and visually consistent, while preserving the longer-range analytical widgets below them.

This pass focuses on:

- replacing the `Current Month` / `All Time` split sections with one shared summary-area toggle
- fixing summary-widget clickthroughs so current-month navigation carries the correct month filter
- tightening donut alignment and removing zero-value legend noise
- moving `Monthly Spend` and `Daily Spend` higher in the page hierarchy
- investigating and fixing the remaining weekday/hour heatmap consistency issue using the test database

This pass does not cover:

- command palette / keyboard shortcut work
- bulk transaction selection / bulk mute
- final end-to-end cleanup across every remaining dashboard task

## User Intent

The dashboard currently feels duplicative because many summary widgets exist in both `Current Month` and `All Time` sections. The user wants one shared summary stack controlled by a dashboard-level toggle, while the inherently historical widgets remain below unchanged.

The user also called out concrete regressions and polish issues:

- `Spend by Category` clickthrough from the current-month summary does not carry the month filter
- donut cards appear visually misaligned
- `Monthly Spend` and `Daily Spend` are buried lower than they should be
- the weekday/hour heatmap still appears inconsistent with the transactions page
- the generalized `Spend Breakdown` timeline should not show zero-contribution series in legends/tooltips where that creates clutter

## Layout Model

The dashboard becomes a single page with two layers:

### 1. Summary Layer

A shared dashboard-level toggle sits above the summary widget stack:

- `Current Month`
- `All Time`

This toggle controls only widgets where alternate granularity is useful:

- merged `Total Spend` + `Total Transactions` widget
- `Spend by Category`
- `Needs / Wants / Savings`
- `By Category`
- `By Bucket`
- `By Label`
- `By Source`

These widgets render once and swap datasets based on the active toggle.

### 2. Historical Layer

These widgets remain outside the toggle and keep their historical scope:

- `Monthly Spend`
- `Daily Spend`
- `Spend Breakdown (12 months)` line graph
- `Spending Patterns` heatmaps
- `Recent transactions`

`Monthly Spend` and `Daily Spend` should be placed high on the page, immediately below the summary layer, because they are broadly useful and should not feel buried behind an all-time-only section.

## Summary Widgets

### Merged KPI Widget

Replace the separate `Total Transactions` and `Total Spend` cards with one combined KPI card:

- primary value: total spend in blue, as it is today
- secondary value: total transaction count in muted/grey styling

Both values are clickable:

- in `Current Month` mode they navigate to `/transactions` with the current calendar month applied in the configured app timezone
- in `All Time` mode they navigate to `/transactions` without month filtering

### Spend By Category

This widget stays as a ranked bar list. Clicking a category navigates to `/transactions` with:

- `category=<selected>`
- current-month date bounds when the summary toggle is `Current Month`
- no date bounds when the summary toggle is `All Time`

### Donut Widgets

The donut cards should share the same card height and inner alignment rules so borders, totals, donut centers, and legends feel even across the row.

Zero-value entries must be removed before rendering:

- donut slices
- legend rows
- tooltip rows where applicable

This rule applies consistently across all summary donut dimensions, not just categories.

## Historical Widgets

### Monthly Spend And Daily Spend

These stay historical and should move toward the top of the post-summary layout.

They do not participate in the summary toggle.

### Spend Breakdown Timeline

The existing `Spend Breakdown (12 months)` line graph remains a historical widget and keeps its `Labels / Categories / Buckets` toggle.

The Y-axis keeps compact formatting like `46.4K`.

Zero-only series should be excluded from the rendered legend and tooltip list for the active dimension, so the graph only describes categories/labels/buckets that materially contribute within the visible time window.

### Spending Patterns Heatmaps

The heatmaps remain historical widgets and keep their month/year controls.

However, clickthrough behavior must mirror the exact rendered bucket scope:

- weekday
- hour
- timezone
- month range when month-scoped

Without all four, the transactions-page results will not match the heatmap cell.

## Heatmap Investigation Plan

This issue must be verified against the real test database before changing code.

The debugging approach is:

1. query the heatmap bucket data semantics directly in the test DB using:
   `docker exec -it expensor-dev-postgres psql -U expensor`
2. compare a chosen weekday/hour bucket against an equivalent transactions query
3. compare those DB results to the frontend clickthrough URL
4. compare that URL to the transactions-page filter parsing and resulting API request

Interpretation:

- if DB aggregate and equivalent DB transaction query agree, but UI clickthrough disagrees, the bug is in frontend filter propagation
- if DB aggregate and equivalent DB transaction query disagree, the bug is in backend/store query semantics
- if month-scoped heatmap clickthrough omits date bounds, that is a frontend bug regardless of backend correctness

## Timezone And Date Rules

For `Current Month` summary mode:

- month bounds must be derived from the calendar month in the configured app timezone from `app_config`
- browser-local month boundaries must not be used for clickthrough filters

For heatmap clickthrough:

- timezone must be preserved explicitly so the transactions page evaluates weekday/hour using the same timezone contract as the heatmap

## Expected Code Changes

Likely files:

- `frontend/src/pages/Dashboard.tsx`
  - replace split-section summary rendering with a shared top-level summary toggle
  - merge KPI cards
  - move `Monthly Spend` and `Daily Spend` higher
  - ensure summary clickthroughs include active granularity filters
  - filter zero-only legend content in the spend-breakdown timeline
- `frontend/src/components/WeekdayHourHeatmap.tsx`
  - keep weekday+hour clickthrough shape if already present
  - ensure month-scoped clickthrough metadata is propagated by the caller
- `frontend/src/lib/utils.ts`
  - retain compact numeric formatting helper used by the line-chart Y axis
- `frontend/src/pages/Transactions.tsx`
  - likely needs verification that weekday/hour/date/tz params are honored together
- `backend/internal/store/store.go`
  - may need query adjustments if DB verification shows aggregate/list mismatch
- `backend/internal/store/store_test.go`
  - add or refine integration coverage for the remaining heatmap mismatch case

## Testing

Testing for this pass should include:

### Backend

- reproduce the heatmap mismatch against store-level queries if it exists
- add regression coverage for the confirmed bug path

### Frontend

- run `npm run lint`
- verify that summary clickthrough URLs include current-month range when appropriate
- verify heatmap clickthrough preserves weekday/hour/date/tz contract

### Manual / DB Verification

- inspect at least one all-time heatmap bucket in the test DB
- inspect at least one month-scoped heatmap bucket in the test DB
- compare those results against the transactions page filters produced from the UI logic

## Risks

- the summary-toggle refactor touches several dashboard widgets at once, so alignment improvements can easily regress clickthrough behavior if the filter contract is not centralized
- heatmap inconsistency may be split across backend semantics and frontend URL construction, so fixing only one side may leave the issue partially visible
- zero-series filtering in the line graph must be done carefully so month alignment remains stable across the remaining series
