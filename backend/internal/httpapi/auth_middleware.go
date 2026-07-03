package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/auth"
	"github.com/ArionMiles/expensor/backend/internal/store"
)

func authMiddleware(h *Handlers, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicRoute(r) {
			next.ServeHTTP(w, r)
			return
		}

		principal, ok := h.authenticateRequest(w, r)
		if !ok {
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), principal)))
	})
}

func isPublicRoute(r *http.Request) bool {
	if r.Method == http.MethodOptions || !strings.HasPrefix(r.URL.Path, "/api/") {
		return true
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/health":
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/version":
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/bootstrap":
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/bootstrap":
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/session":
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/account-setup":
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/account-setup":
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/auth/callback":
		return true
	default:
		return false
	}
}

func (h *Handlers) authenticateRequest(w http.ResponseWriter, r *http.Request) (auth.Principal, bool) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		return h.authenticateSession(w, r, cookie.Value)
	}
	if token, ok := bearerToken(r.Header.Get("Authorization")); ok {
		return h.authenticateBearer(w, r, token)
	}
	writeError(w, http.StatusUnauthorized, "authentication required")
	return auth.Principal{}, false
}

func (h *Handlers) authenticateSession(w http.ResponseWriter, r *http.Request, raw string) (auth.Principal, bool) {
	session, err := h.authStore.FindSessionByHash(r.Context(), auth.HashOpaqueToken(raw))
	if err != nil || session.RevokedAt != nil || !session.ExpiresAt.After(time.Now()) {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return auth.Principal{}, false
	}
	user, ok := h.authenticatedUser(w, r, session.UserID)
	if !ok {
		return auth.Principal{}, false
	}
	return principalForUser(user, "session"), true
}

func (h *Handlers) authenticateBearer(w http.ResponseWriter, r *http.Request, raw string) (auth.Principal, bool) {
	token, err := h.authStore.FindAccessTokenByHash(r.Context(), auth.HashOpaqueToken(raw))
	if err != nil || token.RevokedAt != nil || (token.ExpiresAt != nil && !token.ExpiresAt.After(time.Now())) {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return auth.Principal{}, false
	}
	user, ok := h.authenticatedUser(w, r, token.UserID)
	if !ok {
		return auth.Principal{}, false
	}
	return principalForUser(user, "bearer"), true
}

func (h *Handlers) authenticatedUser(w http.ResponseWriter, r *http.Request, userID string) (*store.User, bool) {
	user, err := h.authStore.FindUserByID(r.Context(), userID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			h.logger.Error("authenticate user", "error", err)
		}
		writeError(w, http.StatusUnauthorized, "authentication required")
		return nil, false
	}
	if user.DisabledAt != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return nil, false
	}
	return user, true
}

func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(strings.TrimSpace(header), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}

func principalForUser(user *store.User, method string) auth.Principal {
	return auth.Principal{
		UserID:     user.ID,
		TenantID:   user.TenantID,
		Role:       auth.Role(user.Role),
		AuthMethod: method,
	}
}
