// Package api provides the HTTP server and route definitions for Expensor.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Server wraps the HTTP server and its dependencies.
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// NewServer builds an HTTP server with all routes registered.
func NewServer(port int, handlers *Handlers, logger *slog.Logger) *Server {
	mux := http.NewServeMux()
	registerRoutes(mux, handlers)

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

// registerRoutes attaches all API routes to mux.
func registerRoutes(mux *http.ServeMux, h *Handlers) {
	// Health & status
	mux.HandleFunc("GET /api/health", h.HandleHealth)
	mux.HandleFunc("GET /api/status", h.HandleStatus)
	mux.HandleFunc("POST /api/daemon/start", h.HandleStartDaemon)
	mux.HandleFunc("POST /api/daemon/rescan", h.HandleRescan)
	mux.HandleFunc("GET /api/config/active-reader", h.HandleGetActiveReader)

	// Plugin listing
	mux.HandleFunc("GET /api/plugins/readers", h.HandleListReaders)
	mux.HandleFunc("GET /api/plugins/writers", h.HandleListWriters)

	// Reader credentials (OAuth readers that need client_secret.json upload)
	mux.HandleFunc("POST /api/readers/{name}/credentials", h.HandleUploadCredentials)
	mux.HandleFunc("GET /api/readers/{name}/credentials/status", h.HandleCredentialsStatus)

	// OAuth flow
	mux.HandleFunc("POST /api/readers/{name}/auth/start", h.HandleAuthStart)
	mux.HandleFunc("GET /api/auth/callback", h.HandleAuthCallback) // shared redirect URI
	mux.HandleFunc("GET /api/readers/{name}/auth/status", h.HandleAuthStatus)
	mux.HandleFunc("DELETE /api/readers/{name}/auth/token", h.HandleRevokeToken)

	// Reader config (config-only readers like Thunderbird, plus optional settings for OAuth readers)
	mux.HandleFunc("GET /api/readers/{name}/config", h.HandleGetReaderConfig)
	mux.HandleFunc("POST /api/readers/{name}/config", h.HandleSaveReaderConfig)

	// Reader overall readiness
	mux.HandleFunc("GET /api/readers/{name}/status", h.HandleReaderStatus)

	// Full reader disconnect (removes all credentials/token/config files)
	mux.HandleFunc("DELETE /api/readers/{name}", h.HandleDisconnectReader)

	// Chart data
	mux.HandleFunc("GET /api/stats/charts", h.HandleGetChartData)
	mux.HandleFunc("GET /api/stats/heatmap", h.HandleGetHeatmap)
	mux.HandleFunc("GET /api/stats/heatmap/annual", h.HandleGetAnnualHeatmap)

	// App configuration
	mux.HandleFunc("GET /api/config/base-currency", h.HandleGetBaseCurrency)
	mux.HandleFunc("PUT /api/config/base-currency", h.HandleSetBaseCurrency)
	mux.HandleFunc("GET /api/config/scan-interval", h.HandleGetScanInterval)
	mux.HandleFunc("PUT /api/config/scan-interval", h.HandleSetScanInterval)
	mux.HandleFunc("GET /api/config/lookback-days", h.HandleGetLookbackDays)
	mux.HandleFunc("PUT /api/config/lookback-days", h.HandleSetLookbackDays)

	// Labels taxonomy
	mux.HandleFunc("GET /api/config/labels", h.HandleListLabels)
	mux.HandleFunc("POST /api/config/labels", h.HandleCreateLabel)
	mux.HandleFunc("PUT /api/config/labels/{name}", h.HandleUpdateLabel)
	mux.HandleFunc("DELETE /api/config/labels/{name}", h.HandleDeleteLabel)
	mux.HandleFunc("POST /api/config/labels/{name}/apply", h.HandleApplyLabel)

	// Categories and buckets
	mux.HandleFunc("GET /api/config/categories", h.HandleListCategories)
	mux.HandleFunc("POST /api/config/categories", h.HandleCreateCategory)
	mux.HandleFunc("DELETE /api/config/categories/{name}", h.HandleDeleteCategory)
	mux.HandleFunc("GET /api/config/buckets", h.HandleListBuckets)
	mux.HandleFunc("POST /api/config/buckets", h.HandleCreateBucket)
	mux.HandleFunc("DELETE /api/config/buckets/{name}", h.HandleDeleteBucket)

	// Rules — export and import before /{id} to avoid wildcard capture
	mux.HandleFunc("GET /api/rules", h.HandleListRules)
	mux.HandleFunc("GET /api/rules/export", h.HandleExportRules)
	mux.HandleFunc("POST /api/rules/import", h.HandleImportRules)
	mux.HandleFunc("POST /api/rules", h.HandleCreateRule)
	mux.HandleFunc("PUT /api/rules/{id}", h.HandleUpdateRule)
	mux.HandleFunc("DELETE /api/rules/{id}", h.HandleDeleteRule)

	// Transactions
	// /search and /facets must be registered before /{id} to avoid the wildcard swallowing them.
	mux.HandleFunc("GET /api/transactions/search", h.HandleSearchTransactions)
	mux.HandleFunc("GET /api/transactions/facets", h.HandleGetFacets)
	mux.HandleFunc("GET /api/transactions", h.HandleListTransactions)
	mux.HandleFunc("GET /api/transactions/{id}", h.HandleGetTransaction)
	mux.HandleFunc("PUT /api/transactions/{id}", h.HandleUpdateTransaction)
	mux.HandleFunc("POST /api/transactions/{id}/labels", h.HandleAddLabels)
	mux.HandleFunc("DELETE /api/transactions/{id}/labels/{label}", h.HandleRemoveLabel)
}
