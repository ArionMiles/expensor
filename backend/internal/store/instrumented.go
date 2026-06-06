package store

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/observability"
)

// InstrumentedStore records telemetry around the full store surface.
type InstrumentedStore struct {
	next  *Store
	scope *observability.Scope
}

func NewInstrumentedStore(next *Store, scope *observability.Scope, logger *slog.Logger) *InstrumentedStore {
	if logger == nil {
		logger = slog.Default()
	}
	if scope == nil {
		scope = observability.NewScope(logger, "store")
	}
	return &InstrumentedStore{next: next, scope: scope}
}

func (s *InstrumentedStore) recordOperation(ctx context.Context, name string, err error) {
	s.scope.RecordOperation(ctx, observability.Operation{
		Namespace: "store",
		Name:      name,
		Err:       err,
	})
}

func (s *InstrumentedStore) ListTransactions(ctx context.Context, f ListFilter) ([]Transaction, TransactionListResult, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.list")
	defer span.End()

	rows, result, err := s.next.ListTransactions(ctx, f)
	s.recordOperation(ctx, "transactions.list", err)
	return rows, result, err
}

func (s *InstrumentedStore) GetTransaction(ctx context.Context, id string) (*Transaction, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.get")
	defer span.End()

	transaction, err := s.next.GetTransaction(ctx, id)
	s.recordOperation(ctx, "transactions.get", err)
	return transaction, err
}

func (s *InstrumentedStore) UpdateDescription(ctx context.Context, id, description string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.update_description")
	defer span.End()

	err := s.next.UpdateDescription(ctx, id, description)
	s.recordOperation(ctx, "transactions.update_description", err)
	return err
}

func (s *InstrumentedStore) AddLabel(ctx context.Context, transactionID, label string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.add_label")
	defer span.End()

	err := s.next.AddLabel(ctx, transactionID, label)
	s.recordOperation(ctx, "transactions.add_label", err)
	return err
}

func (s *InstrumentedStore) AddLabels(ctx context.Context, transactionID string, labels []string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.add_labels")
	defer span.End()

	err := s.next.AddLabels(ctx, transactionID, labels)
	s.recordOperation(ctx, "transactions.add_labels", err)
	return err
}

func (s *InstrumentedStore) RemoveLabel(ctx context.Context, transactionID, label string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.remove_label")
	defer span.End()

	err := s.next.RemoveLabel(ctx, transactionID, label)
	s.recordOperation(ctx, "transactions.remove_label", err)
	return err
}

func (s *InstrumentedStore) SearchTransactions(ctx context.Context, query string, f ListFilter) ([]Transaction, TransactionListResult, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.search")
	defer span.End()

	rows, result, err := s.next.SearchTransactions(ctx, query, f)
	s.recordOperation(ctx, "transactions.search", err)
	return rows, result, err
}

func (s *InstrumentedStore) GetStats(ctx context.Context, baseCurrency string) (*Stats, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_stats")
	defer span.End()

	stats, err := s.next.GetStats(ctx, baseCurrency)
	s.recordOperation(ctx, "read_model.get_stats", err)
	return stats, err
}

func (s *InstrumentedStore) GetChartData(ctx context.Context) (*ChartData, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_chart_data")
	defer span.End()

	data, err := s.next.GetChartData(ctx)
	s.recordOperation(ctx, "read_model.get_chart_data", err)
	return data, err
}

func (s *InstrumentedStore) GetDashboardData(ctx context.Context) (*DashboardData, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_dashboard_data")
	defer span.End()

	data, err := s.next.GetDashboardData(ctx)
	s.recordOperation(ctx, "read_model.get_dashboard_data", err)
	return data, err
}

func (s *InstrumentedStore) GetSpendingHeatmap(ctx context.Context, from, to *time.Time) (*HeatmapData, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_spending_heatmap")
	defer span.End()

	data, err := s.next.GetSpendingHeatmap(ctx, from, to)
	s.recordOperation(ctx, "read_model.get_spending_heatmap", err)
	return data, err
}

func (s *InstrumentedStore) GetAnnualSpend(ctx context.Context, year int) ([]DailyBucket, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_annual_spend")
	defer span.End()

	buckets, err := s.next.GetAnnualSpend(ctx, year)
	s.recordOperation(ctx, "read_model.get_annual_spend", err)
	return buckets, err
}

func (s *InstrumentedStore) GetAppConfig(ctx context.Context, key string) (string, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_app_config")
	defer span.End()

	value, err := s.next.GetAppConfig(ctx, key)
	s.recordOperation(ctx, "runtime.get_app_config", err)
	return value, err
}

func (s *InstrumentedStore) SetAppConfig(ctx context.Context, key, value string) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_app_config")
	defer span.End()

	err := s.next.SetAppConfig(ctx, key, value)
	s.recordOperation(ctx, "runtime.set_app_config", err)
	return err
}

func (s *InstrumentedStore) IsMessageProcessed(ctx context.Context, key string) (bool, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.is_message_processed")
	defer span.End()

	processed, err := s.next.IsMessageProcessed(ctx, key)
	s.recordOperation(ctx, "runtime.is_message_processed", err)
	return processed, err
}

func (s *InstrumentedStore) MarkMessageProcessed(ctx context.Context, key string, at time.Time) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.mark_message_processed")
	defer span.End()

	err := s.next.MarkMessageProcessed(ctx, key, at)
	s.recordOperation(ctx, "runtime.mark_message_processed", err)
	return err
}

func (s *InstrumentedStore) SetActiveReader(ctx context.Context, reader string) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_active_reader")
	defer span.End()

	err := s.next.SetActiveReader(ctx, reader)
	s.recordOperation(ctx, "runtime.set_active_reader", err)
	return err
}

func (s *InstrumentedStore) GetActiveReader(ctx context.Context) (string, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_active_reader")
	defer span.End()

	reader, err := s.next.GetActiveReader(ctx)
	s.recordOperation(ctx, "runtime.get_active_reader", err)
	return reader, err
}

func (s *InstrumentedStore) SetReaderSecret(ctx context.Context, reader string, secret []byte) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_reader_secret")
	defer span.End()

	err := s.next.SetReaderSecret(ctx, reader, secret)
	s.recordOperation(ctx, "runtime.set_reader_secret", err)
	return err
}

func (s *InstrumentedStore) GetReaderSecret(ctx context.Context, reader string) (secret []byte, found bool, err error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_reader_secret")
	defer span.End()

	secret, found, err = s.next.GetReaderSecret(ctx, reader)
	s.recordOperation(ctx, "runtime.get_reader_secret", err)
	return secret, found, err
}

func (s *InstrumentedStore) SetReaderToken(ctx context.Context, reader string, token []byte) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_reader_token")
	defer span.End()

	err := s.next.SetReaderToken(ctx, reader, token)
	s.recordOperation(ctx, "runtime.set_reader_token", err)
	return err
}

func (s *InstrumentedStore) GetReaderToken(ctx context.Context, reader string) (token []byte, found bool, err error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_reader_token")
	defer span.End()

	token, found, err = s.next.GetReaderToken(ctx, reader)
	s.recordOperation(ctx, "runtime.get_reader_token", err)
	return token, found, err
}

func (s *InstrumentedStore) DeleteReaderToken(ctx context.Context, reader string) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.delete_reader_token")
	defer span.End()

	err := s.next.DeleteReaderToken(ctx, reader)
	s.recordOperation(ctx, "runtime.delete_reader_token", err)
	return err
}

func (s *InstrumentedStore) SetReaderConfig(ctx context.Context, reader string, config json.RawMessage) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_reader_config")
	defer span.End()

	err := s.next.SetReaderConfig(ctx, reader, config)
	s.recordOperation(ctx, "runtime.set_reader_config", err)
	return err
}

func (s *InstrumentedStore) GetReaderConfig(ctx context.Context, reader string) (json.RawMessage, bool, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_reader_config")
	defer span.End()

	config, found, err := s.next.GetReaderConfig(ctx, reader)
	s.recordOperation(ctx, "runtime.get_reader_config", err)
	return config, found, err
}

func (s *InstrumentedStore) DeleteReaderRuntime(ctx context.Context, reader string) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.delete_reader_runtime")
	defer span.End()

	err := s.next.DeleteReaderRuntime(ctx, reader)
	s.recordOperation(ctx, "runtime.delete_reader_runtime", err)
	return err
}

func (s *InstrumentedStore) GetCommunityURL(ctx context.Context) (string, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_community_url")
	defer span.End()

	url, err := s.next.GetCommunityURL(ctx)
	s.recordOperation(ctx, "runtime.get_community_url", err)
	return url, err
}

func (s *InstrumentedStore) SetCommunityURL(ctx context.Context, url string) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_community_url")
	defer span.End()

	err := s.next.SetCommunityURL(ctx, url)
	s.recordOperation(ctx, "runtime.set_community_url", err)
	return err
}

func (s *InstrumentedStore) GetFacets(ctx context.Context) (*Facets, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.get_facets")
	defer span.End()

	facets, err := s.next.GetFacets(ctx)
	s.recordOperation(ctx, "transactions.get_facets", err)
	return facets, err
}

func (s *InstrumentedStore) ListLabels(ctx context.Context) ([]Label, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.list_labels")
	defer span.End()

	labels, err := s.next.ListLabels(ctx)
	s.recordOperation(ctx, "taxonomy.list_labels", err)
	return labels, err
}

func (s *InstrumentedStore) CreateLabel(ctx context.Context, name, color string) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.create_label")
	defer span.End()

	err := s.next.CreateLabel(ctx, name, color)
	s.recordOperation(ctx, "taxonomy.create_label", err)
	return err
}

func (s *InstrumentedStore) UpdateLabel(ctx context.Context, name, color string) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.update_label")
	defer span.End()

	err := s.next.UpdateLabel(ctx, name, color)
	s.recordOperation(ctx, "taxonomy.update_label", err)
	return err
}

func (s *InstrumentedStore) DeleteLabel(ctx context.Context, name string, removeFromTransactions bool) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.delete_label")
	defer span.End()

	err := s.next.DeleteLabel(ctx, name, removeFromTransactions)
	s.recordOperation(ctx, "taxonomy.delete_label", err)
	return err
}

func (s *InstrumentedStore) ApplyLabelByMerchant(ctx context.Context, label, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.apply_label_by_merchant")
	defer span.End()

	affected, err := s.next.ApplyLabelByMerchant(ctx, label, pattern)
	s.recordOperation(ctx, "taxonomy.apply_label_by_merchant", err)
	return affected, err
}

func (s *InstrumentedStore) RemoveLabelByMerchant(ctx context.Context, label, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.remove_label_by_merchant")
	defer span.End()

	removed, err := s.next.RemoveLabelByMerchant(ctx, label, pattern)
	s.recordOperation(ctx, "taxonomy.remove_label_by_merchant", err)
	return removed, err
}

func (s *InstrumentedStore) GetLabelMappings(ctx context.Context) (map[string][]string, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.get_label_mappings")
	defer span.End()

	mappings, err := s.next.GetLabelMappings(ctx)
	s.recordOperation(ctx, "taxonomy.get_label_mappings", err)
	return mappings, err
}

func (s *InstrumentedStore) GetMonthlyBreakdownSpend(ctx context.Context, dimension string, months int) (*MonthlyBreakdownData, error) {
	ctx, span := s.scope.Start(ctx, "store.read_model.get_monthly_breakdown_spend")
	defer span.End()

	data, err := s.next.GetMonthlyBreakdownSpend(ctx, dimension, months)
	s.recordOperation(ctx, "read_model.get_monthly_breakdown_spend", err)
	return data, err
}

func (s *InstrumentedStore) ListCategories(ctx context.Context) ([]Category, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.list_categories")
	defer span.End()

	categories, err := s.next.ListCategories(ctx)
	s.recordOperation(ctx, "taxonomy.list_categories", err)
	return categories, err
}

func (s *InstrumentedStore) CreateCategory(ctx context.Context, name, description string) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.create_category")
	defer span.End()

	err := s.next.CreateCategory(ctx, name, description)
	s.recordOperation(ctx, "taxonomy.create_category", err)
	return err
}

func (s *InstrumentedStore) DeleteCategory(ctx context.Context, name string, removeFromTransactions bool) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.delete_category")
	defer span.End()

	err := s.next.DeleteCategory(ctx, name, removeFromTransactions)
	s.recordOperation(ctx, "taxonomy.delete_category", err)
	return err
}

func (s *InstrumentedStore) ApplyCategoryByMerchant(ctx context.Context, category, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.apply_category_by_merchant")
	defer span.End()

	affected, err := s.next.ApplyCategoryByMerchant(ctx, category, pattern)
	s.recordOperation(ctx, "taxonomy.apply_category_by_merchant", err)
	return affected, err
}

func (s *InstrumentedStore) RemoveCategoryByMerchant(ctx context.Context, category, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.remove_category_by_merchant")
	defer span.End()

	removed, err := s.next.RemoveCategoryByMerchant(ctx, category, pattern)
	s.recordOperation(ctx, "taxonomy.remove_category_by_merchant", err)
	return removed, err
}

func (s *InstrumentedStore) GetCategoryMappings(ctx context.Context) (map[string][]string, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.get_category_mappings")
	defer span.End()

	mappings, err := s.next.GetCategoryMappings(ctx)
	s.recordOperation(ctx, "taxonomy.get_category_mappings", err)
	return mappings, err
}

func (s *InstrumentedStore) ListBuckets(ctx context.Context) ([]Bucket, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.list_buckets")
	defer span.End()

	buckets, err := s.next.ListBuckets(ctx)
	s.recordOperation(ctx, "taxonomy.list_buckets", err)
	return buckets, err
}

func (s *InstrumentedStore) CreateBucket(ctx context.Context, name, description string) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.create_bucket")
	defer span.End()

	err := s.next.CreateBucket(ctx, name, description)
	s.recordOperation(ctx, "taxonomy.create_bucket", err)
	return err
}

func (s *InstrumentedStore) DeleteBucket(ctx context.Context, name string, removeFromTransactions bool) error {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.delete_bucket")
	defer span.End()

	err := s.next.DeleteBucket(ctx, name, removeFromTransactions)
	s.recordOperation(ctx, "taxonomy.delete_bucket", err)
	return err
}

func (s *InstrumentedStore) ApplyBucketByMerchant(ctx context.Context, bucket, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.apply_bucket_by_merchant")
	defer span.End()

	affected, err := s.next.ApplyBucketByMerchant(ctx, bucket, pattern)
	s.recordOperation(ctx, "taxonomy.apply_bucket_by_merchant", err)
	return affected, err
}

func (s *InstrumentedStore) RemoveBucketByMerchant(ctx context.Context, bucket, pattern string) (int64, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.remove_bucket_by_merchant")
	defer span.End()

	removed, err := s.next.RemoveBucketByMerchant(ctx, bucket, pattern)
	s.recordOperation(ctx, "taxonomy.remove_bucket_by_merchant", err)
	return removed, err
}

func (s *InstrumentedStore) GetBucketMappings(ctx context.Context) (map[string][]string, error) {
	ctx, span := s.scope.Start(ctx, "store.taxonomy.get_bucket_mappings")
	defer span.End()

	mappings, err := s.next.GetBucketMappings(ctx)
	s.recordOperation(ctx, "taxonomy.get_bucket_mappings", err)
	return mappings, err
}

func (s *InstrumentedStore) UpdateTransaction(ctx context.Context, id string, u TransactionUpdate) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.update")
	defer span.End()

	err := s.next.UpdateTransaction(ctx, id, u)
	s.recordOperation(ctx, "transactions.update", err)
	return err
}

func (s *InstrumentedStore) MuteTransaction(ctx context.Context, id string, muted bool, reason string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.mute")
	defer span.End()

	err := s.next.MuteTransaction(ctx, id, muted, reason)
	s.recordOperation(ctx, "transactions.mute", err)
	return err
}

func (s *InstrumentedStore) UpdateMuteReason(ctx context.Context, id, reason string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.update_mute_reason")
	defer span.End()

	err := s.next.UpdateMuteReason(ctx, id, reason)
	s.recordOperation(ctx, "transactions.update_mute_reason", err)
	return err
}

func (s *InstrumentedStore) UpdateMerchantReason(ctx context.Context, id, reason string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.update_merchant_reason")
	defer span.End()

	err := s.next.UpdateMerchantReason(ctx, id, reason)
	s.recordOperation(ctx, "transactions.update_merchant_reason", err)
	return err
}

func (s *InstrumentedStore) MuteByMerchant(ctx context.Context, pattern, reason string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.mute_by_merchant")
	defer span.End()

	err := s.next.MuteByMerchant(ctx, pattern, reason)
	s.recordOperation(ctx, "transactions.mute_by_merchant", err)
	return err
}

func (s *InstrumentedStore) ListMutedMerchants(ctx context.Context) ([]MutedMerchant, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.list_muted_merchants")
	defer span.End()

	merchants, err := s.next.ListMutedMerchants(ctx)
	s.recordOperation(ctx, "transactions.list_muted_merchants", err)
	return merchants, err
}

func (s *InstrumentedStore) GetMutedMerchantsWithCount(ctx context.Context) ([]MutedMerchantWithCount, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.get_muted_merchants_with_count")
	defer span.End()

	merchants, err := s.next.GetMutedMerchantsWithCount(ctx)
	s.recordOperation(ctx, "transactions.get_muted_merchants_with_count", err)
	return merchants, err
}

func (s *InstrumentedStore) DeleteMutedMerchant(ctx context.Context, id string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.delete_muted_merchant")
	defer span.End()

	err := s.next.DeleteMutedMerchant(ctx, id)
	s.recordOperation(ctx, "transactions.delete_muted_merchant", err)
	return err
}

func (s *InstrumentedStore) UnmuteByPattern(ctx context.Context, pattern string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.unmute_by_pattern")
	defer span.End()

	err := s.next.UnmuteByPattern(ctx, pattern)
	s.recordOperation(ctx, "transactions.unmute_by_pattern", err)
	return err
}

func (s *InstrumentedStore) DeleteMutedMerchantAndUnmute(ctx context.Context, id string) error {
	ctx, span := s.scope.Start(ctx, "store.transactions.delete_muted_merchant_and_unmute")
	defer span.End()

	err := s.next.DeleteMutedMerchantAndUnmute(ctx, id)
	s.recordOperation(ctx, "transactions.delete_muted_merchant_and_unmute", err)
	return err
}

func (s *InstrumentedStore) GetMutedMerchantPatterns(ctx context.Context) ([]string, error) {
	ctx, span := s.scope.Start(ctx, "store.transactions.get_muted_merchant_patterns")
	defer span.End()

	patterns, err := s.next.GetMutedMerchantPatterns(ctx)
	s.recordOperation(ctx, "transactions.get_muted_merchant_patterns", err)
	return patterns, err
}

func (s *InstrumentedStore) CategorizeMerchant(ctx context.Context, merchant, category, bucket string) (int, error) {
	ctx, span := s.scope.Start(ctx, "store.community.categorize_merchant")
	defer span.End()

	updated, err := s.next.CategorizeMerchant(ctx, merchant, category, bucket)
	s.recordOperation(ctx, "community.categorize_merchant", err)
	return updated, err
}

func (s *InstrumentedStore) ListRules(ctx context.Context) ([]RuleRow, error) {
	ctx, span := s.scope.Start(ctx, "store.rules.list")
	defer span.End()

	rules, err := s.next.ListRules(ctx)
	s.recordOperation(ctx, "rules.list", err)
	return rules, err
}

func (s *InstrumentedStore) GetRule(ctx context.Context, id string) (*RuleRow, error) {
	ctx, span := s.scope.Start(ctx, "store.rules.get")
	defer span.End()

	rule, err := s.next.GetRule(ctx, id)
	s.recordOperation(ctx, "rules.get", err)
	return rule, err
}

func (s *InstrumentedStore) CreateRule(ctx context.Context, r RuleRow) (*RuleRow, error) {
	ctx, span := s.scope.Start(ctx, "store.rules.create")
	defer span.End()

	rule, err := s.next.CreateRule(ctx, r)
	s.recordOperation(ctx, "rules.create", err)
	return rule, err
}

func (s *InstrumentedStore) UpdateRule(ctx context.Context, id string, r RuleRow) (*RuleRow, error) {
	ctx, span := s.scope.Start(ctx, "store.rules.update")
	defer span.End()

	rule, err := s.next.UpdateRule(ctx, id, r)
	s.recordOperation(ctx, "rules.update", err)
	return rule, err
}

func (s *InstrumentedStore) DeleteRule(ctx context.Context, id string) error {
	ctx, span := s.scope.Start(ctx, "store.rules.delete")
	defer span.End()

	err := s.next.DeleteRule(ctx, id)
	s.recordOperation(ctx, "rules.delete", err)
	return err
}

func (s *InstrumentedStore) SeedPredefinedRules(ctx context.Context, rules []RuleRow) error {
	ctx, span := s.scope.Start(ctx, "store.rules.seed_predefined")
	defer span.End()

	err := s.next.SeedPredefinedRules(ctx, rules)
	s.recordOperation(ctx, "rules.seed_predefined", err)
	return err
}

func (s *InstrumentedStore) ImportUserRules(ctx context.Context, rules []RuleRow) error {
	ctx, span := s.scope.Start(ctx, "store.rules.import_user")
	defer span.End()

	err := s.next.ImportUserRules(ctx, rules)
	s.recordOperation(ctx, "rules.import_user", err)
	return err
}

func (s *InstrumentedStore) SeedMCCCodes(ctx context.Context, entries []MCCEntry) error {
	ctx, span := s.scope.Start(ctx, "store.community.seed_mcc_codes")
	defer span.End()

	err := s.next.SeedMCCCodes(ctx, entries)
	s.recordOperation(ctx, "community.seed_mcc_codes", err)
	return err
}

func (s *InstrumentedStore) SeedMerchantCategories(ctx context.Context, entries []MerchantCategoryEntry) (int, error) {
	ctx, span := s.scope.Start(ctx, "store.community.seed_merchant_categories")
	defer span.End()

	updated, err := s.next.SeedMerchantCategories(ctx, entries)
	s.recordOperation(ctx, "community.seed_merchant_categories", err)
	return updated, err
}

func (s *InstrumentedStore) LoadCategorySnapshot(ctx context.Context) (api.CategoryResolver, error) {
	ctx, span := s.scope.Start(ctx, "store.community.load_category_snapshot")
	defer span.End()

	resolver, err := s.next.LoadCategorySnapshot(ctx)
	s.recordOperation(ctx, "community.load_category_snapshot", err)
	return resolver, err
}

func (s *InstrumentedStore) SeedMCCCategories(ctx context.Context, names []string) error {
	ctx, span := s.scope.Start(ctx, "store.community.seed_mcc_categories")
	defer span.End()

	err := s.next.SeedMCCCategories(ctx, names)
	s.recordOperation(ctx, "community.seed_mcc_categories", err)
	return err
}

func (s *InstrumentedStore) GetSyncStatus(ctx context.Context) (SyncStatus, error) {
	ctx, span := s.scope.Start(ctx, "store.runtime.get_sync_status")
	defer span.End()

	status, err := s.next.GetSyncStatus(ctx)
	s.recordOperation(ctx, "runtime.get_sync_status", err)
	return status, err
}

func (s *InstrumentedStore) SetSyncStatus(ctx context.Context, status SyncStatus) error {
	ctx, span := s.scope.Start(ctx, "store.runtime.set_sync_status")
	defer span.End()

	err := s.next.SetSyncStatus(ctx, status)
	s.recordOperation(ctx, "runtime.set_sync_status", err)
	return err
}

func (s *InstrumentedStore) ListExtractionDiagnostics(ctx context.Context, filter DiagnosticFilter) ([]ExtractionDiagnosticRow, error) {
	ctx, span := s.scope.Start(ctx, "store.diagnostics.list_extraction")
	defer span.End()

	rows, err := s.next.ListExtractionDiagnostics(ctx, filter)
	s.recordOperation(ctx, "diagnostics.list_extraction", err)
	return rows, err
}

func (s *InstrumentedStore) GetExtractionDiagnostic(ctx context.Context, id string) (*ExtractionDiagnosticRow, error) {
	ctx, span := s.scope.Start(ctx, "store.diagnostics.get_extraction")
	defer span.End()

	row, err := s.next.GetExtractionDiagnostic(ctx, id)
	s.recordOperation(ctx, "diagnostics.get_extraction", err)
	return row, err
}

func (s *InstrumentedStore) UpdateExtractionDiagnosticStatus(ctx context.Context, id, status string) (*ExtractionDiagnosticRow, error) {
	ctx, span := s.scope.Start(ctx, "store.diagnostics.update_extraction_status")
	defer span.End()

	row, err := s.next.UpdateExtractionDiagnosticStatus(ctx, id, status)
	s.recordOperation(ctx, "diagnostics.update_extraction_status", err)
	return row, err
}

func (s *InstrumentedStore) RecordExtractionDiagnostic(ctx context.Context, diagnostic api.ExtractionDiagnostic) error {
	ctx, span := s.scope.Start(ctx, "store.diagnostics.record_extraction")
	defer span.End()

	err := s.next.RecordExtractionDiagnostic(ctx, diagnostic)
	s.recordOperation(ctx, "diagnostics.record_extraction", err)
	return err
}
