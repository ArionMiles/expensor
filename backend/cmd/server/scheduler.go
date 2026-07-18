package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/daemon"
	scanscheduler "github.com/ArionMiles/expensor/backend/internal/daemon/scheduler"
	"github.com/ArionMiles/expensor/backend/internal/oauth"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/rules"
	"github.com/ArionMiles/expensor/backend/internal/state"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type scheduledScanRunner struct {
	registry          *plugins.Registry
	cfg               config.App
	systemRules       []api.Rule
	resolver          api.CategoryResolver
	st                daemonStore
	runtimeStore      daemonRuntimeStore
	diagnostics       daemon.DiagnosticStore
	transactionWriter store.TransactionBatchWriter
	logger            *slog.Logger
}

func (r *scheduledScanRunner) Run(ctx context.Context, tenant store.Tenant, readerName string) error {
	provider, err := r.registry.GetProvider(readerName)
	if err != nil {
		return scanscheduler.NewReaderNotConfiguredFailure(err)
	}
	if err := r.ensureReaderReady(ctx, tenant, provider); err != nil {
		return err
	}

	httpClient, err := r.oauthClient(ctx, tenant, readerName)
	if err != nil {
		return err
	}

	runCfg := applyScanOverrides(ctx, r.cfg, r.st, tenant)
	runCfg.RunOnce = true
	runCfg.LastScanAt = loadLastScanAt(ctx, r.st, tenant, readerName, r.logger)
	runCfg.OnCheckpoint = func(t time.Time) {
		key := "reader." + readerName + ".last_scan_at"
		if err := r.st.SetAppConfig(ctx, tenant, key, t.Format(time.RFC3339)); err != nil {
			r.logger.Warn("failed to save scan checkpoint", "reader", readerName, "error", err)
		}
	}

	runner := daemon.New(daemon.RunnerDeps{
		Registry:          r.registry,
		TransactionWriter: r.transactionWriter,
		Diagnostics:       r.diagnostics,
		HTTPClient:        httpClient,
		Logger:            r.logger,
	})
	err = runner.Run(ctx, daemon.RunConfig{
		ReaderName:   readerName,
		Tenant:       tenant,
		Config:       &runCfg,
		Rules:        rules.MergeRules(r.systemRules, loadUserRules(ctx, r.st, tenant, r.logger)),
		Resolver:     r.resolver,
		StateManager: state.NewDBManager(r.runtimeStore, tenant, r.logger),
		RuntimeStore: r.runtimeStore,
		ForceRescan:  false,
	})
	if err != nil && oauth.IsInvalidGrant(err) {
		return scanscheduler.NewInvalidGrantFailure(err)
	}
	return err
}

func (r *scheduledScanRunner) ensureReaderReady(ctx context.Context, tenant store.Tenant, provider plugins.Provider) error {
	meta := provider.Metadata
	if meta.Auth.Type != plugins.AuthTypeConfig || len(meta.ConfigSchema) == 0 {
		return nil
	}
	rawConfig, ok, err := r.runtimeStore.GetReaderConfig(ctx, tenant, meta.Name)
	if err != nil {
		return err
	}
	if !ok || !readerConfigHasRequiredFields(rawConfig, meta.ConfigSchema) {
		return scanscheduler.NewReaderNotConfiguredFailure(fmt.Errorf("reader %q config is incomplete", meta.Name))
	}
	return nil
}

func (r *scheduledScanRunner) oauthClient(ctx context.Context, tenant store.Tenant, readerName string) (*http.Client, error) {
	scopes, err := r.registry.GetAllScopes(readerName)
	if err != nil {
		return nil, scanscheduler.NewReaderNotConfiguredFailure(err)
	}
	if len(scopes) == 0 {
		return nil, nil
	}

	secretJSON, ok, err := r.runtimeStore.GetReaderSecret(ctx, tenant, readerName)
	if err != nil {
		return nil, err
	}
	if !ok {
		err := errors.E("server.scheduledScanRunner.oauthClient", oauth.KindCredentialsMissing, "reader credentials missing")
		return nil, scanscheduler.NewMissingCredentialsFailure(err)
	}

	httpClient, err := oauth.NewFromJSONAndStore(ctx, oauth.StoreClientInput{
		SecretJSON: secretJSON,
		Store:      r.runtimeStore,
		Tenant:     tenant,
		Reader:     readerName,
		Scopes:     scopes,
	})
	if err == nil {
		return httpClient, nil
	}
	switch {
	case errors.WhatKind(err) == oauth.KindTokenMissing:
		return nil, scanscheduler.NewMissingTokenFailure(err)
	case oauth.IsInvalidGrant(err):
		return nil, scanscheduler.NewInvalidGrantFailure(err)
	default:
		return nil, err
	}
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
		if s, ok := value.(string); ok && strings.TrimSpace(s) == "" {
			return false
		}
	}
	return true
}
