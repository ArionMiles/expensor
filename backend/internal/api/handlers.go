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
	oauthStateTTL      = 10 * time.Minute
)

// oauthStateEntry holds a pending OAuth state with an expiry time.
type oauthStateEntry struct {
	readerName string
	expiresAt  time.Time
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
	registry     *plugins.Registry
	store        Storer
	daemon       DaemonStatusProvider
	baseURL      string // e.g. "http://localhost:8080"
	frontendURL  string // e.g. "http://localhost:5173" — used for OAuth redirects
	dataDir      string
	baseCurrency string
	scanInterval int                 // default scan interval in seconds
	lookbackDays int                 // default lookback in days
	startFn      func(reader string) // called by POST /api/daemon/start; may be nil
	logger       *slog.Logger

	// oauthStates maps state token → entry for in-flight OAuth flows.
	mu          sync.Mutex
	oauthStates map[string]oauthStateEntry
}

// NewHandlers creates a Handlers instance.
// Pass nil for st when no database is configured; transaction endpoints will return 503.
// Pass nil for startFn to disable the daemon start endpoint.
func NewHandlers( //nolint:revive // dependency injection requires all these parameters; callers use named fields
	registry *plugins.Registry,
	st Storer,
	daemon DaemonStatusProvider,
	baseURL string,
	frontendURL string,
	dataDir string,
	baseCurrency string,
	scanInterval int,
	lookbackDays int,
	startFn func(reader string),
	logger *slog.Logger,
) *Handlers {
	if frontendURL == "" {
		frontendURL = baseURL
	}
	if dataDir == "" {
		dataDir = "data"
	}
	if baseCurrency == "" {
		baseCurrency = "INR"
	}
	if scanInterval <= 0 {
		scanInterval = 60
	}
	if lookbackDays <= 0 {
		lookbackDays = 180
	}
	return &Handlers{
		registry:     registry,
		store:        st,
		daemon:       daemon,
		baseURL:      strings.TrimRight(baseURL, "/"),
		frontendURL:  strings.TrimRight(frontendURL, "/"),
		dataDir:      dataDir,
		baseCurrency: baseCurrency,
		scanInterval: scanInterval,
		lookbackDays: lookbackDays,
		startFn:      startFn,
		logger:       logger,
		oauthStates:  make(map[string]oauthStateEntry),
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
		currency := h.baseCurrency
		if dbCurrency, err := h.store.GetAppConfig(r.Context(), "base_currency"); err == nil && dbCurrency != "" {
			currency = dbCurrency
		}
		if stats, err := h.store.GetStats(r.Context(), currency); err == nil {
			resp.Stats = stats
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

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
			`invalid credentials file: expected a Google OAuth2 client_secret.json with a "web" or "installed"`+
				` top-level key — download it from Google Cloud Console → APIs & Services → Credentials → OAuth 2.0 Client IDs`)
		return
	}

	dest := filepath.Join(h.dataDir, fmt.Sprintf("client_secret_%s.json", name))
	if err := os.MkdirAll(h.dataDir, 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create data directory")
		return
	}
	if err := os.WriteFile(dest, body, 0o600); err != nil { //nolint:gosec // dest is built from validated reader name and configured data dir
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

	_, err := os.Stat(filepath.Join(h.dataDir, fmt.Sprintf("client_secret_%s.json", name))) //nolint:gosec // validated reader name
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

	credFile := filepath.Join(h.dataDir, fmt.Sprintf("client_secret_%s.json", name))
	h.logger.Debug("reading credentials file", "reader", name, "path", credFile)
	secretJSON, err := os.ReadFile(credFile) //nolint:gosec // path built from validated reader name and configured data dir
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
	authURL := oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
	h.logger.Debug("OAuth URL generated", "reader", name, "state", state)
	writeJSON(w, http.StatusOK, map[string]string{"url": authURL})
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

	credFile := filepath.Join(h.dataDir, fmt.Sprintf("client_secret_%s.json", name))
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

	if err := client.SaveToken(filepath.Join(h.dataDir, fmt.Sprintf("token_%s.json", name)), tok); err != nil {
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

	tok, err := client.TokenFromFile(filepath.Join(h.dataDir, fmt.Sprintf("token_%s.json", name)))
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
		filepath.Join(h.dataDir, fmt.Sprintf("client_secret_%s.json", name)),
		filepath.Join(h.dataDir, fmt.Sprintf("token_%s.json", name)),
		filepath.Join(h.dataDir, fmt.Sprintf("config_%s.json", name)),
	}

	var removed []string
	for _, f := range files {
		if err := os.Remove(f); err == nil { //nolint:gosec // paths built from validated reader name and configured data dir
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

	tokenFile := filepath.Join(h.dataDir, fmt.Sprintf("token_%s.json", name))
	if err := os.Remove(tokenFile); err != nil { //nolint:gosec // path built from validated reader name and configured data dir
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

	cfgFile := filepath.Join(h.dataDir, fmt.Sprintf("config_%s.json", name))
	data, err := os.ReadFile(cfgFile) //nolint:gosec // path built from validated reader name and configured data dir
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
	_, _ = w.Write(data) //nolint:gosec // data is JSON read from a known config file; Content-Type is already set to application/json
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

	cfgFile := filepath.Join(h.dataDir, fmt.Sprintf("config_%s.json", name))
	if err := os.MkdirAll(h.dataDir, 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create data directory")
		return
	}
	if err := os.WriteFile(cfgFile, body, 0o600); err != nil { //nolint:gosec // path built from validated reader name and configured data dir
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
		_, err := os.Stat(filepath.Join(h.dataDir, fmt.Sprintf("client_secret_%s.json", name))) //nolint:gosec // validated reader name
		st.CredentialsUploaded = err == nil
	} else {
		st.CredentialsUploaded = true
	}

	if plugin.AuthType() == plugins.AuthTypeOAuth {
		tok, err := client.TokenFromFile(filepath.Join(h.dataDir, fmt.Sprintf("token_%s.json", name)))
		st.Authenticated = err == nil && tok.Valid()
	} else {
		st.Authenticated = true
	}

	cfgFile := filepath.Join(h.dataDir, fmt.Sprintf("config_%s.json", name))
	_, err = os.Stat(cfgFile) //nolint:gosec // path built from validated reader name and configured data dir
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
		Source:   r.URL.Query().Get("source"),
		Label:    r.URL.Query().Get("label"),
	}
	if v := r.URL.Query().Get("date_from"); v != "" {
		// JavaScript toISOString() includes milliseconds (RFC3339Nano); try that first.
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			f.From = &t
		} else if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = &t
		}
	}
	if v := r.URL.Query().Get("date_to"); v != "" {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			f.To = &t
		} else if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = &t
		}
	}
	f.SortBy = r.URL.Query().Get("sort_by")
	f.SortDir = r.URL.Query().Get("sort_dir")

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

// validateCategory checks that the given category name exists in the store.
// Returns false and writes an error response if validation fails.
func (h *Handlers) validateCategory(w http.ResponseWriter, r *http.Request, name string) bool {
	cats, err := h.store.ListCategories(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to validate category")
		return false
	}
	for _, c := range cats {
		if c.Name == name {
			return true
		}
	}
	writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("category %q does not exist", name))
	return false
}

// validateBucket checks that the given bucket name exists in the store.
// Returns false and writes an error response if validation fails.
func (h *Handlers) validateBucket(w http.ResponseWriter, r *http.Request, name string) bool {
	bkts, err := h.store.ListBuckets(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to validate bucket")
		return false
	}
	for _, b := range bkts {
		if b.Name == name {
			return true
		}
	}
	writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("bucket %q does not exist", name))
	return false
}

// HandleUpdateTransaction handles PUT /api/transactions/{id}.
// Body: {"description": "...", "category": "...", "bucket": "..."}
// All fields are optional; only non-nil fields are written.
func (h *Handlers) HandleUpdateTransaction(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	id := r.PathValue("id")
	var body struct {
		Description *string `json:"description"`
		Category    *string `json:"category"`
		Bucket      *string `json:"bucket"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}

	if body.Category != nil && *body.Category != "" && !h.validateCategory(w, r, *body.Category) {
		return
	}
	if body.Bucket != nil && *body.Bucket != "" && !h.validateBucket(w, r, *body.Bucket) {
		return
	}

	u := store.TransactionUpdate{
		Description: body.Description,
		Category:    body.Category,
		Bucket:      body.Bucket,
	}
	if err := h.store.UpdateTransaction(r.Context(), id, u); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found")
			return
		}
		h.logger.Error("update transaction", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update transaction")
		return
	}

	tx, err := h.store.GetTransaction(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch updated transaction")
		return
	}
	writeJSON(w, http.StatusOK, tx)
}

// HandleListLabels handles GET /api/config/labels.
func (h *Handlers) HandleListLabels(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	labels, err := h.store.ListLabels(r.Context())
	if err != nil {
		h.logger.Error("list labels", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list labels")
		return
	}
	writeJSON(w, http.StatusOK, labels)
}

// HandleCreateLabel handles POST /api/config/labels.
// Body: {"name": "food", "color": "#f59e0b"}
func (h *Handlers) HandleCreateLabel(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must be {\"name\": \"<name>\", \"color\": \"<hex>\"}")
		return
	}
	if body.Color == "" {
		body.Color = "#6366f1"
	}
	if err := h.store.CreateLabel(r.Context(), body.Name, body.Color); err != nil {
		h.logger.Error("create label", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create label")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": body.Name, "color": body.Color})
}

// HandleUpdateLabel handles PUT /api/config/labels/{name}.
// Body: {"color": "#f59e0b"}
func (h *Handlers) HandleUpdateLabel(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	var body struct {
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Color == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must be {\"color\": \"<hex>\"}")
		return
	}
	if err := h.store.UpdateLabel(r.Context(), name, body.Color); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "label not found")
			return
		}
		h.logger.Error("update label", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update label")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name, "color": body.Color})
}

// HandleDeleteLabel handles DELETE /api/config/labels/{name}.
func (h *Handlers) HandleDeleteLabel(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	if err := h.store.DeleteLabel(r.Context(), name); err != nil {
		h.logger.Error("delete label", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete label")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleApplyLabel handles POST /api/config/labels/{name}/apply.
// Body: {"merchant_pattern": "swiggy"}
func (h *Handlers) HandleApplyLabel(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	var body struct {
		MerchantPattern string `json:"merchant_pattern"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.MerchantPattern == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must be {\"merchant_pattern\": \"<pattern>\"}")
		return
	}
	affected, err := h.store.ApplyLabelByMerchant(r.Context(), name, body.MerchantPattern)
	if err != nil {
		h.logger.Error("apply label", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to apply label")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applied": affected})
}

// HandleListCategories handles GET /api/config/categories.
func (h *Handlers) HandleListCategories(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	cats, err := h.store.ListCategories(r.Context())
	if err != nil {
		h.logger.Error("list categories", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list categories")
		return
	}
	writeJSON(w, http.StatusOK, cats)
}

// HandleCreateCategory handles POST /api/config/categories.
func (h *Handlers) HandleCreateCategory(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must include \"name\"")
		return
	}
	if err := h.store.CreateCategory(r.Context(), body.Name, body.Description); err != nil {
		h.logger.Error("create category", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create category")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": body.Name})
}

// HandleDeleteCategory handles DELETE /api/config/categories/{name}.
func (h *Handlers) HandleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	if err := h.store.DeleteCategory(r.Context(), name); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "category not found")
			return
		}
		// Default categories return a plain error string.
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleListBuckets handles GET /api/config/buckets.
func (h *Handlers) HandleListBuckets(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	bkts, err := h.store.ListBuckets(r.Context())
	if err != nil {
		h.logger.Error("list buckets", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list buckets")
		return
	}
	writeJSON(w, http.StatusOK, bkts)
}

// HandleCreateBucket handles POST /api/config/buckets.
func (h *Handlers) HandleCreateBucket(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must include \"name\"")
		return
	}
	if err := h.store.CreateBucket(r.Context(), body.Name, body.Description); err != nil {
		h.logger.Error("create bucket", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create bucket")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": body.Name})
}

// HandleDeleteBucket handles DELETE /api/config/buckets/{name}.
func (h *Handlers) HandleDeleteBucket(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	if err := h.store.DeleteBucket(r.Context(), name); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "bucket not found")
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

	if err := h.store.AddLabels(r.Context(), id, body.Labels); err != nil {
		h.logger.Error("add labels", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to add labels")
		return
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

// HandleGetFacets handles GET /api/transactions/facets.
// Returns distinct values for source, category, currency, and label — used to
// populate filter dropdowns in the UI.
func (h *Handlers) HandleGetFacets(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	facets, err := h.store.GetFacets(r.Context())
	if err != nil {
		h.logger.Error("get facets", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch facets")
		return
	}
	writeJSON(w, http.StatusOK, facets)
}

// HandleGetBaseCurrency handles GET /api/config/base-currency.
func (h *Handlers) HandleGetBaseCurrency(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	currency := h.baseCurrency
	if dbVal, err := h.store.GetAppConfig(r.Context(), "base_currency"); err == nil && dbVal != "" {
		currency = dbVal
	}
	writeJSON(w, http.StatusOK, map[string]string{"base_currency": currency})
}

// HandleSetBaseCurrency handles PUT /api/config/base-currency.
// Body: {"base_currency": "USD"}
func (h *Handlers) HandleSetBaseCurrency(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		BaseCurrency string `json:"base_currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}
	currency := strings.ToUpper(strings.TrimSpace(body.BaseCurrency))
	if len(currency) != 3 {
		writeError(w, http.StatusBadRequest, "base_currency must be a 3-letter ISO 4217 code (e.g. INR, USD)")
		return
	}
	for _, c := range currency {
		if c < 'A' || c > 'Z' {
			writeError(w, http.StatusBadRequest, "base_currency must be a 3-letter ISO 4217 code (e.g. INR, USD)")
			return
		}
	}
	if err := h.store.SetAppConfig(r.Context(), "base_currency", currency); err != nil {
		h.logger.Error("set base currency", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update base currency")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"base_currency": currency})
}

// HandleGetScanInterval handles GET /api/config/scan-interval.
func (h *Handlers) HandleGetScanInterval(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	val := strconv.Itoa(h.scanInterval)
	if dbVal, err := h.store.GetAppConfig(r.Context(), "scan_interval"); err == nil && dbVal != "" {
		val = dbVal
	}
	writeJSON(w, http.StatusOK, map[string]string{"scan_interval": val})
}

// HandleSetScanInterval handles PUT /api/config/scan-interval.
// Body: {"scan_interval": "120"}
func (h *Handlers) HandleSetScanInterval(w http.ResponseWriter, r *http.Request) { //nolint:dupl // same shape as SetLookbackDays; different key and bounds
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		ScanInterval string `json:"scan_interval"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ScanInterval == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must be {\"scan_interval\": \"<seconds>\"}")
		return
	}
	n, err := strconv.Atoi(body.ScanInterval)
	if err != nil || n < 10 || n > 3600 {
		writeError(w, http.StatusBadRequest, "scan_interval must be an integer between 10 and 3600 seconds")
		return
	}
	if err := h.store.SetAppConfig(r.Context(), "scan_interval", body.ScanInterval); err != nil {
		h.logger.Error("set scan interval", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update scan interval")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"scan_interval": body.ScanInterval})
}

// HandleGetLookbackDays handles GET /api/config/lookback-days.
func (h *Handlers) HandleGetLookbackDays(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	val := strconv.Itoa(h.lookbackDays)
	if dbVal, err := h.store.GetAppConfig(r.Context(), "lookback_days"); err == nil && dbVal != "" {
		val = dbVal
	}
	writeJSON(w, http.StatusOK, map[string]string{"lookback_days": val})
}

// HandleSetLookbackDays handles PUT /api/config/lookback-days.
// Body: {"lookback_days": "365"}
func (h *Handlers) HandleSetLookbackDays(w http.ResponseWriter, r *http.Request) { //nolint:dupl // same shape as SetScanInterval; different key and bounds
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		LookbackDays string `json:"lookback_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.LookbackDays == "" {
		writeError(w, http.StatusUnprocessableEntity, "body must be {\"lookback_days\": \"<days>\"}")
		return
	}
	n, err := strconv.Atoi(body.LookbackDays)
	if err != nil || n < 1 || n > 3650 {
		writeError(w, http.StatusBadRequest, "lookback_days must be an integer between 1 and 3650")
		return
	}
	if err := h.store.SetAppConfig(r.Context(), "lookback_days", body.LookbackDays); err != nil {
		h.logger.Error("set lookback days", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update lookback days")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"lookback_days": body.LookbackDays})
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
