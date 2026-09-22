// Package httpapi provides Expensor's HTTP server, routes, handlers, and transport types.
package httpapi

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"
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
	if ui := uiFileSystem(staticDir, embeddedUIFileSystem()); ui != nil {
		mux.HandleFunc("/", spaHandler(ui))
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

// uiFileSystem selects a disk override when configured, then the bundled UI.
func uiFileSystem(staticDir string, embedded fs.FS) fs.FS {
	if staticDir != "" {
		return os.DirFS(staticDir)
	}
	return embedded
}

// spaHandler returns an http.HandlerFunc that serves static files from files.
// For paths that don't resolve to an existing file, it falls back to index.html
// to support client-side SPA routing (React Router, etc.).
func spaHandler(files fs.FS) http.HandlerFunc {
	fileServer := http.FileServer(http.FS(files))
	return func(w http.ResponseWriter, r *http.Request) {
		upath := path.Clean("/" + r.URL.Path)
		name := strings.TrimPrefix(upath, "/")
		if name == "" {
			name = "."
		}
		if _, err := fs.Stat(files, name); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(upath, "/assets/") || path.Ext(upath) != "" {
			http.NotFound(w, r)
			return
		}

		fallback := r.Clone(r.Context())
		fallback.URL.Path = "/"
		fileServer.ServeHTTP(w, fallback)
	}
}
