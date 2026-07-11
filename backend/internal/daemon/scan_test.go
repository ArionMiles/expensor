package daemon

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/oauth"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type scanStoreStub struct {
	appConfig    map[string]string
	checkpoints  map[string]string
	rules        []store.RuleRow
	readerConfig json.RawMessage
	hasConfig    bool
	secret       []byte
	hasSecret    bool
	resolver     api.CategoryResolver
}

func (s *scanStoreStub) ListRules(context.Context, store.Tenant) ([]store.RuleRow, error) {
	return s.rules, nil
}

func (s *scanStoreStub) GetAppConfig(_ context.Context, _ store.Tenant, key string) (string, error) {
	value, ok := s.appConfig[key]
	if !ok {
		return "", errors.E(errors.NotFound, "not found")
	}
	return value, nil
}

func (s *scanStoreStub) SetAppConfig(_ context.Context, _ store.Tenant, key, value string) error {
	if s.checkpoints == nil {
		s.checkpoints = make(map[string]string)
	}
	s.checkpoints[key] = value
	return nil
}

func (s *scanStoreStub) GetReaderSecret(context.Context, store.Tenant, string) (secret []byte, ok bool, err error) {
	return s.secret, s.hasSecret, nil
}

func (*scanStoreStub) GetReaderToken(context.Context, store.Tenant, string) (token []byte, ok bool, err error) {
	return nil, false, nil
}
func (*scanStoreStub) SetReaderToken(context.Context, store.Tenant, string, []byte) error { return nil }
func (s *scanStoreStub) GetReaderConfig(context.Context, store.Tenant, string) (json.RawMessage, bool, error) {
	return s.readerConfig, s.hasConfig, nil
}

func (*scanStoreStub) IsMessageProcessed(context.Context, store.Tenant, string) (bool, error) {
	return false, nil
}

func (*scanStoreStub) MarkMessageProcessed(context.Context, store.Tenant, string, time.Time) error {
	return nil
}

func (s *scanStoreStub) LoadCategorySnapshot(context.Context) (api.CategoryResolver, error) {
	return s.resolver, nil
}

type transactionWriterStub struct{}

func (transactionWriterStub) Write(context.Context, store.IngestionBatch) error { return nil }

type scanRunnerStub struct {
	configs []RunConfig
	err     error
}

func (r *scanRunnerStub) Run(_ context.Context, cfg RunConfig) error {
	r.configs = append(r.configs, cfg)
	return r.err
}

func TestScanServiceAppliesModePoliciesAndMergesRules(t *testing.T) {
	checkpoint := time.Date(2026, time.July, 1, 10, 0, 0, 0, time.UTC)
	st := &scanStoreStub{
		appConfig: map[string]string{"reader.test.last_scan_at": checkpoint.Format(time.RFC3339)},
		rules: []store.RuleRow{{
			ID: "user-rule", Name: "Bundled", SenderEmail: "user@example.test", AmountRegex: `([0-9]+)`, MerchantRegex: `(shop)`,
		}},
	}
	service := newScanServiceForTest(t, st, testProvider("test", plugins.AuthType(""), nil), []api.Rule{{
		Name: "Bundled", SenderEmail: "system@example.test", Amount: regexp.MustCompile(`(\d+)`), MerchantInfo: regexp.MustCompile(`(shop)`),
	}})
	runner := &scanRunnerStub{}
	service.newRunner = func(*http.Client) scanRunner { return runner }

	for _, mode := range []ScanMode{ScanContinuous, ScanScheduled, ScanRescan} {
		if err := service.Run(context.Background(), ScanRequest{Tenant: store.Tenant{ID: "tenant-a"}, Reader: "test", Mode: mode}); err != nil {
			t.Fatalf("Run(%v) error = %v", mode, err)
		}
	}
	if len(runner.configs) != 3 {
		t.Fatalf("runner calls = %d, want 3", len(runner.configs))
	}
	continuous, scheduled, rescan := runner.configs[0], runner.configs[1], runner.configs[2]
	if continuous.Config.RunOnce || continuous.Config.ForceFullScan || continuous.Config.LastScanAt == nil || continuous.StateManager == nil {
		t.Fatalf("continuous config = %#v", continuous)
	}
	if !scheduled.Config.RunOnce || scheduled.Config.ForceFullScan || scheduled.Config.LastScanAt == nil || scheduled.StateManager == nil {
		t.Fatalf("scheduled config = %#v", scheduled)
	}
	if rescan.Config.RunOnce || !rescan.Config.ForceFullScan || rescan.Config.LastScanAt != nil || rescan.StateManager != nil || !rescan.ForceRescan {
		t.Fatalf("rescan config = %#v", rescan)
	}
	if len(continuous.Rules) != 1 || continuous.Rules[0].ID != "user-rule" {
		t.Fatalf("merged rules = %#v", continuous.Rules)
	}
	scheduled.Config.OnCheckpoint(checkpoint.Add(time.Hour))
	if st.checkpoints["reader.test.last_scan_at"] != checkpoint.Add(time.Hour).Format(time.RFC3339) {
		t.Fatalf("saved checkpoints = %#v", st.checkpoints)
	}
}

func TestScanServiceRejectsIncompleteReaderConfig(t *testing.T) {
	st := &scanStoreStub{appConfig: map[string]string{}, hasConfig: true, readerConfig: json.RawMessage(`{"config":{"profilePath":""}}`)}
	service := newScanServiceForTest(t, st, testProvider("configured", plugins.AuthTypeConfig, []plugins.ConfigField{
		{Key: "profilePath", Required: true},
	}), nil)
	err := service.Run(context.Background(), ScanRequest{Tenant: store.Tenant{ID: "tenant-a"}, Reader: "configured", Mode: ScanScheduled})
	if errors.WhatKind(err) != KindReaderNotConfigured {
		t.Fatalf("Run() error kind = %v, error = %v", errors.WhatKind(err), err)
	}
}

func TestScanServiceClassifiesMissingOAuthCredentials(t *testing.T) {
	provider := testProvider("oauth", plugins.AuthTypeOAuth, nil)
	provider.Metadata.Auth.RequiredScopes = []string{"mail.read"}
	service := newScanServiceForTest(t, &scanStoreStub{appConfig: map[string]string{}}, provider, nil)
	err := service.Run(context.Background(), ScanRequest{Tenant: store.Tenant{ID: "tenant-a"}, Reader: "oauth", Mode: ScanScheduled})
	if errors.WhatKind(err) != oauth.KindCredentialsMissing {
		t.Fatalf("Run() error kind = %v, error = %v", errors.WhatKind(err), err)
	}
}

func TestScanServiceRefreshesResolverSnapshot(t *testing.T) {
	resolver := func(string) (string, string) { return "Dining", "Wants" }
	st := &scanStoreStub{appConfig: map[string]string{}, resolver: resolver}
	service := newScanServiceForTest(t, st, testProvider("test", plugins.AuthType(""), nil), nil)
	if err := service.RefreshResolver(context.Background()); err != nil {
		t.Fatalf("RefreshResolver() error = %v", err)
	}
	category, bucket := service.resolverSnapshot()("merchant")
	if category != "Dining" || bucket != "Wants" {
		t.Fatalf("resolver result = %q, %q", category, bucket)
	}
}

func newScanServiceForTest(t *testing.T, st scanStore, provider plugins.Provider, systemRules []api.Rule) *ScanService {
	t.Helper()
	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider(provider); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	service, err := NewScanService(ScanDependencies{
		Registry: registry, Config: config.App{Persisted: config.Persisted{ReadTimeout: time.Second}},
		SystemRules: systemRules, Store: st, TransactionWriter: transactionWriterStub{},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("NewScanService() error = %v", err)
	}
	return service
}

func testProvider(name string, authType plugins.AuthType, schema []plugins.ConfigField) plugins.Provider {
	return plugins.Provider{
		Metadata:         plugins.ProviderMetadata{Name: name, Auth: plugins.AuthSpec{Type: authType}, ConfigSchema: schema},
		NewReader:        func(plugins.ProviderInput) (api.Reader, error) { return nil, nil },
		NewEmailSearcher: func(plugins.ProviderInput) (api.EmailSearcher, error) { return nil, nil },
	}
}
