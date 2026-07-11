// Package community synchronizes community-maintained taxonomy content.
package community

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type syncStore interface {
	SeedMCCCodes(ctx context.Context, entries []store.MCCEntry) error
	SeedMerchantCategories(ctx context.Context, entries []store.MerchantCategoryEntry) (int64, error)
	SetSyncStatus(ctx context.Context, status store.SyncStatus) error
	GetCommunitySyncSettings(ctx context.Context) (store.CommunitySyncSettings, error)
}

type runtimeStore interface {
	GetCommunityURL(ctx context.Context) (string, error)
	SetCommunityURL(ctx context.Context, url string) error
}

// ResolverRefresher reloads category mappings after community content changes.
type ResolverRefresher interface {
	RefreshResolver(ctx context.Context)
}

// Dependencies configures a Service.
type Dependencies struct {
	Config   config.Community
	Client   *http.Client
	Store    syncStore
	Runtime  runtimeStore
	Resolver ResolverRefresher
	Logger   *slog.Logger
	Scope    *observability.Scope
}

// Service owns automatic, periodic, and manually triggered community syncs.
type Service struct {
	rootCtx   context.Context
	config    config.Community
	client    *http.Client
	store     syncStore
	resolver  ResolverRefresher
	logger    *slog.Logger
	scope     *observability.Scope
	newTicker func(time.Duration) serviceTicker
}

type serviceTicker interface {
	C() <-chan time.Time
	Stop()
}

type realTicker struct{ ticker *time.Ticker }

func (t realTicker) C() <-chan time.Time { return t.ticker.C }
func (t realTicker) Stop()               { t.ticker.Stop() }

// New seeds the configured default URL and constructs a Service without starting workers.
func New(ctx context.Context, deps Dependencies) (*Service, error) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	client := deps.Client
	if client == nil {
		client = http.DefaultClient
	}
	scope := deps.Scope
	if scope == nil {
		scope = observability.NewScope(logger, "github.com/ArionMiles/expensor/backend/internal/community")
	}
	if deps.Store == nil || deps.Runtime == nil {
		return nil, errors.E("community.new", errors.FailedPrecondition, "community store dependencies are required")
	}
	if existing, err := deps.Runtime.GetCommunityURL(ctx); err != nil || existing == "" {
		if setErr := deps.Runtime.SetCommunityURL(ctx, deps.Config.URL); setErr != nil {
			logger.Warn("failed to seed default community URL", "error", setErr)
		}
	}
	return &Service{
		rootCtx:  ctx,
		config:   deps.Config,
		client:   client,
		store:    deps.Store,
		resolver: deps.Resolver,
		logger:   logger.With("component", "sync"),
		scope:    scope,
		newTicker: func(interval time.Duration) serviceTicker {
			return realTicker{ticker: time.NewTicker(interval)}
		},
	}, nil
}

// Run performs the initial automatic sync and then blocks while running periodic syncs.
func (s *Service) Run(ctx context.Context) error {
	ticker := s.newTicker(s.config.SyncInterval)
	defer ticker.Stop()
	s.syncAutomaticWithTimeout(ctx)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C():
			syncCtx, cancel := context.WithTimeout(ctx, s.config.SyncTimeout)
			s.syncAutomatic(syncCtx)
			s.refresh(syncCtx)
			cancel()
		}
	}
}

// Trigger performs a manual sync using the application lifetime context.
func (s *Service) Trigger() {
	syncCtx, cancel := context.WithTimeout(s.rootCtx, s.config.SyncTimeout)
	defer cancel()
	s.sync(syncCtx)
	s.refresh(syncCtx)
}

func (s *Service) syncAutomaticWithTimeout(ctx context.Context) {
	syncCtx, cancel := context.WithTimeout(ctx, s.config.SyncTimeout)
	defer cancel()
	s.syncAutomatic(syncCtx)
}

func (s *Service) syncAutomatic(ctx context.Context) {
	settings, err := s.store.GetCommunitySyncSettings(ctx)
	if err != nil {
		s.logger.Warn("failed to read community sync settings", "error", err)
		return
	}
	if settings.AutomaticSyncEnabled != nil && !*settings.AutomaticSyncEnabled {
		s.logger.Info("automatic community sync disabled")
		return
	}
	s.sync(ctx)
}

func (s *Service) sync(ctx context.Context) {
	if s.config.URL == "" {
		return
	}
	ctx, span := s.scope.Start(ctx, "community_sync.sync")
	defer span.End()
	started := time.Now()
	var syncErr error
	defer func() {
		s.scope.RecordDuration(ctx, observability.DurationOperation{
			Namespace: "community_sync",
			Name:      "sync",
			Duration:  time.Since(started),
			Err:       syncErr,
		})
	}()
	s.logger.Info("syncing community content")

	var mccEntries []store.MCCEntry
	if err := s.fetchJSON(ctx, "mcc.json", &mccEntries); err != nil {
		syncErr = err
		s.recordError(ctx, err)
		return
	}
	var categoryEntries []store.MerchantCategoryEntry
	if err := s.fetchJSON(ctx, "categories.json", &categoryEntries); err != nil {
		syncErr = err
		s.recordError(ctx, err)
		return
	}
	if err := s.store.SeedMCCCodes(ctx, mccEntries); err != nil {
		syncErr = err
		s.recordError(ctx, err)
		return
	}
	updated, err := s.store.SeedMerchantCategories(ctx, categoryEntries)
	if err != nil {
		syncErr = err
		s.recordError(ctx, err)
		return
	}

	now := time.Now().UTC()
	if err := s.store.SetSyncStatus(ctx, store.SyncStatus{LastSyncedAt: &now, EntriesUpdated: updated}); err != nil {
		s.logger.Warn("failed to persist sync status", "error", err)
	}
	s.logger.Info("community sync complete", "entries_updated", updated)
}

func (s *Service) fetchJSON(ctx context.Context, path string, dest any) error {
	url := strings.TrimRight(s.config.URL, "/") + "/" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return errors.E("community.fetch", errors.InvalidArgument, "building request", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return errors.E("community.fetch", errors.Unavailable, "fetching community content", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.E("community.fetch", errors.BadGateway, "unexpected community content status")
	}
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return errors.E("community.fetch", errors.InvalidInput, "decoding community content", err)
	}
	return nil
}

func (s *Service) recordError(ctx context.Context, syncErr error) {
	s.logger.Warn("community sync failed", "error", syncErr)
	message := syncErr.Error()
	if err := s.store.SetSyncStatus(ctx, store.SyncStatus{Error: &message}); err != nil {
		s.logger.Warn("failed to persist sync error status", "error", err)
	}
}

func (s *Service) refresh(ctx context.Context) {
	if s.resolver != nil {
		s.resolver.RefreshResolver(ctx)
	}
}
