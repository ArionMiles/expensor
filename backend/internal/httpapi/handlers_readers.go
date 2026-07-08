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
	"os"
	"time"

	"golang.org/x/oauth2"

	"github.com/ArionMiles/expensor/backend/internal/auth"
	"github.com/ArionMiles/expensor/backend/internal/oauth"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
	"github.com/ArionMiles/expensor/backend/pkg/reader/thunderbird"
)

// --- provider listing ---

// ProviderInfo is the API representation of an email provider.
type ProviderInfo struct {
	Name                      string                `json:"name"`
	Description               string                `json:"description"`
	AuthType                  plugins.AuthType      `json:"auth_type"`
	RequiresCredentialsUpload bool                  `json:"requires_credentials_upload"`
	ConfigSchema              []plugins.ConfigField `json:"config_schema"`
}

// ListProviders handles GET /api/providers.
// @Summary List providers
// @Tags Providers
// @Produce json
// @Success 200 {array} ProviderInfoResponse
// @Router /providers [get]
func (h *Handlers) ListProviders(w http.ResponseWriter, _ *http.Request) {
	rps := h.registry.ListProviders()
	infos := make([]ProviderInfo, 0, len(rps))
	for _, p := range rps {
		meta := p.Metadata
		configSchema := meta.ConfigSchema
		if configSchema == nil {
			configSchema = []plugins.ConfigField{}
		}
		infos = append(infos, ProviderInfo{
			Name:                      meta.Name,
			Description:               meta.Description,
			AuthType:                  meta.Auth.Type,
			RequiresCredentialsUpload: meta.Auth.RequiresCredentialsUpload,
			ConfigSchema:              configSchema,
		})
	}
	writeJSON(w, http.StatusOK, infos)
}

const defaultProviderMessageSearchLimit = 10

// SearchProviderMessages handles GET /api/providers/{name}/messages.
// @Summary Search provider messages for rule samples
// @Tags Providers
// @Produce json
// @Param name path string true "Provider name" example(gmail)
// @Param subject query string true "Subject substring"
// @Param limit query int false "Maximum messages to return" minimum(1) maximum(50)
// @Success 200 {object} ProviderSearchResponse
// @Failure 404 {object} ErrorResponse
// @Failure 412 {object} ErrorResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /providers/{name}/messages [get]
func (h *Handlers) SearchProviderMessages(w http.ResponseWriter, r *http.Request) {
	query, ok := decodeAndValidateQuery[providerMessagesQuery](h, w, r)
	if !ok {
		return
	}
	if query.Limit == 0 {
		query.Limit = defaultProviderMessageSearchLimit
	}

	name := r.PathValue("name")
	searcher, err := h.newEmailSearcher(r.Context(), requestTenant(r), name)
	if err != nil {
		switch {
		case errors.WhatKind(err) == errors.NotFound:
			writeError(w, http.StatusNotFound, fmt.Sprintf("provider %q not found", name))
		case errors.WhatKind(err) == oauth.KindCredentialsMissing, errors.WhatKind(err) == oauth.KindTokenMissing:
			writeError(w, http.StatusPreconditionFailed, "provider is not authenticated")
		default:
			h.logger.Error("create provider for message search", "provider", name, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to prepare provider search")
		}
		return
	}

	results, err := searcher.Search(r.Context(), api.EmailSearchQuery{
		SubjectQuery: query.Subject,
		Limit:        query.Limit,
	})
	if err != nil {
		h.logger.Error("search provider messages", "provider", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to search provider messages")
		return
	}
	writeJSON(w, http.StatusOK, ProviderSearchResponse{Results: providerSearchResultsToHTTP(results)})
}

func (h *Handlers) newEmailSearcher(ctx context.Context, tenant store.Tenant, name string) (api.EmailSearcher, error) {
	const op = "httpapi.Handlers.newEmailSearcher"

	provider, err := h.registry.GetProvider(name)
	if err != nil {
		return nil, errors.E(op, errors.NotFound, fmt.Sprintf("reader %q is no longer registered", name), err)
	}

	var httpClient *http.Client
	meta := provider.Metadata
	if meta.Auth.Type == plugins.AuthTypeOAuth {
		secretJSON, ok, err := h.readerRuntimeStore.GetReaderSecret(ctx, tenant, name)
		if err != nil {
			return nil, fmt.Errorf("loading credentials for provider %q: %w", name, err)
		}
		if !ok {
			return nil, errors.E(op, oauth.KindCredentialsMissing, "credentials file missing")
		}
		httpClient, err = oauth.NewFromJSONAndStore(ctx, oauth.StoreClientInput{
			SecretJSON: secretJSON,
			Store:      h.readerRuntimeStore,
			Tenant:     tenant,
			Reader:     name,
			Scopes:     meta.Auth.RequiredScopes,
		})
		if err != nil {
			return nil, err
		}
	}

	readerConfig, _, err := h.readerRuntimeStore.GetReaderConfig(ctx, tenant, name)
	if err != nil {
		return nil, fmt.Errorf("loading provider config for %q: %w", name, err)
	}
	return provider.NewEmailSearcher(plugins.ProviderInput{
		HTTPClient:   httpClient,
		AppConfig:    &config.App{ScanInterval: h.scanInterval, LookbackDays: h.lookbackDays},
		ReaderConfig: readerConfig,
		Logger:       h.logger,
	})
}

func providerSearchResultsToHTTP(results []api.EmailSearchResult) []ProviderSearchResultResponse {
	out := make([]ProviderSearchResultResponse, 0, len(results))
	for _, result := range results {
		out = append(out, ProviderSearchResultResponse{
			ID:          result.ID,
			SenderEmail: result.SenderEmail,
			Subject:     result.Subject,
			Body:        result.Body,
			ReceivedAt:  result.ReceivedAt,
		})
	}
	return out
}

// --- provider credentials upload ---

func requestTenant(r *http.Request) store.Tenant {
	if principal, ok := auth.PrincipalFromContext(r.Context()); ok {
		return store.Tenant{ID: principal.TenantID}
	}
	return store.Tenant{}
}

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
		writeError(w, http.StatusNotFound, fmt.Sprintf("provider %q not found", name))
		return
	}
	if !provider.Metadata.Auth.RequiresCredentialsUpload {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("provider %q does not require credentials upload", name))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCredentialsSize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "file too large (max 5 MB)")
		return
	}

	// Validate it is valid OAuth2 client credentials JSON.
	// Google client_secret.json must have a "web" or "installed" top-level key.
	var creds struct {
		Web       json.RawMessage `json:"web"`
		Installed json.RawMessage `json:"installed"`
	}
	if err := json.Unmarshal(body, &creds); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "file is not valid JSON")
		return
	}
	if creds.Web == nil && creds.Installed == nil {
		writeError(w, http.StatusUnprocessableEntity,
			`invalid credentials file: expected a Google OAuth2 client_secret.json with a "web" or "installed"`+
				` top-level key — download it from Google Cloud Console → APIs & Services → Credentials → OAuth 2.0 Client IDs`)
		return
	}

	if err := h.readerRuntimeStore.SetReaderSecret(r.Context(), requestTenant(r), name, body); err != nil {
		h.logger.Error("failed to save credentials", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save credentials")
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
		writeError(w, http.StatusNotFound, fmt.Sprintf("provider %q not found", name))
		return
	}

	_, exists, err := h.readerRuntimeStore.GetReaderSecret(r.Context(), requestTenant(r), name)
	if err != nil {
		h.logger.Error("failed to load credentials status", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load credentials status")
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
		writeError(w, http.StatusNotFound, fmt.Sprintf("provider %q not found", name))
		return
	}
	if provider.Metadata.Auth.Type != plugins.AuthTypeOAuth {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("provider %q does not use OAuth", name))
		return
	}

	tenant := requestTenant(r)
	h.logger.Debug("reading credentials from store", "reader", name)
	secretJSON, ok, err := h.readerRuntimeStore.GetReaderSecret(r.Context(), tenant, name)
	if err != nil {
		h.logger.Error("failed to load credentials", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load credentials")
		return
	}
	if !ok {
		writeError(w, http.StatusPreconditionFailed, "credentials not uploaded — upload client credentials first")
		return
	}

	redirectURL := h.baseURL + "/api/auth/callback"
	scopes := provider.Metadata.Auth.RequiredScopes
	h.logger.Debug("building OAuth config", "reader", name, "redirect_url", redirectURL, "scopes", scopes)
	oauthCfg, err := oauth.GetOAuthConfig(secretJSON, redirectURL, scopes...)
	if err != nil {
		h.logger.Error("failed to parse credentials", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to parse credentials: "+err.Error())
		return
	}

	// Generate a random state token that encodes the reader name.
	state, err := generateState(name)
	if err != nil {
		h.logger.Error("failed to generate OAuth state", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to initiate OAuth flow")
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
		return fmt.Errorf("failed to load credentials: %w", err)
	}
	if !ok {
		return errors.E(op, oauth.KindCredentialsMissing, "credentials file missing")
	}

	oauthCfg, err := oauth.GetOAuthConfig(secretJSON, redirectURL, provider.Metadata.Auth.RequiredScopes...)
	if err != nil {
		return fmt.Errorf("failed to parse credentials: %w", err)
	}

	tok, err := oauthCfg.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	tokenJSON, err := json.Marshal(tok) //nolint:gosec // OAuth tokens are intentionally serialized into the runtime store.
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}
	if err := h.readerRuntimeStore.SetReaderToken(ctx, tenant, name, tokenJSON); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}
	h.queueReaderScanning(ctx, tenant, name)
	h.restartReaderDaemonAfterAuth(tenant, name)
	return nil
}

func (h *Handlers) restartReaderDaemonAfterAuth(tenant store.Tenant, name string) {
	if h.daemon.Status().Running && h.restartFn != nil {
		h.restartFn(DaemonRunRequest{Tenant: tenant, Reader: name})
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
		writeError(w, http.StatusBadRequest, "invalid or expired OAuth state")
		return
	}

	redirectURL := h.baseURL + "/api/auth/callback"
	tenant, tenantOK := entryTenant(entry, requestTenant(r))
	if !tenantOK {
		writeError(w, http.StatusBadRequest, "invalid or expired OAuth state")
		return
	}
	if err := h.exchangeAndSaveToken(r.Context(), tenant, name, code, redirectURL); err != nil {
		h.logger.Error("OAuth token exchange failed", "reader", name, "error", err)
		if errors.WhatKind(err) == oauth.KindCredentialsMissing || errors.WhatKind(err) == errors.NotFound {
			writeError(w, http.StatusInternalServerError, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
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
// @Failure 422 {object} ValidationErrorResponse
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
		writeError(w, http.StatusBadRequest, "invalid url: "+err.Error())
		return
	}

	code := parsed.Query().Get("code")
	state := parsed.Query().Get("state")

	if code == "" {
		writeError(w, http.StatusBadRequest, "url is missing the \"code\" query parameter")
		return
	}
	if state == "" {
		writeError(w, http.StatusBadRequest, "url is missing the \"state\" query parameter")
		return
	}

	h.mu.Lock()
	entry, ok := h.oauthStates[state]
	if ok {
		delete(h.oauthStates, state)
	}
	h.mu.Unlock()

	if !ok || time.Now().After(entry.expiresAt) {
		writeError(w, http.StatusBadRequest, "invalid or expired OAuth state")
		return
	}

	redirectURL := h.baseURL + "/api/auth/callback"
	tenant, tenantOK := entryTenant(entry, requestTenant(r))
	if !tenantOK {
		writeError(w, http.StatusBadRequest, "invalid or expired OAuth state")
		return
	}
	if err := h.exchangeAndSaveToken(r.Context(), tenant, name, code, redirectURL); err != nil {
		h.logger.Error("manual OAuth exchange failed", "reader", name, "error", err)
		if errors.WhatKind(err) == oauth.KindCredentialsMissing || errors.WhatKind(err) == errors.NotFound {
			writeError(w, http.StatusInternalServerError, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
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
		writeError(w, http.StatusNotFound, fmt.Sprintf("provider %q not found", name))
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
		h.logger.Error("failed to load token", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load token")
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
		h.logger.Error("failed to resolve OAuth token state", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to resolve OAuth token state")
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
		return oauthTokenState{authState: authStateRefreshPending, expiry: expiry}, fmt.Errorf("loading credentials for token refresh: %w", err)
	}
	if !ok {
		return oauthTokenState{authState: authStateReauthorizationRequired, expiry: expiry}, nil
	}
	oauthCfg, err := oauth.GetOAuthConfig(secretJSON, h.baseURL+"/api/auth/callback", scopes...)
	if err != nil {
		return oauthTokenState{authState: authStateReauthorizationRequired, expiry: expiry}, fmt.Errorf("parsing credentials for token refresh: %w", err)
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
		return oauthTokenState{authState: authStateRefreshPending, expiry: expiry}, fmt.Errorf("marshaling refreshed token: %w", err)
	}
	if err := h.readerRuntimeStore.SetReaderToken(ctx, tenant, name, refreshedJSON); err != nil {
		return oauthTokenState{authState: authStateRefreshPending, expiry: expiry}, fmt.Errorf("saving refreshed token: %w", err)
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
		writeError(w, http.StatusNotFound, fmt.Sprintf("provider %q not found", name))
		return
	}

	tenant := requestTenant(r)
	state, err := h.scanningStore.GetScanningState(r.Context(), tenant)
	if err != nil {
		h.logger.Error("failed to read scanning state before disconnect", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to disconnect reader")
		return
	}
	if err := h.readerRuntimeStore.DeleteReaderRuntime(r.Context(), tenant, name); err != nil {
		h.logger.Error("failed to disconnect reader", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to disconnect reader")
		return
	}
	if state.ActiveReader == name {
		if err := h.scanningStore.ClearActiveScanningReader(r.Context(), tenant); err != nil {
			h.logger.Error("failed to clear active scanning reader", "reader", name, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to disconnect reader")
			return
		}
		if h.daemon.Status().Running && h.stopFn != nil {
			h.stopFn()
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
		writeError(w, http.StatusNotFound, fmt.Sprintf("provider %q not found", name))
		return
	}

	if _, ok, err := h.readerRuntimeStore.GetReaderToken(r.Context(), requestTenant(r), name); err != nil {
		h.logger.Error("failed to load token", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to remove token")
		return
	} else if !ok {
		writeError(w, http.StatusNotFound, "no token found")
		return
	}
	if err := h.readerRuntimeStore.DeleteReaderToken(r.Context(), requestTenant(r), name); err != nil {
		h.logger.Error("failed to remove token", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to remove token")
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

// --- provider config (for config-only readers like Thunderbird) ---

// GetReaderConfig handles GET /api/providers/{name}/config.
// @Summary Get reader runtime config
// @Tags Providers
// @Produce json
// @Param name path string true "Provider name" example(thunderbird)
// @Success 200 {object} ProviderConfigResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /providers/{name}/config [get]
func (h *Handlers) GetReaderConfig(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.registry.GetProvider(name); err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("provider %q not found", name))
		return
	}

	data, ok, err := h.readerRuntimeStore.GetReaderConfig(r.Context(), requestTenant(r), name)
	if err != nil {
		h.logger.Error("failed to read config", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to read config")
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	//nolint:gosec // provider config is stored JSON returned with application/json content type
	_, _ = w.Write(data)
}

// SaveReaderConfig handles PUT /api/providers/{name}/config.
// @Summary Save reader runtime config
// @Tags Providers
// @Accept json
// @Produce json
// @Param name path string true "Provider name" example(thunderbird)
// @Param request body ProviderConfigRequest true "Provider config JSON"
// @Success 200 {object} ProviderConfigSaveResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /providers/{name}/config [put]
func (h *Handlers) SaveReaderConfig(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.registry.GetProvider(name); err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("provider %q not found", name))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCredentialsSize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	// Validate JSON.
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := h.readerRuntimeStore.SetReaderConfig(r.Context(), requestTenant(r), name, json.RawMessage(body)); err != nil {
		h.logger.Error("failed to save config", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}

	h.queueReaderScanning(r.Context(), requestTenant(r), name)
	h.logger.Info("provider config saved", "reader", name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// ReaderStatus handles GET /api/providers/{name}/status.
// Returns overall readiness: credentials present, auth valid, config present.
// @Summary Get provider readiness status
// @Tags Providers
// @Produce json
// @Param name path string true "Provider name" example(thunderbird)
// @Success 200 {object} ProviderStatusResponse
// @Failure 404 {object} ErrorResponse
// @Router /providers/{name}/status [get]
func (h *Handlers) ReaderStatus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	provider, err := h.registry.GetProvider(name)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("provider %q not found", name))
		return
	}

	type readerStatus struct {
		CredentialsUploaded bool             `json:"credentials_uploaded"`
		Authenticated       bool             `json:"authenticated"`
		ConfigPresent       bool             `json:"config_present"`
		AuthType            plugins.AuthType `json:"auth_type"`
		AuthState           string           `json:"auth_state"`
		Ready               bool             `json:"ready"`
	}

	meta := provider.Metadata
	st := readerStatus{AuthType: meta.Auth.Type}

	if meta.Auth.RequiresCredentialsUpload {
		_, ok, err := h.readerRuntimeStore.GetReaderSecret(r.Context(), requestTenant(r), name)
		if err != nil {
			h.logger.Error("failed to load credentials status", "reader", name, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to load credentials status")
			return
		}
		st.CredentialsUploaded = ok
	} else {
		st.CredentialsUploaded = true
	}

	st.Authenticated, st.AuthState = h.resolveReaderAuthStatus(r.Context(), requestTenant(r), name, meta)

	if len(meta.ConfigSchema) == 0 {
		st.ConfigPresent = true
	} else {
		_, ok, err := h.readerRuntimeStore.GetReaderConfig(r.Context(), requestTenant(r), name)
		if err != nil {
			h.logger.Error("failed to load provider config status", "reader", name, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to load provider config status")
			return
		}
		st.ConfigPresent = ok
	}

	st.Ready = st.CredentialsUploaded && st.Authenticated && st.ConfigPresent
	writeJSON(w, http.StatusOK, st)
}

// DiscoverProfiles handles GET /api/providers/thunderbird/discover/profiles.
// Returns discovered Thunderbird profile directories from platform paths,
// the Docker mount /thunderbird-profile, and THUNDERBIRD_DATA_DIR env var.
// @Summary Discover Thunderbird profiles
// @Tags Providers
// @Produce json
// @Success 200 {object} ThunderbirdProfilesResponse
// @Router /providers/thunderbird/discover/profiles [get]
func (h *Handlers) DiscoverProfiles(w http.ResponseWriter, _ *http.Request) {
	var paths []string
	seen := make(map[string]struct{})

	addIfExists := func(p string) {
		if p == "" {
			return
		}
		if _, err := os.Stat(p); err == nil {
			if _, exists := seen[p]; !exists {
				seen[p] = struct{}{}
				paths = append(paths, p)
			}
		}
	}

	if discovered, err := thunderbird.FindProfiles(); err == nil {
		for _, p := range discovered {
			addIfExists(p)
		}
	}
	addIfExists("/thunderbird-profile")
	addIfExists(h.thunderbirdDataDir)

	if paths == nil {
		paths = []string{}
	}
	writeJSON(w, http.StatusOK, map[string][]string{"profiles": paths})
}

// DiscoverMailboxes handles GET /api/providers/thunderbird/discover/mailboxes?profile=<path>.
// Returns available MBOX mailbox names within the given Thunderbird profile directory.
// @Summary Discover Thunderbird mailboxes
// @Tags Providers
// @Produce json
// @Param profile query string true "Thunderbird profile path"
// @Success 200 {object} ThunderbirdMailboxesResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /providers/thunderbird/discover/mailboxes [get]
func (h *Handlers) DiscoverMailboxes(w http.ResponseWriter, r *http.Request) {
	query, ok := decodeAndValidateQuery[mailboxDiscoveryQuery](h, w, r)
	if !ok {
		return
	}
	profile := query.Profile
	if _, err := os.Stat(profile); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "profile directory not found")
		return
	}
	mailboxes, err := thunderbird.ListMailboxes(profile)
	if err != nil {
		h.logger.Error("discovering mailboxes", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to discover mailboxes")
		return
	}
	if mailboxes == nil {
		mailboxes = []string{}
	}
	writeJSON(w, http.StatusOK, map[string][]string{"mailboxes": mailboxes})
}

// GetProviderGuide handles GET /api/providers/{name}/guide.
// Returns the structured setup guide for a provider when metadata includes one.
// @Summary Get provider setup guide
// @Tags Providers
// @Produce json
// @Param name path string true "Provider name" example(thunderbird)
// @Success 200 {object} ProviderGuideResponse
// @Failure 404 {object} ErrorResponse
// @Router /providers/{name}/guide [get]
func (h *Handlers) GetProviderGuide(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	provider, err := h.registry.GetProvider(name)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("provider %q not found", name))
		return
	}
	guideData := provider.Metadata.SetupGuide
	if len(guideData) == 0 {
		writeError(w, http.StatusNotFound, "no setup guide available for this provider")
		return
	}
	var guide plugins.ProviderGuide
	if err := json.Unmarshal(guideData, &guide); err != nil {
		h.logger.Error("parsing provider guide", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to parse provider guide")
		return
	}
	writeJSON(w, http.StatusOK, guide)
}

// generateState creates a cryptographically random OAuth state token.
func generateState(readerName string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating OAuth state: %w", err)
	}
	return fmt.Sprintf("reader:%s:%s", readerName, hex.EncodeToString(b)), nil
}
