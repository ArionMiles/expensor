// Package storetest provides backend-neutral conformance tests for store.Backend.
package storetest

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
)

// Run exercises the backend-neutral store contract. The supplied backend must
// be connected to a freshly migrated empty database.
func Run(t *testing.T, backend store.Backend) {
	t.Helper()

	ctx := context.Background()

	t.Run("Health", func(t *testing.T) { testHealth(ctx, t, backend) })
	t.Run("Auth", func(t *testing.T) { testAuth(ctx, t, backend) })
	t.Run("Runtime", func(t *testing.T) { testRuntime(ctx, t, backend) })
	t.Run("Scanning", func(t *testing.T) { testScanning(ctx, t, backend) })
	t.Run("Taxonomy", func(t *testing.T) { testTaxonomy(ctx, t, backend) })
	t.Run("Community", func(t *testing.T) { testCommunity(ctx, t, backend) })
	t.Run("Rules", func(t *testing.T) { testRules(ctx, t, backend) })
	t.Run("Ingestion", func(t *testing.T) { testIngestion(ctx, t, backend) })
	t.Run("Diagnostics", func(t *testing.T) { testDiagnostics(ctx, t, backend) })
}

func testHealth(ctx context.Context, t *testing.T, backend store.Backend) {
	t.Helper()

	if err := backend.HealthCheck(ctx); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
}

//nolint:gocognit // Conformance subtests intentionally exercise a broad backend contract in one scenario.
func testAuth(ctx context.Context, t *testing.T, backend store.Backend) {
	t.Helper()

	required, err := backend.BootstrapRequired(ctx)
	if err != nil {
		t.Fatalf("BootstrapRequired before admin: %v", err)
	}
	if !required {
		t.Fatal("BootstrapRequired before admin = false, want true")
	}

	admin, err := backend.CreateBootstrapAdmin(ctx, store.CreateBootstrapAdminInput{
		Email:        email(t, "admin"),
		DisplayName:  "Conformance Admin",
		PasswordHash: "hash-admin",
		AvatarKey:    "avatar-admin",
	})
	if err != nil {
		t.Fatalf("CreateBootstrapAdmin: %v", err)
	}
	if admin.ID == "" || admin.TenantID == "" || admin.Role != store.UserRoleAdmin {
		t.Fatalf("CreateBootstrapAdmin returned invalid user: %#v", admin)
	}

	required, err = backend.BootstrapRequired(ctx)
	if err != nil {
		t.Fatalf("BootstrapRequired after admin: %v", err)
	}
	if required {
		t.Fatal("BootstrapRequired after admin = true, want false")
	}

	user, err := backend.CreateUser(ctx, store.CreateUserInput{
		Email:        email(t, "user"),
		DisplayName:  "Conformance User",
		Role:         store.UserRoleUser,
		AvatarKey:    "avatar-user",
		PasswordHash: "hash-user",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.ID == "" || user.TenantID == "" || user.Role != store.UserRoleUser {
		t.Fatalf("CreateUser returned invalid user: %#v", user)
	}

	found, err := backend.FindUserByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("FindUserByEmail: %v", err)
	}
	if found.ID != user.ID {
		t.Fatalf("FindUserByEmail ID = %q, want %q", found.ID, user.ID)
	}
	foundByID, err := backend.FindUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindUserByID: %v", err)
	}
	if foundByID.Email != user.Email {
		t.Fatalf("FindUserByID Email = %q, want %q", foundByID.Email, user.Email)
	}
	users, err := backend.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) < 2 {
		t.Fatalf("ListUsers len = %d, want at least 2", len(users))
	}

	displayName := "Updated Conformance User"
	updated, err := backend.UpdateUser(ctx, user.ID, store.UpdateUserInput{DisplayName: &displayName})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if updated.DisplayName != displayName {
		t.Fatalf("UpdateUser DisplayName = %q, want %q", updated.DisplayName, displayName)
	}

	if err := backend.UpdateUserPassword(ctx, user.ID, store.UpdateUserPasswordInput{PasswordHash: "hash-updated"}); err != nil {
		t.Fatalf("UpdateUserPassword: %v", err)
	}

	session, err := backend.CreateSession(ctx, store.CreateSessionInput{
		UserID:    user.ID,
		TokenHash: "session-" + suffix(t),
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, err := backend.FindSessionByHash(ctx, session.TokenHash); err != nil {
		t.Fatalf("FindSessionByHash: %v", err)
	}
	if err := backend.RevokeSession(ctx, session.ID); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}

	accessToken, err := backend.CreateAccessToken(ctx, store.CreateAccessTokenInput{
		UserID:    user.ID,
		Name:      "conformance-token",
		TokenHash: "access-" + suffix(t),
	})
	if err != nil {
		t.Fatalf("CreateAccessToken: %v", err)
	}
	tokens, err := backend.ListAccessTokens(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListAccessTokens: %v", err)
	}
	if len(tokens) == 0 {
		t.Fatal("ListAccessTokens returned no tokens")
	}
	if _, err := backend.FindAccessTokenByHash(ctx, accessToken.TokenHash); err != nil {
		t.Fatalf("FindAccessTokenByHash: %v", err)
	}
	if err := backend.RevokeAccessToken(ctx, accessToken.ID, user.ID); err != nil {
		t.Fatalf("RevokeAccessToken: %v", err)
	}

	setupToken, err := backend.CreateAccountSetupToken(ctx, store.CreateAccountSetupTokenInput{
		UserID:    user.ID,
		TokenHash: "setup-" + suffix(t),
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateAccountSetupToken: %v", err)
	}
	if _, err := backend.FindAccountSetupTokenByHash(ctx, setupToken.TokenHash); err != nil {
		t.Fatalf("FindAccountSetupTokenByHash: %v", err)
	}
	completed, err := backend.CompleteAccountSetup(ctx, store.CompleteAccountSetupInput{
		TokenHash:    setupToken.TokenHash,
		PasswordHash: "hash-completed",
		DisplayName:  "Completed Conformance User",
		AvatarKey:    "avatar-completed",
	})
	if err != nil {
		t.Fatalf("CompleteAccountSetup: %v", err)
	}
	if completed.DisplayName != "Completed Conformance User" {
		t.Fatalf("CompleteAccountSetup DisplayName = %q, want completed display name", completed.DisplayName)
	}

	usedToken, err := backend.CreateAccountSetupToken(ctx, store.CreateAccountSetupTokenInput{
		UserID:    user.ID,
		TokenHash: "setup-used-" + suffix(t),
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateAccountSetupToken for MarkAccountSetupTokenUsed: %v", err)
	}
	if err := backend.MarkAccountSetupTokenUsed(ctx, usedToken.ID); err != nil {
		t.Fatalf("MarkAccountSetupTokenUsed: %v", err)
	}
}

//nolint:gocognit // Conformance subtests intentionally exercise a broad backend contract in one scenario.
func testRuntime(ctx context.Context, t *testing.T, backend store.Backend) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "runtime")

	if err := backend.SetAppConfig(ctx, tenant, "base_currency", "INR"); err != nil {
		t.Fatalf("SetAppConfig: %v", err)
	}
	value, err := backend.GetAppConfig(ctx, tenant, "base_currency")
	if err != nil {
		t.Fatalf("GetAppConfig: %v", err)
	}
	if value != "INR" {
		t.Fatalf("GetAppConfig = %q, want INR", value)
	}

	if err := backend.MarkMessageProcessed(ctx, tenant, "msg-runtime", time.Now()); err != nil {
		t.Fatalf("MarkMessageProcessed: %v", err)
	}
	processed, err := backend.IsMessageProcessed(ctx, tenant, "msg-runtime")
	if err != nil {
		t.Fatalf("IsMessageProcessed: %v", err)
	}
	if !processed {
		t.Fatal("IsMessageProcessed = false, want true")
	}

	readerConfig := json.RawMessage(`{"mailboxes":["Inbox"]}`)
	if err := backend.SetReaderConfig(ctx, tenant, "thunderbird", readerConfig); err != nil {
		t.Fatalf("SetReaderConfig: %v", err)
	}
	gotReaderConfig, found, err := backend.GetReaderConfig(ctx, tenant, "thunderbird")
	if err != nil || !found {
		t.Fatalf("GetReaderConfig found=%v err=%v", found, err)
	}
	assertJSON(t, gotReaderConfig, readerConfig)

	if err := backend.SetReaderSecret(ctx, tenant, "gmail", []byte(`{"installed":{}}`)); err != nil {
		t.Fatalf("SetReaderSecret: %v", err)
	}
	if secret, secretFound, err := backend.GetReaderSecret(ctx, tenant, "gmail"); err != nil || !secretFound || string(secret) != `{"installed":{}}` {
		t.Fatalf("GetReaderSecret secret=%q found=%v err=%v", secret, secretFound, err)
	}
	if err := backend.SetReaderToken(ctx, tenant, "gmail", []byte(`{"access_token":"token"}`)); err != nil {
		t.Fatalf("SetReaderToken: %v", err)
	}
	if token, tokenFound, err := backend.GetReaderToken(ctx, tenant, "gmail"); err != nil || !tokenFound || string(token) != `{"access_token":"token"}` {
		t.Fatalf("GetReaderToken token=%q found=%v err=%v", token, tokenFound, err)
	}
	if err := backend.DeleteReaderToken(ctx, tenant, "gmail"); err != nil {
		t.Fatalf("DeleteReaderToken: %v", err)
	}
	if _, tokenFound, err := backend.GetReaderToken(ctx, tenant, "gmail"); err != nil || tokenFound {
		t.Fatalf("GetReaderToken after delete found=%v err=%v", tokenFound, err)
	}

	llmConfig := json.RawMessage(`{"model":"test"}`)
	if err := backend.SetLLMProviderConfig(ctx, tenant, "openai", llmConfig); err != nil {
		t.Fatalf("SetLLMProviderConfig: %v", err)
	}
	if err := backend.SetLLMProviderCredentials(ctx, tenant, "openai", []byte(`{"api_key":"test"}`)); err != nil {
		t.Fatalf("SetLLMProviderCredentials: %v", err)
	}
	if err := backend.SetActiveLLMProvider(ctx, tenant, "openai"); err != nil {
		t.Fatalf("SetActiveLLMProvider: %v", err)
	}
	runtime, found, err := backend.GetActiveLLMProviderRuntime(ctx, tenant)
	if err != nil || !found {
		t.Fatalf("GetActiveLLMProviderRuntime found=%v err=%v", found, err)
	}
	if runtime.Provider != "openai" || !runtime.Active || !runtime.HasCredentials {
		t.Fatalf("GetActiveLLMProviderRuntime returned invalid runtime: %#v", runtime)
	}
	assertJSON(t, runtime.Config, llmConfig)

	if err := backend.SetCommunityURL(ctx, "https://example.test/community"); err != nil {
		t.Fatalf("SetCommunityURL: %v", err)
	}
	if url, err := backend.GetCommunityURL(ctx); err != nil || url != "https://example.test/community" {
		t.Fatalf("GetCommunityURL = %q / %v", url, err)
	}

	enabled := false
	settings, err := backend.PatchCommunitySyncSettings(ctx, store.CommunitySyncSettingsPatch{AutomaticSyncEnabled: &enabled})
	if err != nil {
		t.Fatalf("PatchCommunitySyncSettings: %v", err)
	}
	if settings.AutomaticSyncEnabled == nil || *settings.AutomaticSyncEnabled {
		t.Fatalf("PatchCommunitySyncSettings = %#v, want false", settings)
	}
	if err := backend.SetSyncStatus(ctx, store.SyncStatus{EntriesUpdated: 3}); err != nil {
		t.Fatalf("SetSyncStatus: %v", err)
	}
	status, err := backend.GetSyncStatus(ctx)
	if err != nil {
		t.Fatalf("GetSyncStatus: %v", err)
	}
	if status.EntriesUpdated != 3 {
		t.Fatalf("GetSyncStatus EntriesUpdated = %d, want 3", status.EntriesUpdated)
	}
}

func testScanning(ctx context.Context, t *testing.T, backend store.Backend) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "scanning")

	state, err := backend.GetScanningState(ctx, tenant)
	if err != nil {
		t.Fatalf("GetScanningState: %v", err)
	}
	if state.State != store.ScanningStateStopped || !state.Enabled {
		t.Fatalf("initial scanning state = %#v", state)
	}

	if err := backend.SetActiveScanningReader(ctx, tenant, "gmail"); err != nil {
		t.Fatalf("SetActiveScanningReader: %v", err)
	}
	state, err = backend.GetScanningState(ctx, tenant)
	if err != nil {
		t.Fatalf("GetScanningState after reader: %v", err)
	}
	if state.ActiveReader != "gmail" || state.State != store.ScanningStateQueued {
		t.Fatalf("scanning state after reader = %#v", state)
	}

	retryCount := 2
	nextRetry := time.Now().Add(time.Minute).UTC().Truncate(time.Microsecond)
	if err := backend.UpdateScanningState(ctx, tenant, store.ScanningStateUpdate{
		State:         store.ScanningStateBackingOff,
		ReasonCode:    store.ScanningReasonTemporaryFailure,
		PublicMessage: "temporary failure",
		NextRetryAt:   &nextRetry,
		RetryCount:    &retryCount,
	}); err != nil {
		t.Fatalf("UpdateScanningState: %v", err)
	}
	state, err = backend.GetScanningState(ctx, tenant)
	if err != nil {
		t.Fatalf("GetScanningState after update: %v", err)
	}
	if state.State != store.ScanningStateBackingOff || state.RetryCount != retryCount || state.NextRetryAt == nil {
		t.Fatalf("scanning state after update = %#v", state)
	}

	if err := backend.SetScanningEnabled(ctx, tenant, false); err != nil {
		t.Fatalf("SetScanningEnabled false: %v", err)
	}
	state, err = backend.GetScanningState(ctx, tenant)
	if err != nil {
		t.Fatalf("GetScanningState after disable: %v", err)
	}
	if state.Enabled || state.State != store.ScanningStatePaused {
		t.Fatalf("disabled scanning state = %#v", state)
	}

	cfg, err := backend.PatchSchedulerConfig(ctx, store.SchedulerConfigPatch{MaxConcurrentScans: intPtr(3)})
	if err != nil {
		t.Fatalf("PatchSchedulerConfig: %v", err)
	}
	if cfg.MaxConcurrentScans != 3 {
		t.Fatalf("MaxConcurrentScans = %d, want 3", cfg.MaxConcurrentScans)
	}
}

//nolint:gocognit // Conformance subtests intentionally exercise a broad backend contract in one scenario.
func testTaxonomy(ctx context.Context, t *testing.T, backend store.Backend) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "taxonomy")
	label := "conf-label-" + suffix(t)
	category := "Conf Category " + suffix(t)
	bucket := "Conf Bucket " + suffix(t)

	if err := backend.CreateLabel(ctx, tenant, label, "#38bdf8"); err != nil {
		t.Fatalf("CreateLabel: %v", err)
	}
	if err := backend.UpdateLabel(ctx, tenant, label, "#0ea5e9"); err != nil {
		t.Fatalf("UpdateLabel: %v", err)
	}
	labels, err := backend.ListLabels(ctx, tenant)
	if err != nil {
		t.Fatalf("ListLabels: %v", err)
	}
	if !hasLabel(labels, label, "#0ea5e9") {
		t.Fatalf("ListLabels missing updated label %q in %#v", label, labels)
	}

	if err := backend.CreateCategory(ctx, tenant, category, "conformance category"); err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	categories, err := backend.ListCategories(ctx, tenant)
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	if !hasCategory(categories, category) {
		t.Fatalf("ListCategories missing category %q in %#v", category, categories)
	}

	if err := backend.CreateBucket(ctx, tenant, bucket, "conformance bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	buckets, err := backend.ListBuckets(ctx, tenant)
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	if !hasBucket(buckets, bucket) {
		t.Fatalf("ListBuckets missing bucket %q in %#v", bucket, buckets)
	}

	merchantName := "Taxonomy Merchant " + suffix(t)
	if err := backend.Write(ctx, store.IngestionBatch{
		Tenant: tenant,
		Transactions: []*api.TransactionDetails{{
			MessageID:    "taxonomy-" + suffix(t),
			Amount:       42,
			Currency:     "INR",
			Timestamp:    time.Now().UTC().Format(time.RFC3339),
			MerchantInfo: merchantName,
			Source:       api.Source{Type: "credit-card", Label: "Taxonomy Card", Bank: "Example"},
		}},
	}); err != nil {
		t.Fatalf("Write taxonomy transaction: %v", err)
	}
	if affected, err := backend.ApplyLabelByMerchant(ctx, tenant, label, merchantName); err != nil || affected != 1 {
		t.Fatalf("ApplyLabelByMerchant affected=%d err=%v, want 1 nil", affected, err)
	}
	labelMappings, err := backend.GetLabelMappings(ctx, tenant)
	if err != nil {
		t.Fatalf("GetLabelMappings: %v", err)
	}
	if !mappingContains(labelMappings, label, merchantName) {
		t.Fatalf("GetLabelMappings missing %q -> %q in %#v", label, merchantName, labelMappings)
	}
	if affected, err := backend.RemoveLabelByMerchant(ctx, tenant, label, merchantName); err != nil || affected != 1 {
		t.Fatalf("RemoveLabelByMerchant affected=%d err=%v, want 1 nil", affected, err)
	}
	if affected, err := backend.ApplyCategoryByMerchant(ctx, tenant, category, merchantName); err != nil || affected != 1 {
		t.Fatalf("ApplyCategoryByMerchant affected=%d err=%v, want 1 nil", affected, err)
	}
	categoryMappings, err := backend.GetCategoryMappings(ctx, tenant)
	if err != nil {
		t.Fatalf("GetCategoryMappings: %v", err)
	}
	if !mappingContains(categoryMappings, category, merchantName) {
		t.Fatalf("GetCategoryMappings missing %q -> %q in %#v", category, merchantName, categoryMappings)
	}
	if affected, err := backend.RemoveCategoryByMerchant(ctx, tenant, category, merchantName); err != nil || affected != 1 {
		t.Fatalf("RemoveCategoryByMerchant affected=%d err=%v, want 1 nil", affected, err)
	}
	if affected, err := backend.ApplyBucketByMerchant(ctx, tenant, bucket, merchantName); err != nil || affected != 1 {
		t.Fatalf("ApplyBucketByMerchant affected=%d err=%v, want 1 nil", affected, err)
	}
	bucketMappings, err := backend.GetBucketMappings(ctx, tenant)
	if err != nil {
		t.Fatalf("GetBucketMappings: %v", err)
	}
	if !mappingContains(bucketMappings, bucket, merchantName) {
		t.Fatalf("GetBucketMappings missing %q -> %q in %#v", bucket, merchantName, bucketMappings)
	}
	if affected, err := backend.RemoveBucketByMerchant(ctx, tenant, bucket, merchantName); err != nil || affected != 1 {
		t.Fatalf("RemoveBucketByMerchant affected=%d err=%v, want 1 nil", affected, err)
	}

	if err := backend.DeleteLabel(ctx, tenant, label, true); err != nil {
		t.Fatalf("DeleteLabel: %v", err)
	}
	if err := backend.DeleteCategory(ctx, tenant, category, true); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}
	if err := backend.DeleteBucket(ctx, tenant, bucket, true); err != nil {
		t.Fatalf("DeleteBucket: %v", err)
	}
}

func testRules(ctx context.Context, t *testing.T, backend store.Backend) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "rules")
	ruleName := "conformance rule " + suffix(t)

	created, err := backend.CreateRule(ctx, tenant, store.RuleRow{
		Name:              ruleName,
		SenderEmails:      []string{"alerts@example.test"},
		SubjectContains:   "spent",
		AmountRegex:       `INR\s+([0-9.]+)`,
		MerchantRegex:     `at\s+(.+)$`,
		CurrencyRegex:     `(INR)`,
		TransactionSource: "Example Card",
		SourceType:        "credit-card",
		SourceLabel:       "Example Card",
		Bank:              "Example",
	})
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	if created.ID == "" || created.Name != ruleName {
		t.Fatalf("CreateRule returned invalid row: %#v", created)
	}

	got, err := backend.GetRule(ctx, tenant, created.ID)
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if got.Name != ruleName || len(got.SenderEmails) != 1 || got.SenderEmails[0] != "alerts@example.test" {
		t.Fatalf("GetRule returned invalid row: %#v", got)
	}

	got.SubjectContains = "updated"
	updated, err := backend.UpdateRule(ctx, tenant, created.ID, *got)
	if err != nil {
		t.Fatalf("UpdateRule: %v", err)
	}
	if updated.SubjectContains != "updated" {
		t.Fatalf("UpdateRule SubjectContains = %q, want updated", updated.SubjectContains)
	}

	rules, err := backend.ListRules(ctx, tenant)
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	if !hasRule(rules, created.ID) {
		t.Fatalf("ListRules missing rule %q in %#v", created.ID, rules)
	}

	if err := backend.DeleteRule(ctx, tenant, created.ID); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}
	if _, err := backend.GetRule(ctx, tenant, created.ID); err == nil {
		t.Fatal("GetRule after DeleteRule returned nil error")
	}
}

func testCommunity(ctx context.Context, t *testing.T, backend store.Backend) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "community")
	category := "Seed Category " + suffix(t)
	bucket := "Seed Bucket " + suffix(t)
	merchant := "Seed Merchant " + suffix(t)

	resolver, err := backend.Seed(ctx, store.SeedContent{
		Rules: []api.Rule{{
			Name:            "seed rule " + suffix(t),
			SenderEmails:    []string{"seed@example.test"},
			SubjectContains: "spent",
			Amount:          regexp.MustCompile(`INR\s+([0-9.]+)`),
			MerchantInfo:    regexp.MustCompile(`at\s+(.+)$`),
			Currency:        regexp.MustCompile(`(INR)`),
			Source:          api.Source{Type: "credit-card", Label: "Seed Card", Bank: "Example"},
		}},
		MCCEntries: []store.MCCEntry{{
			Code:        "1234",
			Description: "seeded conformance mcc",
			Category:    category,
			Bucket:      bucket,
		}},
		MerchantCategories: []store.MerchantCategoryEntry{{
			Fragment: merchant,
			Category: &category,
			Bucket:   &bucket,
		}},
	})
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	gotCategory, gotBucket := resolver("paid at " + merchant)
	if gotCategory != category || gotBucket != bucket {
		t.Fatalf("Seed resolver returned category=%q bucket=%q, want %q %q", gotCategory, gotBucket, category, bucket)
	}

	if err := backend.Write(ctx, store.IngestionBatch{
		Tenant: tenant,
		Transactions: []*api.TransactionDetails{{
			MessageID:    "community-" + suffix(t),
			Amount:       88,
			Currency:     "INR",
			Timestamp:    time.Now().UTC().Format(time.RFC3339),
			MerchantInfo: merchant,
			Source:       api.Source{Type: "credit-card", Label: "Community Card", Bank: "Example"},
		}},
	}); err != nil {
		t.Fatalf("Write community transaction: %v", err)
	}
	manualCategory := "Manual Category " + suffix(t)
	manualBucket := "Manual Bucket " + suffix(t)
	if affected, err := backend.CategorizeMerchant(ctx, tenant, merchant, manualCategory, manualBucket); err != nil || affected != 1 {
		t.Fatalf("CategorizeMerchant affected=%d err=%v, want 1 nil", affected, err)
	}
	rows, result, err := backend.ListTransactions(ctx, tenant, store.ListFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListTransactions after CategorizeMerchant: %v", err)
	}
	if result.Total != 1 || rows[0].Category != manualCategory || rows[0].Bucket != manualBucket {
		t.Fatalf("transaction after CategorizeMerchant result=%#v rows=%#v", result, rows)
	}

	snapshot, err := backend.LoadCategorySnapshot(ctx)
	if err != nil {
		t.Fatalf("LoadCategorySnapshot: %v", err)
	}
	gotCategory, gotBucket = snapshot(merchant)
	if gotCategory != category || gotBucket != bucket {
		t.Fatalf("LoadCategorySnapshot resolver returned category=%q bucket=%q, want %q %q", gotCategory, gotBucket, category, bucket)
	}
}

//nolint:gocognit // Conformance subtests intentionally exercise a broad backend contract in one scenario.
func testIngestion(ctx context.Context, t *testing.T, backend store.Backend) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "ingestion")
	messageID := "message-" + suffix(t)
	timestamp := time.Date(2026, time.January, 2, 10, 30, 0, 0, time.UTC)

	if err := backend.Write(ctx, store.IngestionBatch{
		Tenant: tenant,
		Transactions: []*api.TransactionDetails{{
			MessageID:    messageID,
			Amount:       123.45,
			Currency:     "INR",
			Timestamp:    timestamp.Format(time.RFC3339),
			MerchantInfo: "Conformance Merchant",
			Category:     "Food",
			Bucket:       "Needs",
			Source:       api.Source{Type: "credit-card", Label: "Example Card", Bank: "Example"},
			Description:  "conformance transaction",
			Labels:       []string{"conf-label"},
		}},
	}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	rows, result, err := backend.ListTransactions(ctx, tenant, store.ListFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if result.Total != 1 || len(rows) != 1 {
		t.Fatalf("ListTransactions total=%d len=%d rows=%#v", result.Total, len(rows), rows)
	}
	txn := rows[0]
	if txn.MessageID != messageID || txn.MerchantInfo != "Conformance Merchant" || txn.Source.Type != "credit-card" {
		t.Fatalf("ListTransactions returned invalid transaction: %#v", txn)
	}
	if !containsString(txn.Labels, "conf-label") {
		t.Fatalf("transaction labels = %#v, want conf-label", txn.Labels)
	}

	got, err := backend.GetTransaction(ctx, tenant, txn.ID)
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if got.ID != txn.ID {
		t.Fatalf("GetTransaction ID = %q, want %q", got.ID, txn.ID)
	}

	description := "updated description"
	if err := backend.UpdateTransaction(ctx, tenant, txn.ID, store.TransactionUpdate{Description: &description}); err != nil {
		t.Fatalf("UpdateTransaction: %v", err)
	}
	if err := backend.UpdateDescription(ctx, tenant, txn.ID, "description from UpdateDescription"); err != nil {
		t.Fatalf("UpdateDescription: %v", err)
	}
	if err := backend.AddLabel(ctx, tenant, txn.ID, "manual"); err != nil {
		t.Fatalf("AddLabel: %v", err)
	}
	if err := backend.AddLabels(ctx, tenant, txn.ID, []string{"batch-a", "batch-b"}); err != nil {
		t.Fatalf("AddLabels: %v", err)
	}
	if err := backend.RemoveLabel(ctx, tenant, txn.ID, "conf-label"); err != nil {
		t.Fatalf("RemoveLabel: %v", err)
	}
	got, err = backend.GetTransaction(ctx, tenant, txn.ID)
	if err != nil {
		t.Fatalf("GetTransaction after updates: %v", err)
	}
	if got.Description != "description from UpdateDescription" || !containsString(got.Labels, "batch-a") {
		t.Fatalf("transaction after updates = %#v", got)
	}

	searchRows, searchResult, err := backend.SearchTransactions(ctx, tenant, "merchant", store.ListFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("SearchTransactions: %v", err)
	}
	if searchResult.Total != 1 || len(searchRows) != 1 {
		t.Fatalf("SearchTransactions total=%d len=%d rows=%#v", searchResult.Total, len(searchRows), searchRows)
	}

	facets, err := backend.GetFacets(ctx, tenant)
	if err != nil {
		t.Fatalf("GetFacets: %v", err)
	}
	if !containsString(facets.Currencies, "INR") || !containsString(facets.Merchants, "Conformance Merchant") {
		t.Fatalf("GetFacets missing transaction values: %#v", facets)
	}

	if err := backend.MuteTransaction(ctx, tenant, txn.ID, true, "manual mute"); err != nil {
		t.Fatalf("MuteTransaction true: %v", err)
	}
	if err := backend.UpdateMuteReason(ctx, tenant, txn.ID, "updated mute reason"); err != nil {
		t.Fatalf("UpdateMuteReason: %v", err)
	}
	got, err = backend.GetTransaction(ctx, tenant, txn.ID)
	if err != nil {
		t.Fatalf("GetTransaction after mute: %v", err)
	}
	if !got.Muted || got.MuteReason != "updated mute reason" {
		t.Fatalf("muted transaction = %#v", got)
	}
	if err := backend.MuteTransaction(ctx, tenant, txn.ID, false, ""); err != nil {
		t.Fatalf("MuteTransaction false: %v", err)
	}
	if err := backend.MuteByMerchant(ctx, tenant, "Conformance", "merchant mute"); err != nil {
		t.Fatalf("MuteByMerchant: %v", err)
	}
	mutedMerchants, err := backend.ListMutedMerchants(ctx, tenant)
	if err != nil {
		t.Fatalf("ListMutedMerchants: %v", err)
	}
	if !hasMutedMerchant(mutedMerchants, "Conformance") {
		t.Fatalf("ListMutedMerchants missing Conformance in %#v", mutedMerchants)
	}
	mutedWithCount, err := backend.GetMutedMerchantsWithCount(ctx, tenant)
	if err != nil {
		t.Fatalf("GetMutedMerchantsWithCount: %v", err)
	}
	if len(mutedWithCount) != 1 || mutedWithCount[0].MutedCount != 1 {
		t.Fatalf("GetMutedMerchantsWithCount = %#v, want one muted transaction", mutedWithCount)
	}
	if err := backend.UpdateMerchantReason(ctx, tenant, mutedWithCount[0].ID, "updated merchant reason"); err != nil {
		t.Fatalf("UpdateMerchantReason: %v", err)
	}
	if err := backend.DeleteMutedMerchantAndUnmute(ctx, tenant, mutedWithCount[0].ID); err != nil {
		t.Fatalf("DeleteMutedMerchantAndUnmute: %v", err)
	}
	got, err = backend.GetTransaction(ctx, tenant, txn.ID)
	if err != nil {
		t.Fatalf("GetTransaction after DeleteMutedMerchantAndUnmute: %v", err)
	}
	if got.Muted {
		t.Fatalf("transaction after DeleteMutedMerchantAndUnmute still muted: %#v", got)
	}
	if err := backend.MuteByMerchant(ctx, tenant, "Conformance", "merchant mute"); err != nil {
		t.Fatalf("MuteByMerchant before DeleteMutedMerchant: %v", err)
	}
	mutedMerchants, err = backend.ListMutedMerchants(ctx, tenant)
	if err != nil {
		t.Fatalf("ListMutedMerchants before DeleteMutedMerchant: %v", err)
	}
	if len(mutedMerchants) != 1 {
		t.Fatalf("ListMutedMerchants len=%d, want 1", len(mutedMerchants))
	}
	if err := backend.DeleteMutedMerchant(ctx, tenant, mutedMerchants[0].ID); err != nil {
		t.Fatalf("DeleteMutedMerchant: %v", err)
	}
	if err := backend.UnmuteByPattern(ctx, tenant, "Conformance"); err != nil {
		t.Fatalf("UnmuteByPattern: %v", err)
	}
}

func testDiagnostics(ctx context.Context, t *testing.T, backend store.Backend) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "diagnostics")
	receivedAt := time.Now().UTC().Truncate(time.Microsecond)

	if err := backend.RecordTenantExtractionDiagnostic(ctx, tenant, api.ExtractionDiagnostic{
		Reader:         "gmail",
		MessageID:      "diag-" + suffix(t),
		Source:         "Example",
		SenderEmail:    "sender@example.test",
		Subject:        "transaction alert",
		EmailBody:      "body",
		ReceivedAt:     &receivedAt,
		Snippet:        "snippet",
		RuleName:       "diag rule",
		AmountRegex:    `INR\s+([0-9.]+)`,
		MerchantRegex:  `at\s+(.+)$`,
		CurrencyRegex:  `(INR)`,
		FailureReasons: []string{api.FailureMerchantEmpty},
	}); err != nil {
		t.Fatalf("RecordTenantExtractionDiagnostic: %v", err)
	}

	rows, err := backend.ListExtractionDiagnostics(ctx, tenant, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen, Limit: 10})
	if err != nil {
		t.Fatalf("ListExtractionDiagnostics: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("ListExtractionDiagnostics len=%d rows=%#v", len(rows), rows)
	}
	if rows[0].SenderEmail != "sender@example.test" || rows[0].Status != store.DiagnosticStatusOpen {
		t.Fatalf("ListExtractionDiagnostics returned invalid row: %#v", rows[0])
	}

	got, err := backend.GetExtractionDiagnostic(ctx, tenant, rows[0].ID)
	if err != nil {
		t.Fatalf("GetExtractionDiagnostic: %v", err)
	}
	if got.ID != rows[0].ID {
		t.Fatalf("GetExtractionDiagnostic ID = %q, want %q", got.ID, rows[0].ID)
	}

	updated, err := backend.UpdateExtractionDiagnosticStatus(ctx, tenant, rows[0].ID, store.DiagnosticStatusResolved)
	if err != nil {
		t.Fatalf("UpdateExtractionDiagnosticStatus: %v", err)
	}
	if updated.Status != store.DiagnosticStatusResolved || updated.ResolvedAt == nil {
		t.Fatalf("UpdateExtractionDiagnosticStatus returned invalid row: %#v", updated)
	}
}

func createTenant(ctx context.Context, t *testing.T, backend store.Backend, name string) store.Tenant {
	t.Helper()

	user, err := backend.CreateUser(ctx, store.CreateUserInput{
		Email:        email(t, name),
		DisplayName:  "Conformance " + name,
		Role:         store.UserRoleUser,
		AvatarKey:    "avatar-" + name,
		PasswordHash: "hash-" + name,
	})
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", name, err)
	}
	return store.Tenant{ID: user.TenantID}
}

func email(t *testing.T, name string) string {
	t.Helper()
	return fmt.Sprintf("%s-%s@example.test", name, suffix(t))
}

func suffix(t *testing.T) string {
	t.Helper()
	replacer := strings.NewReplacer("/", "-", " ", "-", "_", "-", "#", "-")
	return strings.ToLower(replacer.Replace(t.Name()))
}

func assertJSON(t *testing.T, got, want []byte) {
	t.Helper()
	var gotValue any
	var wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("unmarshal got JSON: %v", err)
	}
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("unmarshal want JSON: %v", err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("JSON = %s, want %s", got, want)
	}
}

func intPtr(v int) *int {
	return &v
}

func hasLabel(labels []store.Label, name, color string) bool {
	for _, label := range labels {
		if label.Name == name && label.Color == color {
			return true
		}
	}
	return false
}

func hasCategory(categories []store.Category, name string) bool {
	for _, category := range categories {
		if category.Name == name {
			return true
		}
	}
	return false
}

func hasBucket(buckets []store.Bucket, name string) bool {
	for _, bucket := range buckets {
		if bucket.Name == name {
			return true
		}
	}
	return false
}

func hasRule(rules []store.RuleRow, id string) bool {
	for _, rule := range rules {
		if rule.ID == id {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func mappingContains(mappings map[string][]string, key, value string) bool {
	for _, candidate := range mappings[key] {
		if candidate == value {
			return true
		}
	}
	return false
}

func hasMutedMerchant(merchants []store.MutedMerchant, pattern string) bool {
	for _, merchant := range merchants {
		if merchant.Pattern == pattern {
			return true
		}
	}
	return false
}
