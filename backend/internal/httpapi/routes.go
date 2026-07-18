package httpapi

import (
	"net/http"
	"strings"

	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

// registerRoutes attaches all API routes to mux.
func registerRoutes(mux *http.ServeMux, h *Handlers) {
	registerBootstrapRoutes(mux, h)
	registerScanningRoutes(mux, h)
	registerLLMProviderRoutes(mux, h)
	registerReaderRoutes(mux, h)
	registerStatsRoutes(mux, h)
	registerConfigurationRoutes(mux, h)
	registerTaxonomyRoutes(mux, h)
	registerRuleRoutes(mux, h)
	registerTransactionRoutes(mux, h)
	registerDiagnosticRoutes(mux, h)
	registerMerchantRoutes(mux, h)
}

func registerBootstrapRoutes(mux *http.ServeMux, h *Handlers) {
	mux.HandleFunc("GET /api/health", h.Health)
	mux.HandleFunc("GET /api/status", h.Status)
	mux.HandleFunc("GET /api/version", h.Version)
	mux.HandleFunc("GET /api/bootstrap", h.GetBootstrap)
	mux.HandleFunc("POST /api/bootstrap", h.Bootstrap)
	mux.HandleFunc("POST /api/session", h.Login)
	mux.HandleFunc("GET /api/account-setup", h.GetAccountSetup)
	mux.HandleFunc("POST /api/account-setup", h.CompleteAccountSetup)
	mux.HandleFunc("GET /api/session", h.GetSession)
	mux.HandleFunc("DELETE /api/session", h.Logout)
	mux.HandleFunc("GET /api/profile", h.GetProfile)
	mux.HandleFunc("PATCH /api/profile", h.UpdateProfile)
	mux.HandleFunc("PATCH /api/profile/password", h.UpdatePassword)
	mux.HandleFunc("GET /api/tokens", h.ListAccessTokens)
	mux.HandleFunc("POST /api/tokens", h.CreateAccessToken)
	mux.HandleFunc("DELETE /api/tokens/{id}", h.RevokeAccessToken)
	mux.HandleFunc("GET /api/admin/users", h.ListUsers)
	mux.HandleFunc("POST /api/admin/users", h.CreateUser)
	mux.HandleFunc("PATCH /api/admin/users/{id}", h.UpdateUser)
	mux.HandleFunc("DELETE /api/admin/users/{id}", h.DeleteUser)
	mux.HandleFunc("POST /api/admin/users/{id}/setup-tokens", h.CreateSetupToken)
}

func registerScanningRoutes(mux *http.ServeMux, h *Handlers) {
	mux.HandleFunc("GET /api/admin/scanning/settings", h.GetAdminScanningSettings)
	mux.HandleFunc("PATCH /api/admin/scanning/settings", h.PatchAdminScanningSettings)
	mux.HandleFunc("GET /api/admin/logging/settings", h.GetAdminLoggingSettings)
	mux.HandleFunc("PATCH /api/admin/logging/settings", h.PatchAdminLoggingSettings)
	mux.HandleFunc("POST /api/daemon/start", h.StartDaemon)
	mux.HandleFunc("POST /api/daemon/rescan", h.Rescan)
	mux.HandleFunc("GET /api/scanning/settings", h.GetScanningSettings)
	mux.HandleFunc("PATCH /api/scanning/settings", h.PatchScanningSettings)
	mux.HandleFunc("GET /api/scanning/status", h.GetScanningStatus)
	mux.HandleFunc("POST /api/scanning/rescans", h.CreateScanningRescan)
}

func registerLLMProviderRoutes(mux *http.ServeMux, h *Handlers) {
	mux.HandleFunc("GET /api/llm/providers", h.ListLLMProviders)
	mux.HandleFunc("GET /api/llm/providers/{name}/status", h.GetLLMProviderStatus)
	mux.HandleFunc("PUT /api/llm/providers/{name}/config", h.SaveLLMProviderConfig)
	mux.HandleFunc("PUT /api/llm/providers/{name}/credentials", h.SaveLLMProviderCredentials)
	mux.HandleFunc("POST /api/llm/providers/{name}/healthcheck", h.HealthCheckLLMProvider)
	mux.HandleFunc("POST /api/llm/providers/{name}/activate", h.ActivateLLMProvider)
	mux.HandleFunc("DELETE /api/llm/providers/{name}", h.DisconnectLLMProvider)
}

func registerReaderRoutes(mux *http.ServeMux, h *Handlers) {
	mux.HandleFunc("GET /api/providers", h.ListProviders)
	mux.HandleFunc("GET /api/providers/thunderbird/discover/profiles", h.DiscoverProfiles)
	mux.HandleFunc("GET /api/providers/thunderbird/discover/mailboxes", h.DiscoverMailboxes)
	mux.HandleFunc("GET /api/providers/{name}/guide", h.GetProviderGuide)
	mux.HandleFunc("POST /api/providers/{name}/credentials", h.UploadCredentials)
	mux.HandleFunc("GET /api/providers/{name}/credentials/status", h.CredentialsStatus)
	mux.HandleFunc("POST /api/providers/{name}/auth/start", h.AuthStart)
	mux.HandleFunc("GET /api/auth/callback", h.AuthCallback)
	mux.HandleFunc("POST /api/providers/{name}/auth/exchange", h.AuthExchange)
	mux.HandleFunc("GET /api/providers/{name}/auth/status", h.AuthStatus)
	mux.HandleFunc("DELETE /api/providers/{name}/auth/token", h.RevokeToken)
	mux.HandleFunc("GET /api/providers/{name}/config", h.GetReaderConfig)
	mux.HandleFunc("PUT /api/providers/{name}/config", h.SaveReaderConfig)
	mux.HandleFunc("GET /api/providers/{name}/status", h.ReaderStatus)
	mux.HandleFunc("GET /api/providers/{name}/messages", h.SearchProviderMessages)
	mux.HandleFunc("DELETE /api/providers/{name}", h.DisconnectReader)
}

func registerStatsRoutes(mux *http.ServeMux, h *Handlers) {
	mux.HandleFunc("GET /api/stats/dashboard", h.GetDashboardData)
	mux.HandleFunc("GET /api/stats/charts", h.GetChartData)
	mux.HandleFunc("GET /api/stats/labels/monthly", h.GetLabelMonthlySpend)
	mux.HandleFunc("GET /api/stats/heatmap", h.GetHeatmap)
}

func registerConfigurationRoutes(mux *http.ServeMux, h *Handlers) {
	mux.HandleFunc("GET /api/config/banks", h.ListBanks)
	mux.HandleFunc("GET /api/config/setup-status", h.GetSetupStatus)
	mux.HandleFunc("POST /api/config/sync", h.TriggerSync)
	mux.HandleFunc("GET /api/config/sync/status", h.GetSyncStatus)
	mux.HandleFunc("GET /api/config/sync/settings", h.GetCommunitySyncSettings)
	mux.HandleFunc("PATCH /api/config/sync/settings", h.PatchCommunitySyncSettings)
	mux.HandleFunc("GET /api/config/preferences", h.GetPreferences)
	mux.HandleFunc("PATCH /api/config/preferences", h.PatchPreferences)
	mux.HandleFunc("GET /api/config/providers/{name}/checkpoint", h.GetReaderCheckpoint)
	mux.HandleFunc("DELETE /api/config/providers/{name}/checkpoint", h.ClearReaderCheckpoint)
}

func registerTaxonomyRoutes(mux *http.ServeMux, h *Handlers) {
	mux.HandleFunc("GET /api/config/labels/export", h.ExportLabels)
	mux.HandleFunc("GET /api/config/labels/mappings", h.GetLabelMappings)
	mux.HandleFunc("GET /api/config/labels", h.ListLabels)
	mux.HandleFunc("POST /api/config/labels", h.CreateLabel)
	mux.HandleFunc("PUT /api/config/labels/{name}", h.UpdateLabel)
	mux.HandleFunc("DELETE /api/config/labels/{name}", h.DeleteLabel)
	mux.HandleFunc("PUT /api/config/labels/{name}/merchant-mappings/{pattern}", h.ApplyLabel)
	mux.HandleFunc("DELETE /api/config/labels/{name}/merchant-mappings/{pattern}", h.RemoveLabelByMerchant)
	mux.HandleFunc("GET /api/config/categories/export", h.ExportCategories)
	mux.HandleFunc("GET /api/config/categories/mappings", h.GetCategoryMappings)
	mux.HandleFunc("GET /api/config/categories", h.ListCategories)
	mux.HandleFunc("POST /api/config/categories", h.CreateCategory)
	mux.HandleFunc("DELETE /api/config/categories/{name}", h.DeleteCategory)
	mux.HandleFunc("PUT /api/config/categories/{name}/merchant-mappings/{pattern}", h.ApplyCategoryByMerchant)
	mux.HandleFunc("DELETE /api/config/categories/{name}/merchant-mappings/{pattern}", h.RemoveCategoryByMerchant)
	mux.HandleFunc("GET /api/config/buckets/export", h.ExportBuckets)
	mux.HandleFunc("GET /api/config/buckets/mappings", h.GetBucketMappings)
	mux.HandleFunc("GET /api/config/buckets", h.ListBuckets)
	mux.HandleFunc("POST /api/config/buckets", h.CreateBucket)
	mux.HandleFunc("DELETE /api/config/buckets/{name}", h.DeleteBucket)
	mux.HandleFunc("PUT /api/config/buckets/{name}/merchant-mappings/{pattern}", h.ApplyBucketByMerchant)
	mux.HandleFunc("DELETE /api/config/buckets/{name}/merchant-mappings/{pattern}", h.RemoveBucketByMerchant)
}

func registerRuleRoutes(mux *http.ServeMux, h *Handlers) {
	mux.HandleFunc("GET /api/rules", h.ListRules)
	mux.HandleFunc("GET /api/rules/export", h.ExportRules)
	mux.HandleFunc("POST /api/rules/import", h.ImportRules)
	mux.HandleFunc("POST /api/rule-drafts", h.CreateRuleDraft)
	mux.HandleFunc("POST /api/rules", h.CreateRule)
	mux.HandleFunc("PUT /api/rules/{id}", h.UpdateRule)
	mux.HandleFunc("DELETE /api/rules/{id}", h.DeleteRule)
}

func registerTransactionRoutes(mux *http.ServeMux, h *Handlers) {
	mux.HandleFunc("GET /api/transactions/facets", h.GetFacets)
	mux.HandleFunc("GET /api/transactions", h.ListTransactions)
	mux.HandleFunc("GET /api/transactions/{id}", h.GetTransaction)
	mux.HandleFunc("PATCH /api/transactions/{id}", h.UpdateTransaction)
	mux.HandleFunc("POST /api/transactions/{id}/labels", h.AddLabels)
	mux.HandleFunc("DELETE /api/transactions/{id}/labels/{label}", h.RemoveLabel)
}

func registerDiagnosticRoutes(mux *http.ServeMux, h *Handlers) {
	mux.HandleFunc("GET /api/extraction-diagnostics", h.ListExtractionDiagnostics)
	mux.HandleFunc("GET /api/extraction-diagnostics/{id}", h.GetExtractionDiagnostic)
	mux.HandleFunc("PATCH /api/extraction-diagnostics/{id}", h.UpdateExtractionDiagnosticStatus)
}

func registerMerchantRoutes(mux *http.ServeMux, h *Handlers) {
	mux.HandleFunc("GET /api/muted-merchants", h.ListMutedMerchants)
	mux.HandleFunc("POST /api/muted-merchants", h.MuteByMerchant)
	mux.HandleFunc("PATCH /api/muted-merchants/{id}", h.UpdateMerchantReason)
	mux.HandleFunc("DELETE /api/muted-merchants/{id}", h.DeleteMutedMerchant)
	mux.HandleFunc("POST /api/merchants/categorize", h.CategorizeMerchant)
}

// apiErrorFallback replaces the default ServeMux 404 and 405 bodies for API
// paths with the standard JSON error response. It probes only an unmatched
// ServeMux handler, so matched routes still own their responses.
func apiErrorFallback(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler, pattern := mux.Handler(r)
		if pattern != "" || !isAPIPath(r.URL.Path) {
			handler.ServeHTTP(w, r)
			return
		}

		probe := &muxResponseProbe{header: make(http.Header)}
		handler.ServeHTTP(probe, r)
		switch probe.status {
		case http.StatusNotFound:
			writeError(w, r, errors.E(errors.NotFound, errors.User("API endpoint not found.")))
		case http.StatusMethodNotAllowed:
			writeError(w, r, errors.E(errors.MethodNotAllowed, errors.User("Method not allowed.")))
		default:
			handler.ServeHTTP(w, r)
		}
	})
}

func isAPIPath(routePath string) bool {
	return routePath == "/api" || strings.HasPrefix(routePath, "/api/")
}

type muxResponseProbe struct {
	header http.Header
	status int
}

func (p *muxResponseProbe) Header() http.Header {
	return p.header
}

func (p *muxResponseProbe) WriteHeader(status int) {
	if p.status == 0 {
		p.status = status
	}
}

func (p *muxResponseProbe) Write(data []byte) (int, error) {
	if p.status == 0 {
		p.WriteHeader(http.StatusOK)
	}
	return len(data), nil
}
