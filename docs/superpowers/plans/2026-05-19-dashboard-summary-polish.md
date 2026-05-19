# Dashboard Summary Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor the dashboard summary area into a shared current-month/all-time toggle, fix summary clickthroughs and donut alignment, and resolve the remaining weekday/hour heatmap consistency issue.

**Architecture:** Start by reproducing the heatmap mismatch against the real query contract and pin it down with backend tests, because the frontend refactor should consume a stable weekday/hour/date/timezone filter contract. Then reshape the dashboard summary layer around a single top-level toggle, move the historical charts higher, and finish with layout cleanup plus frontend verification.

**Tech Stack:** Go (`pgx/v5`, testcontainers), React 18, React Router, React Query, Tailwind

---

## Status

- Status: Complete
- Last updated: 2026-05-17
- Owner: Unassigned
- Execution note: Status reconciled after implementation; dashboard summary polish is complete.

---

## File Map

| File | Responsibility |
|------|----------------|
| `backend/internal/store/store.go` | Heatmap/list query semantics and any timezone/filter fixes uncovered by DB verification |
| `backend/internal/store/store_test.go` | Integration coverage for confirmed heatmap mismatch cases |
| `backend/internal/api/handlers.go` | Preserve heatmap clickthrough contract if request parsing changes are needed |
| `backend/internal/api/handlers_test.go` | Handler coverage for weekday/hour/date/tz parsing if changed |
| `frontend/src/pages/Dashboard.tsx` | Summary-toggle refactor, merged KPI card, widget ordering, clickthrough fixes, donut layout, timeline legend filtering |
| `frontend/src/pages/Transactions.tsx` | Verification target for weekday/hour/date/tz URL filters, modify only if parsing or URL-state handling is missing something |
| `frontend/src/components/WeekdayHourHeatmap.tsx` | Weekday/hour click callback shape if further propagation changes are needed |
| `frontend/src/lib/utils.ts` | Shared month-range helpers or compact-number formatting if needed |

---

### Task 1: Reproduce the remaining heatmap mismatch against DB and backend tests

**Files:**
- Modify: `backend/internal/store/store_test.go`
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/api/handlers_test.go`
- Modify: `backend/internal/api/handlers.go` (only if request parsing needs adjustment)

- [ ] **Step 1: Inspect one all-time heatmap cell directly in the dev test database**

Run:

```bash
docker exec -i expensor-dev-postgres psql -U expensor -d expensor <<'SQL'
SELECT key, value FROM app_config WHERE key IN ('app.timezone');
SQL
```

Then inspect one concrete weekday/hour bucket using the configured timezone:

```bash
docker exec -i expensor-dev-postgres psql -U expensor -d expensor <<'SQL'
WITH bucket AS (
  SELECT
    EXTRACT(DOW FROM timestamp AT TIME ZONE 'Asia/Kolkata')::int AS weekday,
    EXTRACT(HOUR FROM timestamp AT TIME ZONE 'Asia/Kolkata')::int AS hour,
    COUNT(*) AS txn_count,
    COALESCE(SUM(amount), 0) AS total_amount
  FROM transactions
  WHERE muted = false
  GROUP BY 1, 2
)
SELECT * FROM bucket
ORDER BY txn_count DESC, total_amount DESC
LIMIT 10;
SQL
```

Expected: identify one real `(weekday, hour)` pair with a non-trivial count/amount to use in later test assertions.

- [ ] **Step 2: Compare that bucket to an equivalent transactions query**

Replace `WEEKDAY` and `HOUR` with the chosen values from Step 1 and run:

```bash
docker exec -i expensor-dev-postgres psql -U expensor -d expensor <<'SQL'
SELECT
  COUNT(*) AS txn_count,
  COALESCE(SUM(amount), 0) AS total_amount
FROM transactions
WHERE muted = false
  AND EXTRACT(DOW FROM timestamp AT TIME ZONE 'Asia/Kolkata')::int = WEEKDAY
  AND EXTRACT(HOUR FROM timestamp AT TIME ZONE 'Asia/Kolkata')::int = HOUR;
SQL
```

Expected: counts and amounts either match the bucket exactly or reveal the backend/store query mismatch that needs fixing.

- [ ] **Step 3: Add a failing store regression test for the confirmed mismatch path**

Append a test to `backend/internal/store/store_test.go` near the other heatmap tests. Use seeded timestamps that make the mismatch explicit. The target should compare:

```go
heatmap, err := ts.GetSpendingHeatmap(ctx, from, to)
txns, total, err := ts.ListTransactions(ctx, store.ListFilter{
	PageSize: 100,
	Weekday:  ptrInt(targetWeekday),
	HourFrom: ptrInt(targetHour),
	HourTo:   ptrInt(targetHour),
	Timezone: "Asia/Kolkata",
	From:     from,
	To:       to,
})
```

Assert both transaction count and summed amount match the chosen heatmap bucket:

```go
if bucket.Count != total {
	t.Fatalf("bucket count %d != list total %d", bucket.Count, total)
}
var listedAmount float64
for _, tx := range txns {
	listedAmount += tx.Amount
}
if listedAmount != bucket.Amount {
	t.Fatalf("bucket amount %v != listed amount %v", bucket.Amount, listedAmount)
}
```

- [ ] **Step 4: Run the focused backend tests and verify the new test fails for the expected reason**

Run:

```bash
cd backend
go test ./internal/store -run 'TestHeatmapBucketMatchesListTransactionsForWeekdayHour|Test.*Heatmap.*' -count=1
```

Expected: the newly added regression test fails if the mismatch still exists. If it passes immediately, keep the evidence and treat the remaining issue as frontend/filter propagation instead.

- [ ] **Step 5: Implement the minimal backend fix if the mismatch is in query semantics**

If Step 4 fails due to store semantics, fix only the confirmed issue in `backend/internal/store/store.go`, such as:

- inconsistent timezone expression between aggregate and list filters
- missing `From` / `To` range handling in one path
- muted-filter mismatch

Keep the fix narrow and preserve the existing `ListFilter` contract.

- [ ] **Step 6: Add or adjust handler coverage if request parsing is part of the bug**

If DB/store results are correct but the API contract needs help, add/adjust a handler test in `backend/internal/api/handlers_test.go` that proves `weekday`, `hour_from`, `hour_to`, `tz`, `date_from`, and `date_to` all survive parsing together.

- [ ] **Step 7: Re-run the focused backend tests**

Run:

```bash
cd backend
go test ./internal/store -run 'TestHeatmapBucketMatchesListTransactionsForWeekdayHour|Test.*Heatmap.*' -count=1
go test ./internal/api -run 'TestHandleGetHeatmap|TestHandleListTransactions' -count=1
```

Expected: all targeted tests pass.

- [ ] **Step 8: Commit the heatmap-consistency fix**

```bash
git add backend/internal/store/store.go backend/internal/store/store_test.go backend/internal/api/handlers.go backend/internal/api/handlers_test.go
git commit --no-gpg-sign -m "fix: align heatmap drilldowns with transaction filters"
```

---

### Task 2: Refactor the dashboard summary area around a shared top-level toggle

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx`
- Modify: `frontend/src/lib/utils.ts` (only if a month-range helper reduces duplication)

- [ ] **Step 1: Write a failing dashboard interaction test if a frontend test harness already exists**

Check for an existing page/component test pattern under `frontend/src/**/*.test.tsx`. If there is an established harness for page-level rendering, add a targeted test for:

- summary toggle switching between current-month and all-time KPI values
- `Spend by Category` clickthrough including current-month date bounds when current-month is active

If there is no existing dashboard/page test harness, skip creating a new test framework and rely on `npm run lint` plus manual URL verification in later steps.

- [ ] **Step 2: Add a page-level summary toggle state**

In `frontend/src/pages/Dashboard.tsx`, add:

```ts
type SummaryMode = 'current_month' | 'all_time'
const [summaryMode, setSummaryMode] = useState<SummaryMode>('current_month')
```

and a helper:

```ts
const activeSummary = summaryMode === 'current_month' ? dashboardData?.current_month : dashboardData?.all_time
```

- [ ] **Step 3: Replace split summary sections with one shared summary block**

In `Dashboard.tsx`, remove the duplicated `Current Month` / `All Time` summary rendering and instead render one top section containing:

- a top-level toggle control
- one merged KPI card
- one `Spend by Category` card
- one row of summary donuts

Keep `Monthly Spend`, `Daily Spend`, `Spend Breakdown (12 months)`, `Spending Patterns`, and `Recent transactions` outside this toggle-controlled block.

- [ ] **Step 4: Merge the KPI cards into one clickable summary card**

Inside `Dashboard.tsx`, replace the separate cards with a single summary widget that renders:

```tsx
<p className="text-xs uppercase tracking-wider text-muted-foreground">Summary</p>
<button ...>{formatCurrency(activeSummary.stats.total_base, currency)}</button>
<button ...>{activeSummary.stats.total_count.toLocaleString('en-IN')} transactions</button>
```

The spend button and count button both navigate to `/transactions`, but:

- in `current_month` mode include month-scoped `date_from` / `date_to`
- in `all_time` mode do not include month bounds

- [ ] **Step 5: Add a current-month range helper using the configured timezone label already returned by the backend**

Inside `Dashboard.tsx`, add a helper that derives the current-month filter range from the active dashboard state rather than browser-local assumptions. If needed, centralize the range creation into a small utility such as:

```ts
function buildMonthRangeParams(now: Date): { from: string; to: string } { ... }
```

Only add a shared helper in `frontend/src/lib/utils.ts` if the date-range code is duplicated in more than one place.

- [ ] **Step 6: Fix `Spend by Category` clickthrough to honor summary mode**

Update the current category-row click handler so it builds the URL with:

```ts
category=<selected>
show_filters=1
```

and, when `summaryMode === 'current_month'`, also adds the active current-month `date_from` / `date_to`.

- [ ] **Step 7: Move `Monthly Spend` and `Daily Spend` directly below the summary block**

In `Dashboard.tsx`, ensure the historical ordering is:

1. summary block
2. monthly spend / daily spend row
3. spend breakdown timeline
4. spending patterns
5. recent transactions

Do not gate these lower widgets behind the summary toggle.

- [ ] **Step 8: Run frontend typecheck**

```bash
cd frontend
npm run lint
```

Expected: pass.

- [ ] **Step 9: Commit the summary-toggle refactor**

```bash
git add frontend/src/pages/Dashboard.tsx frontend/src/lib/utils.ts
git commit --no-gpg-sign -m "feat: add shared dashboard summary toggle"
```

---

### Task 3: Tighten donut alignment and remove zero-value legend noise

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx`

- [ ] **Step 1: Write down the zero-value rule directly in the donut data preparation**

In the donut/breakdown helper logic, ensure the entries are built from:

```ts
Object.entries(data ?? {})
  .filter(([, value]) => value > 0)
  .sort(([, a], [, b]) => b - a)
```

Do this for all summary donuts, not just one dimension.

- [ ] **Step 2: Normalize donut-card height and inner alignment**

In `Dashboard.tsx`, update the donut card container classes so every summary donut uses:

```tsx
className="flex h-full flex-col rounded-lg border border-border bg-card p-4 shadow-sm"
```

and center the donut region consistently:

```tsx
<div className="flex flex-1 flex-col items-center justify-center gap-4">
```

Apply the same structural alignment to every summary donut card.

- [ ] **Step 3: Verify totals and legends render cleanly when only a few slices remain**

Check that a donut with 1-2 non-zero entries still aligns correctly and does not leave an awkward legend gap or mismatched footer.

- [ ] **Step 4: Run frontend typecheck**

```bash
cd frontend
npm run lint
```

Expected: pass.

- [ ] **Step 5: Commit the donut polish**

```bash
git add frontend/src/pages/Dashboard.tsx
git commit --no-gpg-sign -m "fix: align dashboard donut summaries"
```

---

### Task 4: Filter zero-only series and polish the spend-breakdown timeline

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx`

- [ ] **Step 1: Filter out zero-only series before legend and tooltip rendering**

In the `Spend Breakdown (12 months)` timeline component, derive:

```ts
const visibleSeries = series.filter((s) => s.data.some((value) => value > 0))
```

Then use `visibleSeries` instead of the full `series` for:

- legend rows
- paths
- point markers
- tooltip rows

- [ ] **Step 2: Keep the compact Y-axis formatting**

Use the existing compact formatter:

```ts
label: formatCompactNumber(maxVal * f)
```

and ensure the left padding remains wide enough to avoid clipping.

- [ ] **Step 3: Run frontend typecheck**

```bash
cd frontend
npm run lint
```

Expected: pass.

- [ ] **Step 4: Commit the timeline polish**

```bash
git add frontend/src/pages/Dashboard.tsx
git commit --no-gpg-sign -m "fix: filter empty spend breakdown series"
```

---

### Task 5: Fix heatmap clickthrough propagation in the dashboard and transactions page

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx`
- Modify: `frontend/src/pages/Transactions.tsx`
- Modify: `frontend/src/components/WeekdayHourHeatmap.tsx` (only if callback payload changes)

- [ ] **Step 1: Verify the dashboard clickthrough URL shape against the confirmed backend contract**

In `Dashboard.tsx`, inspect the weekday/hour heatmap click handler and ensure it constructs:

```ts
weekday
hour_from
hour_to
show_filters=1
tz=<active timezone when available>
date_from/date_to when month-scoped
```

If `tz` is not currently propagated, add it.

- [ ] **Step 2: Verify the transactions page preserves all four filters together**

In `frontend/src/pages/Transactions.tsx`, ensure URL parsing and `TransactionFilters` construction include all of:

- `weekday`
- `hour_from`
- `hour_to`
- `tz`
- `date_from`
- `date_to`

If one is being dropped or overwritten during URL-state updates, fix that path minimally.

- [ ] **Step 3: Reproduce the month-scoped clickthrough issue locally**

With the UI logic in mind, confirm that the dashboard handler for a month-scoped heatmap cell includes the same `date_from` / `date_to` that were used to request that month’s heatmap data.

- [ ] **Step 4: Run frontend typecheck**

```bash
cd frontend
npm run lint
```

Expected: pass.

- [ ] **Step 5: Commit the heatmap clickthrough fix**

```bash
git add frontend/src/pages/Dashboard.tsx frontend/src/pages/Transactions.tsx frontend/src/components/WeekdayHourHeatmap.tsx
git commit --no-gpg-sign -m "fix: preserve heatmap drilldown filters in dashboard"
```

---

### Task 6: Final verification and cleanup

**Files:**
- Modify only if verification reveals a concrete issue

- [ ] **Step 1: Re-run the focused backend tests**

```bash
cd backend
go test ./internal/store -run 'TestHeatmapBucketMatchesListTransactionsForWeekdayHour|Test.*Heatmap.*' -count=1
go test ./internal/api -run 'TestHandleGetHeatmap|TestHandleListTransactions' -count=1
```

Expected: pass.

- [ ] **Step 2: Re-run frontend typecheck**

```bash
cd frontend
npm run lint
```

Expected: pass.

- [ ] **Step 3: Re-run the DB spot checks for one all-time and one month-scoped heatmap cell**

Use the same `docker exec -i expensor-dev-postgres psql -U expensor -d expensor` workflow from Task 1 to confirm the chosen bucket and equivalent transaction query still agree after the code changes.

- [ ] **Step 4: Inspect the git diff for accidental spillover**

```bash
git status --short
git diff --stat HEAD~4..HEAD
```

Expected: only the intended dashboard/store/api files are included.

- [ ] **Step 5: Commit any verification-only follow-up fixes**

If Step 1-4 reveal a concrete remaining bug, make the smallest fix and commit it:

```bash
git add <exact files>
git commit --no-gpg-sign -m "fix: polish dashboard summary interactions"
```
