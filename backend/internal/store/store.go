// Package store provides database query and persistence operations for Expensor.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
)

// Store wraps a pgxpool.Pool and provides query operations for the API layer.
type Store struct {
	pool      *pgxpool.Pool
	logger    *slog.Logger
	now       func() time.Time
	auth      *pgAuthRepository
	community *pgCommunityRepository
	diag      *pgDiagnosticsRepository
	readModel *pgReadModelRepository
	rules     *pgRulesRepository
	runtime   *pgRuntimeRepository
	taxonomy  *pgTaxonomyRepository
	txns      *pgTransactionsRepository
}

var _ api.DiagnosticSink = (*Store)(nil)

// New creates a Store connected to the PostgreSQL instance described by cfg.
func New(cfg config.Postgres, logger *slog.Logger) (*Store, error) {
	if logger == nil {
		logger = slog.Default()
	}

	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	poolCfg, err := ParsePoolConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parsing store connection string: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxPoolSize
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
	logger.Info("store connected to PostgreSQL", "host", cfg.Host, "database", cfg.Database)
	return s, nil
}

func (s *Store) initRepositories() {
	deps := repositoryDependencies{
		pool:   s.pool,
		logger: s.logger,
		now:    s.now,
	}
	s.auth = newPGAuthRepository(deps)
	s.community = newPGCommunityRepository(deps)
	s.diag = newPGDiagnosticsRepository(deps)
	s.rules = newPGRulesRepository(deps)
	s.runtime = newPGRuntimeRepository(deps)
	s.readModel = newPGReadModelRepository(deps, s.runtime)
	s.taxonomy = newPGTaxonomyRepository(deps)
	s.txns = newPGTransactionsRepository(deps)
}

// Close releases the store's connection pool.
func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) BootstrapRequired(ctx context.Context) (bool, error) {
	return s.auth.BootstrapRequired(ctx)
}

func (s *Store) CreateBootstrapAdmin(ctx context.Context, input CreateBootstrapAdminInput) (*User, error) {
	return s.auth.CreateBootstrapAdmin(ctx, input)
}

func (s *Store) CreateUser(ctx context.Context, input CreateUserInput) (*User, error) {
	return s.auth.CreateUser(ctx, input)
}

func (s *Store) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	return s.auth.FindUserByEmail(ctx, email)
}

func (s *Store) FindUserByID(ctx context.Context, id string) (*User, error) {
	return s.auth.FindUserByID(ctx, id)
}

func (s *Store) CreateSession(ctx context.Context, input CreateSessionInput) (*Session, error) {
	return s.auth.CreateSession(ctx, input)
}

func (s *Store) FindSessionByHash(ctx context.Context, tokenHash string) (*Session, error) {
	return s.auth.FindSessionByHash(ctx, tokenHash)
}

func (s *Store) RevokeSession(ctx context.Context, id string) error {
	return s.auth.RevokeSession(ctx, id)
}

func (s *Store) CreateAccessToken(ctx context.Context, input CreateAccessTokenInput) (*AccessToken, error) {
	return s.auth.CreateAccessToken(ctx, input)
}

func (s *Store) FindAccessTokenByHash(ctx context.Context, tokenHash string) (*AccessToken, error) {
	return s.auth.FindAccessTokenByHash(ctx, tokenHash)
}

func (s *Store) RevokeAccessToken(ctx context.Context, id, userID string) error {
	return s.auth.RevokeAccessToken(ctx, id, userID)
}

func (s *Store) CreateAccountSetupToken(ctx context.Context, input CreateAccountSetupTokenInput) (*AccountSetupToken, error) {
	return s.auth.CreateAccountSetupToken(ctx, input)
}

func (s *Store) FindAccountSetupTokenByHash(ctx context.Context, tokenHash string) (*AccountSetupToken, error) {
	return s.auth.FindAccountSetupTokenByHash(ctx, tokenHash)
}

func (s *Store) MarkAccountSetupTokenUsed(ctx context.Context, id string) error {
	return s.auth.MarkAccountSetupTokenUsed(ctx, id)
}

func (s *Store) queryTransactionTotals(
	ctx context.Context,
	join string,
	where string,
	args []any,
) (TransactionListResult, error) {
	return s.txns.queryTransactionTotals(ctx, join, where, args)
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

// GetChartData returns time-series and breakdown data for dashboard charts.
// All chart queries run concurrently.
func (s *Store) GetChartData(ctx context.Context) (*ChartData, error) {
	return s.readModel.GetChartData(ctx)
}

// GetDashboardData returns dashboard data split into current-month and all-time sections.
func (s *Store) GetDashboardData(ctx context.Context) (*DashboardData, error) {
	return s.readModel.GetDashboardData(ctx)
}

// by day-of-month. When from and to are both non-nil, only transactions within
// [from, to] (inclusive) are included; nil/nil returns all-time data.
func (s *Store) GetSpendingHeatmap(ctx context.Context, from, to *time.Time) (*HeatmapData, error) {
	return s.readModel.GetSpendingHeatmap(ctx, from, to)
}

// the year has no transactions.
func (s *Store) GetAnnualSpend(ctx context.Context, year int) ([]DailyBucket, error) {
	return s.readModel.GetAnnualSpend(ctx, year)
}

// GetSpendingHeatmap. Returns empty string and nil args when both are nil.
// across all transactions. Used to populate filter dropdowns in the UI.
func (s *Store) GetFacets(ctx context.Context) (*Facets, error) {
	return s.txns.GetFacets(ctx)
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

// --- helpers ---

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
func (s *Store) CategorizeMerchant(ctx context.Context, merchant, category, bucket string) (int64, error) {
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
	return s.txns.loadLabels(ctx, txns)
}

// SeedMCCCodes upserts all MCC codes. Community content is authoritative for
// MCC definitions; this always overwrites existing rows.
func (s *Store) SeedMCCCodes(ctx context.Context, entries []MCCEntry) error {
	return s.community.SeedMCCCodes(ctx, entries)
}

// SeedMerchantCategories upserts community merchant fragment mappings, skipping
// rows where user_locked = true (user has explicitly modified the entry).
func (s *Store) SeedMerchantCategories(ctx context.Context, entries []MerchantCategoryEntry) (int64, error) {
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
