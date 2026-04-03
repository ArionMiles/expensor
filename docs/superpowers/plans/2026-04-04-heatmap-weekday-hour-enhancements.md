# WeekdayHour Heatmap Enhancements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the existing weekday/hour heatmap with optional month-range filtering, a full-width responsive SVG layout, month navigation controls, and a tooltip that shows month context when a filter is active.

**Architecture:** No DB migration. The store method gains optional `from, to *time.Time` params (nil = all-time). The handler parses `?from=`/`?to=` RFC3339 query params. The frontend API client and hook become parameterized. `SpendingPatternsSection` grows `{ year, month } | null` nav state with ← Month Year → controls that derive date ranges and pass them through the hook. `WeekdayHourHeatmap` switches from fixed-width SVG to `width="100%"` with a `viewBox`.

**Tech Stack:** Go 1.23+, pgx/v5, React 18, TypeScript, TanStack Query, Tailwind CSS, SVG.

---

## File Map

| File | Change |
|------|--------|
| `backend/internal/store/store.go` | `GetSpendingHeatmap(ctx, from, to *time.Time)` — conditional WHERE clause |
| `backend/internal/api/store.go` | Update `Storer` interface signature |
| `backend/internal/api/handlers.go` | Parse `?from=`/`?to=` RFC3339 params; pass to store; return 400 on bad parse |
| `backend/internal/api/handlers_test.go` | Update mock signature; add 2 new tests (with-range 200, invalid-from 400) |
| `frontend/src/api/client.ts` | `api.stats.heatmap(from?, to?)` |
| `frontend/src/api/queries.ts` | `queryKeys.heatmap` → function; `useHeatmapData(from?, to?, enabled?)` |
| `frontend/src/components/WeekdayHourHeatmap.tsx` | `width="100%"`, remove `overflow-x-auto`, add `monthLabel?` prop for tooltip |
| `frontend/src/pages/Dashboard.tsx` | `SpendingPatternsSection` owns nav state + controls; `Dashboard` drops standalone heatmap fetch |

---

## Task 1: Backend — parameterized GetSpendingHeatmap + handler + mock

**Files:**
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/api/store.go`
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/handlers_test.go`

### Background

`GetSpendingHeatmap(ctx)` runs unconditional aggregates. Adding `from, to *time.Time` keeps all-time behaviour when both are nil and adds `WHERE timestamp >= $1 AND timestamp <= $2` when both are set. The handler mirrors the `date_from`/`date_to` pattern in `HandleListTransactions`. TDD order: update the mock first (compile error), then fix `Storer`, then `store.Store`, then the handler.

- [ ] **Step 1: Update mock signature and add 2 new handler tests**

In `backend/internal/api/handlers_test.go`:

Change the existing `GetSpendingHeatmap` mock method signature:

```go
func (m *mockStore) GetSpendingHeatmap(_ context.Context, _, _ *time.Time) (*store.HeatmapData, error) {
	if m.heatmapErr != nil {
		return nil, m.heatmapErr
	}
	if m.heatmapData != nil {
		return m.heatmapData, nil
	}
	return &store.HeatmapData{
		ByWeekdayHour: []store.WeekdayHourBucket{},
		ByDayOfMonth:  []store.DayOfMonthBucket{},
	}, nil
}
```

Add `"time"` to imports if not already present.

Add these two tests after the existing `TestHandleGetHeatmap_*` tests:

```go
func TestHandleGetHeatmap_WithFromTo_Returns200(t *testing.T) {
	ms := &mockStore{
		heatmapData: &store.HeatmapData{
			ByWeekdayHour: []store.WeekdayHourBucket{{Weekday: 0, Hour: 10, Amount: 100, Count: 1}},
			ByDayOfMonth:  []store.DayOfMonthBucket{},
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet,
		"/api/stats/heatmap?from=2026-04-01T00:00:00Z&to=2026-04-30T23:59:59Z",
		nil,
	)
	rr := httptest.NewRecorder()
	h.HandleGetHeatmap(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp store.HeatmapData
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp.ByWeekdayHour) != 1 {
		t.Errorf("expected 1 bucket, got %d", len(resp.ByWeekdayHour))
	}
}

func TestHandleGetHeatmap_InvalidFrom_Returns400(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet,
		"/api/stats/heatmap?from=not-a-date",
		nil,
	)
	rr := httptest.NewRecorder()
	h.HandleGetHeatmap(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}
```

- [ ] **Step 2: Confirm compile error**

```bash
cd /Users/ksingh/code/expensor/backend && go build ./... 2>&1 | head -15
```
Expected: compile errors — `mockStore.GetSpendingHeatmap` has new signature but `Storer` and `store.Store` still have old one.

- [ ] **Step 3: Update Storer interface in api/store.go**

In `backend/internal/api/store.go`, add `"time"` to the import block and change the `GetSpendingHeatmap` line:

```go
package api

import (
	"context"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

// Storer is the subset of store.Store operations used by the API handlers.
type Storer interface {
	ListTransactions(ctx context.Context, f store.ListFilter) ([]store.Transaction, int, error)
	GetTransaction(ctx context.Context, id string) (*store.Transaction, error)
	UpdateDescription(ctx context.Context, id, description string) error
	AddLabel(ctx context.Context, transactionID, label string) error
	AddLabels(ctx context.Context, transactionID string, labels []string) error
	RemoveLabel(ctx context.Context, transactionID, label string) error
	SearchTransactions(ctx context.Context, query string, f store.ListFilter) ([]store.Transaction, int, error)
	GetStats(ctx context.Context, baseCurrency string) (*store.Stats, error)
	GetChartData(ctx context.Context) (*store.ChartData, error)
	GetSpendingHeatmap(ctx context.Context, from, to *time.Time) (*store.HeatmapData, error)
	GetAppConfig(ctx context.Context, key string) (string, error)
	SetAppConfig(ctx context.Context, key, value string) error
	GetFacets(ctx context.Context) (*store.Facets, error)
	// Labels
	ListLabels(ctx context.Context) ([]store.Label, error)
	CreateLabel(ctx context.Context, name, color string) error
	UpdateLabel(ctx context.Context, name, color string) error
	DeleteLabel(ctx context.Context, name string) error
	ApplyLabelByMerchant(ctx context.Context, label, pattern string) (int64, error)
	// Categories
	ListCategories(ctx context.Context) ([]store.Category, error)
	CreateCategory(ctx context.Context, name, description string) error
	DeleteCategory(ctx context.Context, name string) error
	// Buckets
	ListBuckets(ctx context.Context) ([]store.Bucket, error)
	CreateBucket(ctx context.Context, name, description string) error
	DeleteBucket(ctx context.Context, name string) error
	// Extended transaction update
	UpdateTransaction(ctx context.Context, id string, u store.TransactionUpdate) error
}

// compile-time check: *store.Store must satisfy Storer.
var _ Storer = (*store.Store)(nil)
```

- [ ] **Step 4: Update GetSpendingHeatmap in store.go**

Replace the existing `GetSpendingHeatmap` method and add the `buildHeatmapWhere` helper:

```go
// GetSpendingHeatmap returns transaction totals aggregated by weekday×hour and
// by day-of-month. When from and to are both non-nil, only transactions within
// [from, to] (inclusive) are included; nil/nil returns all-time data.
func (s *Store) GetSpendingHeatmap(ctx context.Context, from, to *time.Time) (*HeatmapData, error) {
	hd := &HeatmapData{
		ByWeekdayHour: []WeekdayHourBucket{},
		ByDayOfMonth:  []DayOfMonthBucket{},
	}

	where, args := buildHeatmapWhere(from, to)

	// Weekday × hour grid (7 rows × 24 columns = up to 168 buckets).
	wdhQuery := fmt.Sprintf(`
		SELECT
			EXTRACT(DOW  FROM timestamp)::int AS weekday,
			EXTRACT(HOUR FROM timestamp)::int AS hour,
			COALESCE(SUM(amount), 0)          AS amount,
			COUNT(*)                          AS count
		FROM transactions%s
		GROUP BY 1, 2
		ORDER BY 1, 2
	`, where)
	wdhRows, err := s.pool.Query(ctx, wdhQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("fetching weekday/hour heatmap: %w", err)
	}
	defer wdhRows.Close()
	for wdhRows.Next() {
		var b WeekdayHourBucket
		if err := wdhRows.Scan(&b.Weekday, &b.Hour, &b.Amount, &b.Count); err != nil {
			return nil, fmt.Errorf("scanning weekday/hour bucket: %w", err)
		}
		hd.ByWeekdayHour = append(hd.ByWeekdayHour, b)
	}
	if err := wdhRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating weekday/hour rows: %w", err)
	}
	wdhRows.Close() // release connection before opening second query

	// Day of month strip (up to 31 buckets, one per calendar day).
	domQuery := fmt.Sprintf(`
		SELECT
			EXTRACT(DAY FROM timestamp)::int AS day,
			COALESCE(SUM(amount), 0)         AS amount,
			COUNT(*)                         AS count
		FROM transactions%s
		GROUP BY 1
		ORDER BY 1
	`, where)
	domRows, err := s.pool.Query(ctx, domQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("fetching day-of-month heatmap: %w", err)
	}
	defer domRows.Close()
	for domRows.Next() {
		var b DayOfMonthBucket
		if err := domRows.Scan(&b.Day, &b.Amount, &b.Count); err != nil {
			return nil, fmt.Errorf("scanning day-of-month bucket: %w", err)
		}
		hd.ByDayOfMonth = append(hd.ByDayOfMonth, b)
	}
	if err := domRows.Err(); err != nil {
		return nil, fmt.Errorf("iterating day-of-month rows: %w", err)
	}

	return hd, nil
}

// buildHeatmapWhere returns a WHERE clause and positional args for
// GetSpendingHeatmap. Returns empty string and nil args when both are nil.
func buildHeatmapWhere(from, to *time.Time) (string, []any) {
	if from == nil && to == nil {
		return "", nil
	}
	return " WHERE timestamp >= $1 AND timestamp <= $2", []any{*from, *to}
}
```

- [ ] **Step 5: Update HandleGetHeatmap in handlers.go**

Replace the existing `HandleGetHeatmap` function:

```go
// HandleGetHeatmap handles GET /api/stats/heatmap.
// Optional query params: from=<RFC3339>, to=<RFC3339> (both or neither).
// Returns 400 if either param is present but malformed.
func (h *Handlers) HandleGetHeatmap(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	from, to, err := parseHeatmapRange(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	data, err := h.store.GetSpendingHeatmap(r.Context(), from, to)
	if err != nil {
		h.logger.Error("get heatmap", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch heatmap data")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// parseHeatmapRange parses optional ?from= and ?to= RFC3339 query parameters.
// Returns nil, nil when neither is provided. Returns an error if either is
// present but cannot be parsed as RFC3339.
func parseHeatmapRange(r *http.Request) (from, to *time.Time, err error) {
	if v := r.URL.Query().Get("from"); v != "" {
		t, parseErr := time.Parse(time.RFC3339, v)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("invalid 'from' param: must be RFC3339 (e.g. 2026-04-01T00:00:00Z)")
		}
		from = &t
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, parseErr := time.Parse(time.RFC3339, v)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("invalid 'to' param: must be RFC3339 (e.g. 2026-04-30T23:59:59Z)")
		}
		to = &t
	}
	return from, to, nil
}
```

- [ ] **Step 6: Run heatmap tests**

```bash
cd /Users/ksingh/code/expensor/backend && go test ./internal/api/... -v -run TestHandleGetHeatmap 2>&1
```
Expected: all 5 tests pass (`_Success`, `_WithFromTo_Returns200`, `_InvalidFrom_Returns400`, `_StoreError_Returns500`, `_NoStore_Returns503`).

- [ ] **Step 7: Run prod linter**

```bash
cd /Users/ksingh/code/expensor && task lint:be:prod 2>&1 | tail -10
```
Expected: `0 issues`.

- [ ] **Step 8: Run full backend test suite**

```bash
cd /Users/ksingh/code/expensor/backend && go test -short ./... 2>&1
```
Expected: all pass.

- [ ] **Step 9: Commit**

```bash
cd /Users/ksingh/code/expensor && git add \
  backend/internal/store/store.go \
  backend/internal/api/store.go \
  backend/internal/api/handlers.go \
  backend/internal/api/handlers_test.go
git commit --no-gpg-sign -m "feat: add from/to date filtering to GetSpendingHeatmap

Changes GetSpendingHeatmap(ctx) to GetSpendingHeatmap(ctx, from, to *time.Time).
nil/nil keeps all-time behaviour; non-nil adds WHERE timestamp >= \$1 AND timestamp <= \$2.
HandleGetHeatmap parses optional RFC3339 ?from= and ?to= query params,
returning 400 on malformed values."
```

---

## Task 2: Frontend — full-width SVG, month navigation, parameterized hook, tooltip context

**Files:**
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/api/queries.ts`
- Modify: `frontend/src/components/WeekdayHourHeatmap.tsx`
- Modify: `frontend/src/pages/Dashboard.tsx`

### Background

Four related changes: (1) API client passes optional date strings as query params; (2) the query key becomes a function so React Query re-fetches on date change; (3) `WeekdayHourHeatmap` uses `width="100%"` SVG and accepts an optional `monthLabel` prop for the tooltip; (4) `SpendingPatternsSection` owns `{ year, month } | null` state, renders ← Month Year → controls, and passes derived ISO strings to the hook.

- [ ] **Step 1: Update api.stats.heatmap in client.ts**

In `frontend/src/api/client.ts`, replace the `heatmap` line in `api.stats`:

```ts
heatmap: (from?: string, to?: string) => {
  const params = new URLSearchParams()
  if (from) params.set('from', from)
  if (to) params.set('to', to)
  const qs = params.toString()
  return apiClient.get<HeatmapData>(qs ? `/stats/heatmap?${qs}` : '/stats/heatmap')
},
```

- [ ] **Step 2: Update queryKeys.heatmap and useHeatmapData in queries.ts**

In `frontend/src/api/queries.ts`, change `queryKeys.heatmap` from a static value to a function:

```ts
  heatmap: (from?: string, to?: string) => ['stats', 'heatmap', from ?? null, to ?? null] as const,
```

Replace `useHeatmapData`:

```ts
export function useHeatmapData(from?: string, to?: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.heatmap(from, to),
    queryFn: () => api.stats.heatmap(from, to).then((r) => r.data),
    staleTime: 5 * 60 * 1000,
    enabled,
  })
}
```

- [ ] **Step 3: Update WeekdayHourHeatmap.tsx — full-width + monthLabel prop**

Replace the full file content:

```tsx
import { useState } from 'react'
import type { WeekdayHourBucket } from '@/api/types'
import { intensityColor } from '@/lib/heatmap'

const DAY_LABELS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

const CELL_SIZE = 14
const CELL_GAP = 2
const ROW_LABEL_WIDTH = 32
const COL_LABEL_HEIGHT = 16

interface TooltipState {
  x: number
  y: number
  day: string
  hour: number
  amount: number
  count: number
}

interface Props {
  data: WeekdayHourBucket[]
  metric: 'amount' | 'count'
  /** When set, shown as the first line of the hover tooltip, e.g. "Apr 2026". */
  monthLabel?: string
}

export function WeekdayHourHeatmap({ data, metric, monthLabel }: Props) {
  const [tooltip, setTooltip] = useState<TooltipState | null>(null)

  const grid: Record<number, Record<number, WeekdayHourBucket>> = {}
  for (const b of data) {
    if (!grid[b.weekday]) grid[b.weekday] = {}
    grid[b.weekday][b.hour] = b
  }

  const maxAmount = Math.max(...data.map((b) => b.amount), 1)
  const maxCount = Math.max(...data.map((b) => b.count), 1)
  const allZero = data.length === 0

  const totalWidth = ROW_LABEL_WIDTH + 24 * (CELL_SIZE + CELL_GAP) - CELL_GAP
  const totalHeight = COL_LABEL_HEIGHT + 7 * (CELL_SIZE + CELL_GAP) - CELL_GAP

  return (
    <div className="relative w-full">
      {allZero && (
        <div className="absolute inset-0 flex items-center justify-center bg-card/80">
          <span className="text-xs text-muted-foreground">No transaction data yet</span>
        </div>
      )}
      <svg
        width="100%"
        height={totalHeight}
        viewBox={`0 0 ${totalWidth} ${totalHeight}`}
        preserveAspectRatio="xMinYMid meet"
        aria-label="Spending heatmap by weekday and hour"
      >
        {[0, 6, 12, 18].map((hour) => (
          <text
            key={hour}
            x={ROW_LABEL_WIDTH + hour * (CELL_SIZE + CELL_GAP) + CELL_SIZE / 2}
            y={COL_LABEL_HEIGHT - 3}
            textAnchor="middle"
            fontSize="8"
            fill="currentColor"
            className="fill-muted-foreground"
          >
            {String(hour).padStart(2, '0')}
          </text>
        ))}

        {Array.from({ length: 7 }, (_, weekday) =>
          Array.from({ length: 24 }, (_, hour) => {
            const bucket = grid[weekday]?.[hour]
            const value = bucket ? (metric === 'amount' ? bucket.amount : bucket.count) : 0
            const max = metric === 'amount' ? maxAmount : maxCount
            const x = ROW_LABEL_WIDTH + hour * (CELL_SIZE + CELL_GAP)
            const y = COL_LABEL_HEIGHT + weekday * (CELL_SIZE + CELL_GAP)
            return (
              <rect
                key={`${weekday}-${hour}`}
                x={x}
                y={y}
                width={CELL_SIZE}
                height={CELL_SIZE}
                rx={2}
                fill={intensityColor(value, max)}
                onMouseEnter={(e) => {
                  if (!bucket) return
                  setTooltip({ x: e.clientX, y: e.clientY, day: DAY_LABELS[weekday], hour,
                    amount: bucket.amount, count: bucket.count })
                }}
                onMouseLeave={() => setTooltip(null)}
              />
            )
          }),
        )}

        {DAY_LABELS.map((label, weekday) => (
          <text
            key={label}
            x={ROW_LABEL_WIDTH - 4}
            y={COL_LABEL_HEIGHT + weekday * (CELL_SIZE + CELL_GAP) + CELL_SIZE / 2 + 3}
            textAnchor="end"
            fontSize="8"
            fill="currentColor"
            className="fill-muted-foreground"
          >
            {label}
          </text>
        ))}
      </svg>

      {tooltip && (
        <div
          className="pointer-events-none fixed z-50 rounded border border-border bg-secondary px-2 py-1 text-xs shadow-lg"
          style={{ left: tooltip.x + 8, top: tooltip.y - 8 }}
        >
          {monthLabel && (
            <span className="block text-muted-foreground">{monthLabel}</span>
          )}
          <span className="font-medium text-foreground">
            {tooltip.day} {String(tooltip.hour).padStart(2, '0')}:00
          </span>
          <br />
          <span className="text-muted-foreground">
            ₹{tooltip.amount.toLocaleString('en-IN')} &middot; {tooltip.count} txn
            {tooltip.count !== 1 ? 's' : ''}
          </span>
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 4: Update SpendingPatternsSection in Dashboard.tsx**

Add month-nav helpers before `MetricToggle` (or anywhere before `SpendingPatternsSection`):

```tsx
// ─── Month navigation ─────────────────────────────────────────────────────────

interface MonthNav { year: number; month: number }

const MONTH_NAMES = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']

function monthRangeISO(nav: MonthNav): { from: string; to: string } {
  const from = new Date(Date.UTC(nav.year, nav.month - 1, 1))
  const to = new Date(Date.UTC(nav.year, nav.month, 0, 23, 59, 59))
  return {
    from: from.toISOString().split('.')[0] + 'Z',
    to: to.toISOString().split('.')[0] + 'Z',
  }
}

function prevMonth(n: MonthNav): MonthNav {
  return n.month === 1 ? { year: n.year - 1, month: 12 } : { year: n.year, month: n.month - 1 }
}

function nextMonth(n: MonthNav): MonthNav {
  return n.month === 12 ? { year: n.year + 1, month: 1 } : { year: n.year, month: n.month + 1 }
}
```

Replace the existing `SpendingPatternsSection` and `Dashboard` functions:

```tsx
// ─── Spending patterns ────────────────────────────────────────────────────────

function SpendingPatternsSection() {
  const [metric, setMetric] = useState<'amount' | 'count'>('amount')
  const now = new Date()
  const [monthNav, setMonthNav] = useState<MonthNav | null>(null)

  const dateRange = monthNav ? monthRangeISO(monthNav) : undefined
  const { data: heatmap, isLoading } = useHeatmapData(dateRange?.from, dateRange?.to)

  const monthLabel = monthNav ? `${MONTH_NAMES[monthNav.month - 1]} ${monthNav.year}` : undefined

  const isCurrentMonth =
    monthNav !== null &&
    monthNav.year === now.getFullYear() &&
    monthNav.month === now.getMonth() + 1

  if (isLoading) {
    return <div className="h-40 animate-pulse rounded-lg border border-border bg-card shadow-sm" />
  }

  if (!heatmap) return null

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-xs uppercase tracking-wider text-muted-foreground">Spending Patterns</h2>
        <MetricToggle value={metric} onChange={setMetric} />
      </div>

      <div className="flex items-center gap-2">
        <button
          onClick={() => setMonthNav((p) => prevMonth(p ?? { year: now.getFullYear(), month: now.getMonth() + 1 }))}
          className="rounded border border-border px-2 py-0.5 text-xs text-muted-foreground hover:text-foreground"
          aria-label="Previous month"
        >←</button>
        <span className="min-w-[6rem] text-center text-xs font-medium text-foreground">
          {monthLabel ?? 'All time'}
        </span>
        <button
          onClick={() => setMonthNav((p) => nextMonth(p ?? { year: now.getFullYear(), month: now.getMonth() + 1 }))}
          disabled={isCurrentMonth}
          className="rounded border border-border px-2 py-0.5 text-xs text-muted-foreground hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40"
          aria-label="Next month"
        >→</button>
        {monthNav !== null && (
          <button
            onClick={() => setMonthNav(null)}
            className="ml-1 rounded border border-border px-2 py-0.5 text-xs text-muted-foreground hover:text-foreground"
          >All time</button>
        )}
      </div>

      <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
        <h3 className="mb-3 text-xs uppercase tracking-wider text-muted-foreground">By weekday &amp; hour</h3>
        <WeekdayHourHeatmap data={heatmap.by_weekday_hour} metric={metric} monthLabel={monthLabel} />
      </div>
      <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
        <h3 className="mb-3 text-xs uppercase tracking-wider text-muted-foreground">By day of month</h3>
        <DayOfMonthHeatmap data={heatmap.by_day_of_month} metric={metric} />
      </div>
      <HeatmapLegend />
    </div>
  )
}
```

Update `Dashboard` to remove the standalone heatmap fetch (now owned by `SpendingPatternsSection`):

```tsx
export function Dashboard() {
  const { data: chartData } = useChartData()

  return (
    <div className="mx-auto w-full max-w-6xl space-y-6 px-6 py-6">
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2">
          <ErrorBoundary>
            <StatsSection />
          </ErrorBoundary>
        </div>
        <div className="lg:col-span-1">
          <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-xs uppercase tracking-wider text-muted-foreground">Recent transactions</h2>
            </div>
            <ErrorBoundary>
              <RecentTransactions />
            </ErrorBoundary>
          </div>
        </div>
      </div>

      {chartData && (
        <ErrorBoundary>
          <ChartsSection charts={chartData} />
        </ErrorBoundary>
      )}

      <ErrorBoundary>
        <SpendingPatternsSection />
      </ErrorBoundary>
    </div>
  )
}

export default Dashboard
```

Also update the import block at the top of `Dashboard.tsx` — remove the `HeatmapData` type import (no longer used at file level) and ensure `useHeatmapData` is imported:

```tsx
import { useState } from 'react'
import { useChartData, useHeatmapData, useStatus, useTransactions } from '@/api/queries'
import type { ChartData, TimeBucket } from '@/api/types'
import { DayOfMonthHeatmap } from '@/components/DayOfMonthHeatmap'
import { HeatmapLegend } from '@/components/HeatmapLegend'
import { WeekdayHourHeatmap } from '@/components/WeekdayHourHeatmap'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { formatCurrency, formatRelative } from '@/lib/utils'
import { Link } from 'react-router-dom'
```

- [ ] **Step 5: TypeScript type check**

```bash
cd /Users/ksingh/code/expensor && task lint:fe 2>&1
```
Expected: 0 errors. Common issues: if `queryKeys.heatmap` is now a function, any call sites that used it as a value (`queryKeys.heatmap` instead of `queryKeys.heatmap()`) will error — fix by calling as `queryKeys.heatmap(from, to)`.

- [ ] **Step 6: Commit**

```bash
cd /Users/ksingh/code/expensor && git add \
  frontend/src/api/client.ts \
  frontend/src/api/queries.ts \
  frontend/src/components/WeekdayHourHeatmap.tsx \
  frontend/src/pages/Dashboard.tsx
git commit --no-gpg-sign -m "feat: month navigation and full-width layout for weekday/hour heatmap

SpendingPatternsSection now owns month navigation state ({ year, month } | null)
with prev/next arrow controls and an 'All time' toggle. The selected month's
RFC3339 date range flows through useHeatmapData → api.stats.heatmap → the backend.
WeekdayHourHeatmap switches to width=100% SVG (fills container, no scroll) and
accepts monthLabel to prefix the hover tooltip with date context."
```
