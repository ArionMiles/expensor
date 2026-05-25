// Package api provides the HTTP server and route definitions for Expensor.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"
)

// Server wraps the HTTP server and its dependencies.
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// NewServer builds an HTTP server with all routes registered.
// Pass a non-empty staticDir to serve a bundled SPA for all non-/api paths.
// Leave empty in local dev (Vite serves the frontend separately).
func NewServer(port int, handlers *Handlers, staticDir string, logger *slog.Logger) *Server {
	mux := http.NewServeMux()
	registerRoutes(mux, handlers)
	if staticDir != "" {
		mux.HandleFunc("/", spaHandler(staticDir))
	}

	chain := corsMiddleware(loggingMiddleware(logger, recoveryMiddleware(logger, mux)))

	return &Server{
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      chain,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		logger: logger,
	}
}

// Start listens and serves until ctx is canceled.
func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("HTTP server listening", "addr", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}

// spaHandler returns an http.HandlerFunc that serves static files from dir.
// For paths that don't resolve to an existing file, it falls back to index.html
// to support client-side SPA routing (React Router, etc.).
func spaHandler(dir string) http.HandlerFunc {
	fs := http.FileServer(http.Dir(dir))
	return func(w http.ResponseWriter, r *http.Request) {
		// path.Clean normalises the URL path and removes traversal sequences.
		// filepath.Join with a cleaned path starting with "/" is safe in Go:
		// Join never treats intermediate absolute components as new roots.
		upath := path.Clean("/" + r.URL.Path)
		fsPath := filepath.Join(dir, filepath.FromSlash(upath))
		if _, err := os.Stat(fsPath); os.IsNotExist(err) {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	}
}

// registerRoutes attaches all API routes to mux.
func registerRoutes(mux *http.ServeMux, h *Handlers) {
	// Health, status & version
	mux.HandleFunc("GET /api/health", h.Health)
	mux.HandleFunc("GET /api/status", h.Status)
	mux.HandleFunc("GET /api/version", h.Version)
	mux.HandleFunc("POST /api/daemon/start", h.StartDaemon)
	mux.HandleFunc("POST /api/daemon/rescan", h.Rescan)
	mux.HandleFunc("GET /api/config/active-reader", h.GetActiveReader)

	// Plugin listing
	mux.HandleFunc("GET /api/plugins/readers", h.ListReaders)
	mux.HandleFunc("GET /api/plugins/writers", h.ListWriters)

	// Thunderbird profile/mailbox discovery (must precede wildcard /api/readers/{name}/... routes)
	mux.HandleFunc("GET /api/readers/thunderbird/discover/profiles", h.DiscoverProfiles)
	mux.HandleFunc("GET /api/readers/thunderbird/discover/mailboxes", h.DiscoverMailboxes)

	// Reader setup guide (must precede wildcard /api/readers/{name}/... routes)
	mux.HandleFunc("GET /api/readers/{name}/guide", h.GetReaderGuide)

	// Reader credentials (OAuth readers that need client_secret.json upload)
	mux.HandleFunc("POST /api/readers/{name}/credentials", h.UploadCredentials)
	mux.HandleFunc("GET /api/readers/{name}/credentials/status", h.CredentialsStatus)

	// OAuth flow
	mux.HandleFunc("POST /api/readers/{name}/auth/start", h.AuthStart)
	mux.HandleFunc("GET /api/auth/callback", h.AuthCallback)                 // shared redirect URI
	mux.HandleFunc("POST /api/readers/{name}/auth/exchange", h.AuthExchange) // manual paste flow for homeservers
	mux.HandleFunc("GET /api/readers/{name}/auth/status", h.AuthStatus)
	mux.HandleFunc("DELETE /api/readers/{name}/auth/token", h.RevokeToken)

	// Reader config (config-only readers like Thunderbird, plus optional settings for OAuth readers)
	mux.HandleFunc("GET /api/readers/{name}/config", h.GetReaderConfig)
	mux.HandleFunc("POST /api/readers/{name}/config", h.SaveReaderConfig)

	// Reader overall readiness
	mux.HandleFunc("GET /api/readers/{name}/status", h.ReaderStatus)

	// Full reader disconnect (removes all credentials/token/config files)
	mux.HandleFunc("DELETE /api/readers/{name}", h.DisconnectReader)

	// Chart data
	mux.HandleFunc("GET /api/stats/dashboard", h.GetDashboardData)
	mux.HandleFunc("GET /api/stats/charts", h.GetChartData)
	mux.HandleFunc("GET /api/stats/labels/monthly", h.GetLabelMonthlySpend)
	mux.HandleFunc("GET /api/stats/heatmap", h.GetHeatmap)
	mux.HandleFunc("GET /api/stats/heatmap/annual", h.GetAnnualHeatmap)

	// App configuration
	mux.HandleFunc("GET /api/config/banks", h.ListBanks)
	mux.HandleFunc("GET /api/config/setup-status", h.GetSetupStatus)
	mux.HandleFunc("POST /api/config/sync", h.TriggerSync)
	mux.HandleFunc("GET /api/config/sync/status", h.GetSyncStatus)
	mux.HandleFunc("GET /api/config/base-currency", h.GetBaseCurrency)
	mux.HandleFunc("PUT /api/config/base-currency", h.SetBaseCurrency)
	mux.HandleFunc("GET /api/config/scan-interval", h.GetScanInterval)
	mux.HandleFunc("PUT /api/config/scan-interval", h.SetScanInterval)
	mux.HandleFunc("GET /api/config/lookback-days", h.GetLookbackDays)
	mux.HandleFunc("PUT /api/config/lookback-days", h.SetLookbackDays)
	mux.HandleFunc("GET /api/config/timezone", h.GetTimezone)
	mux.HandleFunc("PUT /api/config/timezone", h.SetTimezone)
	mux.HandleFunc("GET /api/config/time-format", h.GetTimeFormat)
	mux.HandleFunc("PUT /api/config/time-format", h.SetTimeFormat)
	mux.HandleFunc("GET /api/config/readers/{name}/checkpoint", h.GetReaderCheckpoint)
	mux.HandleFunc("DELETE /api/config/readers/{name}/checkpoint", h.ClearReaderCheckpoint)

	// Labels taxonomy
	mux.HandleFunc("GET /api/config/labels/export", h.ExportLabels)       // must precede /{name}
	mux.HandleFunc("GET /api/config/labels/mappings", h.GetLabelMappings) // must precede /{name}
	mux.HandleFunc("GET /api/config/labels", h.ListLabels)
	mux.HandleFunc("POST /api/config/labels", h.CreateLabel)
	mux.HandleFunc("PUT /api/config/labels/{name}", h.UpdateLabel)
	mux.HandleFunc("DELETE /api/config/labels/{name}", h.DeleteLabel)
	mux.HandleFunc("POST /api/config/labels/{name}/apply", h.ApplyLabel)
	mux.HandleFunc("DELETE /api/config/labels/{name}/merchant", h.RemoveLabelByMerchant)

	// Categories and buckets
	mux.HandleFunc("GET /api/config/categories/export", h.ExportCategories)      // must precede /{name}
	mux.HandleFunc("GET /api/config/categories/mappings", h.GetCategoryMappings) // must precede /{name}
	mux.HandleFunc("GET /api/config/categories", h.ListCategories)
	mux.HandleFunc("POST /api/config/categories", h.CreateCategory)
	mux.HandleFunc("DELETE /api/config/categories/{name}", h.DeleteCategory)
	mux.HandleFunc("POST /api/config/categories/{name}/apply", h.ApplyCategoryByMerchant)
	mux.HandleFunc("DELETE /api/config/categories/{name}/merchant", h.RemoveCategoryByMerchant)
	mux.HandleFunc("GET /api/config/buckets/export", h.ExportBuckets)       // must precede /{name}
	mux.HandleFunc("GET /api/config/buckets/mappings", h.GetBucketMappings) // must precede /{name}
	mux.HandleFunc("GET /api/config/buckets", h.ListBuckets)
	mux.HandleFunc("POST /api/config/buckets", h.CreateBucket)
	mux.HandleFunc("DELETE /api/config/buckets/{name}", h.DeleteBucket)
	mux.HandleFunc("POST /api/config/buckets/{name}/apply", h.ApplyBucketByMerchant)
	mux.HandleFunc("DELETE /api/config/buckets/{name}/merchant", h.RemoveBucketByMerchant)

	// Rules — export and import before /{id} to avoid wildcard capture
	mux.HandleFunc("GET /api/rules", h.ListRules)
	mux.HandleFunc("GET /api/rules/export", h.ExportRules)
	mux.HandleFunc("POST /api/rules/import", h.ImportRules)
	mux.HandleFunc("POST /api/rules", h.CreateRule)
	mux.HandleFunc("PUT /api/rules/{id}", h.UpdateRule)
	mux.HandleFunc("DELETE /api/rules/{id}", h.DeleteRule)

	// Transactions
	// /search and /facets must be registered before /{id} to avoid the wildcard swallowing them.
	mux.HandleFunc("GET /api/transactions/search", h.SearchTransactions)
	mux.HandleFunc("GET /api/transactions/facets", h.GetFacets)
	mux.HandleFunc("GET /api/transactions", h.ListTransactions)
	mux.HandleFunc("GET /api/transactions/{id}", h.GetTransaction)
	mux.HandleFunc("PUT /api/transactions/{id}", h.UpdateTransaction)
	mux.HandleFunc("POST /api/transactions/{id}/labels", h.AddLabels)
	mux.HandleFunc("DELETE /api/transactions/{id}/labels/{label}", h.RemoveLabel)
	mux.HandleFunc("PUT /api/transactions/{id}/mute", h.MuteTransaction)
	mux.HandleFunc("PUT /api/transactions/{id}/mute-reason", h.UpdateMuteReason)

	// Extraction diagnostics
	mux.HandleFunc("GET /api/extraction-diagnostics", h.ListExtractionDiagnostics)
	mux.HandleFunc("GET /api/extraction-diagnostics/{id}", h.GetExtractionDiagnostic)
	mux.HandleFunc("PUT /api/extraction-diagnostics/{id}/status", h.UpdateExtractionDiagnosticStatus)

	// Muted merchants
	mux.HandleFunc("GET /api/muted-merchants", h.ListMutedMerchants)
	mux.HandleFunc("POST /api/muted-merchants", h.MuteByMerchant)
	mux.HandleFunc("PUT /api/muted-merchants/{id}/reason", h.UpdateMerchantReason)
	mux.HandleFunc("DELETE /api/muted-merchants/{id}", h.DeleteMutedMerchant)

	// Merchant-wide categorization
	mux.HandleFunc("POST /api/merchants/categorize", h.CategorizeMerchant)
}
