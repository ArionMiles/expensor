package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const (
	defaultPollInterval = 10 * time.Second
	baseRetryDelay      = time.Minute
	maxRetryDelay       = time.Hour
)

// StateStore is the scheduler persistence surface.
type StateStore interface {
	ListUsers(ctx context.Context) ([]store.User, error)
	EnsureScanningStateForTenant(ctx context.Context, tenant store.Tenant) error
	GetSchedulerConfig(ctx context.Context) (store.SchedulerConfig, error)
	ListRunnableScanningStates(ctx context.Context) ([]store.TenantScanningState, error)
	UpdateScanningState(ctx context.Context, tenant store.Tenant, update store.ScanningStateUpdate) error
}

// Runner executes one bounded scan for a tenant and reader.
type Runner interface {
	Run(ctx context.Context, tenant store.Tenant, reader string) error
}

// Clock isolates time for tests.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

// Scheduler starts fair, bounded tenant scan runs.
type Scheduler struct {
	store        StateStore
	runner       Runner
	clock        Clock
	pollInterval time.Duration
	logger       *slog.Logger

	mu      sync.Mutex
	running map[string]context.CancelFunc
	runs    sync.WaitGroup
}

// Config contains Scheduler dependencies.
type Config struct {
	Store        StateStore
	Runner       Runner
	Clock        Clock
	PollInterval time.Duration
	Logger       *slog.Logger
}

func New(cfg Config) *Scheduler {
	clock := cfg.Clock
	if clock == nil {
		clock = realClock{}
	}
	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		store:        cfg.Store,
		runner:       cfg.Runner,
		clock:        clock,
		pollInterval: pollInterval,
		logger:       logger,
		running:      make(map[string]context.CancelFunc),
	}
}

// Start runs the scheduler loop until ctx is canceled.
func (s *Scheduler) Start(ctx context.Context) error {
	if s.store == nil {
		return errors.E(errors.FailedPrecondition, "scheduler store is nil")
	}
	if s.runner == nil {
		return errors.E(errors.FailedPrecondition, "scheduler runner is nil")
	}
	if err := s.Reconcile(ctx); err != nil {
		s.logger.Error("initial scheduler reconcile failed", "error", err)
	}

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.Stop()
			s.runs.Wait()
			return ctx.Err()
		case <-ticker.C:
			if err := s.Reconcile(ctx); err != nil {
				s.logger.Error("scheduler reconcile failed", "error", err)
			}
		}
	}
}

// Stop cancels currently running scans.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for tenantID, cancel := range s.running {
		cancel()
		delete(s.running, tenantID)
	}
}

// Reconcile seeds missing tenant states and starts queued work up to the configured concurrency limit.
func (s *Scheduler) Reconcile(ctx context.Context) error {
	if err := s.ensureTenantStates(ctx); err != nil {
		return err
	}

	cfg, err := s.store.GetSchedulerConfig(ctx)
	if err != nil {
		return err
	}
	states, err := s.store.ListRunnableScanningStates(ctx)
	if err != nil {
		return err
	}

	capacity := s.availableCapacity(cfg.MaxConcurrentScans)
	for _, state := range states {
		if capacity <= 0 {
			return nil
		}
		if state.ActiveReader == "" || s.isRunning(state.TenantID) {
			continue
		}
		if err := s.startTenant(ctx, state); err != nil {
			return err
		}
		capacity--
	}
	return nil
}

func (s *Scheduler) ensureTenantStates(ctx context.Context) error {
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return err
	}
	for _, user := range users {
		if user.DisabledAt != nil {
			continue
		}
		if err := s.store.EnsureScanningStateForTenant(ctx, store.Tenant{ID: user.TenantID}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) availableCapacity(limit int) int {
	if limit < 1 {
		limit = 1
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return limit - len(s.running)
}

func (s *Scheduler) isRunning(tenantID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.running[tenantID]
	return ok
}

func (s *Scheduler) startTenant(ctx context.Context, state store.TenantScanningState) error {
	tenant := store.Tenant{ID: state.TenantID}
	now := s.clock.Now()
	if err := s.store.UpdateScanningState(ctx, tenant, store.ScanningStateUpdate{
		State:         store.ScanningStateStarting,
		ReasonCode:    store.ScanningReasonNone,
		PublicMessage: "",
		LastStartedAt: &now,
	}); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.running[state.TenantID] = cancel
	s.mu.Unlock()

	s.runs.Add(1)
	go func() {
		defer s.runs.Done()
		s.runTenant(runCtx, state, cancel)
	}()
	return nil
}

func (s *Scheduler) runTenant(ctx context.Context, state store.TenantScanningState, cancel context.CancelFunc) {
	defer cancel()
	defer s.clearRunning(state.TenantID)

	tenant := store.Tenant{ID: state.TenantID}
	startedAt := s.clock.Now()
	if err := s.store.UpdateScanningState(ctx, tenant, store.ScanningStateUpdate{
		State:         store.ScanningStateRunning,
		ReasonCode:    store.ScanningReasonNone,
		PublicMessage: "",
		LastStartedAt: &startedAt,
	}); err != nil {
		s.logger.Error("failed to mark scan running", "error", err)
		return
	}

	err := s.runner.Run(ctx, tenant, state.ActiveReader)
	finishedAt := s.clock.Now()
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		if updateErr := s.store.UpdateScanningState(ctx, tenant, store.ScanningStateUpdate{
			State:         store.ScanningStateQueued,
			ReasonCode:    store.ScanningReasonNone,
			PublicMessage: "",
			LastStoppedAt: &finishedAt,
			RetryCount:    intPtr(0),
		}); updateErr != nil {
			s.logger.Error("failed to mark scan complete", "error", updateErr)
		}
		return
	}

	failure := classifyFailure(err)
	update := failureStateUpdate(failure, state.RetryCount, finishedAt)
	if updateErr := s.store.UpdateScanningState(contextWithoutCancel(ctx), tenant, update); updateErr != nil {
		s.logger.Error("failed to mark scan failed", "error", updateErr)
	}
}

func (s *Scheduler) clearRunning(tenantID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.running, tenantID)
}

func failureStateUpdate(failure FailureError, currentRetry int, now time.Time) store.ScanningStateUpdate {
	switch failure.Kind {
	case FailureNeedsAuth:
		return store.ScanningStateUpdate{
			State:         store.ScanningStateNeedsAuth,
			ReasonCode:    failure.ReasonCode,
			PublicMessage: failure.PublicMessage,
			LastFailedAt:  &now,
			RetryCount:    intPtr(0),
		}
	case FailureReaderNotConfigured:
		return store.ScanningStateUpdate{
			State:         store.ScanningStateReaderNotConfigured,
			ReasonCode:    failure.ReasonCode,
			PublicMessage: failure.PublicMessage,
			LastFailedAt:  &now,
			RetryCount:    intPtr(0),
		}
	default:
		nextRetryCount := currentRetry + 1
		nextRetry := now.Add(retryDelay(nextRetryCount))
		return store.ScanningStateUpdate{
			State:         store.ScanningStateBackingOff,
			ReasonCode:    failure.ReasonCode,
			PublicMessage: failure.PublicMessage,
			LastFailedAt:  &now,
			NextRetryAt:   &nextRetry,
			RetryCount:    &nextRetryCount,
		}
	}
}

func retryDelay(retryCount int) time.Duration {
	if retryCount < 1 {
		retryCount = 1
	}
	delay := baseRetryDelay
	for i := 1; i < retryCount && delay < maxRetryDelay; i++ {
		delay *= 2
	}
	if delay > maxRetryDelay {
		return maxRetryDelay
	}
	return delay
}

func intPtr(value int) *int {
	return &value
}

func contextWithoutCancel(ctx context.Context) context.Context {
	return context.WithoutCancel(ctx)
}
