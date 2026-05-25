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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	pkgapi "github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/client"
	tbreader "github.com/ArionMiles/expensor/backend/pkg/reader/thunderbird"
	pkgrules "github.com/ArionMiles/expensor/backend/pkg/rules"
)

const (
	defaultBaseCurrency = "INR"
	queryValueTrue      = "true"
	maxCredentialsSize  = 5 << 20 // 5 MB
	oauthStateTTL       = 10 * time.Minute
	maxPageParam        = 10000
	maxPageSizeParam    = 500
)

var (
	errCredentialsMissing  = errors.New("credentials file missing")
	errReaderNotRegistered = errors.New("reader no longer registered")
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
	registry           *plugins.Registry
	store              Storer
	daemon             DaemonStatusProvider
	version            string // set at build time via ldflags
	baseURL            string // e.g. "http://localhost:8080"
	frontendURL        string // e.g. "http://localhost:5173" — used for OAuth redirects
	dataDir            string
	thunderbirdDataDir string
	scanInterval       int                 // default scan interval in seconds
	lookbackDays       int                 // default lookback in days
	startFn            func(reader string) // called by POST /api/daemon/start; may be nil
	rescanFn           func(reader string) // called by POST /api/daemon/rescan; may be nil
	restartFn          func(reader string) // called after checkpoint clear to reload from DB; may be nil
	syncFn             func()              // called by POST /api/config/sync; may be nil
	banksData          []byte
	logger             *slog.Logger

	// oauthStates maps state token → entry for in-flight OAuth flows.
	mu          sync.Mutex
	oauthStates map[string]oauthStateEntry
}

// HandlersConfig holds all dependencies for NewHandlers.
type HandlersConfig struct {
	Registry           *plugins.Registry
	Store              Storer
	Daemon             DaemonStatusProvider
	Version            string
	BaseURL            string
	FrontendURL        string
	DataDir            string
	ThunderbirdDataDir string
	ScanInterval       int
	LookbackDays       int
	StartFn            func(reader string)
	RescanFn           func(reader string)
	RestartFn          func(reader string)
	SyncFn             func()
	BanksData          []byte
	Logger             *slog.Logger
}

// NewHandlers creates a Handlers instance.
// Pass nil Store when no database is configured; transaction endpoints will return 503.
// Pass nil StartFn to disable the daemon start endpoint.
func NewHandlers(cfg HandlersConfig) *Handlers {
	if cfg.FrontendURL == "" {
		cfg.FrontendURL = cfg.BaseURL
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "data"
	}
	if cfg.ScanInterval <= 0 {
		cfg.ScanInterval = 60
	}
	if cfg.LookbackDays <= 0 {
		cfg.LookbackDays = 180
	}
	return &Handlers{
		registry:           cfg.Registry,
		store:              cfg.Store,
		daemon:             cfg.Daemon,
		version:            cfg.Version,
		baseURL:            strings.TrimRight(cfg.BaseURL, "/"),
		frontendURL:        strings.TrimRight(cfg.FrontendURL, "/"),
		dataDir:            cfg.DataDir,
		thunderbirdDataDir: cfg.ThunderbirdDataDir,
		scanInterval:       cfg.ScanInterval,
		lookbackDays:       cfg.LookbackDays,
		startFn:            cfg.StartFn,
		rescanFn:           cfg.RescanFn,
		restartFn:          cfg.RestartFn,
		syncFn:             cfg.SyncFn,
		banksData:          cfg.BanksData,
		logger:             cfg.Logger,
		oauthStates:        make(map[string]oauthStateEntry),
	}
}

// --- helpers ---

// getBaseCurrency returns the base currency from the DB, falling back to INR.
func (h *Handlers) getBaseCurrency(ctx context.Context) string {
	if h.store != nil {
		if val, err := h.store.GetAppConfig(ctx, "base_currency"); err == nil && val != "" {
			return val
		}
	}
	return defaultBaseCurrency
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// DocErrorResponse is the standard JSON error payload for OpenAPI generation.
type DocErrorResponse struct {
	Error string `json:"error" example:"database not connected"`
}

// DocHealthResponse is the health check payload.
type DocHealthResponse struct {
	Status string `json:"status" example:"ok"`
}

// DocVersionResponse is the version payload.
type DocVersionResponse struct {
	Version string `json:"version" example:"dev"`
}

// DocStatsResponse documents the stats payload embedded in status responses.
type DocStatsResponse struct {
	TotalCount         int                `json:"total_count"`
	TotalBase          float64            `json:"total_base"`
	BaseCurrency       string             `json:"base_currency" example:"INR"`
	TotalByCategory    map[string]float64 `json:"total_by_category"`
	TotalCategoryCount map[string]int     `json:"total_category_count"`
}

// DocStatusResponse documents the combined daemon and stats status payload.
type DocStatusResponse struct {
	Daemon DaemonStatus      `json:"daemon"`
	Stats  *DocStatsResponse `json:"stats,omitempty"`
}

// DocDaemonReaderRequest is the daemon start/rescan request body.
type DocDaemonReaderRequest struct {
	Reader string `json:"reader" example:"gmail"`
}

// DocStatusOnlyResponse is a simple status message payload.
type DocStatusOnlyResponse struct {
	Status string `json:"status" example:"ok"`
}

// DocActiveReaderResponse is the active reader config payload.
type DocActiveReaderResponse struct {
	Reader string `json:"reader" example:"gmail"`
}

// DocBaseCurrencyRequest is the base currency update payload.
type DocBaseCurrencyRequest struct {
	BaseCurrency string `json:"base_currency" example:"USD"`
}

// DocBaseCurrencyResponse is the base currency payload.
type DocBaseCurrencyResponse struct {
	BaseCurrency string `json:"base_currency" example:"USD"`
}

// DocScanIntervalRequest is the scan interval update payload.
type DocScanIntervalRequest struct {
	ScanInterval string `json:"scan_interval" example:"120"`
}

// DocScanIntervalResponse is the scan interval payload.
type DocScanIntervalResponse struct {
	ScanInterval string `json:"scan_interval" example:"120"`
}

// DocLookbackDaysRequest is the lookback days update payload.
type DocLookbackDaysRequest struct {
	LookbackDays string `json:"lookback_days" example:"365"`
}

// DocLookbackDaysResponse is the lookback days payload.
type DocLookbackDaysResponse struct {
	LookbackDays string `json:"lookback_days" example:"365"`
}

// DocTimezoneRequest is the timezone update payload.
type DocTimezoneRequest struct {
	Timezone string `json:"timezone" example:"Asia/Kolkata"`
}

// DocTimezoneResponse is the timezone payload.
type DocTimezoneResponse struct {
	Timezone string `json:"timezone" example:"Asia/Kolkata"`
}

// DocTimeFormatRequest is the time format update payload.
type DocTimeFormatRequest struct {
	TimeFormat string `json:"time_format" example:"HH:mm"`
}

// DocTimeFormatResponse is the time format payload.
type DocTimeFormatResponse struct {
	TimeFormat string `json:"time_format" example:"HH:mm"`
}

// DocSetupStatusResponse is the first-run setup status payload.
type DocSetupStatusResponse struct {
	Required bool     `json:"required" example:"true"`
	Missing  []string `json:"missing" example:"base_currency,timezone,time_format"`
}

// DocReaderCheckpointResponse is the reader checkpoint payload.
type DocReaderCheckpointResponse struct {
	LastScanAt *string `json:"last_scan_at" example:"2026-04-14T09:00:00Z" extensions:"x-nullable"`
}

// DocSyncStatusResponse is the community sync status payload.
type DocSyncStatusResponse struct {
	LastSyncedAt   *time.Time `json:"last_synced_at,omitempty" extensions:"x-nullable"`
	Error          *string    `json:"error,omitempty" extensions:"x-nullable"`
	EntriesUpdated int        `json:"entries_updated"`
}

// DocLabelResponse documents a managed label.
type DocLabelResponse struct {
	Name      string    `json:"name" example:"food"`
	Color     string    `json:"color" example:"#f59e0b"`
	CreatedAt time.Time `json:"created_at"`
}

// DocCreateLabelRequest is the label creation payload.
type DocCreateLabelRequest struct {
	Name  string `json:"name" example:"food"`
	Color string `json:"color" example:"#f59e0b"`
}

// DocUpdateLabelRequest is the label update payload.
type DocUpdateLabelRequest struct {
	Color string `json:"color" example:"#f59e0b"`
}

// DocLabelMutationResponse is the label create/update response payload.
type DocLabelMutationResponse struct {
	Name  string `json:"name" example:"food"`
	Color string `json:"color" example:"#f59e0b"`
}

// DocApplyLabelRequest is the label-by-merchant apply payload.
type DocApplyLabelRequest struct {
	MerchantPattern string `json:"merchant_pattern" example:"swiggy"`
}

// DocAppliedCountResponse is the count payload for apply actions.
type DocAppliedCountResponse struct {
	Applied int64 `json:"applied"`
}

// DocLabelMappingsResponse documents label-to-merchant mappings.
type DocLabelMappingsResponse map[string][]string

// DocCategoryResponse documents a managed category.
type DocCategoryResponse struct {
	Name        string `json:"name" example:"Food"`
	Description string `json:"description,omitempty" example:"Restaurants and groceries"`
	IsDefault   bool   `json:"is_default"`
}

// DocCreateCategoryRequest is the category creation payload.
type DocCreateCategoryRequest struct {
	Name        string `json:"name" example:"Food"`
	Description string `json:"description,omitempty" example:"Restaurants and groceries"`
}

// DocBucketResponse documents a managed bucket.
type DocBucketResponse struct {
	Name        string `json:"name" example:"Needs"`
	Description string `json:"description,omitempty" example:"Essential spending"`
	IsDefault   bool   `json:"is_default"`
}

// DocCreateBucketRequest is the bucket creation payload.
type DocCreateBucketRequest struct {
	Name        string `json:"name" example:"Needs"`
	Description string `json:"description,omitempty" example:"Essential spending"`
}

// DocNameResponse is a simple named resource payload.
type DocNameResponse struct {
	Name string `json:"name" example:"Food"`
}

// DocBankColorResponse documents an embedded bank color mapping.
type DocBankColorResponse struct {
	Fragment string `json:"fragment" example:"hdfc"`
	Color    string `json:"color" example:"#2563eb"`
	Name     string `json:"name" example:"HDFC Bank"`
}

// DocTransactionResponse documents a transaction payload.
type DocTransactionResponse struct {
	ID               string    `json:"id" example:"tx_123"`
	MessageID        string    `json:"message_id" example:"gmail-message-id"`
	Amount           float64   `json:"amount" example:"249.50"`
	Currency         string    `json:"currency" example:"INR"`
	OriginalAmount   *float64  `json:"original_amount,omitempty"`
	OriginalCurrency *string   `json:"original_currency,omitempty"`
	ExchangeRate     *float64  `json:"exchange_rate,omitempty"`
	Timestamp        time.Time `json:"timestamp"`
	MerchantInfo     string    `json:"merchant_info" example:"Swiggy"`
	Category         string    `json:"category" example:"Food"`
	Bucket           string    `json:"bucket" example:"Needs"`
	Source           string    `json:"source" example:"gmail"`
	Description      string    `json:"description" example:"Dinner order"`
	Labels           []string  `json:"labels"`
	Muted            bool      `json:"muted"`
	MutedByMerchant  bool      `json:"muted_by_merchant"`
	MuteReason       string    `json:"mute_reason,omitempty" example:"Internal transfer"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// DocTransactionsListResponse documents the paginated list payload.
type DocTransactionsListResponse struct {
	Transactions []DocTransactionResponse `json:"transactions"`
	Total        int                      `json:"total"`
	TotalAmount  float64                  `json:"total_amount"`
	BaseCurrency string                   `json:"base_currency" example:"INR"`
	Page         int                      `json:"page"`
	PageSize     int                      `json:"page_size"`
}

// DocTransactionsSearchResponse documents the paginated search payload.
type DocTransactionsSearchResponse struct {
	Transactions []DocTransactionResponse `json:"transactions"`
	Total        int                      `json:"total"`
	TotalAmount  float64                  `json:"total_amount"`
	BaseCurrency string                   `json:"base_currency" example:"INR"`
	Page         int                      `json:"page"`
	PageSize     int                      `json:"page_size"`
	Query        string                   `json:"query" example:"coffee"`
}

// DocExtractionDiagnosticResponse documents an extraction diagnostic payload.
type DocExtractionDiagnosticResponse struct {
	ID             string     `json:"id" example:"11111111-1111-1111-1111-111111111111"`
	Status         string     `json:"status" example:"open"`
	Reader         string     `json:"reader" example:"gmail"`
	MessageID      string     `json:"message_id" example:"gmail-message-id"`
	Source         string     `json:"source" example:"HDFC Bank"`
	Sender         string     `json:"sender" example:"HDFC Bank"`
	SenderEmail    string     `json:"sender_email" example:"alerts@hdfcbank.net"`
	Subject        string     `json:"subject" example:"Transaction alert"`
	EmailBody      string     `json:"email_body" example:"Your card was charged INR 249.50 at Swiggy"`
	ReceivedAt     *time.Time `json:"received_at,omitempty"`
	Snippet        string     `json:"snippet" example:"Your card was charged INR 249.50 at Swiggy"`
	RuleID         *string    `json:"rule_id,omitempty" example:"22222222-2222-2222-2222-222222222222"`
	RuleName       string     `json:"rule_name" example:"HDFC credit card"`
	AmountRegex    string     `json:"amount_regex" example:"INR\\s+([0-9,.]+)"`
	MerchantRegex  string     `json:"merchant_regex" example:"at\\s+(.+)$"`
	CurrencyRegex  string     `json:"currency_regex" example:"(INR)"`
	FailureReasons []string   `json:"failure_reasons" example:"amount_not_found"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

// DocExtractionDiagnosticStatusRequest is the diagnostic status update payload.
type DocExtractionDiagnosticStatusRequest struct {
	Status string `json:"status" example:"resolved"`
}

// DocFacetsResponse documents the distinct transaction filter values.
type DocFacetsResponse struct {
	Sources    []string `json:"sources"`
	Categories []string `json:"categories"`
	Currencies []string `json:"currencies"`
	Merchants  []string `json:"merchants"`
	Labels     []string `json:"labels"`
	Buckets    []string `json:"buckets"`
}

// DocTransactionUpdateRequest is the transaction patch payload.
type DocTransactionUpdateRequest struct {
	Description *string `json:"description,omitempty" example:"Dinner order"`
	Category    *string `json:"category,omitempty" example:"Food"`
	Bucket      *string `json:"bucket,omitempty" example:"Needs"`
}

// DocTransactionLabelsRequest is the transaction labels mutation payload.
type DocTransactionLabelsRequest struct {
	Labels []string `json:"labels"`
}

// DocMuteTransactionRequest is the transaction mute payload.
type DocMuteTransactionRequest struct {
	Muted  bool   `json:"muted"`
	Reason string `json:"reason,omitempty" example:"Internal transfer"`
}

// DocMuteTransactionResponse is the transaction mute response payload.
type DocMuteTransactionResponse struct {
	Muted  bool   `json:"muted"`
	Reason string `json:"reason,omitempty" example:"Internal transfer"`
}

// DocUpdateMuteReasonRequest is the mute reason update payload.
type DocUpdateMuteReasonRequest struct {
	Reason string `json:"reason" example:"Internal transfer"`
}

// DocMuteReasonResponse is the mute reason response payload.
type DocMuteReasonResponse struct {
	Reason string `json:"reason" example:"Internal transfer"`
}

// --- health & status ---

// HandleHealth handles GET /api/health.
// @Summary Health check
// @Tags Bootstrap
// @Produce json
// @Success 200 {object} DocHealthResponse
// @Router /health [get]
func (h *Handlers) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleVersion handles GET /api/version.
// @Summary Get backend version
// @Tags Bootstrap
// @Produce json
// @Success 200 {object} DocVersionResponse
// @Router /version [get]
func (h *Handlers) HandleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": h.version})
}

// HandleStatus handles GET /api/status.
// @Summary Get daemon and stats status
// @Tags Bootstrap
// @Produce json
// @Success 200 {object} DocStatusResponse
// @Router /status [get]
func (h *Handlers) HandleStatus(w http.ResponseWriter, r *http.Request) {
	ds := h.daemon.Status()

	type statusResponse struct {
		Daemon DaemonStatus `json:"daemon"`
		Stats  *store.Stats `json:"stats,omitempty"`
	}
	resp := statusResponse{Daemon: ds}

	if h.store != nil {
		currency := h.getBaseCurrency(r.Context())
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

// HandleListWriters handles GET /api/plugins/writers.
func (h *Handlers) HandleListWriters(w http.ResponseWriter, _ *http.Request) {
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

// HandleUploadCredentials handles POST /api/readers/{name}/credentials.
// Accepts a JSON file upload (e.g. Google client_secret.json) and saves it to the runtime store.
func (h *Handlers) HandleUploadCredentials(w http.ResponseWriter, r *http.Request) {
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

// HandleCredentialsStatus handles GET /api/readers/{name}/credentials/status.
func (h *Handlers) HandleCredentialsStatus(w http.ResponseWriter, r *http.Request) {
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

// HandleAuthStart handles POST /api/readers/{name}/auth/start.
// Returns a Google OAuth consent URL for the given reader.
func (h *Handlers) HandleAuthStart(w http.ResponseWriter, r *http.Request) {
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

// HandleAuthExchange handles POST /api/readers/{name}/auth/exchange.
// Accepts the full redirect URL (containing code and state params) pasted by
// the user when the automatic redirect is unreachable (e.g. homeserver without
// a public domain). Validates state, exchanges the code, and saves the token.
func (h *Handlers) HandleAuthExchange(w http.ResponseWriter, r *http.Request) {
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

// HandleAuthStatus handles GET /api/readers/{name}/auth/status.
func (h *Handlers) HandleAuthStatus(w http.ResponseWriter, r *http.Request) {
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

// HandleDisconnectReader handles DELETE /api/readers/{name}.
// Removes all stored credentials, token, and config files for the reader.
func (h *Handlers) HandleDisconnectReader(w http.ResponseWriter, r *http.Request) {
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

// HandleRevokeToken handles DELETE /api/readers/{name}/auth/token.
func (h *Handlers) HandleRevokeToken(w http.ResponseWriter, r *http.Request) {
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

// HandleGetReaderConfig handles GET /api/readers/{name}/config.
func (h *Handlers) HandleGetReaderConfig(w http.ResponseWriter, r *http.Request) {
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

// --- daemon control ---

// HandleStartDaemon handles POST /api/daemon/start.
// Body: {"reader": "gmail"}
// Triggers the background daemon with the given reader if it is not already running.
// @Summary Start the daemon
// @Tags Bootstrap
// @Accept json
// @Produce json
// @Param request body DocDaemonReaderRequest true "Daemon start request"
// @Success 200 {object} DocStatusOnlyResponse "Daemon already running"
// @Success 202 {object} DocStatusOnlyResponse "Daemon starting"
// @Failure 400 {object} DocErrorResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 501 {object} DocErrorResponse
// @Router /daemon/start [post]
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

// HandleRescan handles POST /api/daemon/rescan.
// Body: {"reader": "<name>"}
// Stops any running daemon and restarts with forceRescan=true, bypassing the
// checkpoint and state deduplication so the full lookback window is scanned.
// @Summary Trigger a full rescan
// @Tags Bootstrap
// @Accept json
// @Produce json
// @Param request body DocDaemonReaderRequest true "Daemon rescan request"
// @Success 202 {object} DocStatusOnlyResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 501 {object} DocErrorResponse
// @Router /daemon/rescan [post]
func (h *Handlers) HandleRescan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Reader string `json:"reader"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Reader == "" {
		writeError(w, http.StatusBadRequest, `body must be {"reader": "<name>"}`)
		return
	}
	if _, err := h.registry.GetReader(body.Reader); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("reader %q not found", body.Reader))
		return
	}

	if h.rescanFn == nil {
		writeError(w, http.StatusNotImplemented, "rescan not configured")
		return
	}
	h.rescanFn(body.Reader)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "rescanning"})
}

// HandleGetActiveReader handles GET /api/config/active-reader.
// Returns the reader name persisted from the last daemon start, or "" if none.
// @Summary Get the active reader
// @Tags Config
// @Produce json
// @Success 200 {object} DocActiveReaderResponse
// @Failure 500 {object} DocErrorResponse
// @Router /config/active-reader [get]
func (h *Handlers) HandleGetActiveReader(w http.ResponseWriter, r *http.Request) {
	reader, err := h.readActiveReader(r.Context())
	if err != nil {
		h.logger.Error("failed to read active reader", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to read active reader")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"reader": reader})
}

// --- transactions ---

// HandleListTransactions handles GET /api/transactions.
// @Summary List transactions
// @Tags Transactions
// @Produce json
// @Param page query int false "1-based page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param merchant query string false "Merchant filter"
// @Param category query string false "Category filter"
// @Param category_missing query int false "Only transactions without a category when set to 1" Enums(1)
// @Param exclude_categories query string false "Comma-separated categories to exclude"
// @Param currency query string false "Currency filter"
// @Param source query string false "Source filter"
// @Param exclude_sources query string false "Comma-separated sources to exclude"
// @Param label query string false "Label filter"
// @Param label_missing query int false "Only transactions without labels when set to 1" Enums(1)
// @Param exclude_labels query string false "Comma-separated labels to exclude"
// @Param bucket query string false "Bucket filter"
// @Param bucket_missing query int false "Only transactions without a bucket when set to 1" Enums(1)
// @Param exclude_buckets query string false "Comma-separated buckets to exclude"
// @Param date_from query string false "RFC3339 start timestamp"
// @Param date_to query string false "RFC3339 end timestamp"
// @Param show_muted query int false "Include muted transactions when set to 1" Enums(1)
// @Param muted_only query int false "Return only muted transactions when set to 1" Enums(1)
// @Param weekday query int false "PostgreSQL DOW weekday filter (0=Sunday...6=Saturday)" Enums(0,1,2,3,4,5,6)
// @Param hour_from query int false "Minimum hour filter (0-23)"
// @Param hour_to query int false "Maximum hour filter (0-23)"
// @Param tz query string false "IANA timezone used for weekday/hour filters"
// @Param sort_dir query string false "Sort direction" Enums(asc,desc)
// @Success 200 {object} DocTransactionsListResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /transactions [get]
func (h *Handlers) HandleListTransactions(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	if invalidKey, ok := firstInvalidFilterParam(
		r,
		"merchant",
		"category",
		"currency",
		"source",
		"source_type",
		"bank",
		"label",
		"bucket",
		"exclude_categories",
		"exclude_sources",
		"exclude_source_types",
		"exclude_banks",
		"exclude_labels",
		"exclude_buckets",
	); ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid %s filter", invalidKey))
		return
	}

	f := store.ListFilter{
		Page:               queryInt(r, "page", 1),
		PageSize:           queryInt(r, "page_size", 20),
		Merchant:           r.URL.Query().Get("merchant"),
		Category:           r.URL.Query().Get("category"),
		CategoryMissing:    r.URL.Query().Get("category_missing") == "1",
		ExcludeCategories:  queryCSV(r, "exclude_categories"),
		Currency:           r.URL.Query().Get("currency"),
		Source:             r.URL.Query().Get("source"),
		ExcludeSources:     queryCSV(r, "exclude_sources"),
		SourceType:         r.URL.Query().Get("source_type"),
		ExcludeSourceTypes: queryCSV(r, "exclude_source_types"),
		Bank:               r.URL.Query().Get("bank"),
		ExcludeBanks:       queryCSV(r, "exclude_banks"),
		Label:              r.URL.Query().Get("label"),
		ExcludeLabels:      queryCSV(r, "exclude_labels"),
		Bucket:             r.URL.Query().Get("bucket"),
		BucketMissing:      r.URL.Query().Get("bucket_missing") == "1",
		ExcludeBuckets:     queryCSV(r, "exclude_buckets"),
		LabelMissing:       r.URL.Query().Get("label_missing") == "1",
		ShowMuted:          r.URL.Query().Get("show_muted") == "1",
		MutedOnly:          r.URL.Query().Get("muted_only") == "1",
		IndividualOnly:     r.URL.Query().Get("individual_only") == "1",
		Weekday:            queryWeekday(r, "weekday"),
		HourFrom:           queryHour(r, "hour_from"),
		HourTo:             queryHour(r, "hour_to"),
		Timezone:           h.resolveTimezone(r.Context(), r.URL.Query().Get("tz")),
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

	txns, result, err := h.store.ListTransactions(r.Context(), f)
	if err != nil {
		h.logger.Error("list transactions", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list transactions")
		return
	}
	if txns == nil {
		txns = []store.Transaction{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"transactions":  txns,
		"total":         result.Total,
		"total_amount":  result.TotalAmount,
		"base_currency": h.getBaseCurrency(r.Context()),
		"page":          f.Page,
		"page_size":     f.PageSize,
	})
}

func queryCSV(r *http.Request, key string) []string {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func firstInvalidFilterParam(r *http.Request, keys ...string) (string, bool) {
	for _, key := range keys {
		if containsControlChars(r.URL.Query().Get(key)) {
			return key, true
		}
	}
	return "", false
}

func containsControlChars(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// HandleGetTransaction handles GET /api/transactions/{id}.
// @Summary Get a transaction
// @Tags Transactions
// @Produce json
// @Param id path string true "Transaction ID"
// @Success 200 {object} DocTransactionResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 404 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /transactions/{id} [get]
func (h *Handlers) HandleGetTransaction(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid transaction id")
		return
	}
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
// @Summary Update a transaction
// @Tags Transactions
// @Accept json
// @Produce json
// @Param id path string true "Transaction ID"
// @Param request body DocTransactionUpdateRequest true "Transaction update payload"
// @Success 200 {object} DocTransactionResponse
// @Failure 404 {object} DocErrorResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /transactions/{id} [put]
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

// HandleListExtractionDiagnostics handles GET /api/extraction-diagnostics.
// @Summary List extraction diagnostics
// @Tags Extraction Diagnostics
// @Produce json
// @Param status query string false "Diagnostic status filter"
// @Param limit query int false "Maximum rows to return"
// @Success 200 {array} DocExtractionDiagnosticResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /extraction-diagnostics [get]
func (h *Handlers) HandleListExtractionDiagnostics(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	status := r.URL.Query().Get("status")
	if status == "" {
		status = store.DiagnosticStatusOpen
	}
	if err := store.ValidateDiagnosticFilterStatus(status); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid diagnostic status")
		return
	}

	filter := store.DiagnosticFilter{Status: status}
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		limit, err := strconv.Atoi(rawLimit)
		if err != nil || limit <= 0 {
			writeError(w, http.StatusUnprocessableEntity, "invalid limit")
			return
		}
		filter.Limit = limit
	}

	rows, err := h.store.ListExtractionDiagnostics(r.Context(), filter)
	if err != nil {
		h.logger.Error("list extraction diagnostics", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list extraction diagnostics")
		return
	}
	if rows == nil {
		rows = []store.ExtractionDiagnosticRow{}
	}
	writeJSON(w, http.StatusOK, rows)
}

// HandleGetExtractionDiagnostic handles GET /api/extraction-diagnostics/{id}.
// @Summary Get an extraction diagnostic
// @Tags Extraction Diagnostics
// @Produce json
// @Param id path string true "Diagnostic ID"
// @Success 200 {object} DocExtractionDiagnosticResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 404 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /extraction-diagnostics/{id} [get]
func (h *Handlers) HandleGetExtractionDiagnostic(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid extraction diagnostic id")
		return
	}

	row, err := h.store.GetExtractionDiagnostic(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "extraction diagnostic not found")
			return
		}
		h.logger.Error("get extraction diagnostic", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch extraction diagnostic")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// HandleUpdateExtractionDiagnosticStatus handles PUT /api/extraction-diagnostics/{id}/status.
// @Summary Update extraction diagnostic status
// @Tags Extraction Diagnostics
// @Accept json
// @Produce json
// @Param id path string true "Diagnostic ID"
// @Param request body DocExtractionDiagnosticStatusRequest true "Diagnostic status payload"
// @Success 200 {object} DocExtractionDiagnosticResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 404 {object} DocErrorResponse
// @Failure 409 {object} DocErrorResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /extraction-diagnostics/{id}/status [put]
func (h *Handlers) HandleUpdateExtractionDiagnosticStatus(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid extraction diagnostic id")
		return
	}

	var body DocExtractionDiagnosticStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}
	if !validDiagnosticUpdateStatus(body.Status) {
		writeError(w, http.StatusUnprocessableEntity, "invalid diagnostic status")
		return
	}

	row, err := h.store.UpdateExtractionDiagnosticStatus(r.Context(), id, body.Status)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "extraction diagnostic not found")
			return
		}
		if errors.Is(err, store.ErrDiagnosticConflict) {
			writeError(w, http.StatusConflict, "open extraction diagnostic already exists")
			return
		}
		h.logger.Error("update extraction diagnostic status", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update extraction diagnostic")
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func validDiagnosticUpdateStatus(status string) bool {
	switch status {
	case store.DiagnosticStatusOpen, store.DiagnosticStatusResolved, store.DiagnosticStatusIgnored:
		return true
	default:
		return false
	}
}

// HandleListLabels handles GET /api/config/labels.
// @Summary List labels
// @Tags Taxonomy
// @Produce json
// @Success 200 {array} DocLabelResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/labels [get]
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
// @Summary Create a label
// @Tags Taxonomy
// @Accept json
// @Produce json
// @Param request body DocCreateLabelRequest true "Label payload"
// @Success 201 {object} DocLabelMutationResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/labels [post]
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
// @Summary Update a label
// @Tags Taxonomy
// @Accept json
// @Produce json
// @Param name path string true "Label name"
// @Param request body DocUpdateLabelRequest true "Label color payload"
// @Success 200 {object} DocLabelMutationResponse
// @Failure 404 {object} DocErrorResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/labels/{name} [put]
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
// Body: {"remove_from_transactions": true}
// @Summary Delete a label
// @Tags Taxonomy
// @Produce json
// @Param name path string true "Label name"
// @Success 204 "No Content"
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/labels/{name} [delete]
func (h *Handlers) HandleDeleteLabel(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	removeFromTransactions, ok := taxonomyCleanupFlag(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteLabel(r.Context(), name, removeFromTransactions); err != nil {
		h.logger.Error("delete label", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete label")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleApplyLabel handles POST /api/config/labels/{name}/apply.
// Body: {"merchant_pattern": "swiggy"}
// @Summary Apply a label by merchant pattern
// @Tags Taxonomy
// @Accept json
// @Produce json
// @Param name path string true "Label name"
// @Param request body DocApplyLabelRequest true "Merchant pattern payload"
// @Success 200 {object} DocAppliedCountResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/labels/{name}/apply [post]
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

// HandleRemoveLabelByMerchant handles DELETE /api/config/labels/{name}/merchant.
// Body: {"merchant_pattern": "swiggy"}
func (h *Handlers) HandleRemoveLabelByMerchant(w http.ResponseWriter, r *http.Request) {
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
	removed, err := h.store.RemoveLabelByMerchant(r.Context(), name, body.MerchantPattern)
	if err != nil {
		h.logger.Error("remove label by merchant", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to remove label")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
}

// HandleGetLabelMonthlySpend handles GET /api/stats/labels/monthly.
// Query params:
//   - dimension=labels|categories|buckets (default: labels)
//
// Response: {"labels":["Food","Travel"], "months":["2025-05","2025-06",...], "series":[{"label":"Food","data":[...]}]}
func (h *Handlers) HandleGetLabelMonthlySpend(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	dimension := strings.TrimSpace(r.URL.Query().Get("dimension"))
	if dimension == "" {
		dimension = "labels"
	}
	switch dimension {
	case "labels", "categories", "buckets":
	default:
		writeError(w, http.StatusBadRequest, "invalid dimension")
		return
	}

	data, err := h.store.GetMonthlyBreakdownSpend(r.Context(), dimension, 12)
	if err != nil {
		h.logger.Error("get monthly breakdown spend", "dimension", dimension, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch monthly breakdown spend")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// HandleGetLabelMappings handles GET /api/config/labels/mappings.
// Returns a map of label → persisted merchant patterns.
// @Summary Get label mappings
// @Tags Taxonomy
// @Produce json
// @Success 200 {object} DocLabelMappingsResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/labels/mappings [get]
func (h *Handlers) HandleGetLabelMappings(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	mappings, err := h.store.GetLabelMappings(r.Context())
	if err != nil {
		h.logger.Error("get label mappings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get label mappings")
		return
	}
	writeJSON(w, http.StatusOK, mappings)
}

// HandleExportLabels handles GET /api/config/labels/export.
// Returns labels with their persisted merchant mappings as a downloadable JSON file.
func (h *Handlers) HandleExportLabels(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	labels, err := h.store.ListLabels(r.Context())
	if err != nil {
		h.logger.Error("export labels", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list labels")
		return
	}
	mappings, err := h.store.GetLabelMappings(r.Context())
	if err != nil {
		h.logger.Error("export label mappings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list label mappings")
		return
	}
	type exportRow struct {
		Name      string   `json:"name"`
		Color     string   `json:"color"`
		Merchants []string `json:"merchants,omitempty"`
	}
	export := make([]exportRow, 0, len(labels))
	for _, l := range labels {
		export = append(export, exportRow{
			Name:      l.Name,
			Color:     l.Color,
			Merchants: mappings[l.Name],
		})
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="expensor-labels.json"`)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(export)
}

// HandleExportCategories handles GET /api/config/categories/export.
// Returns categories with their persisted merchant mappings as a downloadable JSON file.
func (h *Handlers) HandleExportCategories(w http.ResponseWriter, r *http.Request) {
	handleExportNamedTaxonomy(w, r, taxonomyExportConfig[store.Category]{
		handlers:    h,
		singular:    "category",
		plural:      "categories",
		filename:    "expensor-categories.json",
		list:        func(ctx context.Context) ([]store.Category, error) { return h.store.ListCategories(ctx) },
		getMappings: h.store.GetCategoryMappings,
		nameOf:      func(item store.Category) string { return item.Name },
	})
}

// HandleGetCategoryMappings handles GET /api/config/categories/mappings.
func (h *Handlers) HandleGetCategoryMappings(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	mappings, err := h.store.GetCategoryMappings(r.Context())
	if err != nil {
		h.logger.Error("get category mappings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get category mappings")
		return
	}
	writeJSON(w, http.StatusOK, mappings)
}

// HandleListCategories handles GET /api/config/categories.
// @Summary List categories
// @Tags Taxonomy
// @Produce json
// @Success 200 {array} DocCategoryResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/categories [get]
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
// @Summary Create a category
// @Tags Taxonomy
// @Accept json
// @Produce json
// @Param request body DocCreateCategoryRequest true "Category payload"
// @Success 201 {object} DocNameResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/categories [post]
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
// @Summary Delete a category
// @Tags Taxonomy
// @Produce json
// @Param name path string true "Category name"
// @Success 204 "No Content"
// @Failure 404 {object} DocErrorResponse
// @Failure 409 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/categories/{name} [delete]
func (h *Handlers) HandleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	removeFromTransactions, ok := taxonomyCleanupFlag(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteCategory(r.Context(), name, removeFromTransactions); err != nil {
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

// HandleApplyCategoryByMerchant handles POST /api/config/categories/{name}/apply.
func (h *Handlers) HandleApplyCategoryByMerchant(w http.ResponseWriter, r *http.Request) {
	h.handleApplyTaxonomyMerchant(w, r, "category", h.store.ApplyCategoryByMerchant)
}

// HandleRemoveCategoryByMerchant handles DELETE /api/config/categories/{name}/merchant.
func (h *Handlers) HandleRemoveCategoryByMerchant(w http.ResponseWriter, r *http.Request) {
	h.handleRemoveTaxonomyMerchant(w, r, "category", h.store.RemoveCategoryByMerchant)
}

// HandleListBuckets handles GET /api/config/buckets.
// @Summary List buckets
// @Tags Taxonomy
// @Produce json
// @Success 200 {array} DocBucketResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/buckets [get]
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
// @Summary Create a bucket
// @Tags Taxonomy
// @Accept json
// @Produce json
// @Param request body DocCreateBucketRequest true "Bucket payload"
// @Success 201 {object} DocNameResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/buckets [post]
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
// @Summary Delete a bucket
// @Tags Taxonomy
// @Produce json
// @Param name path string true "Bucket name"
// @Success 204 "No Content"
// @Failure 404 {object} DocErrorResponse
// @Failure 409 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/buckets/{name} [delete]
func (h *Handlers) HandleDeleteBucket(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	removeFromTransactions, ok := taxonomyCleanupFlag(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteBucket(r.Context(), name, removeFromTransactions); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "bucket not found")
			return
		}
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleExportBuckets handles GET /api/config/buckets/export.
// Returns buckets with their persisted merchant mappings as a downloadable JSON file.
func (h *Handlers) HandleExportBuckets(w http.ResponseWriter, r *http.Request) {
	handleExportNamedTaxonomy(w, r, taxonomyExportConfig[store.Bucket]{
		handlers:    h,
		singular:    "bucket",
		plural:      "buckets",
		filename:    "expensor-buckets.json",
		list:        func(ctx context.Context) ([]store.Bucket, error) { return h.store.ListBuckets(ctx) },
		getMappings: h.store.GetBucketMappings,
		nameOf:      func(item store.Bucket) string { return item.Name },
	})
}

// HandleGetBucketMappings handles GET /api/config/buckets/mappings.
func (h *Handlers) HandleGetBucketMappings(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	mappings, err := h.store.GetBucketMappings(r.Context())
	if err != nil {
		h.logger.Error("get bucket mappings", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get bucket mappings")
		return
	}
	writeJSON(w, http.StatusOK, mappings)
}

// HandleApplyBucketByMerchant handles POST /api/config/buckets/{name}/apply.
func (h *Handlers) HandleApplyBucketByMerchant(w http.ResponseWriter, r *http.Request) {
	h.handleApplyTaxonomyMerchant(w, r, "bucket", h.store.ApplyBucketByMerchant)
}

// HandleRemoveBucketByMerchant handles DELETE /api/config/buckets/{name}/merchant.
func (h *Handlers) HandleRemoveBucketByMerchant(w http.ResponseWriter, r *http.Request) {
	h.handleRemoveTaxonomyMerchant(w, r, "bucket", h.store.RemoveBucketByMerchant)
}

func taxonomyCleanupFlag(w http.ResponseWriter, r *http.Request) (bool, bool) {
	var body struct {
		RemoveFromTransactions bool `json:"remove_from_transactions"`
	}
	removeFromTransactions := r.URL.Query().Get("remove_from_transactions") == queryValueTrue
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusUnprocessableEntity, "body must be {\"remove_from_transactions\": <bool>}")
			return false, false
		}
	}
	return removeFromTransactions || body.RemoveFromTransactions, true
}

type taxonomyExportRow struct {
	Name      string   `json:"name"`
	Merchants []string `json:"merchants,omitempty"`
}

type taxonomyExportConfig[T any] struct {
	handlers    *Handlers
	singular    string
	plural      string
	filename    string
	list        func(context.Context) ([]T, error)
	getMappings func(context.Context) (map[string][]string, error)
	nameOf      func(T) string
}

func handleExportNamedTaxonomy[T any](
	w http.ResponseWriter,
	r *http.Request,
	config taxonomyExportConfig[T],
) {
	h := config.handlers
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	items, err := config.list(r.Context())
	if err != nil {
		h.logger.Error("export taxonomy", "kind", config.singular, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list "+config.plural)
		return
	}
	mappings, err := config.getMappings(r.Context())
	if err != nil {
		h.logger.Error("export taxonomy mappings", "kind", config.singular, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list "+config.singular+" mappings")
		return
	}
	export := make([]taxonomyExportRow, 0, len(items))
	for _, item := range items {
		name := config.nameOf(item)
		export = append(export, taxonomyExportRow{Name: name, Merchants: mappings[name]})
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", config.filename))
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(export)
}

func (h *Handlers) handleApplyTaxonomyMerchant(
	w http.ResponseWriter,
	r *http.Request,
	kind string,
	apply func(context.Context, string, string) (int64, error),
) {
	h.handleTaxonomyMerchant(w, r, taxonomyMerchantAction{
		kind:        kind,
		action:      "apply",
		responseKey: "applied",
		update:      apply,
	})
}

func (h *Handlers) handleRemoveTaxonomyMerchant(
	w http.ResponseWriter,
	r *http.Request,
	kind string,
	remove func(context.Context, string, string) (int64, error),
) {
	h.handleTaxonomyMerchant(w, r, taxonomyMerchantAction{
		kind:        kind,
		action:      "remove",
		responseKey: "removed",
		update:      remove,
	})
}

type taxonomyMerchantAction struct {
	kind        string
	action      string
	responseKey string
	update      func(context.Context, string, string) (int64, error)
}

func (h *Handlers) handleTaxonomyMerchant(
	w http.ResponseWriter,
	r *http.Request,
	action taxonomyMerchantAction,
) {
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
	count, err := action.update(r.Context(), name, body.MerchantPattern)
	if err != nil {
		h.logger.Error(action.action+" taxonomy merchant", "kind", action.kind, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to "+action.action+" merchant")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{action.responseKey: count})
}

// HandleAddLabels handles POST /api/transactions/{id}/labels.
// Body: {"labels": ["food", "recurring"]}
// @Summary Add labels to a transaction
// @Tags Transactions
// @Accept json
// @Produce json
// @Param id path string true "Transaction ID"
// @Param request body DocTransactionLabelsRequest true "Labels payload"
// @Success 200 {object} DocStatusOnlyResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /transactions/{id}/labels [post]
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
// @Summary Remove a label from a transaction
// @Tags Transactions
// @Produce json
// @Param id path string true "Transaction ID"
// @Param label path string true "Label name"
// @Success 200 {object} DocStatusOnlyResponse
// @Failure 404 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /transactions/{id}/labels/{label} [delete]
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

// --- muted transactions ---

// HandleMuteTransaction handles PUT /api/transactions/{id}/mute.
// Body: {"muted": true|false}
// @Summary Mute or unmute a transaction
// @Tags Transactions
// @Accept json
// @Produce json
// @Param id path string true "Transaction ID"
// @Param request body DocMuteTransactionRequest true "Mute payload"
// @Success 200 {object} DocMuteTransactionResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 404 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /transactions/{id}/mute [put]
func (h *Handlers) HandleMuteTransaction(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	id := r.PathValue("id")
	var body struct {
		Muted  bool   `json:"muted"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.store.MuteTransaction(r.Context(), id, body.Muted, body.Reason); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found")
			return
		}
		h.logger.Error("mute transaction", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update transaction")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"muted": body.Muted, "reason": body.Reason})
}

// HandleUpdateMuteReason handles PUT /api/transactions/{id}/mute-reason.
// Body: {"reason": "optional text"}
//
// @Summary Update a transaction mute reason
// @Tags Transactions
// @Accept json
// @Produce json
// @Param id path string true "Transaction ID"
// @Param request body DocUpdateMuteReasonRequest true "Mute reason payload"
// @Success 200 {object} DocMuteReasonResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 404 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /transactions/{id}/mute-reason [put]
//
//nolint:dupl // structurally similar to HandleUpdateMerchantReason but calls a different store method
func (h *Handlers) HandleUpdateMuteReason(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	id := r.PathValue("id")
	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.store.UpdateMuteReason(r.Context(), id, body.Reason); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "transaction not found or not muted")
			return
		}
		h.logger.Error("update mute reason", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update reason")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"reason": body.Reason})
}

// HandleListMutedMerchants handles GET /api/muted-merchants.
func (h *Handlers) HandleListMutedMerchants(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	merchants, err := h.store.GetMutedMerchantsWithCount(r.Context())
	if err != nil {
		h.logger.Error("list muted merchants", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list muted merchants")
		return
	}
	writeJSON(w, http.StatusOK, merchants)
}

// HandleMuteByMerchant handles POST /api/muted-merchants.
// Body: {"pattern": "MERCHANT NAME", "reason": "optional"}
func (h *Handlers) HandleMuteByMerchant(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		Pattern string `json:"pattern"`
		Reason  string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Pattern == "" {
		writeError(w, http.StatusBadRequest, "request body must be JSON with a non-empty \"pattern\" field")
		return
	}
	if err := h.store.MuteByMerchant(r.Context(), body.Pattern, body.Reason); err != nil {
		h.logger.Error("mute by merchant", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to mute merchant")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"pattern": body.Pattern})
}

// HandleUpdateMerchantReason handles PUT /api/muted-merchants/{id}/reason.
// Body: {"reason": "optional text"}
//
//nolint:dupl // structurally similar to HandleUpdateMuteReason but calls a different store method
func (h *Handlers) HandleUpdateMerchantReason(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	id := r.PathValue("id")
	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := h.store.UpdateMerchantReason(r.Context(), id, body.Reason); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "muted merchant not found")
			return
		}
		h.logger.Error("update merchant reason", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update reason")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"reason": body.Reason})
}

// HandleDeleteMutedMerchant handles DELETE /api/muted-merchants/{id}.
// Optional query param: ?unmute=true — atomically deletes the rule and
// sets muted=false on all existing transactions that matched the pattern.
func (h *Handlers) HandleDeleteMutedMerchant(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	id := r.PathValue("id")

	var err error
	if r.URL.Query().Get("unmute") == queryValueTrue {
		err = h.store.DeleteMutedMerchantAndUnmute(r.Context(), id)
	} else {
		err = h.store.DeleteMutedMerchant(r.Context(), id)
	}

	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "muted merchant not found")
			return
		}
		h.logger.Error("delete muted merchant", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete muted merchant")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleCategorizeMerchant handles POST /api/merchants/categorize.
// Body: {"merchant": "Name", "category": "Cat", "bucket": "Bucket"}
// Response: {"updated": N}
func (h *Handlers) HandleCategorizeMerchant(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		Merchant string `json:"merchant"`
		Category string `json:"category"`
		Bucket   string `json:"bucket"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "request body must be valid JSON")
		return
	}
	if body.Merchant == "" {
		writeError(w, http.StatusBadRequest, "\"merchant\" must not be empty")
		return
	}
	n, err := h.store.CategorizeMerchant(r.Context(), body.Merchant, body.Category, body.Bucket)
	if err != nil {
		h.logger.Error("categorize merchant", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to categorize merchant")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"updated": n})
}

// --- rules ---

type ruleHTTPJSON struct {
	ID                string        `json:"id,omitempty"`
	Name              string        `json:"name"`
	SenderEmail       string        `json:"sender_email,omitempty"`
	SenderEmails      []string      `json:"sender_emails"`
	SubjectContains   string        `json:"subject_contains"`
	AmountRegex       string        `json:"amount_regex"`
	MerchantRegex     string        `json:"merchant_regex"`
	CurrencyRegex     string        `json:"currency_regex"`
	TransactionSource string        `json:"transaction_source,omitempty"`
	SourceType        string        `json:"source_type,omitempty"`
	SourceLabel       string        `json:"source_label,omitempty"`
	Bank              string        `json:"bank,omitempty"`
	Source            pkgapi.Source `json:"source"`
	Predefined        bool          `json:"predefined"`
	CreatedAt         time.Time     `json:"created_at,omitempty"`
	UpdatedAt         time.Time     `json:"updated_at,omitempty"`
}

type ruleDocumentJSON struct {
	Version int                 `json:"version"`
	Presets pkgrules.Presets    `json:"presets"`
	Rules   []ruleDocumentEntry `json:"rules"`
}

type ruleDocumentEntry struct {
	Name            string        `json:"name"`
	SenderEmails    []string      `json:"sender_emails"`
	SubjectContains string        `json:"subject_contains"`
	AmountRegex     string        `json:"amount_regex"`
	MerchantRegex   string        `json:"merchant_regex"`
	CurrencyRegex   string        `json:"currency_regex"`
	Source          pkgapi.Source `json:"source"`
}

func ruleRowToHTTP(row store.RuleRow) ruleHTTPJSON {
	source := pkgapi.Source{Type: row.SourceType, Label: row.SourceLabel, Bank: row.Bank}
	return ruleHTTPJSON{
		ID:                row.ID,
		Name:              row.Name,
		SenderEmail:       row.SenderEmail,
		SenderEmails:      normalizedHTTPSenders(row.SenderEmails, row.SenderEmail),
		SubjectContains:   row.SubjectContains,
		AmountRegex:       row.AmountRegex,
		MerchantRegex:     row.MerchantRegex,
		CurrencyRegex:     row.CurrencyRegex,
		TransactionSource: row.TransactionSource,
		SourceType:        row.SourceType,
		SourceLabel:       row.SourceLabel,
		Bank:              row.Bank,
		Source:            source,
		Predefined:        row.Predefined,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}
}

func ruleRowsToHTTP(rows []store.RuleRow) []ruleHTTPJSON {
	out := make([]ruleHTTPJSON, 0, len(rows))
	for _, row := range rows {
		out = append(out, ruleRowToHTTP(row))
	}
	return out
}

func ruleHTTPToRow(body ruleHTTPJSON) store.RuleRow {
	source := body.Source
	if source.Type == "" {
		source.Type = body.SourceType
	}
	if source.Label == "" {
		source.Label = body.SourceLabel
	}
	if source.Bank == "" {
		source.Bank = body.Bank
	}
	senders := normalizedHTTPSenders(body.SenderEmails, body.SenderEmail)
	row := store.RuleRow{
		Name:              strings.TrimSpace(body.Name),
		SenderEmail:       "",
		SenderEmails:      senders,
		SubjectContains:   strings.TrimSpace(body.SubjectContains),
		AmountRegex:       strings.TrimSpace(body.AmountRegex),
		MerchantRegex:     strings.TrimSpace(body.MerchantRegex),
		CurrencyRegex:     strings.TrimSpace(body.CurrencyRegex),
		TransactionSource: strings.TrimSpace(body.TransactionSource),
		SourceType:        strings.TrimSpace(source.Type),
		SourceLabel:       strings.TrimSpace(source.Label),
		Bank:              strings.TrimSpace(source.Bank),
	}
	if len(senders) > 0 {
		row.SenderEmail = senders[0]
	}
	if row.TransactionSource == "" {
		row.TransactionSource = pkgapi.Source{Type: row.SourceType, Label: row.SourceLabel, Bank: row.Bank}.Display()
	}
	return row
}

func normalizedHTTPSenders(senders []string, fallback string) []string {
	seen := make(map[string]struct{}, len(senders)+1)
	out := make([]string, 0, len(senders)+1)
	for _, value := range append(senders, fallback) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func ruleToRow(rule pkgapi.Rule) store.RuleRow {
	row := store.RuleRow{
		Name:            rule.Name,
		SenderEmail:     rule.SenderEmail,
		SenderEmails:    normalizedHTTPSenders(rule.SenderEmails, rule.SenderEmail),
		SubjectContains: rule.SubjectContains,
		SourceType:      rule.Source.Type,
		SourceLabel:     rule.Source.Label,
		Bank:            rule.Source.Bank,
	}
	if row.SenderEmail == "" && len(row.SenderEmails) > 0 {
		row.SenderEmail = row.SenderEmails[0]
	}
	if rule.Amount != nil {
		row.AmountRegex = rule.Amount.String()
	}
	if rule.MerchantInfo != nil {
		row.MerchantRegex = rule.MerchantInfo.String()
	}
	if rule.Currency != nil {
		row.CurrencyRegex = rule.Currency.String()
	}
	row.TransactionSource = rule.Source.Display()
	return row
}

func ruleDocumentEntryFromRow(row store.RuleRow) ruleDocumentEntry {
	return ruleDocumentEntry{
		Name:            row.Name,
		SenderEmails:    normalizedHTTPSenders(row.SenderEmails, row.SenderEmail),
		SubjectContains: row.SubjectContains,
		AmountRegex:     row.AmountRegex,
		MerchantRegex:   row.MerchantRegex,
		CurrencyRegex:   row.CurrencyRegex,
		Source:          pkgapi.Source{Type: row.SourceType, Label: row.SourceLabel, Bank: row.Bank},
	}
}

func ruleDocumentPresets(entries []ruleDocumentEntry) pkgrules.Presets {
	return pkgrules.Presets{
		SourceTypes: presetValuesFromRules(entries, func(source pkgapi.Source) string { return source.Type }),
		Banks:       presetValuesFromRules(entries, func(source pkgapi.Source) string { return source.Bank }),
	}
}

func presetValuesFromRules(entries []ruleDocumentEntry, value func(pkgapi.Source) string) []pkgrules.PresetValue {
	seen := map[string]struct{}{}
	out := []pkgrules.PresetValue{}
	for _, entry := range entries {
		v := strings.TrimSpace(value(entry.Source))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, pkgrules.PresetValue{Value: v, Origin: "custom"})
	}
	return out
}

// validateRuleRegexes compiles the three regex fields on a RuleRow and returns the first error.
// An empty pattern is skipped (optional fields are allowed to be unset for updates).
func validateRuleRegexes(amountRegex, merchantRegex, currencyRegex string) error {
	if amountRegex != "" {
		if _, err := regexp.Compile(amountRegex); err != nil {
			return fmt.Errorf("invalid amount_regex: %w", err)
		}
	}
	if merchantRegex != "" {
		if _, err := regexp.Compile(merchantRegex); err != nil {
			return fmt.Errorf("invalid merchant_regex: %w", err)
		}
	}
	if currencyRegex != "" {
		if _, err := regexp.Compile(currencyRegex); err != nil {
			return fmt.Errorf("invalid currency_regex: %w", err)
		}
	}
	return nil
}

func validateRuleRow(row store.RuleRow) error {
	if row.Name == "" {
		return errors.New("name is required")
	}
	if len(row.SenderEmails) == 0 {
		return errors.New("sender_emails is required")
	}
	if row.AmountRegex == "" {
		return errors.New("amount_regex is required")
	}
	if row.MerchantRegex == "" {
		return errors.New("merchant_regex is required")
	}
	if row.SourceType == "" {
		return errors.New("source.type is required")
	}
	if row.Bank == "" {
		return errors.New("source.bank is required")
	}
	return validateRuleRegexes(row.AmountRegex, row.MerchantRegex, row.CurrencyRegex)
}

// HandleListRules handles GET /api/rules.
func (h *Handlers) HandleListRules(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	rules, err := h.store.ListRules(r.Context())
	if err != nil {
		h.logger.Error("list rules", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list rules")
		return
	}
	writeJSON(w, http.StatusOK, ruleRowsToHTTP(rules))
}

// HandleCreateRule handles POST /api/rules.
func (h *Handlers) HandleCreateRule(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body ruleHTTPJSON
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}
	row := ruleHTTPToRow(body)
	if err := validateRuleRow(row); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	created, err := h.store.CreateRule(r.Context(), row)
	if err != nil {
		if errors.Is(err, store.ErrRuleNameConflict) {
			writeError(w, http.StatusConflict, "rule name already exists")
			return
		}
		h.logger.Error("create rule", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create rule")
		return
	}
	h.clearActiveReaderCheckpointForNewRule(r.Context())
	writeJSON(w, http.StatusCreated, ruleRowToHTTP(*created))
}

func (h *Handlers) clearActiveReaderCheckpointForNewRule(ctx context.Context) {
	reader, err := h.readActiveReader(ctx)
	if err != nil || strings.TrimSpace(reader) == "" || h.store == nil {
		return
	}
	reader = strings.TrimSpace(reader)
	if err := h.store.SetAppConfig(ctx, "reader."+reader+".last_scan_at", ""); err != nil {
		h.logger.Warn("failed to clear checkpoint after rule creation", "reader", reader, "error", err)
		return
	}
	if h.daemon.Status().Running && h.restartFn != nil {
		h.restartFn(reader)
	}
}

func (h *Handlers) readActiveReader(ctx context.Context) (string, error) {
	if h.store == nil {
		return "", nil
	}
	return h.store.GetActiveReader(ctx)
}

// HandleUpdateRule handles PUT /api/rules/{id}.
// All rules (predefined and user-created) are fully editable.
func (h *Handlers) HandleUpdateRule(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	id := r.PathValue("id")
	var body ruleHTTPJSON
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}
	row := ruleHTTPToRow(body)
	if err := validateRuleRow(row); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	updated, err := h.store.UpdateRule(r.Context(), id, row)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		if errors.Is(err, store.ErrRuleNameConflict) {
			writeError(w, http.StatusConflict, "rule name already exists")
			return
		}
		h.logger.Error("update rule", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update rule")
		return
	}
	writeJSON(w, http.StatusOK, ruleRowToHTTP(*updated))
}

// HandleDeleteRule handles DELETE /api/rules/{id}.
// Returns 403 for system rules.
func (h *Handlers) HandleDeleteRule(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	id := r.PathValue("id")
	existing, err := h.store.GetRule(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch rule")
		return
	}
	if existing.Predefined {
		writeError(w, http.StatusForbidden, "predefined rules cannot be deleted")
		return
	}
	if err := h.store.DeleteRule(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		h.logger.Error("delete rule", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete rule")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleExportRules handles GET /api/rules/export.
// Downloads all user rules as a JSON file in rules.json format.
func (h *Handlers) HandleExportRules(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	all, err := h.store.ListRules(r.Context())
	if err != nil {
		h.logger.Error("export rules", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch rules")
		return
	}
	export := make([]ruleDocumentEntry, 0)
	for _, row := range all {
		if row.Predefined {
			continue // export only user-created rules
		}
		export = append(export, ruleDocumentEntryFromRow(row))
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="expensor-rules.json"`)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ruleDocumentJSON{Version: 2, Presets: ruleDocumentPresets(export), Rules: export})
}

// HandleImportRules handles POST /api/rules/import.
// Validates all rules first; rejects the entire import if any rule fails.
func (h *Handlers) HandleImportRules(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}
	doc, err := pkgrules.ParseDocument(body)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	rows := make([]store.RuleRow, 0, len(doc.Rules))
	for _, rule := range doc.Rules {
		rows = append(rows, ruleToRow(rule))
	}
	if err := h.store.ImportUserRules(r.Context(), rows); err != nil {
		h.logger.Error("import rules", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to import rules")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"imported": len(rows)})
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

// HandleGetDashboardData handles GET /api/stats/dashboard.
func (h *Handlers) HandleGetDashboardData(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	data, err := h.store.GetDashboardData(r.Context())
	if err != nil {
		h.logger.Error("get dashboard data", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch dashboard data")
		return
	}

	writeJSON(w, http.StatusOK, data)
}

// HandleDiscoverProfiles handles GET /api/readers/thunderbird/discover/profiles.
// Returns discovered Thunderbird profile directories from platform paths,
// the Docker mount /thunderbird-profile, and THUNDERBIRD_DATA_DIR env var.
func (h *Handlers) HandleDiscoverProfiles(w http.ResponseWriter, _ *http.Request) {
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

// HandleDiscoverMailboxes handles GET /api/readers/thunderbird/discover/mailboxes?profile=<path>.
// Returns available MBOX mailbox names within the given Thunderbird profile directory.
func (h *Handlers) HandleDiscoverMailboxes(w http.ResponseWriter, r *http.Request) {
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

// HandleGetReaderGuide handles GET /api/readers/{name}/guide.
// Returns the structured setup guide for a reader when metadata includes one.
func (h *Handlers) HandleGetReaderGuide(w http.ResponseWriter, r *http.Request) {
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

// HandleGetHeatmap handles GET /api/stats/heatmap.
// Optional query params: from=<RFC3339>, to=<RFC3339> (both or neither).
// Returns 400 if either param is present but malformed.
func (h *Handlers) HandleGetHeatmap(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	from, to, err := parseHeatmapRange(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	data, err := h.store.GetSpendingHeatmap(r.Context(), from, to)
	if err != nil {
		h.logger.Error("get heatmap", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch heatmap data")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// HandleGetAnnualHeatmap handles GET /api/stats/heatmap/annual?year=YYYY.
// Returns per-day transaction totals for the requested calendar year.
// Defaults to the current year when ?year is absent or invalid.
func (h *Handlers) HandleGetAnnualHeatmap(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	yearStr := r.URL.Query().Get("year")
	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 1 {
		year = time.Now().Year()
	}

	buckets, err := h.store.GetAnnualSpend(r.Context(), year)
	if err != nil {
		h.logger.Error("get annual heatmap", "error", err, "year", year)
		writeError(w, http.StatusInternalServerError, "failed to fetch annual heatmap data")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"year":    year,
		"buckets": buckets,
	})
}

// parseHeatmapRange parses optional ?from= and ?to= RFC3339 query parameters.
// Returns nil, nil when neither is provided. Returns an error if either is
// present but cannot be parsed as RFC3339.
func parseHeatmapRange(r *http.Request) (from, to *time.Time, err error) {
	if v := r.URL.Query().Get("from"); v != "" {
		t, parseErr := time.Parse(time.RFC3339, v)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("invalid 'from' param: must be RFC3339 (e.g. 2026-04-01T00:00:00Z)")
		}
		from = &t
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, parseErr := time.Parse(time.RFC3339, v)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("invalid 'to' param: must be RFC3339 (e.g. 2026-04-30T23:59:59Z)")
		}
		to = &t
	}
	return from, to, nil
}

// HandleSearchTransactions handles GET /api/transactions/search?q=...
// @Summary Search transactions
// @Tags Transactions
// @Produce json
// @Param q query string true "Search query"
// @Param page query int false "1-based page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param show_muted query int false "Include muted transactions when set to 1" Enums(1)
// @Success 200 {object} DocTransactionsSearchResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /transactions/search [get]
func (h *Handlers) HandleSearchTransactions(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	q := r.URL.Query().Get("q")
	if containsControlChars(q) {
		writeError(w, http.StatusBadRequest, "invalid q filter")
		return
	}
	f := store.ListFilter{
		Page:           queryInt(r, "page", 1),
		PageSize:       queryInt(r, "page_size", 20),
		ShowMuted:      r.URL.Query().Get("show_muted") == "1",
		MutedOnly:      r.URL.Query().Get("muted_only") == "1",
		IndividualOnly: r.URL.Query().Get("individual_only") == "1",
	}

	txns, result, err := h.store.SearchTransactions(r.Context(), q, f)
	if err != nil {
		h.logger.Error("search transactions", "error", err)
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}
	if txns == nil {
		txns = []store.Transaction{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"transactions":  txns,
		"total":         result.Total,
		"total_amount":  result.TotalAmount,
		"base_currency": h.getBaseCurrency(r.Context()),
		"page":          f.Page,
		"page_size":     f.PageSize,
		"query":         q,
	})
}

// HandleGetFacets handles GET /api/transactions/facets.
// Returns distinct values for source, category, currency, and label — used to
// populate filter dropdowns in the UI.
// @Summary Get transaction facets
// @Tags Transactions
// @Produce json
// @Success 200 {object} DocFacetsResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /transactions/facets [get]
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

// HandleListBanks returns the embedded bank color mappings.
// GET /api/config/banks
// @Summary List bank color mappings
// @Tags Taxonomy
// @Produce json
// @Success 200 {array} DocBankColorResponse
// @Router /config/banks [get]
func (h *Handlers) HandleListBanks(w http.ResponseWriter, r *http.Request) {
	data := h.banksData
	if len(data) == 0 {
		data = []byte("[]")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		h.logger.Warn("failed to write banks response", "error", err)
	}
}

// HandleGetBaseCurrency handles GET /api/config/base-currency.
// @Summary Get the base currency
// @Tags Config
// @Produce json
// @Success 200 {object} DocBaseCurrencyResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/base-currency [get]
func (h *Handlers) HandleGetBaseCurrency(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"base_currency": h.getBaseCurrency(r.Context())})
}

// HandleSetBaseCurrency handles PUT /api/config/base-currency.
// Body: {"base_currency": "USD"}
// @Summary Set the base currency
// @Tags Config
// @Accept json
// @Produce json
// @Param request body DocBaseCurrencyRequest true "Base currency payload"
// @Success 200 {object} DocBaseCurrencyResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/base-currency [put]
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
// @Summary Get the scan interval
// @Tags Config
// @Produce json
// @Success 200 {object} DocScanIntervalResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/scan-interval [get]
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
// @Summary Set the scan interval
// @Tags Config
// @Accept json
// @Produce json
// @Param request body DocScanIntervalRequest true "Scan interval payload"
// @Success 200 {object} DocScanIntervalResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/scan-interval [put]
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
// @Summary Get lookback days
// @Tags Config
// @Produce json
// @Success 200 {object} DocLookbackDaysResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/lookback-days [get]
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
// @Summary Set lookback days
// @Tags Config
// @Accept json
// @Produce json
// @Param request body DocLookbackDaysRequest true "Lookback days payload"
// @Success 200 {object} DocLookbackDaysResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/lookback-days [put]
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

// validTimeFormats is the set of accepted time_format values.
var validTimeFormats = map[string]bool{
	"HH:mm":     true,
	"HH:mm:ss":  true,
	"h:mm a":    true,
	"h:mm:ss a": true,
}

// HandleGetTimezone handles GET /api/config/timezone.
// @Summary Get the application timezone
// @Tags Config
// @Produce json
// @Success 200 {object} DocTimezoneResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/timezone [get]
func (h *Handlers) HandleGetTimezone(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	tz := ""
	if dbVal, err := h.store.GetAppConfig(r.Context(), "app.timezone"); err == nil && dbVal != "" {
		tz = dbVal
	}
	writeJSON(w, http.StatusOK, map[string]string{"timezone": tz})
}

// HandleSetTimezone handles PUT /api/config/timezone.
// Body: {"timezone": "Asia/Kolkata"}
// @Summary Set the application timezone
// @Tags Config
// @Accept json
// @Produce json
// @Param request body DocTimezoneRequest true "Timezone payload"
// @Success 200 {object} DocTimezoneResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/timezone [put]
func (h *Handlers) HandleSetTimezone(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		Timezone string `json:"timezone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}
	tz := strings.TrimSpace(body.Timezone)
	if _, err := time.LoadLocation(tz); err != nil {
		writeError(w, http.StatusBadRequest, "invalid IANA timezone string")
		return
	}
	if err := h.store.SetAppConfig(r.Context(), "app.timezone", tz); err != nil {
		h.logger.Error("set timezone", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update timezone")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"timezone": tz})
}

// HandleGetTimeFormat handles GET /api/config/time-format.
// @Summary Get the time format
// @Tags Config
// @Produce json
// @Success 200 {object} DocTimeFormatResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/time-format [get]
func (h *Handlers) HandleGetTimeFormat(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	tf := "HH:mm"
	if dbVal, err := h.store.GetAppConfig(r.Context(), "app.time_format"); err == nil && dbVal != "" {
		tf = dbVal
	}
	writeJSON(w, http.StatusOK, map[string]string{"time_format": tf})
}

// HandleSetTimeFormat handles PUT /api/config/time-format.
// Body: {"time_format": "HH:mm"}
// @Summary Set the time format
// @Tags Config
// @Accept json
// @Produce json
// @Param request body DocTimeFormatRequest true "Time format payload"
// @Success 200 {object} DocTimeFormatResponse
// @Failure 400 {object} DocErrorResponse
// @Failure 422 {object} DocErrorResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/time-format [put]
func (h *Handlers) HandleSetTimeFormat(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	var body struct {
		TimeFormat string `json:"time_format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}
	tf := strings.TrimSpace(body.TimeFormat)
	if !validTimeFormats[tf] {
		writeError(w, http.StatusBadRequest, "invalid time_format; accepted: HH:mm, HH:mm:ss, h:mm a, h:mm:ss a")
		return
	}
	if err := h.store.SetAppConfig(r.Context(), "app.time_format", tf); err != nil {
		h.logger.Error("set time format", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update time format")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"time_format": tf})
}

func (h *Handlers) missingSetupPreferences(ctx context.Context) []string {
	required := []struct {
		key   string
		field string
	}{
		{key: "base_currency", field: "base_currency"},
		{key: "app.timezone", field: "timezone"},
		{key: "app.time_format", field: "time_format"},
	}
	missing := make([]string, 0, len(required))
	for _, pref := range required {
		value, err := h.store.GetAppConfig(ctx, pref.key)
		if err != nil || strings.TrimSpace(value) == "" {
			missing = append(missing, pref.field)
		}
	}
	return missing
}

// HandleGetSetupStatus handles GET /api/config/setup-status.
// @Summary Get first-run setup status
// @Tags Config
// @Produce json
// @Success 200 {object} DocSetupStatusResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/setup-status [get]
func (h *Handlers) HandleGetSetupStatus(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	missing := h.missingSetupPreferences(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"required": len(missing) > 0,
		"missing":  missing,
	})
}

// HandleGetReaderCheckpoint handles GET /api/config/readers/{name}/checkpoint.
// Returns the last scan timestamp for the reader (or null if no checkpoint exists).
// @Summary Get a reader checkpoint
// @Tags Config
// @Produce json
// @Param name path string true "Reader name"
// @Success 200 {object} DocReaderCheckpointResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/readers/{name}/checkpoint [get]
func (h *Handlers) HandleGetReaderCheckpoint(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	val, err := h.store.GetAppConfig(r.Context(), "reader."+name+".last_scan_at")
	if err != nil || val == "" {
		writeJSON(w, http.StatusOK, map[string]any{"last_scan_at": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"last_scan_at": val})
}

// HandleClearReaderCheckpoint handles DELETE /api/config/readers/{name}/checkpoint.
// Clears the checkpoint so the next scan processes the full lookback window.
// If the daemon is currently running, it is restarted so it picks up the
// now-absent checkpoint immediately rather than waiting for the next interval.
// @Summary Clear a reader checkpoint
// @Tags Config
// @Produce json
// @Param name path string true "Reader name"
// @Success 204 "No Content"
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/readers/{name}/checkpoint [delete]
func (h *Handlers) HandleClearReaderCheckpoint(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	name := r.PathValue("name")
	if err := h.store.SetAppConfig(r.Context(), "reader."+name+".last_scan_at", ""); err != nil {
		h.logger.Error("clear reader checkpoint", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to clear checkpoint")
		return
	}
	// Restart the running daemon so it reloads the (now-absent) checkpoint and
	// immediately starts a full-lookback scan rather than continuing from the
	// stale in-memory value.
	if h.daemon.Status().Running && h.restartFn != nil {
		h.restartFn(name)
	}
	w.WriteHeader(http.StatusNoContent)
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
	switch key {
	case "page":
		if n > maxPageParam {
			return def
		}
	case "page_size":
		if n > maxPageSizeParam {
			return def
		}
	}
	return n
}

// queryHour parses an hour filter (0–23) from a query parameter.
// Returns nil when the parameter is absent or out of range.
func queryHour(r *http.Request, key string) *int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 || n > 23 {
		return nil
	}
	return &n
}

// queryWeekday parses a PostgreSQL DOW weekday filter (0–6) from a query parameter.
// Returns nil when the parameter is absent or out of range.
func queryWeekday(r *http.Request, key string) *int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 || n > 6 {
		return nil
	}
	return &n
}

func (h *Handlers) resolveTimezone(ctx context.Context, requested string) string {
	const fallback = "UTC"

	if requested != "" {
		if _, err := time.LoadLocation(requested); err == nil {
			return requested
		}
	}
	if h.store != nil {
		if configured, err := h.store.GetAppConfig(ctx, "app.timezone"); err == nil && configured != "" {
			if _, err := time.LoadLocation(configured); err == nil {
				return configured
			}
		}
	}
	return fallback
}

// generateState creates a cryptographically random OAuth state token.
func generateState(readerName string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating OAuth state: %w", err)
	}
	return fmt.Sprintf("reader:%s:%s", readerName, hex.EncodeToString(b)), nil
}

// HandleTriggerSync triggers an immediate community content sync.
// POST /api/config/sync
// @Summary Trigger community content sync
// @Tags Config
// @Accept json
// @Produce json
// @Success 200 {object} DocStatusOnlyResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/sync [post]
func (h *Handlers) HandleTriggerSync(w http.ResponseWriter, r *http.Request) {
	if h.syncFn == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sync not configured"})
		return
	}
	go h.syncFn()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleGetSyncStatus returns the last community sync status.
// GET /api/config/sync/status
// @Summary Get community sync status
// @Tags Config
// @Produce json
// @Success 200 {object} DocSyncStatusResponse
// @Failure 500 {object} DocErrorResponse
// @Failure 503 {object} DocErrorResponse
// @Router /config/sync/status [get]
func (h *Handlers) HandleGetSyncStatus(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	status, err := h.store.GetSyncStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}
