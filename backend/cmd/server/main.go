package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"

	httpapi "github.com/ArionMiles/expensor/backend/internal/api"
	"github.com/ArionMiles/expensor/backend/internal/daemon"
	"github.com/ArionMiles/expensor/backend/internal/migration"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/migrations"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/client"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/observability"
	gmailplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/readers/gmail"
	thunderbirdplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/readers/thunderbird"
	postgresplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/writers/postgres"
	pkgrules "github.com/ArionMiles/expensor/backend/pkg/rules"
	"github.com/ArionMiles/expensor/backend/pkg/state"
)

// Version is set at build time via -ldflags="-X main.Version=<version>".
// Defaults to "dev" when built without the flag (e.g. go run ./cmd/server).
var Version = "dev"

const (
	pgConnectTimeout = 30 * time.Second
	pgRetryInterval  = 2 * time.Second

	// defaultCommunityURL is the canonical source for community-maintained MCC and
	// merchant category data. It is seeded into app_config on first startup and is
	// not user-configurable.
	defaultCommunityURL = "https://raw.githubusercontent.com/ArionMiles/expensor/main/content"
)

var (
	//go:embed content/rules.json
	rulesInput string
	//go:embed content/mcc.json
	mccInput []byte
	//go:embed content/categories.json
	categoriesInput []byte
	//go:embed content/banks.json
	banksInput []byte
	//go:embed content/readers/gmail/guide.json content/readers/thunderbird/guide.json
	readersFS embed.FS
)

// embeddedContent bundles the parsed results of all embedded JSON assets.
type embeddedContent struct {
	rawRules   []api.Rule
	rules      []api.Rule
	mccEntries []store.MCCEntry
	catEntries []store.MerchantCategoryEntry
}

// RuleJSON represents a rule in JSON format for parsing.
type RuleJSON struct {
	Name            string `json:"name"`
	SenderEmail     string `json:"senderEmail"`
	SubjectContains string `json:"subjectContains"`
	AmountRegex     string `json:"amountRegex"`
	MerchantRegex   string `json:"merchantInfoRegex"`
	CurrencyRegex   string `json:"currencyRegex,omitempty"`
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
	mu           sync.Mutex
	ctx          context.Context
	cancelFn     context.CancelFunc // cancels the current daemon run; nil when idle
	activeReader string             // reader name currently running or last launched
	registry     *plugins.Registry
	cfg          config.Config
	systemRules  []api.Rule
	resolver     api.CategoryResolver
	st           httpapi.Storer
	diagnostics  api.DiagnosticSink
	dm           *daemonManager
	logger       *slog.Logger
}

// launch builds runtime config and merged rules then spawns runDaemon in a goroutine.
// Must be called with c.mu held.
func (c *daemonCoordinator) launch(readerName string, forceRescan bool) {
	runCtx, cancel := context.WithCancel(c.ctx)
	c.cancelFn = cancel
	c.activeReader = readerName
	runtimeCfg := applyScanOverrides(c.cfg, c.st)

	// Load scanning checkpoint from DB.
	if !forceRescan {
		runtimeCfg.LastScanAt = loadLastScanAt(c.ctx, c.st, readerName, c.logger)
	} else {
		runtimeCfg.ForceFullScan = true
	}

	// OnCheckpoint saves the scan timestamp back to DB after each successful scan.
	if c.st != nil {
		runtimeCfg.OnCheckpoint = func(t time.Time) {
			key := "reader." + readerName + ".last_scan_at"
			if err := c.st.SetAppConfig(c.ctx, key, t.Format(time.RFC3339)); err != nil {
				c.logger.Warn("failed to save scan checkpoint", "reader", readerName, "error", err)
			} else {
				c.logger.Debug("scan checkpoint saved", "reader", readerName, "at", t.Format(time.RFC3339))
			}
		}
	}

	merged := pkgrules.MergeRules(c.systemRules, loadUserRules(c.ctx, c.st, c.logger))
	go func() {
		defer cancel()
		runDaemon(runCtx, c.registry, readerName, runtimeCfg, merged, c.resolver, c.diagnostics, c.st, c.dm, c.logger, forceRescan)
	}()
}

// loadLastScanAt reads the last scan checkpoint for a reader from app_config.
// Returns nil if no checkpoint exists (first run).
func loadLastScanAt(ctx context.Context, st httpapi.Storer, readerName string, logger *slog.Logger) *time.Time {
	if st == nil {
		return nil
	}
	key := "reader." + readerName + ".last_scan_at"
	val, err := st.GetAppConfig(ctx, key)
	if err != nil {
		return nil // no checkpoint yet — will do a full lookback scan
	}
	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		logger.Warn("invalid scan checkpoint, will do full scan", "reader", readerName, "value", val)
		return nil
	}
	logger.Debug("loaded scan checkpoint", "reader", readerName, "at", t.Format(time.RFC3339))
	return &t
}

// stopCurrent cancels the running daemon and clears cancelFn so concurrent callers
// that arrive during the wait can detect that the teardown is already in progress.
// Must be called with c.mu held.
func (c *daemonCoordinator) stopCurrent() {
	if c.cancelFn != nil {
		c.cancelFn()
		c.cancelFn = nil
	}
}

// waitStopped blocks until the daemon manager reports that the daemon has stopped.
// Must NOT be called with c.mu held (the daemon goroutine does not need it, but
// releasing the lock avoids a deadlock if anything upstream tries to take it).
func (c *daemonCoordinator) waitStopped() {
	for c.dm.Status().Running {
		time.Sleep(50 * time.Millisecond)
	}
}

// start is safe to call concurrently. If the daemon is already running with the same
// reader it is a no-op. If a different reader is requested, the current daemon is
// stopped and the new one is started.
func (c *daemonCoordinator) start(readerName string) {
	c.mu.Lock()
	if c.dm.Status().Running {
		if c.activeReader == readerName {
			c.mu.Unlock()
			return
		}
		// Stop the current daemon so we can switch readers.
		c.stopCurrent()
		c.mu.Unlock()
		c.waitStopped()
		c.mu.Lock()
		// A concurrent goroutine may have already launched a new daemon while we waited.
		if c.cancelFn != nil {
			c.mu.Unlock()
			return
		}
	}
	if err := saveActiveReader(c.ctx, c.st, readerName); err != nil {
		c.logger.Warn("failed to persist active reader", "error", err)
	}
	c.launch(readerName, false)
	c.mu.Unlock()
}

func (c *daemonCoordinator) stop() {
	c.mu.Lock()
	if c.dm.Status().Running {
		c.stopCurrent()
		c.activeReader = ""
		c.mu.Unlock()
		c.waitStopped()
		return
	}
	c.activeReader = ""
	c.mu.Unlock()
}

// rescan stops any running daemon and relaunches with forceRescan=true, bypassing
// state deduplication and the checkpoint so the full lookback window is scanned.
func (c *daemonCoordinator) rescan(readerName string) {
	c.mu.Lock()
	if c.dm.Status().Running {
		c.stopCurrent()
		c.mu.Unlock()
		c.waitStopped()
		c.mu.Lock()
		if c.cancelFn != nil {
			c.mu.Unlock()
			return
		}
	}
	c.launch(readerName, true)
	c.mu.Unlock()
}

// restart stops any running daemon and relaunches without force rescan, causing
// the reader to reload the checkpoint from DB. Used after checkpoint clear so
// the next scan uses the full lookback window without bypassing deduplication.
func (c *daemonCoordinator) restart(readerName string) {
	c.mu.Lock()
	if c.dm.Status().Running {
		c.stopCurrent()
		c.mu.Unlock()
		c.waitStopped()
		c.mu.Lock()
		if c.cancelFn != nil {
			c.mu.Unlock()
			return
		}
	}
	c.launch(readerName, false)
	c.mu.Unlock()
}

// refreshResolver reloads the CategoryResolver from the DB snapshot and, if
// the daemon is running, restarts it so the new mappings take effect immediately.
func (c *daemonCoordinator) refreshResolver(ctx context.Context) {
	resolver, err := c.st.LoadCategorySnapshot(ctx)
	if err != nil {
		c.logger.Warn("failed to reload category snapshot after sync", "error", err)
		return
	}
	c.mu.Lock()
	c.resolver = resolver
	active := c.activeReader
	running := c.dm.Status().Running
	c.mu.Unlock()

	if running && active != "" {
		c.logger.Info("restarting daemon to apply updated category resolver")
		c.start(active)
	}
}

func main() {
	os.Exit(run())
}

func run() int {
	shutdownObservability, logger, err := observability.Setup(context.Background(), observability.DefaultConfig())
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize observability: %v\n", err)
		return 1
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := shutdownObservability(shutdownCtx); err != nil {
			logger.Warn("failed to shutdown observability", "error", err)
		}
	}()

	registry := plugins.NewRegistry()
	if err := registerPlugins(registry, readersFS, logger); err != nil {
		logger.Error("failed to register plugins", "error", err)
		return 1
	}
	logger.Info("plugins registered",
		"readers", len(registry.ListReaders()),
		"writers", len(registry.ListWriters()),
	)

	cfg, err := loadAppConfig()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		return 1
	}
	cfg.ApplyDefaults()

	// Validate and wait for postgres connectivity — fatal if unavailable after timeout.
	if err := cfg.ValidatePostgres(); err != nil {
		logger.Error("postgres configuration incomplete", "error", err)
		return 1
	}
	if err := waitForPostgres(cfg.Postgres, logger); err != nil {
		logger.Error("postgres not reachable at startup", "error", err)
		return 1
	}

	content, err := parseEmbedded(rulesInput, mccInput, categoriesInput)
	if err != nil {
		logger.Error("failed to parse embedded content", "error", err)
		return 1
	}
	logger.Info("loaded embedded content", "rules", len(content.rules), "mcc_codes", len(content.mccEntries), "merchant_categories", len(content.catEntries))

	// Root context — canceled on SIGINT/SIGTERM.
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Info("received shutdown signal", "signal", sig)
		cancel()
	}()

	// Run schema migrations before opening the store connection.
	if err := runMigrations(cfg.Postgres, logger); err != nil {
		logger.Error("failed to run migrations", "error", err)
		return 1
	}

	// Connect store (API query layer).
	pgStore, storeErr := store.New(cfg.Postgres, logger.With("component", "store"))
	if storeErr != nil {
		logger.Error("failed to connect store", "error", storeErr)
		return 1
	}

	// Seed embedded content and build the CategoryResolver. Close is deferred only
	// after seeding succeeds, so the failure path closes the store explicitly.
	resolver, err := seedStartupData(ctx, pgStore, content, logger)
	if err != nil {
		pgStore.Close()
		logger.Error("startup seeding failed", "error", err)
		return 1
	}
	defer pgStore.Close()

	storeLogger := logger.With("component", "store")
	storeScope := observability.NewScope(storeLogger, "github.com/ArionMiles/expensor/backend/internal/store")
	instrumentedStore := store.NewInstrumentedStore(pgStore, storeScope, storeLogger)
	var st httpapi.Storer = instrumentedStore

	// dm is started on demand via POST /api/daemon/start.
	dm := &daemonManager{}

	// dc coordinates daemon start and rescan requests with a shared mutex.
	dc := &daemonCoordinator{
		ctx: ctx, registry: registry, cfg: cfg,
		systemRules: content.rules, resolver: resolver,
		st: st, diagnostics: instrumentedStore, dm: dm, logger: logger,
	}

	// Auto-start daemon if a previous reader selection was persisted.
	if savedReader := loadActiveReader(ctx, st, logger); savedReader != "" {
		logger.Info("resuming daemon from previous session", "reader", savedReader)
		dc.start(savedReader)
	}

	syncFn := startCommunitySync(ctx, pgStore, st, dc, logger)
	handlers := httpapi.NewHandlers(httpapi.HandlersConfig{
		Registry:           registry,
		Store:              st,
		Daemon:             dm,
		Version:            Version,
		BaseURL:            cfg.BaseURL,
		FrontendURL:        cfg.FrontendURL,
		ThunderbirdDataDir: cfg.Thunderbird.DataDir,
		ScanInterval:       cfg.ScanInterval,
		LookbackDays:       cfg.LookbackDays,
		StartFn:            dc.start,
		StopFn:             dc.stop,
		RescanFn:           dc.rescan,
		RestartFn:          dc.restart,
		SyncFn:             syncFn,
		BanksData:          banksInput,
		Logger:             logger.With("component", "api"),
	})
	server := httpapi.NewServer(cfg.Port, handlers, cfg.StaticDir, logger.With("component", "http"))

	if err := server.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("HTTP server error", "error", err)
		return 1 // allow deferred cleanup (pgStore.Close) to run
	}
	logger.Info("shutdown complete")
	return 0
}

// runDaemon builds the OAuth client and daemon runner, then blocks until ctx is done.
func runDaemon( //nolint:revive // all parameters are required; splitting further would obscure the call site
	ctx context.Context,
	registry *plugins.Registry,
	readerName string,
	cfg config.Config,
	rules []api.Rule,
	resolver api.CategoryResolver,
	diagnosticSink api.DiagnosticSink,
	runtimeStore httpapi.Storer,
	dm *daemonManager,
	logger *slog.Logger,
	forceRescan bool,
) {
	const writerName = "postgres"
	logger.Debug("runDaemon starting", "reader", readerName, "writer", writerName)

	scopes, err := registry.GetAllScopes(readerName, writerName)
	if err != nil {
		logger.Error("failed to resolve OAuth scopes", "reader", readerName, "writer", writerName, "error", err)
		dm.setStopped(err)
		return
	}
	logger.Debug("resolved OAuth scopes", "scopes", scopes)

	var httpClient *http.Client
	if len(scopes) > 0 {
		if runtimeStore == nil {
			err := errors.New("runtime store is nil")
			logger.Error("failed to create OAuth client", "error", err)
			dm.setStopped(err)
			return
		}
		secretJSON, ok, err := runtimeStore.GetReaderSecret(ctx, readerName)
		if err != nil {
			logger.Error("failed to load OAuth credentials", "reader", readerName, "error", err)
			dm.setStopped(err)
			return
		}
		if !ok {
			err := fmt.Errorf("credentials not uploaded for reader %q", readerName)
			logger.Error("failed to create OAuth client — run onboarding first", "error", err)
			dm.setStopped(err)
			return
		}
		logger.Debug("creating OAuth HTTP client", "reader", readerName)
		httpClient, err = client.NewFromJSONAndStore(ctx, secretJSON, runtimeStore, readerName, scopes...)
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
		stateManager = state.NewDBManager(runtimeStore, logger)
	}

	runner := daemon.New(registry, httpClient, logger)
	runCfg := daemon.RunConfig{
		ReaderName:     readerName,
		WriterName:     writerName,
		Config:         &cfg,
		Rules:          rules,
		Resolver:       resolver,
		StateManager:   stateManager,
		DiagnosticSink: diagnosticSink,
		RuntimeStore:   runtimeStore,
		ForceRescan:    forceRescan,
	}

	logger.Info("daemon starting", "reader", readerName, "writer", writerName)
	dm.setRunning(time.Now())

	runErr := runner.Run(ctx, runCfg)
	if runErr != nil {
		logger.Error("daemon stopped with error", "error", runErr)
	}
	dm.setStopped(runErr)
}

// buildSystemRuleRows converts parsed rules to store.RuleRow values ready for seeding.
func buildSystemRuleRows(raw []api.Rule) []store.RuleRow {
	rows := make([]store.RuleRow, 0, len(raw))
	for _, r := range raw {
		sender := r.SenderEmail
		if sender == "" && len(r.SenderEmails) > 0 {
			sender = r.SenderEmails[0]
		}
		rows = append(rows, store.RuleRow{
			Name: r.Name, SenderEmail: sender, SubjectContains: r.SubjectContains,
			AmountRegex: regexString(r.Amount), MerchantRegex: regexString(r.MerchantInfo),
			CurrencyRegex: regexString(r.Currency), TransactionSource: r.Source.Display(),
			SenderEmails: r.SenderEmails, SourceType: r.Source.Type, SourceLabel: r.Source.Label, Bank: r.Source.Bank,
		})
	}
	return rows
}

func regexString(re *regexp.Regexp) string {
	if re == nil {
		return ""
	}
	return re.String()
}

// loadUserRules fetches all user-created (non-predefined) rules from the store and compiles them.
// Returns nil on any error; daemon falls back to embedded predefined rules via MergeRules.
func loadUserRules(ctx context.Context, st httpapi.Storer, logger *slog.Logger) []api.Rule {
	if st == nil {
		return nil
	}
	rows, err := st.ListRules(ctx)
	if err != nil {
		logger.Warn("failed to load rules from DB, falling back to embedded rules", "error", err)
		return nil
	}
	var out []api.Rule
	for _, row := range rows {
		if row.Predefined {
			continue // predefined rules are already in c.systemRules; DB edits override via MergeRules
		}
		r, compileErr := compileRule(row)
		if compileErr != nil {
			logger.Warn("skipping rule with invalid regex", "rule", row.Name, "error", compileErr)
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
		ID: row.ID, Name: row.Name, SenderEmail: row.SenderEmail, SubjectContains: row.SubjectContains,
		Amount: amount, MerchantInfo: merchant, Currency: currency,
		SenderEmails: row.SenderEmails,
		Source:       api.Source{Type: row.SourceType, Label: row.SourceLabel, Bank: row.Bank},
	}, nil
}

// runMigrations opens a short-lived pool, applies all pending numbered SQL
// migrations from the embedded migrations directory, and closes the pool.
// Called once at startup before the store and writer pools are created.
func runMigrations(pgCfg config.PostgresConfig, logger *slog.Logger) error {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s pool_max_conns=1",
		pgCfg.Host, pgCfg.Port, pgCfg.User, pgCfg.Password, pgCfg.Database, pgCfg.SSLMode,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return fmt.Errorf("opening migration pool: %w", err)
	}
	defer pool.Close()

	return migration.Run(ctx, pool, migrations.FS, logger)
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

// saveActiveReader persists the reader name so it can be resumed on the next startup.
func saveActiveReader(ctx context.Context, st httpapi.Storer, readerName string) error {
	if st == nil {
		return nil
	}
	return st.SetActiveReader(ctx, readerName)
}

// loadActiveReader reads the persisted reader name. Returns "" if absent.
func loadActiveReader(ctx context.Context, st httpapi.Storer, logger *slog.Logger) string {
	if st == nil {
		return ""
	}
	reader, err := st.GetActiveReader(ctx)
	if err != nil {
		logger.Warn("failed to read active reader", "error", err)
		return ""
	}
	return reader
}

// parseEmbedded parses the embedded rules, MCC codes, and merchant categories JSON.
func parseEmbedded(rulesJSON string, mccJSON, categoriesJSON []byte) (embeddedContent, error) {
	compiled, err := parseRules(rulesJSON)
	if err != nil {
		return embeddedContent{}, err
	}
	var mccEntries []store.MCCEntry
	if err := json.Unmarshal(mccJSON, &mccEntries); err != nil {
		return embeddedContent{}, fmt.Errorf("parsing mcc JSON: %w", err)
	}
	var catEntries []store.MerchantCategoryEntry
	if err := json.Unmarshal(categoriesJSON, &catEntries); err != nil {
		return embeddedContent{}, fmt.Errorf("parsing categories JSON: %w", err)
	}
	return embeddedContent{rawRules: compiled, rules: compiled, mccEntries: mccEntries, catEntries: catEntries}, nil
}

// uniqueCategoryNames extracts unique category names from MCC entries, sorted.
func uniqueCategoryNames(entries []store.MCCEntry) []string {
	seen := make(map[string]struct{})
	for _, e := range entries {
		seen[e.Category] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// parseRules parses the embedded rules JSON into []api.Rule.
func parseRules(rulesJSON string) ([]api.Rule, error) {
	doc, err := pkgrules.ParseDocument([]byte(rulesJSON))
	if err != nil {
		return nil, fmt.Errorf("parsing rules JSON: %w", err)
	}
	return doc.Rules, nil
}

// registerPlugins loads guide data from the embedded FS into each reader plugin,
// then registers all readers and the postgres writer in the registry.
func registerPlugins(registry *plugins.Registry, fs embed.FS, logger *slog.Logger) error {
	gmailPlugin := &gmailplugin.Plugin{}
	tbPlugin := &thunderbirdplugin.Plugin{}

	if data, err := fs.ReadFile("content/readers/gmail/guide.json"); err == nil {
		gmailPlugin.SetGuideData(data)
	} else {
		logger.Warn("could not load gmail guide", "error", err)
	}
	if data, err := fs.ReadFile("content/readers/thunderbird/guide.json"); err == nil {
		tbPlugin.SetGuideData(data)
	} else {
		logger.Warn("could not load thunderbird guide", "error", err)
	}

	for _, p := range []plugins.ReaderPlugin{gmailPlugin, tbPlugin} {
		if err := registry.RegisterReader(p); err != nil {
			return fmt.Errorf("registering reader %s: %w", p.Metadata().Name, err)
		}
	}
	postgresPlugin := &postgresplugin.Plugin{}
	if err := registry.RegisterWriter(postgresPlugin); err != nil {
		return fmt.Errorf("registering writer %s: %w", postgresPlugin.Metadata().Name, err)
	}
	return nil
}

// loadAppConfig reads all environment variables and returns a populated Config.
// Prefix-based loads cover the majority of config; a separate pass picks up
// the small set of prefix-less server vars (PORT, BASE_URL, FRONTEND_URL).
func loadAppConfig() (config.Config, error) {
	k := koanf.New(".")
	for _, prefix := range []string{"EXPENSOR_", "POSTGRES_"} {
		if err := k.Load(env.Provider(prefix, ".", func(s string) string { return s }), nil); err != nil {
			return config.Config{}, fmt.Errorf("loading env prefix %s: %w", prefix, err)
		}
	}
	// Load prefix-less server vars by allowlist to avoid ingesting PATH, HOME, etc.
	serverVars := map[string]bool{"PORT": true, "BASE_URL": true, "FRONTEND_URL": true, "THUNDERBIRD_DATA_DIR": true}
	if err := k.Load(env.Provider("", ".", func(s string) string {
		if serverVars[s] {
			return s
		}
		return ""
	}), nil); err != nil {
		return config.Config{}, fmt.Errorf("loading server env vars: %w", err)
	}
	var cfg config.Config
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf", FlatPaths: true}); err != nil {
		return config.Config{}, fmt.Errorf("unmarshaling config: %w", err)
	}
	return cfg, nil
}

// startCommunitySync seeds the default community URL (if absent), launches the
// initial sync, and starts the background ticker goroutine. It returns a syncFn
// that triggers a manual sync + resolver refresh.
func startCommunitySync(ctx context.Context, pgStore *store.Store, st httpapi.Storer, dc *daemonCoordinator, logger *slog.Logger) func() {
	syncInterval := 24 * time.Hour
	if v := os.Getenv("EXPENSOR_CONTENT_SYNC_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			syncInterval = d
		}
	}
	if existing, err := pgStore.GetCommunityURL(ctx); err != nil || existing == "" {
		if setErr := pgStore.SetCommunityURL(ctx, defaultCommunityURL); setErr != nil {
			logger.Warn("failed to seed default community URL", "error", setErr)
		}
	}
	syncLog := logger.With("component", "sync")
	go syncCommunityContent(ctx, st, defaultCommunityURL, syncLog)
	go func() {
		ticker := time.NewTicker(syncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				syncCommunityContent(ctx, st, defaultCommunityURL, syncLog)
				dc.refreshResolver(ctx)
			}
		}
	}()
	return func() {
		syncCommunityContent(context.Background(), st, defaultCommunityURL, syncLog)
		dc.refreshResolver(context.Background())
	}
}

// seedStartupData seeds all embedded content into the database and returns a
// CategoryResolver built from the resulting DB snapshot. All errors are returned
// so that the caller can close the store before calling os.Exit.
func seedStartupData(ctx context.Context, pgStore *store.Store, content embeddedContent, logger *slog.Logger) (api.CategoryResolver, error) {
	systemRuleRows := buildSystemRuleRows(content.rawRules)
	if err := pgStore.SeedPredefinedRules(ctx, systemRuleRows); err != nil {
		return nil, fmt.Errorf("seeding predefined rules: %w", err)
	}
	logger.Info("predefined rules seeded", "count", len(systemRuleRows))

	if err := pgStore.SeedMCCCodes(ctx, content.mccEntries); err != nil {
		return nil, fmt.Errorf("seeding MCC codes: %w", err)
	}
	if _, err := pgStore.SeedMerchantCategories(ctx, content.catEntries); err != nil {
		return nil, fmt.Errorf("seeding merchant categories: %w", err)
	}
	mccCategoryNames := uniqueCategoryNames(content.mccEntries)
	if err := pgStore.SeedMCCCategories(ctx, mccCategoryNames); err != nil {
		return nil, fmt.Errorf("seeding MCC category names: %w", err)
	}

	resolver, err := pgStore.LoadCategorySnapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading category snapshot: %w", err)
	}
	logger.Info("category resolver loaded")
	return resolver, nil
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

// syncCommunityContent fetches mcc.json and categories.json from the community URL,
// upserts them into the DB, and updates the sync status in app_config.
// Non-fatal: errors are logged and stored as sync status; caller continues.
func syncCommunityContent(ctx context.Context, st httpapi.Storer, baseURL string, logger *slog.Logger) {
	if baseURL == "" {
		return
	}
	scope := observability.NewScope(logger, "github.com/ArionMiles/expensor/backend/cmd/server/community_sync")
	ctx, span := scope.Start(ctx, "community_sync.sync")
	defer span.End()
	start := time.Now()
	var syncErr error
	defer func() {
		scope.RecordDuration(ctx, observability.DurationOperation{
			Namespace: "community_sync",
			Name:      "sync",
			Duration:  time.Since(start),
			Err:       syncErr,
		})
	}()
	logger.Info("syncing community content", "url", baseURL)

	fetchJSON := func(path string, dest any) error {
		url := strings.TrimRight(baseURL, "/") + "/" + path
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("building request: %w", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("fetching %s: %w", path, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status %d fetching %s", resp.StatusCode, path)
		}
		return json.NewDecoder(resp.Body).Decode(dest)
	}

	var mccEntries []store.MCCEntry
	if err := fetchJSON("mcc.json", &mccEntries); err != nil {
		syncErr = err
		recordSyncError(ctx, st, err, logger)
		return
	}
	var catEntries []store.MerchantCategoryEntry
	if err := fetchJSON("categories.json", &catEntries); err != nil {
		syncErr = err
		recordSyncError(ctx, st, err, logger)
		return
	}

	if err := st.SeedMCCCodes(ctx, mccEntries); err != nil {
		syncErr = err
		recordSyncError(ctx, st, err, logger)
		return
	}
	updated, err := st.SeedMerchantCategories(ctx, catEntries)
	if err != nil {
		syncErr = err
		recordSyncError(ctx, st, err, logger)
		return
	}

	now := time.Now().UTC()
	status := store.SyncStatus{LastSyncedAt: &now, EntriesUpdated: updated}
	if err := st.SetSyncStatus(ctx, status); err != nil {
		logger.Warn("failed to persist sync status", "error", err)
	}
	logger.Info("community sync complete", "entries_updated", updated)
}

func recordSyncError(ctx context.Context, st httpapi.Storer, syncErr error, logger *slog.Logger) {
	logger.Warn("community sync failed", "error", syncErr)
	errStr := syncErr.Error()
	status := store.SyncStatus{Error: &errStr}
	if err := st.SetSyncStatus(ctx, status); err != nil {
		logger.Warn("failed to persist sync error status", "error", err)
	}
}
