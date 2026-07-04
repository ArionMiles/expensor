package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

type fakeCommunitySyncStore struct {
	settings store.CommunitySyncSettings
}

func (s *fakeCommunitySyncStore) SeedMCCCodes(context.Context, []store.MCCEntry) error {
	return nil
}

func (s *fakeCommunitySyncStore) SeedMerchantCategories(context.Context, []store.MerchantCategoryEntry) (int64, error) {
	return 0, nil
}

func (s *fakeCommunitySyncStore) SetSyncStatus(context.Context, store.SyncStatus) error {
	return nil
}

func (s *fakeCommunitySyncStore) GetCommunitySyncSettings(context.Context) (store.CommunitySyncSettings, error) {
	return s.settings, nil
}

func TestSyncCommunityContentIfAutomaticDisabledSkipsFetch(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	defer server.Close()

	disabled := false
	st := &fakeCommunitySyncStore{
		settings: store.CommunitySyncSettings{AutomaticSyncEnabled: &disabled},
	}

	syncCommunityContentIfAutomaticEnabled(context.Background(), st, server.URL, slog.Default())

	if called {
		t.Fatalf("community content server was called with automatic sync disabled")
	}
}
