package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/daemon"
	"github.com/ArionMiles/expensor/backend/internal/httpapi"
	"github.com/ArionMiles/expensor/backend/internal/oauth"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/rules"
	"github.com/ArionMiles/expensor/backend/internal/state"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
)

// daemonManager tracks the lifecycle of the background daemon goroutine and
// implements httpapi.DaemonStatusProvider for the status endpoint.
type daemonManager struct {
	mu        sync.RWMutex
	running   bool
	startedAt *time.Time
	lastError string
}

type categorySnapshotStore interface {
	LoadCategorySnapshot(ctx context.Context) (api.CategoryResolver, error)
}

type daemonRuntimeStore interface {
	GetReaderSecret(ctx context.Context, tenant store.Tenant, reader string) ([]byte, bool, error)
	GetReaderToken(ctx context.Context, tenant store.Tenant, reader string) ([]byte, bool, error)
	SetReaderToken(ctx context.Context, tenant store.Tenant, reader string, token []byte) error
	GetReaderConfig(ctx context.Context, tenant store.Tenant, reader string) (json.RawMessage, bool, error)
	IsMessageProcessed(ctx context.Context, tenant store.Tenant, key string) (bool, error)
	MarkMessageProcessed(ctx context.Context, tenant store.Tenant, key string, at time.Time) error
}

type daemonStore interface {
	GetAppConfig(ctx context.Context, tenant store.Tenant, key string) (string, error)
	SetAppConfig(ctx context.Context, tenant store.Tenant, key, value string) error
	SetActiveScanningReader(ctx context.Context, tenant store.Tenant, reader string) error
	ListRules(ctx context.Context, tenant store.Tenant) ([]store.RuleRow, error)
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
// It exposes start and rescan as plain methods so they can be passed directly
// to the HTTP handlers without adding closure complexity to main.
type daemonCoordinator struct {
	mu                sync.Mutex
	ctx               context.Context
	cancelFn          context.CancelFunc // cancels the current daemon run; nil when idle
	activeReader      string             // reader name currently running or last launched
	activeTenant      store.Tenant       // tenant currently running or last launched
	registry          *plugins.Registry
	cfg               config.App
	systemRules       []api.Rule
	resolver          api.CategoryResolver
	st                daemonStore
	runtimeStore      daemonRuntimeStore
	resolverStore     categorySnapshotStore
	diagnostics       daemon.DiagnosticStore
	transactionWriter store.TransactionBatchWriter
	dm                *daemonManager
	logger            *slog.Logger
}

type daemonRun struct {
	readerName    string
	tenant        store.Tenant
	cfg           config.App
	compiledRules []api.Rule
	resolver      api.CategoryResolver
	forceRescan   bool
}

// launch builds runtime config and merged rules then spawns runDaemon in a goroutine.
// Must be called with c.mu held.
func (c *daemonCoordinator) launch(req httpapi.DaemonRunRequest, forceRescan bool) {
	runCtx, cancel := context.WithCancel(c.ctx)
	c.cancelFn = cancel
	c.activeReader = req.Reader
	c.activeTenant = req.Tenant
	runtimeCfg := applyScanOverrides(runCtx, c.cfg, c.st, req.Tenant)

	// Load scanning checkpoint from DB.
	if !forceRescan {
		runtimeCfg.LastScanAt = loadLastScanAt(c.ctx, c.st, req.Tenant, req.Reader, c.logger)
	} else {
		runtimeCfg.ForceFullScan = true
	}

	// OnCheckpoint saves the scan timestamp back to DB after each successful scan.
	runtimeCfg.OnCheckpoint = func(t time.Time) {
		key := "reader." + req.Reader + ".last_scan_at"
		if err := c.st.SetAppConfig(c.ctx, req.Tenant, key, t.Format(time.RFC3339)); err != nil {
			c.logger.Warn("failed to save scan checkpoint", "reader", req.Reader, "error", err)
		} else {
			c.logger.Debug("scan checkpoint saved", "reader", req.Reader, "at", t.Format(time.RFC3339))
		}
	}

	merged := rules.MergeRules(c.systemRules, rules.LoadPersisted(c.ctx, c.st, req.Tenant, c.logger))
	run := daemonRun{
		readerName:    req.Reader,
		tenant:        req.Tenant,
		cfg:           runtimeCfg,
		compiledRules: merged,
		resolver:      c.resolver,
		forceRescan:   forceRescan,
	}
	go func() {
		defer cancel()
		c.runDaemon(runCtx, run)
	}()
}

// loadLastScanAt reads the last scan checkpoint for a reader from app_config.
// Returns nil if no checkpoint exists (first run).
func loadLastScanAt(ctx context.Context, st daemonStore, tenant store.Tenant, readerName string, logger *slog.Logger) *time.Time {
	key := "reader." + readerName + ".last_scan_at"
	val, err := st.GetAppConfig(ctx, tenant, key)
	if err != nil {
		return nil // no checkpoint yet; the reader will use the full lookback window
	}
	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		logger.Warn("invalid scan checkpoint, will do full scan", "reader", readerName, "value", val)
		return nil
	}
	logger.Debug("loaded scan checkpoint", "reader", readerName, "at", t.Format(time.RFC3339))
	return &t
}

// stopCurrent cancels the running daemon and clears cancelFn.
// Must be called with c.mu held.
func (c *daemonCoordinator) stopCurrent() {
	if c.cancelFn != nil {
		c.cancelFn()
		c.cancelFn = nil
	}
}

// waitStopped blocks until the daemon manager reports that the daemon has stopped.
// Must not be called with c.mu held.
func (c *daemonCoordinator) waitStopped() {
	for c.dm.Status().Running {
		time.Sleep(50 * time.Millisecond)
	}
}

// start launches the requested reader, stopping a different active reader first.
func (c *daemonCoordinator) start(req httpapi.DaemonRunRequest) {
	c.mu.Lock()
	if c.dm.Status().Running {
		if c.activeReader == req.Reader && c.activeTenant.ID == req.Tenant.ID {
			c.mu.Unlock()
			return
		}
		c.stopCurrent()
		c.mu.Unlock()
		c.waitStopped()
		c.mu.Lock()
		if c.cancelFn != nil {
			c.mu.Unlock()
			return
		}
	}
	if err := saveActiveReader(c.ctx, c.st, req.Tenant, req.Reader); err != nil {
		c.logger.Warn("failed to persist active reader", "error", err)
	}
	c.launch(req, false)
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
	c.activeTenant = store.Tenant{}
	c.mu.Unlock()
}

// rescan relaunches the reader without checkpoint or deduplication constraints.
func (c *daemonCoordinator) rescan(req httpapi.DaemonRunRequest) {
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
	if err := saveActiveReader(c.ctx, c.st, req.Tenant, req.Reader); err != nil {
		c.logger.Warn("failed to persist active reader", "error", err)
	}
	c.launch(req, true)
	c.mu.Unlock()
}

// restart reloads the persisted checkpoint and retains normal deduplication.
func (c *daemonCoordinator) restart(req httpapi.DaemonRunRequest) {
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
	c.launch(req, false)
	c.mu.Unlock()
}

// RefreshResolver reloads category mappings and restarts an active daemon.
func (c *daemonCoordinator) RefreshResolver(ctx context.Context) {
	resolver, err := c.resolverStore.LoadCategorySnapshot(ctx)
	if err != nil {
		c.logger.Warn("failed to reload category snapshot after sync", "error", err)
		return
	}
	c.mu.Lock()
	c.resolver = resolver
	active := c.activeReader
	tenant := c.activeTenant
	running := c.dm.Status().Running
	c.mu.Unlock()

	if running && active != "" {
		c.logger.Info("restarting daemon to apply updated category resolver")
		c.start(httpapi.DaemonRunRequest{Tenant: tenant, Reader: active})
	}
}

// runDaemon builds the OAuth client and daemon runner, then blocks until ctx is done.
func (c *daemonCoordinator) runDaemon(
	ctx context.Context,
	run daemonRun,
) {
	c.logger.Debug("runDaemon starting", "reader", run.readerName)

	scopes, err := c.registry.GetAllScopes(run.readerName)
	if err != nil {
		c.logger.Error("failed to resolve OAuth scopes", "reader", run.readerName, "error", err)
		c.dm.setStopped(err)
		return
	}
	c.logger.Debug("resolved OAuth scopes", "scopes", scopes)

	var httpClient *http.Client
	if len(scopes) > 0 {
		secretJSON, ok, err := c.runtimeStore.GetReaderSecret(ctx, run.tenant, run.readerName)
		if err != nil {
			c.logger.Error("failed to load OAuth credentials", "reader", run.readerName, "error", err)
			c.dm.setStopped(err)
			return
		}
		if !ok {
			err := fmt.Errorf("credentials not uploaded for reader %q", run.readerName)
			c.logger.Error("failed to create OAuth client — run onboarding first", "error", err)
			c.dm.setStopped(err)
			return
		}
		c.logger.Debug("creating OAuth HTTP client", "reader", run.readerName)
		httpClient, err = oauth.NewFromJSONAndStore(ctx, oauth.StoreClientInput{
			SecretJSON: secretJSON,
			Store:      c.runtimeStore,
			Tenant:     run.tenant,
			Reader:     run.readerName,
			Scopes:     scopes,
		})
		if err != nil {
			c.logger.Error("failed to create OAuth client — run onboarding first", "error", err)
			c.dm.setStopped(err)
			return
		}
		c.logger.Debug("OAuth HTTP client created successfully")
	}

	// Forced rescans intentionally bypass processed-message deduplication.
	var stateManager *state.Manager
	if !run.forceRescan {
		stateManager = state.NewDBManager(c.runtimeStore, run.tenant, c.logger)
	}

	runner := daemon.New(daemon.RunnerDeps{
		Registry:          c.registry,
		TransactionWriter: c.transactionWriter,
		Diagnostics:       c.diagnostics,
		HTTPClient:        httpClient,
		Logger:            c.logger,
	})
	runCfg := daemon.RunConfig{
		ReaderName:   run.readerName,
		Tenant:       run.tenant,
		Config:       &run.cfg,
		Rules:        run.compiledRules,
		Resolver:     run.resolver,
		StateManager: stateManager,
		RuntimeStore: c.runtimeStore,
		ForceRescan:  run.forceRescan,
	}

	c.logger.Info("daemon starting", "reader", run.readerName)
	c.dm.setRunning(time.Now())

	runErr := runner.Run(ctx, runCfg)
	if runErr != nil {
		c.logger.Error("daemon stopped with error", "error", runErr)
	}
	c.dm.setStopped(runErr)
}

// saveActiveReader persists the reader name so startup can resume it.
func saveActiveReader(ctx context.Context, st daemonStore, tenant store.Tenant, readerName string) error {
	return st.SetActiveScanningReader(ctx, tenant, readerName)
}

// applyScanOverrides applies valid UI-managed scan settings to a config copy.
func applyScanOverrides(ctx context.Context, cfg config.App, st daemonStore, tenant store.Tenant) config.App {
	if v, err := getAppConfigWithTimeout(ctx, st, tenant, "scan_interval", cfg.Persisted.ReadTimeout); err == nil {
		if n, convErr := strconv.Atoi(v); convErr == nil && n > 0 {
			cfg.ScanInterval = n
		}
	}
	if v, err := getAppConfigWithTimeout(ctx, st, tenant, "lookback_days", cfg.Persisted.ReadTimeout); err == nil {
		if n, convErr := strconv.Atoi(v); convErr == nil && n > 0 {
			cfg.LookbackDays = n
		}
	}
	return cfg
}

func getAppConfigWithTimeout(ctx context.Context, st daemonStore, tenant store.Tenant, key string, timeout time.Duration) (string, error) {
	readCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return st.GetAppConfig(readCtx, tenant, key)
}
