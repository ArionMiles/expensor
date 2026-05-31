package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ReadModelRepository interface {
	GetStats(ctx context.Context, baseCurrency string) (*Stats, error)
	GetChartData(ctx context.Context) (*ChartData, error)
	GetDashboardData(ctx context.Context) (*DashboardData, error)
	GetSpendingHeatmap(ctx context.Context, from, to *time.Time) (*HeatmapData, error)
	GetAnnualSpend(ctx context.Context, year int) ([]DailyBucket, error)
	GetMonthlyBreakdownSpend(ctx context.Context, dimension string, months int) (*MonthlyBreakdownData, error)
}

type pgReadModelRepository struct {
	pool    *pgxpool.Pool
	runtime RuntimeRepository
	now     func() time.Time
}

func NewReadModelRepository(deps repositoryDependencies, runtime RuntimeRepository) ReadModelRepository {
	return newPGReadModelRepository(deps, runtime)
}

func newPGReadModelRepository(deps repositoryDependencies, runtime RuntimeRepository) *pgReadModelRepository {
	return &pgReadModelRepository{
		pool:    deps.pool,
		runtime: runtime,
		now:     deps.now,
	}
}

func (r *pgReadModelRepository) GetStats(ctx context.Context, baseCurrency string) (*Stats, error) {
	return r.statsReadModel(ctx, baseCurrency)
}

func (r *pgReadModelRepository) GetChartData(ctx context.Context) (*ChartData, error) {
	return r.chartDataReadModel(ctx)
}

func (r *pgReadModelRepository) GetDashboardData(ctx context.Context) (*DashboardData, error) {
	return r.dashboardDataReadModel(ctx)
}

func (r *pgReadModelRepository) GetSpendingHeatmap(ctx context.Context, from, to *time.Time) (*HeatmapData, error) {
	return r.spendingHeatmapReadModel(ctx, from, to)
}

func (r *pgReadModelRepository) GetAnnualSpend(ctx context.Context, year int) ([]DailyBucket, error) {
	return r.annualSpendReadModel(ctx, year)
}

func (r *pgReadModelRepository) GetMonthlyBreakdownSpend(ctx context.Context, dimension string, months int) (*MonthlyBreakdownData, error) {
	return r.monthlyBreakdownSpendReadModel(ctx, dimension, months)
}

func (r *pgReadModelRepository) statsReadModel(ctx context.Context, baseCurrency string) (*Stats, error) {
	const mainQ = `
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN currency = $1 THEN amount ELSE 0 END), 0)
		FROM transactions
		WHERE muted = false
	`
	var st Stats
	st.BaseCurrency = baseCurrency
	if err := r.pool.QueryRow(ctx, mainQ, baseCurrency).Scan(&st.TotalCount, &st.TotalBase); err != nil {
		return nil, fmt.Errorf("fetching stats: %w", err)
	}

	const catQ = `
		SELECT COALESCE(NULLIF(category, ''), 'Uncategorized'), COALESCE(SUM(amount), 0), COUNT(*)
		FROM transactions
		WHERE muted = false
		GROUP BY COALESCE(NULLIF(category, ''), 'Uncategorized')
		ORDER BY SUM(amount) DESC
	`
	rows, err := r.pool.Query(ctx, catQ)
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

func (r *pgReadModelRepository) chartDataReadModel(ctx context.Context) (*ChartData, error) {
	return r.getChartDataAt(ctx, r.nowTime())
}

func (r *pgReadModelRepository) getChartDataAt(ctx context.Context, now time.Time) (*ChartData, error) {
	cd := &ChartData{
		MonthlySpend:      []TimeBucket{},
		DailySpend:        []TimeBucket{},
		ByCategory:        make(map[string]float64),
		ByBucket:          make(map[string]float64),
		ByLabel:           make(map[string]float64),
		BySource:          make(map[string]float64),
		BySourceType:      make(map[string]float64),
		ByBank:            make(map[string]float64),
		ByCategoryMonthly: make(map[string]CategoryMonthlyEntry),
	}

	var mu sync.Mutex
	var firstErr error

	recordErr := func(err error) {
		mu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		mu.Unlock()
	}

	tz := r.appTimezone(ctx)

	var wg sync.WaitGroup
	wg.Add(9)

	go func() {
		defer wg.Done()
		buckets, err := r.queryTimeBuckets(ctx, `
			SELECT TO_CHAR(timestamp AT TIME ZONE $1, 'YYYY-MM') AS period,
			       COALESCE(SUM(amount), 0)                     AS amount,
			       COUNT(*)                                     AS cnt
			FROM transactions
			WHERE muted = false AND timestamp >= $2
			GROUP BY period
			ORDER BY period
		`, tz, now.AddDate(-1, 0, 0))
		if err != nil {
			recordErr(fmt.Errorf("fetching monthly spend: %w", err))
			return
		}
		mu.Lock()
		cd.MonthlySpend = buckets
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		buckets, err := r.queryTimeBuckets(ctx, `
			SELECT TO_CHAR(timestamp AT TIME ZONE $1, 'YYYY-MM-DD') AS period,
			       COALESCE(SUM(amount), 0)                        AS amount,
			       COUNT(*)                                        AS cnt
			FROM transactions
			WHERE muted = false AND timestamp >= $2
			GROUP BY period
			ORDER BY period
		`, tz, now.AddDate(0, 0, -30))
		if err != nil {
			recordErr(fmt.Errorf("fetching daily spend: %w", err))
			return
		}
		mu.Lock()
		cd.DailySpend = buckets
		mu.Unlock()
	}()

	r.loadStringFloatChart(ctx, &wg, &mu, recordErr, chartLoadRequest{Target: &cd.ByCategory, Label: "category", Query: `
			SELECT COALESCE(NULLIF(category, ''), 'Uncategorized'), COALESCE(SUM(amount), 0)
			FROM transactions
			WHERE muted = false
			GROUP BY COALESCE(NULLIF(category, ''), 'Uncategorized')
			ORDER BY SUM(amount) DESC
		`})

	r.loadStringFloatChart(ctx, &wg, &mu, recordErr, chartLoadRequest{Target: &cd.ByBucket, Label: "bucket", Query: `
			SELECT COALESCE(NULLIF(bucket, ''), 'Uncategorized'), COALESCE(SUM(amount), 0)
			FROM transactions
			WHERE muted = false
			GROUP BY COALESCE(NULLIF(bucket, ''), 'Uncategorized')
			ORDER BY SUM(amount) DESC
		`})

	r.loadStringFloatChart(ctx, &wg, &mu, recordErr, chartLoadRequest{Target: &cd.ByLabel, Label: "label", Query: `
			SELECT COALESCE(tl.label, 'Uncategorized'), COALESCE(SUM(t.amount), 0)
			FROM transactions t
			LEFT JOIN transaction_labels tl ON tl.transaction_id = t.id
			WHERE t.muted = false
			GROUP BY COALESCE(tl.label, 'Uncategorized')
			ORDER BY SUM(t.amount) DESC
			LIMIT 20
		`})

	r.loadStringFloatChart(ctx, &wg, &mu, recordErr, chartLoadRequest{Target: &cd.BySource, Label: "source", Query: `
			SELECT COALESCE(source, ''), COALESCE(SUM(amount), 0)
			FROM transactions
			WHERE muted = false AND source IS NOT NULL AND source != ''
			GROUP BY source
			ORDER BY SUM(amount) DESC
		`})

	r.loadStringFloatChart(ctx, &wg, &mu, recordErr, chartLoadRequest{Target: &cd.BySourceType, Label: "source type", Query: `
			SELECT COALESCE(source_type, ''), COALESCE(SUM(amount), 0)
			FROM transactions
			WHERE muted = false AND source_type IS NOT NULL AND source_type != ''
			GROUP BY source_type
			ORDER BY SUM(amount) DESC
		`})

	r.loadStringFloatChart(ctx, &wg, &mu, recordErr, chartLoadRequest{Target: &cd.ByBank, Label: "bank", Query: `
			SELECT COALESCE(bank, ''), COALESCE(SUM(amount), 0)
			FROM transactions
			WHERE muted = false AND bank IS NOT NULL AND bank != ''
			GROUP BY bank
			ORDER BY SUM(amount) DESC
		`})

	go func() {
		defer wg.Done()
		m, err := r.queryCategoryMonthlyAt(ctx, now)
		if err != nil {
			recordErr(err)
			return
		}
		mu.Lock()
		cd.ByCategoryMonthly = m
		mu.Unlock()
	}()

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	return cd, nil
}

func (r *pgReadModelRepository) dashboardDataReadModel(ctx context.Context) (*DashboardData, error) {
	baseCurrency := r.dashboardBaseCurrency(ctx)
	now := r.nowTime()
	window := r.dashboardMonthBounds(ctx, now)

	currentStats, err := r.getStatsBetween(ctx, baseCurrency, window.startUTC, window.endUTC)
	if err != nil {
		return nil, fmt.Errorf("fetching current-month stats: %w", err)
	}
	currentCharts, err := r.getChartDataBetween(ctx, window.loc, window.startUTC, window.endUTC)
	if err != nil {
		return nil, fmt.Errorf("fetching current-month charts: %w", err)
	}

	allTimeStats, err := r.GetStats(ctx, baseCurrency)
	if err != nil {
		return nil, fmt.Errorf("fetching all-time stats: %w", err)
	}
	allTimeCharts, err := r.getChartDataAt(ctx, now)
	if err != nil {
		return nil, fmt.Errorf("fetching all-time charts: %w", err)
	}

	return &DashboardData{
		CurrentMonth: DashboardSection{
			Label:  window.label,
			Stats:  *currentStats,
			Charts: *currentCharts,
		},
		AllTime: DashboardSection{
			Label:  "All Time",
			Stats:  *allTimeStats,
			Charts: *allTimeCharts,
		},
	}, nil
}

func (r *pgReadModelRepository) getStatsBetween(ctx context.Context, baseCurrency string, startUTC, endUTC time.Time) (*Stats, error) {
	const mainQ = `
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN currency = $1 THEN amount ELSE 0 END), 0)
		FROM transactions
		WHERE muted = false AND timestamp >= $2 AND timestamp < $3
	`

	st := &Stats{
		BaseCurrency:       baseCurrency,
		TotalByCategory:    make(map[string]float64),
		TotalCategoryCount: make(map[string]int),
	}
	if err := r.pool.QueryRow(ctx, mainQ, baseCurrency, startUTC, endUTC).Scan(&st.TotalCount, &st.TotalBase); err != nil {
		return nil, fmt.Errorf("fetching range stats: %w", err)
	}

	const catQ = `
		SELECT COALESCE(NULLIF(category, ''), 'Uncategorized'), COALESCE(SUM(amount), 0), COUNT(*)
		FROM transactions
		WHERE muted = false AND timestamp >= $1 AND timestamp < $2
		GROUP BY COALESCE(NULLIF(category, ''), 'Uncategorized')
		ORDER BY SUM(amount) DESC
	`
	rows, err := r.pool.Query(ctx, catQ, startUTC, endUTC)
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

func (r *pgReadModelRepository) getChartDataBetween(ctx context.Context, loc *time.Location, startUTC, endUTC time.Time) (*ChartData, error) {
	cd := &ChartData{
		MonthlySpend:      []TimeBucket{},
		DailySpend:        []TimeBucket{},
		ByCategory:        make(map[string]float64),
		ByBucket:          make(map[string]float64),
		ByLabel:           make(map[string]float64),
		BySource:          make(map[string]float64),
		BySourceType:      make(map[string]float64),
		ByBank:            make(map[string]float64),
		ByCategoryMonthly: make(map[string]CategoryMonthlyEntry),
	}

	tz := loc.String()

	var err error
	if cd.MonthlySpend, err = r.queryTimeBuckets(ctx, `
		SELECT TO_CHAR(timestamp AT TIME ZONE $1, 'YYYY-MM') AS period,
		       COALESCE(SUM(amount), 0)                    AS amount,
		       COUNT(*)                                    AS cnt
		FROM transactions
		WHERE muted = false AND timestamp >= $2 AND timestamp < $3
		GROUP BY period
		ORDER BY period
	`, tz, startUTC, endUTC); err != nil {
		return nil, fmt.Errorf("fetching range monthly spend: %w", err)
	}

	if cd.DailySpend, err = r.queryTimeBuckets(ctx, `
		SELECT TO_CHAR(timestamp AT TIME ZONE $1, 'YYYY-MM-DD') AS period,
		       COALESCE(SUM(amount), 0)                        AS amount,
		       COUNT(*)                                        AS cnt
		FROM transactions
		WHERE muted = false AND timestamp >= $2 AND timestamp < $3
		GROUP BY period
		ORDER BY period
	`, tz, startUTC, endUTC); err != nil {
		return nil, fmt.Errorf("fetching range daily spend: %w", err)
	}

	if err := r.queryStringFloat(ctx, `
		SELECT COALESCE(NULLIF(category, ''), 'Uncategorized'), COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE muted = false AND timestamp >= $1 AND timestamp < $2
		GROUP BY COALESCE(NULLIF(category, ''), 'Uncategorized')
		ORDER BY SUM(amount) DESC
	`, cd.ByCategory, startUTC, endUTC); err != nil {
		return nil, fmt.Errorf("fetching range category chart data: %w", err)
	}

	if err := r.queryStringFloat(ctx, `
		SELECT COALESCE(NULLIF(bucket, ''), 'Uncategorized'), COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE muted = false AND timestamp >= $1 AND timestamp < $2
		GROUP BY COALESCE(NULLIF(bucket, ''), 'Uncategorized')
		ORDER BY SUM(amount) DESC
	`, cd.ByBucket, startUTC, endUTC); err != nil {
		return nil, fmt.Errorf("fetching range bucket chart data: %w", err)
	}

	if err := r.queryStringFloat(ctx, `
		SELECT COALESCE(tl.label, 'Uncategorized'), COALESCE(SUM(t.amount), 0)
		FROM transactions t
		LEFT JOIN transaction_labels tl ON tl.transaction_id = t.id
		WHERE t.muted = false AND t.timestamp >= $1 AND t.timestamp < $2
		GROUP BY COALESCE(tl.label, 'Uncategorized')
		ORDER BY SUM(t.amount) DESC
		LIMIT 20
	`, cd.ByLabel, startUTC, endUTC); err != nil {
		return nil, fmt.Errorf("fetching range label chart data: %w", err)
	}

	if err := r.queryStringFloat(ctx, `
		SELECT COALESCE(source, ''), COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE muted = false AND timestamp >= $1 AND timestamp < $2
		  AND source IS NOT NULL AND source != ''
		GROUP BY source
		ORDER BY SUM(amount) DESC
	`, cd.BySource, startUTC, endUTC); err != nil {
		return nil, fmt.Errorf("fetching range source chart data: %w", err)
	}

	if err := r.queryStringFloat(ctx, `
		SELECT COALESCE(source_type, ''), COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE muted = false AND timestamp >= $1 AND timestamp < $2
		  AND source_type IS NOT NULL AND source_type != ''
		GROUP BY source_type
		ORDER BY SUM(amount) DESC
	`, cd.BySourceType, startUTC, endUTC); err != nil {
		return nil, fmt.Errorf("fetching range source type chart data: %w", err)
	}

	if err := r.queryStringFloat(ctx, `
		SELECT COALESCE(bank, ''), COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE muted = false AND timestamp >= $1 AND timestamp < $2
		  AND bank IS NOT NULL AND bank != ''
		GROUP BY bank
		ORDER BY SUM(amount) DESC
	`, cd.ByBank, startUTC, endUTC); err != nil {
		return nil, fmt.Errorf("fetching range bank chart data: %w", err)
	}

	if cd.ByCategoryMonthly, err = r.queryCategoryMonthlyBetween(ctx, loc, startUTC, endUTC); err != nil {
		return nil, err
	}

	return cd, nil
}

// queryCategoryMonthly returns per-category spend totals for the current and prior calendar month.
func (r *pgReadModelRepository) queryCategoryMonthly(ctx context.Context) (map[string]CategoryMonthlyEntry, error) {
	return r.queryCategoryMonthlyAt(ctx, r.nowTime())
}

func (r *pgReadModelRepository) queryCategoryMonthlyAt(ctx context.Context, now time.Time) (map[string]CategoryMonthlyEntry, error) {
	window := r.dashboardMonthBounds(ctx, now)
	return r.queryCategoryMonthlyBetween(ctx, window.loc, window.startUTC, window.endUTC)
}

func (r *pgReadModelRepository) queryCategoryMonthlyBetween(
	ctx context.Context,
	loc *time.Location,
	startUTC, endUTC time.Time,
) (map[string]CategoryMonthlyEntry, error) {
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
		    AND timestamp >= $3
		    AND timestamp < $2
		GROUP BY COALESCE(NULLIF(category, ''), 'Uncategorized')
	`
	rows, err := r.pool.Query(ctx, q, startUTC, endUTC, priorStartUTC)
	if err != nil {
		return nil, fmt.Errorf("fetching category monthly data: %w", err)
	}
	defer rows.Close()

	m := make(map[string]CategoryMonthlyEntry)
	for rows.Next() {
		var cat string
		var entry CategoryMonthlyEntry
		if err := rows.Scan(&cat, &entry.Current, &entry.Prior); err != nil {
			return nil, fmt.Errorf("scanning category monthly row: %w", err)
		}
		m[cat] = entry
	}
	return m, rows.Err()
}

func (r *pgReadModelRepository) nowTime() time.Time {
	if r != nil && r.now != nil {
		return r.now()
	}
	return time.Now()
}

// GetSpendingHeatmap returns transaction totals aggregated by weekday×hour and
func (r *pgReadModelRepository) spendingHeatmapReadModel(ctx context.Context, from, to *time.Time) (*HeatmapData, error) {
	hd := &HeatmapData{
		ByWeekdayHour: []WeekdayHourBucket{},
		ByDayOfMonth:  []DayOfMonthBucket{},
	}

	where, args := buildHeatmapWhere(from, to)
	tz := r.appTimezone(ctx)
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

// GetAnnualSpend returns per-day transaction totals for a given calendar year.
// Results are ordered by date ascending. Returns an empty (non-nil) slice when
func (r *pgReadModelRepository) annualSpendReadModel(ctx context.Context, year int) ([]DailyBucket, error) {
	buckets := []DailyBucket{}

	rows, err := r.pool.Query(ctx, `
		SELECT
			timestamp::date          AS date,
			COALESCE(SUM(amount), 0) AS amount,
			COUNT(*)                 AS count
		FROM transactions
		WHERE muted = false AND EXTRACT(YEAR FROM timestamp) = $1
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

// buildHeatmapWhere returns a WHERE clause and positional args for
// GetSpendingHeatmap. Returns empty string and nil args when both are nil.
func buildHeatmapWhere(from, to *time.Time) (string, []any) {
	if from == nil && to == nil {
		return " WHERE muted = false", nil
	}
	if from == nil {
		return " WHERE muted = false AND timestamp <= $1", []any{*to}
	}
	if to == nil {
		return " WHERE muted = false AND timestamp >= $1", []any{*from}
	}
	return " WHERE muted = false AND timestamp >= $1 AND timestamp <= $2", []any{*from, *to}
}

// Facets holds distinct filter values for the transactions UI dropdowns.
type Facets struct {
	Sources        []string       `json:"sources"`
	SourceTypes    []string       `json:"source_types"`
	Banks          []string       `json:"banks"`
	Categories     []string       `json:"categories"`
	CategoryCounts map[string]int `json:"category_counts"`
	Currencies     []string       `json:"currencies"`
	Merchants      []string       `json:"merchants"`
	Labels         []string       `json:"labels"`
	LabelCounts    map[string]int `json:"label_counts"`
	Buckets        []string       `json:"buckets"`
	BucketCounts   map[string]int `json:"bucket_counts"`
}

// GetFacets returns the distinct non-empty values for source, category, currency, and label
func (r *pgReadModelRepository) queryTimeBuckets(ctx context.Context, q string, args ...any) ([]TimeBucket, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []TimeBucket
	for rows.Next() {
		var b TimeBucket
		if err := rows.Scan(&b.Period, &b.Amount, &b.Count); err != nil {
			return nil, err
		}
		buckets = append(buckets, b)
	}
	return buckets, rows.Err()
}

func (r *pgReadModelRepository) queryStringFloat(ctx context.Context, q string, dest map[string]float64, args ...any) error {
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

func (r *pgReadModelRepository) loadStringFloatChart(
	ctx context.Context,
	wg *sync.WaitGroup,
	mu *sync.Mutex,
	recordErr func(error),
	request chartLoadRequest,
) {
	go func() {
		defer wg.Done()
		values := make(map[string]float64)
		if err := r.queryStringFloat(ctx, request.Query, values, request.Args...); err != nil {
			recordErr(fmt.Errorf("fetching %s chart data: %w", request.Label, err))
			return
		}
		mu.Lock()
		*request.Target = values
		mu.Unlock()
	}()
}

// initAppConfig creates the app_config table and seeds operational defaults.
// It is called once from New and is idempotent.
func (r *pgReadModelRepository) monthlyBreakdownSpendReadModel(ctx context.Context, dimension string, months int) (*MonthlyBreakdownData, error) {
	if months <= 0 {
		return &MonthlyBreakdownData{
			Labels: []string{},
			Months: []string{},
			Series: []MonthlyBreakdownSeries{},
		}, nil
	}

	dimension = strings.ToLower(strings.TrimSpace(dimension))
	if dimension == "" {
		dimension = "labels"
	}

	tz := r.appTimezone(ctx)
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
			GROUP BY COALESCE(tl.label, 'Uncategorized'), month
			ORDER BY month, label
		`
		args = []any{tz, startUTC}
	case "categories":
		query = `
			SELECT
				COALESCE(NULLIF(t.category, ''), 'Uncategorized') AS label,
				TO_CHAR(t.timestamp AT TIME ZONE $1, 'YYYY-MM') AS month,
				COALESCE(SUM(t.amount), 0) AS amount
			FROM transactions t
			WHERE t.muted = false
			  AND t.timestamp >= $2
			GROUP BY COALESCE(NULLIF(t.category, ''), 'Uncategorized'), month
			ORDER BY month, label
		`
		args = []any{tz, startUTC}
	case "buckets":
		query = `
			SELECT
				COALESCE(NULLIF(t.bucket, ''), 'Uncategorized') AS label,
				TO_CHAR(t.timestamp AT TIME ZONE $1, 'YYYY-MM') AS month,
				COALESCE(SUM(t.amount), 0) AS amount
			FROM transactions t
			WHERE t.muted = false
			  AND t.timestamp >= $2
			GROUP BY COALESCE(NULLIF(t.bucket, ''), 'Uncategorized'), month
			ORDER BY month, label
		`
		args = []any{tz, startUTC}
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

	series := make([]MonthlyBreakdownSeries, 0, len(labels))
	for _, label := range labels {
		values := make([]float64, len(monthLabels))
		for i, month := range monthLabels {
			values[i] = lookup[label][month]
		}
		series = append(series, MonthlyBreakdownSeries{Label: label, Data: values})
	}

	return &MonthlyBreakdownData{
		Labels: labels,
		Months: monthLabels,
		Series: series,
	}, nil
}

func (r *pgReadModelRepository) appTimezone(ctx context.Context) string {
	const fallback = "UTC"

	tz, err := r.runtime.GetAppConfig(ctx, "app.timezone")
	if err != nil || tz == "" {
		return fallback
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return fallback
	}
	return tz
}

func (r *pgReadModelRepository) dashboardBaseCurrency(ctx context.Context) string {
	const fallback = "INR"

	baseCurrency, err := r.runtime.GetAppConfig(ctx, "base_currency")
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

func (r *pgReadModelRepository) dashboardMonthBounds(ctx context.Context, now time.Time) dashboardMonthWindow {
	tz := r.appTimezone(ctx)
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
