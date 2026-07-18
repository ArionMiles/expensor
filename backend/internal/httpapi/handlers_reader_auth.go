package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"

	"github.com/ArionMiles/expensor/backend/internal/daemon"
	"github.com/ArionMiles/expensor/backend/internal/oauth"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

// --- provider credentials upload ---

func entryTenant(entry oauthStateEntry, fallback store.Tenant) (store.Tenant, bool) {
	if entry.tenant.ID != "" {
		return entry.tenant, fallback.ID == "" || fallback.ID == entry.tenant.ID
	}
	return fallback, true
}

// UploadCredentials handles POST /api/providers/{name}/credentials.
// Accepts a JSON file upload (e.g. Google client_secret.json) and saves it to the runtime store.
// @Summary Upload reader OAuth credentials
// @Tags Providers
// @Accept json
// @Produce json
// @Param name path string true "Provider name" example(gmail)
// @Param request body object true "OAuth client credentials JSON"
// @Success 200 {object} UploadCredentialsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 413 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /providers/{name}/credentials [post]
func (h *Handlers) UploadCredentials(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	provider, err := h.registry.GetProvider(name)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if !provider.Metadata.Auth.RequiresCredentialsUpload {
		writeError(w, r, errors.E(errors.InvalidArgument, errors.User(fmt.Sprintf("provider %q does not require credentials upload", name))))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCredentialsSize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, r, errors.E(errors.PayloadTooLarge, errors.User("file too large (max 5 MB)"), err))
		} else {
			writeError(w, r, err)
		}
		return
	}

	// Validate it is valid OAuth2 client credentials JSON.
	// Google client_secret.json must have a "web" or "installed" top-level key.
	var creds struct {
		Web       json.RawMessage `json:"web"`
		Installed json.RawMessage `json:"installed"`
	}
	if err := json.Unmarshal(body, &creds); err != nil {
		writeError(w, r, errors.E(errors.InvalidInput, errors.User("file is not valid JSON"), err))
		return
	}
	if creds.Web == nil && creds.Installed == nil {
		writeError(w, r, errors.E(
			errors.InvalidInput,
			errors.User(`invalid credentials file: expected a Google OAuth2 client_secret.json with a "web" or "installed"`+
				` top-level key — download it from Google Cloud Console → APIs & Services → Credentials → OAuth 2.0 Client IDs`),
		))
		return
	}

	if err := h.readerRuntimeStore.SetReaderSecret(r.Context(), requestTenant(r), name, body); err != nil {
		writeError(w, r, err)
		return
	}

	h.logger.Info("credentials uploaded", "reader", name)
	writeJSON(w, http.StatusOK, map[string]string{"path": fmt.Sprintf("db://reader_runtime/%s/client_secret", name)})
}

// CredentialsStatus handles GET /api/providers/{name}/credentials/status.
// @Summary Get provider credentials status
// @Tags Providers
// @Produce json
// @Param name path string true "Provider name" example(gmail)
// @Success 200 {object} CredentialsStatusResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /providers/{name}/credentials/status [get]
func (h *Handlers) CredentialsStatus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.registry.GetProvider(name); err != nil {
		writeError(w, r, err)
		return
	}

	_, exists, err := h.readerRuntimeStore.GetReaderSecret(r.Context(), requestTenant(r), name)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"exists": exists})
}

// --- OAuth flow ---

const (
	authStateConnected               = "connected"
	authStateReauthorizationRequired = "reauthorization_required"
	authStateRefreshPending          = "refresh_pending"
)

type oauthTokenState struct {
	authenticated bool
	authState     string
	expiry        *time.Time
}

// AuthStart handles POST /api/providers/{name}/auth/start.
// Returns a Google OAuth consent URL for the given reader.
// @Summary Start reader OAuth authorization
// @Tags Providers
// @Produce json
// @Param name path string true "Provider name" example(gmail)
// @Success 200 {object} AuthStartResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 412 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /providers/{name}/auth/start [post]
func (h *Handlers) AuthStart(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	provider, err := h.registry.GetProvider(name)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if provider.Metadata.Auth.Type != plugins.AuthTypeOAuth {
		writeError(w, r, errors.E(errors.InvalidArgument, errors.User(fmt.Sprintf("provider %q does not use OAuth", name))))
		return
	}

	tenant := requestTenant(r)
	h.logger.Debug("reading credentials from store", "reader", name)
	secretJSON, ok, err := h.readerRuntimeStore.GetReaderSecret(r.Context(), tenant, name)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if !ok {
		writeError(w, r, errors.E(errors.FailedPrecondition, errors.User("credentials not uploaded — upload client credentials first")))
		return
	}

	redirectURL := h.baseURL + "/api/auth/callback"
	scopes := provider.Metadata.Auth.RequiredScopes
	h.logger.Debug("building OAuth config", "reader", name, "redirect_url", redirectURL, "scopes", scopes)
	oauthCfg, err := oauth.GetOAuthConfig(secretJSON, redirectURL, scopes...)
	if err != nil {
		writeError(w, r, errors.E(errors.User("Provider credentials could not be parsed."), err))
		return
	}

	// Generate a random state token that encodes the reader name.
	state, err := generateState(name)
	if err != nil {
		writeError(w, r, err)
		return
	}
	h.mu.Lock()
	// Prune expired entries before adding a new one.
	for k, v := range h.oauthStates {
		if time.Now().After(v.expiresAt) {
			delete(h.oauthStates, k)
		}
	}
	h.oauthStates[state] = oauthStateEntry{
		readerName: name,
		tenant:     tenant,
		expiresAt:  time.Now().Add(oauthStateTTL),
	}
	h.mu.Unlock()

	// prompt=consent forces Google to always return a refresh token, even if the
	// user has previously authorized this app. Without it, re-authorizations only
	// return an access token, which cannot be refreshed after expiry.
	authURL := oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
	h.logger.Debug("OAuth URL generated", "reader", name, "state", state)
	writeJSON(w, http.StatusOK, map[string]string{
		"url":          authURL,
		"redirect_uri": redirectURL,
	})
}

// exchangeAndSaveToken loads credentials for the named reader, exchanges the
// authorization code for a token, and persists it to the runtime store. The
// redirectURL must match the one used when building the authorization URL.
func (h *Handlers) exchangeAndSaveToken(ctx context.Context, tenant store.Tenant, name, code, redirectURL string) error {
	const op = "httpapi.Handlers.exchangeAndSaveToken"

	provider, err := h.registry.GetProvider(name)
	if err != nil {
		return errors.E(op, errors.NotFound, fmt.Sprintf("reader %q is no longer registered", name), err)
	}

	secretJSON, ok, err := h.readerRuntimeStore.GetReaderSecret(ctx, tenant, name)
	if err != nil {
		return errors.E("httpapi.handlers_readers.exchange_and_save_token", "failed to load credentials", err)
	}
	if !ok {
		return errors.E(
			op,
			oauth.KindCredentialsMissing,
			errors.User("credentials not uploaded — upload client credentials first"),
			"credentials file missing",
		)
	}

	oauthCfg, err := oauth.GetOAuthConfig(secretJSON, redirectURL, provider.Metadata.Auth.RequiredScopes...)
	if err != nil {
		return errors.E("httpapi.handlers_readers.exchange_and_save_token", "failed to parse credentials", err)
	}

	tok, err := oauthCfg.Exchange(ctx, code)
	if err != nil {
		return errors.E("httpapi.handlers_readers.exchange_and_save_token", "token exchange failed", err)
	}

	tokenJSON, err := json.Marshal(tok) //nolint:gosec // OAuth tokens are intentionally serialized into the runtime store.
	if err != nil {
		return errors.E("httpapi.handlers_readers.exchange_and_save_token", "failed to marshal token", err)
	}
	if err := h.readerRuntimeStore.SetReaderToken(ctx, tenant, name, tokenJSON); err != nil {
		return errors.E("httpapi.handlers_readers.exchange_and_save_token", "failed to save token", err)
	}
	h.queueReaderScanning(ctx, tenant, name)
	h.restartReaderDaemonAfterAuth(tenant, name)
	return nil
}

func (h *Handlers) restartReaderDaemonAfterAuth(tenant store.Tenant, name string) {
	if h.daemon != nil && h.daemon.Status().Running {
		h.daemon.Restart(daemon.RunRequest{Tenant: tenant, Reader: name})
	}
}

// AuthCallback handles GET /api/auth/callback.
// This is the shared OAuth redirect target for all readers.
// @Summary Handle reader OAuth callback
// @Tags Providers
// @Produce html
// @Param state query string true "OAuth state"
// @Param code query string true "OAuth authorization code"
// @Success 200 "OK"
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /auth/callback [get]
func (h *Handlers) AuthCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	h.mu.Lock()
	entry, ok := h.oauthStates[state]
	if ok {
		delete(h.oauthStates, state)
	}
	h.mu.Unlock()

	name := entry.readerName
	h.logger.Debug("OAuth callback received", "state_valid", ok, "reader", name, "has_code", code != "")
	if !ok || time.Now().After(entry.expiresAt) {
		writeError(w, r, errors.E(errors.InvalidArgument, errors.User("invalid or expired OAuth state")))
		return
	}

	redirectURL := h.baseURL + "/api/auth/callback"
	tenant, tenantOK := entryTenant(entry, requestTenant(r))
	if !tenantOK {
		writeError(w, r, errors.E(errors.InvalidArgument, errors.User("invalid or expired OAuth state")))
		return
	}
	if err := h.exchangeAndSaveToken(r.Context(), tenant, name, code, redirectURL); err != nil {
		writeError(w, r, err)
		return
	}

	h.logger.Info("OAuth token saved", "reader", name)
	h.writeOAuthClosePage(w, name)
}

func (h *Handlers) writeOAuthClosePage(w http.ResponseWriter, name string) {
	setupURL := h.frontendURL + "/setup?auth=success&reader=" + url.QueryEscape(name)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="en" class="closing">
<head>
  <meta charset="utf-8">
  <title>Expensor authorization complete</title>
  <script>
    window.close();
    setTimeout(function () { document.documentElement.classList.remove('closing'); }, 300);
  </script>
  <style>
    html.closing body { display: none; }
    body { color: #0f172a; font-family: system-ui, sans-serif; margin: 2rem; }
    a { color: #2563eb; }
  </style>
</head>
<body>
  <p>Authorization complete. You can close this tab and return to Expensor.</p>
  <p><a href="%s">Return to Expensor</a></p>
</body>
</html>`, html.EscapeString(setupURL))
}

// AuthExchange handles POST /api/providers/{name}/auth/exchange.
// Accepts the full redirect URL (containing code and state params) pasted by
// the user when the automatic redirect is unreachable (e.g. homeserver without
// a public domain). Validates state, exchanges the code, and saves the token.
// @Summary Exchange a pasted reader OAuth callback URL
// @Tags Providers
// @Accept json
// @Produce json
// @Param name path string true "Provider name" example(gmail)
// @Param request body AuthExchangeRequest true "OAuth callback URL payload"
// @Success 200 {object} AuthExchangeResponse
// @Failure 400 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /providers/{name}/auth/exchange [post]
func (h *Handlers) AuthExchange(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	body, ok := decodeAndValidateJSON[AuthExchangeRequest](h, w, r)
	if !ok {
		return
	}

	parsed, err := url.Parse(body.URL)
	if err != nil {
		writeError(w, r, errors.E(errors.InvalidArgument, errors.User("Invalid URL."), err))
		return
	}

	code := parsed.Query().Get("code")
	state := parsed.Query().Get("state")

	if code == "" {
		writeError(w, r, errors.E(errors.InvalidArgument, errors.User("url is missing the \"code\" query parameter")))
		return
	}
	if state == "" {
		writeError(w, r, errors.E(errors.InvalidArgument, errors.User("url is missing the \"state\" query parameter")))
		return
	}

	h.mu.Lock()
	entry, ok := h.oauthStates[state]
	if ok {
		delete(h.oauthStates, state)
	}
	h.mu.Unlock()

	if !ok || time.Now().After(entry.expiresAt) {
		writeError(w, r, errors.E(errors.InvalidArgument, errors.User("invalid or expired OAuth state")))
		return
	}

	redirectURL := h.baseURL + "/api/auth/callback"
	tenant, tenantOK := entryTenant(entry, requestTenant(r))
	if !tenantOK {
		writeError(w, r, errors.E(errors.InvalidArgument, errors.User("invalid or expired OAuth state")))
		return
	}
	if err := h.exchangeAndSaveToken(r.Context(), tenant, name, code, redirectURL); err != nil {
		writeError(w, r, err)
		return
	}

	h.logger.Info("OAuth token saved via manual exchange", "reader", name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "authorized"})
}

// AuthStatus handles GET /api/providers/{name}/auth/status.
// @Summary Get provider auth status
// @Tags Providers
// @Produce json
// @Param name path string true "Provider name" example(gmail)
// @Success 200 {object} AuthStatusResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /providers/{name}/auth/status [get]
func (h *Handlers) AuthStatus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	provider, err := h.registry.GetProvider(name)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if provider.Metadata.Auth.Type != plugins.AuthTypeOAuth {
		// Config-only readers are always "authenticated" once configured.
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": true,
			"auth_type":     plugins.AuthTypeConfig,
			"auth_state":    authStateConnected,
		})
		return
	}

	tokenJSON, ok, err := h.readerRuntimeStore.GetReaderToken(r.Context(), requestTenant(r), name)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": false,
			"auth_state":    authStateReauthorizationRequired,
		})
		return
	}

	tokenState, err := h.resolveOAuthTokenState(r.Context(), requestTenant(r), name, provider.Metadata.Auth.RequiredScopes, tokenJSON)
	if err != nil {
		writeError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": tokenState.authenticated,
		"auth_state":    tokenState.authState,
		"expiry":        tokenState.expiry,
	})
}

func (h *Handlers) resolveOAuthTokenState(ctx context.Context, tenant store.Tenant, name string, scopes []string, tokenJSON []byte) (oauthTokenState, error) {
	var tok oauth2.Token
	if err := json.Unmarshal(tokenJSON, &tok); err != nil {
		h.logger.Warn("failed to parse token", "reader", name, "error", err)
		return oauthTokenState{authState: authStateReauthorizationRequired}, nil
	}
	expiry := tokenExpiry(tok)
	if tok.Valid() {
		return oauthTokenState{authenticated: true, authState: authStateConnected, expiry: expiry}, nil
	}
	if tok.RefreshToken == "" {
		return oauthTokenState{authState: authStateReauthorizationRequired, expiry: expiry}, nil
	}

	secretJSON, ok, err := h.readerRuntimeStore.GetReaderSecret(ctx, tenant, name)
	if err != nil {
		return oauthTokenState{authState: authStateRefreshPending, expiry: expiry}, errors.E(
			"httpapi.handlers_readers.resolve_o_auth_token_state", "loading credentials for token refresh", err,
		)
	}
	if !ok {
		return oauthTokenState{authState: authStateReauthorizationRequired, expiry: expiry}, nil
	}
	oauthCfg, err := oauth.GetOAuthConfig(secretJSON, h.baseURL+"/api/auth/callback", scopes...)
	if err != nil {
		return oauthTokenState{authState: authStateReauthorizationRequired, expiry: expiry}, errors.E(
			"httpapi.handlers_readers.resolve_o_auth_token_state", "parsing credentials for token refresh", err,
		)
	}
	refreshed, err := oauthCfg.TokenSource(ctx, &tok).Token()
	if err != nil {
		if isInvalidOAuthGrant(err) {
			return oauthTokenState{authState: authStateReauthorizationRequired, expiry: expiry}, nil
		}
		h.logger.Warn("OAuth token refresh pending", "reader", name)
		return oauthTokenState{authState: authStateRefreshPending, expiry: expiry}, nil
	}
	refreshedJSON, err := json.Marshal(refreshed) //nolint:gosec // OAuth tokens are intentionally serialized into the runtime store.
	if err != nil {
		return oauthTokenState{authState: authStateRefreshPending, expiry: expiry}, errors.E(
			"httpapi.handlers_readers.resolve_o_auth_token_state", "marshaling refreshed token", err,
		)
	}
	if err := h.readerRuntimeStore.SetReaderToken(ctx, tenant, name, refreshedJSON); err != nil {
		return oauthTokenState{authState: authStateRefreshPending, expiry: expiry}, errors.E(
			"httpapi.handlers_readers.resolve_o_auth_token_state", "saving refreshed token", err,
		)
	}
	return oauthTokenState{authenticated: true, authState: authStateConnected, expiry: tokenExpiry(*refreshed)}, nil
}

func tokenExpiry(tok oauth2.Token) *time.Time {
	if tok.Expiry.IsZero() {
		return nil
	}
	expiry := tok.Expiry
	return &expiry
}

func isInvalidOAuthGrant(err error) bool {
	var retrieveErr *oauth2.RetrieveError
	return errors.As(err, &retrieveErr) && retrieveErr.ErrorCode == "invalid_grant"
}

func (h *Handlers) resolveReaderAuthStatus(ctx context.Context, tenant store.Tenant, name string, meta plugins.ProviderMetadata) (bool, string) {
	if meta.Auth.Type != plugins.AuthTypeOAuth {
		return true, authStateConnected
	}
	tokenJSON, ok, err := h.readerRuntimeStore.GetReaderToken(ctx, tenant, name)
	if err != nil || !ok {
		return false, authStateReauthorizationRequired
	}
	tokenState, err := h.resolveOAuthTokenState(ctx, tenant, name, meta.Auth.RequiredScopes, tokenJSON)
	if err != nil {
		return false, authStateRefreshPending
	}
	return tokenState.authenticated, tokenState.authState
}

// DisconnectReader handles DELETE /api/providers/{name}.
// Removes all stored credentials, token, and config files for the reader.
// @Summary Disconnect a reader
// @Tags Providers
// @Produce json
// @Param name path string true "Provider name" Enums(thunderbird,gmail) example(thunderbird)
// @Success 200 {object} ProviderDisconnectResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /providers/{name} [delete]
func (h *Handlers) DisconnectReader(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.registry.GetProvider(name); err != nil {
		writeError(w, r, err)
		return
	}

	tenant := requestTenant(r)
	state, err := h.scanningStore.GetScanningState(r.Context(), tenant)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if err := h.readerRuntimeStore.DeleteReaderRuntime(r.Context(), tenant, name); err != nil {
		writeError(w, r, err)
		return
	}
	if state.ActiveReader == name {
		if err := h.scanningStore.ClearActiveScanningReader(r.Context(), tenant); err != nil {
			writeError(w, r, err)
			return
		}
		if h.daemon != nil && h.daemon.Status().Running {
			h.daemon.Stop()
		}
	}

	h.logger.Info("provider disconnected", "reader", name)
	writeJSON(w, http.StatusOK, map[string]any{"status": "disconnected", "files_removed": []string{}})
}

// RevokeToken handles DELETE /api/providers/{name}/auth/token.
// @Summary Revoke a reader OAuth token
// @Tags Providers
// @Produce json
// @Param name path string true "Provider name" example(gmail)
// @Success 200 {object} ProviderTokenRevokeResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /providers/{name}/auth/token [delete]
func (h *Handlers) RevokeToken(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.registry.GetProvider(name); err != nil {
		writeError(w, r, err)
		return
	}

	if _, ok, err := h.readerRuntimeStore.GetReaderToken(r.Context(), requestTenant(r), name); err != nil {
		writeError(w, r, err)
		return
	} else if !ok {
		writeError(w, r, errors.E(errors.NotFound, errors.User("no token found")))
		return
	}
	if err := h.readerRuntimeStore.DeleteReaderToken(r.Context(), requestTenant(r), name); err != nil {
		writeError(w, r, err)
		return
	}
	if state, err := h.scanningStore.GetScanningState(r.Context(), requestTenant(r)); err == nil && state.ActiveReader == name {
		now := time.Now()
		if err := h.scanningStore.UpdateScanningState(r.Context(), requestTenant(r), store.ScanningStateUpdate{
			State:         store.ScanningStateNeedsAuth,
			ReasonCode:    store.ScanningReasonMissingToken,
			PublicMessage: "Connect your reader account to continue scanning.",
			LastFailedAt:  &now,
			RetryCount:    intPointer(0),
		}); err != nil {
			h.logger.Warn("failed to update scanning state after token removal", "reader", name, "error", err)
		}
	}

	h.logger.Info("token revoked", "reader", name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// generateState creates a cryptographically random OAuth state token.
func generateState(readerName string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", errors.E("httpapi.handlers_readers.generate_state", "generating OAuth state", err)
	}
	return fmt.Sprintf("reader:%s:%s", readerName, hex.EncodeToString(b)), nil
}
