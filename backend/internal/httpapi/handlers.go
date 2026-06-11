package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/form/v4"
	"github.com/go-playground/validator/v10"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
)

const (
	defaultBaseCurrency = "INR"
	maxCredentialsSize  = 5 << 20 // 5 MB
	oauthStateTTL       = 10 * time.Minute
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
	settingsStore      settingsStore
	analyticsStore     analyticsStore
	transactionStore   transactionStore
	muteStore          muteStore
	taxonomyStore      taxonomyStore
	readerRuntimeStore readerRuntimeStore
	ruleStore          ruleStore
	syncStore          syncStore
	diagnosticStore    diagnosticStore
	daemon             DaemonStatusProvider
	version            string // set at build time via ldflags
	baseURL            string // e.g. "http://localhost:8080"
	frontendURL        string // e.g. "http://localhost:5173" — used for OAuth redirects
	thunderbirdDataDir string
	scanInterval       int                 // default scan interval in seconds
	lookbackDays       int                 // default lookback in days
	startFn            func(reader string) // called by POST /api/daemon/start; may be nil
	stopFn             func()              // called when active reader runtime is removed; may be nil
	rescanFn           func(reader string) // called by POST /api/daemon/rescan; may be nil
	restartFn          func(reader string) // called after checkpoint clear to reload from DB; may be nil
	syncFn             func()              // called by POST /api/config/sync; may be nil
	banksData          []byte
	logger             *slog.Logger
	validate           *validator.Validate
	queryDecoder       *form.Decoder

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
	ThunderbirdDataDir string
	ScanInterval       int
	LookbackDays       int
	StartFn            func(reader string)
	StopFn             func()
	RescanFn           func(reader string)
	RestartFn          func(reader string)
	SyncFn             func()
	BanksData          []byte
	Logger             *slog.Logger
}

// NewHandlers creates a Handlers instance.
// Pass nil StartFn to disable the daemon start endpoint.
func NewHandlers(cfg HandlersConfig) *Handlers {
	if cfg.FrontendURL == "" {
		cfg.FrontendURL = cfg.BaseURL
	}
	if cfg.ScanInterval <= 0 {
		cfg.ScanInterval = 60
	}
	if cfg.LookbackDays <= 0 {
		cfg.LookbackDays = 180
	}
	return &Handlers{
		registry:           cfg.Registry,
		settingsStore:      cfg.Store,
		analyticsStore:     cfg.Store,
		transactionStore:   cfg.Store,
		muteStore:          cfg.Store,
		taxonomyStore:      cfg.Store,
		readerRuntimeStore: cfg.Store,
		ruleStore:          cfg.Store,
		syncStore:          cfg.Store,
		diagnosticStore:    cfg.Store,
		daemon:             cfg.Daemon,
		version:            cfg.Version,
		baseURL:            strings.TrimRight(cfg.BaseURL, "/"),
		frontendURL:        strings.TrimRight(cfg.FrontendURL, "/"),
		thunderbirdDataDir: cfg.ThunderbirdDataDir,
		scanInterval:       cfg.ScanInterval,
		lookbackDays:       cfg.LookbackDays,
		startFn:            cfg.StartFn,
		stopFn:             cfg.StopFn,
		rescanFn:           cfg.RescanFn,
		restartFn:          cfg.RestartFn,
		syncFn:             cfg.SyncFn,
		banksData:          cfg.BanksData,
		logger:             cfg.Logger,
		validate:           newRequestValidator(),
		queryDecoder:       newQueryDecoder(),
		oauthStates:        make(map[string]oauthStateEntry),
	}
}

// --- health & status ---

// Health handles GET /api/health.
// @Summary Health check
// @Tags Bootstrap
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (h *Handlers) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Version handles GET /api/version.
// @Summary Get backend version
// @Tags Bootstrap
// @Produce json
// @Success 200 {object} VersionResponse
// @Router /version [get]
func (h *Handlers) Version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": h.version})
}

// Status handles GET /api/status.
// @Summary Get daemon and stats status
// @Tags Bootstrap
// @Produce json
// @Success 200 {object} StatusResponse
// @Router /status [get]
func (h *Handlers) Status(w http.ResponseWriter, r *http.Request) {
	ds := h.daemon.Status()

	type statusResponse struct {
		Daemon DaemonStatus `json:"daemon"`
		Stats  *store.Stats `json:"stats,omitempty"`
	}
	resp := statusResponse{Daemon: ds}

	currency := h.currentBaseCurrency(r.Context())
	stats, err := h.analyticsStore.GetStats(r.Context(), currency)
	if err != nil {
		h.logger.Error("get status stats", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch stats")
		return
	}
	resp.Stats = stats

	writeJSON(w, http.StatusOK, resp)
}
