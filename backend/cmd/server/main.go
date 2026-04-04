package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"

	httpapi "github.com/ArionMiles/expensor/backend/internal/api"
	"github.com/ArionMiles/expensor/backend/internal/daemon"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/client"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/logging"
	gmailplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/readers/gmail"
	thunderbirdplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/readers/thunderbird"
	postgresplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/writers/postgres"
	pkgrules "github.com/ArionMiles/expensor/backend/pkg/rules"
	"github.com/ArionMiles/expensor/backend/pkg/state"
)

const (
	pgConnectTimeout = 30 * time.Second
	pgRetryInterval  = 2 * time.Second
)

var (
	//go:embed content/rules.json
	rulesInput string
	//go:embed content/labels.json
	labelsInput string
)

// RuleJSON represents a rule in JSON format for parsing.
type RuleJSON struct {
	Name            string `json:"name"`
	SenderEmail     string `json:"senderEmail"`
	SubjectContains string `json:"subjectContains"`
	AmountRegex     string `json:"amountRegex"`
	MerchantRegex   string `json:"merchantInfoRegex"`
	CurrencyRegex   string `json:"currencyRegex,omitempty"`
	Enabled         bool   `json:"enabled"`
	Source          string `json:"source"`
}

// daemonManager tracks the lifecycle of the background daemon goroutine and
// implements httpapi.DaemonStatusProvider for the status endpoint.
type daemonManager struct {
	mu        sync.RWMutex
	running   bool
	startedAt *time.Time
	lastError string
}

func (m *daemonManager) setRunning(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = true
	m.startedAt = &t
	m.lastError = ""
}

func (m *daemonManager) setStopped(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		m.lastError = err.Error()
	}
}

// Status implements httpapi.DaemonStatusProvider.
func (m *daemonManager) Status() httpapi.DaemonStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return httpapi.DaemonStatus{
		Running:   m.running,
		StartedAt: m.startedAt,
		LastError: m.lastError,
	}
}

// daemonCoordinator owns the mutex and shared dependencies for starting daemon runs.
// It exposes start and rescan as plain methods so they can be passed as func(string)
// values without adding closure complexity to main.
type daemonCoordinator struct {
	mu          sync.Mutex
	ctx         context.Context
	registry    *plugins.Registry
	cfg         config.Config
	systemRules []api.Rule
	labels      api.Labels
	st          httpapi.Storer
	dm          *daemonManager
	logger      *slog.Logger
}

// launch builds runtime config and merged rules then spawns runDaemon in a goroutine.
func (c *daemonCoordinator) launch(readerName string, forceRescan bool) {
	runtimeCfg := applyScanOverrides(c.cfg, c.st)
	merged := pkgrules.FilterEnabled(pkgrules.MergeRules(c.systemRules, loadUserRules(c.ctx, c.st, c.logger)))
	go runDaemon(c.ctx, c.registry, readerName, runtimeCfg, merged, c.labels, c.dm, c.logger, forceRescan)
}

// start is safe to call concurrently; it is a no-op when the daemon is already running.
// It persists the reader name and checks the rescan_pending flag before launching.
func (c *daemonCoordinator) start(readerName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dm.Status().Running {
		return
	}
	if err := saveActiveReader(c.cfg.DataDir, readerName); err != nil {
		c.logger.Warn("failed to persist active reader", "error", err)
	}
	c.launch(readerName, checkAndClearRescanPending(c.ctx, c.st, c.logger))
}

// rescan starts a daemon run with forceRescan=true, bypassing state deduplication.
// Called by POST /api/daemon/rescan when the daemon is not running.
func (c *daemonCoordinator) rescan(readerName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dm.Status().Running {
		return
	}
	c.launch(readerName, true)
}

func main() {
	logger := logging.Setup(logging.DefaultConfig())

	// Build plugin registry.
	registry := plugins.NewRegistry()
	for _, p := range []plugins.ReaderPlugin{
		&gmailplugin.Plugin{},
		&thunderbirdplugin.Plugin{},
	} {
		if err := registry.RegisterReader(p); err != nil {
			logger.Error("failed to register reader", "plugin", p.Name(), "error", err)
			os.Exit(1)
		}
	}
	if err := registry.RegisterWriter(&postgresplugin.Plugin{}); err != nil {
		logger.Error("failed to register postgres writer", "error", err)
		os.Exit(1)
	}
	logger.Info("plugins registered",
		"readers", len(registry.ListReaders()),
		"writers", len(registry.ListWriters()),
	)

	// Load configuration from environment variables.
	// Only load variables under known namespaces to avoid polluting koanf with
	// unrelated process environment variables (PATH, HOME, etc.).
	k := koanf.New(".")
	for _, prefix := range []string{"EXPENSOR_", "GMAIL_", "THUNDERBIRD_", "POSTGRES_"} {
		if err := k.Load(env.Provider(prefix, ".", func(s string) string { return s }), nil); err != nil {
			logger.Error("failed to load env config", "prefix", prefix, "error", err)
			os.Exit(1)
		}
	}
	var cfg config.Config
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf", FlatPaths: true}); err != nil {
		logger.Error("failed to unmarshal config", "error", err)
		os.Exit(1)
	}
	cfg.ApplyDefaults()

	// Validate and wait for postgres connectivity — fatal if unavailable after timeout.
	if err := cfg.ValidatePostgres(); err != nil {
		logger.Error("postgres configuration incomplete", "error", err)
		os.Exit(1)
	}
	if err := waitForPostgres(cfg.Postgres, logger); err != nil {
		logger.Error("postgres not reachable at startup", "error", err)
		os.Exit(1)
	}

	// Parse embedded rules and labels.
	rawRules, systemRules, labels, err := parseEmbedded(rulesInput, labelsInput)
	if err != nil {
		logger.Error("failed to parse embedded content", "error", err)
		os.Exit(1)
	}
	logger.Info("loaded embedded content", "rules", len(systemRules), "labels", len(labels))

	// Root context — canceled on SIGINT/SIGTERM.
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Info("received shutdown signal", "signal", sig)
		cancel()
	}()

	// Connect store (API query layer).
	pgStore, storeErr := store.New(cfg.Postgres, logger.With("component", "store"))
	if storeErr != nil {
		logger.Error("failed to connect store", "error", storeErr)
		os.Exit(1)
	}
	var st httpapi.Storer = pgStore

	// Seed embedded rules as system rules — idempotent, safe on every startup.
	systemRuleRows := buildSystemRuleRows(rawRules)
	if seedErr := pgStore.SeedSystemRules(ctx, systemRuleRows); seedErr != nil {
		pgStore.Close()
		logger.Error("failed to seed system rules", "error", seedErr)
		os.Exit(1)
	}
	logger.Info("system rules seeded", "count", len(systemRuleRows))
	defer pgStore.Close()

	// dm is started on demand via POST /api/daemon/start.
	dm := &daemonManager{}

	// Start HTTP server.
	port := envInt("PORT", 8080)
	baseURL := envStr("BASE_URL", fmt.Sprintf("http://localhost:%d", port))
	frontendURL := envStr("FRONTEND_URL", "http://localhost:5173")

	// dc coordinates daemon start and rescan requests with a shared mutex.
	dc := &daemonCoordinator{
		ctx: ctx, registry: registry, cfg: cfg,
		systemRules: systemRules, labels: labels,
		st: st, dm: dm, logger: logger,
	}

	// Auto-start daemon if a previous reader selection was persisted.
	if savedReader := loadActiveReader(cfg.DataDir, logger); savedReader != "" {
		logger.Info("resuming daemon from previous session", "reader", savedReader)
		dc.start(savedReader)
	}

	handlers := httpapi.NewHandlers(
		registry, st, dm, baseURL, frontendURL,
		cfg.DataDir, cfg.BaseCurrency,
		cfg.ScanInterval, cfg.LookbackDays,
		dc.start, dc.rescan,
		logger.With("component", "api"),
	)
	server := httpapi.NewServer(port, handlers, logger.With("component", "http"))

	if err := server.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("HTTP server error", "error", err)
		return // allow deferred cleanup (pgStore.Close) to run
	}
	logger.Info("shutdown complete")
}

// runDaemon builds the OAuth client and daemon runner, then blocks until ctx is done.
func runDaemon( //nolint:revive // all parameters are required; splitting further would obscure the call site
	ctx context.Context,
	registry *plugins.Registry,
	readerName string,
	cfg config.Config,
	rules []api.Rule,
	labels api.Labels,
	dm *daemonManager,
	logger *slog.Logger,
	forceRescan bool,
) {
	const writerName = "postgres"
	logger.Debug("runDaemon starting", "reader", readerName, "writer", writerName)

	// Resolve the credentials file for the configured reader.
	credFile := filepath.Join(cfg.DataDir, fmt.Sprintf("client_secret_%s.json", readerName))
	if _, err := os.Stat(credFile); os.IsNotExist(err) {
		logger.Debug("per-reader cred file not found, falling back to legacy path",
			"tried", credFile, "fallback", config.ClientSecretFile)
		credFile = config.ClientSecretFile
	}
	logger.Debug("using credentials file", "path", credFile)

	scopes, err := registry.GetAllScopes(readerName, writerName)
	if err != nil {
		logger.Error("failed to resolve OAuth scopes", "reader", readerName, "writer", writerName, "error", err)
		dm.setStopped(err)
		return
	}
	logger.Debug("resolved OAuth scopes", "scopes", scopes)

	tokenFile := filepath.Join(cfg.DataDir, fmt.Sprintf("token_%s.json", readerName))
	logger.Debug("using token file", "path", tokenFile)

	var httpClient *http.Client
	if len(scopes) > 0 {
		logger.Debug("creating OAuth HTTP client", "cred_file", credFile, "token_file", tokenFile)
		httpClient, err = client.New(credFile, tokenFile, scopes...)
		if err != nil {
			logger.Error("failed to create OAuth client — run onboarding first", "error", err)
			dm.setStopped(err)
			return
		}
		logger.Debug("OAuth HTTP client created successfully")
	}

	// Create state manager. Skip for forced rescans — readers handle nil gracefully
	// (they guard with `if r.state != nil && r.state.IsProcessed(...)`), allowing
	// already-processed emails to be re-extracted and upserted into the DB.
	var stateManager *state.Manager
	if !forceRescan {
		stateManager, err = state.New(cfg.StateFile, logger)
		if err != nil {
			logger.Error("failed to create state manager", "error", err)
			dm.setStopped(err)
			return
		}
	}

	runner := daemon.New(registry, httpClient, logger)
	runCfg := daemon.RunConfig{
		ReaderName:   readerName,
		WriterName:   writerName,
		Config:       &cfg,
		Rules:        rules,
		Labels:       labels,
		StateManager: stateManager,
		ForceRescan:  forceRescan,
	}

	logger.Info("daemon starting", "reader", readerName, "writer", writerName)
	dm.setRunning(time.Now())

	runErr := runner.Run(ctx, runCfg)
	if runErr != nil {
		logger.Error("daemon stopped with error", "error", runErr)
	}
	dm.setStopped(runErr)
}

// buildSystemRuleRows converts parsed RuleJSON entries to store.RuleRow values ready for seeding.
func buildSystemRuleRows(raw []RuleJSON) []store.RuleRow {
	rows := make([]store.RuleRow, 0, len(raw))
	for _, r := range raw {
		rows = append(rows, store.RuleRow{
			Name: r.Name, SenderEmail: r.SenderEmail, SubjectContains: r.SubjectContains,
			AmountRegex: r.AmountRegex, MerchantRegex: r.MerchantRegex,
			CurrencyRegex: r.CurrencyRegex, Enabled: r.Enabled,
		})
	}
	return rows
}

// loadUserRules fetches user rules from the store and compiles their regexes.
// Returns nil on any error; daemon falls back to system rules only.
func loadUserRules(ctx context.Context, st httpapi.Storer, logger *slog.Logger) []api.Rule {
	if st == nil {
		return nil
	}
	rows, err := st.ListRules(ctx)
	if err != nil {
		logger.Warn("failed to load user rules, using system rules only", "error", err)
		return nil
	}
	var out []api.Rule
	for _, row := range rows {
		if row.Source != "user" {
			continue
		}
		r, compileErr := compileRule(row)
		if compileErr != nil {
			logger.Warn("skipping user rule with invalid regex", "rule", row.Name, "error", compileErr)
			continue
		}
		out = append(out, r)
	}
	return out
}

// compileRule converts a store.RuleRow to an api.Rule by compiling its regex strings.
func compileRule(row store.RuleRow) (api.Rule, error) {
	amount, err := regexp.Compile(row.AmountRegex)
	if err != nil {
		return api.Rule{}, fmt.Errorf("amount_regex: %w", err)
	}
	merchant, err := regexp.Compile(row.MerchantRegex)
	if err != nil {
		return api.Rule{}, fmt.Errorf("merchant_regex: %w", err)
	}
	var currency *regexp.Regexp
	if row.CurrencyRegex != "" {
		currency, err = regexp.Compile(row.CurrencyRegex)
		if err != nil {
			return api.Rule{}, fmt.Errorf("currency_regex: %w", err)
		}
	}
	return api.Rule{
		Name: row.Name, SenderEmail: row.SenderEmail, SubjectContains: row.SubjectContains,
		Amount: amount, MerchantInfo: merchant, Currency: currency,
		Enabled: row.Enabled, Source: row.Source,
	}, nil
}

// waitForPostgres retries a postgres ping until the connection succeeds or the
// timeout is reached. It gives the container time to accept connections after
// being started by `task dev`.
func waitForPostgres(pgCfg config.PostgresConfig, logger *slog.Logger) error {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s pool_max_conns=1",
		pgCfg.Host, pgCfg.Port, pgCfg.User, pgCfg.Password, pgCfg.Database, pgCfg.SSLMode,
	)
	deadline := time.Now().Add(pgConnectTimeout)
	for attempt := 1; ; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		pool, err := pgxpool.New(ctx, connStr)
		cancel()
		if err == nil {
			pingCtx, pingCancel := context.WithTimeout(context.Background(), 3*time.Second)
			err = pool.Ping(pingCtx)
			pingCancel()
			pool.Close()
			if err == nil {
				logger.Info("postgres is ready", "host", pgCfg.Host, "port", pgCfg.Port)
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("postgres not ready after %s: %w", pgConnectTimeout, err)
		}
		logger.Info("waiting for postgres", "attempt", attempt, "error", err)
		time.Sleep(pgRetryInterval)
	}
}

// saveActiveReader persists the reader name to disk so it can be resumed on
// the next startup.
func saveActiveReader(dataDir, readerName string) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, "active_reader"), []byte(readerName), 0o600)
}

// loadActiveReader reads the persisted reader name. Returns "" if absent.
func loadActiveReader(dataDir string, logger *slog.Logger) string {
	b, err := os.ReadFile(filepath.Join(dataDir, "active_reader"))
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("failed to read active reader file", "error", err)
		}
		return ""
	}
	return string(b)
}

// parseEmbedded parses the embedded rules and labels JSON, returning the raw rule
// structs (for DB seeding), compiled rules, and labels.
//
//nolint:revive // four returns are necessary: raw rules, compiled rules, labels, and error
func parseEmbedded(rulesJSON, labelsJSON string) ([]RuleJSON, []api.Rule, api.Labels, error) {
	var raw []RuleJSON
	if err := json.Unmarshal([]byte(rulesJSON), &raw); err != nil {
		return nil, nil, nil, fmt.Errorf("parsing rules JSON: %w", err)
	}
	compiled, err := parseRules(rulesJSON)
	if err != nil {
		return nil, nil, nil, err
	}
	var labels api.Labels
	if err := json.Unmarshal([]byte(labelsJSON), &labels); err != nil {
		return nil, nil, nil, fmt.Errorf("parsing labels JSON: %w", err)
	}
	return raw, compiled, labels, nil
}

// parseRules parses the embedded rules JSON into []api.Rule.
func parseRules(rulesJSON string) ([]api.Rule, error) {
	var rawRules []RuleJSON
	if err := json.Unmarshal([]byte(rulesJSON), &rawRules); err != nil {
		return nil, fmt.Errorf("parsing rules JSON: %w", err)
	}

	rules := make([]api.Rule, 0, len(rawRules))
	for i, raw := range rawRules {
		amountRegex, err := regexp.Compile(raw.AmountRegex)
		if err != nil {
			return nil, fmt.Errorf("compiling amountRegex for rule %d (%s): %w", i, raw.Name, err)
		}
		merchantRegex, err := regexp.Compile(raw.MerchantRegex)
		if err != nil {
			return nil, fmt.Errorf("compiling merchantInfoRegex for rule %d (%s): %w", i, raw.Name, err)
		}
		var currencyRegex *regexp.Regexp
		if raw.CurrencyRegex != "" {
			currencyRegex, err = regexp.Compile(raw.CurrencyRegex)
			if err != nil {
				return nil, fmt.Errorf("compiling currencyRegex for rule %d (%s): %w", i, raw.Name, err)
			}
		}

		rules = append(rules, api.Rule{
			Name:            raw.Name,
			SenderEmail:     raw.SenderEmail,
			SubjectContains: raw.SubjectContains,
			Amount:          amountRegex,
			MerchantInfo:    merchantRegex,
			Currency:        currencyRegex,
			Enabled:         raw.Enabled,
			Source:          raw.Source,
		})
	}
	return rules, nil
}

// checkAndClearRescanPending reads the rescan_pending app_config flag.
// If set to "true" it clears the flag (best-effort) and returns true so the
// caller can start the daemon with forceRescan enabled.
func checkAndClearRescanPending(ctx context.Context, st httpapi.Storer, logger *slog.Logger) bool {
	if st == nil {
		return false
	}
	val, err := st.GetAppConfig(ctx, "rescan_pending")
	if err != nil || val != "true" {
		return false
	}
	_ = st.SetAppConfig(ctx, "rescan_pending", "") // clear flag; best-effort
	logger.Info("rescan_pending flag detected — starting with force rescan")
	return true
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// applyScanOverrides returns a copy of cfg with ScanInterval and LookbackDays
// overridden from app_config when valid UI-set values exist.
func applyScanOverrides(cfg config.Config, st httpapi.Storer) config.Config {
	if st == nil {
		return cfg
	}
	if v, err := st.GetAppConfig(context.Background(), "scan_interval"); err == nil {
		if n, convErr := strconv.Atoi(v); convErr == nil && n > 0 {
			cfg.ScanInterval = n
		}
	}
	if v, err := st.GetAppConfig(context.Background(), "lookback_days"); err == nil {
		if n, convErr := strconv.Atoi(v); convErr == nil && n > 0 {
			cfg.LookbackDays = n
		}
	}
	return cfg
}
