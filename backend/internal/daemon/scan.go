package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/oauth"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/rules"
	"github.com/ArionMiles/expensor/backend/internal/state"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

// ScanMode identifies the checkpoint and deduplication policy for a scan.
type ScanMode uint8

const (
	ScanContinuous ScanMode = iota
	ScanScheduled
	ScanRescan
)

// ScanRequest describes one scan execution.
type ScanRequest struct {
	Tenant store.Tenant
	Reader string
	Mode   ScanMode
}

var KindReaderNotConfigured = errors.Kind{Code: "reader_not_configured"}

type scanStore interface {
	rules.PersistedStore
	GetAppConfig(ctx context.Context, tenant store.Tenant, key string) (string, error)
	SetAppConfig(ctx context.Context, tenant store.Tenant, key, value string) error
	GetReaderSecret(ctx context.Context, tenant store.Tenant, reader string) ([]byte, bool, error)
	GetReaderToken(ctx context.Context, tenant store.Tenant, reader string) ([]byte, bool, error)
	SetReaderToken(ctx context.Context, tenant store.Tenant, reader string, token []byte) error
	GetReaderConfig(ctx context.Context, tenant store.Tenant, reader string) (json.RawMessage, bool, error)
	IsMessageProcessed(ctx context.Context, tenant store.Tenant, key string) (bool, error)
	MarkMessageProcessed(ctx context.Context, tenant store.Tenant, key string, at time.Time) error
	LoadCategorySnapshot(ctx context.Context) (api.CategoryResolver, error)
}

// ScanDependencies configures a ScanService.
type ScanDependencies struct {
	Registry          *plugins.Registry
	Config            config.App
	SystemRules       []api.Rule
	Resolver          api.CategoryResolver
	Store             scanStore
	Diagnostics       DiagnosticStore
	TransactionWriter store.TransactionBatchWriter
	Logger            *slog.Logger
}

// ScanService constructs and executes reader scans for every application mode.
type ScanService struct {
	registry          *plugins.Registry
	config            config.App
	systemRules       []api.Rule
	store             scanStore
	diagnostics       DiagnosticStore
	transactionWriter store.TransactionBatchWriter
	logger            *slog.Logger
	resolverMu        sync.RWMutex
	resolver          api.CategoryResolver
	newRunner         func(RunnerDeps) scanRunner
}

type scanRunner interface {
	Run(ctx context.Context, cfg RunConfig) error
}

// NewScanService constructs the shared scan execution service.
func NewScanService(deps ScanDependencies) (*ScanService, error) {
	if deps.Registry == nil || deps.Store == nil || deps.TransactionWriter == nil {
		return nil, errors.E("daemon.scan.new", errors.FailedPrecondition, "scan dependencies are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	service := &ScanService{
		registry: deps.Registry, config: deps.Config, systemRules: deps.SystemRules, resolver: deps.Resolver,
		store: deps.Store, diagnostics: deps.Diagnostics, transactionWriter: deps.TransactionWriter, logger: logger,
	}
	service.newRunner = func(deps RunnerDeps) scanRunner { return New(deps) }
	return service, nil
}

// Run executes one scan according to the requested mode.
func (s *ScanService) Run(ctx context.Context, request ScanRequest) error {
	provider, err := s.registry.GetProvider(request.Reader)
	if err != nil {
		return errors.E("daemon.scan.run", KindReaderNotConfigured, "reader is not registered", err)
	}
	if err := s.ensureReaderReady(ctx, request.Tenant, provider); err != nil {
		return err
	}
	httpClient, err := s.oauthClient(ctx, request.Tenant, request.Reader)
	if err != nil {
		return err
	}

	runtimeConfig := s.scanConfig(ctx, request)
	forceRescan := request.Mode == ScanRescan
	var stateManager *state.Manager
	if !forceRescan {
		stateManager = state.NewDBManager(s.store, request.Tenant, s.logger)
	}
	runner := s.newRunner(RunnerDeps{
		Registry:          s.registry,
		TransactionWriter: s.transactionWriter,
		Diagnostics:       s.diagnostics,
		HTTPClient:        httpClient,
		Logger:            s.logger,
	})
	err = runner.Run(ctx, RunConfig{
		ReaderName: request.Reader, Tenant: request.Tenant, Config: &runtimeConfig,
		Rules:    rules.MergeRules(s.systemRules, rules.LoadPersisted(ctx, s.store, request.Tenant, s.logger)),
		Resolver: s.resolverSnapshot(), StateManager: stateManager, RuntimeStore: s.store, ForceRescan: forceRescan,
	})
	if err != nil {
		return errors.E("daemon.scan.run", err)
	}
	return nil
}

// RefreshResolver reloads the category snapshot used by future scan runs.
func (s *ScanService) RefreshResolver(ctx context.Context) error {
	resolver, err := s.store.LoadCategorySnapshot(ctx)
	if err != nil {
		return errors.E("daemon.scan.refresh_resolver", errors.Internal, "loading category snapshot", err)
	}
	s.resolverMu.Lock()
	s.resolver = resolver
	s.resolverMu.Unlock()
	return nil
}

func (s *ScanService) scanConfig(ctx context.Context, request ScanRequest) config.App {
	runtimeConfig := applyScanOverrides(ctx, s.config, s.store, request.Tenant)
	switch request.Mode {
	case ScanScheduled:
		runtimeConfig.RunOnce = true
		runtimeConfig.LastScanAt = loadLastScanAt(ctx, s.store, request.Tenant, request.Reader, s.logger)
	case ScanRescan:
		runtimeConfig.ForceFullScan = true
	default:
		runtimeConfig.LastScanAt = loadLastScanAt(ctx, s.store, request.Tenant, request.Reader, s.logger)
	}
	runtimeConfig.OnCheckpoint = func(checkpoint time.Time) {
		key := "reader." + request.Reader + ".last_scan_at"
		if err := s.store.SetAppConfig(ctx, request.Tenant, key, checkpoint.Format(time.RFC3339)); err != nil {
			s.logger.Warn("failed to save scan checkpoint", "reader", request.Reader, "error", err)
		}
	}
	return runtimeConfig
}

func (s *ScanService) ensureReaderReady(ctx context.Context, tenant store.Tenant, provider plugins.Provider) error {
	metadata := provider.Metadata
	if metadata.Auth.Type != plugins.AuthTypeConfig || len(metadata.ConfigSchema) == 0 {
		return nil
	}
	rawConfig, ok, err := s.store.GetReaderConfig(ctx, tenant, metadata.Name)
	if err != nil {
		return errors.E("daemon.scan.reader_ready", err)
	}
	if !ok || !readerConfigHasRequiredFields(rawConfig, metadata.ConfigSchema) {
		return errors.E("daemon.scan.reader_ready", KindReaderNotConfigured, fmt.Sprintf("reader %q config is incomplete", metadata.Name))
	}
	return nil
}

func (s *ScanService) oauthClient(ctx context.Context, tenant store.Tenant, reader string) (*http.Client, error) {
	scopes, err := s.registry.GetAllScopes(reader)
	if err != nil {
		return nil, errors.E("daemon.scan.oauth_client", KindReaderNotConfigured, "resolving reader scopes", err)
	}
	if len(scopes) == 0 {
		return nil, nil
	}
	secretJSON, ok, err := s.store.GetReaderSecret(ctx, tenant, reader)
	if err != nil {
		return nil, errors.E("daemon.scan.oauth_client", err)
	}
	if !ok {
		return nil, errors.E("daemon.scan.oauth_client", oauth.KindCredentialsMissing, "reader credentials missing")
	}
	client, err := oauth.NewFromJSONAndStore(ctx, oauth.StoreClientInput{
		SecretJSON: secretJSON, Store: s.store, Tenant: tenant, Reader: reader, Scopes: scopes,
	})
	if err != nil {
		return nil, errors.E("daemon.scan.oauth_client", err)
	}
	return client, nil
}

func (s *ScanService) resolverSnapshot() api.CategoryResolver {
	s.resolverMu.RLock()
	defer s.resolverMu.RUnlock()
	return s.resolver
}

func loadLastScanAt(ctx context.Context, st scanStore, tenant store.Tenant, reader string, logger *slog.Logger) *time.Time {
	value, err := st.GetAppConfig(ctx, tenant, "reader."+reader+".last_scan_at")
	if err != nil {
		return nil
	}
	checkpoint, err := time.Parse(time.RFC3339, value)
	if err != nil {
		logger.Warn("invalid scan checkpoint, will do full scan", "reader", reader, "value", value)
		return nil
	}
	return &checkpoint
}

func applyScanOverrides(ctx context.Context, cfg config.App, st scanStore, tenant store.Tenant) config.App {
	if value, err := getAppConfigWithTimeout(ctx, st, tenant, "scan_interval", cfg.Persisted.ReadTimeout); err == nil {
		if interval, convErr := strconv.Atoi(value); convErr == nil && interval > 0 {
			cfg.ScanInterval = interval
		}
	}
	if value, err := getAppConfigWithTimeout(ctx, st, tenant, "lookback_days", cfg.Persisted.ReadTimeout); err == nil {
		if days, convErr := strconv.Atoi(value); convErr == nil && days > 0 {
			cfg.LookbackDays = days
		}
	}
	return cfg
}

func getAppConfigWithTimeout(ctx context.Context, st scanStore, tenant store.Tenant, key string, timeout time.Duration) (string, error) {
	readCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return st.GetAppConfig(readCtx, tenant, key)
}

func readerConfigHasRequiredFields(rawConfig json.RawMessage, schema []plugins.ConfigField) bool {
	var body struct {
		Config map[string]any `json:"config"`
	}
	if err := json.Unmarshal(rawConfig, &body); err != nil {
		return false
	}
	for _, field := range schema {
		if !field.Required {
			continue
		}
		value, ok := body.Config[field.Key]
		if !ok {
			return false
		}
		if stringValue, ok := value.(string); ok && strings.TrimSpace(stringValue) == "" {
			return false
		}
	}
	return true
}
