package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	apperrors "github.com/ArionMiles/expensor/backend/pkg/errors"
)

func TestStartRequiresDependencies(t *testing.T) {
	tests := []struct {
		name      string
		scheduler *Scheduler
		message   string
	}{
		{
			name:      "store",
			scheduler: New(Config{Runner: &staticRunner{}}),
			message:   "scheduler store is nil",
		},
		{
			name:      "runner",
			scheduler: New(Config{Store: newFakeStore(nil)}),
			message:   "scheduler runner is nil",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.scheduler.Start(context.Background())
			if err == nil || err.Error() != tc.message {
				t.Fatalf("Start() error = %v, want %q", err, tc.message)
			}
			if got := apperrors.WhatKind(err); got != apperrors.FailedPrecondition {
				t.Fatalf("WhatKind() = %#v, want %#v", got, apperrors.FailedPrecondition)
			}
		})
	}
}

func TestReconcileStartsRunnableTenantsUpToLimit(t *testing.T) {
	ctx := context.Background()
	fakeStore := newFakeStore([]store.TenantScanningState{
		{TenantID: "tenant-a", ActiveReader: "gmail", Enabled: true, State: store.ScanningStateQueued},
		{TenantID: "tenant-b", ActiveReader: "gmail", Enabled: true, State: store.ScanningStateQueued},
		{TenantID: "tenant-c", ActiveReader: "gmail", Enabled: true, State: store.ScanningStateQueued},
	})
	fakeStore.cfg.MaxConcurrentScans = 2
	runner := newBlockingRunner()
	scheduler := New(Config{
		Store:  fakeStore,
		Runner: runner,
		Clock:  fixedClock{now: time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)},
	})

	if err := scheduler.Reconcile(ctx); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	runner.waitStarted(t, 2)
	if runner.wasStarted("tenant-c") {
		t.Fatal("tenant-c started before capacity was available")
	}

	runner.release()
	fakeStore.waitForState(t, "tenant-a", store.ScanningStateQueued)
	fakeStore.waitForState(t, "tenant-b", store.ScanningStateQueued)
}

func TestRunTenantMapsAuthFailureToNeedsAuth(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	fakeStore := newFakeStore([]store.TenantScanningState{
		{TenantID: "tenant-a", ActiveReader: "gmail", Enabled: true, State: store.ScanningStateQueued},
	})
	runner := &staticRunner{err: NewInvalidGrantFailure(errors.New("oauth2: invalid_grant"))}
	scheduler := New(Config{Store: fakeStore, Runner: runner, Clock: fixedClock{now: now}})

	if err := scheduler.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	state := fakeStore.waitForState(t, "tenant-a", store.ScanningStateNeedsAuth)
	if state.ReasonCode != store.ScanningReasonInvalidGrant {
		t.Fatalf("ReasonCode = %q, want %q", state.ReasonCode, store.ScanningReasonInvalidGrant)
	}
	if state.PublicMessage == "" || state.PublicMessage == "oauth2: invalid_grant" {
		t.Fatalf("PublicMessage = %q, want safe user-facing message", state.PublicMessage)
	}
}

func TestRunTenantBacksOffUnknownFailures(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	fakeStore := newFakeStore([]store.TenantScanningState{
		{TenantID: "tenant-a", ActiveReader: "gmail", Enabled: true, State: store.ScanningStateQueued, RetryCount: 1},
	})
	runner := &staticRunner{err: errors.New("postgres: connection refused")}
	scheduler := New(Config{Store: fakeStore, Runner: runner, Clock: fixedClock{now: now}})

	if err := scheduler.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	state := fakeStore.waitForState(t, "tenant-a", store.ScanningStateBackingOff)
	if state.ReasonCode != store.ScanningReasonTemporaryFailure {
		t.Fatalf("ReasonCode = %q, want %q", state.ReasonCode, store.ScanningReasonTemporaryFailure)
	}
	if state.RetryCount != 2 {
		t.Fatalf("RetryCount = %d, want 2", state.RetryCount)
	}
	if state.NextRetryAt == nil || !state.NextRetryAt.Equal(now.Add(2*time.Minute)) {
		t.Fatalf("NextRetryAt = %v, want %v", state.NextRetryAt, now.Add(2*time.Minute))
	}
}

func TestStartWaitsForCanceledTenantRuns(t *testing.T) {
	fakeStore := newFakeStore([]store.TenantScanningState{
		{TenantID: "tenant-a", ActiveReader: "gmail", Enabled: true, State: store.ScanningStateQueued},
	})
	runner := &cancelGateRunner{started: make(chan struct{}), canceled: make(chan struct{}), release: make(chan struct{})}
	scheduler := New(Config{Store: fakeStore, Runner: runner, PollInterval: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- scheduler.Start(ctx) }()
	<-runner.started
	cancel()
	<-runner.canceled
	select {
	case err := <-done:
		t.Fatalf("Start returned before tenant scan exited: %v", err)
	default:
	}
	close(runner.release)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Start() error = %v, want context.Canceled", err)
	}
}

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

type fakeSchedulerStore struct {
	mu      sync.Mutex
	cfg     store.SchedulerConfig
	users   []store.User
	order   []string
	states  map[string]store.TenantScanningState
	updates chan store.TenantScanningState
}

func newFakeStore(states []store.TenantScanningState) *fakeSchedulerStore {
	fake := &fakeSchedulerStore{
		cfg:     store.SchedulerConfig{MaxConcurrentScans: 4},
		states:  make(map[string]store.TenantScanningState, len(states)),
		updates: make(chan store.TenantScanningState, 32),
	}
	for _, state := range states {
		fake.users = append(fake.users, store.User{ID: state.TenantID, TenantID: state.TenantID})
		fake.order = append(fake.order, state.TenantID)
		fake.states[state.TenantID] = state
	}
	return fake
}

func (s *fakeSchedulerStore) ListUsers(_ context.Context) ([]store.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]store.User(nil), s.users...), nil
}

func (s *fakeSchedulerStore) EnsureScanningStateForTenant(_ context.Context, tenant store.Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.states[tenant.ID]; !ok {
		s.states[tenant.ID] = store.TenantScanningState{TenantID: tenant.ID, Enabled: true, State: store.ScanningStateStopped}
	}
	return nil
}

func (s *fakeSchedulerStore) GetSchedulerConfig(_ context.Context) (store.SchedulerConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg, nil
}

func (s *fakeSchedulerStore) ListRunnableScanningStates(_ context.Context) ([]store.TenantScanningState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	states := make([]store.TenantScanningState, 0, len(s.states))
	for _, tenantID := range s.order {
		state := s.states[tenantID]
		if state.Enabled && state.ActiveReader != "" {
			states = append(states, state)
		}
	}
	return states, nil
}

func (s *fakeSchedulerStore) UpdateScanningState(_ context.Context, tenant store.Tenant, update store.ScanningStateUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.states[tenant.ID]
	state.State = update.State
	state.ReasonCode = update.ReasonCode
	state.PublicMessage = update.PublicMessage
	if update.LastStartedAt != nil {
		state.LastStartedAt = update.LastStartedAt
	}
	if update.LastStoppedAt != nil {
		state.LastStoppedAt = update.LastStoppedAt
	}
	if update.LastFailedAt != nil {
		state.LastFailedAt = update.LastFailedAt
	}
	state.NextRetryAt = update.NextRetryAt
	if update.RetryCount != nil {
		state.RetryCount = *update.RetryCount
	}
	s.states[tenant.ID] = state
	s.updates <- state
	return nil
}

func (s *fakeSchedulerStore) waitForState(t *testing.T, tenantID string, want store.ScanningState) store.TenantScanningState {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for {
		s.mu.Lock()
		current := s.states[tenantID]
		s.mu.Unlock()
		if current.State == want {
			return current
		}
		select {
		case state := <-s.updates:
			if state.TenantID == tenantID && state.State == want {
				return state
			}
		case <-timeout:
			t.Fatalf("timed out waiting for tenant %s state %s", tenantID, want)
		}
	}
}

type blockingRunner struct {
	mu      sync.Mutex
	started map[string]struct{}
	ch      chan struct{}
}

func newBlockingRunner() *blockingRunner {
	return &blockingRunner{started: make(map[string]struct{}), ch: make(chan struct{})}
}

func (r *blockingRunner) Run(_ context.Context, tenant store.Tenant, _ string) error {
	r.mu.Lock()
	r.started[tenant.ID] = struct{}{}
	r.mu.Unlock()
	<-r.ch
	return nil
}

func (r *blockingRunner) waitStarted(t *testing.T, count int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		started := len(r.started)
		r.mu.Unlock()
		if started == count {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d started runners", count)
}

func (r *blockingRunner) wasStarted(tenantID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.started[tenantID]
	return ok
}

func (r *blockingRunner) release() {
	close(r.ch)
}

type staticRunner struct {
	err error
}

type cancelGateRunner struct {
	started  chan struct{}
	canceled chan struct{}
	release  chan struct{}
}

func (r *cancelGateRunner) Run(ctx context.Context, _ store.Tenant, _ string) error {
	close(r.started)
	<-ctx.Done()
	close(r.canceled)
	<-r.release
	return ctx.Err()
}

func (r *staticRunner) Run(context.Context, store.Tenant, string) error {
	return r.err
}
