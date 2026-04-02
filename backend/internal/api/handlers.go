package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/client"
)

const (
	maxCredentialsSize = 5 << 20 // 5 MB
	dataDir            = "data"
	oauthStateTTL      = 10 * time.Minute
)

// oauthStateEntry holds a pending OAuth state with an expiry time.
type oauthStateEntry struct {
	readerName string
	expiresAt  time.Time
}

// credentialsFileName returns the per-reader credentials file path.
func credentialsFileName(readerName string) string {
	return filepath.Join(dataDir, fmt.Sprintf("client_secret_%s.json", readerName))
}

// tokenFileName returns the per-reader token file path.
func tokenFileName(readerName string) string {
	return filepath.Join(dataDir, fmt.Sprintf("token_%s.json", readerName))
}

// DaemonStatus represents the state of the background daemon.
type DaemonStatus struct {
	Running   bool       `json:"running"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	LastError string     `json:"last_error,omitempty"`
}

// DaemonStatusProvider is implemented by the daemon manager in main.go.
type DaemonStatusProvider interface {
	Status() DaemonStatus
}

// Handlers holds all dependencies for HTTP endpoint handlers.
type Handlers struct {
	registry    *plugins.Registry
	store       Storer
	daemon      DaemonStatusProvider
	baseURL     string // e.g. "http://localhost:8080"
	frontendURL string // e.g. "http://localhost:5173" — used for OAuth redirects
	startFn     func(reader string) // called by POST /api/daemon/start; may be nil
	logger      *slog.Logger

	// oauthStates maps state token → entry for in-flight OAuth flows.
	mu          sync.Mutex
	oauthStates map[string]oauthStateEntry
}

// NewHandlers creates a Handlers instance.
// Pass nil for st when no database is configured; transaction endpoints will return 503.
// Pass nil for startFn to disable the daemon start endpoint.
func NewHandlers(
	registry *plugins.Registry,
	st Storer,
	daemon DaemonStatusProvider,
	baseURL string,
	frontendURL string,
	startFn func(reader string),
	logger *slog.Logger,
) *Handlers {
	if frontendURL == "" {
		frontendURL = baseURL
	}
	return &Handlers{
		registry:    registry,
		store:       st,
		daemon:      daemon,
		baseURL:     strings.TrimRight(baseURL, "/"),
		frontendURL: strings.TrimRight(frontendURL, "/"),
		startFn:     startFn,
		logger:      logger,
		oauthStates: make(map[string]oauthStateEntry),
	}
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// --- health & status ---

// HandleHealth handles GET /api/health.
func (h *Handlers) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleStatus handles GET /api/status.
func (h *Handlers) HandleStatus(w http.ResponseWriter, r *http.Request) {
	ds := h.daemon.Status()

	type statusResponse struct {
		Daemon DaemonStatus `json:"daemon"`
		Stats  *store.Stats `json:"stats,omitempty"`
	}
	resp := statusResponse{Daemon: ds}

	if h.store != nil {
		if stats, err := h.store.GetStats(r.Context()); err == nil {
			resp.Stats = stats
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- plugin listing ---

// ReaderInfo is the API representation of a reader plugin.
type ReaderInfo struct {
	Name                      string               `json:"name"`
	Description               string               `json:"description"`
	AuthType                  plugins.AuthType     `json:"auth_type"`
	RequiresCredentialsUpload bool                 `json:"requires_credentials_upload"`
	ConfigSchema              []plugins.ConfigField `json:"config_schema"`
}

// WriterInfo is the API representation of a writer plugin.
type WriterInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// HandleListReaders handles GET /api/plugins/readers.
func (h *Handlers) HandleListReaders(w http.ResponseWriter, _ *http.Request) {
	rps := h.registry.ListReaders()
	infos := make([]ReaderInfo, 0, len(rps))
	for _, p := range rps {
		infos = append(infos, ReaderInfo{
			Name:                      p.Name(),
			Description:               p.Description(),
			AuthType:                  p.AuthType(),
			RequiresCredentialsUpload: p.RequiresCredentialsUpload(),
			ConfigSchema:              p.ConfigSchema(),
		})
	}
	writeJSON(w, http.StatusOK, infos)
}

// HandleListWriters handles GET /api/plugins/writers.
func (h *Handlers) HandleListWriters(w http.ResponseWriter, _ *http.Request) {
	wps := h.registry.ListWriters()
	infos := make([]WriterInfo, 0, len(wps))
	for _, p := range wps {
		infos = append(infos, WriterInfo{
			Name:        p.Name(),
			Description: p.Description(),
		})
	}
	writeJSON(w, http.StatusOK, infos)
}

// --- reader credentials upload ---

// HandleUploadCredentials handles POST /api/readers/{name}/credentials.
// Accepts a JSON file upload (e.g. Google client_secret.json) and saves it to data/.
func (h *Handlers) HandleUploadCredentials(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	plugin, err := h.registry.GetReader(name)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
		return
	}
	if !plugin.RequiresCredentialsUpload() {
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
			`invalid credentials file: expected a Google OAuth2 client_secret.json with a "web" or "installed" top-level key — download it from Google Cloud Console → APIs & Services → Credentials → OAuth 2.0 Client IDs`)
		return
	}

	dest := credentialsFileName(name)
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create data directory")
		return
	}
	if err := os.WriteFile(dest, body, 0o600); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save credentials")
		return
	}

	h.logger.Info("credentials uploaded", "reader", name, "path", dest)
	writeJSON(w, http.StatusOK, map[string]string{"path": dest})
}

// HandleCredentialsStatus handles GET /api/readers/{name}/credentials/status.
func (h *Handlers) HandleCredentialsStatus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.registry.GetReader(name); err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
		return
	}

	_, err := os.Stat(credentialsFileName(name))
	writeJSON(w, http.StatusOK, map[string]bool{"exists": err == nil})
}

// --- OAuth flow ---

// HandleAuthStart handles POST /api/readers/{name}/auth/start.
// Returns a Google OAuth consent URL for the given reader.
func (h *Handlers) HandleAuthStart(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	plugin, err := h.registry.GetReader(name)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
		return
	}
	if plugin.AuthType() != plugins.AuthTypeOAuth {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("reader %q does not use OAuth", name))
		return
	}

	credFile := credentialsFileName(name)
	h.logger.Debug("reading credentials file", "reader", name, "path", credFile)
	secretJSON, err := os.ReadFile(credFile)
	if err != nil {
		h.logger.Debug("credentials file not found", "reader", name, "path", credFile, "error", err)
		writeError(w, http.StatusPreconditionFailed, "credentials not uploaded — upload client credentials first")
		return
	}

	redirectURL := h.baseURL + "/api/auth/callback"
	h.logger.Debug("building OAuth config", "reader", name, "redirect_url", redirectURL, "scopes", plugin.RequiredScopes())
	oauthCfg, err := client.GetOAuthConfig(secretJSON, redirectURL, plugin.RequiredScopes()...)
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
	url := oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
	h.logger.Debug("OAuth URL generated", "reader", name, "state", state)
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

// HandleAuthCallback handles GET /api/auth/callback.
// This is the shared OAuth redirect target for all readers.
func (h *Handlers) HandleAuthCallback(w http.ResponseWriter, r *http.Request) {
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

	plugin, err := h.registry.GetReader(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "reader no longer registered")
		return
	}

	credFile := credentialsFileName(name)
	secretJSON, err := os.ReadFile(credFile)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "credentials file missing")
		return
	}

	redirectURL := h.baseURL + "/api/auth/callback"
	oauthCfg, err := client.GetOAuthConfig(secretJSON, redirectURL, plugin.RequiredScopes()...)
	if err != nil {
		h.logger.Error("failed to parse credentials in callback", "reader", name, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to parse credentials: "+err.Error())
		return
	}

	tok, err := oauthCfg.Exchange(context.Background(), code)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("token exchange failed: %v", err))
		return
	}

	if err := client.SaveToken(tokenFileName(name), tok); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save token")
		return
	}

	h.logger.Info("OAuth token saved", "reader", name)
	// Redirect back to the frontend wizard so the OAuth step can detect completion.
	http.Redirect(w, r, h.frontendURL+"/setup?auth=success&reader="+url.QueryEscape(name), http.StatusFound)
}

// HandleAuthStatus handles GET /api/readers/{name}/auth/status.
func (h *Handlers) HandleAuthStatus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	plugin, err := h.registry.GetReader(name)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
		return
	}
	if plugin.AuthType() != plugins.AuthTypeOAuth {
		// Config-only readers are always "authenticated" once configured.
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "auth_type": plugins.AuthTypeConfig})
		return
	}

	tok, err := client.TokenFromFile(tokenFileName(name))
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": tok.Valid(),
		"expiry":        tok.Expiry,
	})
}

// HandleDisconnectReader handles DELETE /api/readers/{name}.
// Removes all stored credentials, token, and config files for the reader.
func (h *Handlers) HandleDisconnectReader(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.registry.GetReader(name); err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
		return
	}

	files := []string{
		credentialsFileName(name),
		tokenFileName(name),
		filepath.Join(dataDir, fmt.Sprintf("config_%s.json", name)),
	}

	var removed []string
	for _, f := range files {
		if err := os.Remove(f); err == nil {
			removed = append(removed, filepath.Base(f))
		}
	}

	h.logger.Info("reader disconnected", "reader", name, "files_removed", removed)
	writeJSON(w, http.StatusOK, map[string]any{"status": "disconnected", "files_removed": removed})
}

// HandleRevokeToken handles DELETE /api/readers/{name}/auth/token.
func (h *Handlers) HandleRevokeToken(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.registry.GetReader(name); err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
		return
	}

	tokenFile := tokenFileName(name)
	if err := os.Remove(tokenFile); err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "no token found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to remove token")
		return
	}

	h.logger.Info("token revoked", "reader", name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// --- reader config (for config-only readers like Thunderbird) ---

// HandleGetReaderConfig handles GET /api/readers/{name}/config.
func (h *Handlers) HandleGetReaderConfig(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := h.registry.GetReader(name); err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("reader %q not found", name))
		return
	}

	cfgFile := filepath.Join(dataDir, fmt.Sprintf("config_%s.json", name))
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, map[string]any{})
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to read config")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// HandleSaveReaderConfig handles POST /api/readers/{name}/config.
func (h *Handlers) HandleSaveReaderConfig(w http.ResponseWriter, r *http.Request) {
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

	cfgFile := filepath.Join(dataDir, fmt.Sprintf("config_%s.json", name))
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create data directory")
		return
	}
	if err := os.WriteFile(cfgFile, body, 0o600); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}

	h.logger.Info("reader config saved", "reader", name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// HandleReaderStatus handles GET /api/readers/{name}/status.
// Returns overall readiness: credentials present, auth valid, config present.
func (h *Handlers) HandleReaderStatus(w http.ResponseWriter, r *http.Request) {
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

	st := readerStatus{AuthType: plugin.AuthType()}

	if plugin.RequiresCredentialsUpload() {
		_, err := os.Stat(credentialsFileName(name))
		st.CredentialsUploaded = err == nil
	} else {
		st.CredentialsUploaded = true
	}

	if plugin.AuthType() == plugins.AuthTypeOAuth {
		tok, err := client.TokenFromFile(tokenFileName(name))
		st.Authenticated = err == nil && tok.Valid()
	} else {
		st.Authenticated = true
	}

	cfgFile := filepath.Join(dataDir, fmt.Sprintf("config_%s.json", name))
	_, err = os.Stat(cfgFile)
	st.ConfigPresent = err == nil || len(plugin.ConfigSchema()) == 0

	st.Ready = st.CredentialsUploaded && st.Authenticated && st.ConfigPresent
	writeJSON(w, http.StatusOK, st)
}

// --- daemon control ---

// HandleStartDaemon handles POST /api/daemon/start.
// Body: {"reader": "gmail"}
// Triggers the background daemon with the given reader if it is not already running.
func (h *Handlers) HandleStartDaemon(w http.ResponseWriter, r *http.Request) {
	if h.startFn == nil {
		writeError(w, http.StatusNotImplemented, "daemon start not configured")
		return
	}
	if h.daemon.Status().Running {
		writeJSON(w, http.StatusOK, map[string]string{"status": "already_running"})
		return
	}

	var body struct {
		Reader string `json:"reader"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Reader == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must be {\"reader\": \"<name>\"}")
		return
	}
	if _, err := h.registry.GetReader(body.Reader); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("reader %q not found", body.Reader))
		return
	}

	h.logger.Info("daemon start requested", "reader", body.Reader)
	h.startFn(body.Reader)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "starting"})
}

// --- transactions ---

// HandleListTransactions handles GET /api/transactions.
func (h *Handlers) HandleListTransactions(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	f := store.ListFilter{
		Page:     queryInt(r, "page", 1),
		PageSize: queryInt(r, "page_size", 20),
		Category: r.URL.Query().Get("category"),
		Currency: r.URL.Query().Get("currency"),
		Label:    r.URL.Query().Get("label"),
	}
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = &t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = &t
		}
	}

	txns, total, err := h.store.ListTransactions(r.Context(), f)
	if err != nil {
		h.logger.Error("list transactions", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list transactions")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"transactions": txns,
		"total":        total,
		"page":         f.Page,
		"page_size":    f.PageSize,
	})
}

// HandleGetTransaction handles GET /api/transactions/{id}.
func (h *Handlers) HandleGetTransaction(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	id := r.PathValue("id")
	txn, err := h.store.GetTransaction(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found")
			return
		}
		h.logger.Error("get transaction", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch transaction")
		return
	}
	writeJSON(w, http.StatusOK, txn)
}

// HandleUpdateTransaction handles PUT /api/transactions/{id}.
// Body: {"description": "..."}
func (h *Handlers) HandleUpdateTransaction(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	id := r.PathValue("id")
	var body struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}

	if err := h.store.UpdateDescription(r.Context(), id, body.Description); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found")
			return
		}
		h.logger.Error("update description", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update transaction")
		return
	}

	txn, err := h.store.GetTransaction(r.Context(), id)
	if err != nil || txn == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
		return
	}
	writeJSON(w, http.StatusOK, txn)
}

// HandleAddLabels handles POST /api/transactions/{id}/labels.
// Body: {"labels": ["food", "recurring"]}
func (h *Handlers) HandleAddLabels(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	id := r.PathValue("id")
	var body struct {
		Labels []string `json:"labels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}

	for _, label := range body.Labels {
		if err := h.store.AddLabel(r.Context(), id, label); err != nil {
			h.logger.Error("add label", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to add label")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
}

// HandleRemoveLabel handles DELETE /api/transactions/{id}/labels/{label}.
func (h *Handlers) HandleRemoveLabel(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	id := r.PathValue("id")
	label := r.PathValue("label")

	if err := h.store.RemoveLabel(r.Context(), id, label); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "label not found on transaction")
			return
		}
		h.logger.Error("remove label", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to remove label")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// HandleGetChartData handles GET /api/stats/charts.
func (h *Handlers) HandleGetChartData(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	cd, err := h.store.GetChartData(r.Context())
	if err != nil {
		h.logger.Error("get chart data", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch chart data")
		return
	}
	writeJSON(w, http.StatusOK, cd)
}

// HandleSearchTransactions handles GET /api/transactions/search?q=...
func (h *Handlers) HandleSearchTransactions(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	q := r.URL.Query().Get("q")
	f := store.ListFilter{
		Page:     queryInt(r, "page", 1),
		PageSize: queryInt(r, "page_size", 20),
	}

	txns, total, err := h.store.SearchTransactions(r.Context(), q, f)
	if err != nil {
		h.logger.Error("search transactions", "error", err)
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"transactions": txns,
		"total":        total,
		"page":         f.Page,
		"page_size":    f.PageSize,
		"query":        q,
	})
}

// --- helpers ---

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return def
	}
	return n
}

// generateState creates a cryptographically random OAuth state token.
func generateState(readerName string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating OAuth state: %w", err)
	}
	return fmt.Sprintf("reader:%s:%s", readerName, hex.EncodeToString(b)), nil
}
