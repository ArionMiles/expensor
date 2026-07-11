package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

// RunRequest identifies the tenant and reader for an interactive daemon action.
type RunRequest struct {
	Tenant store.Tenant
	Reader string
}

// Status describes the current interactive daemon run.
type Status struct {
	Running   bool
	StartedAt *time.Time
	LastError string
}

type scanExecutor interface {
	Run(ctx context.Context, request ScanRequest) error
	RefreshResolver(ctx context.Context) error
}

type activeReaderStore interface {
	SetActiveScanningReader(ctx context.Context, tenant store.Tenant, reader string) error
}

// ControllerDependencies configures a Controller.
type ControllerDependencies struct {
	Context context.Context
	Scanner scanExecutor
	Store   activeReaderStore
	Logger  *slog.Logger
}

// Controller owns interactive daemon start, stop, restart, rescan, and status behavior.
type Controller struct {
	mu      sync.RWMutex
	actions chan struct{}
	ctx     context.Context
	scanner scanExecutor
	store   activeReaderStore
	logger  *slog.Logger

	cancel       context.CancelFunc
	done         chan struct{}
	activeReader string
	activeTenant store.Tenant
	running      bool
	startedAt    *time.Time
	lastError    string
}

// NewController constructs an interactive daemon controller.
func NewController(deps ControllerDependencies) (*Controller, error) {
	if deps.Context == nil || deps.Scanner == nil || deps.Store == nil {
		return nil, errors.E("daemon.controller.new", errors.FailedPrecondition, "controller dependencies are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	controller := &Controller{
		ctx: deps.Context, scanner: deps.Scanner, store: deps.Store, logger: logger,
		actions: make(chan struct{}, 1),
	}
	controller.actions <- struct{}{}
	return controller, nil
}

// Start launches a continuous scan, switching readers when necessary.
func (c *Controller) Start(request RunRequest) {
	go func() {
		_ = c.withAction(context.Background(), func() error {
			c.transitionStart(request)
			return nil
		})
	}()
}

func (c *Controller) transitionStart(request RunRequest) {
	c.mu.RLock()
	sameRun := c.running && c.activeReader == request.Reader && c.activeTenant.ID == request.Tenant.ID
	running := c.running
	c.mu.RUnlock()
	if sameRun {
		return
	}
	if running {
		_ = c.stopCurrent(context.Background())
	}
	c.persistActiveReader(request)
	c.launch(request, ScanContinuous)
}

// Stop cancels and waits for the active daemon run.
func (c *Controller) Stop() {
	go func() {
		_ = c.withAction(context.Background(), func() error {
			c.transitionStop()
			return nil
		})
	}()
}

func (c *Controller) transitionStop() {
	_ = c.stopCurrent(context.Background())
	c.mu.Lock()
	c.activeReader = ""
	c.activeTenant = store.Tenant{}
	c.mu.Unlock()
}

// Rescan cancels the active run and launches a full-lookback scan without deduplication.
func (c *Controller) Rescan(request RunRequest) {
	go func() {
		_ = c.withAction(context.Background(), func() error {
			c.transitionRescan(request)
			return nil
		})
	}()
}

func (c *Controller) transitionRescan(request RunRequest) {
	_ = c.stopCurrent(context.Background())
	c.persistActiveReader(request)
	c.launch(request, ScanRescan)
}

// Restart reloads persisted scan state and launches a normal continuous scan.
func (c *Controller) Restart(request RunRequest) {
	go func() {
		_ = c.withAction(context.Background(), func() error {
			c.transitionRestart(request)
			return nil
		})
	}()
}

func (c *Controller) transitionRestart(request RunRequest) {
	_ = c.stopCurrent(context.Background())
	c.launch(request, ScanContinuous)
}

// RefreshResolver reloads the category resolver and restarts an active daemon.
func (c *Controller) RefreshResolver(ctx context.Context) {
	if err := c.scanner.RefreshResolver(ctx); err != nil {
		c.logger.Warn("failed to reload category snapshot after sync", "error", err)
		return
	}
	c.mu.RLock()
	running := c.running
	request := RunRequest{Tenant: c.activeTenant, Reader: c.activeReader}
	c.mu.RUnlock()
	if running && request.Reader != "" {
		c.logger.Info("restarting daemon to apply updated category resolver")
		c.Restart(request)
	}
}

// Status returns a snapshot of the interactive daemon state.
func (c *Controller) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Status{Running: c.running, StartedAt: c.startedAt, LastError: c.lastError}
}

// Close cancels active work and waits until it stops or ctx expires.
func (c *Controller) Close(ctx context.Context) error {
	if err := c.withAction(ctx, func() error { return c.stopCurrent(ctx) }); err != nil {
		return errors.E("daemon.controller.close", err)
	}
	return nil
}

func (c *Controller) launch(request RunRequest, mode ScanMode) {
	runCtx, cancel := context.WithCancel(c.ctx)
	done := make(chan struct{})
	startedAt := time.Now()
	c.mu.Lock()
	c.cancel = cancel
	c.done = done
	c.activeReader = request.Reader
	c.activeTenant = request.Tenant
	c.running = true
	c.startedAt = &startedAt
	c.lastError = ""
	c.mu.Unlock()

	go func() {
		err := c.scanner.Run(runCtx, ScanRequest{Tenant: request.Tenant, Reader: request.Reader, Mode: mode})
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			c.logger.Error("daemon stopped with error", "error", err)
		}
		c.mu.Lock()
		if c.done == done {
			c.running = false
			c.cancel = nil
			if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				c.lastError = err.Error()
			}
		}
		close(done)
		c.mu.Unlock()
	}()
}

func (c *Controller) stopCurrent(ctx context.Context) error {
	c.mu.RLock()
	cancel := c.cancel
	done := c.done
	c.mu.RUnlock()
	if cancel == nil || done == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Controller) persistActiveReader(request RunRequest) {
	if err := c.store.SetActiveScanningReader(c.ctx, request.Tenant, request.Reader); err != nil {
		c.logger.Warn("failed to persist active reader", "error", err)
	}
}

func (c *Controller) withAction(ctx context.Context, action func() error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.actions:
	}
	defer func() { c.actions <- struct{}{} }()
	return action()
}
