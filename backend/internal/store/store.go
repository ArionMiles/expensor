// Package store provides database query operations for the Expensor API.
// It is separate from the writer plugin's pool — the store is used exclusively
// by HTTP handlers for reads and user-initiated writes (descriptions, labels).
package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/pkg/config"
)

// Transaction represents a single expense transaction as returned by the API.
type Transaction struct {
	ID               string    `json:"id"`
	MessageID        string    `json:"message_id"`
	Amount           float64   `json:"amount"`
	Currency         string    `json:"currency"`
	OriginalAmount   *float64  `json:"original_amount,omitempty"`
	OriginalCurrency *string   `json:"original_currency,omitempty"`
	ExchangeRate     *float64  `json:"exchange_rate,omitempty"`
	Timestamp        time.Time `json:"timestamp"`
	MerchantInfo     string    `json:"merchant_info"`
	Category         string    `json:"category"`
	Bucket           string    `json:"bucket"`
	Source           string    `json:"source"`
	Description      string    `json:"description"`
	Labels           []string  `json:"labels"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Stats holds aggregate statistics about stored transactions.
type Stats struct {
	TotalCount      int                `json:"total_count"`
	TotalBase       float64            `json:"total_base"`
	BaseCurrency    string             `json:"base_currency"`
	TotalByCategory map[string]float64 `json:"total_by_category"`
}

// TimeBucket is a single time-period data point used by chart queries.
type TimeBucket struct {
	Period string  `json:"period"` // "2024-01" for monthly, "2024-01-15" for daily
	Amount float64 `json:"amount"`
	Count  int     `json:"count"`
}

// ChartData holds all time-series and breakdown data for the dashboard charts.
type ChartData struct {
	MonthlySpend []TimeBucket       `json:"monthly_spend"`
	DailySpend   []TimeBucket       `json:"daily_spend"`
	ByCategory   map[string]float64 `json:"by_category"`
	ByBucket     map[string]float64 `json:"by_bucket"`
	ByLabel      map[string]float64 `json:"by_label"`
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

// Bucket is a managed spend bucket (needs / wants / savings / income).
type Bucket struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsDefault   bool   `json:"is_default"`
}

// TransactionUpdate carries optional fields for updating a transaction.
// Only non-nil fields are written.
type TransactionUpdate struct {
	Description *string
	Category    *string
	Bucket      *string
}

// ListFilter controls pagination and filtering for ListTransactions.
type ListFilter struct {
	Page     int    // 1-based
	PageSize int    // max rows per page
	Category string // exact match, empty = all
	Currency string // exact match, empty = all
	Source   string // exact match, empty = all
	Label    string // filter by label, empty = all
	From     *time.Time
	To       *time.Time
	SortBy   string // "timestamp" (only supported value for now); default = "timestamp"
	SortDir  string // "asc" | "desc"; default = "desc"
}

// Store wraps a pgxpool.Pool and provides query operations for the API layer.
type Store struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

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

	s := &Store{pool: pool, logger: logger}
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
	logger.Info("store connected to PostgreSQL", "host", cfg.Host, "database", cfg.Database)
	return s, nil
}

// Close releases the store's connection pool.
func (s *Store) Close() {
	s.pool.Close()
}

// ListTransactions returns a paginated, filtered list of transactions and the total
// count matching the filter (ignoring pagination).
func (s *Store) ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}

	// Build WHERE clauses dynamically.
	where, args := buildListWhere(f)
	offset := (f.Page - 1) * f.PageSize

	// Count query.
	countSQL := "SELECT COUNT(DISTINCT t.id) FROM transactions t" + joinLabel(f.Label) + where
	var total int
	if err := s.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting transactions: %w", err)
	}

	// Data query — append LIMIT/OFFSET args after the WHERE args.
	args = append(args, f.PageSize, offset)
	limitArg := len(args) - 1
	offsetArg := len(args)

	orderClause := "t.timestamp DESC"
	if strings.ToLower(f.SortDir) == "asc" {
		orderClause = "t.timestamp ASC"
	}

	dataSQL := fmt.Sprintf(`
		SELECT DISTINCT t.id, t.message_id, t.amount, t.currency,
		       t.original_amount, t.original_currency, t.exchange_rate,
		       t.timestamp, t.merchant_info,
		       COALESCE(t.category, ''), COALESCE(t.bucket, ''), t.source,
		       COALESCE(t.description, ''), t.created_at, t.updated_at
		FROM transactions t%s%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, joinLabel(f.Label), where, orderClause, limitArg, offsetArg)

	rows, err := s.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing transactions: %w", err)
	}
	defer rows.Close()

	txns, err := scanTransactions(rows)
	if err != nil {
		return nil, 0, err
	}

	if err := s.loadLabels(ctx, txns); err != nil {
		return nil, 0, err
	}

	return txns, total, nil
}

// GetTransaction fetches a single transaction by UUID, including its labels.
func (s *Store) GetTransaction(ctx context.Context, id string) (*Transaction, error) {
	const q = `
		SELECT t.id, t.message_id, t.amount, t.currency,
		       t.original_amount, t.original_currency, t.exchange_rate,
		       t.timestamp, t.merchant_info,
		       COALESCE(t.category, ''), COALESCE(t.bucket, ''), t.source,
		       COALESCE(t.description, ''), t.created_at, t.updated_at
		FROM transactions t
		WHERE t.id = $1
	`
	rows, err := s.pool.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("fetching transaction: %w", err)
	}
	defer rows.Close()

	txns, err := scanTransactions(rows)
	if err != nil {
		return nil, err
	}
	if len(txns) == 0 {
		return nil, ErrNotFound
	}

	if err := s.loadLabels(ctx, txns); err != nil {
		return nil, err
	}
	return &txns[0], nil
}

// UpdateDescription sets the user-provided description on a transaction.
func (s *Store) UpdateDescription(ctx context.Context, id, description string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE transactions SET description = $1 WHERE id = $2`,
		description, id,
	)
	if err != nil {
		return fmt.Errorf("updating description: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AddLabel attaches a label to a transaction (idempotent — ignores duplicates).
func (s *Store) AddLabel(ctx context.Context, transactionID, label string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO transaction_labels (transaction_id, label) VALUES ($1, $2)
		 ON CONFLICT (transaction_id, label) DO NOTHING`,
		transactionID, label,
	)
	if err != nil {
		return fmt.Errorf("adding label: %w", err)
	}
	return nil
}

// AddLabels attaches multiple labels to a transaction in a single round-trip (idempotent).
func (s *Store) AddLabels(ctx context.Context, transactionID string, labels []string) error {
	if len(labels) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO transaction_labels (transaction_id, label)
		 SELECT $1, unnest($2::text[])
		 ON CONFLICT (transaction_id, label) DO NOTHING`,
		transactionID, labels,
	)
	if err != nil {
		return fmt.Errorf("adding labels: %w", err)
	}
	return nil
}

// RemoveLabel detaches a label from a transaction.
func (s *Store) RemoveLabel(ctx context.Context, transactionID, label string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM transaction_labels WHERE transaction_id = $1 AND label = $2`,
		transactionID, label,
	)
	if err != nil {
		return fmt.Errorf("removing label: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SearchTransactions performs a full-text search over merchant_info and description.
func (s *Store) SearchTransactions(ctx context.Context, query string, f ListFilter) ([]Transaction, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}

	// tsquery from the search term.
	tsq := strings.Join(strings.Fields(query), " & ")
	if tsq == "" {
		return s.ListTransactions(ctx, f)
	}

	where, args := buildListWhere(f)
	// Prepend search condition; existing args are already in place.
	searchCond := buildSearchCondition(tsq, &args)
	fullWhere := combineWhere(searchCond, where)
	offset := (f.Page - 1) * f.PageSize

	countSQL := "SELECT COUNT(DISTINCT t.id) FROM transactions t" + joinLabel(f.Label) + fullWhere
	var total int
	if err := s.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting search results: %w", err)
	}

	args = append(args, f.PageSize, offset)
	limitArg := len(args) - 1
	offsetArg := len(args)

	dataSQL := fmt.Sprintf(`
		SELECT DISTINCT t.id, t.message_id, t.amount, t.currency,
		       t.original_amount, t.original_currency, t.exchange_rate,
		       t.timestamp, t.merchant_info,
		       COALESCE(t.category, ''), COALESCE(t.bucket, ''), t.source,
		       COALESCE(t.description, ''), t.created_at, t.updated_at
		FROM transactions t%s%s
		ORDER BY t.timestamp DESC
		LIMIT $%d OFFSET $%d
	`, joinLabel(f.Label), fullWhere, limitArg, offsetArg)

	rows, err := s.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("searching transactions: %w", err)
	}
	defer rows.Close()

	txns, err := scanTransactions(rows)
	if err != nil {
		return nil, 0, err
	}
	if err := s.loadLabels(ctx, txns); err != nil {
		return nil, 0, err
	}
	return txns, total, nil
}

// GetStats returns aggregate counts and totals across all transactions.
func (s *Store) GetStats(ctx context.Context, baseCurrency string) (*Stats, error) {
	const mainQ = `
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN currency = $1 THEN amount ELSE 0 END), 0)
		FROM transactions
	`
	var st Stats
	st.BaseCurrency = baseCurrency
	if err := s.pool.QueryRow(ctx, mainQ, baseCurrency).Scan(&st.TotalCount, &st.TotalBase); err != nil {
		return nil, fmt.Errorf("fetching stats: %w", err)
	}

	const catQ = `
		SELECT COALESCE(category, ''), COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE category IS NOT NULL AND category != ''
		GROUP BY category
		ORDER BY SUM(amount) DESC
	`
	rows, err := s.pool.Query(ctx, catQ)
	if err != nil {
		return nil, fmt.Errorf("fetching category stats: %w", err)
	}
	defer rows.Close()

	st.TotalByCategory = make(map[string]float64)
	for rows.Next() {
		var cat string
		var amt float64
		if err := rows.Scan(&cat, &amt); err != nil {
			return nil, fmt.Errorf("scanning category row: %w", err)
		}
		st.TotalByCategory[cat] = amt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating category rows: %w", err)
	}

	return &st, nil
}

// GetChartData returns time-series and breakdown data for dashboard charts.
// All 5 queries run concurrently.
func (s *Store) GetChartData(ctx context.Context) (*ChartData, error) {
	cd := &ChartData{
		MonthlySpend: []TimeBucket{},
		DailySpend:   []TimeBucket{},
		ByCategory:   make(map[string]float64),
		ByBucket:     make(map[string]float64),
		ByLabel:      make(map[string]float64),
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

	var wg sync.WaitGroup
	wg.Add(5)

	go func() {
		defer wg.Done()
		buckets, err := s.queryTimeBuckets(ctx, `
			SELECT TO_CHAR(timestamp, 'YYYY-MM') AS period,
			       COALESCE(SUM(amount), 0)      AS amount,
			       COUNT(*)                      AS cnt
			FROM transactions
			WHERE timestamp >= NOW() - INTERVAL '12 months'
			GROUP BY period
			ORDER BY period
		`)
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
		buckets, err := s.queryTimeBuckets(ctx, `
			SELECT TO_CHAR(timestamp, 'YYYY-MM-DD') AS period,
			       COALESCE(SUM(amount), 0)         AS amount,
			       COUNT(*)                         AS cnt
			FROM transactions
			WHERE timestamp >= NOW() - INTERVAL '30 days'
			GROUP BY period
			ORDER BY period
		`)
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
		if err := s.queryStringFloat(ctx, `
			SELECT COALESCE(category, ''), COALESCE(SUM(amount), 0)
			FROM transactions
			WHERE category IS NOT NULL AND category != ''
			GROUP BY category
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
		if err := s.queryStringFloat(ctx, `
			SELECT COALESCE(bucket, ''), COALESCE(SUM(amount), 0)
			FROM transactions
			WHERE bucket IS NOT NULL AND bucket != ''
			GROUP BY bucket
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
		if err := s.queryStringFloat(ctx, `
			SELECT tl.label, COALESCE(SUM(t.amount), 0)
			FROM transactions t
			JOIN transaction_labels tl ON tl.transaction_id = t.id
			GROUP BY tl.label
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

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	return cd, nil
}

// Facets holds distinct filter values for the transactions UI dropdowns.
type Facets struct {
	Sources    []string `json:"sources"`
	Categories []string `json:"categories"`
	Currencies []string `json:"currencies"`
	Labels     []string `json:"labels"`
}

// GetFacets returns the distinct non-empty values for source, category, currency, and label
// across all transactions. Used to populate filter dropdowns in the UI.
func (s *Store) GetFacets(ctx context.Context) (*Facets, error) {
	var f Facets

	queries := []struct {
		sql  string
		dest *[]string
	}{
		{
			`SELECT DISTINCT source FROM transactions
             WHERE source IS NOT NULL AND source != ''
             ORDER BY source`,
			&f.Sources,
		},
		{
			`SELECT DISTINCT category FROM transactions
             WHERE category IS NOT NULL AND category != ''
             ORDER BY category`,
			&f.Categories,
		},
		{
			`SELECT DISTINCT currency FROM transactions
             WHERE currency IS NOT NULL AND currency != ''
             ORDER BY currency`,
			&f.Currencies,
		},
		{
			`SELECT DISTINCT label FROM transaction_labels
             ORDER BY label`,
			&f.Labels,
		},
	}

	for _, q := range queries {
		rows, err := s.pool.Query(ctx, q.sql)
		if err != nil {
			return nil, fmt.Errorf("fetching facets: %w", err)
		}
		var vals []string
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scanning facet value: %w", err)
			}
			vals = append(vals, v)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterating facet rows: %w", err)
		}
		*q.dest = vals
	}

	if f.Sources == nil {
		f.Sources = []string{}
	}
	if f.Categories == nil {
		f.Categories = []string{}
	}
	if f.Currencies == nil {
		f.Currencies = []string{}
	}
	if f.Labels == nil {
		f.Labels = []string{}
	}

	return &f, nil
}

func (s *Store) queryTimeBuckets(ctx context.Context, q string) ([]TimeBucket, error) {
	rows, err := s.pool.Query(ctx, q)
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

func (s *Store) queryStringFloat(ctx context.Context, q string, dest map[string]float64) error {
	rows, err := s.pool.Query(ctx, q)
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

// initAppConfig creates the app_config table and seeds the default base_currency row.
// It is called once from New and is idempotent.
func (s *Store) initAppConfig(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS app_config (
		    key   TEXT PRIMARY KEY,
		    value TEXT NOT NULL
		);
		INSERT INTO app_config (key, value) VALUES ('base_currency', 'INR')
		ON CONFLICT (key) DO NOTHING;
	`)
	return err
}

// GetAppConfig retrieves a configuration value by key.
// Returns an error if the key does not exist.
func (s *Store) GetAppConfig(ctx context.Context, key string) (string, error) {
	var value string
	err := s.pool.QueryRow(ctx, `SELECT value FROM app_config WHERE key = $1`, key).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("getting app config %q: %w", key, err)
	}
	return value, nil
}

// SetAppConfig upserts a configuration value.
func (s *Store) SetAppConfig(ctx context.Context, key, value string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO app_config (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("setting app config %q: %w", key, err)
	}
	return nil
}

// initLabels creates the labels table and seeds the default rows. Idempotent.
func (s *Store) initLabels(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS labels (
			name        TEXT PRIMARY KEY,
			color       TEXT NOT NULL DEFAULT '#6366f1',
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		INSERT INTO labels (name, color) VALUES
			('food',          '#f59e0b'),
			('transport',     '#3b82f6'),
			('shopping',      '#8b5cf6'),
			('utilities',     '#06b6d4'),
			('healthcare',    '#10b981'),
			('entertainment', '#ec4899'),
			('travel',        '#f97316'),
			('recurring',     '#6366f1')
		ON CONFLICT (name) DO NOTHING;
	`)
	return err
}

// initCategoriesBuckets creates the categories and buckets tables and seeds defaults. Idempotent.
func (s *Store) initCategoriesBuckets(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS categories (
			name        TEXT PRIMARY KEY,
			description TEXT,
			is_default  BOOLEAN NOT NULL DEFAULT false,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS buckets (
			name        TEXT PRIMARY KEY,
			description TEXT,
			is_default  BOOLEAN NOT NULL DEFAULT false,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		INSERT INTO categories (name, is_default) VALUES
			('food & dining', true),('transport', true),('shopping', true),
			('utilities', true),('healthcare', true),('entertainment', true),
			('travel', true),('finance', true),('uncategorized', true)
		ON CONFLICT (name) DO NOTHING;
		INSERT INTO buckets (name, is_default) VALUES
			('needs', true),('wants', true),('savings', true),('income', true)
		ON CONFLICT (name) DO NOTHING;
	`)
	return err
}

// --- Labels ---

// ListLabels returns all labels ordered by name.
func (s *Store) ListLabels(ctx context.Context) ([]Label, error) {
	rows, err := s.pool.Query(ctx, `SELECT name, color, created_at FROM labels ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing labels: %w", err)
	}
	defer rows.Close()
	var labels []Label
	for rows.Next() {
		var l Label
		if err := rows.Scan(&l.Name, &l.Color, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning label: %w", err)
		}
		labels = append(labels, l)
	}
	if labels == nil {
		labels = []Label{}
	}
	return labels, rows.Err()
}

// CreateLabel inserts a new label. Silently ignores duplicate names.
func (s *Store) CreateLabel(ctx context.Context, name, color string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO labels (name, color) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING`,
		name, color,
	)
	if err != nil {
		return fmt.Errorf("creating label: %w", err)
	}
	return nil
}

// UpdateLabel changes the color of an existing label. Returns ErrNotFound if no row matched.
func (s *Store) UpdateLabel(ctx context.Context, name, color string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE labels SET color = $1 WHERE name = $2`, color, name)
	if err != nil {
		return fmt.Errorf("updating label: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteLabel removes a label by name.
func (s *Store) DeleteLabel(ctx context.Context, name string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM labels WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("deleting label: %w", err)
	}
	return nil
}

// ApplyLabelByMerchant bulk-applies a label to all transactions whose
// merchant_info matches the given pattern (case-insensitive contains).
// Returns the number of rows inserted.
func (s *Store) ApplyLabelByMerchant(ctx context.Context, label, pattern string) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO transaction_labels (transaction_id, label)
		 SELECT id, $1 FROM transactions
		 WHERE merchant_info ILIKE '%' || $2 || '%'
		 ON CONFLICT (transaction_id, label) DO NOTHING`,
		label, pattern,
	)
	if err != nil {
		return 0, fmt.Errorf("applying label by merchant: %w", err)
	}
	return tag.RowsAffected(), nil
}

// --- Categories ---

// ListCategories returns all categories ordered by name.
func (s *Store) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT name, COALESCE(description,''), is_default FROM categories ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}
	defer rows.Close()
	var cats []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.Name, &c.Description, &c.IsDefault); err != nil {
			return nil, fmt.Errorf("scanning category: %w", err)
		}
		cats = append(cats, c)
	}
	if cats == nil {
		cats = []Category{}
	}
	return cats, rows.Err()
}

// CreateCategory inserts a new category. Silently ignores duplicate names.
func (s *Store) CreateCategory(ctx context.Context, name, description string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO categories (name, description) VALUES ($1, NULLIF($2,''))
		 ON CONFLICT (name) DO NOTHING`,
		name, description,
	)
	if err != nil {
		return fmt.Errorf("creating category: %w", err)
	}
	return nil
}

// DeleteCategory removes a category by name. Returns ErrNotFound if it does not exist.
// Returns an error if the category is a default one.
func (s *Store) DeleteCategory(ctx context.Context, name string) error {
	var isDefault bool
	err := s.pool.QueryRow(ctx, `SELECT is_default FROM categories WHERE name = $1`, name).Scan(&isDefault)
	if err != nil {
		return ErrNotFound
	}
	if isDefault {
		return fmt.Errorf("cannot delete default category %q", name)
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM categories WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("deleting category: %w", err)
	}
	return nil
}

// --- Buckets ---

// ListBuckets returns all buckets ordered by name.
func (s *Store) ListBuckets(ctx context.Context) ([]Bucket, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT name, COALESCE(description,''), is_default FROM buckets ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing buckets: %w", err)
	}
	defer rows.Close()
	var buckets []Bucket
	for rows.Next() {
		var b Bucket
		if err := rows.Scan(&b.Name, &b.Description, &b.IsDefault); err != nil {
			return nil, fmt.Errorf("scanning bucket: %w", err)
		}
		buckets = append(buckets, b)
	}
	if buckets == nil {
		buckets = []Bucket{}
	}
	return buckets, rows.Err()
}

// CreateBucket inserts a new bucket. Silently ignores duplicate names.
func (s *Store) CreateBucket(ctx context.Context, name, description string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO buckets (name, description) VALUES ($1, NULLIF($2,''))
		 ON CONFLICT (name) DO NOTHING`,
		name, description,
	)
	if err != nil {
		return fmt.Errorf("creating bucket: %w", err)
	}
	return nil
}

// DeleteBucket removes a bucket by name. Returns ErrNotFound if it does not exist.
// Returns an error if the bucket is a default one.
func (s *Store) DeleteBucket(ctx context.Context, name string) error {
	var isDefault bool
	err := s.pool.QueryRow(ctx, `SELECT is_default FROM buckets WHERE name = $1`, name).Scan(&isDefault)
	if err != nil {
		return ErrNotFound
	}
	if isDefault {
		return fmt.Errorf("cannot delete default bucket %q", name)
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM buckets WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("deleting bucket: %w", err)
	}
	return nil
}

// --- Transaction update ---

// UpdateTransaction updates one or more optional fields on a transaction.
// Only non-nil pointer fields are written. Returns ErrNotFound if no row matched.
func (s *Store) UpdateTransaction(ctx context.Context, id string, u TransactionUpdate) error {
	if u.Description == nil && u.Category == nil && u.Bucket == nil {
		return nil
	}
	var setClauses []string
	var args []any
	n := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if u.Description != nil {
		setClauses = append(setClauses, "description = "+n(*u.Description))
	}
	if u.Category != nil {
		setClauses = append(setClauses, "category = "+n(*u.Category))
	}
	if u.Bucket != nil {
		setClauses = append(setClauses, "bucket = "+n(*u.Bucket))
	}
	args = append(args, id)
	q := fmt.Sprintf(
		"UPDATE transactions SET %s, updated_at = NOW() WHERE id = $%d",
		strings.Join(setClauses, ", "), len(args),
	)
	tag, err := s.pool.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("updating transaction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ErrNotFound is returned when an operation targets a row that does not exist.
var ErrNotFound = errors.New("not found")

// --- helpers ---

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

	if f.Label != "" {
		conds = append(conds, fmt.Sprintf("tl.label ILIKE %s", next("%"+f.Label+"%")))
	}
	if f.Category != "" {
		conds = append(conds, fmt.Sprintf("t.category ILIKE %s", next("%"+f.Category+"%")))
	}
	if f.Currency != "" {
		conds = append(conds, fmt.Sprintf("t.currency ILIKE %s", next("%"+f.Currency+"%")))
	}
	if f.Source != "" {
		conds = append(conds, fmt.Sprintf("t.source ILIKE %s", next("%"+f.Source+"%")))
	}
	if f.From != nil {
		conds = append(conds, fmt.Sprintf("t.timestamp >= %s", next(*f.From)))
	}
	if f.To != nil {
		conds = append(conds, fmt.Sprintf("t.timestamp <= %s", next(*f.To)))
	}

	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// buildSearchCondition appends the tsvector search arg and returns its condition string.
func buildSearchCondition(tsq string, args *[]any) string {
	*args = append(*args, tsq)
	n := len(*args)
	return fmt.Sprintf(
		"(to_tsvector('english', t.merchant_info) || to_tsvector('english', COALESCE(t.description,''))) @@ to_tsquery('english', $%d)",
		n,
	)
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
		if err := rows.Scan(
			&t.ID, &t.MessageID, &t.Amount, &t.Currency,
			&t.OriginalAmount, &t.OriginalCurrency, &t.ExchangeRate,
			&t.Timestamp, &t.MerchantInfo, &t.Category, &t.Bucket,
			&t.Source, &t.Description, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning transaction row: %w", err)
		}
		t.Labels = []string{}
		txns = append(txns, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating transaction rows: %w", err)
	}
	return txns, nil
}

// loadLabels fetches labels for all transactions in a single query and attaches them.
func (s *Store) loadLabels(ctx context.Context, txns []Transaction) error {
	if len(txns) == 0 {
		return nil
	}

	ids := make([]string, len(txns))
	idx := make(map[string]int, len(txns))
	for i, t := range txns {
		ids[i] = t.ID
		idx[t.ID] = i
	}

	rows, err := s.pool.Query(ctx,
		`SELECT transaction_id, label FROM transaction_labels WHERE transaction_id = ANY($1) ORDER BY label`,
		ids,
	)
	if err != nil {
		return fmt.Errorf("fetching labels: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tid, label string
		if err := rows.Scan(&tid, &label); err != nil {
			return fmt.Errorf("scanning label row: %w", err)
		}
		if i, ok := idx[tid]; ok {
			txns[i].Labels = append(txns[i].Labels, label)
		}
	}
	return rows.Err()
}
