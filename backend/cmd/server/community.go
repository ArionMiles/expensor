package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/config"
)

type communitySyncStore interface {
	SeedMCCCodes(ctx context.Context, entries []store.MCCEntry) error
	SeedMerchantCategories(ctx context.Context, entries []store.MerchantCategoryEntry) (int64, error)
	SetSyncStatus(ctx context.Context, status store.SyncStatus) error
}

type communitySyncDependencies struct {
	config      config.Community
	store       communitySyncStore
	pgStore     *store.Store
	coordinator *daemonCoordinator
	logger      *slog.Logger
}

// startCommunitySync starts initial and periodic syncs and returns a manual trigger.
func startCommunitySync(ctx context.Context, deps communitySyncDependencies) func() {
	if existing, err := deps.pgStore.GetCommunityURL(ctx); err != nil || existing == "" {
		if setErr := deps.pgStore.SetCommunityURL(ctx, deps.config.URL); setErr != nil {
			deps.logger.Warn("failed to seed default community URL", "error", setErr)
		}
	}
	syncLog := deps.logger.With("component", "sync")
	go func() {
		syncCtx, syncCancel := communitySyncContext(ctx, deps.config.SyncTimeout)
		defer syncCancel()
		syncCommunityContent(syncCtx, deps.store, deps.config.URL, syncLog)
	}()
	go func() {
		ticker := time.NewTicker(deps.config.SyncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				syncCtx, syncCancel := communitySyncContext(ctx, deps.config.SyncTimeout)
				syncCommunityContent(syncCtx, deps.store, deps.config.URL, syncLog)
				deps.coordinator.refreshResolver(syncCtx)
				syncCancel()
			}
		}
	}()
	return func() {
		syncCtx, syncCancel := communitySyncContext(ctx, deps.config.SyncTimeout)
		defer syncCancel()
		syncCommunityContent(syncCtx, deps.store, deps.config.URL, syncLog)
		deps.coordinator.refreshResolver(syncCtx)
	}
}

func communitySyncContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

// syncCommunityContent fetches and persists community-managed taxonomy content.
func syncCommunityContent(ctx context.Context, st communitySyncStore, baseURL string, logger *slog.Logger) {
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

func recordSyncError(ctx context.Context, st communitySyncStore, syncErr error, logger *slog.Logger) {
	logger.Warn("community sync failed", "error", syncErr)
	errStr := syncErr.Error()
	status := store.SyncStatus{Error: &errStr}
	if err := st.SetSyncStatus(ctx, status); err != nil {
		logger.Warn("failed to persist sync error status", "error", err)
	}
}
