package community

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/config"
)

type fakeStore struct {
	settings          store.CommunitySyncSettings
	settingsErr       error
	seedMCCErr        error
	seedCategoriesErr error
	statuses          []store.SyncStatus
	defaultURL        string
	seededURL         string
	mccEntries        []store.MCCEntry
	categoryEntries   []store.MerchantCategoryEntry
	statusNotify      chan struct{}
}

func (s *fakeStore) SeedMCCCodes(_ context.Context, entries []store.MCCEntry) error {
	s.mccEntries = entries
	return s.seedMCCErr
}

func (s *fakeStore) SeedMerchantCategories(_ context.Context, entries []store.MerchantCategoryEntry) (int64, error) {
	s.categoryEntries = entries
	return 2, s.seedCategoriesErr
}

func (s *fakeStore) SetSyncStatus(_ context.Context, status store.SyncStatus) error {
	s.statuses = append(s.statuses, status)
	if s.statusNotify != nil {
		s.statusNotify <- struct{}{}
	}
	return nil
}

func (s *fakeStore) GetCommunitySyncSettings(context.Context) (store.CommunitySyncSettings, error) {
	return s.settings, s.settingsErr
}

func (s *fakeStore) GetCommunityURL(context.Context) (string, error) { return s.defaultURL, nil }

func (s *fakeStore) SetCommunityURL(_ context.Context, url string) error {
	s.seededURL = url
	return nil
}

type fakeRefresher struct{ calls int }

func (r *fakeRefresher) RefreshResolver(context.Context) { r.calls++ }

func TestNewSeedsDefaultURL(t *testing.T) {
	st := &fakeStore{}
	_, err := New(context.Background(), Dependencies{Config: config.Community{URL: "https://example.test/content"}, Store: st, Runtime: st})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if st.seededURL != "https://example.test/content" {
		t.Fatalf("seeded URL = %q", st.seededURL)
	}
}

func TestAutomaticDisabledSkipsFetch(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer server.Close()
	disabled := false
	st := &fakeStore{settings: store.CommunitySyncSettings{AutomaticSyncEnabled: &disabled}}
	service := newTestService(t, server.URL, st, nil)
	service.syncAutomatic(context.Background())
	if called {
		t.Fatal("community content server was called with automatic sync disabled")
	}
}

func TestSyncPersistsSuccessfulContent(t *testing.T) {
	server := contentServer(t)
	defer server.Close()
	st := &fakeStore{}
	newTestService(t, server.URL, st, nil).sync(context.Background())
	if len(st.statuses) != 1 || st.statuses[0].LastSyncedAt == nil || st.statuses[0].EntriesUpdated != 2 {
		t.Fatalf("statuses = %#v", st.statuses)
	}
	if len(st.mccEntries) != 1 || st.mccEntries[0].Code != "5411" || st.mccEntries[0].Category != "Groceries" {
		t.Fatalf("MCC entries = %#v", st.mccEntries)
	}
	if len(st.categoryEntries) != 1 || st.categoryEntries[0].Fragment != "shop" || st.categoryEntries[0].Category == nil ||
		*st.categoryEntries[0].Category != "Groceries" {
		t.Fatalf("category entries = %#v", st.categoryEntries)
	}
}

func TestSyncRecordsFetchFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no", http.StatusBadGateway) }))
	defer server.Close()
	st := &fakeStore{}
	newTestService(t, server.URL, st, nil).sync(context.Background())
	if len(st.statuses) != 1 || st.statuses[0].Error == nil {
		t.Fatalf("statuses = %#v", st.statuses)
	}
}

func TestSyncRecordsPersistenceFailure(t *testing.T) {
	server := contentServer(t)
	defer server.Close()
	st := &fakeStore{seedMCCErr: errors.New("write failed")}
	newTestService(t, server.URL, st, nil).sync(context.Background())
	if len(st.statuses) != 1 || st.statuses[0].Error == nil {
		t.Fatalf("statuses = %#v", st.statuses)
	}
}

func TestRunCancelsInFlightRequest(t *testing.T) {
	requestStarted := make(chan struct{})
	requestCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done()
		close(requestCanceled)
	}))
	defer server.Close()
	service := newTestService(t, server.URL, &fakeStore{}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()
	<-requestStarted
	cancel()
	select {
	case <-requestCanceled:
	case <-time.After(time.Second):
		t.Fatal("HTTP request was not canceled")
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}

func TestRunAnchorsTickerBeforeInitialSync(t *testing.T) {
	initialStarted := make(chan struct{})
	releaseInitial := make(chan struct{})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			close(initialStarted)
			<-releaseInitial
		}
		serveContent(w, r)
	}))
	defer server.Close()

	st := &fakeStore{statusNotify: make(chan struct{}, 2)}
	service := newTestService(t, server.URL, st, nil)
	ticker := &fakeTicker{ticks: make(chan time.Time, 1)}
	service.newTicker = func(time.Duration) serviceTicker { return ticker }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()
	<-initialStarted
	ticker.ticks <- time.Now()
	close(releaseInitial)
	<-st.statusNotify
	<-st.statusNotify
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestTriggerSyncsAndRefreshesResolver(t *testing.T) {
	server := contentServer(t)
	defer server.Close()
	st := &fakeStore{}
	refresher := &fakeRefresher{}
	newTestService(t, server.URL, st, refresher).Trigger()
	if len(st.statuses) != 1 || refresher.calls != 1 {
		t.Fatalf("statuses = %#v, refresh calls = %d", st.statuses, refresher.calls)
	}
}

func TestCloseCancelsAndWaitsForManualSync(t *testing.T) {
	requestStarted := make(chan struct{})
	requestCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done()
		close(requestCanceled)
	}))
	defer server.Close()
	service := newTestService(t, server.URL, &fakeStore{}, nil)
	triggerDone := make(chan struct{})
	go func() {
		service.Trigger()
		close(triggerDone)
	}()
	<-requestStarted
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := service.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	select {
	case <-requestCanceled:
	case <-time.After(time.Second):
		t.Fatal("manual sync request was not canceled")
	}
	select {
	case <-triggerDone:
	default:
		t.Fatal("Close returned before the manual sync exited")
	}
}

type fakeTicker struct{ ticks chan time.Time }

func (t *fakeTicker) C() <-chan time.Time { return t.ticks }
func (*fakeTicker) Stop()                 {}

func newTestService(t *testing.T, url string, st *fakeStore, refresher ResolverRefresher) *Service {
	t.Helper()
	service, err := New(context.Background(), Dependencies{
		Config: config.Community{URL: url, SyncInterval: time.Hour, SyncTimeout: time.Second},
		Store:  st, Runtime: st, Resolver: refresher, Logger: slog.Default(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return service
}

func contentServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(serveContent))
}

func serveContent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/mcc.json":
		_, _ = w.Write([]byte(`[{"code":"5411","description":"Grocery Stores","category":"Groceries","bucket":"Needs"}]`))
	case "/categories.json":
		_, _ = w.Write([]byte(`[{"fragment":"shop","category":"Groceries"}]`))
	default:
		http.NotFound(w, r)
	}
}
