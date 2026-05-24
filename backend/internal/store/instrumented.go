package store

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/observability"
)

// InstrumentedTransactionStore records telemetry around transaction store calls.
type InstrumentedTransactionStore struct {
	next  TransactionStore
	scope *observability.Scope
	now   func() time.Time
}

func NewInstrumentedTransactionStore(next TransactionStore, scope *observability.Scope, logger *slog.Logger) *InstrumentedTransactionStore {
	if logger == nil {
		logger = slog.Default()
	}
	if scope == nil {
		scope = observability.NewScope(logger, "store")
	}
	return &InstrumentedTransactionStore{
		next:  next,
		scope: scope,
		now:   time.Now,
	}
}

func (s *InstrumentedTransactionStore) SetNowForTest(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

func (s *InstrumentedTransactionStore) ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, TransactionListResult, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.list")
	defer span.End()

	start := s.now()
	rows, result, err := s.next.ListTransactions(ctx, f)
	s.scope.RecordOperation(ctx, observability.Operation{
		Namespace: "store",
		Name:      "transactions.list",
		Duration:  time.Since(start),
		Err:       err,
	})
	return rows, result, err
}

// InstrumentedStore records telemetry around the full store surface.
type InstrumentedStore struct {
	next  FullStore
	scope *observability.Scope
	now   func() time.Time
}

func NewInstrumentedStore(next FullStore, scope *observability.Scope, logger *slog.Logger) *InstrumentedStore {
	if logger == nil {
		logger = slog.Default()
	}
	if scope == nil {
		scope = observability.NewScope(logger, "store")
	}
	return &InstrumentedStore{next: next, scope: scope, now: time.Now}
}

func (s *InstrumentedStore) SetNowForTest(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

func (s *InstrumentedStore) observe(ctx context.Context, name string, fn func(context.Context) error) error {
	ctx, span := s.scope.Start(ctx, "store."+name)
	defer span.End()

	start := s.now()
	err := fn(ctx)
	s.scope.RecordOperation(ctx, observability.Operation{
		Namespace: "store",
		Name:      name,
		Duration:  time.Since(start),
		Err:       err,
	})
	return err
}

func observe1[T any](ctx context.Context, s *InstrumentedStore, name string, fn func(context.Context) (T, error)) (T, error) {
	var out T
	err := s.observe(ctx, name, func(ctx context.Context) error {
		var err error
		out, err = fn(ctx)
		return err
	})
	return out, err
}

func observe2[A, B any](
	ctx context.Context,
	s *InstrumentedStore,
	name string,
	fn func(context.Context) (A, B, error),
) (A, B, error) {
	var outA A
	var outB B
	err := s.observe(ctx, name, func(ctx context.Context) error {
		var err error
		outA, outB, err = fn(ctx)
		return err
	})
	return outA, outB, err
}

func (s *InstrumentedStore) ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, TransactionListResult, error) {
	return observe2(ctx, s, "transactions.list", func(ctx context.Context) ([]Transaction, TransactionListResult, error) {
		return s.next.ListTransactions(ctx, f)
	})
}

func (s *InstrumentedStore) GetTransaction(ctx context.Context, id string) (*Transaction, error) {
	return observe1(ctx, s, "transactions.get", func(ctx context.Context) (*Transaction, error) {
		return s.next.GetTransaction(ctx, id)
	})
}

func (s *InstrumentedStore) UpdateDescription(ctx context.Context, id, description string) error {
	return s.observe(ctx, "transactions.update_description", func(ctx context.Context) error {
		return s.next.UpdateDescription(ctx, id, description)
	})
}

func (s *InstrumentedStore) AddLabel(ctx context.Context, transactionID, label string) error {
	return s.observe(ctx, "transactions.add_label", func(ctx context.Context) error {
		return s.next.AddLabel(ctx, transactionID, label)
	})
}

func (s *InstrumentedStore) AddLabels(ctx context.Context, transactionID string, labels []string) error {
	return s.observe(ctx, "transactions.add_labels", func(ctx context.Context) error {
		return s.next.AddLabels(ctx, transactionID, labels)
	})
}

func (s *InstrumentedStore) RemoveLabel(ctx context.Context, transactionID, label string) error {
	return s.observe(ctx, "transactions.remove_label", func(ctx context.Context) error {
		return s.next.RemoveLabel(ctx, transactionID, label)
	})
}

func (s *InstrumentedStore) SearchTransactions(ctx context.Context, query string, f ListFilter) ([]Transaction, TransactionListResult, error) {
	return observe2(ctx, s, "transactions.search", func(ctx context.Context) ([]Transaction, TransactionListResult, error) {
		return s.next.SearchTransactions(ctx, query, f)
	})
}

func (s *InstrumentedStore) GetStats(ctx context.Context, baseCurrency string) (*Stats, error) {
	return observe1(ctx, s, "read_model.get_stats", func(ctx context.Context) (*Stats, error) {
		return s.next.GetStats(ctx, baseCurrency)
	})
}

func (s *InstrumentedStore) GetChartData(ctx context.Context) (*ChartData, error) {
	return observe1(ctx, s, "read_model.get_chart_data", s.next.GetChartData)
}

func (s *InstrumentedStore) GetDashboardData(ctx context.Context) (*DashboardData, error) {
	return observe1(ctx, s, "read_model.get_dashboard_data", s.next.GetDashboardData)
}

func (s *InstrumentedStore) GetSpendingHeatmap(ctx context.Context, from, to *time.Time) (*HeatmapData, error) {
	return observe1(ctx, s, "read_model.get_spending_heatmap", func(ctx context.Context) (*HeatmapData, error) {
		return s.next.GetSpendingHeatmap(ctx, from, to)
	})
}

func (s *InstrumentedStore) GetAnnualSpend(ctx context.Context, year int) ([]DailyBucket, error) {
	return observe1(ctx, s, "read_model.get_annual_spend", func(ctx context.Context) ([]DailyBucket, error) {
		return s.next.GetAnnualSpend(ctx, year)
	})
}

func (s *InstrumentedStore) GetAppConfig(ctx context.Context, key string) (string, error) {
	return observe1(ctx, s, "runtime.get_app_config", func(ctx context.Context) (string, error) {
		return s.next.GetAppConfig(ctx, key)
	})
}

func (s *InstrumentedStore) SetAppConfig(ctx context.Context, key, value string) error {
	return s.observe(ctx, "runtime.set_app_config", func(ctx context.Context) error {
		return s.next.SetAppConfig(ctx, key, value)
	})
}

func (s *InstrumentedStore) IsMessageProcessed(ctx context.Context, key string) (bool, error) {
	return observe1(ctx, s, "runtime.is_message_processed", func(ctx context.Context) (bool, error) {
		return s.next.IsMessageProcessed(ctx, key)
	})
}

func (s *InstrumentedStore) MarkMessageProcessed(ctx context.Context, key string, at time.Time) error {
	return s.observe(ctx, "runtime.mark_message_processed", func(ctx context.Context) error {
		return s.next.MarkMessageProcessed(ctx, key, at)
	})
}

func (s *InstrumentedStore) SetActiveReader(ctx context.Context, reader string) error {
	return s.observe(ctx, "runtime.set_active_reader", func(ctx context.Context) error {
		return s.next.SetActiveReader(ctx, reader)
	})
}

func (s *InstrumentedStore) GetActiveReader(ctx context.Context) (string, error) {
	return observe1(ctx, s, "runtime.get_active_reader", s.next.GetActiveReader)
}

func (s *InstrumentedStore) SetReaderSecret(ctx context.Context, reader string, secret []byte) error {
	return s.observe(ctx, "runtime.set_reader_secret", func(ctx context.Context) error {
		return s.next.SetReaderSecret(ctx, reader, secret)
	})
}

func (s *InstrumentedStore) GetReaderSecret(ctx context.Context, reader string) ([]byte, bool, error) {
	return observe2(ctx, s, "runtime.get_reader_secret", func(ctx context.Context) ([]byte, bool, error) {
		return s.next.GetReaderSecret(ctx, reader)
	})
}

func (s *InstrumentedStore) SetReaderToken(ctx context.Context, reader string, token []byte) error {
	return s.observe(ctx, "runtime.set_reader_token", func(ctx context.Context) error {
		return s.next.SetReaderToken(ctx, reader, token)
	})
}

func (s *InstrumentedStore) GetReaderToken(ctx context.Context, reader string) ([]byte, bool, error) {
	return observe2(ctx, s, "runtime.get_reader_token", func(ctx context.Context) ([]byte, bool, error) {
		return s.next.GetReaderToken(ctx, reader)
	})
}

func (s *InstrumentedStore) DeleteReaderToken(ctx context.Context, reader string) error {
	return s.observe(ctx, "runtime.delete_reader_token", func(ctx context.Context) error {
		return s.next.DeleteReaderToken(ctx, reader)
	})
}

func (s *InstrumentedStore) SetReaderConfig(ctx context.Context, reader string, config json.RawMessage) error {
	return s.observe(ctx, "runtime.set_reader_config", func(ctx context.Context) error {
		return s.next.SetReaderConfig(ctx, reader, config)
	})
}

func (s *InstrumentedStore) GetReaderConfig(ctx context.Context, reader string) (json.RawMessage, bool, error) {
	return observe2(ctx, s, "runtime.get_reader_config", func(ctx context.Context) (json.RawMessage, bool, error) {
		return s.next.GetReaderConfig(ctx, reader)
	})
}

func (s *InstrumentedStore) DeleteReaderRuntime(ctx context.Context, reader string) error {
	return s.observe(ctx, "runtime.delete_reader_runtime", func(ctx context.Context) error {
		return s.next.DeleteReaderRuntime(ctx, reader)
	})
}

func (s *InstrumentedStore) GetFacets(ctx context.Context) (*Facets, error) {
	return observe1(ctx, s, "transactions.get_facets", s.next.GetFacets)
}

func (s *InstrumentedStore) ListLabels(ctx context.Context) ([]Label, error) {
	return observe1(ctx, s, "taxonomy.list_labels", s.next.ListLabels)
}

func (s *InstrumentedStore) CreateLabel(ctx context.Context, name, color string) error {
	return s.observe(ctx, "taxonomy.create_label", func(ctx context.Context) error {
		return s.next.CreateLabel(ctx, name, color)
	})
}

func (s *InstrumentedStore) UpdateLabel(ctx context.Context, name, color string) error {
	return s.observe(ctx, "taxonomy.update_label", func(ctx context.Context) error {
		return s.next.UpdateLabel(ctx, name, color)
	})
}

func (s *InstrumentedStore) DeleteLabel(ctx context.Context, name string, removeFromTransactions bool) error {
	return s.observe(ctx, "taxonomy.delete_label", func(ctx context.Context) error {
		return s.next.DeleteLabel(ctx, name, removeFromTransactions)
	})
}

func (s *InstrumentedStore) ApplyLabelByMerchant(ctx context.Context, label, pattern string) (int64, error) {
	return observe1(ctx, s, "taxonomy.apply_label_by_merchant", func(ctx context.Context) (int64, error) {
		return s.next.ApplyLabelByMerchant(ctx, label, pattern)
	})
}

func (s *InstrumentedStore) RemoveLabelByMerchant(ctx context.Context, label, pattern string) (int64, error) {
	return observe1(ctx, s, "taxonomy.remove_label_by_merchant", func(ctx context.Context) (int64, error) {
		return s.next.RemoveLabelByMerchant(ctx, label, pattern)
	})
}

func (s *InstrumentedStore) GetLabelMappings(ctx context.Context) (map[string][]string, error) {
	return observe1(ctx, s, "taxonomy.get_label_mappings", s.next.GetLabelMappings)
}

func (s *InstrumentedStore) GetMonthlyBreakdownSpend(ctx context.Context, dimension string, months int) (*MonthlyBreakdownData, error) {
	return observe1(ctx, s, "read_model.get_monthly_breakdown_spend", func(ctx context.Context) (*MonthlyBreakdownData, error) {
		return s.next.GetMonthlyBreakdownSpend(ctx, dimension, months)
	})
}

func (s *InstrumentedStore) ListCategories(ctx context.Context) ([]Category, error) {
	return observe1(ctx, s, "taxonomy.list_categories", s.next.ListCategories)
}

func (s *InstrumentedStore) CreateCategory(ctx context.Context, name, description string) error {
	return s.observe(ctx, "taxonomy.create_category", func(ctx context.Context) error {
		return s.next.CreateCategory(ctx, name, description)
	})
}

func (s *InstrumentedStore) DeleteCategory(ctx context.Context, name string, removeFromTransactions bool) error {
	return s.observe(ctx, "taxonomy.delete_category", func(ctx context.Context) error {
		return s.next.DeleteCategory(ctx, name, removeFromTransactions)
	})
}

func (s *InstrumentedStore) ApplyCategoryByMerchant(ctx context.Context, category, pattern string) (int64, error) {
	return observe1(ctx, s, "taxonomy.apply_category_by_merchant", func(ctx context.Context) (int64, error) {
		return s.next.ApplyCategoryByMerchant(ctx, category, pattern)
	})
}

func (s *InstrumentedStore) RemoveCategoryByMerchant(ctx context.Context, category, pattern string) (int64, error) {
	return observe1(ctx, s, "taxonomy.remove_category_by_merchant", func(ctx context.Context) (int64, error) {
		return s.next.RemoveCategoryByMerchant(ctx, category, pattern)
	})
}

func (s *InstrumentedStore) GetCategoryMappings(ctx context.Context) (map[string][]string, error) {
	return observe1(ctx, s, "taxonomy.get_category_mappings", s.next.GetCategoryMappings)
}

func (s *InstrumentedStore) ListBuckets(ctx context.Context) ([]Bucket, error) {
	return observe1(ctx, s, "taxonomy.list_buckets", s.next.ListBuckets)
}

func (s *InstrumentedStore) CreateBucket(ctx context.Context, name, description string) error {
	return s.observe(ctx, "taxonomy.create_bucket", func(ctx context.Context) error {
		return s.next.CreateBucket(ctx, name, description)
	})
}

func (s *InstrumentedStore) DeleteBucket(ctx context.Context, name string, removeFromTransactions bool) error {
	return s.observe(ctx, "taxonomy.delete_bucket", func(ctx context.Context) error {
		return s.next.DeleteBucket(ctx, name, removeFromTransactions)
	})
}

func (s *InstrumentedStore) ApplyBucketByMerchant(ctx context.Context, bucket, pattern string) (int64, error) {
	return observe1(ctx, s, "taxonomy.apply_bucket_by_merchant", func(ctx context.Context) (int64, error) {
		return s.next.ApplyBucketByMerchant(ctx, bucket, pattern)
	})
}

func (s *InstrumentedStore) RemoveBucketByMerchant(ctx context.Context, bucket, pattern string) (int64, error) {
	return observe1(ctx, s, "taxonomy.remove_bucket_by_merchant", func(ctx context.Context) (int64, error) {
		return s.next.RemoveBucketByMerchant(ctx, bucket, pattern)
	})
}

func (s *InstrumentedStore) GetBucketMappings(ctx context.Context) (map[string][]string, error) {
	return observe1(ctx, s, "taxonomy.get_bucket_mappings", s.next.GetBucketMappings)
}

func (s *InstrumentedStore) UpdateTransaction(ctx context.Context, id string, u TransactionUpdate) error {
	return s.observe(ctx, "transactions.update", func(ctx context.Context) error {
		return s.next.UpdateTransaction(ctx, id, u)
	})
}

func (s *InstrumentedStore) MuteTransaction(ctx context.Context, id string, muted bool, reason string) error {
	return s.observe(ctx, "transactions.mute", func(ctx context.Context) error {
		return s.next.MuteTransaction(ctx, id, muted, reason)
	})
}

func (s *InstrumentedStore) UpdateMuteReason(ctx context.Context, id, reason string) error {
	return s.observe(ctx, "transactions.update_mute_reason", func(ctx context.Context) error {
		return s.next.UpdateMuteReason(ctx, id, reason)
	})
}

func (s *InstrumentedStore) UpdateMerchantReason(ctx context.Context, id, reason string) error {
	return s.observe(ctx, "transactions.update_merchant_reason", func(ctx context.Context) error {
		return s.next.UpdateMerchantReason(ctx, id, reason)
	})
}

func (s *InstrumentedStore) MuteByMerchant(ctx context.Context, pattern, reason string) error {
	return s.observe(ctx, "transactions.mute_by_merchant", func(ctx context.Context) error {
		return s.next.MuteByMerchant(ctx, pattern, reason)
	})
}

func (s *InstrumentedStore) ListMutedMerchants(ctx context.Context) ([]MutedMerchant, error) {
	return observe1(ctx, s, "transactions.list_muted_merchants", s.next.ListMutedMerchants)
}

func (s *InstrumentedStore) GetMutedMerchantsWithCount(ctx context.Context) ([]MutedMerchantWithCount, error) {
	return observe1(ctx, s, "transactions.get_muted_merchants_with_count", s.next.GetMutedMerchantsWithCount)
}

func (s *InstrumentedStore) DeleteMutedMerchant(ctx context.Context, id string) error {
	return s.observe(ctx, "transactions.delete_muted_merchant", func(ctx context.Context) error {
		return s.next.DeleteMutedMerchant(ctx, id)
	})
}

func (s *InstrumentedStore) UnmuteByPattern(ctx context.Context, pattern string) error {
	return s.observe(ctx, "transactions.unmute_by_pattern", func(ctx context.Context) error {
		return s.next.UnmuteByPattern(ctx, pattern)
	})
}

func (s *InstrumentedStore) DeleteMutedMerchantAndUnmute(ctx context.Context, id string) error {
	return s.observe(ctx, "transactions.delete_muted_merchant_and_unmute", func(ctx context.Context) error {
		return s.next.DeleteMutedMerchantAndUnmute(ctx, id)
	})
}

func (s *InstrumentedStore) GetMutedMerchantPatterns(ctx context.Context) ([]string, error) {
	return observe1(ctx, s, "transactions.get_muted_merchant_patterns", s.next.GetMutedMerchantPatterns)
}

func (s *InstrumentedStore) CategorizeMerchant(ctx context.Context, merchant, category, bucket string) (int, error) {
	return observe1(ctx, s, "community.categorize_merchant", func(ctx context.Context) (int, error) {
		return s.next.CategorizeMerchant(ctx, merchant, category, bucket)
	})
}

func (s *InstrumentedStore) ListRules(ctx context.Context) ([]RuleRow, error) {
	return observe1(ctx, s, "rules.list", s.next.ListRules)
}

func (s *InstrumentedStore) GetRule(ctx context.Context, id string) (*RuleRow, error) {
	return observe1(ctx, s, "rules.get", func(ctx context.Context) (*RuleRow, error) {
		return s.next.GetRule(ctx, id)
	})
}

func (s *InstrumentedStore) CreateRule(ctx context.Context, r RuleRow) (*RuleRow, error) {
	return observe1(ctx, s, "rules.create", func(ctx context.Context) (*RuleRow, error) {
		return s.next.CreateRule(ctx, r)
	})
}

func (s *InstrumentedStore) UpdateRule(ctx context.Context, id string, r RuleRow) (*RuleRow, error) {
	return observe1(ctx, s, "rules.update", func(ctx context.Context) (*RuleRow, error) {
		return s.next.UpdateRule(ctx, id, r)
	})
}

func (s *InstrumentedStore) DeleteRule(ctx context.Context, id string) error {
	return s.observe(ctx, "rules.delete", func(ctx context.Context) error {
		return s.next.DeleteRule(ctx, id)
	})
}

func (s *InstrumentedStore) SeedPredefinedRules(ctx context.Context, rules []RuleRow) error {
	return s.observe(ctx, "rules.seed_predefined", func(ctx context.Context) error {
		return s.next.SeedPredefinedRules(ctx, rules)
	})
}

func (s *InstrumentedStore) ImportUserRules(ctx context.Context, rules []RuleRow) error {
	return s.observe(ctx, "rules.import_user", func(ctx context.Context) error {
		return s.next.ImportUserRules(ctx, rules)
	})
}

func (s *InstrumentedStore) SeedMCCCodes(ctx context.Context, entries []MCCEntry) error {
	return s.observe(ctx, "community.seed_mcc_codes", func(ctx context.Context) error {
		return s.next.SeedMCCCodes(ctx, entries)
	})
}

func (s *InstrumentedStore) SeedMerchantCategories(ctx context.Context, entries []MerchantCategoryEntry) (int, error) {
	return observe1(ctx, s, "community.seed_merchant_categories", func(ctx context.Context) (int, error) {
		return s.next.SeedMerchantCategories(ctx, entries)
	})
}

func (s *InstrumentedStore) LoadCategorySnapshot(ctx context.Context) (api.CategoryResolver, error) {
	return observe1(ctx, s, "community.load_category_snapshot", s.next.LoadCategorySnapshot)
}

func (s *InstrumentedStore) SeedMCCCategories(ctx context.Context, names []string) error {
	return s.observe(ctx, "community.seed_mcc_categories", func(ctx context.Context) error {
		return s.next.SeedMCCCategories(ctx, names)
	})
}

func (s *InstrumentedStore) GetSyncStatus(ctx context.Context) (SyncStatus, error) {
	return observe1(ctx, s, "runtime.get_sync_status", s.next.GetSyncStatus)
}

func (s *InstrumentedStore) SetSyncStatus(ctx context.Context, status SyncStatus) error {
	return s.observe(ctx, "runtime.set_sync_status", func(ctx context.Context) error {
		return s.next.SetSyncStatus(ctx, status)
	})
}

func (s *InstrumentedStore) ListExtractionDiagnostics(ctx context.Context, filter DiagnosticFilter) ([]ExtractionDiagnosticRow, error) {
	return observe1(ctx, s, "diagnostics.list_extraction", func(ctx context.Context) ([]ExtractionDiagnosticRow, error) {
		return s.next.ListExtractionDiagnostics(ctx, filter)
	})
}

func (s *InstrumentedStore) GetExtractionDiagnostic(ctx context.Context, id string) (*ExtractionDiagnosticRow, error) {
	return observe1(ctx, s, "diagnostics.get_extraction", func(ctx context.Context) (*ExtractionDiagnosticRow, error) {
		return s.next.GetExtractionDiagnostic(ctx, id)
	})
}

func (s *InstrumentedStore) UpdateExtractionDiagnosticStatus(ctx context.Context, id, status string) (*ExtractionDiagnosticRow, error) {
	return observe1(ctx, s, "diagnostics.update_extraction_status", func(ctx context.Context) (*ExtractionDiagnosticRow, error) {
		return s.next.UpdateExtractionDiagnosticStatus(ctx, id, status)
	})
}

func (s *InstrumentedStore) RecordExtractionDiagnostic(ctx context.Context, diagnostic api.ExtractionDiagnostic) error {
	return s.observe(ctx, "diagnostics.record_extraction", func(ctx context.Context) error {
		return s.next.RecordExtractionDiagnostic(ctx, diagnostic)
	})
}
