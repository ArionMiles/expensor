# Annual Calendar Heatmap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the existing `DayOfMonthHeatmap` (1×31 strip) in the Dashboard's `SpendingPatternsSection` with a GitHub contribution-graph-style annual calendar heatmap backed by a new per-day backend aggregate and a new API endpoint.

**Architecture:** No DB migration needed (query runs over the existing `transactions` table). The backend adds `DailyBucket` type and `GetAnnualSpend` to the store, extends the `Storer` interface, adds `HandleGetAnnualHeatmap`, and registers `GET /api/stats/heatmap/annual?year=YYYY`. The frontend adds `DailyBucket`/`AnnualHeatmapData` types, a new `api.stats.annualHeatmap(year)` client method, a `useAnnualHeatmapData(year)` hook, and an `AnnualCalendarHeatmap` component that owns its own year state and replaces `DayOfMonthHeatmap` in `SpendingPatternsSection`.

**Tech Stack:** Go 1.23+, pgx/v5, React 18, TypeScript, TanStack Query, Tailwind CSS, SVG.

---

## File Map

| File | Change |
|------|--------|
| `backend/internal/store/store.go` | Add `DailyBucket` type; add `GetAnnualSpend` method |
| `backend/internal/store/store_test.go` | Add `TestGetAnnualSpend_EmptyDB` integration test |
| `backend/internal/api/store.go` | Add `GetAnnualSpend` to `Storer` interface |
| `backend/internal/api/handlers.go` | Add `HandleGetAnnualHeatmap` handler |
| `backend/internal/api/handlers_test.go` | Add `annualData`/`annualErr` to `mockStore`; add mock method; add 3 handler tests |
| `backend/internal/api/server.go` | Register `GET /api/stats/heatmap/annual` |
| `frontend/src/api/types.ts` | Add `DailyBucket`, `AnnualHeatmapData` interfaces |
| `frontend/src/api/client.ts` | Add `api.stats.annualHeatmap(year)` |
| `frontend/src/api/queries.ts` | Add `queryKeys.annualHeatmap(year)` + `useAnnualHeatmapData(year)` |
| `frontend/src/components/AnnualCalendarHeatmap.tsx` | New — GitHub-style 7-row × 52-col SVG calendar with year nav |
| `frontend/src/components/DayOfMonthHeatmap.tsx` | Delete (replaced) |
| `frontend/src/pages/Dashboard.tsx` | Replace `DayOfMonthHeatmap` import/usage with `AnnualCalendarHeatmap` |

---

## Task 1: Backend store — `DailyBucket` type + `GetAnnualSpend` method + integration test

**Files:**
- Modify: `backend/internal/store/store.go`
- Modify: `backend/internal/store/store_test.go`

### Background

`store.go` already defines `WeekdayHourBucket`, `DayOfMonthBucket`, and `HeatmapData`. Add `DailyBucket` and `GetAnnualSpend` following the same patterns. The new method takes a `year int` parameter and uses a `WHERE EXTRACT(YEAR FROM timestamp) = $1` predicate. Like the existing heatmap method, initialize the slice to `[]DailyBucket{}` (not nil) so the JSON response never contains a null array.

The integration test follows the same pattern as `TestGetSpendingHeatmap_EmptyDB` — it lives in `package store_test`, uses `newTestStore(t)`, and is skipped automatically in short mode.

- [ ] **Step 1: Write the failing integration test**

Add to `backend/internal/store/store_test.go` at the end of the file (after `TestGetSpendingHeatmap_EmptyDB`):

```go
func TestGetAnnualSpend_EmptyDB(t *testing.T) {
	ts := newTestStore(t) // skips automatically when -short is passed
	defer ts.cleanup()

	buckets, err := ts.GetAnnualSpend(context.Background(), 2026)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buckets == nil {
		t.Error("GetAnnualSpend must return a non-nil slice, got nil")
	}
	if len(buckets) != 0 {
		t.Errorf("expected 0 buckets in empty DB, got %d", len(buckets))
	}
}
```

- [ ] **Step 2: Confirm compile error**

```bash
cd /Users/ksingh/code/expensor/backend && go test ./internal/store/... -short -run TestGetAnnualSpend 2>&1 | head -10
```

Expected: compile error — `GetAnnualSpend` is not yet defined on `*Store`.

- [ ] **Step 3: Add `DailyBucket` type to store.go**

In `backend/internal/store/store.go`, after the `HeatmapData` struct (around line 87), add:

```go
// DailyBucket holds transaction totals for a single calendar date.
type DailyBucket struct {
	Date   time.Time `json:"date"`
	Amount float64   `json:"amount"`
	Count  int       `json:"count"`
}
```

- [ ] **Step 4: Add `GetAnnualSpend` to store.go**

In `backend/internal/store/store.go`, after `GetSpendingHeatmap` (after line 634), add:

```go
// GetAnnualSpend returns per-day transaction totals for a given calendar year.
// Results are ordered by date ascending. Returns an empty (non-nil) slice when
// the year has no transactions.
func (s *Store) GetAnnualSpend(ctx context.Context, year int) ([]DailyBucket, error) {
	buckets := []DailyBucket{}

	rows, err := s.pool.Query(ctx, `
		SELECT
			timestamp::date        AS date,
			COALESCE(SUM(amount), 0) AS amount,
			COUNT(*)               AS count
		FROM transactions
		WHERE EXTRACT(YEAR FROM timestamp) = $1
		GROUP BY date
		ORDER BY date
	`, year)
	if err != nil {
		return nil, fmt.Errorf("fetching annual spend for %d: %w", year, err)
	}
	defer rows.Close()
	for rows.Next() {
		var b DailyBucket
		if err := rows.Scan(&b.Date, &b.Amount, &b.Count); err != nil {
			return nil, fmt.Errorf("scanning daily bucket: %w", err)
		}
		buckets = append(buckets, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating annual spend rows: %w", err)
	}

	return buckets, nil
}
```

- [ ] **Step 5: Run store tests in short mode to confirm compile success**

```bash
cd /Users/ksingh/code/expensor/backend && go test ./internal/store/... -short -v -run TestGetAnnualSpend
```

Expected: `--- SKIP: TestGetAnnualSpend_EmptyDB (0.00s)` — compiles clean, skips because Docker isn't required in short mode.

- [ ] **Step 6: Run lint**

```bash
task lint:be:prod
```

Expected: `0 issues`.

- [ ] **Step 7: Commit**

```bash
cd /Users/ksingh/code/expensor && git add backend/internal/store/store.go backend/internal/store/store_test.go
git commit --no-gpg-sign -m "feat: add DailyBucket type and GetAnnualSpend store method

Adds DailyBucket (date, amount, count) to the store package and
GetAnnualSpend which aggregates per-day spend for a given calendar year.
Returns a non-nil empty slice when the year has no transactions so the
JSON response never contains a null array."
```

---

## Task 2: Backend API — `Storer` interface + `HandleGetAnnualHeatmap` + route + 3 handler tests

**Files:**
- Modify: `backend/internal/api/store.go`
- Modify: `backend/internal/api/handlers.go`
- Modify: `backend/internal/api/handlers_test.go`
- Modify: `backend/internal/api/server.go`

### Background

Follows the exact same pattern as the existing `HandleGetHeatmap`. The handler parses `?year=YYYY` from the query string using the existing `queryInt` helper, defaulting to `time.Now().Year()`. The route is registered just below `GET /api/stats/heatmap` to keep the stats routes grouped. The three tests cover success, store error (500), and nil store (503).

- [ ] **Step 1: Write the three failing handler tests**

Add to `backend/internal/api/handlers_test.go`:

First, add two fields to the `mockStore` struct (after the `heatmapErr error` field at line 58):

```go
	annualData []store.DailyBucket
	annualErr  error
```

Then add the mock method after `GetSpendingHeatmap` (after line 207):

```go
func (m *mockStore) GetAnnualSpend(_ context.Context, _ int) ([]store.DailyBucket, error) {
	if m.annualErr != nil {
		return nil, m.annualErr
	}
	if m.annualData != nil {
		return m.annualData, nil
	}
	return []store.DailyBucket{}, nil
}
```

Then add the three tests at the end of the file (after line 1170):

```go
// --- annual heatmap ---

func TestHandleGetAnnualHeatmap_Success(t *testing.T) {
	ms := &mockStore{
		annualData: []store.DailyBucket{
			{Date: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Amount: 1500.0, Count: 3},
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/heatmap/annual?year=2026", nil)
	rr := httptest.NewRecorder()
	h.HandleGetAnnualHeatmap(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp struct {
		Year    int                `json:"year"`
		Buckets []store.DailyBucket `json:"buckets"`
	}
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.Year != 2026 {
		t.Errorf("expected year=2026, got %d", resp.Year)
	}
	if len(resp.Buckets) != 1 {
		t.Errorf("expected 1 bucket, got %d", len(resp.Buckets))
	}
	if resp.Buckets[0].Amount != 1500.0 {
		t.Errorf("expected Amount=1500, got %f", resp.Buckets[0].Amount)
	}
}

func TestHandleGetAnnualHeatmap_StoreError_Returns500(t *testing.T) {
	ms := &mockStore{annualErr: errors.New("db connection lost")}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/heatmap/annual?year=2026", nil)
	rr := httptest.NewRecorder()
	h.HandleGetAnnualHeatmap(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestHandleGetAnnualHeatmap_NoStore_Returns503(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/heatmap/annual?year=2026", nil)
	rr := httptest.NewRecorder()
	h.HandleGetAnnualHeatmap(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}
```

- [ ] **Step 2: Confirm compile error**

```bash
cd /Users/ksingh/code/expensor/backend && go test ./internal/api/... -short -run TestHandleGetAnnualHeatmap 2>&1 | head -10
```

Expected: compile error — `HandleGetAnnualHeatmap` does not yet exist on `*Handlers`, and `mockStore` does not yet implement `GetAnnualSpend` in the `Storer` interface.

- [ ] **Step 3: Add `GetAnnualSpend` to the `Storer` interface**

In `backend/internal/api/store.go`, add after `GetSpendingHeatmap`:

```go
	GetAnnualSpend(ctx context.Context, year int) ([]store.DailyBucket, error)
```

- [ ] **Step 4: Add `HandleGetAnnualHeatmap` to handlers.go**

In `backend/internal/api/handlers.go`, add after `HandleGetHeatmap` (after line 1050). The `year` query parameter defaults to `time.Now().Year()` when absent or unparseable. The `queryInt` helper returns its default if the value is 0, so use `strconv.Atoi` directly here to allow any positive integer year.

```go
// HandleGetAnnualHeatmap handles GET /api/stats/heatmap/annual?year=YYYY.
// Returns per-day transaction totals for the requested calendar year.
// Defaults to the current year when ?year is absent or invalid.
func (h *Handlers) HandleGetAnnualHeatmap(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	yearStr := r.URL.Query().Get("year")
	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 1 {
		year = time.Now().Year()
	}

	buckets, err := h.store.GetAnnualSpend(r.Context(), year)
	if err != nil {
		h.logger.Error("get annual heatmap", "error", err, "year", year)
		writeError(w, http.StatusInternalServerError, "failed to fetch annual heatmap data")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"year":    year,
		"buckets": buckets,
	})
}
```

- [ ] **Step 5: Register the route in server.go**

In `backend/internal/api/server.go`, add after `GET /api/stats/heatmap` (after line 90):

```go
	mux.HandleFunc("GET /api/stats/heatmap/annual", h.HandleGetAnnualHeatmap)
```

- [ ] **Step 6: Run all annual API tests**

```bash
cd /Users/ksingh/code/expensor/backend && go test ./internal/api/... -v -run TestHandleGetAnnualHeatmap
```

Expected: all three tests pass.

- [ ] **Step 7: Run the full backend test suite**

```bash
cd /Users/ksingh/code/expensor && task test:be
```

Expected: all existing tests pass, no regressions.

- [ ] **Step 8: Run lint**

```bash
cd /Users/ksingh/code/expensor && task lint:be:prod
```

Expected: `0 issues`.

- [ ] **Step 9: Commit**

```bash
cd /Users/ksingh/code/expensor && git add backend/internal/api/store.go backend/internal/api/handlers.go backend/internal/api/handlers_test.go backend/internal/api/server.go
git commit --no-gpg-sign -m "feat: add GET /api/stats/heatmap/annual endpoint

Extends Storer interface with GetAnnualSpend(ctx, year). Handler parses
?year query param, defaults to current year. Response envelope is
{year, buckets} so callers always know which year was served. Route
registered alongside GET /api/stats/heatmap in server.go."
```

---

## Task 3: Frontend — types, client method, and query hook

**Files:**
- Modify: `frontend/src/api/types.ts`
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/api/queries.ts`

### Background

Three small additive changes, all following existing conventions. `DailyBucket.date` is a `string` (RFC3339 from Go's `time.Time` JSON serialization) and is parsed to a `Date` object inside the component. The query key is `['stats', 'heatmap', 'annual', year]` so different years cache independently.

- [ ] **Step 1: Add types to types.ts**

In `frontend/src/api/types.ts`, after the `HeatmapData` interface (after line 75), add:

```ts
export interface DailyBucket {
  date: string   // RFC3339 date from Go — parse with new Date(b.date)
  amount: number
  count: number
}

export interface AnnualHeatmapData {
  year: number
  buckets: DailyBucket[]
}
```

- [ ] **Step 2: Add client method to client.ts**

In `frontend/src/api/client.ts`, add `AnnualHeatmapData` to the import list (alphabetical position, after `AuthStatus`):

```ts
import type {
  AnnualHeatmapData,
  AuthStartResponse,
  ...
} from './types'
```

Then in the `api.stats` object, add `annualHeatmap` after `heatmap`:

```ts
  stats: {
    charts: () => apiClient.get<ChartData>('/stats/charts'),
    heatmap: () => apiClient.get<HeatmapData>('/stats/heatmap'),
    annualHeatmap: (year: number) =>
      apiClient.get<AnnualHeatmapData>(`/stats/heatmap/annual?year=${year}`),
  },
```

- [ ] **Step 3: Add query key and hook to queries.ts**

In `frontend/src/api/queries.ts`, add `AnnualHeatmapData` to the import from `./types`:

```ts
import type { AnnualHeatmapData, TransactionFilters, TransactionPatch } from './types'
```

Add to the `queryKeys` object after `heatmap`:

```ts
  annualHeatmap: (year: number) => ['stats', 'heatmap', 'annual', year] as const,
```

Add the hook after `useHeatmapData`:

```ts
export function useAnnualHeatmapData(year: number) {
  return useQuery({
    queryKey: queryKeys.annualHeatmap(year),
    queryFn: () => api.stats.annualHeatmap(year).then((r) => r.data),
    staleTime: 5 * 60 * 1000, // 5 minutes
  })
}
```

- [ ] **Step 4: Run TypeScript type check**

```bash
cd /Users/ksingh/code/expensor && task lint:fe
```

Expected: zero TypeScript errors.

- [ ] **Step 5: Commit**

```bash
cd /Users/ksingh/code/expensor && git add frontend/src/api/types.ts frontend/src/api/client.ts frontend/src/api/queries.ts
git commit --no-gpg-sign -m "feat(frontend): add AnnualHeatmapData types, client method, and query hook

DailyBucket mirrors the Go store type. Query key includes year so each
year caches independently. useAnnualHeatmapData follows the same
5-minute staleTime pattern as useHeatmapData."
```

---

## Task 4: Frontend — `AnnualCalendarHeatmap` component + Dashboard wiring + full CI check

**Files:**
- Create: `frontend/src/components/AnnualCalendarHeatmap.tsx`
- Modify: `frontend/src/pages/Dashboard.tsx`
- Delete: `frontend/src/components/DayOfMonthHeatmap.tsx` (remove the file after updating Dashboard)

### Background

The component owns its own `year` state (no prop needed from parent), renders a 7-row × 52-53-column SVG grid, month labels above, day-of-week labels on the left (Mon/Wed/Fri only), year navigation arrows, and fixed-position tooltips. It reuses `intensityColor` from `@/lib/heatmap`. Empty cells (no data for that date, or null padding cells) use `intensityColor(0, max)` which returns `hsl(var(--muted))`.

In `Dashboard.tsx`, the `SpendingPatternsSection` currently renders `DayOfMonthHeatmap` below `WeekdayHourHeatmap`. Replace that block with `<AnnualCalendarHeatmap />`. The component is self-contained (owns year state and fetches its own data), so `SpendingPatternsSection` does not need to pass any props.

The `buildCalendarGrid` utility is defined inside the component file (not exported separately) as it is used only there.

- [ ] **Step 1: Create `AnnualCalendarHeatmap.tsx`**

Create `frontend/src/components/AnnualCalendarHeatmap.tsx`:

```tsx
import { useState } from 'react'
import { useAnnualHeatmapData } from '@/api/queries'
import type { DailyBucket } from '@/api/types'
import { intensityColor } from '@/lib/heatmap'

// ─── Calendar grid builder ────────────────────────────────────────────────────

/**
 * Returns an array of week-columns. Each column has 7 cells (Mon=0 … Sun=6).
 * Cells before Jan 1 and after Dec 31 are null (padding).
 *
 * ISO week convention: Mon=0, Sun=6 (matches GitHub contribution graph).
 */
function buildCalendarGrid(year: number): { date: Date | null }[][] {
  const jan1 = new Date(year, 0, 1)
  // JS getDay(): 0=Sun … 6=Sat → shift so Mon=0, Sun=6
  const startDow = (jan1.getDay() + 6) % 7

  const allDates: Date[] = []
  const d = new Date(jan1)
  while (d.getFullYear() === year) {
    allDates.push(new Date(d))
    d.setDate(d.getDate() + 1)
  }

  // Pad to a multiple of 7
  const totalCells = Math.ceil((allDates.length + startDow) / 7) * 7
  const cells: (Date | null)[] = [
    ...Array(startDow).fill(null),
    ...allDates,
    ...Array(totalCells - allDates.length - startDow).fill(null),
  ]

  const numCols = totalCells / 7
  return Array.from({ length: numCols }, (_, col) =>
    Array.from({ length: 7 }, (_, row) => ({ date: cells[col * 7 + row] })),
  )
}

// ─── Constants ────────────────────────────────────────────────────────────────

const CELL = 11   // px — cell size
const GAP  = 2    // px — gap between cells
const STEP = CELL + GAP

const DAY_LABEL_WIDTH = 28  // px — left gutter for Mon/Wed/Fri labels
const MONTH_LABEL_HEIGHT = 16 // px — top gutter for Jan … Dec labels

// Rows to label: index 0=Mon, 2=Wed, 4=Fri (every other, like GitHub)
const LABELED_ROWS: { row: number; label: string }[] = [
  { row: 0, label: 'Mon' },
  { row: 2, label: 'Wed' },
  { row: 4, label: 'Fri' },
]

const MONTH_NAMES = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
                     'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

// ─── Component ───────────────────────────────────────────────────────────────

interface TooltipState {
  x: number
  y: number
  date: Date
  amount: number
  count: number
}

export function AnnualCalendarHeatmap() {
  const currentYear = new Date().getFullYear()
  const [year, setYear] = useState(currentYear)
  const [tooltip, setTooltip] = useState<TooltipState | null>(null)

  const { data, isLoading } = useAnnualHeatmapData(year)

  // Build a lookup: "YYYY-MM-DD" → DailyBucket
  const lookup = new Map<string, DailyBucket>()
  if (data) {
    for (const b of data.buckets) {
      // Go serializes time.Time as RFC3339; take just the date part
      lookup.set(b.date.slice(0, 10), b)
    }
  }

  const maxAmount = data
    ? Math.max(...data.buckets.map((b) => b.amount), 1)
    : 1

  const grid = buildCalendarGrid(year)
  const numCols = grid.length

  const svgWidth = DAY_LABEL_WIDTH + numCols * STEP - GAP
  const svgHeight = MONTH_LABEL_HEIGHT + 7 * STEP - GAP

  // Compute month label positions: first column whose first date is the 1st of a month
  const monthLabels: { col: number; month: number }[] = []
  grid.forEach((col, colIdx) => {
    const firstDate = col.find((c) => c.date !== null)?.date
    if (firstDate && firstDate.getDate() === 1) {
      monthLabels.push({ col: colIdx, month: firstDate.getMonth() })
    }
  })

  function formatTooltipDate(d: Date): string {
    return d.toLocaleDateString('en-IN', { weekday: 'short', year: 'numeric', month: 'short', day: 'numeric' })
  }

  return (
    <div className="space-y-3">
      {/* Year navigation */}
      <div className="flex items-center gap-3 text-sm">
        <button
          onClick={() => setYear((y) => y - 1)}
          className="rounded px-1.5 py-0.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
          aria-label="Previous year"
        >
          ←
        </button>
        <span className="font-medium tabular-nums text-foreground">{year}</span>
        <button
          onClick={() => setYear((y) => y + 1)}
          disabled={year >= currentYear}
          className="rounded px-1.5 py-0.5 text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40"
          aria-label="Next year"
        >
          →
        </button>
      </div>

      {isLoading && (
        <div className="h-28 animate-pulse rounded bg-secondary" />
      )}

      {!isLoading && (
        <div className="overflow-x-auto">
          <svg
            width={svgWidth}
            height={svgHeight}
            viewBox={`0 0 ${svgWidth} ${svgHeight}`}
            aria-label={`Annual spending heatmap for ${year}`}
          >
            {/* Day-of-week labels: Mon, Wed, Fri */}
            {LABELED_ROWS.map(({ row, label }) => (
              <text
                key={label}
                x={DAY_LABEL_WIDTH - 4}
                y={MONTH_LABEL_HEIGHT + row * STEP + CELL / 2 + 3}
                textAnchor="end"
                fontSize={8}
                className="fill-muted-foreground"
              >
                {label}
              </text>
            ))}

            {/* Month labels */}
            {monthLabels.map(({ col, month }) => (
              <text
                key={`month-${month}`}
                x={DAY_LABEL_WIDTH + col * STEP}
                y={MONTH_LABEL_HEIGHT - 3}
                fontSize={8}
                className="fill-muted-foreground"
              >
                {MONTH_NAMES[month]}
              </text>
            ))}

            {/* Calendar cells */}
            {grid.map((col, colIdx) =>
              col.map(({ date }, rowIdx) => {
                const x = DAY_LABEL_WIDTH + colIdx * STEP
                const y = MONTH_LABEL_HEIGHT + rowIdx * STEP
                if (date === null) {
                  return (
                    <rect
                      key={`${colIdx}-${rowIdx}`}
                      x={x}
                      y={y}
                      width={CELL}
                      height={CELL}
                      rx={2}
                      fill="transparent"
                    />
                  )
                }
                const key = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
                const bucket = lookup.get(key)
                const amount = bucket?.amount ?? 0
                const fill = intensityColor(amount, maxAmount)
                return (
                  <rect
                    key={`${colIdx}-${rowIdx}`}
                    x={x}
                    y={y}
                    width={CELL}
                    height={CELL}
                    rx={2}
                    fill={fill}
                    onMouseEnter={(e) => {
                      setTooltip({
                        x: e.clientX,
                        y: e.clientY,
                        date,
                        amount,
                        count: bucket?.count ?? 0,
                      })
                    }}
                    onMouseLeave={() => setTooltip(null)}
                  />
                )
              }),
            )}
          </svg>
        </div>
      )}

      {tooltip && (
        <div
          className="pointer-events-none fixed z-50 rounded border border-border bg-secondary px-2 py-1 text-xs shadow-lg"
          style={{ left: tooltip.x + 10, top: tooltip.y - 10 }}
        >
          <span className="font-medium text-foreground">{formatTooltipDate(tooltip.date)}</span>
          <br />
          <span className="text-muted-foreground">
            {tooltip.amount > 0
              ? `₹${tooltip.amount.toLocaleString('en-IN')} · ${tooltip.count} txn${tooltip.count !== 1 ? 's' : ''}`
              : 'No transactions'}
          </span>
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Update Dashboard.tsx — replace `DayOfMonthHeatmap` with `AnnualCalendarHeatmap`**

In `frontend/src/pages/Dashboard.tsx`:

1. Remove the import line:
   ```ts
   import { DayOfMonthHeatmap } from '@/components/DayOfMonthHeatmap'
   ```

2. Add the new import in its place (keep it alphabetical among component imports):
   ```ts
   import { AnnualCalendarHeatmap } from '@/components/AnnualCalendarHeatmap'
   ```

3. Remove `HeatmapData` from the type import if it is no longer used by `SpendingPatternsSection`. It is still used as the type annotation on the `heatmap` prop, so check first — if the prop type remains `HeatmapData` it stays; if you move the section to no longer receive it as a prop you can remove it. Since `AnnualCalendarHeatmap` is self-contained, `SpendingPatternsSection` no longer needs the `heatmap` prop at all. Update accordingly:

Remove the `{ heatmap }: { heatmap: HeatmapData }` prop from `SpendingPatternsSection` and its usage at the call site in `Dashboard`. Remove `HeatmapData` from the import if it is no longer referenced elsewhere in the file.

The updated `SpendingPatternsSection`:

```tsx
function SpendingPatternsSection() {
  const [metric, setMetric] = useState<'amount' | 'count'>('amount')

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-xs uppercase tracking-wider text-muted-foreground">
          Spending Patterns
        </h2>
        <MetricToggle value={metric} onChange={setMetric} />
      </div>
      <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
        <h3 className="mb-3 text-xs uppercase tracking-wider text-muted-foreground">
          By weekday &amp; hour
        </h3>
        <WeekdayHourHeatmap data={heatmap.by_weekday_hour} metric={metric} />
      </div>
      <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
        <h3 className="mb-3 text-xs uppercase tracking-wider text-muted-foreground">
          By day of year
        </h3>
        <AnnualCalendarHeatmap />
      </div>
      <HeatmapLegend />
    </div>
  )
}
```

Note: `WeekdayHourHeatmap` still needs `heatmap.by_weekday_hour`, so the parent still passes that data. Keep the existing `heatmap` prop on `SpendingPatternsSection` for the weekday/hour grid — only the `DayOfMonthHeatmap` block is replaced. The final `SpendingPatternsSection` is:

```tsx
function SpendingPatternsSection({ heatmap }: { heatmap: HeatmapData }) {
  const [metric, setMetric] = useState<'amount' | 'count'>('amount')

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-xs uppercase tracking-wider text-muted-foreground">
          Spending Patterns
        </h2>
        <MetricToggle value={metric} onChange={setMetric} />
      </div>
      <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
        <h3 className="mb-3 text-xs uppercase tracking-wider text-muted-foreground">
          By weekday &amp; hour
        </h3>
        <WeekdayHourHeatmap data={heatmap.by_weekday_hour} metric={metric} />
      </div>
      <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
        <h3 className="mb-3 text-xs uppercase tracking-wider text-muted-foreground">
          By day of year
        </h3>
        <AnnualCalendarHeatmap />
      </div>
      <HeatmapLegend />
    </div>
  )
}
```

- [ ] **Step 3: Delete `DayOfMonthHeatmap.tsx`**

```bash
rm /Users/ksingh/code/expensor/frontend/src/components/DayOfMonthHeatmap.tsx
```

- [ ] **Step 4: Run TypeScript type check**

```bash
cd /Users/ksingh/code/expensor && task lint:fe
```

Expected: zero TypeScript errors. If there are unused import warnings (e.g. `DayOfMonthBucket`), remove those imports from `types.ts` only if nothing else references them. Check with grep before removing — the type is still exported from `types.ts` and used in the mock store in `handlers_test.go` (Go side, not TS), so it is safe to leave it in `types.ts` even if the TS component is gone.

- [ ] **Step 5: Run full CI gate**

```bash
cd /Users/ksingh/code/expensor && task ci
```

Expected: `lint:be:prod` = 0 issues, `test:be` all pass, `lint:fe` zero errors, `audit:fe` clean.

- [ ] **Step 6: Commit**

```bash
cd /Users/ksingh/code/expensor && git add frontend/src/components/AnnualCalendarHeatmap.tsx frontend/src/pages/Dashboard.tsx && git rm frontend/src/components/DayOfMonthHeatmap.tsx
git commit --no-gpg-sign -m "feat(frontend): replace DayOfMonthHeatmap with AnnualCalendarHeatmap

AnnualCalendarHeatmap renders a GitHub-style 7×52 annual calendar grid.
It owns its own year state and fetches data via useAnnualHeatmapData,
so SpendingPatternsSection requires no prop changes beyond swapping the
component. Year navigation arrows allow ← prev / → next (capped at
current year). Tooltips show date, amount, and transaction count.
buildCalendarGrid pads the grid with null cells so the first column
always starts on a Monday (ISO week convention)."
```

---

## Self-Review Checklist

| Spec requirement | Covered by |
|---|---|
| 7 rows (Mon–Sun ISO) × 52-53 cols (one per week) | Task 4 — `buildCalendarGrid` in `AnnualCalendarHeatmap` |
| Each cell = one actual calendar date | Task 4 — `buildCalendarGrid` produces `Date` objects |
| Month labels above grid at first week of each month | Task 4 — `monthLabels` derivation in `AnnualCalendarHeatmap` |
| Day labels Mon/Wed/Fri on left | Task 4 — `LABELED_ROWS` constant |
| Year navigation ← / → (default = current year) | Task 4 — `year` state + prev/next buttons |
| Next-year button disabled when `year >= currentYear` | Task 4 — `disabled={year >= currentYear}` |
| Tooltip: date · ₹amount · N txns | Task 4 — `TooltipState` + `onMouseEnter` |
| Empty cells = `intensityColor(0, max)` = muted | Task 4 — `amount = bucket?.amount ?? 0` passed to `intensityColor` |
| `overflow-x-auto` wrapper for narrow viewports | Task 4 — wrapping `<div className="overflow-x-auto">` |
| `GET /api/stats/heatmap/annual?year=YYYY` endpoint | Task 2 — `HandleGetAnnualHeatmap` + route |
| SQL: per-day aggregate filtered by year | Task 1 — `GetAnnualSpend` |
| `DailyBucket` Go type (date, amount, count) | Task 1 |
| `DailyBucket` / `AnnualHeatmapData` TS types | Task 3 |
| `useAnnualHeatmapData(year)` query hook | Task 3 |
| Year-keyed query cache (different years cached independently) | Task 3 — `queryKeys.annualHeatmap(year)` includes year |
| `AnnualCalendarHeatmap` is self-contained (owns year state) | Task 4 — no props required |
| Reuses `intensityColor` from `@/lib/heatmap` | Task 4 |
| `DayOfMonthHeatmap` removed | Task 4 |
| TDD: tests before implementation | Tasks 1 & 2 |
| `httptest.NewRequestWithContext` in all tests | Tasks 1 & 2 |
| `--no-gpg-sign` on all commits | All tasks |
| `task lint:be:prod` = 0 issues before each commit | Tasks 1 & 2 |
| Full `task ci` passes at end | Task 4 step 5 |
