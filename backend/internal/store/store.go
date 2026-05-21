// Package store provides database query operations for the Expensor API.
// It is separate from the writer plugin's pool — the store is used exclusively
// by HTTP handlers for reads and user-initiated writes (descriptions, labels).
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
)

// Transaction represents a single expense transaction as returned by the API.
type Transaction struct {
	ID               string     `json:"id"`
	MessageID        string     `json:"message_id"`
	Amount           float64    `json:"amount"`
	Currency         string     `json:"currency"`
	OriginalAmount   *float64   `json:"original_amount,omitempty"`
	OriginalCurrency *string    `json:"original_currency,omitempty"`
	ExchangeRate     *float64   `json:"exchange_rate,omitempty"`
	Timestamp        time.Time  `json:"timestamp"`
	MerchantInfo     string     `json:"merchant_info"`
	Category         string     `json:"category"`
	Bucket           string     `json:"bucket"`
	Source           api.Source `json:"source"`
	Description      string     `json:"description"`
	Labels           []string   `json:"labels"`
	Muted            bool       `json:"muted"`
	MutedByMerchant  bool       `json:"muted_by_merchant"`
	MuteReason       string     `json:"mute_reason,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// MutedMerchant holds a merchant pattern that auto-mutes matching transactions at write time.
type MutedMerchant struct {
	ID        string    `json:"id"`
	Pattern   string    `json:"pattern"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// MutedMerchantWithCount is a MutedMerchant with the count of currently muted transactions.
type MutedMerchantWithCount struct {
	MutedMerchant
	MutedCount int `json:"muted_count"`
}

// Stats holds aggregate statistics about stored transactions.
type Stats struct {
	TotalCount         int                `json:"total_count"`
	TotalBase          float64            `json:"total_base"`
	BaseCurrency       string             `json:"base_currency"`
	TotalByCategory    map[string]float64 `json:"total_by_category"`
	TotalCategoryCount map[string]int     `json:"total_category_count"`
}

// CategoryMonthlyEntry holds spend totals for a category for the current and prior calendar month.
type CategoryMonthlyEntry struct {
	Current float64 `json:"current"`
	Prior   float64 `json:"prior"`
}

// TimeBucket is a single time-period data point used by chart queries.
type TimeBucket struct {
	Period string  `json:"period"` // "2024-01" for monthly, "2024-01-15" for daily
	Amount float64 `json:"amount"`
	Count  int     `json:"count"`
}

// ChartData holds all time-series and breakdown data for the dashboard charts.
type ChartData struct {
	MonthlySpend      []TimeBucket                    `json:"monthly_spend"`
	DailySpend        []TimeBucket                    `json:"daily_spend"`
	ByCategory        map[string]float64              `json:"by_category"`
	ByBucket          map[string]float64              `json:"by_bucket"`
	ByLabel           map[string]float64              `json:"by_label"`
	BySource          map[string]float64              `json:"by_source"`
	ByCategoryMonthly map[string]CategoryMonthlyEntry `json:"by_category_monthly"`
}

// DashboardSection is one dashboard slice with a label, summary stats, and charts.
type DashboardSection struct {
	Label  string    `json:"label"`
	Stats  Stats     `json:"stats"`
	Charts ChartData `json:"charts"`
}

// DashboardData separates current-month and all-time dashboard data.
type DashboardData struct {
	CurrentMonth DashboardSection `json:"current_month"`
	AllTime      DashboardSection `json:"all_time"`
}

// MonthlyBreakdownSeries is a named 12-month spend series used by the dashboard line chart.
type MonthlyBreakdownSeries struct {
	Label string    `json:"label"`
	Data  []float64 `json:"data"`
}

// MonthlyBreakdownData is the line-chart payload for labels, categories, or buckets.
type MonthlyBreakdownData struct {
	Labels []string                 `json:"labels"`
	Months []string                 `json:"months"`
	Series []MonthlyBreakdownSeries `json:"series"`
}

// WeekdayHourBucket holds transaction totals for a (weekday, hour) cell.
// Weekday follows PostgreSQL DOW convention: 0=Sunday … 6=Saturday.
type WeekdayHourBucket struct {
	Weekday int     `json:"weekday"` // 0–6 (0=Sunday)
	Hour    int     `json:"hour"`    // 0–23
	Amount  float64 `json:"amount"`
	Count   int     `json:"count"`
}

// DayOfMonthBucket holds transaction totals for a single calendar day (1–31).
type DayOfMonthBucket struct {
	Day    int     `json:"day"` // 1–31
	Amount float64 `json:"amount"`
	Count  int     `json:"count"`
}

// HeatmapData contains both heatmap datasets returned by GetSpendingHeatmap.
type HeatmapData struct {
	ByWeekdayHour []WeekdayHourBucket `json:"by_weekday_hour"`
	ByDayOfMonth  []DayOfMonthBucket  `json:"by_day_of_month"`
}

// DailyBucket holds transaction totals for a single calendar date.
type DailyBucket struct {
	Date   time.Time `json:"date"`
	Amount float64   `json:"amount"`
	Count  int       `json:"count"`
}

// Label is a managed label in the taxonomy.
type Label struct {
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

// Category is a managed transaction category.
type Category struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsDefault   bool   `json:"is_default"`
}

// Bucket is a managed spend bucket (needs / wants / investments / income).
type Bucket struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsDefault   bool   `json:"is_default"`
}

// MCCEntry represents a single MCC code record from content/mcc.json.
type MCCEntry struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Bucket      string `json:"bucket"`
}

// MerchantCategoryEntry represents a single fragment mapping from content/categories.json.
type MerchantCategoryEntry struct {
	Fragment string  `json:"fragment"`
	MCC      *string `json:"mcc,omitempty"`
	Category *string `json:"category,omitempty"`
	Bucket   *string `json:"bucket,omitempty"`
}

// SyncStatus holds the result of the last community content sync.
type SyncStatus struct {
	LastSyncedAt   *time.Time `json:"last_synced_at"`
	Error          *string    `json:"error"`
	EntriesUpdated int        `json:"entries_updated"`
}

// RuleRow is a rule as stored in the database.
// Source is either "system" (seeded from embedded rules.json) or "user" (created via UI).
// TransactionSource is the human-readable identifier written to transaction.source (e.g. "Credit Card - HDFC").
type RuleRow struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	SenderEmail       string    `json:"sender_email"`
	SenderEmails      []string  `json:"sender_emails"`
	SubjectContains   string    `json:"subject_contains"`
	AmountRegex       string    `json:"amount_regex"`
	MerchantRegex     string    `json:"merchant_regex"`
	CurrencyRegex     string    `json:"currency_regex"`
	TransactionSource string    `json:"transaction_source"`
	SourceType        string    `json:"source_type"`
	SourceLabel       string    `json:"source_label"`
	Bank              string    `json:"bank"`
	Predefined        bool      `json:"predefined"` // true = seeded from embedded rules.json; editable but not deletable
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

const (
	DiagnosticStatusOpen     = "open"
	DiagnosticStatusResolved = "resolved"
	DiagnosticStatusIgnored  = "ignored"
	DiagnosticStatusAll      = "all"
)

// ExtractionDiagnosticRow is a persisted extraction diagnostic.
type ExtractionDiagnosticRow struct {
	ID             string     `json:"id"`
	Status         string     `json:"status"`
	Reader         string     `json:"reader"`
	MessageID      string     `json:"message_id"`
	Source         string     `json:"source"`
	Sender         string     `json:"sender"`
	SenderEmail    string     `json:"sender_email"`
	Subject        string     `json:"subject"`
	EmailBody      string     `json:"email_body"`
	ReceivedAt     *time.Time `json:"received_at,omitempty"`
	Snippet        string     `json:"snippet"`
	RuleID         *string    `json:"rule_id,omitempty"`
	RuleName       string     `json:"rule_name"`
	AmountRegex    string     `json:"amount_regex"`
	MerchantRegex  string     `json:"merchant_regex"`
	CurrencyRegex  string     `json:"currency_regex"`
	FailureReasons []string   `json:"failure_reasons"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

// DiagnosticFilter controls filtering for extraction diagnostic listings.
type DiagnosticFilter struct {
	Status string
	Limit  int
}

// TransactionUpdate carries optional fields for updating a transaction.
// Only non-nil fields are written.
type TransactionUpdate struct {
	Description *string
	Category    *string
	Bucket      *string
}

// TransactionListResult captures aggregate metadata for a filtered transaction query.
type TransactionListResult struct {
	Total       int     `json:"total"`
	TotalAmount float64 `json:"total_amount"`
}

// ListFilter controls pagination and filtering for ListTransactions.
type ListFilter struct {
	Page               int    // 1-based
	PageSize           int    // max rows per page
	Category           string // partial match (ILIKE), empty = all
	CategoryMissing    bool   // true = category is NULL or empty
	ExcludeCategories  []string
	Currency           string // partial match (ILIKE), empty = all
	Source             string // partial match (ILIKE), empty = all
	ExcludeSources     []string
	SourceType         string
	ExcludeSourceTypes []string
	Bank               string
	ExcludeBanks       []string
	Bucket             string // partial match, empty = all
	BucketMissing      bool   // true = bucket is NULL or empty
	ExcludeBuckets     []string
	Label              string // filter by label, empty = all
	LabelMissing       bool   // true = no labels assigned
	ExcludeLabels      []string
	Merchant           string // partial match (ILIKE) on merchant_info, empty = all
	ShowMuted          bool   // when true, muted transactions are included; default hides them
	MutedOnly          bool   // when true, only muted=true (for click-through from Muted page)
	IndividualOnly     bool   // when true, only muted=true AND muted_by_merchant=false (per-tx mutes)
	Weekday            *int   // nil = all weekdays; uses configured timezone and PostgreSQL DOW convention
	HourFrom           *int   // nil = all hours; non-nil filters EXTRACT(HOUR FROM timestamp) >= *HourFrom
	HourTo             *int   // nil = all hours; non-nil filters EXTRACT(HOUR FROM timestamp) <= *HourTo
	Timezone           string // IANA timezone for hour extraction; defaults to UTC when empty
	From               *time.Time
	To                 *time.Time
	SortBy             string // "timestamp" (only supported value for now); default = "timestamp"
	SortDir            string // "asc" | "desc"; default = "desc"
}

// Store wraps a pgxpool.Pool and provides query operations for the API layer.
type Store struct {
	pool      *pgxpool.Pool
	logger    *slog.Logger
	now       func() time.Time
	community CommunityRepository
	diag      DiagnosticsRepository
	readModel ReadModelRepository
	rules     RulesRepository
	runtime   RuntimeRepository
	taxonomy  TaxonomyRepository
	txns      TransactionsRepository
}

var _ api.DiagnosticSink = (*Store)(nil)

// New creates a Store connected to the PostgreSQL instance described by cfg.
func New(cfg config.PostgresConfig, logger *slog.Logger) (*Store, error) {
	if logger == nil {
		logger = slog.Default()
	}

	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	poolCfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parsing store connection string: %w", err)
	}

	maxConns := min(cfg.MaxPoolSize, math.MaxInt32)
	poolCfg.MaxConns = int32(maxConns) //nolint:gosec // G115: bounded by min
	poolCfg.MinConns = 1
	poolCfg.MaxConnLifetime = 1 * time.Hour
	poolCfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("creating store pool: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging store database: %w", err)
	}

	s := &Store{pool: pool, logger: logger, now: time.Now}
	s.initRepositories()
	if err := s.initAppConfig(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("initializing app_config table: %w", err)
	}
	if err := s.initLabels(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("initializing labels table: %w", err)
	}
	if err := s.initCategoriesBuckets(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("initializing categories/buckets tables: %w", err)
	}
	if err := s.initRules(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("initializing rules table: %w", err)
	}
	logger.Info("store connected to PostgreSQL", "host", cfg.Host, "database", cfg.Database)
	return s, nil
}

func (s *Store) initRepositories() {
	deps := repositoryDependencies{
		pool:    s.pool,
		logger:  s.logger,
		metrics: NewQueryInstrumentation(s.logger),
		now:     s.now,
	}
	s.community = NewCommunityRepository(deps)
	s.diag = NewDiagnosticsRepository(deps)
	s.readModel = NewReadModelRepository(deps, s)
	s.rules = NewRulesRepository(deps)
	s.runtime = NewRuntimeRepository(deps)
	s.taxonomy = NewTaxonomyRepository(deps)
	s.txns = NewTransactionsRepository(deps)
}

// Close releases the store's connection pool.
func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) queryTransactionTotals(
	ctx context.Context,
	join string,
	where string,
	args []any,
) (TransactionListResult, error) {
	repo, ok := s.txns.(*pgTransactionsRepository)
	if !ok {
		return TransactionListResult{}, errors.New("transactions repository unavailable")
	}
	return repo.queryTransactionTotals(ctx, join, where, args)
}

// ListTransactions returns a paginated, filtered list of transactions and the total
// count plus total amount matching the filter (ignoring pagination).
func (s *Store) ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, TransactionListResult, error) {
	return s.txns.ListTransactions(ctx, f)
}

// GetTransaction fetches a single transaction by UUID, including its labels.
func (s *Store) GetTransaction(ctx context.Context, id string) (*Transaction, error) {
	return s.txns.GetTransaction(ctx, id)
}

// UpdateDescription sets the user-provided description on a transaction.
func (s *Store) UpdateDescription(ctx context.Context, id, description string) error {
	return s.txns.UpdateDescription(ctx, id, description)
}

// AddLabel attaches a label to a transaction (idempotent — ignores duplicates).
func (s *Store) AddLabel(ctx context.Context, transactionID, label string) error {
	return s.txns.AddLabel(ctx, transactionID, label)
}

// AddLabels attaches multiple labels to a transaction in a single round-trip (idempotent).
func (s *Store) AddLabels(ctx context.Context, transactionID string, labels []string) error {
	return s.txns.AddLabels(ctx, transactionID, labels)
}

// RemoveLabel detaches a label from a transaction.
func (s *Store) RemoveLabel(ctx context.Context, transactionID, label string) error {
	return s.txns.RemoveLabel(ctx, transactionID, label)
}

// SearchTransactions performs a full-text search over merchant_info and description.
func (s *Store) SearchTransactions(
	ctx context.Context,
	query string,
	f ListFilter,
) ([]Transaction, TransactionListResult, error) {
	return s.txns.SearchTransactions(ctx, query, f)
}

// GetStats returns aggregate counts and totals across all transactions.
func (s *Store) GetStats(ctx context.Context, baseCurrency string) (*Stats, error) {
	return s.readModel.GetStats(ctx, baseCurrency)
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
	if err := r.legacy.pool.QueryRow(ctx, mainQ, baseCurrency).Scan(&st.TotalCount, &st.TotalBase); err != nil {
		return nil, fmt.Errorf("fetching stats: %w", err)
	}

	const catQ = `
		SELECT COALESCE(NULLIF(category, ''), 'Uncategorized'), COALESCE(SUM(amount), 0), COUNT(*)
		FROM transactions
		WHERE muted = false
		GROUP BY COALESCE(NULLIF(category, ''), 'Uncategorized')
		ORDER BY SUM(amount) DESC
	`
	rows, err := r.legacy.pool.Query(ctx, catQ)
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

// GetChartData returns time-series and breakdown data for dashboard charts.
// All 7 queries run concurrently.
func (s *Store) GetChartData(ctx context.Context) (*ChartData, error) {
	return s.readModel.GetChartData(ctx)
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
	wg.Add(7)

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

	go func() {
		defer wg.Done()
		m := make(map[string]float64)
		if err := r.queryStringFloat(ctx, `
			SELECT COALESCE(NULLIF(category, ''), 'Uncategorized'), COALESCE(SUM(amount), 0)
			FROM transactions
			WHERE muted = false
			GROUP BY COALESCE(NULLIF(category, ''), 'Uncategorized')
			ORDER BY SUM(amount) DESC
		`, m); err != nil {
			recordErr(fmt.Errorf("fetching category chart data: %w", err))
			return
		}
		mu.Lock()
		cd.ByCategory = m
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		m := make(map[string]float64)
		if err := r.queryStringFloat(ctx, `
			SELECT COALESCE(NULLIF(bucket, ''), 'Uncategorized'), COALESCE(SUM(amount), 0)
			FROM transactions
			WHERE muted = false
			GROUP BY COALESCE(NULLIF(bucket, ''), 'Uncategorized')
			ORDER BY SUM(amount) DESC
		`, m); err != nil {
			recordErr(fmt.Errorf("fetching bucket chart data: %w", err))
			return
		}
		mu.Lock()
		cd.ByBucket = m
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		m := make(map[string]float64)
		if err := r.queryStringFloat(ctx, `
			SELECT COALESCE(tl.label, 'Uncategorized'), COALESCE(SUM(t.amount), 0)
			FROM transactions t
			LEFT JOIN transaction_labels tl ON tl.transaction_id = t.id
			WHERE t.muted = false
			GROUP BY COALESCE(tl.label, 'Uncategorized')
			ORDER BY SUM(t.amount) DESC
			LIMIT 20
		`, m); err != nil {
			recordErr(fmt.Errorf("fetching label chart data: %w", err))
			return
		}
		mu.Lock()
		cd.ByLabel = m
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		m := make(map[string]float64)
		if err := r.queryStringFloat(ctx, `
			SELECT COALESCE(source, ''), COALESCE(SUM(amount), 0)
			FROM transactions
			WHERE muted = false AND source IS NOT NULL AND source != ''
			GROUP BY source
			ORDER BY SUM(amount) DESC
		`, m); err != nil {
			recordErr(fmt.Errorf("fetching source chart data: %w", err))
			return
		}
		mu.Lock()
		cd.BySource = m
		mu.Unlock()
	}()

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

// GetDashboardData returns dashboard data split into current-month and all-time sections.
func (s *Store) GetDashboardData(ctx context.Context) (*DashboardData, error) {
	return s.readModel.GetDashboardData(ctx)
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
	if err := r.legacy.pool.QueryRow(ctx, mainQ, baseCurrency, startUTC, endUTC).Scan(&st.TotalCount, &st.TotalBase); err != nil {
		return nil, fmt.Errorf("fetching range stats: %w", err)
	}

	const catQ = `
		SELECT COALESCE(NULLIF(category, ''), 'Uncategorized'), COALESCE(SUM(amount), 0), COUNT(*)
		FROM transactions
		WHERE muted = false AND timestamp >= $1 AND timestamp < $2
		GROUP BY COALESCE(NULLIF(category, ''), 'Uncategorized')
		ORDER BY SUM(amount) DESC
	`
	rows, err := r.legacy.pool.Query(ctx, catQ, startUTC, endUTC)
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
	rows, err := r.legacy.pool.Query(ctx, q, startUTC, endUTC, priorStartUTC)
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
	if r != nil && r.legacy != nil && r.legacy.now != nil {
		return r.legacy.now()
	}
	return time.Now()
}

// GetSpendingHeatmap returns transaction totals aggregated by weekday×hour and
// by day-of-month. When from and to are both non-nil, only transactions within
// [from, to] (inclusive) are included; nil/nil returns all-time data.
func (s *Store) GetSpendingHeatmap(ctx context.Context, from, to *time.Time) (*HeatmapData, error) {
	return s.readModel.GetSpendingHeatmap(ctx, from, to)
}

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
	wdhRows, err := r.legacy.pool.Query(ctx, wdhQuery, argsWithTZ...)
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
	domRows, err := r.legacy.pool.Query(ctx, domQuery, argsWithTZ...)
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
// the year has no transactions.
func (s *Store) GetAnnualSpend(ctx context.Context, year int) ([]DailyBucket, error) {
	return s.readModel.GetAnnualSpend(ctx, year)
}

func (r *pgReadModelRepository) annualSpendReadModel(ctx context.Context, year int) ([]DailyBucket, error) {
	buckets := []DailyBucket{}

	rows, err := r.legacy.pool.Query(ctx, `
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
// across all transactions. Used to populate filter dropdowns in the UI.
func (s *Store) GetFacets(ctx context.Context) (*Facets, error) {
	return s.txns.GetFacets(ctx)
}

func (r *pgReadModelRepository) queryTimeBuckets(ctx context.Context, q string, args ...any) ([]TimeBucket, error) {
	rows, err := r.legacy.pool.Query(ctx, q, args...)
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
	rows, err := r.legacy.pool.Query(ctx, q, args...)
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

// initAppConfig creates the app_config table and seeds operational defaults.
// It is called once from New and is idempotent.
func (s *Store) initAppConfig(ctx context.Context) error {
	return s.runtime.InitAppConfig(ctx)
}

// GetAppConfig retrieves a configuration value by key.
// Returns an error if the key does not exist.
func (s *Store) GetAppConfig(ctx context.Context, key string) (string, error) {
	return s.runtime.GetAppConfig(ctx, key)
}

// SetAppConfig upserts a configuration value.
func (s *Store) SetAppConfig(ctx context.Context, key, value string) error {
	return s.runtime.SetAppConfig(ctx, key, value)
}

// SetActiveReader stores the selected reader name.
func (s *Store) SetActiveReader(ctx context.Context, reader string) error {
	return s.runtime.SetActiveReader(ctx, reader)
}

// GetActiveReader returns the selected reader name, or an empty string when unset.
func (s *Store) GetActiveReader(ctx context.Context) (string, error) {
	return s.runtime.GetActiveReader(ctx)
}

// SetReaderSecret stores OAuth client secret JSON for a reader.
func (s *Store) SetReaderSecret(ctx context.Context, reader string, secret []byte) error {
	return s.runtime.SetReaderSecret(ctx, reader, secret)
}

// GetReaderSecret returns OAuth client secret JSON for a reader.
func (s *Store) GetReaderSecret(ctx context.Context, reader string) (secret []byte, found bool, err error) {
	return s.runtime.GetReaderSecret(ctx, reader)
}

// SetReaderToken stores OAuth token JSON for a reader.
func (s *Store) SetReaderToken(ctx context.Context, reader string, token []byte) error {
	return s.runtime.SetReaderToken(ctx, reader, token)
}

// GetReaderToken returns OAuth token JSON for a reader.
func (s *Store) GetReaderToken(ctx context.Context, reader string) (token []byte, found bool, err error) {
	return s.runtime.GetReaderToken(ctx, reader)
}

// DeleteReaderToken removes the OAuth token JSON for a reader without deleting other reader runtime data.
func (s *Store) DeleteReaderToken(ctx context.Context, reader string) error {
	return s.runtime.DeleteReaderToken(ctx, reader)
}

// SetReaderConfig stores reader-specific configuration JSON.
func (s *Store) SetReaderConfig(ctx context.Context, reader string, readerConfig json.RawMessage) error {
	return s.runtime.SetReaderConfig(ctx, reader, readerConfig)
}

// GetReaderConfig returns reader-specific configuration JSON.
func (s *Store) GetReaderConfig(ctx context.Context, reader string) (json.RawMessage, bool, error) {
	return s.runtime.GetReaderConfig(ctx, reader)
}

// DeleteReaderRuntime removes all runtime data for a reader.
func (s *Store) DeleteReaderRuntime(ctx context.Context, reader string) error {
	return s.runtime.DeleteReaderRuntime(ctx, reader)
}

// IsMessageProcessed reports whether a message key has already been processed.
func (s *Store) IsMessageProcessed(ctx context.Context, key string) (bool, error) {
	return s.runtime.IsMessageProcessed(ctx, key)
}

// MarkMessageProcessed records a processed message key at the supplied time.
func (s *Store) MarkMessageProcessed(ctx context.Context, key string, at time.Time) error {
	return s.runtime.MarkMessageProcessed(ctx, key, at)
}

// initLabels creates the labels table and supporting label automation tables. Idempotent.
func (s *Store) initLabels(ctx context.Context) error {
	return s.taxonomy.InitLabels(ctx)
}

// initCategoriesBuckets creates the categories and buckets tables and seeds defaults. Idempotent.
func (s *Store) initCategoriesBuckets(ctx context.Context) error {
	return s.taxonomy.InitCategoriesBuckets(ctx)
}

// initRules creates the rules table with the current schema. Idempotent.
// For databases upgraded from an older schema, the numbered migration
// The initial migration and this initializer both preserve idempotent upgrade guards.
func (s *Store) initRules(ctx context.Context) error {
	return s.rules.InitRules(ctx)
}

// --- Labels ---

// ListLabels returns all labels ordered by name.
func (s *Store) ListLabels(ctx context.Context) ([]Label, error) {
	return s.taxonomy.ListLabels(ctx)
}

// CreateLabel inserts a new label. Silently ignores duplicate names.
func (s *Store) CreateLabel(ctx context.Context, name, color string) error {
	return s.taxonomy.CreateLabel(ctx, name, color)
}

// UpdateLabel changes the color of an existing label. Returns ErrNotFound if no row matched.
func (s *Store) UpdateLabel(ctx context.Context, name, color string) error {
	return s.taxonomy.UpdateLabel(ctx, name, color)
}

// DeleteLabel removes a label by name.
func (s *Store) DeleteLabel(ctx context.Context, name string, removeFromTransactions bool) error {
	return s.taxonomy.DeleteLabel(ctx, name, removeFromTransactions)
}

// ApplyLabelByMerchant bulk-applies a label to all transactions whose
// merchant_info matches the given pattern (case-insensitive contains), and
// persists the mapping for future auto-apply.
// Returns the number of transaction-label rows inserted.
func (s *Store) ApplyLabelByMerchant(ctx context.Context, label, pattern string) (int64, error) {
	return s.taxonomy.ApplyLabelByMerchant(ctx, label, pattern)
}

// RemoveLabelByMerchant removes a label from all transactions whose
// merchant_info matches the pattern (case-insensitive contains), and removes
// the persisted merchant mapping.
func (s *Store) RemoveLabelByMerchant(ctx context.Context, label, pattern string) (int64, error) {
	return s.taxonomy.RemoveLabelByMerchant(ctx, label, pattern)
}

// GetMonthlyBreakdownSpend returns a 12-month spend series for labels, categories, or buckets.
// Muted transactions are excluded. Months are emitted in the configured app timezone.
func (s *Store) GetMonthlyBreakdownSpend(ctx context.Context, dimension string, months int) (*MonthlyBreakdownData, error) {
	return s.readModel.GetMonthlyBreakdownSpend(ctx, dimension, months)
}

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

	rows, err := r.legacy.pool.Query(ctx, query, args...)
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

// GetLabelMappings returns persisted merchant patterns for each label.
func (s *Store) GetLabelMappings(ctx context.Context) (map[string][]string, error) {
	return s.taxonomy.GetLabelMappings(ctx)
}

// --- Categories ---

// ListCategories returns all categories ordered by name.
func (s *Store) ListCategories(ctx context.Context) ([]Category, error) {
	return s.taxonomy.ListCategories(ctx)
}

// CreateCategory inserts a new category. Silently ignores duplicate names.
func (s *Store) CreateCategory(ctx context.Context, name, description string) error {
	return s.taxonomy.CreateCategory(ctx, name, description)
}

// DeleteCategory removes a category by name. Returns ErrNotFound if it does not exist.
// Returns an error if the category is a default one.
func (s *Store) DeleteCategory(ctx context.Context, name string, removeFromTransactions bool) error {
	return s.taxonomy.DeleteCategory(ctx, name, removeFromTransactions)
}

// GetCategoryMappings returns persisted merchant patterns for each category.
func (s *Store) GetCategoryMappings(ctx context.Context) (map[string][]string, error) {
	return s.community.GetCategoryMappings(ctx)
}

// ApplyCategoryByMerchant updates matching transactions and future category auto-apply rules.
func (s *Store) ApplyCategoryByMerchant(ctx context.Context, category, pattern string) (int64, error) {
	return s.community.ApplyCategoryByMerchant(ctx, category, pattern)
}

// RemoveCategoryByMerchant removes a merchant category auto-apply rule.
func (s *Store) RemoveCategoryByMerchant(ctx context.Context, category, pattern string) (int64, error) {
	return s.community.RemoveCategoryByMerchant(ctx, category, pattern)
}

// --- Buckets ---

// ListBuckets returns all buckets ordered by name.
func (s *Store) ListBuckets(ctx context.Context) ([]Bucket, error) {
	return s.taxonomy.ListBuckets(ctx)
}

// CreateBucket inserts a new bucket. Silently ignores duplicate names.
func (s *Store) CreateBucket(ctx context.Context, name, description string) error {
	return s.taxonomy.CreateBucket(ctx, name, description)
}

// DeleteBucket removes a bucket by name. Returns ErrNotFound if it does not exist.
// Returns an error if the bucket is a default one.
func (s *Store) DeleteBucket(ctx context.Context, name string, removeFromTransactions bool) error {
	return s.taxonomy.DeleteBucket(ctx, name, removeFromTransactions)
}

// GetBucketMappings returns persisted merchant patterns for each bucket.
func (s *Store) GetBucketMappings(ctx context.Context) (map[string][]string, error) {
	return s.community.GetBucketMappings(ctx)
}

// ApplyBucketByMerchant updates matching transactions and future bucket auto-apply rules.
func (s *Store) ApplyBucketByMerchant(ctx context.Context, bucket, pattern string) (int64, error) {
	return s.community.ApplyBucketByMerchant(ctx, bucket, pattern)
}

// RemoveBucketByMerchant removes a merchant bucket auto-apply rule.
func (s *Store) RemoveBucketByMerchant(ctx context.Context, bucket, pattern string) (int64, error) {
	return s.community.RemoveBucketByMerchant(ctx, bucket, pattern)
}

// --- Transaction update ---

// UpdateTransaction updates one or more optional fields on a transaction.
// Only non-nil pointer fields are written. Returns ErrNotFound if no row matched.
func (s *Store) UpdateTransaction(ctx context.Context, id string, u TransactionUpdate) error {
	return s.txns.UpdateTransaction(ctx, id, u)
}

// ErrNotFound is returned when an operation targets a row that does not exist.
var ErrNotFound = errors.New("not found")

// ErrDiagnosticConflict is returned when reopening a diagnostic would duplicate an existing open diagnostic.
var ErrDiagnosticConflict = errors.New("diagnostic conflict")

// --- helpers ---

const diagnosticColumns = `
	id::text, status, reader, COALESCE(message_id, ''), source, sender, sender_email, subject, email_body,
	received_at, snippet, rule_id::text, rule_name, amount_regex, merchant_regex, currency_regex, failure_reasons,
	created_at, updated_at, resolved_at
`

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func isDiagnosticOpenConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "extraction_diagnostics_open_unique"
}

func diagnosticFailureReasons(reasons []string) []string {
	if reasons == nil {
		return []string{}
	}
	return reasons
}

func diagnosticSender(diagnostic api.ExtractionDiagnostic) string {
	if diagnostic.Sender != "" {
		return diagnostic.Sender
	}
	return diagnostic.SenderEmail
}

func scanDiagnosticRows(rows pgx.Rows) ([]ExtractionDiagnosticRow, error) {
	var result []ExtractionDiagnosticRow
	for rows.Next() {
		var row ExtractionDiagnosticRow
		var ruleID pgtype.Text
		var receivedAt pgtype.Timestamptz
		var resolvedAt pgtype.Timestamptz
		if err := rows.Scan(
			&row.ID,
			&row.Status,
			&row.Reader,
			&row.MessageID,
			&row.Source,
			&row.Sender,
			&row.SenderEmail,
			&row.Subject,
			&row.EmailBody,
			&receivedAt,
			&row.Snippet,
			&ruleID,
			&row.RuleName,
			&row.AmountRegex,
			&row.MerchantRegex,
			&row.CurrencyRegex,
			&row.FailureReasons,
			&row.CreatedAt,
			&row.UpdatedAt,
			&resolvedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning extraction diagnostic: %w", err)
		}
		if ruleID.Valid {
			row.RuleID = &ruleID.String
		}
		if receivedAt.Valid {
			received := receivedAt.Time
			row.ReceivedAt = &received
		}
		if resolvedAt.Valid {
			resolved := resolvedAt.Time
			row.ResolvedAt = &resolved
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if result == nil {
		result = []ExtractionDiagnosticRow{}
	}
	return result, nil
}

func joinLabel(label string) string {
	if label == "" {
		return ""
	}
	return " JOIN transaction_labels tl ON tl.transaction_id = t.id"
}

// buildListWhere builds the WHERE clause and argument list for ListTransactions / SearchTransactions.
// args is grown in-place; the first placeholder index is len(existingArgs)+1.
func buildListWhere(f ListFilter) (string, []any) {
	var conds []string
	var args []any

	next := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	switch {
	case f.IndividualOnly:
		conds = append(conds, "t.muted = true AND t.muted_by_merchant = false")
	case f.MutedOnly:
		conds = append(conds, "t.muted = true")
	case !f.ShowMuted:
		conds = append(conds, "t.muted = false")
	}
	if f.Label != "" {
		conds = append(conds, fmt.Sprintf("tl.label ILIKE %s", next("%"+f.Label+"%")))
	}
	if f.Merchant != "" {
		conds = append(conds, fmt.Sprintf("t.merchant_info ILIKE %s", next("%"+f.Merchant+"%")))
	}
	conds = appendTaxonomyListWhere(conds, f, next)
	if f.Currency != "" {
		conds = append(conds, fmt.Sprintf("t.currency ILIKE %s", next("%"+f.Currency+"%")))
	}
	if f.Source != "" {
		conds = append(conds, fmt.Sprintf("t.source ILIKE %s", next("%"+f.Source+"%")))
	}
	if len(f.ExcludeSources) > 0 {
		conds = append(conds, "COALESCE(t.source, '') != ''")
		conds = append(conds, fmt.Sprintf("NOT (t.source = ANY(%s))", next(f.ExcludeSources)))
	}
	if f.SourceType != "" {
		conds = append(conds, fmt.Sprintf("t.source_type ILIKE %s", next("%"+f.SourceType+"%")))
	}
	if len(f.ExcludeSourceTypes) > 0 {
		conds = append(conds, "COALESCE(t.source_type, '') != ''")
		conds = append(conds, fmt.Sprintf("NOT (t.source_type = ANY(%s))", next(f.ExcludeSourceTypes)))
	}
	if f.Bank != "" {
		conds = append(conds, fmt.Sprintf("t.bank ILIKE %s", next("%"+f.Bank+"%")))
	}
	if len(f.ExcludeBanks) > 0 {
		conds = append(conds, "COALESCE(t.bank, '') != ''")
		conds = append(conds, fmt.Sprintf("NOT (t.bank = ANY(%s))", next(f.ExcludeBanks)))
	}
	if f.From != nil {
		conds = append(conds, fmt.Sprintf("t.timestamp >= %s", next(*f.From)))
	}
	if f.To != nil {
		conds = append(conds, fmt.Sprintf("t.timestamp <= %s", next(*f.To)))
	}
	tz := f.Timezone
	if tz == "" {
		tz = "UTC"
	}
	localTimestampExpr := func() string {
		return fmt.Sprintf("t.timestamp AT TIME ZONE %s", next(tz))
	}
	if f.Weekday != nil {
		conds = append(conds, fmt.Sprintf(
			"EXTRACT(DOW FROM %s)::int = %s",
			localTimestampExpr(), next(*f.Weekday)))
	}
	if f.HourFrom != nil {
		conds = append(conds, fmt.Sprintf(
			"EXTRACT(HOUR FROM %s)::int >= %s",
			localTimestampExpr(), next(*f.HourFrom)))
	}
	if f.HourTo != nil {
		conds = append(conds, fmt.Sprintf(
			"EXTRACT(HOUR FROM %s)::int <= %s",
			localTimestampExpr(), next(*f.HourTo)))
	}

	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func appendTaxonomyListWhere(conds []string, f ListFilter, next func(any) string) []string {
	if f.Category != "" {
		conds = append(conds, fmt.Sprintf("t.category ILIKE %s", next("%"+f.Category+"%")))
	}
	if f.CategoryMissing {
		conds = append(conds, "COALESCE(t.category, '') = ''")
	}
	if len(f.ExcludeCategories) > 0 {
		conds = append(conds, "COALESCE(t.category, '') != ''")
		conds = append(conds, fmt.Sprintf("NOT (t.category = ANY(%s))", next(f.ExcludeCategories)))
	}
	if f.Bucket != "" {
		conds = append(conds, fmt.Sprintf("t.bucket ILIKE %s", next("%"+f.Bucket+"%")))
	}
	if f.BucketMissing {
		conds = append(conds, "COALESCE(t.bucket, '') = ''")
	}
	if len(f.ExcludeBuckets) > 0 {
		conds = append(conds, "COALESCE(t.bucket, '') != ''")
		conds = append(conds, fmt.Sprintf("NOT (t.bucket = ANY(%s))", next(f.ExcludeBuckets)))
	}
	if len(f.ExcludeLabels) > 0 {
		conds = append(conds, fmt.Sprintf(
			`EXISTS (
				SELECT 1
				FROM transaction_labels tl_include
				WHERE tl_include.transaction_id = t.id
				  AND NOT (tl_include.label = ANY(%s))
			)`,
			next(f.ExcludeLabels),
		))
	}
	if f.LabelMissing {
		conds = append(conds, `NOT EXISTS (
			SELECT 1
			FROM transaction_labels tl_missing
			WHERE tl_missing.transaction_id = t.id
		)`)
	}
	return conds
}

func (r *pgReadModelRepository) appTimezone(ctx context.Context) string {
	const fallback = "UTC"

	tz, err := r.legacy.GetAppConfig(ctx, "app.timezone")
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

	baseCurrency, err := r.legacy.GetAppConfig(ctx, "base_currency")
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

// buildSearchCondition appends raw search text and returns a safe hybrid search condition.
func buildSearchCondition(query string, args *[]any) string {
	*args = append(*args, query)
	tsArg := len(*args)
	*args = append(*args, escapeLikePattern(query))
	likeArg := len(*args)
	return fmt.Sprintf(
		`(
			(to_tsvector('english', t.merchant_info) || to_tsvector('english', COALESCE(t.description,''))) @@ websearch_to_tsquery('english', $%d)
			OR t.merchant_info ILIKE $%d ESCAPE '\'
			OR COALESCE(t.description, '') ILIKE $%d ESCAPE '\'
		)`,
		tsArg,
		likeArg,
		likeArg,
	)
}

func escapeLikePattern(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(value) + "%"
}

// combineWhere merges a bare condition with an existing WHERE clause.
func combineWhere(cond, existing string) string {
	if existing == "" {
		return " WHERE " + cond
	}
	// existing already starts with " WHERE "
	return existing + " AND " + cond
}

func scanTransactions(rows pgx.Rows) ([]Transaction, error) {
	var txns []Transaction
	for rows.Next() {
		var t Transaction
		var legacySource, sourceType, sourceLabel, bank string
		if err := rows.Scan(
			&t.ID, &t.MessageID, &t.Amount, &t.Currency,
			&t.OriginalAmount, &t.OriginalCurrency, &t.ExchangeRate,
			&t.Timestamp, &t.MerchantInfo, &t.Category, &t.Bucket,
			&legacySource, &sourceType, &sourceLabel, &bank,
			&t.Description, &t.Muted, &t.MutedByMerchant, &t.MuteReason, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning transaction row: %w", err)
		}
		if sourceLabel == "" {
			sourceLabel = legacySource
		}
		t.Source = api.Source{Type: sourceType, Label: sourceLabel, Bank: bank}
		t.Labels = []string{}
		txns = append(txns, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating transaction rows: %w", err)
	}
	return txns, nil
}

// --- Rules ---

const ruleColumns = `id, name, sender_email, sender_emails, subject_contains, amount_regex, merchant_regex,
	currency_regex, transaction_source, source_type, source_label, bank, predefined, created_at, updated_at`

func scanRuleRows(rows pgx.Rows) ([]RuleRow, error) {
	var result []RuleRow
	for rows.Next() {
		var r RuleRow
		if err := rows.Scan(
			&r.ID, &r.Name, &r.SenderEmail, &r.SenderEmails, &r.SubjectContains,
			&r.AmountRegex, &r.MerchantRegex, &r.CurrencyRegex,
			&r.TransactionSource, &r.SourceType, &r.SourceLabel, &r.Bank, &r.Predefined,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning rule row: %w", err)
		}
		result = append(result, r)
	}
	if result == nil {
		result = []RuleRow{}
	}
	return result, rows.Err()
}

// ListRules returns all rules ordered by user rules first, then predefined rules, both by name.
func (s *Store) ListRules(ctx context.Context) ([]RuleRow, error) {
	return s.rules.ListRules(ctx)
}

// GetRule fetches a single rule by UUID. Returns ErrNotFound if no row matched.
func (s *Store) GetRule(ctx context.Context, id string) (*RuleRow, error) {
	return s.rules.GetRule(ctx, id)
}

// CreateRule inserts a new user rule and returns the created row.
func (s *Store) CreateRule(ctx context.Context, r RuleRow) (*RuleRow, error) {
	return s.rules.CreateRule(ctx, r)
}

// UpdateRule updates any rule by ID. All rules (predefined and user-created) are editable.
// Returns ErrNotFound if no row matched.
func (s *Store) UpdateRule(ctx context.Context, id string, r RuleRow) (*RuleRow, error) {
	return s.rules.UpdateRule(ctx, id, r)
}

// DeleteRule removes a non-predefined rule by ID. Returns ErrNotFound if no row matched.
// Predefined rules cannot be deleted.
func (s *Store) DeleteRule(ctx context.Context, id string) error {
	return s.rules.DeleteRule(ctx, id)
}

// SeedPredefinedRules inserts predefined rules from the embedded rules.json.
// Uses ON CONFLICT DO NOTHING so user edits to predefined rules are never overwritten.
func (s *Store) SeedPredefinedRules(ctx context.Context, rules []RuleRow) error {
	return s.rules.SeedPredefinedRules(ctx, rules)
}

// ValidateDiagnosticFilterStatus reports whether status is a supported diagnostic filter value.
func ValidateDiagnosticFilterStatus(status string) error {
	switch status {
	case DiagnosticStatusOpen, DiagnosticStatusResolved, DiagnosticStatusIgnored, DiagnosticStatusAll:
		return nil
	default:
		return fmt.Errorf("invalid diagnostic status %q", status)
	}
}

func validateDiagnosticRowStatus(status string) error {
	switch status {
	case DiagnosticStatusOpen, DiagnosticStatusResolved, DiagnosticStatusIgnored:
		return nil
	default:
		return fmt.Errorf("invalid diagnostic status %q", status)
	}
}

// RecordExtractionDiagnostic persists a failed extraction attempt for later inspection.
func (s *Store) RecordExtractionDiagnostic(ctx context.Context, diagnostic api.ExtractionDiagnostic) error {
	return s.diag.RecordExtractionDiagnostic(ctx, diagnostic)
}

// ListExtractionDiagnostics returns diagnostics matching the supplied status filter.
func (s *Store) ListExtractionDiagnostics(ctx context.Context, f DiagnosticFilter) ([]ExtractionDiagnosticRow, error) {
	return s.diag.ListExtractionDiagnostics(ctx, f)
}

// GetExtractionDiagnostic fetches one diagnostic by UUID.
func (s *Store) GetExtractionDiagnostic(ctx context.Context, id string) (*ExtractionDiagnosticRow, error) {
	return s.diag.GetExtractionDiagnostic(ctx, id)
}

// UpdateExtractionDiagnosticStatus changes a diagnostic status and returns the updated row.
func (s *Store) UpdateExtractionDiagnosticStatus(ctx context.Context, id, status string) (*ExtractionDiagnosticRow, error) {
	return s.diag.UpdateExtractionDiagnosticStatus(ctx, id, status)
}

// ImportUserRules upserts user-supplied rules inside a transaction. Idempotent per name.
func (s *Store) ImportUserRules(ctx context.Context, rules []RuleRow) error {
	return s.rules.ImportUserRules(ctx, rules)
}

// loadLabels fetches labels for all transactions in a single query and attaches them.
// --- Muted transactions ---

// MuteTransaction sets or clears the muted flag on a single transaction.
// reason is optional; pass empty string to leave it unchanged when muted=false.
func (s *Store) MuteTransaction(ctx context.Context, id string, muted bool, reason string) error {
	return s.txns.MuteTransaction(ctx, id, muted, reason)
}

// UpdateMuteReason updates the mute_reason on an individually muted transaction.
func (s *Store) UpdateMuteReason(ctx context.Context, id, reason string) error {
	return s.txns.UpdateMuteReason(ctx, id, reason)
}

// UpdateMerchantReason updates the reason on a muted_merchants entry.
func (s *Store) UpdateMerchantReason(ctx context.Context, id, reason string) error {
	return s.txns.UpdateMerchantReason(ctx, id, reason)
}

// MuteByMerchant mutes all matching transactions (muted_by_merchant=true) and
// stores the pattern in muted_merchants for future auto-muting.
func (s *Store) MuteByMerchant(ctx context.Context, pattern, reason string) error {
	return s.txns.MuteByMerchant(ctx, pattern, reason)
}

// CategorizeMerchant atomically updates all transactions with the given merchant_info
// (exact case-sensitive equality match, not substring) and upserts a user_locked entry
// in merchant_categories for future scans. Returns the number of transaction rows updated.
func (s *Store) CategorizeMerchant(ctx context.Context, merchant, category, bucket string) (int, error) {
	return s.community.CategorizeMerchant(ctx, merchant, category, bucket)
}

// ListMutedMerchants returns all muted merchant patterns ordered by creation time.
func (s *Store) ListMutedMerchants(ctx context.Context) ([]MutedMerchant, error) {
	return s.txns.ListMutedMerchants(ctx)
}

// GetMutedMerchantsWithCount returns each muted merchant with the count of
// transactions currently muted by that merchant-wide rule.
func (s *Store) GetMutedMerchantsWithCount(ctx context.Context) ([]MutedMerchantWithCount, error) {
	return s.txns.GetMutedMerchantsWithCount(ctx)
}

// DeleteMutedMerchant removes a muted merchant pattern by ID.
func (s *Store) DeleteMutedMerchant(ctx context.Context, id string) error {
	return s.txns.DeleteMutedMerchant(ctx, id)
}

// UnmuteByPattern sets muted=false on all transactions whose merchant_info
// matches the pattern (ILIKE contains). Used when removing a merchant-wide rule.
func (s *Store) UnmuteByPattern(ctx context.Context, pattern string) error {
	return s.txns.UnmuteByPattern(ctx, pattern)
}

// DeleteMutedMerchantAndUnmute atomically deletes the merchant pattern and
// sets muted=false on all matching transactions in a single transaction.
// Returns ErrNotFound if no row matched the id.
func (s *Store) DeleteMutedMerchantAndUnmute(ctx context.Context, id string) error {
	return s.txns.DeleteMutedMerchantAndUnmute(ctx, id)
}

// GetMutedMerchantPatterns returns all active ILIKE patterns used for auto-muting at write time.
func (s *Store) GetMutedMerchantPatterns(ctx context.Context) ([]string, error) {
	return s.txns.GetMutedMerchantPatterns(ctx)
}

func (s *Store) loadLabels(ctx context.Context, txns []Transaction) error {
	repo, ok := s.txns.(*pgTransactionsRepository)
	if !ok {
		return errors.New("transactions repository unavailable")
	}
	return repo.loadLabels(ctx, txns)
}

// SeedMCCCodes upserts all MCC codes. Community content is authoritative for
// MCC definitions; this always overwrites existing rows.
func (s *Store) SeedMCCCodes(ctx context.Context, entries []MCCEntry) error {
	return s.community.SeedMCCCodes(ctx, entries)
}

// SeedMerchantCategories upserts community merchant fragment mappings, skipping
// rows where user_locked = true (user has explicitly modified the entry).
func (s *Store) SeedMerchantCategories(ctx context.Context, entries []MerchantCategoryEntry) (int, error) {
	return s.community.SeedMerchantCategories(ctx, entries)
}

// LoadCategorySnapshot builds a CategoryResolver from all merchant_categories rows
// joined with mcc_codes. The resolver does a linear scan and returns the match
// with the longest fragment (most specific wins).
func (s *Store) LoadCategorySnapshot(ctx context.Context) (api.CategoryResolver, error) {
	return s.community.LoadCategorySnapshot(ctx)
}

// SeedMCCCategories inserts MCC-derived category names into the categories table.
// Uses ON CONFLICT DO NOTHING — existing user-created categories are unaffected.
func (s *Store) SeedMCCCategories(ctx context.Context, names []string) error {
	return s.community.SeedMCCCategories(ctx, names)
}

// GetSyncStatus reads the community content sync status from app_config.
// Returns a zero-value SyncStatus (LastSyncedAt = nil) if never synced.
func (s *Store) GetSyncStatus(ctx context.Context) (SyncStatus, error) {
	return s.runtime.GetSyncStatus(ctx)
}

// SetSyncStatus stores the community content sync status in app_config.
func (s *Store) SetSyncStatus(ctx context.Context, status SyncStatus) error {
	return s.runtime.SetSyncStatus(ctx, status)
}

// GetCommunityURL retrieves the community content URL from app_config.
func (s *Store) GetCommunityURL(ctx context.Context) (string, error) {
	return s.runtime.GetCommunityURL(ctx)
}

// SetCommunityURL stores the community content URL in app_config.
func (s *Store) SetCommunityURL(ctx context.Context, url string) error {
	return s.runtime.SetCommunityURL(ctx, url)
}
