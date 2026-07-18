// Package httpapi provides Expensor's HTTP server, routes, handlers, and transport types.
package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/observability"
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

	scope := observability.NewScope(logger, "github.com/ArionMiles/expensor/backend/internal/httpapi")
	protectedMux := authMiddleware(handlers, apiErrorFallback(mux))
	chain := requestIDMiddleware(corsMiddleware(observabilityMiddleware(scope, loggingMiddleware(logger, recoveryMiddleware(logger, protectedMux)))))

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
		// path.Clean normalizes the URL path and removes traversal sequences.
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
