// Package storetest provides backend-neutral store conformance tests.
package storetest

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const (
	baseCurrencyINR = "INR"
	baseCurrencyUSD = "USD"
)

// Subject holds a store under test and its cleanup function.
type Subject[T any] struct {
	Store                  T
	Close                  func()
	Restart                func(t *testing.T) T
	RejectIngestionMessage func(t *testing.T, messageID string)
	Now                    time.Time
}

// Factory creates a fresh store subject.
type Factory[T any] func(t *testing.T) Subject[T]

// BackendFactory creates a complete backend subject.
type BackendFactory func(t *testing.T) Subject[store.Backend]

// RuntimeSubject contains the contracts used by the runtime scenario.
type RuntimeSubject interface {
	store.AuthStore
	store.RuntimeStore
}

// ScanningSubject contains the contracts used by the scanning scenario.
type ScanningSubject interface {
	store.AuthStore
	store.ScanningStore
}

// TaxonomySubject contains the contracts used by the taxonomy scenario.
type TaxonomySubject interface {
	store.AuthStore
	store.TaxonomyStore
}

// TaxonomyTransactionSubject contains the contracts used by taxonomy and transaction integration scenarios.
type TaxonomyTransactionSubject interface {
	store.AuthStore
	store.TaxonomyStore
	store.TransactionStore
	store.TransactionBatchWriter
}

// CommunityTransactionSubject contains the contracts used by community and transaction integration scenarios.
type CommunityTransactionSubject interface {
	store.AuthStore
	store.CommunityStore
	store.TaxonomyStore
	store.TransactionStore
	store.TransactionBatchWriter
}

// CommunitySeedSubject contains the contracts used by community seed scenarios.
type CommunitySeedSubject interface {
	store.RuleStore
	store.Seeder
}

// RulesSubject contains the contracts used by the rules scenario.
type RulesSubject interface {
	store.AuthStore
	store.RuleStore
}

// TransactionSubject contains the contracts used by the transaction scenario.
type TransactionSubject interface {
	store.AuthStore
	store.TransactionStore
	store.TransactionBatchWriter
}

// IngestionSubject contains the contracts used by ingestion scenarios.
type IngestionSubject interface {
	store.AuthStore
	store.CommunityStore
	store.TaxonomyStore
	store.TransactionStore
	store.TransactionBatchWriter
}

// AnalyticsSubject contains the contracts used by analytics scenarios.
type AnalyticsSubject interface {
	store.AuthStore
	store.AnalyticsStore
	store.RuntimeStore
	store.TransactionStore
	store.TransactionBatchWriter
}

// DiagnosticsSubject contains the contracts used by the diagnostics scenario.
type DiagnosticsSubject interface {
	store.AuthStore
	store.DiagnosticStore
}

// Run exercises the complete backend-neutral store contract. Each factory call
// must return a freshly migrated empty database.
func Run(t *testing.T, factory BackendFactory) {
	t.Helper()

	t.Run("Health", func(t *testing.T) {
		RunHealth(t, adaptFactory(factory, func(backend store.Backend) store.HealthChecker { return backend }))
	})
	t.Run("Auth", func(t *testing.T) {
		RunAuth(t, adaptFactory(factory, func(backend store.Backend) store.AuthStore { return backend }))
	})
	t.Run("Runtime", func(t *testing.T) {
		RunRuntime(t, adaptFactory(factory, func(backend store.Backend) RuntimeSubject { return backend }))
	})
	t.Run("Scanning", func(t *testing.T) {
		RunScanning(t, adaptFactory(factory, func(backend store.Backend) ScanningSubject { return backend }))
	})
	t.Run("Taxonomy", func(t *testing.T) {
		RunTaxonomy(t, adaptFactory(factory, func(backend store.Backend) TaxonomySubject { return backend }))
	})
	t.Run("TaxonomyTransactions", func(t *testing.T) {
		RunTaxonomyTransactions(t, adaptFactory(factory, func(backend store.Backend) TaxonomyTransactionSubject { return backend }))
	})
	t.Run("Community", func(t *testing.T) {
		RunCommunity(t, adaptFactory(factory, func(backend store.Backend) store.CommunityStore { return backend }))
	})
	t.Run("CommunitySeed", func(t *testing.T) {
		RunCommunitySeed(t, adaptFactory(factory, func(backend store.Backend) CommunitySeedSubject { return backend }))
	})
	t.Run("CommunityTransactions", func(t *testing.T) {
		RunCommunityTransactions(t, adaptFactory(factory, func(backend store.Backend) CommunityTransactionSubject { return backend }))
	})
	t.Run("Rules", func(t *testing.T) {
		RunRules(t, adaptFactory(factory, func(backend store.Backend) RulesSubject { return backend }))
	})
	t.Run("Transactions", func(t *testing.T) {
		RunTransactions(t, adaptFactory(factory, func(backend store.Backend) TransactionSubject { return backend }))
	})
	t.Run("Search", func(t *testing.T) {
		RunSearch(t, adaptFactory(factory, func(backend store.Backend) TransactionSubject { return backend }))
	})
	t.Run("Ingestion", func(t *testing.T) {
		RunIngestion(t, adaptFactory(factory, func(backend store.Backend) IngestionSubject { return backend }))
	})
	t.Run("Analytics", func(t *testing.T) {
		RunAnalytics(t, adaptFactory(factory, func(backend store.Backend) AnalyticsSubject { return backend }))
	})
	t.Run("Diagnostics", func(t *testing.T) {
		RunDiagnostics(t, adaptFactory(factory, func(backend store.Backend) DiagnosticsSubject { return backend }))
	})
}

// RunHealth exercises the health contract with a fresh subject.
func RunHealth(t *testing.T, factory Factory[store.HealthChecker]) {
	t.Helper()
	subject := factory(t)
	t.Cleanup(subject.Close)
	testHealth(context.Background(), t, subject.Store)
}

// RunAuth exercises the authentication contract with a fresh subject.
func RunAuth(t *testing.T, factory Factory[store.AuthStore]) {
	t.Helper()

	runConformanceCase(t, "Lifecycle", factory, testAuth)
	runConformanceCase(t, "Users", factory, testAuthUsers)
	runConformanceCase(t, "MissingRows", factory, testAuthMissingRows)
	runConformanceCase(t, "AccessTokens", factory, testAuthAccessTokens)
	runConformanceCase(t, "SetupTokens", factory, testAuthSetupTokens)
	runConformanceCase(t, "UniqueHashesAndCascades", factory, testAuthUniqueHashesAndCascades)
}

// RunRuntime exercises the runtime contract with a fresh subject.
func RunRuntime(t *testing.T, factory Factory[RuntimeSubject]) {
	t.Helper()

	runConformanceCase(t, "Basic", factory, testRuntime)
	runConformanceCase(t, "MessagesAndAppConfig", factory, testRuntimeMessagesAndAppConfig)
	runConformanceCase(t, "ReaderRuntime", factory, testReaderRuntime)
	runConformanceCase(t, "LLMRuntime", factory, testLLMRuntime)
	runConformanceCase(t, "GlobalSettings", factory, testRuntimeGlobalSettings)
	runConformanceCase(t, "UserCascade", factory, testRuntimeUserCascade)
}

// RunRuntimeWithoutSecret exercises runtime failures when encryption is not configured.
func RunRuntimeWithoutSecret(t *testing.T, factory Factory[RuntimeSubject]) {
	t.Helper()

	runConformanceCase(t, "MissingSecretBox", factory, testRuntimeWithoutSecret)
}

// RunScanning exercises the scanning contract with a fresh subject.
func RunScanning(t *testing.T, factory Factory[ScanningSubject]) {
	t.Helper()

	runConformanceCase(t, "Basic", factory, testScanning)
	runConformanceCase(t, "Scheduler", factory, testScanningScheduler)
	runConformanceCase(t, "StateLifecycle", factory, testScanningStateLifecycle)
	runConformanceCase(t, "ListingsAndCascade", factory, testScanningListingsAndCascade)
}

// RunTaxonomy exercises the taxonomy contract with a fresh subject.
func RunTaxonomy(t *testing.T, factory Factory[TaxonomySubject]) {
	t.Helper()

	runConformanceCase(t, "Lifecycle", factory, testTaxonomy)
	runConformanceCase(t, "VisibilityAndOverrides", factory, testTaxonomyVisibilityAndOverrides)
	runConformanceCase(t, "MissingAndDefaultDeletes", factory, testTaxonomyMissingAndDefaultDeletes)
	runConformanceCase(t, "Mappings", factory, testTaxonomyMappings)
}

// RunTaxonomyTransactions exercises taxonomy integration with ingested transactions.
func RunTaxonomyTransactions(t *testing.T, factory Factory[TaxonomyTransactionSubject]) {
	t.Helper()

	runConformanceCase(t, "Lifecycle", factory, testTaxonomyTransactions)
	runConformanceCase(t, "DeleteTransactionCleanup", factory, testTaxonomyDeleteTransactionCleanup)
	runConformanceCase(t, "MerchantMatching", factory, testTaxonomyMerchantMatching)
	runConformanceCase(t, "LabelProvenance", factory, testTaxonomyLabelProvenance)
}

// RunCommunity exercises the community contract with a fresh subject.
func RunCommunity(t *testing.T, factory Factory[store.CommunityStore]) {
	t.Helper()

	runConformanceCase(t, "Lifecycle", factory, testCommunity)
	runConformanceCase(t, "MCCOverwrite", factory, testCommunityMCCOverwrite)
	runConformanceCase(t, "Resolution", factory, testCommunityResolution)
}

// RunCommunitySeed exercises bundled community seeding with a fresh subject.
func RunCommunitySeed(t *testing.T, factory Factory[CommunitySeedSubject]) {
	t.Helper()

	runConformanceCase(t, "Lifecycle", factory, testCommunitySeed)
	runConformanceCase(t, "Idempotence", factory, testCommunitySeedIdempotence)
}

// RunCommunityTransactions exercises community integration with transactions.
func RunCommunityTransactions(t *testing.T, factory Factory[CommunityTransactionSubject]) {
	t.Helper()

	runConformanceCase(t, "Lifecycle", factory, testCommunityTransactions)
	runConformanceCase(t, "ExactMerchantAndIsolation", factory, testCommunityExactMerchantAndIsolation)
}

// RunRules exercises the rules contract with a fresh subject.
func RunRules(t *testing.T, factory Factory[RulesSubject]) {
	t.Helper()

	runConformanceCase(t, "Lifecycle", factory, testRules)
	runConformanceCase(t, "VisibilityAndIsolation", factory, testRulesVisibilityAndIsolation)
	runConformanceCase(t, "Uniqueness", factory, testRulesUniqueness)
	runConformanceCase(t, "SeedAndPredefinedSemantics", factory, testRulesSeedAndPredefinedSemantics)
	runConformanceCase(t, "ImportAndRoundTrip", factory, testRulesImportAndRoundTrip)
}

// RunTransactions exercises the transaction contract with a fresh subject.
func RunTransactions(t *testing.T, factory Factory[TransactionSubject]) {
	t.Helper()

	runConformanceCase(t, "Mutations", factory, testTransactions)
	runConformanceCase(t, "ListFilters", factory, testTransactionListFilters)
	runConformanceCase(t, "MissingExclusionsAndMutePrecedence", factory, testTransactionMissingExclusionsAndMutePrecedence)
	runConformanceCase(t, "PaginationSortAndTotals", factory, testTransactionPaginationSortAndTotals)
	runConformanceCase(t, "FacetsAndEmptyOutputs", factory, testTransactionFacetsAndEmptyOutputs)
	runConformanceCase(t, "MerchantMutatorIsolation", factory, testTransactionMerchantMutatorIsolation)
}

// RunSearch exercises transaction search with a fresh subject per case.
func RunSearch(t *testing.T, factory Factory[TransactionSubject]) {
	t.Helper()

	runConformanceCase(t, "MatchingAndLiteralSafety", factory, testSearchMatchingAndLiteralSafety)
	runConformanceCase(t, "FiltersSortPaginationAndTotals", factory, testSearchFiltersSortPaginationAndTotals)
	runConformanceCase(t, "MutationDeleteAndIsolation", factory, testSearchMutationDeleteAndIsolation)
}

// RunIngestion exercises ingestion with a fresh subject per case.
func RunIngestion(t *testing.T, factory Factory[IngestionSubject]) {
	t.Helper()

	runConformanceCase(t, "EmptyDefaultsAndDuplicates", factory, testIngestionEmptyDefaultsAndDuplicates)
	runConformanceCase(t, "RefreshPreservesUserFields", factory, testIngestionRefreshPreservesUserFields)
	runConformanceCase(t, "LabelsMappingsAndMuteRules", factory, testIngestionLabelsMappingsAndMuteRules)
	t.Run("AtomicRollback", func(t *testing.T) {
		subject := factory(t)
		t.Cleanup(subject.Close)
		testIngestionAtomicRollback(context.Background(), t, subject)
	})
	t.Run("RestartPersistence", func(t *testing.T) {
		subject := factory(t)
		t.Cleanup(subject.Close)
		testIngestionRestartPersistence(context.Background(), t, subject)
	})
}

// RunAnalytics exercises all analytics read models with a fresh subject per case.
func RunAnalytics(t *testing.T, factory Factory[AnalyticsSubject]) {
	t.Helper()

	runConformanceCase(t, "EmptyOutputs", factory, testAnalyticsEmptyOutputs)
	runConformanceCase(t, "StatsChartsAndDashboard", factory, testAnalyticsStatsChartsAndDashboard)
	runConformanceCase(t, "TimezoneAndDST", factory, testAnalyticsTimezoneAndDST)
	runConformanceCase(t, "HeatmapAnnualAndBounds", factory, testAnalyticsHeatmapAnnualAndBounds)
	runConformanceCase(t, "MonthlyBreakdown", factory, testAnalyticsMonthlyBreakdown)
	runConformanceCase(t, "TenantIsolation", factory, testAnalyticsTenantIsolation)
}

// RunDiagnostics exercises the diagnostic contract with a fresh subject.
func RunDiagnostics(t *testing.T, factory Factory[DiagnosticsSubject]) {
	t.Helper()

	runConformanceCase(t, "Lifecycle", factory, testDiagnostics)
	runConformanceCase(t, "ValidationAndIsolation", factory, testDiagnosticsValidationAndIsolation)
	runConformanceCase(t, "ListOrderAndLimit", factory, testDiagnosticsListOrderAndLimit)
	runConformanceCase(t, "Deduplication", factory, testDiagnosticsDeduplication)
	runConformanceCase(t, "EmptyMessageAndFallbacks", factory, testDiagnosticsEmptyMessageAndFallbacks)
	runConformanceCase(t, "ReopenConflict", factory, testDiagnosticsReopenConflict)
	runConformanceCase(t, "NotFoundAndCascade", factory, testDiagnosticsNotFoundAndCascade)
}

func adaptFactory[T any](factory BackendFactory, narrow func(store.Backend) T) Factory[T] {
	return func(t *testing.T) Subject[T] {
		t.Helper()
		subject := factory(t)
		var restart func(*testing.T) T
		if subject.Restart != nil {
			restart = func(t *testing.T) T {
				t.Helper()
				return narrow(subject.Restart(t))
			}
		}
		return Subject[T]{
			Store:                  narrow(subject.Store),
			Close:                  subject.Close,
			Restart:                restart,
			RejectIngestionMessage: subject.RejectIngestionMessage,
			Now:                    subject.Now,
		}
	}
}

func runConformanceCase[T any](t *testing.T, name string, factory Factory[T], test func(context.Context, *testing.T, T)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		subject := factory(t)
		t.Cleanup(subject.Close)
		test(context.Background(), t, subject.Store)
	})
}

func testHealth(ctx context.Context, t *testing.T, backend store.HealthChecker) {
	t.Helper()

	if err := backend.HealthCheck(ctx); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
}

//nolint:gocognit // Conformance subtests intentionally exercise a broad backend contract in one scenario.
func testAuth(ctx context.Context, t *testing.T, backend store.AuthStore) {
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
	_, err = backend.CreateBootstrapAdmin(ctx, store.CreateBootstrapAdminInput{
		Email:        email(t, "second-admin"),
		DisplayName:  "Second Conformance Admin",
		PasswordHash: "hash-second-admin",
		AvatarKey:    "default",
	})
	assertErrorKind(t, err, errors.Conflict, "CreateBootstrapAdmin second attempt")

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
	if err := backend.MarkAccessTokenUsed(ctx, accessToken.ID); err != nil {
		t.Fatalf("MarkAccessTokenUsed: %v", err)
	}
	tokens, err = backend.ListAccessTokens(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListAccessTokens after use: %v", err)
	}
	if tokens[0].LastUsedAt == nil {
		t.Fatal("MarkAccessTokenUsed did not persist a timestamp")
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

func testAuthUsers(ctx context.Context, t *testing.T, backend store.AuthStore) {
	t.Helper()

	user, err := backend.CreateUser(ctx, store.CreateUserInput{
		Email:       "case-user@example.test",
		DisplayName: "Case User",
	})
	if err != nil {
		t.Fatalf("CreateUser defaults: %v", err)
	}
	if user.Role != store.UserRoleUser || user.AvatarKey != "default" {
		t.Fatalf("CreateUser defaults = role %q avatar %q, want user/default", user.Role, user.AvatarKey)
	}

	_, err = backend.CreateUser(ctx, store.CreateUserInput{
		Email:       "CASE-USER@EXAMPLE.TEST",
		DisplayName: "Duplicate User",
	})
	assertErrorKind(t, err, errors.Conflict, "CreateUser case-variant duplicate")

	role := store.UserRoleAdmin
	avatar := "wallet"
	disabled := true
	updated, err := backend.UpdateUser(ctx, user.ID, store.UpdateUserInput{
		Role:      &role,
		AvatarKey: &avatar,
		Disabled:  &disabled,
	})
	if err != nil {
		t.Fatalf("UpdateUser role/avatar/disabled: %v", err)
	}
	if updated.Role != role || updated.AvatarKey != avatar || updated.DisabledAt == nil {
		t.Fatalf("UpdateUser result = %#v, want admin/wallet/disabled", updated)
	}
	disabled = false
	updated, err = backend.UpdateUser(ctx, user.ID, store.UpdateUserInput{Disabled: &disabled})
	if err != nil {
		t.Fatalf("UpdateUser re-enable: %v", err)
	}
	if updated.DisabledAt != nil {
		t.Fatalf("UpdateUser re-enable DisabledAt = %v, want nil", updated.DisabledAt)
	}
}

func testAuthMissingRows(ctx context.Context, t *testing.T, backend store.AuthStore) {
	t.Helper()

	const missingID = "00000000-0000-0000-0000-000000000000"
	displayName := "Missing"
	_, err := backend.UpdateUser(ctx, missingID, store.UpdateUserInput{DisplayName: &displayName})
	assertErrorKind(t, err, errors.NotFound, "UpdateUser missing")
	assertErrorKind(
		t,
		backend.UpdateUserPassword(ctx, missingID, store.UpdateUserPasswordInput{PasswordHash: "hash"}),
		errors.NotFound,
		"UpdateUserPassword missing",
	)
	assertErrorKind(t, backend.DeleteUser(ctx, missingID), errors.NotFound, "DeleteUser missing")
	_, err = backend.FindUserByID(ctx, missingID)
	assertErrorKind(t, err, errors.NotFound, "FindUserByID missing")
	_, err = backend.FindUserByEmail(ctx, "missing@example.test")
	assertErrorKind(t, err, errors.NotFound, "FindUserByEmail missing")
	_, err = backend.FindSessionByHash(ctx, "missing-session")
	assertErrorKind(t, err, errors.NotFound, "FindSessionByHash missing")
	assertErrorKind(t, backend.RevokeSession(ctx, missingID), errors.NotFound, "RevokeSession missing")
	_, err = backend.FindAccessTokenByHash(ctx, "missing-access-token")
	assertErrorKind(t, err, errors.NotFound, "FindAccessTokenByHash missing")
	assertErrorKind(t, backend.MarkAccessTokenUsed(ctx, missingID), errors.NotFound, "MarkAccessTokenUsed missing")
	assertErrorKind(t, backend.RevokeAccessToken(ctx, missingID, missingID), errors.NotFound, "RevokeAccessToken missing")
	_, err = backend.FindAccountSetupTokenByHash(ctx, "missing-setup-token")
	assertErrorKind(t, err, errors.NotFound, "FindAccountSetupTokenByHash missing")
	assertErrorKind(t, backend.MarkAccountSetupTokenUsed(ctx, missingID), errors.NotFound, "MarkAccountSetupTokenUsed missing")
	_, err = backend.CompleteAccountSetup(ctx, store.CompleteAccountSetupInput{TokenHash: "missing-setup-token"})
	assertErrorKind(t, err, errors.NotFound, "CompleteAccountSetup missing")
}

func testAuthAccessTokens(ctx context.Context, t *testing.T, backend store.AuthStore) {
	t.Helper()

	owner := createUser(ctx, t, backend, "token-owner")
	other := createUser(ctx, t, backend, "token-other")
	active := createAccessToken(ctx, t, backend, owner.ID, "cli", "access-active")

	_, err := backend.CreateAccessToken(ctx, store.CreateAccessTokenInput{
		UserID: owner.ID, Name: active.Name, TokenHash: "access-name-conflict",
	})
	assertErrorKind(t, err, errors.Conflict, "CreateAccessToken active name conflict")

	assertErrorKind(t, backend.RevokeAccessToken(ctx, active.ID, other.ID), errors.NotFound, "RevokeAccessToken wrong user")
	unchanged, err := backend.FindAccessTokenByHash(ctx, active.TokenHash)
	if err != nil {
		t.Fatalf("FindAccessTokenByHash after wrong-user revoke: %v", err)
	}
	if unchanged.RevokedAt != nil {
		t.Fatalf("wrong-user revoke changed token: %#v", unchanged)
	}

	revoked := createAccessToken(ctx, t, backend, owner.ID, "old", "access-revoked")
	if err := backend.RevokeAccessToken(ctx, revoked.ID, owner.ID); err != nil {
		t.Fatalf("RevokeAccessToken: %v", err)
	}
	tokens, err := backend.ListAccessTokens(ctx, owner.ID)
	if err != nil {
		t.Fatalf("ListAccessTokens: %v", err)
	}
	if len(tokens) != 1 || tokens[0].ID != active.ID {
		t.Fatalf("ListAccessTokens = %#v, want only active token", tokens)
	}
	assertErrorKind(t, backend.MarkAccessTokenUsed(ctx, revoked.ID), errors.NotFound, "MarkAccessTokenUsed revoked")

	if _, err := backend.CreateAccessToken(ctx, store.CreateAccessTokenInput{
		UserID: owner.ID, Name: revoked.Name, TokenHash: "access-reused-name",
	}); err != nil {
		t.Fatalf("CreateAccessToken reused revoked name: %v", err)
	}
}

func testAuthSetupTokens(ctx context.Context, t *testing.T, backend store.AuthStore) {
	t.Helper()

	user := createUser(ctx, t, backend, "setup-user")
	expired := createSetupToken(ctx, t, backend, user.ID, "setup-expired", time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC))
	before, err := backend.FindUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindUserByID before expired setup: %v", err)
	}
	_, err = backend.CompleteAccountSetup(ctx, store.CompleteAccountSetupInput{
		TokenHash: expired.TokenHash, PasswordHash: "expired", DisplayName: "Expired", AvatarKey: "default",
	})
	assertErrorKind(t, err, errors.NotFound, "CompleteAccountSetup expired")
	after, err := backend.FindUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindUserByID after expired setup: %v", err)
	}
	if after.PasswordHash != before.PasswordHash || after.DisplayName != before.DisplayName || after.AvatarKey != before.AvatarKey {
		t.Fatalf("expired CompleteAccountSetup changed user: before=%#v after=%#v", before, after)
	}

	active := createSetupToken(ctx, t, backend, user.ID, "setup-active", time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC))
	input := store.CompleteAccountSetupInput{
		TokenHash: active.TokenHash, PasswordHash: "completed", DisplayName: "Completed", AvatarKey: "wallet",
	}
	if _, err := backend.CompleteAccountSetup(ctx, input); err != nil {
		t.Fatalf("CompleteAccountSetup active: %v", err)
	}
	_, err = backend.CompleteAccountSetup(ctx, input)
	assertErrorKind(t, err, errors.NotFound, "CompleteAccountSetup reused")
}

func testAuthUniqueHashesAndCascades(ctx context.Context, t *testing.T, backend store.AuthStore) {
	t.Helper()

	user := createUser(ctx, t, backend, "hash-user")
	other := createUser(ctx, t, backend, "hash-other")
	expires := time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC)
	session, err := backend.CreateSession(ctx, store.CreateSessionInput{UserID: user.ID, TokenHash: "shared-session-hash", ExpiresAt: expires})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	_, err = backend.CreateSession(ctx, store.CreateSessionInput{UserID: other.ID, TokenHash: session.TokenHash, ExpiresAt: expires})
	assertErrorKind(t, err, errors.Conflict, "CreateSession duplicate hash")

	access := createAccessToken(ctx, t, backend, user.ID, "primary", "shared-access-hash")
	_, err = backend.CreateAccessToken(ctx, store.CreateAccessTokenInput{UserID: other.ID, Name: "secondary", TokenHash: access.TokenHash})
	assertErrorKind(t, err, errors.Conflict, "CreateAccessToken duplicate hash")

	setup := createSetupToken(ctx, t, backend, user.ID, "shared-setup-hash", expires)
	_, err = backend.CreateAccountSetupToken(ctx, store.CreateAccountSetupTokenInput{UserID: other.ID, TokenHash: setup.TokenHash, ExpiresAt: expires})
	assertErrorKind(t, err, errors.Conflict, "CreateAccountSetupToken duplicate hash")

	if err := backend.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	_, err = backend.FindUserByID(ctx, user.ID)
	assertErrorKind(t, err, errors.NotFound, "FindUserByID deleted")
	_, err = backend.FindSessionByHash(ctx, session.TokenHash)
	assertErrorKind(t, err, errors.NotFound, "FindSessionByHash after user delete")
	_, err = backend.FindAccessTokenByHash(ctx, access.TokenHash)
	assertErrorKind(t, err, errors.NotFound, "FindAccessTokenByHash after user delete")
	_, err = backend.FindAccountSetupTokenByHash(ctx, setup.TokenHash)
	assertErrorKind(t, err, errors.NotFound, "FindAccountSetupTokenByHash after user delete")
}

//nolint:gocognit // Conformance subtests intentionally exercise a broad backend contract in one scenario.
func testRuntime(ctx context.Context, t *testing.T, backend RuntimeSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "runtime")

	if err := backend.SetAppConfig(ctx, tenant, "base_currency", baseCurrencyINR); err != nil {
		t.Fatalf("SetAppConfig: %v", err)
	}
	value, err := backend.GetAppConfig(ctx, tenant, "base_currency")
	if err != nil {
		t.Fatalf("GetAppConfig: %v", err)
	}
	if value != baseCurrencyINR {
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

func testRuntimeMessagesAndAppConfig(ctx context.Context, t *testing.T, backend RuntimeSubject) {
	t.Helper()

	tenantA := createTenant(ctx, t, backend, "runtime-config-a")
	tenantB := createTenant(ctx, t, backend, "runtime-config-b")
	if value, err := backend.GetAppConfig(ctx, tenantA, "missing"); err == nil {
		t.Fatalf("GetAppConfig missing = %q, want error", value)
	}
	if err := backend.SetAppConfig(ctx, tenantA, "currency", baseCurrencyINR); err != nil {
		t.Fatalf("SetAppConfig tenant A: %v", err)
	}
	if err := backend.SetAppConfig(ctx, tenantB, "currency", baseCurrencyUSD); err != nil {
		t.Fatalf("SetAppConfig tenant B: %v", err)
	}
	for _, tc := range []struct {
		tenant store.Tenant
		want   string
	}{{tenantA, baseCurrencyINR}, {tenantB, baseCurrencyUSD}} {
		value, err := backend.GetAppConfig(ctx, tc.tenant, "currency")
		if err != nil || value != tc.want {
			t.Fatalf("GetAppConfig tenant %q = %q / %v, want %q", tc.tenant.ID, value, err, tc.want)
		}
	}

	processed, err := backend.IsMessageProcessed(ctx, tenantA, "")
	if err != nil || processed {
		t.Fatalf("IsMessageProcessed empty = %v / %v, want false/nil", processed, err)
	}
	assertErrorKind(t, backend.MarkMessageProcessed(ctx, tenantA, "", time.Time{}), errors.InvalidInput, "MarkMessageProcessed empty")

	at := time.Date(2026, time.September, 15, 12, 0, 0, 0, time.UTC)
	if err := backend.MarkMessageProcessed(ctx, tenantA, "shared-message", at); err != nil {
		t.Fatalf("MarkMessageProcessed tenant A: %v", err)
	}
	processed, err = backend.IsMessageProcessed(ctx, tenantB, "shared-message")
	if err != nil || processed {
		t.Fatalf("IsMessageProcessed tenant B = %v / %v, want false/nil", processed, err)
	}
	if err := backend.MarkMessageProcessed(ctx, tenantA, "shared-message", at.Add(time.Hour)); err != nil {
		t.Fatalf("MarkMessageProcessed overwrite: %v", err)
	}
}

func testReaderRuntime(ctx context.Context, t *testing.T, backend RuntimeSubject) {
	t.Helper()

	tenantA := createTenant(ctx, t, backend, "reader-a")
	tenantB := createTenant(ctx, t, backend, "reader-b")
	secretA := []byte(`{"client":"a"}`)
	tokenA := []byte(`{"token":"a"}`)
	configA := json.RawMessage(`{"mailbox":"A"}`)
	if err := backend.SetReaderSecret(ctx, tenantA, "gmail", secretA); err != nil {
		t.Fatalf("SetReaderSecret: %v", err)
	}
	if err := backend.SetReaderToken(ctx, tenantA, "gmail", tokenA); err != nil {
		t.Fatalf("SetReaderToken: %v", err)
	}
	if err := backend.SetReaderConfig(ctx, tenantA, "gmail", configA); err != nil {
		t.Fatalf("SetReaderConfig: %v", err)
	}
	assertRuntimeBytes(t, runtimeBytes(backend.GetReaderSecret(ctx, tenantA, "gmail")), secretA, "GetReaderSecret")
	assertRuntimeBytes(t, runtimeBytes(backend.GetReaderToken(ctx, tenantA, "gmail")), tokenA, "GetReaderToken")
	config, found, err := backend.GetReaderConfig(ctx, tenantA, "gmail")
	if err != nil || !found {
		t.Fatalf("GetReaderConfig found=%v err=%v", found, err)
	}
	assertJSON(t, config, configA)
	assertRuntimeMissing(t, runtimeBytes(backend.GetReaderSecret(ctx, tenantB, "gmail")), "GetReaderSecret tenant isolation")
	assertRuntimeMissing(t, runtimeBytes(backend.GetReaderToken(ctx, tenantB, "gmail")), "GetReaderToken tenant isolation")
	if _, tenantFound, err := backend.GetReaderConfig(ctx, tenantB, "gmail"); err != nil || tenantFound {
		t.Fatalf("GetReaderConfig tenant isolation found=%v err=%v", tenantFound, err)
	}

	invalid := []byte(`{"broken"`)
	assertErrorKind(t, backend.SetReaderSecret(ctx, tenantA, "invalid", invalid), errors.InvalidInput, "SetReaderSecret invalid JSON")
	assertErrorKind(t, backend.SetReaderToken(ctx, tenantA, "invalid", invalid), errors.InvalidInput, "SetReaderToken invalid JSON")
	assertErrorKind(t, backend.SetReaderConfig(ctx, tenantA, "invalid", invalid), errors.InvalidInput, "SetReaderConfig invalid JSON")
	assertRuntimeMissing(t, runtimeBytes(backend.GetReaderSecret(ctx, tenantA, "invalid")), "GetReaderSecret after invalid JSON")
	assertRuntimeMissing(t, runtimeBytes(backend.GetReaderToken(ctx, tenantA, "invalid")), "GetReaderToken after invalid JSON")
	if _, invalidFound, err := backend.GetReaderConfig(ctx, tenantA, "invalid"); err != nil || invalidFound {
		t.Fatalf("GetReaderConfig after invalid JSON found=%v err=%v", invalidFound, err)
	}
	assertErrorKind(t, backend.SetReaderSecret(ctx, tenantA, "gmail", invalid), errors.InvalidInput, "SetReaderSecret invalid replacement")
	assertErrorKind(t, backend.SetReaderToken(ctx, tenantA, "gmail", invalid), errors.InvalidInput, "SetReaderToken invalid replacement")
	assertErrorKind(t, backend.SetReaderConfig(ctx, tenantA, "gmail", invalid), errors.InvalidInput, "SetReaderConfig invalid replacement")
	assertRuntimeBytes(t, runtimeBytes(backend.GetReaderSecret(ctx, tenantA, "gmail")), secretA, "GetReaderSecret after invalid replacement")
	assertRuntimeBytes(t, runtimeBytes(backend.GetReaderToken(ctx, tenantA, "gmail")), tokenA, "GetReaderToken after invalid replacement")
	config, found, err = backend.GetReaderConfig(ctx, tenantA, "gmail")
	if err != nil || !found {
		t.Fatalf("GetReaderConfig after invalid replacement found=%v err=%v", found, err)
	}
	assertJSON(t, config, configA)

	if err := backend.DeleteReaderToken(ctx, tenantA, "gmail"); err != nil {
		t.Fatalf("DeleteReaderToken: %v", err)
	}
	assertRuntimeMissing(t, runtimeBytes(backend.GetReaderToken(ctx, tenantA, "gmail")), "GetReaderToken after delete")
	assertRuntimeBytes(t, runtimeBytes(backend.GetReaderSecret(ctx, tenantA, "gmail")), secretA, "GetReaderSecret after token delete")
	if err := backend.SetReaderToken(ctx, tenantA, "gmail", tokenA); err != nil {
		t.Fatalf("SetReaderToken before runtime delete: %v", err)
	}
	if err := backend.DeleteReaderRuntime(ctx, tenantA, "gmail"); err != nil {
		t.Fatalf("DeleteReaderRuntime: %v", err)
	}
	assertRuntimeMissing(t, runtimeBytes(backend.GetReaderSecret(ctx, tenantA, "gmail")), "GetReaderSecret after runtime delete")
	assertRuntimeMissing(t, runtimeBytes(backend.GetReaderToken(ctx, tenantA, "gmail")), "GetReaderToken after runtime delete")
	if _, found, err := backend.GetReaderConfig(ctx, tenantA, "gmail"); err != nil || found {
		t.Fatalf("GetReaderConfig after runtime delete found=%v err=%v", found, err)
	}
}

//nolint:gocognit // One scenario checks the complete LLM runtime lifecycle and isolation contract.
func testLLMRuntime(ctx context.Context, t *testing.T, backend RuntimeSubject) {
	t.Helper()

	tenantA := createTenant(ctx, t, backend, "llm-a")
	tenantB := createTenant(ctx, t, backend, "llm-b")
	configA := json.RawMessage(`{"model":"a"}`)
	credentialsA := []byte(`{"api_key":"a"}`)
	if err := backend.SetLLMProviderConfig(ctx, tenantA, "shared", configA); err != nil {
		t.Fatalf("SetLLMProviderConfig: %v", err)
	}
	if err := backend.SetLLMProviderCredentials(ctx, tenantA, "shared", credentialsA); err != nil {
		t.Fatalf("SetLLMProviderCredentials: %v", err)
	}
	config, found, err := backend.GetLLMProviderConfig(ctx, tenantA, "shared")
	if err != nil || !found {
		t.Fatalf("GetLLMProviderConfig found=%v err=%v", found, err)
	}
	assertJSON(t, config, configA)
	assertRuntimeBytes(t, runtimeBytes(backend.GetLLMProviderCredentials(ctx, tenantA, "shared")), credentialsA, "GetLLMProviderCredentials")
	if _, tenantFound, err := backend.GetLLMProviderConfig(ctx, tenantB, "shared"); err != nil || tenantFound {
		t.Fatalf("GetLLMProviderConfig tenant isolation found=%v err=%v", tenantFound, err)
	}
	assertRuntimeMissing(t, runtimeBytes(backend.GetLLMProviderCredentials(ctx, tenantB, "shared")), "GetLLMProviderCredentials tenant isolation")

	invalid := []byte(`{"broken"`)
	assertErrorKind(t, backend.SetLLMProviderConfig(ctx, tenantA, "invalid", invalid), errors.InvalidInput, "SetLLMProviderConfig invalid JSON")
	assertErrorKind(t, backend.SetLLMProviderCredentials(ctx, tenantA, "invalid", invalid), errors.InvalidInput, "SetLLMProviderCredentials invalid JSON")
	if _, invalidFound, err := backend.GetLLMProviderConfig(ctx, tenantA, "invalid"); err != nil || invalidFound {
		t.Fatalf("GetLLMProviderConfig after invalid JSON found=%v err=%v", invalidFound, err)
	}
	assertRuntimeMissing(t, runtimeBytes(backend.GetLLMProviderCredentials(ctx, tenantA, "invalid")), "GetLLMProviderCredentials after invalid JSON")
	assertErrorKind(t, backend.SetLLMProviderConfig(ctx, tenantA, "shared", invalid), errors.InvalidInput, "SetLLMProviderConfig invalid replacement")
	assertErrorKind(t, backend.SetLLMProviderCredentials(ctx, tenantA, "shared", invalid), errors.InvalidInput, "SetLLMProviderCredentials invalid replacement")
	config, found, err = backend.GetLLMProviderConfig(ctx, tenantA, "shared")
	if err != nil || !found {
		t.Fatalf("GetLLMProviderConfig after invalid replacement found=%v err=%v", found, err)
	}
	assertJSON(t, config, configA)
	assertRuntimeBytes(
		t,
		runtimeBytes(backend.GetLLMProviderCredentials(ctx, tenantA, "shared")),
		credentialsA,
		"GetLLMProviderCredentials after invalid replacement",
	)

	if err := backend.SetActiveLLMProvider(ctx, tenantA, "shared"); err != nil {
		t.Fatalf("SetActiveLLMProvider shared tenant A: %v", err)
	}
	if err := backend.SetActiveLLMProvider(ctx, tenantA, "other"); err != nil {
		t.Fatalf("SetActiveLLMProvider other tenant A: %v", err)
	}
	runtime, found, err := backend.GetActiveLLMProviderRuntime(ctx, tenantA)
	if err != nil || !found || runtime.Provider != "other" {
		t.Fatalf("tenant A active runtime = %#v found=%v err=%v, want other", runtime, found, err)
	}
	configB := json.RawMessage(`{"model":"b"}`)
	credentialsB := []byte(`{"api_key":"b"}`)
	if err := backend.SetLLMProviderConfig(ctx, tenantB, "shared", configB); err != nil {
		t.Fatalf("SetLLMProviderConfig tenant B: %v", err)
	}
	if err := backend.SetLLMProviderCredentials(ctx, tenantB, "shared", credentialsB); err != nil {
		t.Fatalf("SetLLMProviderCredentials tenant B: %v", err)
	}
	if err := backend.SetActiveLLMProvider(ctx, tenantB, "shared"); err != nil {
		t.Fatalf("SetActiveLLMProvider shared tenant B: %v", err)
	}
	runtime, found, err = backend.GetActiveLLMProviderRuntime(ctx, tenantB)
	if err != nil || !found || runtime.Provider != "shared" {
		t.Fatalf("tenant B active runtime = %#v found=%v err=%v, want shared", runtime, found, err)
	}
	runtime, found, err = backend.GetActiveLLMProviderRuntime(ctx, tenantA)
	if err != nil || !found || runtime.Provider != "other" {
		t.Fatalf("tenant A active runtime after tenant B activation = %#v found=%v err=%v, want other", runtime, found, err)
	}
	if err := backend.ClearActiveLLMProvider(ctx, tenantA); err != nil {
		t.Fatalf("ClearActiveLLMProvider: %v", err)
	}
	if _, found, err := backend.GetActiveLLMProviderRuntime(ctx, tenantA); err != nil || found {
		t.Fatalf("GetActiveLLMProviderRuntime after clear found=%v err=%v", found, err)
	}
	if err := backend.DeleteLLMProviderRuntime(ctx, tenantB, "shared"); err != nil {
		t.Fatalf("DeleteLLMProviderRuntime: %v", err)
	}
	if _, found, err := backend.GetLLMProviderConfig(ctx, tenantB, "shared"); err != nil || found {
		t.Fatalf("GetLLMProviderConfig after delete found=%v err=%v", found, err)
	}
	assertRuntimeMissing(t, runtimeBytes(backend.GetLLMProviderCredentials(ctx, tenantB, "shared")), "GetLLMProviderCredentials after delete")
}

func testRuntimeGlobalSettings(ctx context.Context, t *testing.T, backend RuntimeSubject) {
	t.Helper()

	settings, err := backend.GetCommunitySyncSettings(ctx)
	if err != nil {
		t.Fatalf("GetCommunitySyncSettings default: %v", err)
	}
	if settings.AutomaticSyncEnabled == nil || !*settings.AutomaticSyncEnabled {
		t.Fatalf("GetCommunitySyncSettings default = %#v, want true", settings)
	}
	settings, err = backend.PatchCommunitySyncSettings(ctx, store.CommunitySyncSettingsPatch{})
	if err != nil || settings.AutomaticSyncEnabled == nil || !*settings.AutomaticSyncEnabled {
		t.Fatalf("PatchCommunitySyncSettings nil = %#v / %v, want unchanged true", settings, err)
	}
	status, err := backend.GetSyncStatus(ctx)
	if err != nil || !reflect.DeepEqual(status, store.SyncStatus{}) {
		t.Fatalf("GetSyncStatus missing = %#v / %v, want zero", status, err)
	}
	if err := backend.SetCommunityURL(ctx, "https://global.example.test"); err != nil {
		t.Fatalf("SetCommunityURL: %v", err)
	}
	url, err := backend.GetCommunityURL(ctx)
	if err != nil || url != "https://global.example.test" {
		t.Fatalf("GetCommunityURL = %q / %v", url, err)
	}
	failed := "sync failed"
	syncedAt := time.Date(2026, time.September, 15, 13, 0, 0, 0, time.UTC)
	wantStatus := store.SyncStatus{LastSyncedAt: &syncedAt, Error: &failed, EntriesUpdated: 7}
	if err := backend.SetSyncStatus(ctx, wantStatus); err != nil {
		t.Fatalf("SetSyncStatus: %v", err)
	}
	status, err = backend.GetSyncStatus(ctx)
	if err != nil || !reflect.DeepEqual(status, wantStatus) {
		t.Fatalf("GetSyncStatus = %#v / %v, want %#v", status, err, wantStatus)
	}
}

func testRuntimeUserCascade(ctx context.Context, t *testing.T, backend RuntimeSubject) {
	t.Helper()

	user := createUser(ctx, t, backend, "runtime-cascade")
	tenant := store.Tenant{ID: user.TenantID}
	if err := backend.SetAppConfig(ctx, tenant, "key", "value"); err != nil {
		t.Fatalf("SetAppConfig: %v", err)
	}
	if err := backend.SetReaderConfig(ctx, tenant, "reader", json.RawMessage(`{"value":true}`)); err != nil {
		t.Fatalf("SetReaderConfig: %v", err)
	}
	if err := backend.SetLLMProviderConfig(ctx, tenant, "provider", json.RawMessage(`{"value":true}`)); err != nil {
		t.Fatalf("SetLLMProviderConfig: %v", err)
	}
	if err := backend.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := backend.GetAppConfig(ctx, tenant, "key"); err == nil {
		t.Fatal("GetAppConfig after user delete returned nil error")
	}
	if _, found, err := backend.GetReaderConfig(ctx, tenant, "reader"); err != nil || found {
		t.Fatalf("GetReaderConfig after user delete found=%v err=%v", found, err)
	}
	if _, found, err := backend.GetLLMProviderConfig(ctx, tenant, "provider"); err != nil || found {
		t.Fatalf("GetLLMProviderConfig after user delete found=%v err=%v", found, err)
	}
}

func testRuntimeWithoutSecret(ctx context.Context, t *testing.T, backend RuntimeSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "runtime-no-secret")
	valid := []byte(`{"value":true}`)
	assertErrorKind(t, backend.SetReaderSecret(ctx, tenant, "reader", valid), errors.FailedPrecondition, "SetReaderSecret without SecretBox")
	_, _, err := backend.GetReaderSecret(ctx, tenant, "reader")
	assertErrorKind(t, err, errors.FailedPrecondition, "GetReaderSecret without SecretBox")
	assertErrorKind(t, backend.SetReaderToken(ctx, tenant, "reader", valid), errors.FailedPrecondition, "SetReaderToken without SecretBox")
	_, _, err = backend.GetReaderToken(ctx, tenant, "reader")
	assertErrorKind(t, err, errors.FailedPrecondition, "GetReaderToken without SecretBox")
	assertErrorKind(t, backend.SetLLMProviderCredentials(ctx, tenant, "provider", valid), errors.FailedPrecondition, "SetLLMProviderCredentials without SecretBox")
	_, _, err = backend.GetLLMProviderCredentials(ctx, tenant, "provider")
	assertErrorKind(t, err, errors.FailedPrecondition, "GetLLMProviderCredentials without SecretBox")
}

func testScanning(ctx context.Context, t *testing.T, backend ScanningSubject) {
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

func testScanningScheduler(ctx context.Context, t *testing.T, backend ScanningSubject) {
	t.Helper()

	cfg, err := backend.GetSchedulerConfig(ctx)
	if err != nil {
		t.Fatalf("GetSchedulerConfig: %v", err)
	}
	if cfg.MaxConcurrentScans != 4 {
		t.Fatalf("GetSchedulerConfig MaxConcurrentScans = %d, want 4", cfg.MaxConcurrentScans)
	}
	unchanged, err := backend.PatchSchedulerConfig(ctx, store.SchedulerConfigPatch{})
	if err != nil || unchanged.MaxConcurrentScans != cfg.MaxConcurrentScans {
		t.Fatalf("PatchSchedulerConfig nil = %#v / %v, want %#v", unchanged, err, cfg)
	}
	for _, invalid := range []int{0, 65} {
		_, err := backend.PatchSchedulerConfig(ctx, store.SchedulerConfigPatch{MaxConcurrentScans: &invalid})
		assertErrorKind(t, err, errors.InvalidInput, fmt.Sprintf("PatchSchedulerConfig %d", invalid))
	}
	for _, boundary := range []int{1, 64} {
		updated, err := backend.PatchSchedulerConfig(ctx, store.SchedulerConfigPatch{MaxConcurrentScans: &boundary})
		if err != nil || updated.MaxConcurrentScans != boundary {
			t.Fatalf("PatchSchedulerConfig %d = %#v / %v", boundary, updated, err)
		}
	}
}

//nolint:gocognit // One scenario checks all scanning state transitions and rollback behavior.
func testScanningStateLifecycle(ctx context.Context, t *testing.T, backend ScanningSubject) {
	t.Helper()

	assertErrorKind(t, backend.EnsureScanningStateForTenant(ctx, store.Tenant{}), errors.InvalidInput, "EnsureScanningStateForTenant empty tenant")
	missing := store.Tenant{ID: "00000000-0000-0000-0000-000000000000"}
	if err := backend.EnsureScanningStateForTenant(ctx, missing); err != nil {
		t.Fatalf("EnsureScanningStateForTenant missing user: %v", err)
	}
	_, err := backend.GetScanningState(ctx, missing)
	assertErrorKind(t, err, errors.NotFound, "GetScanningState missing user")

	tenant := createTenant(ctx, t, backend, "scanning-lifecycle")
	if err := backend.EnsureScanningStateForTenant(ctx, tenant); err != nil {
		t.Fatalf("EnsureScanningStateForTenant: %v", err)
	}
	state, err := backend.GetScanningState(ctx, tenant)
	if err != nil || state.ActiveReader != "" || !state.Enabled || state.State != store.ScanningStateStopped {
		t.Fatalf("initial scanning state = %#v / %v", state, err)
	}
	if err := backend.ClearActiveScanningReader(ctx, tenant); err != nil {
		t.Fatalf("ClearActiveScanningReader: %v", err)
	}
	state, err = backend.GetScanningState(ctx, tenant)
	if err != nil || state.ActiveReader != "" || state.Enabled || state.State != store.ScanningStateStopped {
		t.Fatalf("cleared scanning state = %#v / %v", state, err)
	}
	if err := backend.SetScanningEnabled(ctx, tenant, true); err != nil {
		t.Fatalf("SetScanningEnabled without reader: %v", err)
	}
	state, err = backend.GetScanningState(ctx, tenant)
	if err != nil || !state.Enabled || state.State != store.ScanningStateStopped {
		t.Fatalf("enabled scanning state without reader = %#v / %v", state, err)
	}
	if err := backend.SetActiveScanningReader(ctx, tenant, "gmail"); err != nil {
		t.Fatalf("SetActiveScanningReader: %v", err)
	}
	state, err = backend.GetScanningState(ctx, tenant)
	if err != nil || !state.Enabled || state.ActiveReader != "gmail" || state.State != store.ScanningStateQueued {
		t.Fatalf("scanning state with reader = %#v / %v", state, err)
	}
	if err := backend.SetScanningEnabled(ctx, tenant, false); err != nil {
		t.Fatalf("SetScanningEnabled false: %v", err)
	}
	if err := backend.SetScanningEnabled(ctx, tenant, true); err != nil {
		t.Fatalf("SetScanningEnabled true with reader: %v", err)
	}
	state, err = backend.GetScanningState(ctx, tenant)
	if err != nil || !state.Enabled || state.State != store.ScanningStateQueued {
		t.Fatalf("re-enabled scanning state with reader = %#v / %v", state, err)
	}

	negative := -1
	before := state
	err = backend.UpdateScanningState(ctx, tenant, store.ScanningStateUpdate{
		State: store.ScanningStateBackingOff, RetryCount: &negative,
	})
	assertErrorKind(t, err, errors.InvalidInput, "UpdateScanningState negative retry count")
	assertErrorOperation(t, err, "store.scanning.update_scanning_state", "UpdateScanningState negative retry count")
	after, err := backend.GetScanningState(ctx, tenant)
	if err != nil {
		t.Fatalf("GetScanningState after negative retry count: %v", err)
	}
	timesEqual := func(left, right *time.Time) bool {
		if left == nil || right == nil {
			return left == nil && right == nil
		}
		return left.Equal(*right)
	}
	if after.TenantID != before.TenantID ||
		after.ActiveReader != before.ActiveReader ||
		after.Enabled != before.Enabled ||
		after.State != before.State ||
		after.ReasonCode != before.ReasonCode ||
		after.PublicMessage != before.PublicMessage ||
		!timesEqual(after.LastStartedAt, before.LastStartedAt) ||
		!timesEqual(after.LastStoppedAt, before.LastStoppedAt) ||
		!timesEqual(after.LastFailedAt, before.LastFailedAt) ||
		!timesEqual(after.NextRetryAt, before.NextRetryAt) ||
		after.RetryCount != before.RetryCount ||
		!after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("negative retry count changed scanning state: before=%#v after=%#v", before, after)
	}
}

//nolint:gocognit // The table setup and listing assertions form one scanning scheduler scenario.
func testScanningListingsAndCascade(ctx context.Context, t *testing.T, backend ScanningSubject) {
	t.Helper()

	type scanningCase struct {
		name      string
		state     store.ScanningState
		nextRetry *time.Time
		paused    bool
		runnable  bool
		user      *store.User
	}
	past := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	future := time.Date(2100, time.January, 1, 0, 0, 0, 0, time.UTC)
	cases := []scanningCase{
		{name: "queued", state: store.ScanningStateQueued, runnable: true},
		{name: "elapsed", state: store.ScanningStateBackingOff, nextRetry: &past, runnable: true},
		{name: "future", state: store.ScanningStateBackingOff, nextRetry: &future},
		{name: "needs-auth", state: store.ScanningStateNeedsAuth},
		{name: "reader-missing", state: store.ScanningStateReaderNotConfigured},
		{name: "paused", state: store.ScanningStatePaused, paused: true},
	}
	for i := range cases {
		cases[i].user = createUser(ctx, t, backend, "scanning-"+cases[i].name)
		tenant := store.Tenant{ID: cases[i].user.TenantID}
		if err := backend.SetActiveScanningReader(ctx, tenant, "reader"); err != nil {
			t.Fatalf("SetActiveScanningReader %s: %v", cases[i].name, err)
		}
		if cases[i].paused {
			if err := backend.SetScanningEnabled(ctx, tenant, false); err != nil {
				t.Fatalf("SetScanningEnabled %s: %v", cases[i].name, err)
			}
			continue
		}
		if err := backend.UpdateScanningState(ctx, tenant, store.ScanningStateUpdate{
			State: cases[i].state, NextRetryAt: cases[i].nextRetry,
		}); err != nil {
			t.Fatalf("UpdateScanningState %s: %v", cases[i].name, err)
		}
	}

	all, err := backend.ListScanningStates(ctx)
	if err != nil {
		t.Fatalf("ListScanningStates: %v", err)
	}
	if len(all) != len(cases) {
		t.Fatalf("ListScanningStates len = %d, want %d: %#v", len(all), len(cases), all)
	}
	runnable, err := backend.ListRunnableScanningStates(ctx)
	if err != nil {
		t.Fatalf("ListRunnableScanningStates: %v", err)
	}
	wantRunnable := make(map[string]bool)
	for _, tc := range cases {
		if tc.runnable {
			wantRunnable[tc.user.TenantID] = true
		}
	}
	if len(runnable) != len(wantRunnable) {
		t.Fatalf("ListRunnableScanningStates len = %d, want %d: %#v", len(runnable), len(wantRunnable), runnable)
	}
	for _, state := range runnable {
		if !wantRunnable[state.TenantID] {
			t.Fatalf("ListRunnableScanningStates included blocked tenant: %#v", state)
		}
	}

	deleted := cases[1].user
	if err := backend.DeleteUser(ctx, deleted.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	all, err = backend.ListScanningStates(ctx)
	if err != nil {
		t.Fatalf("ListScanningStates after user delete: %v", err)
	}
	if hasScanningTenant(all, deleted.TenantID) {
		t.Fatalf("ListScanningStates retained deleted tenant %q: %#v", deleted.TenantID, all)
	}
}

func testTaxonomy(ctx context.Context, t *testing.T, backend TaxonomySubject) {
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

//nolint:gocognit // One scenario checks default visibility, overrides, and duplicate handling.
func testTaxonomyVisibilityAndOverrides(ctx context.Context, t *testing.T, backend TaxonomySubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "taxonomy-visibility")
	labels, err := backend.ListLabels(ctx, tenant)
	if err != nil || labels == nil || len(labels) != 0 {
		t.Fatalf("ListLabels empty = %#v / %v, want non-nil empty", labels, err)
	}
	categories, err := backend.ListCategories(ctx, tenant)
	if err != nil || !hasCategoryWithValues(categories, "Food & Dining", "", true) {
		t.Fatalf("ListCategories global defaults = %#v / %v", categories, err)
	}
	buckets, err := backend.ListBuckets(ctx, tenant)
	if err != nil || !hasBucketWithValues(buckets, "Needs", "", true) {
		t.Fatalf("ListBuckets global defaults = %#v / %v", buckets, err)
	}

	if err := backend.CreateCategory(ctx, tenant, "Food & Dining", "tenant category"); err != nil {
		t.Fatalf("CreateCategory override: %v", err)
	}
	if err := backend.CreateCategory(ctx, tenant, "Food & Dining", "ignored duplicate"); err != nil {
		t.Fatalf("CreateCategory duplicate: %v", err)
	}
	categories, err = backend.ListCategories(ctx, tenant)
	if err != nil || countCategory(categories, "Food & Dining") != 1 ||
		!hasCategoryWithValues(categories, "Food & Dining", "tenant category", false) {
		t.Fatalf("ListCategories tenant override = %#v / %v", categories, err)
	}
	if err := backend.DeleteCategory(ctx, tenant, "Food & Dining", false); err != nil {
		t.Fatalf("DeleteCategory tenant override: %v", err)
	}
	categories, err = backend.ListCategories(ctx, tenant)
	if err != nil || countCategory(categories, "Food & Dining") != 1 ||
		!hasCategoryWithValues(categories, "Food & Dining", "", true) {
		t.Fatalf("ListCategories after tenant override delete = %#v / %v", categories, err)
	}

	if err := backend.CreateBucket(ctx, tenant, "Needs", "tenant bucket"); err != nil {
		t.Fatalf("CreateBucket override: %v", err)
	}
	if err := backend.CreateBucket(ctx, tenant, "Needs", "ignored duplicate"); err != nil {
		t.Fatalf("CreateBucket duplicate: %v", err)
	}
	buckets, err = backend.ListBuckets(ctx, tenant)
	if err != nil || countBucket(buckets, "Needs") != 1 ||
		!hasBucketWithValues(buckets, "Needs", "tenant bucket", false) {
		t.Fatalf("ListBuckets tenant override = %#v / %v", buckets, err)
	}
	if err := backend.DeleteBucket(ctx, tenant, "Needs", false); err != nil {
		t.Fatalf("DeleteBucket tenant override: %v", err)
	}
	buckets, err = backend.ListBuckets(ctx, tenant)
	if err != nil || countBucket(buckets, "Needs") != 1 ||
		!hasBucketWithValues(buckets, "Needs", "", true) {
		t.Fatalf("ListBuckets after tenant override delete = %#v / %v", buckets, err)
	}

	if err := backend.CreateLabel(ctx, tenant, "duplicate", "#111111"); err != nil {
		t.Fatalf("CreateLabel: %v", err)
	}
	if err := backend.CreateLabel(ctx, tenant, "duplicate", "#222222"); err != nil {
		t.Fatalf("CreateLabel duplicate: %v", err)
	}
	labels, err = backend.ListLabels(ctx, tenant)
	if err != nil || len(labels) != 1 || !hasLabel(labels, "duplicate", "#111111") {
		t.Fatalf("ListLabels after duplicate create = %#v / %v", labels, err)
	}
}

func testTaxonomyMissingAndDefaultDeletes(ctx context.Context, t *testing.T, backend TaxonomySubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "taxonomy-delete-errors")
	assertErrorKind(t, backend.UpdateLabel(ctx, tenant, "missing", "#000000"), errors.NotFound, "UpdateLabel missing")
	assertErrorKind(t, backend.DeleteCategory(ctx, tenant, "Missing Category", true), errors.NotFound, "DeleteCategory missing")
	assertErrorKind(t, backend.DeleteBucket(ctx, tenant, "Missing Bucket", true), errors.NotFound, "DeleteBucket missing")

	beforeCategories, err := backend.ListCategories(ctx, tenant)
	if err != nil {
		t.Fatalf("ListCategories before default delete: %v", err)
	}
	assertErrorKind(t, backend.DeleteCategory(ctx, tenant, "Food & Dining", true), errors.Conflict, "DeleteCategory default")
	afterCategories, err := backend.ListCategories(ctx, tenant)
	if err != nil || !reflect.DeepEqual(afterCategories, beforeCategories) {
		t.Fatalf("default category delete changed rows: before=%#v after=%#v err=%v", beforeCategories, afterCategories, err)
	}

	beforeBuckets, err := backend.ListBuckets(ctx, tenant)
	if err != nil {
		t.Fatalf("ListBuckets before default delete: %v", err)
	}
	assertErrorKind(t, backend.DeleteBucket(ctx, tenant, "Needs", true), errors.Conflict, "DeleteBucket default")
	afterBuckets, err := backend.ListBuckets(ctx, tenant)
	if err != nil || !reflect.DeepEqual(afterBuckets, beforeBuckets) {
		t.Fatalf("default bucket delete changed rows: before=%#v after=%#v err=%v", beforeBuckets, afterBuckets, err)
	}
}

//nolint:gocognit // One scenario checks mapping mutation, pruning, and tenant isolation.
func testTaxonomyMappings(ctx context.Context, t *testing.T, backend TaxonomySubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "taxonomy-mappings")
	otherTenant := createTenant(ctx, t, backend, "taxonomy-mappings-other")
	if err := backend.CreateLabel(ctx, tenant, "mapped", "#123456"); err != nil {
		t.Fatalf("CreateLabel: %v", err)
	}
	if _, err := backend.ApplyLabelByMerchant(ctx, tenant, "mapped", "Merchant"); err != nil {
		t.Fatalf("ApplyLabelByMerchant: %v", err)
	}
	if err := backend.DeleteLabel(ctx, tenant, "mapped", false); err != nil {
		t.Fatalf("DeleteLabel: %v", err)
	}
	labelMappings, err := backend.GetLabelMappings(ctx, tenant)
	if err != nil || len(labelMappings) != 0 {
		t.Fatalf("GetLabelMappings after label delete = %#v / %v, want empty", labelMappings, err)
	}

	if err := backend.CreateCategory(ctx, tenant, "Mapped Category", ""); err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if err := backend.CreateBucket(ctx, tenant, "Mapped Bucket", ""); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	if _, err := backend.ApplyCategoryByMerchant(ctx, tenant, "Mapped Category", "Shared Merchant"); err != nil {
		t.Fatalf("ApplyCategoryByMerchant: %v", err)
	}
	if _, err := backend.ApplyBucketByMerchant(ctx, tenant, "Mapped Bucket", "Shared Merchant"); err != nil {
		t.Fatalf("ApplyBucketByMerchant: %v", err)
	}
	if _, err := backend.RemoveCategoryByMerchant(ctx, tenant, "Mapped Category", "Shared Merchant"); err != nil {
		t.Fatalf("RemoveCategoryByMerchant: %v", err)
	}
	categoryMappings, err := backend.GetCategoryMappings(ctx, tenant)
	if err != nil || len(categoryMappings) != 0 {
		t.Fatalf("GetCategoryMappings after category removal = %#v / %v", categoryMappings, err)
	}
	bucketMappings, err := backend.GetBucketMappings(ctx, tenant)
	if err != nil || !mappingContains(bucketMappings, "Mapped Bucket", "Shared Merchant") {
		t.Fatalf("GetBucketMappings after category removal = %#v / %v", bucketMappings, err)
	}
	if _, err := backend.RemoveBucketByMerchant(ctx, tenant, "Mapped Bucket", "Shared Merchant"); err != nil {
		t.Fatalf("RemoveBucketByMerchant: %v", err)
	}
	bucketMappings, err = backend.GetBucketMappings(ctx, tenant)
	if err != nil || len(bucketMappings) != 0 {
		t.Fatalf("GetBucketMappings after empty mapping prune = %#v / %v", bucketMappings, err)
	}

	if _, err := backend.ApplyCategoryByMerchant(ctx, tenant, "Mapped Category", "Category Only"); err != nil {
		t.Fatalf("ApplyCategoryByMerchant category-only: %v", err)
	}
	if _, err := backend.ApplyBucketByMerchant(ctx, tenant, "Mapped Bucket", "Bucket Only"); err != nil {
		t.Fatalf("ApplyBucketByMerchant bucket-only: %v", err)
	}
	categoryMappings, err = backend.GetCategoryMappings(ctx, tenant)
	if err != nil ||
		!mappingContains(categoryMappings, "Mapped Category", "Category Only") ||
		mappingContains(categoryMappings, "Mapped Category", "Bucket Only") {
		t.Fatalf("category-only mappings = %#v / %v", categoryMappings, err)
	}
	bucketMappings, err = backend.GetBucketMappings(ctx, tenant)
	if err != nil || !mappingContains(bucketMappings, "Mapped Bucket", "Bucket Only") || mappingContains(bucketMappings, "Mapped Bucket", "Category Only") {
		t.Fatalf("bucket-only mappings = %#v / %v", bucketMappings, err)
	}
	otherLabelMappings, err := backend.GetLabelMappings(ctx, otherTenant)
	if err != nil || len(otherLabelMappings) != 0 {
		t.Fatalf("other tenant label mappings = %#v / %v, want empty", otherLabelMappings, err)
	}
	otherCategoryMappings, err := backend.GetCategoryMappings(ctx, otherTenant)
	if err != nil || len(otherCategoryMappings) != 0 {
		t.Fatalf("other tenant category mappings = %#v / %v, want empty", otherCategoryMappings, err)
	}
	otherBucketMappings, err := backend.GetBucketMappings(ctx, otherTenant)
	if err != nil || len(otherBucketMappings) != 0 {
		t.Fatalf("other tenant bucket mappings = %#v / %v, want empty", otherBucketMappings, err)
	}
}

//nolint:gocognit // One scenario checks taxonomy mutations against ingested transactions.
func testTaxonomyTransactions(ctx context.Context, t *testing.T, backend TaxonomyTransactionSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "taxonomy-transactions")
	label := "conf-label"
	category := "Conf Category"
	bucket := "Conf Bucket"
	merchantName := "Taxonomy Merchant"

	if err := backend.CreateLabel(ctx, tenant, label, "#38bdf8"); err != nil {
		t.Fatalf("CreateLabel: %v", err)
	}
	if err := backend.CreateCategory(ctx, tenant, category, "conformance category"); err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if err := backend.CreateBucket(ctx, tenant, bucket, "conformance bucket"); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	if err := backend.Write(ctx, store.IngestionBatch{
		Tenant: tenant,
		Transactions: []*api.TransactionDetails{{
			MessageID:    "taxonomy-" + suffix(t),
			Amount:       42,
			Currency:     baseCurrencyINR,
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

func testTaxonomyDeleteTransactionCleanup(ctx context.Context, t *testing.T, backend TaxonomyTransactionSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "taxonomy-cleanup")
	for _, label := range []string{"Keep Label", "Clear Label"} {
		if err := backend.CreateLabel(ctx, tenant, label, "#123456"); err != nil {
			t.Fatalf("CreateLabel %q: %v", label, err)
		}
	}
	for _, category := range []string{"Keep Category", "Clear Category"} {
		if err := backend.CreateCategory(ctx, tenant, category, ""); err != nil {
			t.Fatalf("CreateCategory %q: %v", category, err)
		}
	}
	for _, bucket := range []string{"Keep Bucket", "Clear Bucket"} {
		if err := backend.CreateBucket(ctx, tenant, bucket, ""); err != nil {
			t.Fatalf("CreateBucket %q: %v", bucket, err)
		}
	}
	if err := backend.Write(ctx, store.IngestionBatch{Tenant: tenant, Transactions: []*api.TransactionDetails{
		{
			MessageID: "taxonomy-keep", Amount: 1, Currency: baseCurrencyINR, Timestamp: "2026-01-01T00:00:00Z",
			MerchantInfo: "Keep Merchant", Category: "Keep Category", Bucket: "Keep Bucket", Labels: []string{"Keep Label"},
		},
		{
			MessageID: "taxonomy-clear", Amount: 2, Currency: baseCurrencyINR, Timestamp: "2026-01-02T00:00:00Z",
			MerchantInfo: "Clear Merchant", Category: "Clear Category", Bucket: "Clear Bucket", Labels: []string{"Clear Label"},
		},
	}}); err != nil {
		t.Fatalf("Write taxonomy cleanup transactions: %v", err)
	}

	if err := backend.DeleteLabel(ctx, tenant, "Keep Label", false); err != nil {
		t.Fatalf("DeleteLabel keep: %v", err)
	}
	if err := backend.DeleteCategory(ctx, tenant, "Keep Category", false); err != nil {
		t.Fatalf("DeleteCategory keep: %v", err)
	}
	if err := backend.DeleteBucket(ctx, tenant, "Keep Bucket", false); err != nil {
		t.Fatalf("DeleteBucket keep: %v", err)
	}
	if err := backend.DeleteLabel(ctx, tenant, "Clear Label", true); err != nil {
		t.Fatalf("DeleteLabel clear: %v", err)
	}
	if err := backend.DeleteCategory(ctx, tenant, "Clear Category", true); err != nil {
		t.Fatalf("DeleteCategory clear: %v", err)
	}
	if err := backend.DeleteBucket(ctx, tenant, "Clear Bucket", true); err != nil {
		t.Fatalf("DeleteBucket clear: %v", err)
	}

	keep := transactionByMessage(ctx, t, backend, tenant, "taxonomy-keep")
	if keep.Category != "Keep Category" || keep.Bucket != "Keep Bucket" || !containsString(keep.Labels, "Keep Label") {
		t.Fatalf("removeFromTransactions=false changed transaction: %#v", keep)
	}
	cleared := transactionByMessage(ctx, t, backend, tenant, "taxonomy-clear")
	if cleared.Category != "" || cleared.Bucket != "" || containsString(cleared.Labels, "Clear Label") {
		t.Fatalf("removeFromTransactions=true did not clear transaction: %#v", cleared)
	}
}

func testTaxonomyMerchantMatching(ctx context.Context, t *testing.T, backend TaxonomyTransactionSubject) {
	t.Helper()

	tenantA := createTenant(ctx, t, backend, "taxonomy-match-a")
	tenantB := createTenant(ctx, t, backend, "taxonomy-match-b")
	for _, tenant := range []store.Tenant{tenantA, tenantB} {
		if err := backend.CreateLabel(ctx, tenant, "Matched", "#123456"); err != nil {
			t.Fatalf("CreateLabel: %v", err)
		}
		if err := backend.CreateCategory(ctx, tenant, "Matched Category", ""); err != nil {
			t.Fatalf("CreateCategory: %v", err)
		}
	}
	for _, tenant := range []store.Tenant{tenantA, tenantB} {
		if err := backend.Write(ctx, store.IngestionBatch{Tenant: tenant, Transactions: []*api.TransactionDetails{{
			MessageID: "case-match", Amount: 1, Currency: baseCurrencyINR,
			Timestamp: "2026-01-01T00:00:00Z", MerchantInfo: "Acme SHOP",
		}}}); err != nil {
			t.Fatalf("Write merchant match transaction: %v", err)
		}
	}
	if affected, err := backend.ApplyLabelByMerchant(ctx, tenantA, "Matched", "acme shop"); err != nil || affected != 1 {
		t.Fatalf("ApplyLabelByMerchant case-insensitive affected=%d err=%v", affected, err)
	}
	if affected, err := backend.ApplyCategoryByMerchant(ctx, tenantA, "Matched Category", "Acme SHOP"); err != nil || affected != 1 {
		t.Fatalf("ApplyCategoryByMerchant affected=%d err=%v", affected, err)
	}
	gotA := transactionByMessage(ctx, t, backend, tenantA, "case-match")
	if gotA.Category != "Matched Category" || !containsString(gotA.Labels, "Matched") {
		t.Fatalf("tenant A merchant mapping not applied: %#v", gotA)
	}
	gotB := transactionByMessage(ctx, t, backend, tenantB, "case-match")
	if gotB.Category != "" || len(gotB.Labels) != 0 {
		t.Fatalf("tenant A merchant mapping leaked to tenant B: %#v", gotB)
	}
	mappings, err := backend.GetLabelMappings(ctx, tenantB)
	if err != nil || len(mappings) != 0 {
		t.Fatalf("tenant B label mappings = %#v / %v, want empty", mappings, err)
	}
}

func testTaxonomyLabelProvenance(ctx context.Context, t *testing.T, backend TaxonomyTransactionSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "taxonomy-provenance")
	for _, label := range []string{"Manual", "Overlap"} {
		if err := backend.CreateLabel(ctx, tenant, label, "#123456"); err != nil {
			t.Fatalf("CreateLabel %q: %v", label, err)
		}
	}
	if err := backend.Write(ctx, store.IngestionBatch{Tenant: tenant, Transactions: []*api.TransactionDetails{
		{
			MessageID: "manual-source", Amount: 1, Currency: baseCurrencyINR,
			Timestamp: "2026-01-01T00:00:00Z", MerchantInfo: "Manual Merchant", Labels: []string{"Manual"},
		},
		{
			MessageID: "overlap-source", Amount: 2, Currency: baseCurrencyINR,
			Timestamp: "2026-01-02T00:00:00Z", MerchantInfo: "Uber Eats Pass",
		},
	}}); err != nil {
		t.Fatalf("Write provenance transactions: %v", err)
	}
	if _, err := backend.ApplyLabelByMerchant(ctx, tenant, "Manual", "Manual Merchant"); err != nil {
		t.Fatalf("ApplyLabelByMerchant manual: %v", err)
	}
	if removed, err := backend.RemoveLabelByMerchant(ctx, tenant, "Manual", "Manual Merchant"); err != nil || removed != 0 {
		t.Fatalf("RemoveLabelByMerchant manual removed=%d err=%v", removed, err)
	}
	if got := transactionByMessage(ctx, t, backend, tenant, "manual-source"); !containsString(got.Labels, "Manual") {
		t.Fatalf("merchant removal removed manual label source: %#v", got)
	}

	if affected, err := backend.ApplyLabelByMerchant(ctx, tenant, "Overlap", "Uber"); err != nil || affected != 1 {
		t.Fatalf("ApplyLabelByMerchant Uber affected=%d err=%v", affected, err)
	}
	if affected, err := backend.ApplyLabelByMerchant(ctx, tenant, "Overlap", "Uber Eats"); err != nil || affected != 0 {
		t.Fatalf("ApplyLabelByMerchant Uber Eats affected=%d err=%v", affected, err)
	}
	if removed, err := backend.RemoveLabelByMerchant(ctx, tenant, "Overlap", "Uber"); err != nil || removed != 0 {
		t.Fatalf("RemoveLabelByMerchant overlapping source removed=%d err=%v", removed, err)
	}
	if got := transactionByMessage(ctx, t, backend, tenant, "overlap-source"); !containsString(got.Labels, "Overlap") {
		t.Fatalf("merchant removal removed label with another merchant source: %#v", got)
	}
}

func testRules(ctx context.Context, t *testing.T, backend RulesSubject) {
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

func testRulesVisibilityAndIsolation(ctx context.Context, t *testing.T, backend RulesSubject) {
	t.Helper()

	tenantA := createTenant(ctx, t, backend, "rules-visible-a")
	tenantB := createTenant(ctx, t, backend, "rules-visible-b")
	empty, err := backend.ListRules(ctx, tenantA)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("ListRules empty = %#v / %v, want non-nil empty", empty, err)
	}
	if err := backend.SeedPredefinedRules(ctx, []store.RuleRow{{
		Name: "Shared Rule", AmountRegex: "global-amount", MerchantRegex: "global-merchant",
	}}); err != nil {
		t.Fatalf("SeedPredefinedRules: %v", err)
	}
	private, err := backend.CreateRule(ctx, tenantA, store.RuleRow{Name: "Shared Rule", AmountRegex: "tenant-amount", MerchantRegex: "tenant-merchant"})
	if err != nil {
		t.Fatalf("CreateRule tenant A: %v", err)
	}
	rulesA, err := backend.ListRules(ctx, tenantA)
	if err != nil || countRulesNamed(rulesA, "Shared Rule") != 2 || countPredefinedRulesNamed(rulesA, "Shared Rule") != 1 {
		t.Fatalf("ListRules tenant A = %#v / %v", rulesA, err)
	}
	rulesB, err := backend.ListRules(ctx, tenantB)
	if err != nil || countRulesNamed(rulesB, "Shared Rule") != 1 || countPredefinedRulesNamed(rulesB, "Shared Rule") != 1 {
		t.Fatalf("ListRules tenant B = %#v / %v", rulesB, err)
	}
	_, err = backend.GetRule(ctx, tenantB, private.ID)
	assertErrorKind(t, err, errors.NotFound, "GetRule other tenant")
}

func testRulesUniqueness(ctx context.Context, t *testing.T, backend RulesSubject) {
	t.Helper()

	tenantA := createTenant(ctx, t, backend, "rules-unique-a")
	tenantB := createTenant(ctx, t, backend, "rules-unique-b")
	first, err := backend.CreateRule(ctx, tenantA, store.RuleRow{Name: "Tenant Rule", AmountRegex: "first", MerchantRegex: "merchant"})
	if err != nil {
		t.Fatalf("CreateRule first: %v", err)
	}
	_, err = backend.CreateRule(ctx, tenantA, store.RuleRow{Name: first.Name, AmountRegex: "duplicate", MerchantRegex: "merchant"})
	assertErrorKind(t, err, errors.Conflict, "CreateRule duplicate tenant name")
	rulesA, err := backend.ListRules(ctx, tenantA)
	if err != nil || countRulesNamed(rulesA, first.Name) != 1 || ruleNamed(rulesA, first.Name).AmountRegex != "first" {
		t.Fatalf("duplicate create changed tenant A rules: %#v / %v", rulesA, err)
	}
	if _, err := backend.CreateRule(ctx, tenantB, store.RuleRow{Name: first.Name, AmountRegex: "tenant-b", MerchantRegex: "merchant"}); err != nil {
		t.Fatalf("CreateRule same name tenant B: %v", err)
	}

	second, err := backend.CreateRule(ctx, tenantA, store.RuleRow{Name: "Second Rule", AmountRegex: "second", MerchantRegex: "merchant"})
	if err != nil {
		t.Fatalf("CreateRule second: %v", err)
	}
	second.Name = first.Name
	_, err = backend.UpdateRule(ctx, tenantA, second.ID, *second)
	assertErrorKind(t, err, errors.Conflict, "UpdateRule duplicate tenant name")
	unchanged, err := backend.GetRule(ctx, tenantA, second.ID)
	if err != nil || unchanged.Name != "Second Rule" {
		t.Fatalf("failed UpdateRule changed row = %#v / %v", unchanged, err)
	}
}

func testRulesSeedAndPredefinedSemantics(ctx context.Context, t *testing.T, backend RulesSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "rules-seed")
	original := store.RuleRow{
		Name: "Seeded Rule", SenderEmails: []string{"first@example.test", "second@example.test"},
		SubjectContains: "original", AmountRegex: "original-amount", MerchantRegex: "original-merchant",
		CurrencyRegex: "original-currency", TransactionSource: "Original Source", SourceType: "card", SourceLabel: "Original Label", Bank: "Original Bank",
	}
	if err := backend.SeedPredefinedRules(ctx, []store.RuleRow{original}); err != nil {
		t.Fatalf("SeedPredefinedRules first: %v", err)
	}
	changed := original
	changed.SubjectContains = "seed-overwrite"
	changed.SenderEmails = []string{"changed@example.test"}
	if err := backend.SeedPredefinedRules(ctx, []store.RuleRow{changed}); err != nil {
		t.Fatalf("SeedPredefinedRules second: %v", err)
	}
	rules, err := backend.ListRules(ctx, tenant)
	seeded := ruleNamed(rules, original.Name)
	if err != nil || countRulesNamed(rules, original.Name) != 1 || seeded == nil || !seeded.Predefined || seeded.SubjectContains != original.SubjectContains ||
		!reflect.DeepEqual(seeded.SenderEmails, original.SenderEmails) {
		t.Fatalf("reseed changed predefined rule: %#v / %v", seeded, err)
	}
	seeded.SubjectContains = "public update"
	_, err = backend.UpdateRule(ctx, tenant, seeded.ID, *seeded)
	assertErrorKind(t, err, errors.NotFound, "UpdateRule predefined")
	unchanged, err := backend.GetRule(ctx, tenant, seeded.ID)
	if err != nil || unchanged.SubjectContains != original.SubjectContains {
		t.Fatalf("failed predefined update changed row = %#v / %v", unchanged, err)
	}
	assertErrorKind(t, backend.DeleteRule(ctx, tenant, seeded.ID), errors.NotFound, "DeleteRule predefined")
	if _, err := backend.GetRule(ctx, tenant, seeded.ID); err != nil {
		t.Fatalf("GetRule after rejected predefined delete: %v", err)
	}

	missingID := "00000000-0000-0000-0000-000000000000"
	_, err = backend.UpdateRule(ctx, tenant, missingID, store.RuleRow{Name: "Missing", AmountRegex: "amount", MerchantRegex: "merchant"})
	assertErrorKind(t, err, errors.NotFound, "UpdateRule missing")
}

func testRulesImportAndRoundTrip(ctx context.Context, t *testing.T, backend RulesSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "rules-import")
	legacy := store.RuleRow{
		Name: "Imported Rule", SenderEmail: "legacy@example.test", SubjectContains: "legacy subject",
		AmountRegex: "legacy amount", MerchantRegex: "legacy merchant", CurrencyRegex: "legacy currency", TransactionSource: "Legacy Source",
	}
	if err := backend.ImportUserRules(ctx, tenant, []store.RuleRow{legacy, {
		Name: "Second Imported Rule", SenderEmails: []string{"second@example.test"}, AmountRegex: "second amount", MerchantRegex: "second merchant",
	}}); err != nil {
		t.Fatalf("ImportUserRules create: %v", err)
	}
	rules, err := backend.ListRules(ctx, tenant)
	got := ruleNamed(rules, legacy.Name)
	if err != nil || got == nil || got.SenderEmail != legacy.SenderEmail || !reflect.DeepEqual(got.SenderEmails, []string{legacy.SenderEmail}) ||
		got.TransactionSource != legacy.TransactionSource || got.SourceLabel != legacy.TransactionSource {
		t.Fatalf("legacy import fallback = %#v / %v", got, err)
	}

	updated := store.RuleRow{
		Name: legacy.Name, SenderEmails: []string{"one@example.test", "two@example.test"}, SubjectContains: "updated subject",
		AmountRegex: "updated amount", MerchantRegex: "updated merchant", CurrencyRegex: "updated currency",
		TransactionSource: "ignored legacy source", SourceType: "credit-card", SourceLabel: "Updated Card", Bank: "Updated Bank",
	}
	if err := backend.ImportUserRules(ctx, tenant, []store.RuleRow{updated}); err != nil {
		t.Fatalf("ImportUserRules update: %v", err)
	}
	rules, err = backend.ListRules(ctx, tenant)
	got = ruleNamed(rules, legacy.Name)
	if err != nil || got == nil || got.SenderEmail != updated.SenderEmails[0] || !reflect.DeepEqual(got.SenderEmails, updated.SenderEmails) ||
		got.SubjectContains != updated.SubjectContains || got.SourceType != updated.SourceType || got.SourceLabel != updated.SourceLabel ||
		got.TransactionSource != updated.SourceLabel || got.Bank != updated.Bank {
		t.Fatalf("v2 import round trip = %#v / %v", got, err)
	}
}

func testCommunity(ctx context.Context, t *testing.T, backend store.CommunityStore) {
	t.Helper()

	category := "Community Category " + suffix(t)
	bucket := "Community Bucket " + suffix(t)
	merchant := "Community Merchant " + suffix(t)

	if err := backend.SeedMCCCategories(ctx, []string{category}); err != nil {
		t.Fatalf("SeedMCCCategories: %v", err)
	}
	if err := backend.SeedMCCCodes(ctx, []store.MCCEntry{{
		Code:        "1234",
		Description: "community conformance mcc",
		Category:    category,
		Bucket:      bucket,
	}}); err != nil {
		t.Fatalf("SeedMCCCodes: %v", err)
	}
	updated, err := backend.SeedMerchantCategories(ctx, []store.MerchantCategoryEntry{{
		Fragment: merchant,
		Category: &category,
		Bucket:   &bucket,
	}})
	if err != nil {
		t.Fatalf("SeedMerchantCategories: %v", err)
	}
	if updated != 1 {
		t.Fatalf("SeedMerchantCategories updated = %d, want 1", updated)
	}

	snapshot, err := backend.LoadCategorySnapshot(ctx)
	if err != nil {
		t.Fatalf("LoadCategorySnapshot: %v", err)
	}
	gotCategory, gotBucket := snapshot("paid at " + merchant)
	if gotCategory != category || gotBucket != bucket {
		t.Fatalf("LoadCategorySnapshot resolver returned category=%q bucket=%q, want %q %q", gotCategory, gotBucket, category, bucket)
	}
}

func testCommunityMCCOverwrite(ctx context.Context, t *testing.T, backend store.CommunityStore) {
	t.Helper()

	code := "4321"
	fragment := "MCC Merchant"
	first := store.MCCEntry{Code: code, Description: "first", Category: "First Category", Bucket: "First Bucket"}
	if err := backend.SeedMCCCodes(ctx, []store.MCCEntry{first}); err != nil {
		t.Fatalf("SeedMCCCodes first: %v", err)
	}
	if updated, err := backend.SeedMerchantCategories(ctx, []store.MerchantCategoryEntry{{Fragment: fragment, MCC: &code}}); err != nil || updated != 1 {
		t.Fatalf("SeedMerchantCategories MCC updated=%d err=%v", updated, err)
	}
	assertCommunityResolution(ctx, t, backend, fragment, first.Category, first.Bucket)

	second := store.MCCEntry{Code: code, Description: "second", Category: "Second Category", Bucket: "Second Bucket"}
	if err := backend.SeedMCCCodes(ctx, []store.MCCEntry{second}); err != nil {
		t.Fatalf("SeedMCCCodes overwrite: %v", err)
	}
	assertCommunityResolution(ctx, t, backend, fragment, second.Category, second.Bucket)
}

func testCommunityResolution(ctx context.Context, t *testing.T, backend store.CommunityStore) {
	t.Helper()

	shortCategory, longCategory := "Short Category", "Long Category"
	shortBucket, longBucket := "Short Bucket", "Long Bucket"
	entries := []store.MerchantCategoryEntry{
		{Fragment: "shop", Category: &shortCategory, Bucket: &shortBucket},
		{Fragment: "example shop", Category: &longCategory, Bucket: &longBucket},
		{Fragment: "category only", Category: &shortCategory},
		{Fragment: "bucket only", Bucket: &shortBucket},
	}
	updated, err := backend.SeedMerchantCategories(ctx, entries)
	if err != nil || updated != int64(len(entries)) {
		t.Fatalf("SeedMerchantCategories updated=%d err=%v", updated, err)
	}
	updated, err = backend.SeedMerchantCategories(ctx, entries)
	if err != nil || updated != int64(len(entries)) {
		t.Fatalf("SeedMerchantCategories repeat updated=%d err=%v", updated, err)
	}
	assertCommunityResolution(ctx, t, backend, "PAID AT EXAMPLE SHOP", longCategory, longBucket)
	assertCommunityResolution(ctx, t, backend, "CATEGORY ONLY", shortCategory, "")
	assertCommunityResolution(ctx, t, backend, "BUCKET ONLY", "", shortBucket)
}

func testCommunitySeed(ctx context.Context, t *testing.T, backend CommunitySeedSubject) {
	t.Helper()

	category := "Seed Category"
	bucket := "Seed Bucket"
	merchant := "Seed Merchant"

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
}

func testCommunitySeedIdempotence(ctx context.Context, t *testing.T, backend CommunitySeedSubject) {
	t.Helper()

	category, bucket, merchant := "Idempotent Category", "Idempotent Bucket", "Idempotent Merchant"
	content := store.SeedContent{Rules: []api.Rule{{
		Name: "Idempotent Seed Rule", SenderEmails: []string{"seed@example.test"}, SubjectContains: "original",
		Amount: regexp.MustCompile("original amount"), MerchantInfo: regexp.MustCompile("original merchant"),
		Source: api.Source{Type: "card", Label: "Seed Card", Bank: "Seed Bank"},
	}}, MCCEntries: []store.MCCEntry{{
		Code: "9876", Description: "idempotent MCC", Category: category, Bucket: bucket,
	}}, MerchantCategories: []store.MerchantCategoryEntry{{
		Fragment: merchant, Category: &category, Bucket: &bucket,
	}}}
	if _, err := backend.Seed(ctx, content); err != nil {
		t.Fatalf("Seed first: %v", err)
	}
	changed := content
	changed.Rules = append([]api.Rule(nil), content.Rules...)
	changed.Rules[0].SubjectContains = "changed"
	if _, err := backend.Seed(ctx, changed); err != nil {
		t.Fatalf("Seed second: %v", err)
	}
	rules, err := backend.ListRules(ctx, store.Tenant{ID: "00000000-0000-0000-0000-000000000000"})
	got := ruleNamed(rules, content.Rules[0].Name)
	if err != nil || countRulesNamed(rules, content.Rules[0].Name) != 1 || got == nil || got.SubjectContains != "original" {
		t.Fatalf("Seed idempotence rules = %#v / %v", rules, err)
	}
	resolver, err := backend.Seed(ctx, content)
	if err != nil {
		t.Fatalf("Seed third: %v", err)
	}
	gotCategory, gotBucket := resolver(merchant)
	if gotCategory != category || gotBucket != bucket {
		t.Fatalf("Seed idempotence resolver = (%q, %q), want (%q, %q)", gotCategory, gotBucket, category, bucket)
	}
}

func testCommunityTransactions(ctx context.Context, t *testing.T, backend CommunityTransactionSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "community-transactions")
	category := "Seed Category"
	bucket := "Seed Bucket"
	merchant := "Seed Merchant"
	if _, err := backend.SeedMerchantCategories(ctx, []store.MerchantCategoryEntry{{
		Fragment: merchant,
		Category: &category,
		Bucket:   &bucket,
	}}); err != nil {
		t.Fatalf("SeedMerchantCategories: %v", err)
	}

	if err := backend.Write(ctx, store.IngestionBatch{
		Tenant: tenant,
		Transactions: []*api.TransactionDetails{{
			MessageID:    "community-" + suffix(t),
			Amount:       88,
			Currency:     baseCurrencyINR,
			Timestamp:    time.Now().UTC().Format(time.RFC3339),
			MerchantInfo: merchant,
			Source:       api.Source{Type: "credit-card", Label: "Community Card", Bank: "Example"},
		}},
	}); err != nil {
		t.Fatalf("Write community transaction: %v", err)
	}
	manualCategory := "Manual Category"
	manualBucket := "Manual Bucket"
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
	gotCategory, gotBucket := snapshot(merchant)
	if gotCategory != category || gotBucket != bucket {
		t.Fatalf("LoadCategorySnapshot resolver returned category=%q bucket=%q, want %q %q", gotCategory, gotBucket, category, bucket)
	}
}

//nolint:gocognit // One scenario checks exact matching, tenant isolation, and global precedence.
func testCommunityExactMerchantAndIsolation(ctx context.Context, t *testing.T, backend CommunityTransactionSubject) {
	t.Helper()

	tenantA := createTenant(ctx, t, backend, "community-exact-a")
	tenantB := createTenant(ctx, t, backend, "community-exact-b")
	merchant := "Exact Merchant"
	for _, tenant := range []store.Tenant{tenantA, tenantB} {
		if err := backend.Write(ctx, store.IngestionBatch{Tenant: tenant, Transactions: []*api.TransactionDetails{
			{MessageID: "exact", Amount: 1, Currency: baseCurrencyINR, Timestamp: "2026-01-01T00:00:00Z", MerchantInfo: merchant},
			{
				MessageID: "partial", Amount: 2, Currency: baseCurrencyINR,
				Timestamp: "2026-01-02T00:00:00Z", MerchantInfo: merchant + " Branch",
			},
		}}); err != nil {
			t.Fatalf("Write community exact transactions: %v", err)
		}
	}
	if affected, err := backend.CategorizeMerchant(ctx, tenantA, merchant, "Manual Category", "Manual Bucket"); err != nil || affected != 1 {
		t.Fatalf("CategorizeMerchant affected=%d err=%v, want exact one", affected, err)
	}
	exact := transactionByMessage(ctx, t, backend, tenantA, "exact")
	if exact.Category != "Manual Category" || exact.Bucket != "Manual Bucket" {
		t.Fatalf("exact merchant not categorized: %#v", exact)
	}
	partial := transactionByMessage(ctx, t, backend, tenantA, "partial")
	if partial.Category != "" || partial.Bucket != "" {
		t.Fatalf("partial merchant was categorized: %#v", partial)
	}
	other := transactionByMessage(ctx, t, backend, tenantB, "exact")
	if other.Category != "" || other.Bucket != "" {
		t.Fatalf("merchant categorization leaked tenants: %#v", other)
	}
	tenantOnlyMerchant := "Tenant Only Snapshot Merchant"
	if err := backend.Write(ctx, store.IngestionBatch{Tenant: tenantA, Transactions: []*api.TransactionDetails{{
		MessageID: "tenant-only-snapshot", Amount: 3, Currency: baseCurrencyINR,
		Timestamp: "2026-01-03T00:00:00Z", MerchantInfo: tenantOnlyMerchant,
	}}}); err != nil {
		t.Fatalf("Write tenant-only snapshot transaction: %v", err)
	}
	if affected, err := backend.CategorizeMerchant(ctx, tenantA, tenantOnlyMerchant, "Tenant Category", "Tenant Bucket"); err != nil || affected != 1 {
		t.Fatalf("CategorizeMerchant tenant-only affected=%d err=%v", affected, err)
	}
	snapshot, err := backend.LoadCategorySnapshot(ctx)
	if err != nil {
		t.Fatalf("LoadCategorySnapshot before global seed: %v", err)
	}
	if category, bucket := snapshot(tenantOnlyMerchant); category != "" || bucket != "" {
		t.Fatalf("global snapshot resolved tenant-only mapping = (%q, %q)", category, bucket)
	}

	globalCategory, globalBucket := "Global Category", "Global Bucket"
	if updated, err := backend.SeedMerchantCategories(ctx, []store.MerchantCategoryEntry{{
		Fragment: merchant, Category: &globalCategory, Bucket: &globalBucket,
	}}); err != nil || updated != 1 {
		t.Fatalf("SeedMerchantCategories same fragment updated=%d err=%v", updated, err)
	}
	mappings, err := backend.GetCategoryMappings(ctx, tenantA)
	if err != nil || !mappingContains(mappings, "Manual Category", merchant) {
		t.Fatalf("global seed changed tenant-locked category mapping: %#v / %v", mappings, err)
	}
	assertCommunityResolution(ctx, t, backend, merchant, globalCategory, globalBucket)
}

//nolint:gocognit // Conformance subtests intentionally exercise a broad backend contract in one scenario.
func testTransactions(ctx context.Context, t *testing.T, backend TransactionSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "transactions")
	otherTenant := createTenant(ctx, t, backend, "transactions-other")
	messageID := "message-" + suffix(t)
	timestamp := time.Date(2026, time.January, 2, 10, 30, 0, 0, time.UTC)

	if err := backend.Write(ctx, store.IngestionBatch{
		Tenant: tenant,
		Transactions: []*api.TransactionDetails{{
			MessageID:    messageID,
			Amount:       123.45,
			Currency:     baseCurrencyINR,
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
	_, err = backend.GetTransaction(ctx, otherTenant, txn.ID)
	assertErrorKind(t, err, errors.NotFound, "GetTransaction other tenant")
	crossTenantDescription := "cross-tenant description"
	assertErrorKind(
		t,
		backend.UpdateTransaction(ctx, otherTenant, txn.ID, store.TransactionUpdate{Description: &crossTenantDescription}),
		errors.NotFound,
		"UpdateTransaction other tenant",
	)
	assertErrorKind(t, backend.UpdateDescription(ctx, otherTenant, txn.ID, crossTenantDescription), errors.NotFound, "UpdateDescription other tenant")
	assertErrorKind(t, backend.AddLabel(ctx, otherTenant, txn.ID, "cross-tenant"), errors.NotFound, "AddLabel other tenant")
	assertErrorKind(t, backend.AddLabels(ctx, otherTenant, txn.ID, []string{"cross-tenant-a", "cross-tenant-b"}), errors.NotFound, "AddLabels other tenant")
	assertErrorKind(t, backend.RemoveLabel(ctx, otherTenant, txn.ID, "conf-label"), errors.NotFound, "RemoveLabel other tenant")
	assertErrorKind(t, backend.MuteTransaction(ctx, otherTenant, txn.ID, true, "cross-tenant mute"), errors.NotFound, "MuteTransaction other tenant")
	assertErrorKind(t, backend.UpdateMuteReason(ctx, otherTenant, txn.ID, "cross-tenant reason"), errors.NotFound, "UpdateMuteReason other tenant")
	got, err = backend.GetTransaction(ctx, tenant, txn.ID)
	if err != nil {
		t.Fatalf("GetTransaction after cross-tenant mutations: %v", err)
	}
	if got.Description != "conformance transaction" || got.Muted || !reflect.DeepEqual(got.Labels, []string{"conf-label"}) {
		t.Fatalf("cross-tenant mutations changed transaction: %#v", got)
	}

	description := "updated description"
	category := "Updated Food"
	bucket := "Updated Needs"
	if err := backend.UpdateTransaction(ctx, tenant, txn.ID, store.TransactionUpdate{
		Description: &description,
		Category:    &category,
		Bucket:      &bucket,
	}); err != nil {
		t.Fatalf("UpdateTransaction: %v", err)
	}
	got, err = backend.GetTransaction(ctx, tenant, txn.ID)
	if err != nil || got.Description != description || got.Category != category || got.Bucket != bucket {
		t.Fatalf("transaction after UpdateTransaction = %#v / %v", got, err)
	}
	if err := backend.UpdateDescription(ctx, tenant, txn.ID, "description from UpdateDescription"); err != nil {
		t.Fatalf("UpdateDescription: %v", err)
	}
	got, err = backend.GetTransaction(ctx, tenant, txn.ID)
	if err != nil || got.Description != "description from UpdateDescription" {
		t.Fatalf("transaction after UpdateDescription = %#v / %v", got, err)
	}
	if err := backend.AddLabel(ctx, tenant, txn.ID, "manual"); err != nil {
		t.Fatalf("AddLabel: %v", err)
	}
	got, err = backend.GetTransaction(ctx, tenant, txn.ID)
	if err != nil || !containsString(got.Labels, "manual") {
		t.Fatalf("transaction after AddLabel = %#v / %v", got, err)
	}
	if err := backend.AddLabels(ctx, tenant, txn.ID, []string{"batch-a", "batch-b"}); err != nil {
		t.Fatalf("AddLabels: %v", err)
	}
	got, err = backend.GetTransaction(ctx, tenant, txn.ID)
	if err != nil || !containsString(got.Labels, "manual") || !containsString(got.Labels, "batch-a") || !containsString(got.Labels, "batch-b") {
		t.Fatalf("transaction after AddLabels = %#v / %v", got, err)
	}
	if err := backend.RemoveLabel(ctx, tenant, txn.ID, "conf-label"); err != nil {
		t.Fatalf("RemoveLabel: %v", err)
	}
	got, err = backend.GetTransaction(ctx, tenant, txn.ID)
	if err != nil {
		t.Fatalf("GetTransaction after updates: %v", err)
	}
	if got.Description != "description from UpdateDescription" || !containsString(got.Labels, "manual") ||
		!containsString(got.Labels, "batch-a") || !containsString(got.Labels, "batch-b") || containsString(got.Labels, "conf-label") {
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
	if !containsString(facets.Currencies, baseCurrencyINR) || !containsString(facets.Merchants, "Conformance Merchant") {
		t.Fatalf("GetFacets missing transaction values: %#v", facets)
	}

	if err := backend.MuteTransaction(ctx, tenant, txn.ID, true, "manual mute"); err != nil {
		t.Fatalf("MuteTransaction true: %v", err)
	}
	got, err = backend.GetTransaction(ctx, tenant, txn.ID)
	if err != nil || !got.Muted || got.MuteReason != "manual mute" {
		t.Fatalf("transaction after MuteTransaction true = %#v / %v", got, err)
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
	got, err = backend.GetTransaction(ctx, tenant, txn.ID)
	if err != nil || got.Muted || got.MuteReason != "" {
		t.Fatalf("transaction after MuteTransaction false = %#v / %v", got, err)
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

func testTransactionListFilters(ctx context.Context, t *testing.T, backend TransactionSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "list-filters")
	times := []time.Time{
		time.Date(2026, time.March, 8, 6, 30, 0, 0, time.UTC),
		time.Date(2026, time.March, 9, 15, 0, 0, 0, time.UTC),
		time.Date(2026, time.March, 10, 20, 0, 0, 0, time.UTC),
	}
	writeTransactions(ctx, t, backend, tenant,
		transactionDetails("filter-target", 10, times[0], "Cafe Target", "Food", "Needs", api.Source{Type: "credit-card", Label: "HDFC Card", Bank: "HDFC"}, "alpha"),
		transactionDetails("filter-other", 20, times[1], "Rail Other", "Travel", "Wants", api.Source{Type: "upi", Label: "Phone UPI", Bank: "SBI"}, "beta"),
		transactionDetails(
			"filter-third", 30, times[2], "Book Third", "Books", "Learning",
			api.Source{Type: "debit-card", Label: "Axis Debit", Bank: "Axis"}, "gamma",
		),
	)
	refresh := transactionDetails(
		"filter-target", 10, times[0], "Cafe Target", "Food", "Needs",
		api.Source{Type: "credit-card", Label: "HDFC Card", Bank: "HDFC"}, "alpha",
	)
	refresh.Currency = baseCurrencyUSD
	writeTransactions(ctx, t, backend, tenant, refresh)

	from, to := times[0], times[0]
	weekday, hour := 0, 1
	cases := []struct {
		name   string
		filter store.ListFilter
		want   string
	}{
		{"category", store.ListFilter{Category: "foo"}, "filter-target"},
		{"bucket", store.ListFilter{Bucket: "need"}, "filter-target"},
		{"label", store.ListFilter{Label: "alp"}, "filter-target"},
		{"currency", store.ListFilter{Currency: "usd"}, "filter-target"},
		{"source", store.ListFilter{Source: "hdfc"}, "filter-target"},
		{"source type", store.ListFilter{SourceType: "credit"}, "filter-target"},
		{"bank", store.ListFilter{Bank: "hdf"}, "filter-target"},
		{"merchant", store.ListFilter{Merchant: "target"}, "filter-target"},
		{"from and to inclusive", store.ListFilter{From: &from, To: &to}, "filter-target"},
		{"weekday and hour timezone", store.ListFilter{Weekday: &weekday, HourFrom: &hour, HourTo: &hour, Timezone: "America/New_York"}, "filter-target"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, result := listTransactions(ctx, t, backend, tenant, tc.filter)
			assertSingleTransactionMessage(t, rows, result, tc.want)
		})
	}
}

func testTransactionMissingExclusionsAndMutePrecedence(ctx context.Context, t *testing.T, backend TransactionSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "list-exclusions")
	at := time.Date(2026, time.April, 1, 12, 0, 0, 0, time.UTC)
	keep := transactionDetails(
		"exclude-keep", 10, at, "Keep Merchant", "Keep Category", "Keep Bucket",
		api.Source{Type: "keep-type", Label: "Keep Source", Bank: "Keep Bank"}, "keep-label",
	)
	drop := transactionDetails(
		"exclude-drop", 20, at.Add(time.Hour), "Drop Merchant", "Drop Category", "Drop Bucket",
		api.Source{Type: "drop-type", Label: "Drop Source", Bank: "Drop Bank"}, "drop-label",
	)
	mixed := transactionDetails(
		"exclude-mixed-label", 30, at.Add(2*time.Hour), "Mixed Merchant", "Mixed Category", "Mixed Bucket",
		api.Source{Type: "mixed-type", Label: "Mixed Source", Bank: "Mixed Bank"}, "drop-label", "keep-label",
	)
	missing := transactionDetails("exclude-missing", 40, at.Add(3*time.Hour), "Missing Merchant", "", "", api.Source{}, "")
	missing.Labels = nil
	writeTransactions(ctx, t, backend, tenant, keep, drop, mixed, missing)

	missingRows, missingResult := listTransactions(ctx, t, backend, tenant, store.ListFilter{
		CategoryMissing: true, BucketMissing: true, LabelMissing: true,
	})
	assertSingleTransactionMessage(t, missingRows, missingResult, "exclude-missing")

	exclusions := []struct {
		name   string
		filter store.ListFilter
		want   []string
	}{
		{"categories", store.ListFilter{ExcludeCategories: []string{"Drop Category", "Mixed Category"}}, []string{"exclude-keep"}},
		{"buckets", store.ListFilter{ExcludeBuckets: []string{"Drop Bucket", "Mixed Bucket"}}, []string{"exclude-keep"}},
		{"sources", store.ListFilter{ExcludeSources: []string{drop.Source.Display(), mixed.Source.Display()}}, []string{"exclude-keep"}},
		{"source types", store.ListFilter{ExcludeSourceTypes: []string{"drop-type", "mixed-type"}}, []string{"exclude-keep"}},
		{"banks", store.ListFilter{ExcludeBanks: []string{"Drop Bank", "Mixed Bank"}}, []string{"exclude-keep"}},
		{"labels", store.ListFilter{ExcludeLabels: []string{"drop-label"}}, []string{"exclude-keep", "exclude-mixed-label"}},
	}
	for _, tc := range exclusions {
		t.Run("exclude "+tc.name, func(t *testing.T) {
			rows, _ := listTransactions(ctx, t, backend, tenant, tc.filter)
			assertTransactionMessages(t, rows, tc.want...)
		})
	}

	visible := transactionByMessage(ctx, t, backend, tenant, "exclude-keep")
	individual := transactionByMessage(ctx, t, backend, tenant, "exclude-drop")
	if err := backend.MuteTransaction(ctx, tenant, individual.ID, true, "individual"); err != nil {
		t.Fatalf("MuteTransaction: %v", err)
	}
	individual = transactionByMessage(ctx, t, backend, tenant, "exclude-drop")
	if err := backend.MuteByMerchant(ctx, tenant, "Mixed Merchant", "merchant"); err != nil {
		t.Fatalf("MuteByMerchant: %v", err)
	}
	merchant := transactionByMessage(ctx, t, backend, tenant, "exclude-mixed-label")
	if visible.Muted || !individual.Muted || !merchant.Muted || !merchant.MutedByMerchant {
		t.Fatalf("mute setup visible=%#v individual=%#v merchant=%#v", visible, individual, merchant)
	}
	rows, _ := listTransactions(ctx, t, backend, tenant, store.ListFilter{})
	assertTransactionMessages(t, rows, "exclude-keep", "exclude-missing")
	rows, _ = listTransactions(ctx, t, backend, tenant, store.ListFilter{ShowMuted: true})
	assertTransactionMessages(t, rows, "exclude-keep", "exclude-drop", "exclude-mixed-label", "exclude-missing")
	rows, _ = listTransactions(ctx, t, backend, tenant, store.ListFilter{MutedOnly: true, ShowMuted: true})
	assertTransactionMessages(t, rows, "exclude-drop", "exclude-mixed-label")
	rows, _ = listTransactions(ctx, t, backend, tenant, store.ListFilter{IndividualOnly: true, MutedOnly: true, ShowMuted: true})
	assertTransactionMessages(t, rows, "exclude-drop")
}

func testTransactionPaginationSortAndTotals(ctx context.Context, t *testing.T, backend TransactionSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "list-pagination")
	base := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	for i := range 21 {
		writeTransactions(ctx, t, backend, tenant, transactionDetails(
			fmt.Sprintf("page-%d", i+1), float64(i+1), base.Add(time.Duration(i)*time.Hour), fmt.Sprintf("Page %d", i+1), "Paging", "Needs", api.Source{},
		))
	}
	rows, result := listTransactions(ctx, t, backend, tenant, store.ListFilter{})
	if len(rows) != 20 || result.Total != 21 || result.TotalAmount != 231 || rows[0].MessageID != "page-21" {
		t.Fatalf("default pagination/sort rows=%#v result=%#v", rows, result)
	}
	rows, result = listTransactions(ctx, t, backend, tenant, store.ListFilter{Page: 2, PageSize: 2, SortBy: "timestamp", SortDir: "asc"})
	if len(rows) != 2 || rows[0].MessageID != "page-3" || rows[1].MessageID != "page-4" || result.Total != 21 || result.TotalAmount != 231 {
		t.Fatalf("ascending page rows=%#v result=%#v", rows, result)
	}
	rows, result = listTransactions(ctx, t, backend, tenant, store.ListFilter{Page: 2, PageSize: 3, SortBy: "timestamp", SortDir: "desc"})
	if len(rows) != 3 || rows[0].MessageID != "page-18" || rows[2].MessageID != "page-16" || result.Total != 21 || result.TotalAmount != 231 {
		t.Fatalf("descending page rows=%#v result=%#v", rows, result)
	}
	rows, result = listTransactions(ctx, t, backend, tenant, store.ListFilter{Page: 99, PageSize: 2})
	if rows == nil || len(rows) != 0 || result.Total != 21 || result.TotalAmount != 231 {
		t.Fatalf("overflow page rows=%#v result=%#v", rows, result)
	}
	_, _, err := backend.ListTransactions(ctx, tenant, store.ListFilter{Page: math.MaxInt, PageSize: 100})
	assertErrorKind(t, err, errors.InvalidInput, "ListTransactions offset overflow")
}

func testTransactionFacetsAndEmptyOutputs(ctx context.Context, t *testing.T, backend TransactionSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "facets-empty")
	rows, result := listTransactions(ctx, t, backend, tenant, store.ListFilter{})
	if rows == nil || len(rows) != 0 || result != (store.TransactionListResult{}) {
		t.Fatalf("empty ListTransactions rows=%#v result=%#v", rows, result)
	}
	assertEmptyFacets(ctx, t, backend, tenant)

	at := time.Date(2026, time.June, 1, 12, 0, 0, 0, time.UTC)
	writeTransactions(ctx, t, backend, tenant,
		transactionDetails("facet-one", 10, at, "Facet One", "Food", "Needs", api.Source{Type: "card", Label: "Card A", Bank: "Bank A"}, "shared"),
		transactionDetails(
			"facet-two", 20, at.Add(time.Hour), "Facet Two", "Food", "Wants",
			api.Source{Type: "upi", Label: "UPI B", Bank: "Bank B"}, "shared", "other",
		),
	)
	facets, err := backend.GetFacets(ctx, tenant)
	if err != nil {
		t.Fatalf("GetFacets: %v", err)
	}
	if !reflect.DeepEqual(facets.Categories, []string{"Food"}) || facets.CategoryCounts["Food"] != 2 ||
		facets.BucketCounts["Needs"] != 1 || facets.BucketCounts["Wants"] != 1 || facets.LabelCounts["shared"] != 2 ||
		!containsString(facets.Sources, (api.Source{Type: "card", Label: "Card A", Bank: "Bank A"}).Display()) || !containsString(facets.SourceTypes, "card") ||
		!containsString(facets.Banks, "Bank B") || !containsString(facets.Merchants, "Facet Two") {
		t.Fatalf("facets = %#v", facets)
	}
}

func testTransactionMerchantMutatorIsolation(ctx context.Context, t *testing.T, backend TransactionSubject) {
	t.Helper()

	tenantA := createTenant(ctx, t, backend, "merchant-mutator-a")
	tenantB := createTenant(ctx, t, backend, "merchant-mutator-b")
	at := time.Date(2026, time.July, 1, 12, 0, 0, 0, time.UTC)
	for _, tenant := range []store.Tenant{tenantA, tenantB} {
		writeTransactions(ctx, t, backend, tenant, transactionDetails("shared-message", 10, at, "Shared Merchant", "", "", api.Source{}))
	}
	if err := backend.MuteByMerchant(ctx, tenantA, "Shared", "tenant A"); err != nil {
		t.Fatalf("MuteByMerchant tenant A: %v", err)
	}
	mutedA, err := backend.ListMutedMerchants(ctx, tenantA)
	if err != nil || len(mutedA) != 1 {
		t.Fatalf("ListMutedMerchants tenant A = %#v / %v", mutedA, err)
	}
	if got := transactionByMessage(ctx, t, backend, tenantB, "shared-message"); got.Muted {
		t.Fatalf("MuteByMerchant leaked to tenant B: %#v", got)
	}
	id := mutedA[0].ID
	assertErrorKind(t, backend.UpdateMerchantReason(ctx, tenantB, id, "wrong tenant"), errors.NotFound, "UpdateMerchantReason other tenant")
	assertErrorKind(t, backend.DeleteMutedMerchant(ctx, tenantB, id), errors.NotFound, "DeleteMutedMerchant other tenant")
	assertErrorKind(t, backend.DeleteMutedMerchantAndUnmute(ctx, tenantB, id), errors.NotFound, "DeleteMutedMerchantAndUnmute other tenant")
	if err := backend.UnmuteByPattern(ctx, tenantB, "Shared"); err != nil {
		t.Fatalf("UnmuteByPattern tenant B: %v", err)
	}
	if got := transactionByMessage(ctx, t, backend, tenantA, "shared-message"); !got.Muted {
		t.Fatalf("tenant B mutators changed tenant A transaction: %#v", got)
	}
	patterns, err := backend.GetMutedMerchantPatterns(ctx, tenantB)
	if err != nil || patterns == nil || len(patterns) != 0 {
		t.Fatalf("tenant B patterns = %#v / %v", patterns, err)
	}
	if err := backend.UpdateMerchantReason(ctx, tenantA, id, "updated"); err != nil {
		t.Fatalf("UpdateMerchantReason tenant A: %v", err)
	}
	if err := backend.DeleteMutedMerchantAndUnmute(ctx, tenantA, id); err != nil {
		t.Fatalf("DeleteMutedMerchantAndUnmute tenant A: %v", err)
	}
	if got := transactionByMessage(ctx, t, backend, tenantA, "shared-message"); got.Muted {
		t.Fatalf("DeleteMutedMerchantAndUnmute did not unmute tenant A: %#v", got)
	}
}

func testSearchMatchingAndLiteralSafety(ctx context.Context, t *testing.T, backend TransactionSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "search-matching")
	base := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	rows := []*api.TransactionDetails{
		transactionDetails("search-split", 10, base, "CAFÉ Amazon", "Shopping", "Wants", api.Source{}),
		transactionDetails("search-percent", 20, base.Add(time.Hour), "Save 100% Market", "Shopping", "Wants", api.Source{}),
		transactionDetails("search-percent-wild", 30, base.Add(2*time.Hour), "Save 100X Market", "Shopping", "Wants", api.Source{}),
		transactionDetails("search-underscore", 40, base.Add(3*time.Hour), "under_score", "Shopping", "Wants", api.Source{}),
		transactionDetails("search-underscore-wild", 50, base.Add(4*time.Hour), "underXscore", "Shopping", "Wants", api.Source{}),
		transactionDetails("search-backslash", 60, base.Add(5*time.Hour), `path\name`, "Shopping", "Wants", api.Source{}),
	}
	rows[0].Description = "annual prime membership"
	writeTransactions(ctx, t, backend, tenant, rows...)

	assertSearchMessages(ctx, t, backend, tenant, "café prime", store.ListFilter{}, "search-split")
	assertSearchMessages(ctx, t, backend, tenant, "nual pri", store.ListFilter{}, "search-split")
	assertSearchMessages(ctx, t, backend, tenant, "100%", store.ListFilter{}, "search-percent")
	assertSearchMessages(ctx, t, backend, tenant, "under_score", store.ListFilter{}, "search-underscore")
	assertSearchMessages(ctx, t, backend, tenant, `path\name`, store.ListFilter{}, "search-backslash")
	assertSearchMessages(ctx, t, backend, tenant, "amazon OR path", store.ListFilter{}, "search-backslash", "search-split")
}

func testSearchFiltersSortPaginationAndTotals(ctx context.Context, t *testing.T, backend TransactionSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "search-filtering")
	base := time.Date(2026, time.September, 1, 10, 0, 0, 0, time.UTC)
	for i := range 4 {
		txn := transactionDetails(fmt.Sprintf("search-page-%d", i+1), float64((i+1)*10), base.Add(time.Duration(i)*time.Hour),
			fmt.Sprintf("Coffee %d", i+1), "Food", "Needs", api.Source{Type: "card", Label: "Card", Bank: "Bank"}, "coffee")
		writeTransactions(ctx, t, backend, tenant, txn)
	}
	writeTransactions(ctx, t, backend, tenant,
		transactionDetails(
			"search-filter-distractor", 9, base.Add(-time.Hour), "Coffee Distractor", "Travel", "Wants",
			api.Source{Type: "cash", Label: "Cash", Bank: "Other"}, "coffee",
		),
		transactionDetails(
			"search-filter-muted", 99, base.Add(150*time.Minute), "Coffee Muted", "Food", "Needs",
			api.Source{Type: "card", Label: "Card", Bank: "Bank"}, "coffee",
		),
	)
	muted := transactionByMessage(ctx, t, backend, tenant, "search-filter-muted")
	if err := backend.MuteTransaction(ctx, tenant, muted.ID, true, "search filter"); err != nil {
		t.Fatalf("MuteTransaction search filter: %v", err)
	}
	from, to := base, base.Add(3*time.Hour)
	filter := store.ListFilter{
		Category: "Food", Bucket: "Needs", Label: "coffee", Currency: baseCurrencyINR,
		Source: "Card", SourceType: "card", Bank: "Bank",
		Merchant: "Coffee", From: &from, To: &to, ExcludeCategories: []string{"Travel"}, Page: 2, PageSize: 2, SortDir: "asc",
	}
	rows, result, err := backend.SearchTransactions(ctx, tenant, "coffee", filter)
	if err != nil {
		t.Fatalf("SearchTransactions filtered: %v", err)
	}
	if len(rows) != 2 || rows[0].MessageID != "search-page-3" || rows[1].MessageID != "search-page-4" || result.Total != 4 || result.TotalAmount != 100 {
		t.Fatalf("filtered search rows=%#v result=%#v", rows, result)
	}
	rows, result, err = backend.SearchTransactions(ctx, tenant, "coffee", store.ListFilter{Page: 1, PageSize: 1, SortDir: "desc"})
	if err != nil || len(rows) != 1 || rows[0].MessageID != "search-page-4" || result.Total != 5 || result.TotalAmount != 109 {
		t.Fatalf("descending search rows=%#v result=%#v err=%v", rows, result, err)
	}
	_, _, err = backend.SearchTransactions(ctx, tenant, "coffee", store.ListFilter{Page: math.MaxInt, PageSize: 100})
	assertErrorKind(t, err, errors.InvalidInput, "SearchTransactions offset overflow")
	emptyRows, emptyResult, err := backend.SearchTransactions(ctx, tenant, "", store.ListFilter{})
	if err != nil || emptyRows == nil || len(emptyRows) != 5 || emptyResult.Total != 5 || emptyResult.TotalAmount != 109 {
		t.Fatalf("empty search rows=%#v result=%#v err=%v", emptyRows, emptyResult, err)
	}
}

func testSearchMutationDeleteAndIsolation(ctx context.Context, t *testing.T, backend TransactionSubject) {
	t.Helper()

	userA := createUser(ctx, t, backend, "search-sync-a")
	tenantA := store.Tenant{ID: userA.TenantID}
	tenantB := createTenant(ctx, t, backend, "search-sync-b")
	at := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	for _, tenant := range []store.Tenant{tenantA, tenantB} {
		txn := transactionDetails("search-sync", 10, at, "Static Merchant", "", "", api.Source{})
		txn.Description = "old searchable text"
		writeTransactions(ctx, t, backend, tenant, txn)
	}
	txnA := transactionByMessage(ctx, t, backend, tenantA, "search-sync")
	newDescription := "new searchable text"
	if err := backend.UpdateTransaction(ctx, tenantA, txnA.ID, store.TransactionUpdate{Description: &newDescription}); err != nil {
		t.Fatalf("UpdateTransaction search sync: %v", err)
	}
	assertSearchMessages(ctx, t, backend, tenantA, "new searchable", store.ListFilter{}, "search-sync")
	assertSearchMessages(ctx, t, backend, tenantA, "old searchable", store.ListFilter{})
	assertSearchMessages(ctx, t, backend, tenantB, "old searchable", store.ListFilter{}, "search-sync")
	if err := backend.UpdateDescription(ctx, tenantA, txnA.ID, "final searchable text"); err != nil {
		t.Fatalf("UpdateDescription search sync: %v", err)
	}
	assertSearchMessages(ctx, t, backend, tenantA, "final searchable", store.ListFilter{}, "search-sync")
	if err := backend.DeleteUser(ctx, userA.ID); err != nil {
		t.Fatalf("DeleteUser search sync: %v", err)
	}
	assertSearchMessages(ctx, t, backend, tenantA, "searchable", store.ListFilter{})
	assertSearchMessages(ctx, t, backend, tenantB, "searchable", store.ListFilter{}, "search-sync")
}

func testIngestionEmptyDefaultsAndDuplicates(ctx context.Context, t *testing.T, backend IngestionSubject) {
	t.Helper()

	tenantA := createTenant(ctx, t, backend, "ingestion-default-a")
	tenantB := createTenant(ctx, t, backend, "ingestion-default-b")
	if err := backend.Write(ctx, store.IngestionBatch{Tenant: tenantA}); err != nil {
		t.Fatalf("Write empty batch: %v", err)
	}
	at := time.Date(2026, time.November, 1, 12, 0, 0, 0, time.UTC)
	for _, tenant := range []store.Tenant{tenantA, tenantB} {
		txn := transactionDetails("shared-ingestion-message", 10, at, "Original Merchant", "", "", api.Source{})
		txn.Currency = ""
		writeTransactions(ctx, t, backend, tenant, txn)
	}
	gotA := transactionByMessage(ctx, t, backend, tenantA, "shared-ingestion-message")
	gotB := transactionByMessage(ctx, t, backend, tenantB, "shared-ingestion-message")
	if gotA.ID == gotB.ID || gotA.Currency != baseCurrencyINR || gotB.Currency != baseCurrencyINR {
		t.Fatalf("tenant duplicate/default currency A=%#v B=%#v", gotA, gotB)
	}
	refresh := transactionDetails(
		"shared-ingestion-message", 25, at.Add(time.Hour), "Refreshed Merchant", "", "",
		api.Source{Type: "card", Label: "Refreshed Card", Bank: "Refreshed Bank"},
	)
	writeTransactions(ctx, t, backend, tenantA, refresh)
	updated := transactionByMessage(ctx, t, backend, tenantA, "shared-ingestion-message")
	if updated.ID != gotA.ID || updated.Amount != 25 ||
		updated.MerchantInfo != "Refreshed Merchant" || updated.Source.Bank != "Refreshed Bank" ||
		!updated.Timestamp.Equal(at.Add(time.Hour)) {
		t.Fatalf("duplicate refresh = %#v", updated)
	}
	if other := transactionByMessage(ctx, t, backend, tenantB, "shared-ingestion-message"); other.Amount != 10 || other.MerchantInfo != "Original Merchant" {
		t.Fatalf("tenant A refresh changed tenant B: %#v", other)
	}
}

func testIngestionRefreshPreservesUserFields(ctx context.Context, t *testing.T, backend IngestionSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "ingestion-refresh")
	at := time.Date(2026, time.December, 1, 12, 0, 0, 0, time.UTC)
	initial := transactionDetails(
		"refresh-user-fields", 10, at, "Initial Merchant", "Extracted Category", "Extracted Bucket",
		api.Source{Type: "upi", Label: "Initial Source", Bank: "Initial Bank"},
	)
	writeTransactions(ctx, t, backend, tenant, initial)
	stored := transactionByMessage(ctx, t, backend, tenant, initial.MessageID)
	description, category, bucket := "User Description", "User Category", "User Bucket"
	if err := backend.UpdateTransaction(ctx, tenant, stored.ID, store.TransactionUpdate{
		Description: &description, Category: &category, Bucket: &bucket,
	}); err != nil {
		t.Fatalf("UpdateTransaction user fields: %v", err)
	}
	refresh := transactionDetails(
		initial.MessageID, 99, at.Add(24*time.Hour), "Refreshed Merchant", "New Extracted Category", "New Extracted Bucket",
		api.Source{Type: "card", Label: "New Source", Bank: "New Bank"},
	)
	refresh.Currency = baseCurrencyUSD
	refresh.Description = "Extracted Description"
	originalAmount, originalCurrency, exchangeRate := 120.0, "EUR", 0.825
	refresh.OriginalAmount = &originalAmount
	refresh.OriginalCurrency = &originalCurrency
	refresh.ExchangeRate = &exchangeRate
	writeTransactions(ctx, t, backend, tenant, refresh)
	got := transactionByMessage(ctx, t, backend, tenant, initial.MessageID)
	if got.Amount != 99 || got.Currency != baseCurrencyUSD ||
		!got.Timestamp.Equal(refreshTimestamp(t, refresh)) || got.MerchantInfo != refresh.MerchantInfo ||
		got.Source.Type != "card" || got.Source.Label != "New Source" || got.Source.Bank != "New Bank" ||
		got.OriginalAmount == nil || *got.OriginalAmount != originalAmount || got.OriginalCurrency == nil || *got.OriginalCurrency != originalCurrency ||
		got.ExchangeRate == nil || *got.ExchangeRate != exchangeRate ||
		got.Description != description || got.Category != category || got.Bucket != bucket {
		t.Fatalf("refreshed transaction = %#v", got)
	}
}

func testIngestionLabelsMappingsAndMuteRules(ctx context.Context, t *testing.T, backend IngestionSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "ingestion-rules")
	for _, label := range []string{"Shared", "Merchant"} {
		if err := backend.CreateLabel(ctx, tenant, label, "#123456"); err != nil {
			t.Fatalf("CreateLabel %q: %v", label, err)
		}
	}
	if _, err := backend.ApplyLabelByMerchant(ctx, tenant, "Shared", "Ride Share"); err != nil {
		t.Fatalf("ApplyLabelByMerchant Shared: %v", err)
	}
	if _, err := backend.ApplyLabelByMerchant(ctx, tenant, "Merchant", "Ride"); err != nil {
		t.Fatalf("ApplyLabelByMerchant Merchant: %v", err)
	}
	if _, err := backend.CategorizeMerchant(ctx, tenant, "Ride", "Transport", "Needs"); err != nil {
		t.Fatalf("CategorizeMerchant short: %v", err)
	}
	if _, err := backend.CategorizeMerchant(ctx, tenant, "Ride Share", "Car Pool", "Wants"); err != nil {
		t.Fatalf("CategorizeMerchant long: %v", err)
	}
	if err := backend.MuteByMerchant(ctx, tenant, "Ride Share", "automatic mute"); err != nil {
		t.Fatalf("MuteByMerchant: %v", err)
	}
	txn := transactionDetails("ingestion-rules", 10, time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC), "RIDE SHARE EXPRESS", "", "", api.Source{}, "Shared")
	writeTransactions(ctx, t, backend, tenant, txn)
	got := transactionByMessage(ctx, t, backend, tenant, txn.MessageID)
	if got.Category != "Car Pool" || got.Bucket != "Wants" || !got.Muted || !got.MutedByMerchant || got.MuteReason != "automatic mute" ||
		!containsString(got.Labels, "Shared") || !containsString(got.Labels, "Merchant") {
		t.Fatalf("ingestion mappings = %#v", got)
	}
	if removed, err := backend.RemoveLabelByMerchant(ctx, tenant, "Shared", "Ride Share"); err != nil || removed != 0 {
		t.Fatalf("RemoveLabelByMerchant shared removed=%d err=%v", removed, err)
	}
	if got = transactionByMessage(ctx, t, backend, tenant, txn.MessageID); !containsString(got.Labels, "Shared") {
		t.Fatalf("merchant provenance removal removed payload label: %#v", got)
	}
	writeTransactions(ctx, t, backend, tenant, txn)
	got = transactionByMessage(ctx, t, backend, tenant, txn.MessageID)
	if countString(got.Labels, "Shared") != 1 || countString(got.Labels, "Merchant") != 1 {
		t.Fatalf("duplicate ingestion labels = %#v", got.Labels)
	}

	if err := backend.MuteByMerchant(ctx, tenant, "No Reason", ""); err != nil {
		t.Fatalf("MuteByMerchant without reason: %v", err)
	}
	withoutReason := transactionDetails(
		"ingestion-mute-without-reason", 11, time.Date(2026, time.January, 2, 12, 0, 0, 0, time.UTC),
		"NO REASON MERCHANT", "", "", api.Source{},
	)
	writeTransactions(ctx, t, backend, tenant, withoutReason)
	got = transactionByMessage(ctx, t, backend, tenant, withoutReason.MessageID)
	if !got.Muted || !got.MutedByMerchant || got.MuteReason != "" {
		t.Fatalf("ingestion mute without reason = %#v", got)
	}
}

func testIngestionRestartPersistence(ctx context.Context, t *testing.T, subject Subject[IngestionSubject]) {
	t.Helper()
	if subject.Restart == nil {
		t.Skip("backend conformance factory does not support restart")
	}
	tenant := createTenant(ctx, t, subject.Store, "ingestion-restart")
	txn := transactionDetails(
		"restart-persistence", 42, time.Date(2026, time.February, 1, 12, 0, 0, 0, time.UTC),
		"Persistent Merchant", "Food", "Needs", api.Source{},
	)
	writeTransactions(ctx, t, subject.Store, tenant, txn)
	reopened := subject.Restart(t)
	got := transactionByMessage(ctx, t, reopened, tenant, txn.MessageID)
	if got.Amount != 42 || got.MerchantInfo != "Persistent Merchant" {
		t.Fatalf("transaction after restart = %#v", got)
	}
}

func testIngestionAtomicRollback(ctx context.Context, t *testing.T, subject Subject[IngestionSubject]) {
	t.Helper()
	if subject.RejectIngestionMessage == nil {
		t.Fatal("backend conformance factory does not support ingestion failure injection")
	}

	tenant := createTenant(ctx, t, subject.Store, "ingestion-rollback")
	writeTransactions(ctx, t, subject.Store, tenant, transactionDetails(
		"rollback-existing", 5, time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), "Existing", "", "", api.Source{},
	))
	subject.RejectIngestionMessage(t, "rollback-reject")
	err := subject.Store.Write(ctx, store.IngestionBatch{Tenant: tenant, Transactions: []*api.TransactionDetails{
		transactionDetails("rollback-accepted", 10, time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC), "Accepted", "", "", api.Source{}),
		transactionDetails("rollback-reject", 20, time.Date(2026, time.January, 3, 0, 0, 0, 0, time.UTC), "Rejected", "", "", api.Source{}),
	}})
	if err == nil {
		t.Fatal("Write with rejected second item returned nil error")
	}
	rows, result := listTransactions(ctx, t, subject.Store, tenant, store.ListFilter{Page: 1, PageSize: 20})
	if result.Total != 1 || len(rows) != 1 || rows[0].MessageID != "rollback-existing" {
		t.Fatalf("failed ingestion had side effects: rows=%#v result=%#v", rows, result)
	}
}

// AnalyticsNow is the fixed clock value used by analytics conformance factories.
func AnalyticsNow() time.Time {
	return time.Date(2026, time.April, 15, 12, 0, 0, 0, time.UTC)
}

func testAnalyticsEmptyOutputs(ctx context.Context, t *testing.T, backend AnalyticsSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "analytics-empty")
	stats, err := backend.GetStats(ctx, tenant, baseCurrencyINR)
	if err != nil || stats == nil || stats.TotalCount != 0 || stats.TotalByCategory == nil || stats.TotalCategoryCount == nil {
		t.Fatalf("GetStats empty = %#v / %v", stats, err)
	}
	charts, err := backend.GetChartData(ctx, tenant)
	if err != nil {
		t.Fatalf("GetChartData empty: %v", err)
	}
	assertEmptyChartData(t, charts, "GetChartData")
	dashboard, err := backend.GetDashboardData(ctx, tenant)
	if err != nil {
		t.Fatalf("GetDashboardData empty: %v", err)
	}
	assertEmptyChartData(t, &dashboard.CurrentMonth.Charts, "current dashboard charts")
	assertEmptyChartData(t, &dashboard.AllTime.Charts, "all-time dashboard charts")
	if dashboard.CurrentMonth.Stats.TotalByCategory == nil || dashboard.AllTime.Stats.TotalByCategory == nil {
		t.Fatalf("dashboard empty stats = %#v", dashboard)
	}
	heatmap, err := backend.GetSpendingHeatmap(ctx, tenant, nil, nil)
	if err != nil || heatmap == nil || heatmap.ByWeekdayHour == nil || heatmap.ByDayOfMonth == nil ||
		len(heatmap.ByWeekdayHour) != 0 || len(heatmap.ByDayOfMonth) != 0 {
		t.Fatalf("GetSpendingHeatmap empty = %#v / %v", heatmap, err)
	}
	annual, err := backend.GetAnnualSpend(ctx, tenant, AnalyticsNow().Year())
	if err != nil || annual == nil || len(annual) != 0 {
		t.Fatalf("GetAnnualSpend empty = %#v / %v", annual, err)
	}
	breakdown, err := backend.GetMonthlyBreakdownSpend(ctx, tenant, "labels", 3)
	if err != nil || breakdown == nil || breakdown.Labels == nil || breakdown.Months == nil || breakdown.Series == nil || len(breakdown.Months) != 3 {
		t.Fatalf("GetMonthlyBreakdownSpend empty = %#v / %v", breakdown, err)
	}
}

func testAnalyticsStatsChartsAndDashboard(ctx context.Context, t *testing.T, backend AnalyticsSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "analytics-dashboard")
	if err := backend.SetAppConfig(ctx, tenant, "app.timezone", "Asia/Kolkata"); err != nil {
		t.Fatalf("SetAppConfig timezone: %v", err)
	}
	if err := backend.SetAppConfig(ctx, tenant, "base_currency", baseCurrencyUSD); err != nil {
		t.Fatalf("SetAppConfig base currency: %v", err)
	}
	prior := transactionDetails(
		"analytics-prior", 100, time.Date(2026, time.March, 10, 6, 0, 0, 0, time.UTC),
		"Prior Cafe", "Food", "Needs", api.Source{Type: "card", Label: "HDFC Card", Bank: "HDFC"}, "Dining",
	)
	prior.Currency = baseCurrencyUSD
	current := transactionDetails(
		"analytics-current", 200, time.Date(2026, time.April, 5, 6, 0, 0, 0, time.UTC),
		"Current Cafe", "Food", "Wants", api.Source{Type: "upi", Label: "SBI UPI", Bank: "SBI"}, "Dining",
	)
	current.Currency = baseCurrencyUSD
	uncategorized := transactionDetails("analytics-uncategorized", 300, time.Date(2026, time.April, 6, 6, 0, 0, 0, time.UTC), "Unknown", "", "", api.Source{})
	muted := transactionDetails(
		"analytics-muted", 400, time.Date(2026, time.April, 7, 6, 0, 0, 0, time.UTC),
		"Hidden", "Hidden", "Hidden", api.Source{Type: "hidden", Label: "Hidden", Bank: "Hidden"}, "Hidden",
	)
	muted.Currency = baseCurrencyUSD
	writeTransactions(ctx, t, backend, tenant, prior, current, uncategorized, muted)
	mutedRow := transactionByMessage(ctx, t, backend, tenant, muted.MessageID)
	if err := backend.MuteTransaction(ctx, tenant, mutedRow.ID, true, "hidden"); err != nil {
		t.Fatalf("MuteTransaction analytics: %v", err)
	}

	stats, err := backend.GetStats(ctx, tenant, baseCurrencyUSD)
	if err != nil || stats.TotalCount != 3 || stats.TotalBase != 300 || stats.BaseCurrency != baseCurrencyUSD ||
		stats.TotalByCategory["Food"] != 300 || stats.TotalCategoryCount["Food"] != 2 ||
		stats.TotalByCategory["Uncategorized"] != 300 || stats.TotalCategoryCount["Uncategorized"] != 1 {
		t.Fatalf("GetStats = %#v / %v", stats, err)
	}
	charts, err := backend.GetChartData(ctx, tenant)
	if err != nil {
		t.Fatalf("GetChartData: %v", err)
	}
	if timeBucketAmount(charts.MonthlySpend, "2026-03") != 100 || timeBucketAmount(charts.MonthlySpend, "2026-04") != 500 ||
		timeBucketAmount(charts.DailySpend, "2026-04-05") != 200 || timeBucketAmount(charts.DailySpend, "2026-04-06") != 300 ||
		charts.ByCategory["Food"] != 300 || charts.ByCategory["Uncategorized"] != 300 ||
		charts.ByBucket["Needs"] != 100 || charts.ByBucket["Wants"] != 200 || charts.ByBucket["Uncategorized"] != 300 ||
		charts.ByLabel["Dining"] != 300 || charts.ByLabel["Uncategorized"] != 300 ||
		charts.BySource[prior.Source.Display()] != 100 || charts.BySourceType["upi"] != 200 || charts.ByBank["SBI"] != 200 ||
		charts.ByCategoryMonthly["Food"] != (store.CategoryMonthlyEntry{Current: 200, Prior: 100}) {
		t.Fatalf("GetChartData = %#v", charts)
	}
	dashboard, err := backend.GetDashboardData(ctx, tenant)
	if err != nil {
		t.Fatalf("GetDashboardData: %v", err)
	}
	if dashboard.CurrentMonth.Label != "April 2026" || dashboard.CurrentMonth.Stats.TotalCount != 2 || dashboard.CurrentMonth.Stats.TotalBase != 200 ||
		dashboard.AllTime.Label != "All Time" || dashboard.AllTime.Stats.TotalCount != 3 || dashboard.AllTime.Stats.TotalBase != 300 ||
		dashboard.CurrentMonth.Charts.ByCategoryMonthly["Food"] != (store.CategoryMonthlyEntry{Current: 200, Prior: 100}) ||
		dashboard.AllTime.Charts.ByCategoryMonthly["Food"] != (store.CategoryMonthlyEntry{Current: 200, Prior: 100}) {
		t.Fatalf("GetDashboardData = %#v", dashboard)
	}
}

func testAnalyticsTimezoneAndDST(ctx context.Context, t *testing.T, backend AnalyticsSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "analytics-timezone")
	if err := backend.SetAppConfig(ctx, tenant, "app.timezone", "America/New_York"); err != nil {
		t.Fatalf("SetAppConfig: %v", err)
	}
	writeTransactions(ctx, t, backend, tenant,
		transactionDetails("dst-spring-before", 10, time.Date(2026, time.March, 8, 6, 30, 0, 0, time.UTC), "Spring Before", "Time", "Needs", api.Source{}),
		transactionDetails("dst-spring-after", 20, time.Date(2026, time.March, 8, 7, 30, 0, 0, time.UTC), "Spring After", "Time", "Needs", api.Source{}),
		transactionDetails("month-day-boundary", 30, time.Date(2026, time.April, 1, 3, 30, 0, 0, time.UTC), "Boundary", "Time", "Needs", api.Source{}),
		transactionDetails("dst-fall-first", 40, time.Date(2026, time.November, 1, 5, 30, 0, 0, time.UTC), "Fall First", "Time", "Needs", api.Source{}),
		transactionDetails("dst-fall-second", 50, time.Date(2026, time.November, 1, 6, 30, 0, 0, time.UTC), "Fall Second", "Time", "Needs", api.Source{}),
		transactionDetails("year-boundary", 60, time.Date(2026, time.January, 1, 4, 30, 0, 0, time.UTC), "Year Boundary", "Time", "Needs", api.Source{}),
	)
	charts, err := backend.GetChartData(ctx, tenant)
	if err != nil {
		t.Fatalf("GetChartData: %v", err)
	}
	if timeBucketAmount(charts.MonthlySpend, "2026-03") != 60 || timeBucketAmount(charts.DailySpend, "2026-03-31") != 30 {
		t.Fatalf("timezone chart buckets = %#v / %#v", charts.MonthlySpend, charts.DailySpend)
	}
	heatmap, err := backend.GetSpendingHeatmap(ctx, tenant, nil, nil)
	if err != nil {
		t.Fatalf("GetSpendingHeatmap: %v", err)
	}
	if got := weekdayHourBucket(heatmap, 0, 1); got.Count != 3 || got.Amount != 100 {
		t.Fatalf("DST repeated hour bucket = %#v, want count 3 amount 100", got)
	}
	if got := weekdayHourBucket(heatmap, 0, 2); got.Count != 0 {
		t.Fatalf("DST skipped hour bucket = %#v, want empty", got)
	}
	if got := dayOfMonthBucket(heatmap, 31); got.Count != 2 || got.Amount != 90 {
		t.Fatalf("local day-of-month bucket = %#v, want count 2 amount 90", got)
	}
	annual2025, err := backend.GetAnnualSpend(ctx, tenant, 2025)
	if err != nil || dailyBucketAmount(annual2025, "2025-12-31") != 60 {
		t.Fatalf("GetAnnualSpend local year = %#v / %v", annual2025, err)
	}
}

func testAnalyticsHeatmapAnnualAndBounds(ctx context.Context, t *testing.T, backend AnalyticsSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "analytics-heatmap")
	if err := backend.SetAppConfig(ctx, tenant, "app.timezone", "Asia/Kolkata"); err != nil {
		t.Fatalf("SetAppConfig: %v", err)
	}
	from := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.July, 2, 0, 0, 0, 0, time.UTC)
	writeTransactions(ctx, t, backend, tenant,
		transactionDetails("bound-from", 10, from, "From", "", "", api.Source{}),
		transactionDetails("bound-middle", 20, from.Add(12*time.Hour), "Middle", "", "", api.Source{}),
		transactionDetails("bound-to", 30, to, "To", "", "", api.Source{}),
		transactionDetails("bound-outside", 40, to.Add(time.Second), "Outside", "", "", api.Source{}),
		transactionDetails("bound-muted", 50, from.Add(time.Hour), "Muted", "", "", api.Source{}),
	)
	muted := transactionByMessage(ctx, t, backend, tenant, "bound-muted")
	if err := backend.MuteTransaction(ctx, tenant, muted.ID, true, "hidden"); err != nil {
		t.Fatalf("MuteTransaction: %v", err)
	}
	heatmap, err := backend.GetSpendingHeatmap(ctx, tenant, &from, &to)
	if err != nil || heatmapCount(heatmap) != 3 || heatmapAmount(heatmap) != 60 {
		t.Fatalf("inclusive heatmap = %#v / %v", heatmap, err)
	}
	annual, err := backend.GetAnnualSpend(ctx, tenant, 2026)
	if err != nil || annual == nil || dailyBucketTotalCount(annual) != 4 || dailyBucketTotalAmount(annual) != 100 || !dailyBucketsOrdered(annual) {
		t.Fatalf("GetAnnualSpend = %#v / %v", annual, err)
	}
}

func testAnalyticsMonthlyBreakdown(ctx context.Context, t *testing.T, backend AnalyticsSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "analytics-breakdown")
	if err := backend.SetAppConfig(ctx, tenant, "app.timezone", "Asia/Kolkata"); err != nil {
		t.Fatalf("SetAppConfig: %v", err)
	}
	jan := transactionDetails("breakdown-jan", 10, time.Date(2026, time.January, 5, 6, 0, 0, 0, time.UTC), "Jan", "Food", "Needs", api.Source{}, "Dining")
	feb := transactionDetails("breakdown-feb", 20, time.Date(2026, time.February, 5, 6, 0, 0, 0, time.UTC), "Feb", "Travel", "Wants", api.Source{})
	mar := transactionDetails("breakdown-mar", 30, time.Date(2026, time.March, 5, 6, 0, 0, 0, time.UTC), "Mar", "Food", "Needs", api.Source{}, "Dining")
	muted := transactionDetails("breakdown-muted", 40, time.Date(2026, time.April, 5, 6, 0, 0, 0, time.UTC), "Muted", "Hidden", "Hidden", api.Source{}, "Hidden")
	writeTransactions(ctx, t, backend, tenant, jan, feb, mar, muted)
	mutedRow := transactionByMessage(ctx, t, backend, tenant, muted.MessageID)
	if err := backend.MuteTransaction(ctx, tenant, mutedRow.ID, true, "hidden"); err != nil {
		t.Fatalf("MuteTransaction: %v", err)
	}
	for _, tc := range []struct {
		dimension string
		label     string
		want      []float64
	}{
		{"labels", "Dining", []float64{10, 0, 30, 0}},
		{"", "Dining", []float64{10, 0, 30, 0}},
		{"categories", "Food", []float64{10, 0, 30, 0}},
		{"buckets", "Needs", []float64{10, 0, 30, 0}},
	} {
		data, err := backend.GetMonthlyBreakdownSpend(ctx, tenant, tc.dimension, 4)
		if err != nil || !reflect.DeepEqual(data.Months, []string{"2026-01", "2026-02", "2026-03", "2026-04"}) ||
			!reflect.DeepEqual(monthlyBreakdownSeries(data, tc.label), tc.want) || containsString(data.Labels, "Hidden") {
			t.Fatalf("monthly breakdown %q = %#v / %v", tc.dimension, data, err)
		}
	}
	_, err := backend.GetMonthlyBreakdownSpend(ctx, tenant, "merchants", 4)
	assertErrorKind(t, err, errors.InvalidInput, "GetMonthlyBreakdownSpend invalid dimension")
	for _, months := range []int{0, -1} {
		data, err := backend.GetMonthlyBreakdownSpend(ctx, tenant, "invalid-is-ignored", months)
		if err != nil || data.Labels == nil || data.Months == nil || data.Series == nil || len(data.Labels)+len(data.Months)+len(data.Series) != 0 {
			t.Fatalf("monthly breakdown months=%d = %#v / %v", months, data, err)
		}
	}
}

func testAnalyticsTenantIsolation(ctx context.Context, t *testing.T, backend AnalyticsSubject) {
	t.Helper()

	tenantA := createTenant(ctx, t, backend, "analytics-isolation-a")
	tenantB := createTenant(ctx, t, backend, "analytics-isolation-b")
	if err := backend.SetAppConfig(ctx, tenantA, "base_currency", baseCurrencyUSD); err != nil {
		t.Fatalf("SetAppConfig tenant A: %v", err)
	}
	if err := backend.SetAppConfig(ctx, tenantB, "base_currency", baseCurrencyINR); err != nil {
		t.Fatalf("SetAppConfig tenant B: %v", err)
	}
	txnA := transactionDetails("analytics-isolated", 10, time.Date(2026, time.April, 5, 6, 0, 0, 0, time.UTC), "Tenant A", "A", "A", api.Source{}, "A")
	txnA.Currency = baseCurrencyUSD
	txnB := transactionDetails("analytics-isolated", 99, time.Date(2026, time.April, 6, 6, 0, 0, 0, time.UTC), "Tenant B", "B", "B", api.Source{}, "B")
	writeTransactions(ctx, t, backend, tenantA, txnA)
	writeTransactions(ctx, t, backend, tenantB, txnB)
	stats, err := backend.GetStats(ctx, tenantA, baseCurrencyUSD)
	if err != nil || stats.TotalCount != 1 || stats.TotalBase != 10 || stats.TotalByCategory["A"] != 10 {
		t.Fatalf("tenant A stats = %#v / %v", stats, err)
	}
	charts, err := backend.GetChartData(ctx, tenantA)
	if err != nil || charts.ByCategory["A"] != 10 || charts.ByCategory["B"] != 0 {
		t.Fatalf("tenant A charts = %#v / %v", charts, err)
	}
	dashboard, err := backend.GetDashboardData(ctx, tenantA)
	if err != nil || dashboard.AllTime.Stats.TotalCount != 1 || dashboard.AllTime.Stats.TotalBase != 10 {
		t.Fatalf("tenant A dashboard = %#v / %v", dashboard, err)
	}
	heatmap, err := backend.GetSpendingHeatmap(ctx, tenantA, nil, nil)
	if err != nil || heatmapCount(heatmap) != 1 || heatmapAmount(heatmap) != 10 {
		t.Fatalf("tenant A heatmap = %#v / %v", heatmap, err)
	}
	annual, err := backend.GetAnnualSpend(ctx, tenantA, 2026)
	if err != nil || dailyBucketTotalCount(annual) != 1 || dailyBucketTotalAmount(annual) != 10 {
		t.Fatalf("tenant A annual = %#v / %v", annual, err)
	}
	breakdown, err := backend.GetMonthlyBreakdownSpend(ctx, tenantA, "categories", 1)
	if err != nil || !reflect.DeepEqual(monthlyBreakdownSeries(breakdown, "A"), []float64{10}) || monthlyBreakdownSeries(breakdown, "B") != nil {
		t.Fatalf("tenant A breakdown = %#v / %v", breakdown, err)
	}
}

func testDiagnostics(ctx context.Context, t *testing.T, backend DiagnosticsSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "diagnostics")
	receivedAt := time.Now().UTC().Truncate(time.Microsecond)

	if err := backend.RecordExtractionDiagnostic(ctx, tenant, api.ExtractionDiagnostic{
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
		t.Fatalf("RecordExtractionDiagnostic: %v", err)
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

func testDiagnosticsValidationAndIsolation(ctx context.Context, t *testing.T, backend DiagnosticsSubject) {
	t.Helper()

	userA := createUser(ctx, t, backend, "diagnostics-isolation-a")
	tenantA := store.Tenant{ID: userA.TenantID}
	tenantB := createTenant(ctx, t, backend, "diagnostics-isolation-b")
	if err := backend.RecordExtractionDiagnostic(ctx, tenantA, api.ExtractionDiagnostic{
		Reader: "reader", MessageID: "isolated", RuleName: "Isolation Rule", Subject: "unchanged",
	}); err != nil {
		t.Fatalf("RecordExtractionDiagnostic: %v", err)
	}
	rowsA := listDiagnostics(ctx, t, backend, tenantA, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen})
	if len(rowsA) != 1 {
		t.Fatalf("tenant A diagnostics = %#v, want one", rowsA)
	}
	rowsB := listDiagnostics(ctx, t, backend, tenantB, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen})
	if rowsB == nil || len(rowsB) != 0 {
		t.Fatalf("tenant B diagnostics = %#v, want non-nil empty", rowsB)
	}
	_, err := backend.GetExtractionDiagnostic(ctx, tenantB, rowsA[0].ID)
	assertErrorKind(t, err, errors.NotFound, "GetExtractionDiagnostic other tenant")
	_, err = backend.UpdateExtractionDiagnosticStatus(ctx, tenantB, rowsA[0].ID, store.DiagnosticStatusResolved)
	assertErrorKind(t, err, errors.NotFound, "UpdateExtractionDiagnosticStatus other tenant")

	_, err = backend.ListExtractionDiagnostics(ctx, tenantA, store.DiagnosticFilter{Status: "pending"})
	assertErrorKind(t, err, errors.InvalidInput, "ListExtractionDiagnostics invalid status")
	_, err = backend.UpdateExtractionDiagnosticStatus(ctx, tenantA, rowsA[0].ID, store.DiagnosticStatusAll)
	assertErrorKind(t, err, errors.InvalidInput, "UpdateExtractionDiagnosticStatus invalid status")
	unchanged, err := backend.GetExtractionDiagnostic(ctx, tenantA, rowsA[0].ID)
	if err != nil || unchanged.Status != store.DiagnosticStatusOpen || unchanged.Subject != "unchanged" {
		t.Fatalf("invalid diagnostic operations changed row = %#v / %v", unchanged, err)
	}
	assertErrorKind(t, backend.RecordExtractionDiagnostic(ctx, store.Tenant{}, api.ExtractionDiagnostic{
		Reader: "reader", MessageID: "missing-tenant", RuleName: "Missing Tenant",
	}), errors.InvalidInput, "RecordExtractionDiagnostic empty tenant")
}

func testDiagnosticsListOrderAndLimit(ctx context.Context, t *testing.T, backend DiagnosticsSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "diagnostics-list")
	for i, status := range []string{store.DiagnosticStatusResolved, store.DiagnosticStatusIgnored, store.DiagnosticStatusOpen} {
		messageID := fmt.Sprintf("list-%d", i)
		if err := backend.RecordExtractionDiagnostic(ctx, tenant, api.ExtractionDiagnostic{
			Reader: "reader", MessageID: messageID, RuleName: "List Rule", Subject: messageID,
		}); err != nil {
			t.Fatalf("RecordExtractionDiagnostic %d: %v", i, err)
		}
		open := listDiagnostics(ctx, t, backend, tenant, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen})
		row := diagnosticByMessage(open, messageID)
		if row == nil {
			t.Fatalf("open diagnostics missing %q: %#v", messageID, open)
		}
		if status != store.DiagnosticStatusOpen {
			if _, err := backend.UpdateExtractionDiagnosticStatus(ctx, tenant, row.ID, status); err != nil {
				t.Fatalf("UpdateExtractionDiagnosticStatus %s: %v", status, err)
			}
		}
	}
	all := listDiagnostics(ctx, t, backend, tenant, store.DiagnosticFilter{Status: store.DiagnosticStatusAll})
	if len(all) != 3 || !diagnosticsNewestFirst(all) {
		t.Fatalf("all diagnostics order = %#v", all)
	}
	limited := listDiagnostics(ctx, t, backend, tenant, store.DiagnosticFilter{Status: store.DiagnosticStatusAll, Limit: 2})
	if len(limited) != 2 || !diagnosticsNewestFirst(limited) {
		t.Fatalf("limited diagnostics order = %#v", limited)
	}
	cutoff := all[len(limited)-1].CreatedAt
	seen := make(map[string]bool, len(limited))
	for _, row := range limited {
		if row.CreatedAt.Before(cutoff) || diagnosticByID(all, row.ID) == nil || seen[row.ID] {
			t.Fatalf("limited diagnostics do not contain two newest rows: all=%#v limited=%#v", all, limited)
		}
		seen[row.ID] = true
	}
}

func testDiagnosticsDeduplication(ctx context.Context, t *testing.T, backend DiagnosticsSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "diagnostics-dedup")
	first := api.ExtractionDiagnostic{
		Reader: "reader", MessageID: "dedup", RuleName: "Dedup Rule", SenderEmail: "old@example.test",
		Subject: "old", FailureReasons: []string{"old_reason"},
	}
	if err := backend.RecordExtractionDiagnostic(ctx, tenant, first); err != nil {
		t.Fatalf("RecordExtractionDiagnostic first: %v", err)
	}
	before := listDiagnostics(ctx, t, backend, tenant, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen})
	second := first
	second.SenderEmail = "new@example.test"
	second.Subject = "new"
	second.AmountRegex = "amount"
	second.MerchantRegex = "merchant"
	second.CurrencyRegex = "currency"
	second.FailureReasons = []string{"amount_missing", "merchant_missing"}
	if err := backend.RecordExtractionDiagnostic(ctx, tenant, second); err != nil {
		t.Fatalf("RecordExtractionDiagnostic duplicate: %v", err)
	}
	after := listDiagnostics(ctx, t, backend, tenant, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen})
	if len(before) != 1 || len(after) != 1 || after[0].ID != before[0].ID || after[0].Subject != second.Subject ||
		after[0].SenderEmail != second.SenderEmail || !reflect.DeepEqual(after[0].FailureReasons, second.FailureReasons) {
		t.Fatalf("open diagnostic dedup before=%#v after=%#v", before, after)
	}
}

func testDiagnosticsEmptyMessageAndFallbacks(ctx context.Context, t *testing.T, backend DiagnosticsSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "diagnostics-fallbacks")
	for _, subject := range []string{"first empty message", "second empty message"} {
		if err := backend.RecordExtractionDiagnostic(ctx, tenant, api.ExtractionDiagnostic{
			Reader: "reader", RuleName: "No Message", Subject: subject, SenderEmail: "fallback@example.test",
			FailureReasons: []string{"first", "second"},
		}); err != nil {
			t.Fatalf("RecordExtractionDiagnostic %q: %v", subject, err)
		}
	}
	rows := listDiagnostics(ctx, t, backend, tenant, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen})
	if len(rows) != 2 {
		t.Fatalf("empty message diagnostics deduplicated: %#v", rows)
	}
	for _, row := range rows {
		if row.MessageID != "" || row.Sender != "fallback@example.test" || row.RuleID != nil ||
			!reflect.DeepEqual(row.FailureReasons, []string{"first", "second"}) {
			t.Fatalf("diagnostic fallback round trip = %#v", row)
		}
	}
}

func testDiagnosticsReopenConflict(ctx context.Context, t *testing.T, backend DiagnosticsSubject) {
	t.Helper()

	tenant := createTenant(ctx, t, backend, "diagnostics-reopen")
	diagnostic := api.ExtractionDiagnostic{Reader: "reader", MessageID: "reopen", RuleName: "Reopen Rule", Subject: "original"}
	if err := backend.RecordExtractionDiagnostic(ctx, tenant, diagnostic); err != nil {
		t.Fatalf("RecordExtractionDiagnostic original: %v", err)
	}
	original := listDiagnostics(ctx, t, backend, tenant, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen})[0]
	if _, err := backend.UpdateExtractionDiagnosticStatus(ctx, tenant, original.ID, store.DiagnosticStatusResolved); err != nil {
		t.Fatalf("resolve original: %v", err)
	}
	diagnostic.Subject = "current"
	if err := backend.RecordExtractionDiagnostic(ctx, tenant, diagnostic); err != nil {
		t.Fatalf("RecordExtractionDiagnostic current: %v", err)
	}
	current := listDiagnostics(ctx, t, backend, tenant, store.DiagnosticFilter{Status: store.DiagnosticStatusOpen})[0]
	_, err := backend.UpdateExtractionDiagnosticStatus(ctx, tenant, original.ID, store.DiagnosticStatusOpen)
	assertErrorKind(t, err, errors.Conflict, "UpdateExtractionDiagnosticStatus reopen conflict")
	resolved, err := backend.GetExtractionDiagnostic(ctx, tenant, original.ID)
	if err != nil || resolved.Status != store.DiagnosticStatusResolved || resolved.Subject != "original" {
		t.Fatalf("reopen conflict changed resolved row = %#v / %v", resolved, err)
	}
	open, err := backend.GetExtractionDiagnostic(ctx, tenant, current.ID)
	if err != nil || open.Status != store.DiagnosticStatusOpen || open.Subject != "current" {
		t.Fatalf("reopen conflict changed open row = %#v / %v", open, err)
	}
}

func testDiagnosticsNotFoundAndCascade(ctx context.Context, t *testing.T, backend DiagnosticsSubject) {
	t.Helper()

	user := createUser(ctx, t, backend, "diagnostics-cascade")
	tenant := store.Tenant{ID: user.TenantID}
	missingID := "00000000-0000-0000-0000-000000000000"
	_, err := backend.GetExtractionDiagnostic(ctx, tenant, missingID)
	assertErrorKind(t, err, errors.NotFound, "GetExtractionDiagnostic missing")
	_, err = backend.UpdateExtractionDiagnosticStatus(ctx, tenant, missingID, store.DiagnosticStatusResolved)
	assertErrorKind(t, err, errors.NotFound, "UpdateExtractionDiagnosticStatus missing")
	if err := backend.RecordExtractionDiagnostic(ctx, tenant, api.ExtractionDiagnostic{
		Reader: "reader", MessageID: "cascade", RuleName: "Cascade Rule",
	}); err != nil {
		t.Fatalf("RecordExtractionDiagnostic cascade: %v", err)
	}
	if err := backend.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	rows := listDiagnostics(ctx, t, backend, tenant, store.DiagnosticFilter{Status: store.DiagnosticStatusAll})
	if rows == nil || len(rows) != 0 {
		t.Fatalf("diagnostics after user delete = %#v, want non-nil empty", rows)
	}
}

func createTenant(ctx context.Context, t *testing.T, backend store.AuthStore, name string) store.Tenant {
	t.Helper()

	user := createUser(ctx, t, backend, name)
	return store.Tenant{ID: user.TenantID}
}

func createUser(ctx context.Context, t *testing.T, backend store.AuthStore, name string) *store.User {
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
	return user
}

//nolint:revive // Positional fields keep token fixtures concise and readable at call sites.
func createAccessToken(ctx context.Context, t *testing.T, backend store.AuthStore, userID, name, hash string) *store.AccessToken {
	t.Helper()

	token, err := backend.CreateAccessToken(ctx, store.CreateAccessTokenInput{UserID: userID, Name: name, TokenHash: hash})
	if err != nil {
		t.Fatalf("CreateAccessToken(%s): %v", name, err)
	}
	return token
}

//nolint:revive // Positional fields keep setup-token fixtures concise and readable at call sites.
func createSetupToken(
	ctx context.Context,
	t *testing.T,
	backend store.AuthStore,
	userID, hash string,
	expiresAt time.Time,
) *store.AccountSetupToken {
	t.Helper()

	token, err := backend.CreateAccountSetupToken(ctx, store.CreateAccountSetupTokenInput{
		UserID: userID, TokenHash: hash, ExpiresAt: expiresAt,
	})
	if err != nil {
		t.Fatalf("CreateAccountSetupToken(%s): %v", hash, err)
	}
	return token
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

func assertErrorKind(t *testing.T, err error, want errors.Kind, operation string) {
	t.Helper()
	if got := errors.WhatKind(err); got != want {
		t.Fatalf("%s error kind = %#v, want %#v: %v", operation, got, want, err)
	}
}

func assertErrorOperation(t *testing.T, err error, want, operation string) {
	t.Helper()
	var appErr *errors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("%s error = %v, want structured error", operation, err)
	}
	if ops := appErr.Ops(); !containsString(ops, want) {
		t.Fatalf("%s error operations = %#v, want %q", operation, ops, want)
	}
}

type runtimeBytesResult struct {
	value []byte
	found bool
	err   error
}

func runtimeBytes(value []byte, found bool, err error) runtimeBytesResult {
	return runtimeBytesResult{value: value, found: found, err: err}
}

func assertRuntimeBytes(t *testing.T, result runtimeBytesResult, want []byte, operation string) {
	t.Helper()
	if result.err != nil || !result.found {
		t.Fatalf("%s found=%v err=%v", operation, result.found, result.err)
	}
	assertJSON(t, result.value, want)
}

func assertRuntimeMissing(t *testing.T, result runtimeBytesResult, operation string) {
	t.Helper()
	if result.err != nil || result.found {
		t.Fatalf("%s found=%v err=%v, want false/nil", operation, result.found, result.err)
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

func hasCategoryWithValues(categories []store.Category, name, description string, isDefault bool) bool {
	for _, category := range categories {
		if category.Name == name && category.Description == description && category.IsDefault == isDefault {
			return true
		}
	}
	return false
}

func countCategory(categories []store.Category, name string) int {
	count := 0
	for _, category := range categories {
		if category.Name == name {
			count++
		}
	}
	return count
}

func hasBucket(buckets []store.Bucket, name string) bool {
	for _, bucket := range buckets {
		if bucket.Name == name {
			return true
		}
	}
	return false
}

func hasBucketWithValues(buckets []store.Bucket, name, description string, isDefault bool) bool {
	for _, bucket := range buckets {
		if bucket.Name == name && bucket.Description == description && bucket.IsDefault == isDefault {
			return true
		}
	}
	return false
}

func countBucket(buckets []store.Bucket, name string) int {
	count := 0
	for _, bucket := range buckets {
		if bucket.Name == name {
			count++
		}
	}
	return count
}

func hasRule(rules []store.RuleRow, id string) bool {
	for _, rule := range rules {
		if rule.ID == id {
			return true
		}
	}
	return false
}

func ruleNamed(rules []store.RuleRow, name string) *store.RuleRow {
	for i := range rules {
		if rules[i].Name == name {
			return &rules[i]
		}
	}
	return nil
}

func countRulesNamed(rules []store.RuleRow, name string) int {
	count := 0
	for _, rule := range rules {
		if rule.Name == name {
			count++
		}
	}
	return count
}

func countPredefinedRulesNamed(rules []store.RuleRow, name string) int {
	count := 0
	for _, rule := range rules {
		if rule.Name == name && rule.Predefined {
			count++
		}
	}
	return count
}

func transactionByMessage(
	ctx context.Context,
	t *testing.T,
	backend store.TransactionStore,
	tenant store.Tenant,
	messageID string,
) store.Transaction {
	t.Helper()
	rows, _, err := backend.ListTransactions(ctx, tenant, store.ListFilter{Page: 1, PageSize: 100, ShowMuted: true})
	if err != nil {
		t.Fatalf("ListTransactions for message %q: %v", messageID, err)
	}
	for _, row := range rows {
		if row.MessageID == messageID {
			return row
		}
	}
	t.Fatalf("ListTransactions missing message %q: %#v", messageID, rows)
	return store.Transaction{}
}

//nolint:revive // Positional fields make the many transaction fixtures easy to compare.
func transactionDetails(
	messageID string,
	amount float64,
	timestamp time.Time,
	merchant, category, bucket string,
	source api.Source,
	labels ...string,
) *api.TransactionDetails {
	cleanLabels := make([]string, 0, len(labels))
	for _, label := range labels {
		if label != "" {
			cleanLabels = append(cleanLabels, label)
		}
	}
	return &api.TransactionDetails{
		MessageID: messageID, Amount: amount, Currency: baseCurrencyINR, Timestamp: timestamp.Format(time.RFC3339Nano),
		MerchantInfo: merchant, Category: category, Bucket: bucket, Source: source, Labels: cleanLabels,
	}
}

func writeTransactions(
	ctx context.Context,
	t *testing.T,
	backend store.TransactionBatchWriter,
	tenant store.Tenant,
	transactions ...*api.TransactionDetails,
) {
	t.Helper()
	if err := backend.Write(ctx, store.IngestionBatch{Tenant: tenant, Transactions: transactions}); err != nil {
		t.Fatalf("Write transactions: %v", err)
	}
}

func listTransactions(
	ctx context.Context,
	t *testing.T,
	backend store.TransactionStore,
	tenant store.Tenant,
	filter store.ListFilter,
) ([]store.Transaction, store.TransactionListResult) {
	t.Helper()
	rows, result, err := backend.ListTransactions(ctx, tenant, filter)
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	return rows, result
}

func assertSingleTransactionMessage(t *testing.T, rows []store.Transaction, result store.TransactionListResult, want string) {
	t.Helper()
	if result.Total != 1 || len(rows) != 1 || rows[0].MessageID != want {
		t.Fatalf("transactions rows=%#v result=%#v, want only %q", rows, result, want)
	}
}

func assertTransactionMessages(t *testing.T, rows []store.Transaction, want ...string) {
	t.Helper()
	gotCounts := make(map[string]int, len(rows))
	for _, row := range rows {
		gotCounts[row.MessageID]++
	}
	wantCounts := make(map[string]int, len(want))
	for _, messageID := range want {
		wantCounts[messageID]++
	}
	if !reflect.DeepEqual(gotCounts, wantCounts) {
		t.Fatalf("transaction messages = %#v, want %#v; rows=%#v", gotCounts, wantCounts, rows)
	}
}

//nolint:revive // Keeping query inputs visible makes search assertions easier to read.
func assertSearchMessages(
	ctx context.Context,
	t *testing.T,
	backend store.TransactionStore,
	tenant store.Tenant,
	query string,
	filter store.ListFilter,
	want ...string,
) {
	t.Helper()
	rows, result, err := backend.SearchTransactions(ctx, tenant, query, filter)
	if err != nil {
		t.Fatalf("SearchTransactions(%q): %v", query, err)
	}
	if result.Total != len(want) {
		t.Fatalf("SearchTransactions(%q) total=%d, want %d; rows=%#v", query, result.Total, len(want), rows)
	}
	assertTransactionMessages(t, rows, want...)
}

func assertEmptyFacets(ctx context.Context, t *testing.T, backend store.TransactionStore, tenant store.Tenant) {
	t.Helper()
	facets, err := backend.GetFacets(ctx, tenant)
	if err != nil {
		t.Fatalf("GetFacets empty: %v", err)
	}
	if facets == nil || facets.Sources == nil || facets.SourceTypes == nil || facets.Banks == nil || facets.Categories == nil ||
		facets.Currencies == nil || facets.Merchants == nil || facets.Labels == nil || facets.Buckets == nil ||
		facets.CategoryCounts == nil || facets.BucketCounts == nil || facets.LabelCounts == nil ||
		len(facets.Sources)+len(facets.SourceTypes)+len(facets.Banks)+len(facets.Categories)+len(facets.Currencies)+
			len(facets.Merchants)+len(facets.Labels)+len(facets.Buckets)+len(facets.CategoryCounts)+len(facets.BucketCounts)+len(facets.LabelCounts) != 0 {
		t.Fatalf("GetFacets empty = %#v", facets)
	}
}

func refreshTimestamp(t *testing.T, transaction *api.TransactionDetails) time.Time {
	t.Helper()
	timestamp, err := time.Parse(time.RFC3339, transaction.Timestamp)
	if err != nil {
		t.Fatalf("parse transaction timestamp: %v", err)
	}
	return timestamp
}

func countString(values []string, want string) int {
	count := 0
	for _, value := range values {
		if value == want {
			count++
		}
	}
	return count
}

func assertEmptyChartData(t *testing.T, data *store.ChartData, operation string) {
	t.Helper()
	if data == nil || data.MonthlySpend == nil || data.DailySpend == nil || data.ByCategory == nil || data.ByBucket == nil ||
		data.ByLabel == nil || data.BySource == nil || data.BySourceType == nil || data.ByBank == nil || data.ByCategoryMonthly == nil ||
		len(data.MonthlySpend)+len(data.DailySpend)+len(data.ByCategory)+len(data.ByBucket)+len(data.ByLabel)+
			len(data.BySource)+len(data.BySourceType)+len(data.ByBank)+len(data.ByCategoryMonthly) != 0 {
		t.Fatalf("%s = %#v, want non-nil empty fields", operation, data)
	}
}

func timeBucketAmount(buckets []store.TimeBucket, period string) float64 {
	for _, bucket := range buckets {
		if bucket.Period == period {
			return bucket.Amount
		}
	}
	return 0
}

func weekdayHourBucket(data *store.HeatmapData, weekday, hour int) store.WeekdayHourBucket {
	for _, bucket := range data.ByWeekdayHour {
		if bucket.Weekday == weekday && bucket.Hour == hour {
			return bucket
		}
	}
	return store.WeekdayHourBucket{Weekday: weekday, Hour: hour}
}

func dayOfMonthBucket(data *store.HeatmapData, day int) store.DayOfMonthBucket {
	for _, bucket := range data.ByDayOfMonth {
		if bucket.Day == day {
			return bucket
		}
	}
	return store.DayOfMonthBucket{Day: day}
}

func heatmapCount(data *store.HeatmapData) int {
	count := 0
	for _, bucket := range data.ByWeekdayHour {
		count += bucket.Count
	}
	return count
}

func heatmapAmount(data *store.HeatmapData) float64 {
	amount := 0.0
	for _, bucket := range data.ByWeekdayHour {
		amount += bucket.Amount
	}
	return amount
}

func dailyBucketAmount(buckets []store.DailyBucket, date string) float64 {
	for _, bucket := range buckets {
		if bucket.Date.Format(time.DateOnly) == date {
			return bucket.Amount
		}
	}
	return 0
}

func dailyBucketTotalCount(buckets []store.DailyBucket) int {
	count := 0
	for _, bucket := range buckets {
		count += bucket.Count
	}
	return count
}

func dailyBucketTotalAmount(buckets []store.DailyBucket) float64 {
	amount := 0.0
	for _, bucket := range buckets {
		amount += bucket.Amount
	}
	return amount
}

func dailyBucketsOrdered(buckets []store.DailyBucket) bool {
	for i := 1; i < len(buckets); i++ {
		if buckets[i].Date.Before(buckets[i-1].Date) {
			return false
		}
	}
	return true
}

func monthlyBreakdownSeries(data *store.MonthlyBreakdownData, label string) []float64 {
	for _, series := range data.Series {
		if series.Label == label {
			return series.Data
		}
	}
	return nil
}

//nolint:revive // Expected category and bucket values belong together at each assertion site.
func assertCommunityResolution(
	ctx context.Context,
	t *testing.T,
	backend store.CommunityStore,
	merchant, wantCategory, wantBucket string,
) {
	t.Helper()
	resolver, err := backend.LoadCategorySnapshot(ctx)
	if err != nil {
		t.Fatalf("LoadCategorySnapshot: %v", err)
	}
	category, bucket := resolver(merchant)
	if category != wantCategory || bucket != wantBucket {
		t.Fatalf("category resolution for %q = (%q, %q), want (%q, %q)", merchant, category, bucket, wantCategory, wantBucket)
	}
}

func listDiagnostics(
	ctx context.Context,
	t *testing.T,
	backend store.DiagnosticStore,
	tenant store.Tenant,
	filter store.DiagnosticFilter,
) []store.ExtractionDiagnosticRow {
	t.Helper()
	rows, err := backend.ListExtractionDiagnostics(ctx, tenant, filter)
	if err != nil {
		t.Fatalf("ListExtractionDiagnostics: %v", err)
	}
	return rows
}

func diagnosticByMessage(rows []store.ExtractionDiagnosticRow, messageID string) *store.ExtractionDiagnosticRow {
	for i := range rows {
		if rows[i].MessageID == messageID {
			return &rows[i]
		}
	}
	return nil
}

func diagnosticByID(rows []store.ExtractionDiagnosticRow, id string) *store.ExtractionDiagnosticRow {
	for i := range rows {
		if rows[i].ID == id {
			return &rows[i]
		}
	}
	return nil
}

func diagnosticsNewestFirst(rows []store.ExtractionDiagnosticRow) bool {
	for i := 1; i < len(rows); i++ {
		if rows[i-1].CreatedAt.Before(rows[i].CreatedAt) {
			return false
		}
	}
	return true
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

func hasScanningTenant(states []store.TenantScanningState, tenantID string) bool {
	for _, state := range states {
		if state.TenantID == tenantID {
			return true
		}
	}
	return false
}
