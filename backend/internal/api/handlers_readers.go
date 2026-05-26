package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"golang.org/x/oauth2"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/client"
	tbreader "github.com/ArionMiles/expensor/backend/pkg/reader/thunderbird"
)

// --- plugin listing ---

// ReaderInfo is the API representation of a reader plugin.
type ReaderInfo struct {
	Name                      string                `json:"name"`
	Description               string                `json:"description"`
	AuthType                  plugins.AuthType      `json:"auth_type"`
	RequiresCredentialsUpload bool                  `json:"requires_credentials_upload"`
	ConfigSchema              []plugins.ConfigField `json:"config_schema"`
}

// WriterInfo is the API representation of a writer plugin.
type WriterInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListReaders handles GET /api/plugins/readers.
// @Summary List reader plugins
// @Tags Readers
// @Produce json
// @Success 200 {array} ReaderInfoResponse
// @Router /plugins/readers [get]
func (h *Handlers) ListReaders(w http.ResponseWriter, _ *http.Request) {
	rps := h.registry.ListReaders()
	infos := make([]ReaderInfo, 0, len(rps))
	for _, p := range rps {
		meta := p.Metadata()
		configSchema := meta.ConfigSchema
		if configSchema == nil {
			configSchema = []plugins.ConfigField{}
		}
		infos = append(infos, ReaderInfo{
			Name:                      meta.Name,
			Description:               meta.Description,
			AuthType:                  meta.Auth.Type,
			RequiresCredentialsUpload: meta.Auth.RequiresCredentialsUpload,
			ConfigSchema:              configSchema,
		})
	}
	writeJSON(w, http.StatusOK, infos)
}

// ListWriters handles GET /api/plugins/writers.
// @Summary List writer plugins
// @Tags Readers
// @Produce json
// @Success 200 {array} WriterInfoResponse
// @Router /plugins/writers [get]
func (h *Handlers) ListWriters(w http.ResponseWriter, _ *http.Request) {
	wps := h.registry.ListWriters()
	infos := make([]WriterInfo, 0, len(wps))
	for _, p := range wps {
		meta := p.Metadata()
		infos = append(infos, WriterInfo{
			Name:        meta.Name,
			Description: meta.Description,
		})
	}
	writeJSON(w, http.StatusOK, infos)
}

// --- reader credentials upload ---

// UploadCredentials handles POST /api/readers/{name}/credentials.
// Accepts a JSON file upload (e.g. Google client_secret.json) and saves it to the runtime store.
func (h *Handlers) UploadCredentials(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	plugin, err := h.registry.GetReader(name)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
		return
	}
	if !plugin.Metadata().Auth.RequiresCredentialsUpload {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("reader %q does not require credentials upload", name))
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

	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	if err := h.store.SetReaderSecret(r.Context(), name, body); err != nil {
		h.logger.Error("failed to save credentials", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save credentials")
		return
	}

	h.logger.Info("credentials uploaded", "reader", name)
	writeJSON(w, http.StatusOK, map[string]string{"path": fmt.Sprintf("db://reader_runtime/%s/client_secret", name)})
}

// CredentialsStatus handles GET /api/readers/{name}/credentials/status.
func (h *Handlers) CredentialsStatus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.registry.GetReader(name); err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
		return
	}

	if h.store == nil {
		writeJSON(w, http.StatusOK, map[string]bool{"exists": false})
		return
	}
	_, exists, err := h.store.GetReaderSecret(r.Context(), name)
	if err != nil {
		h.logger.Error("failed to load credentials status", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load credentials status")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"exists": exists})
}

// --- OAuth flow ---

// AuthStart handles POST /api/readers/{name}/auth/start.
// Returns a Google OAuth consent URL for the given reader.
func (h *Handlers) AuthStart(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	plugin, err := h.registry.GetReader(name)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
		return
	}
	if plugin.Metadata().Auth.Type != plugins.AuthTypeOAuth {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("reader %q does not use OAuth", name))
		return
	}

	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	h.logger.Debug("reading credentials from store", "reader", name)
	secretJSON, ok, err := h.store.GetReaderSecret(r.Context(), name)
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
	scopes := plugin.Metadata().Auth.RequiredScopes
	h.logger.Debug("building OAuth config", "reader", name, "redirect_url", redirectURL, "scopes", scopes)
	oauthCfg, err := client.GetOAuthConfig(secretJSON, redirectURL, scopes...)
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
func (h *Handlers) exchangeAndSaveToken(name, code, redirectURL string) error {
	plugin, err := h.registry.GetReader(name)
	if err != nil {
		return fmt.Errorf("%w: %w", errReaderNotRegistered, err)
	}

	if h.store == nil {
		return errors.New("database not connected")
	}
	secretJSON, ok, err := h.store.GetReaderSecret(context.Background(), name)
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}
	if !ok {
		return errCredentialsMissing
	}

	oauthCfg, err := client.GetOAuthConfig(secretJSON, redirectURL, plugin.Metadata().Auth.RequiredScopes...)
	if err != nil {
		return fmt.Errorf("failed to parse credentials: %w", err)
	}

	tok, err := oauthCfg.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	tokenJSON, err := json.Marshal(tok) //nolint:gosec // OAuth tokens are intentionally serialized into the runtime store.
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}
	if err := h.store.SetReaderToken(context.Background(), name, tokenJSON); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}
	h.restartReaderDaemonAfterAuth(name)
	return nil
}

func (h *Handlers) restartReaderDaemonAfterAuth(name string) {
	if h.daemon.Status().Running && h.restartFn != nil {
		h.restartFn(name)
	}
}

// AuthCallback handles GET /api/auth/callback.
// This is the shared OAuth redirect target for all readers.
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
	if err := h.exchangeAndSaveToken(name, code, redirectURL); err != nil {
		h.logger.Error("OAuth token exchange failed", "reader", name, "error", err)
		if errors.Is(err, errCredentialsMissing) || errors.Is(err, errReaderNotRegistered) {
			writeError(w, http.StatusInternalServerError, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	h.logger.Info("OAuth token saved", "reader", name)
	http.Redirect(w, r, h.frontendURL+"/setup?auth=success&reader="+url.QueryEscape(name), http.StatusFound)
}

// AuthExchange handles POST /api/readers/{name}/auth/exchange.
// Accepts the full redirect URL (containing code and state params) pasted by
// the user when the automatic redirect is unreachable (e.g. homeserver without
// a public domain). Validates state, exchanges the code, and saves the token.
func (h *Handlers) AuthExchange(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.URL == "" {
		writeError(w, http.StatusBadRequest, "request body must be JSON with a non-empty \"url\" field")
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
	if err := h.exchangeAndSaveToken(name, code, redirectURL); err != nil {
		h.logger.Error("manual OAuth exchange failed", "reader", name, "error", err)
		if errors.Is(err, errCredentialsMissing) || errors.Is(err, errReaderNotRegistered) {
			writeError(w, http.StatusInternalServerError, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	h.logger.Info("OAuth token saved via manual exchange", "reader", name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "authorized"})
}

// AuthStatus handles GET /api/readers/{name}/auth/status.
func (h *Handlers) AuthStatus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	plugin, err := h.registry.GetReader(name)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
		return
	}
	if plugin.Metadata().Auth.Type != plugins.AuthTypeOAuth {
		// Config-only readers are always "authenticated" once configured.
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "auth_type": plugins.AuthTypeConfig})
		return
	}

	if h.store == nil {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	tokenJSON, ok, err := h.store.GetReaderToken(r.Context(), name)
	if err != nil {
		h.logger.Error("failed to load token", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load token")
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	var tok oauth2.Token
	if err := json.Unmarshal(tokenJSON, &tok); err != nil {
		h.logger.Warn("failed to parse token", "reader", name, "error", err)
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": tok.Valid(),
		"expiry":        tok.Expiry,
	})
}

// DisconnectReader handles DELETE /api/readers/{name}.
// Removes all stored credentials, token, and config files for the reader.
func (h *Handlers) DisconnectReader(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.registry.GetReader(name); err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
		return
	}

	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	if err := h.store.DeleteReaderRuntime(r.Context(), name); err != nil {
		h.logger.Error("failed to disconnect reader", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to disconnect reader")
		return
	}

	h.logger.Info("reader disconnected", "reader", name)
	writeJSON(w, http.StatusOK, map[string]any{"status": "disconnected", "files_removed": []string{}})
}

// RevokeToken handles DELETE /api/readers/{name}/auth/token.
func (h *Handlers) RevokeToken(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.registry.GetReader(name); err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
		return
	}

	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	if _, ok, err := h.store.GetReaderToken(r.Context(), name); err != nil {
		h.logger.Error("failed to load token", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to remove token")
		return
	} else if !ok {
		writeError(w, http.StatusNotFound, "no token found")
		return
	}
	if err := h.store.DeleteReaderToken(r.Context(), name); err != nil {
		h.logger.Error("failed to remove token", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to remove token")
		return
	}

	h.logger.Info("token revoked", "reader", name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// --- reader config (for config-only readers like Thunderbird) ---

// GetReaderConfig handles GET /api/readers/{name}/config.
func (h *Handlers) GetReaderConfig(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.registry.GetReader(name); err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
		return
	}

	if h.store == nil {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	data, ok, err := h.store.GetReaderConfig(r.Context(), name)
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
	_, _ = w.Write(data) //nolint:gosec // data is JSON read from a known config file; Content-Type is already set to application/json
}

// SaveReaderConfig handles POST /api/readers/{name}/config.
func (h *Handlers) SaveReaderConfig(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.registry.GetReader(name); err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
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
		writeError(w, http.StatusUnprocessableEntity, "body is not valid JSON")
		return
	}

	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	if err := h.store.SetReaderConfig(r.Context(), name, json.RawMessage(body)); err != nil {
		h.logger.Error("failed to save config", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}

	h.logger.Info("reader config saved", "reader", name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// ReaderStatus handles GET /api/readers/{name}/status.
// Returns overall readiness: credentials present, auth valid, config present.
func (h *Handlers) ReaderStatus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	plugin, err := h.registry.GetReader(name)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
		return
	}

	type readerStatus struct {
		CredentialsUploaded bool             `json:"credentials_uploaded"`
		Authenticated       bool             `json:"authenticated"`
		ConfigPresent       bool             `json:"config_present"`
		AuthType            plugins.AuthType `json:"auth_type"`
		Ready               bool             `json:"ready"`
	}

	meta := plugin.Metadata()
	st := readerStatus{AuthType: meta.Auth.Type}

	if meta.Auth.RequiresCredentialsUpload {
		if h.store != nil {
			_, ok, err := h.store.GetReaderSecret(r.Context(), name)
			st.CredentialsUploaded = err == nil && ok
		}
	} else {
		st.CredentialsUploaded = true
	}

	if meta.Auth.Type == plugins.AuthTypeOAuth {
		if h.store != nil {
			tokenJSON, ok, err := h.store.GetReaderToken(r.Context(), name)
			if err == nil && ok {
				var tok oauth2.Token
				st.Authenticated = json.Unmarshal(tokenJSON, &tok) == nil && tok.Valid()
			}
		}
	} else {
		st.Authenticated = true
	}

	if len(meta.ConfigSchema) == 0 {
		st.ConfigPresent = true
	} else if h.store != nil {
		_, ok, err := h.store.GetReaderConfig(r.Context(), name)
		st.ConfigPresent = err == nil && ok
	}

	st.Ready = st.CredentialsUploaded && st.Authenticated && st.ConfigPresent
	writeJSON(w, http.StatusOK, st)
}

// DiscoverProfiles handles GET /api/readers/thunderbird/discover/profiles.
// Returns discovered Thunderbird profile directories from platform paths,
// the Docker mount /thunderbird-profile, and THUNDERBIRD_DATA_DIR env var.
func (h *Handlers) DiscoverProfiles(w http.ResponseWriter, _ *http.Request) {
	var paths []string
	seen := make(map[string]struct{})

	addIfExists := func(p string) {
		if p == "" {
			return
		}
		if _, err := os.Stat(p); err == nil { //nolint:gosec // path built from well-known OS locations, not user input
			if _, exists := seen[p]; !exists {
				seen[p] = struct{}{}
				paths = append(paths, p)
			}
		}
	}

	if discovered, err := tbreader.FindProfiles(); err == nil {
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

// DiscoverMailboxes handles GET /api/readers/thunderbird/discover/mailboxes?profile=<path>.
// Returns available MBOX mailbox names within the given Thunderbird profile directory.
func (h *Handlers) DiscoverMailboxes(w http.ResponseWriter, r *http.Request) {
	profile := r.URL.Query().Get("profile")
	if profile == "" {
		writeError(w, http.StatusBadRequest, "profile query parameter is required")
		return
	}
	if _, err := os.Stat(profile); os.IsNotExist(err) { //nolint:gosec // profile from query param, existence checked
		writeError(w, http.StatusNotFound, "profile directory not found")
		return
	}
	mailboxes, err := tbreader.ListMailboxes(profile)
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

// GetReaderGuide handles GET /api/readers/{name}/guide.
// Returns the structured setup guide for a reader when metadata includes one.
// @Summary Get reader setup guide
// @Tags Readers
// @Produce json
// @Param name path string true "Reader name" example(thunderbird)
// @Success 200 {object} ReaderGuideResponse
// @Failure 404 {object} ErrorResponse
// @Router /readers/{name}/guide [get]
func (h *Handlers) GetReaderGuide(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	plugin, err := h.registry.GetReader(name)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
		return
	}
	guideData := plugin.Metadata().SetupGuide
	if len(guideData) == 0 {
		writeError(w, http.StatusNotFound, "no setup guide available for this reader")
		return
	}
	var guide plugins.ReaderGuide
	if err := json.Unmarshal(guideData, &guide); err != nil {
		h.logger.Error("parsing reader guide", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to parse reader guide")
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
