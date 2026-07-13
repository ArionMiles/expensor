package postgres

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

type analyticsRepository struct {
	pool    *pgxpool.Pool
	runtime *runtimeRepository
	now     func() time.Time
}

type chartQueryRequest struct {
	Label string
	Query string
	Args  []any
}

type chartDataLoadRequest struct {
	Monthly           chartQueryRequest
	Daily             chartQueryRequest
	Category          chartQueryRequest
	Bucket            chartQueryRequest
	Label             chartQueryRequest
	Source            chartQueryRequest
	SourceType        chartQueryRequest
	Bank              chartQueryRequest
	CategoryMonthlyFn func(context.Context) (map[string]store.CategoryMonthlyEntry, error)
}

func newAnalyticsRepository(deps repositoryDependencies, runtime *runtimeRepository) *analyticsRepository {
	return &analyticsRepository{
		pool:    deps.pool,
		runtime: runtime,
		now:     deps.now,
	}
}

func (r *analyticsRepository) GetStats(ctx context.Context, tenant store.Tenant, baseCurrency string) (*store.Stats, error) {
	return r.statsReadModel(ctx, tenant, baseCurrency)
}

func (r *analyticsRepository) GetChartData(ctx context.Context, tenant store.Tenant) (*store.ChartData, error) {
	return r.chartDataReadModel(ctx, tenant)
}

func (r *analyticsRepository) GetDashboardData(ctx context.Context, tenant store.Tenant) (*store.DashboardData, error) {
	return r.dashboardDataReadModel(ctx, tenant)
}

func (r *analyticsRepository) GetSpendingHeatmap(ctx context.Context, tenant store.Tenant, from, to *time.Time) (*store.HeatmapData, error) {
	return r.spendingHeatmapReadModel(ctx, tenant, from, to)
}

func (r *analyticsRepository) GetAnnualSpend(ctx context.Context, tenant store.Tenant, year int) ([]store.DailyBucket, error) {
	return r.annualSpendReadModel(ctx, tenant, year)
}

func (r *analyticsRepository) GetMonthlyBreakdownSpend(
	ctx context.Context,
	tenant store.Tenant,
	dimension string,
	months int,
) (*store.MonthlyBreakdownData, error) {
	return r.monthlyBreakdownSpendReadModel(ctx, tenant, dimension, months)
}

func (r *analyticsRepository) statsReadModel(ctx context.Context, tenant store.Tenant, baseCurrency string) (*store.Stats, error) {
	const mainQ = `
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN currency = $1 THEN amount ELSE 0 END), 0)
		FROM transactions
		WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $2
	`
	var st store.Stats
	st.BaseCurrency = baseCurrency
	if err := r.pool.QueryRow(ctx, mainQ, baseCurrency, tenantIDParam(tenant)).Scan(&st.TotalCount, &st.TotalBase); err != nil {
		return nil, fmt.Errorf("fetching stats: %w", err)
	}

	const catQ = `
		SELECT COALESCE(NULLIF(category, ''), 'Uncategorized'), COALESCE(SUM(amount), 0), COUNT(*)
		FROM transactions
		WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $1
		GROUP BY COALESCE(NULLIF(category, ''), 'Uncategorized')
		ORDER BY SUM(amount) DESC
	`
	rows, err := r.pool.Query(ctx, catQ, tenantIDParam(tenant))
	if err != nil {
		return nil, fmt.Errorf("fetching category stats: %w", err)
	}
	defer rows.Close()

	st.TotalByCategory = make(map[string]float64)
	st.TotalCategoryCount = make(map[string]int)
	for rows.Next() {
		var cat string
		var amt float64
		var cnt int
		if err := rows.Scan(&cat, &amt, &cnt); err != nil {
			return nil, fmt.Errorf("scanning category row: %w", err)
		}
		st.TotalByCategory[cat] = amt
		st.TotalCategoryCount[cat] = cnt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating category rows: %w", err)
	}

	return &st, nil
}

func (r *analyticsRepository) chartDataReadModel(ctx context.Context, tenant store.Tenant) (*store.ChartData, error) {
	return r.getChartDataAt(ctx, tenant, r.nowTime())
}

func (r *analyticsRepository) getChartDataAt(ctx context.Context, tenant store.Tenant, now time.Time) (*store.ChartData, error) {
	tz := r.appTimezone(ctx, tenant)
	return r.loadChartData(ctx, chartDataLoadRequest{
		Monthly: chartQueryRequest{
			Label: "monthly spend",
			Query: `
			SELECT TO_CHAR(timestamp AT TIME ZONE $1, 'YYYY-MM') AS period,
			       COALESCE(SUM(amount), 0)                     AS amount,
			       COUNT(*)                                     AS cnt
			FROM transactions
			WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $3 AND timestamp >= $2
			GROUP BY period
			ORDER BY period
		`,
			Args: []any{tz, now.AddDate(-1, 0, 0), tenantIDParam(tenant)},
		},
		Daily: chartQueryRequest{
			Label: "daily spend",
			Query: `
			SELECT TO_CHAR(timestamp AT TIME ZONE $1, 'YYYY-MM-DD') AS period,
			       COALESCE(SUM(amount), 0)                        AS amount,
			       COUNT(*)                                        AS cnt
			FROM transactions
			WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $3 AND timestamp >= $2
			GROUP BY period
			ORDER BY period
		`,
			Args: []any{tz, now.AddDate(0, 0, -30), tenantIDParam(tenant)},
		},
		Category: chartQueryRequest{
			Label: "category chart data",
			Query: `
			SELECT COALESCE(NULLIF(category, ''), 'Uncategorized'), COALESCE(SUM(amount), 0)
			FROM transactions
			WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $1
			GROUP BY COALESCE(NULLIF(category, ''), 'Uncategorized')
			ORDER BY SUM(amount) DESC
		`,
			Args: []any{tenantIDParam(tenant)},
		},
		Bucket: chartQueryRequest{
			Label: "bucket chart data",
			Query: `
			SELECT COALESCE(NULLIF(bucket, ''), 'Uncategorized'), COALESCE(SUM(amount), 0)
			FROM transactions
			WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $1
			GROUP BY COALESCE(NULLIF(bucket, ''), 'Uncategorized')
			ORDER BY SUM(amount) DESC
		`,
			Args: []any{tenantIDParam(tenant)},
		},
		Label: chartQueryRequest{
			Label: "label chart data",
			Query: `
			SELECT COALESCE(tl.label, 'Uncategorized'), COALESCE(SUM(t.amount), 0)
			FROM transactions t
			LEFT JOIN transaction_labels tl ON tl.transaction_id = t.id
			WHERE t.muted = false AND t.tenant_id IS NOT DISTINCT FROM $1
			GROUP BY COALESCE(tl.label, 'Uncategorized')
			ORDER BY SUM(t.amount) DESC
			LIMIT 20
		`,
			Args: []any{tenantIDParam(tenant)},
		},
		Source: chartQueryRequest{
			Label: "source chart data",
			Query: `
			SELECT COALESCE(source, ''), COALESCE(SUM(amount), 0)
			FROM transactions
			WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $1 AND source IS NOT NULL AND source != ''
			GROUP BY source
			ORDER BY SUM(amount) DESC
		`,
			Args: []any{tenantIDParam(tenant)},
		},
		SourceType: chartQueryRequest{
			Label: "source type chart data",
			Query: `
			SELECT COALESCE(source_type, ''), COALESCE(SUM(amount), 0)
			FROM transactions
			WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $1 AND source_type IS NOT NULL AND source_type != ''
			GROUP BY source_type
			ORDER BY SUM(amount) DESC
		`,
			Args: []any{tenantIDParam(tenant)},
		},
		Bank: chartQueryRequest{
			Label: "bank chart data",
			Query: `
			SELECT COALESCE(bank, ''), COALESCE(SUM(amount), 0)
			FROM transactions
			WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $1 AND bank IS NOT NULL AND bank != ''
			GROUP BY bank
			ORDER BY SUM(amount) DESC
		`,
			Args: []any{tenantIDParam(tenant)},
		},
		CategoryMonthlyFn: func(ctx context.Context) (map[string]store.CategoryMonthlyEntry, error) {
			return r.queryCategoryMonthlyAt(ctx, tenant, now)
		},
	})
}

func (r *analyticsRepository) dashboardDataReadModel(ctx context.Context, tenant store.Tenant) (*store.DashboardData, error) {
	baseCurrency := r.dashboardBaseCurrency(ctx, tenant)
	now := r.nowTime()
	window := r.dashboardMonthBounds(ctx, tenant, now)

	currentStats, err := r.getStatsBetween(ctx, tenant, baseCurrency, window.startUTC, window.endUTC)
	if err != nil {
		return nil, fmt.Errorf("fetching current-month stats: %w", err)
	}
	currentCharts, err := r.getChartDataBetween(ctx, tenant, window.loc, window.startUTC, window.endUTC)
	if err != nil {
		return nil, fmt.Errorf("fetching current-month charts: %w", err)
	}

	allTimeStats, err := r.GetStats(ctx, tenant, baseCurrency)
	if err != nil {
		return nil, fmt.Errorf("fetching all-time stats: %w", err)
	}
	allTimeCharts, err := r.getChartDataAt(ctx, tenant, now)
	if err != nil {
		return nil, fmt.Errorf("fetching all-time charts: %w", err)
	}

	return &store.DashboardData{
		CurrentMonth: store.DashboardSection{
			Label:  window.label,
			Stats:  *currentStats,
			Charts: *currentCharts,
		},
		AllTime: store.DashboardSection{
			Label:  "All Time",
			Stats:  *allTimeStats,
			Charts: *allTimeCharts,
		},
	}, nil
}

func (r *analyticsRepository) getStatsBetween(ctx context.Context, tenant store.Tenant, baseCurrency string, startUTC, endUTC time.Time) (*store.Stats, error) {
	const mainQ = `
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN currency = $1 THEN amount ELSE 0 END), 0)
		FROM transactions
		WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $4 AND timestamp >= $2 AND timestamp < $3
	`

	st := &store.Stats{
		BaseCurrency:       baseCurrency,
		TotalByCategory:    make(map[string]float64),
		TotalCategoryCount: make(map[string]int),
	}
	if err := r.pool.QueryRow(ctx, mainQ, baseCurrency, startUTC, endUTC, tenantIDParam(tenant)).Scan(&st.TotalCount, &st.TotalBase); err != nil {
		return nil, fmt.Errorf("fetching range stats: %w", err)
	}

	const catQ = `
		SELECT COALESCE(NULLIF(category, ''), 'Uncategorized'), COALESCE(SUM(amount), 0), COUNT(*)
		FROM transactions
		WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $3 AND timestamp >= $1 AND timestamp < $2
		GROUP BY COALESCE(NULLIF(category, ''), 'Uncategorized')
		ORDER BY SUM(amount) DESC
	`
	rows, err := r.pool.Query(ctx, catQ, startUTC, endUTC, tenantIDParam(tenant))
	if err != nil {
		return nil, fmt.Errorf("fetching range category stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cat string
		var amt float64
		var cnt int
		if err := rows.Scan(&cat, &amt, &cnt); err != nil {
			return nil, fmt.Errorf("scanning range category row: %w", err)
		}
		st.TotalByCategory[cat] = amt
		st.TotalCategoryCount[cat] = cnt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating range category rows: %w", err)
	}

	return st, nil
}

func (r *analyticsRepository) getChartDataBetween(
	ctx context.Context,
	tenant store.Tenant,
	loc *time.Location,
	startUTC time.Time,
	endUTC time.Time,
) (*store.ChartData, error) {
	tz := loc.String()
	return r.loadChartData(ctx, chartDataLoadRequest{
		Monthly: chartQueryRequest{
			Label: "range monthly spend",
			Query: `
		SELECT TO_CHAR(timestamp AT TIME ZONE $1, 'YYYY-MM') AS period,
		       COALESCE(SUM(amount), 0)                    AS amount,
		       COUNT(*)                                    AS cnt
		FROM transactions
		WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $4 AND timestamp >= $2 AND timestamp < $3
		GROUP BY period
		ORDER BY period
	`,
			Args: []any{tz, startUTC, endUTC, tenantIDParam(tenant)},
		},
		Daily: chartQueryRequest{
			Label: "range daily spend",
			Query: `
		SELECT TO_CHAR(timestamp AT TIME ZONE $1, 'YYYY-MM-DD') AS period,
		       COALESCE(SUM(amount), 0)                        AS amount,
		       COUNT(*)                                        AS cnt
		FROM transactions
		WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $4 AND timestamp >= $2 AND timestamp < $3
		GROUP BY period
		ORDER BY period
	`,
			Args: []any{tz, startUTC, endUTC, tenantIDParam(tenant)},
		},
		Category: chartQueryRequest{
			Label: "range category chart data",
			Query: `
		SELECT COALESCE(NULLIF(category, ''), 'Uncategorized'), COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $3 AND timestamp >= $1 AND timestamp < $2
		GROUP BY COALESCE(NULLIF(category, ''), 'Uncategorized')
		ORDER BY SUM(amount) DESC
	`,
			Args: []any{startUTC, endUTC, tenantIDParam(tenant)},
		},
		Bucket: chartQueryRequest{
			Label: "range bucket chart data",
			Query: `
		SELECT COALESCE(NULLIF(bucket, ''), 'Uncategorized'), COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $3 AND timestamp >= $1 AND timestamp < $2
		GROUP BY COALESCE(NULLIF(bucket, ''), 'Uncategorized')
		ORDER BY SUM(amount) DESC
	`,
			Args: []any{startUTC, endUTC, tenantIDParam(tenant)},
		},
		Label: chartQueryRequest{
			Label: "range label chart data",
			Query: `
		SELECT COALESCE(tl.label, 'Uncategorized'), COALESCE(SUM(t.amount), 0)
		FROM transactions t
		LEFT JOIN transaction_labels tl ON tl.transaction_id = t.id
		WHERE t.muted = false AND t.tenant_id IS NOT DISTINCT FROM $3 AND t.timestamp >= $1 AND t.timestamp < $2
		GROUP BY COALESCE(tl.label, 'Uncategorized')
		ORDER BY SUM(t.amount) DESC
		LIMIT 20
	`,
			Args: []any{startUTC, endUTC, tenantIDParam(tenant)},
		},
		Source: chartQueryRequest{
			Label: "range source chart data",
			Query: `
		SELECT COALESCE(source, ''), COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $3 AND timestamp >= $1 AND timestamp < $2
		  AND source IS NOT NULL AND source != ''
		GROUP BY source
		ORDER BY SUM(amount) DESC
	`,
			Args: []any{startUTC, endUTC, tenantIDParam(tenant)},
		},
		SourceType: chartQueryRequest{
			Label: "range source type chart data",
			Query: `
		SELECT COALESCE(source_type, ''), COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $3 AND timestamp >= $1 AND timestamp < $2
		  AND source_type IS NOT NULL AND source_type != ''
		GROUP BY source_type
		ORDER BY SUM(amount) DESC
	`,
			Args: []any{startUTC, endUTC, tenantIDParam(tenant)},
		},
		Bank: chartQueryRequest{
			Label: "range bank chart data",
			Query: `
		SELECT COALESCE(bank, ''), COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $3 AND timestamp >= $1 AND timestamp < $2
		  AND bank IS NOT NULL AND bank != ''
		GROUP BY bank
		ORDER BY SUM(amount) DESC
	`,
			Args: []any{startUTC, endUTC, tenantIDParam(tenant)},
		},
		CategoryMonthlyFn: func(ctx context.Context) (map[string]store.CategoryMonthlyEntry, error) {
			return r.queryCategoryMonthlyBetween(ctx, tenant, loc, startUTC, endUTC)
		},
	})
}

// queryCategoryMonthly returns per-category spend totals for the current and prior calendar month.
func (r *analyticsRepository) queryCategoryMonthly(ctx context.Context, tenant store.Tenant) (map[string]store.CategoryMonthlyEntry, error) {
	return r.queryCategoryMonthlyAt(ctx, tenant, r.nowTime())
}

func (r *analyticsRepository) queryCategoryMonthlyAt(ctx context.Context, tenant store.Tenant, now time.Time) (map[string]store.CategoryMonthlyEntry, error) {
	window := r.dashboardMonthBounds(ctx, tenant, now)
	return r.queryCategoryMonthlyBetween(ctx, tenant, window.loc, window.startUTC, window.endUTC)
}

func (r *analyticsRepository) queryCategoryMonthlyBetween(
	ctx context.Context,
	tenant store.Tenant,
	loc *time.Location,
	startUTC, endUTC time.Time,
) (map[string]store.CategoryMonthlyEntry, error) {
	startLocal := startUTC.In(loc)
	priorStartUTC := time.Date(startLocal.Year(), startLocal.Month()-1, 1, 0, 0, 0, 0, loc).UTC()

	const q = `
		SELECT
		    COALESCE(NULLIF(category, ''), 'Uncategorized') AS category,
		    COALESCE(SUM(amount) FILTER (WHERE timestamp >= $1 AND timestamp < $2), 0) AS current_month,
		    COALESCE(SUM(amount) FILTER (
		        WHERE timestamp >= $3
		          AND timestamp  < $1
		    ), 0) AS prior_month
		FROM transactions
		WHERE muted = false
		    AND tenant_id IS NOT DISTINCT FROM $4
		    AND timestamp >= $3
		    AND timestamp < $2
		GROUP BY COALESCE(NULLIF(category, ''), 'Uncategorized')
	`
	rows, err := r.pool.Query(ctx, q, startUTC, endUTC, priorStartUTC, tenantIDParam(tenant))
	if err != nil {
		return nil, fmt.Errorf("fetching category monthly data: %w", err)
	}
	defer rows.Close()

	m := make(map[string]store.CategoryMonthlyEntry)
	for rows.Next() {
		var cat string
		var entry store.CategoryMonthlyEntry
		if err := rows.Scan(&cat, &entry.Current, &entry.Prior); err != nil {
			return nil, fmt.Errorf("scanning category monthly row: %w", err)
		}
		m[cat] = entry
	}
	return m, rows.Err()
}

func (r *analyticsRepository) nowTime() time.Time {
	if r != nil && r.now != nil {
		return r.now()
	}
	return time.Now()
}

// GetSpendingHeatmap returns transaction totals aggregated by weekday×hour and
func (r *analyticsRepository) spendingHeatmapReadModel(ctx context.Context, tenant store.Tenant, from, to *time.Time) (*store.HeatmapData, error) {
	hd := &store.HeatmapData{
		ByWeekdayHour: []store.WeekdayHourBucket{},
		ByDayOfMonth:  []store.DayOfMonthBucket{},
	}

	where, args := buildHeatmapWhere(tenant, from, to)
	tz := r.appTimezone(ctx, tenant)
	argsWithTZ := append(append([]any{}, args...), tz)
	tzArg := fmt.Sprintf("$%d", len(argsWithTZ))

	// Weekday × hour grid (7 rows × 24 columns = up to 168 buckets).
	wdhQuery := fmt.Sprintf(`
		SELECT
			EXTRACT(DOW  FROM timestamp AT TIME ZONE %s)::int AS weekday,
			EXTRACT(HOUR FROM timestamp AT TIME ZONE %s)::int AS hour,
			COALESCE(SUM(amount), 0)          AS amount,
			COUNT(*)                          AS count
		FROM transactions%s
		GROUP BY 1, 2
		ORDER BY 1, 2
	`, tzArg, tzArg, where)
	wdhRows, err := r.pool.Query(ctx, wdhQuery, argsWithTZ...)
	if err != nil {
		return nil, fmt.Errorf("fetching weekday/hour heatmap: %w", err)
	}
	defer wdhRows.Close()
	for wdhRows.Next() {
		var b store.WeekdayHourBucket
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
			EXTRACT(DAY FROM timestamp AT TIME ZONE %s)::int AS day,
			COALESCE(SUM(amount), 0)         AS amount,
			COUNT(*)                         AS count
		FROM transactions%s
		GROUP BY 1
		ORDER BY 1
	`, tzArg, where)
	domRows, err := r.pool.Query(ctx, domQuery, argsWithTZ...)
	if err != nil {
		return nil, fmt.Errorf("fetching day-of-month heatmap: %w", err)
	}
	defer domRows.Close()
	for domRows.Next() {
		var b store.DayOfMonthBucket
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

// GetAnnualSpend returns per-day transaction totals for a given calendar year.
// Results are ordered by date ascending. Returns an empty (non-nil) slice when
func (r *analyticsRepository) annualSpendReadModel(ctx context.Context, tenant store.Tenant, year int) ([]store.DailyBucket, error) {
	buckets := []store.DailyBucket{}
	tz := r.appTimezone(ctx, tenant)

	rows, err := r.pool.Query(ctx, `
		SELECT
			(timestamp AT TIME ZONE $2)::date AS date,
			COALESCE(SUM(amount), 0)           AS amount,
			COUNT(*)                           AS count
		FROM transactions
		WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $3 AND EXTRACT(YEAR FROM timestamp AT TIME ZONE $2) = $1
		GROUP BY date
		ORDER BY date
	`, year, tz, tenantIDParam(tenant))
	if err != nil {
		return nil, fmt.Errorf("fetching annual spend for %d: %w", year, err)
	}
	defer rows.Close()
	for rows.Next() {
		var b store.DailyBucket
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

// buildHeatmapWhere returns a WHERE clause and positional args for
// GetSpendingHeatmap. Returns empty string and nil args when both are nil.
func buildHeatmapWhere(tenant store.Tenant, from, to *time.Time) (string, []any) {
	if from == nil && to == nil {
		return " WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $1", []any{tenantIDParam(tenant)}
	}
	if from == nil {
		return " WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $2 AND timestamp <= $1", []any{*to, tenantIDParam(tenant)}
	}
	if to == nil {
		return " WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $2 AND timestamp >= $1", []any{*from, tenantIDParam(tenant)}
	}
	return " WHERE muted = false AND tenant_id IS NOT DISTINCT FROM $3 AND timestamp >= $1 AND timestamp <= $2", []any{*from, *to, tenantIDParam(tenant)}
}

// GetFacets returns the distinct non-empty values for source, category, currency, and label
func (r *analyticsRepository) queryTimeBuckets(ctx context.Context, q string, args ...any) ([]store.TimeBucket, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	buckets := []store.TimeBucket{}
	for rows.Next() {
		var b store.TimeBucket
		if err := rows.Scan(&b.Period, &b.Amount, &b.Count); err != nil {
			return nil, err
		}
		buckets = append(buckets, b)
	}
	return buckets, rows.Err()
}

func (r *analyticsRepository) queryStringFloat(ctx context.Context, q string, dest map[string]float64, args ...any) error {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var k string
		var v float64
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		dest[k] = v
	}
	return rows.Err()
}

func (r *analyticsRepository) loadChartData(ctx context.Context, request chartDataLoadRequest) (*store.ChartData, error) {
	cd := newChartData()
	g, groupCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		buckets, err := r.queryTimeBuckets(groupCtx, request.Monthly.Query, request.Monthly.Args...)
		if err != nil {
			return fmt.Errorf("fetching %s: %w", request.Monthly.Label, err)
		}
		cd.MonthlySpend = buckets
		return nil
	})

	g.Go(func() error {
		buckets, err := r.queryTimeBuckets(groupCtx, request.Daily.Query, request.Daily.Args...)
		if err != nil {
			return fmt.Errorf("fetching %s: %w", request.Daily.Label, err)
		}
		cd.DailySpend = buckets
		return nil
	})

	loadStringFloat := func(request chartQueryRequest, dest map[string]float64) {
		g.Go(func() error {
			if err := r.queryStringFloat(groupCtx, request.Query, dest, request.Args...); err != nil {
				return fmt.Errorf("fetching %s: %w", request.Label, err)
			}
			return nil
		})
	}

	loadStringFloat(request.Category, cd.ByCategory)
	loadStringFloat(request.Bucket, cd.ByBucket)
	loadStringFloat(request.Label, cd.ByLabel)
	loadStringFloat(request.Source, cd.BySource)
	loadStringFloat(request.SourceType, cd.BySourceType)
	loadStringFloat(request.Bank, cd.ByBank)

	g.Go(func() error {
		m, err := request.CategoryMonthlyFn(groupCtx)
		if err != nil {
			return err
		}
		cd.ByCategoryMonthly = m
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return cd, nil
}

func newChartData() *store.ChartData {
	return &store.ChartData{
		MonthlySpend:      []store.TimeBucket{},
		DailySpend:        []store.TimeBucket{},
		ByCategory:        make(map[string]float64),
		ByBucket:          make(map[string]float64),
		ByLabel:           make(map[string]float64),
		BySource:          make(map[string]float64),
		BySourceType:      make(map[string]float64),
		ByBank:            make(map[string]float64),
		ByCategoryMonthly: make(map[string]store.CategoryMonthlyEntry),
	}
}

func (r *analyticsRepository) monthlyBreakdownSpendReadModel(
	ctx context.Context,
	tenant store.Tenant,
	dimension string,
	months int,
) (*store.MonthlyBreakdownData, error) {
	if months <= 0 {
		return &store.MonthlyBreakdownData{
			Labels: []string{},
			Months: []string{},
			Series: []store.MonthlyBreakdownSeries{},
		}, nil
	}

	dimension = strings.ToLower(strings.TrimSpace(dimension))
	if dimension == "" {
		dimension = "labels"
	}

	tz := r.appTimezone(ctx, tenant)
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}

	now := r.nowTime().In(loc)
	startLocal := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc).AddDate(0, -(months - 1), 0)
	startUTC := startLocal.UTC()

	type monthlyBreakdownBucket struct {
		Label  string
		Month  string
		Amount float64
	}

	var (
		query string
		args  []any
	)

	switch dimension {
	case "labels":
		query = `
			SELECT
				COALESCE(tl.label, 'Uncategorized') AS label,
				TO_CHAR(t.timestamp AT TIME ZONE $1, 'YYYY-MM') AS month,
				COALESCE(SUM(t.amount), 0) AS amount
			FROM transactions t
			LEFT JOIN transaction_labels tl ON tl.transaction_id = t.id
			WHERE t.muted = false
			  AND t.timestamp >= $2
			  AND t.tenant_id IS NOT DISTINCT FROM $3
			GROUP BY COALESCE(tl.label, 'Uncategorized'), month
			ORDER BY month, label
		`
		args = []any{tz, startUTC, tenantIDParam(tenant)}
	case "categories":
		query = `
			SELECT
				COALESCE(NULLIF(t.category, ''), 'Uncategorized') AS label,
				TO_CHAR(t.timestamp AT TIME ZONE $1, 'YYYY-MM') AS month,
				COALESCE(SUM(t.amount), 0) AS amount
			FROM transactions t
			WHERE t.muted = false
			  AND t.timestamp >= $2
			  AND t.tenant_id IS NOT DISTINCT FROM $3
			GROUP BY COALESCE(NULLIF(t.category, ''), 'Uncategorized'), month
			ORDER BY month, label
		`
		args = []any{tz, startUTC, tenantIDParam(tenant)}
	case "buckets":
		query = `
			SELECT
				COALESCE(NULLIF(t.bucket, ''), 'Uncategorized') AS label,
				TO_CHAR(t.timestamp AT TIME ZONE $1, 'YYYY-MM') AS month,
				COALESCE(SUM(t.amount), 0) AS amount
			FROM transactions t
			WHERE t.muted = false
			  AND t.timestamp >= $2
			  AND t.tenant_id IS NOT DISTINCT FROM $3
			GROUP BY COALESCE(NULLIF(t.bucket, ''), 'Uncategorized'), month
			ORDER BY month, label
		`
		args = []any{tz, startUTC, tenantIDParam(tenant)}
	default:
		return nil, fmt.Errorf("unsupported monthly breakdown dimension %q", dimension)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("fetching %s monthly spend: %w", dimension, err)
	}
	defer rows.Close()

	lookup := make(map[string]map[string]float64)
	labelSet := make(map[string]struct{})
	for rows.Next() {
		var bucket monthlyBreakdownBucket
		if err := rows.Scan(&bucket.Label, &bucket.Month, &bucket.Amount); err != nil {
			return nil, fmt.Errorf("scanning %s monthly bucket: %w", dimension, err)
		}
		if lookup[bucket.Label] == nil {
			lookup[bucket.Label] = make(map[string]float64)
		}
		lookup[bucket.Label][bucket.Month] = bucket.Amount
		labelSet[bucket.Label] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	monthLabels := make([]string, 0, months)
	for i := 0; i < months; i++ {
		monthLabels = append(monthLabels, startLocal.AddDate(0, i, 0).Format("2006-01"))
	}

	labels := make([]string, 0, len(labelSet))
	for label := range labelSet {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	series := make([]store.MonthlyBreakdownSeries, 0, len(labels))
	for _, label := range labels {
		values := make([]float64, len(monthLabels))
		for i, month := range monthLabels {
			values[i] = lookup[label][month]
		}
		series = append(series, store.MonthlyBreakdownSeries{Label: label, Data: values})
	}

	return &store.MonthlyBreakdownData{
		Labels: labels,
		Months: monthLabels,
		Series: series,
	}, nil
}

func (r *analyticsRepository) appTimezone(ctx context.Context, tenant store.Tenant) string {
	const fallback = "UTC"

	tz, err := r.runtime.GetAppConfig(ctx, tenant, "app.timezone")
	if err != nil || tz == "" {
		return fallback
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return fallback
	}
	return tz
}

func (r *analyticsRepository) dashboardBaseCurrency(ctx context.Context, tenant store.Tenant) string {
	const fallback = "INR"

	baseCurrency, err := r.runtime.GetAppConfig(ctx, tenant, "base_currency")
	if err != nil || baseCurrency == "" {
		return fallback
	}
	return baseCurrency
}

type dashboardMonthWindow struct {
	loc      *time.Location
	startUTC time.Time
	endUTC   time.Time
	label    string
}

func (r *analyticsRepository) dashboardMonthBounds(ctx context.Context, tenant store.Tenant, now time.Time) dashboardMonthWindow {
	tz := r.appTimezone(ctx, tenant)
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}

	localNow := now.In(loc)
	startLocal := time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, loc)
	endLocal := startLocal.AddDate(0, 1, 0)
	return dashboardMonthWindow{
		loc:      loc,
		startUTC: startLocal.UTC(),
		endUTC:   endLocal.UTC(),
		label:    startLocal.Format("January 2006"),
	}
}
