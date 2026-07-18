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
	ctx     context.Context
	cancel  context.CancelFunc
	scanner scanExecutor
	store   activeReaderStore
	logger  *slog.Logger

	queueMu       sync.Mutex
	queue         []func(context.Context)
	wake          chan struct{}
	workerOnce    sync.Once
	workerStarted bool
	workerDone    chan struct{}
	closed        bool

	runCancel    context.CancelFunc
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
	controllerCtx, cancel := context.WithCancel(deps.Context)
	controller := &Controller{
		ctx: controllerCtx, cancel: cancel, scanner: deps.Scanner, store: deps.Store, logger: logger,
		wake: make(chan struct{}, 1), workerDone: make(chan struct{}),
	}
	return controller, nil
}

// Start launches a continuous scan, switching readers when necessary.
func (c *Controller) Start(request RunRequest) {
	_ = c.enqueue(func(ctx context.Context) { c.transitionStart(ctx, request) })
}

func (c *Controller) transitionStart(ctx context.Context, request RunRequest) {
	c.mu.RLock()
	sameRun := c.running && c.activeReader == request.Reader && c.activeTenant.ID == request.Tenant.ID
	running := c.running
	c.mu.RUnlock()
	if sameRun {
		return
	}
	if running {
		if err := c.stopCurrent(ctx); err != nil {
			return
		}
	}
	if ctx.Err() != nil {
		return
	}
	c.persistActiveReader(ctx, request)
	c.launch(request, ScanContinuous)
}

// Stop cancels and waits for the active daemon run.
func (c *Controller) Stop() {
	done := make(chan struct{})
	if !c.enqueue(func(ctx context.Context) {
		c.transitionStop(ctx)
		close(done)
	}) {
		return
	}
	select {
	case <-done:
	case <-c.ctx.Done():
	}
}

func (c *Controller) transitionStop(ctx context.Context) {
	if err := c.stopCurrent(ctx); err != nil {
		return
	}
	c.mu.Lock()
	c.activeReader = ""
	c.activeTenant = store.Tenant{}
	c.mu.Unlock()
}

// Rescan cancels the active run and launches a full-lookback scan without deduplication.
func (c *Controller) Rescan(request RunRequest) {
	_ = c.enqueue(func(ctx context.Context) { c.transitionRescan(ctx, request) })
}

func (c *Controller) transitionRescan(ctx context.Context, request RunRequest) {
	if err := c.stopCurrent(ctx); err != nil || ctx.Err() != nil {
		return
	}
	c.persistActiveReader(ctx, request)
	c.launch(request, ScanRescan)
}

// Restart reloads persisted scan state and launches a normal continuous scan.
func (c *Controller) Restart(request RunRequest) {
	_ = c.enqueue(func(ctx context.Context) { c.transitionRestart(ctx, request) })
}

func (c *Controller) transitionRestart(ctx context.Context, request RunRequest) {
	if err := c.stopCurrent(ctx); err != nil || ctx.Err() != nil {
		return
	}
	c.launch(request, ScanContinuous)
}

// RefreshResolver reloads the category resolver and restarts an active daemon.
func (c *Controller) RefreshResolver(ctx context.Context) {
	c.queueMu.Lock()
	closed := c.closed
	c.queueMu.Unlock()
	if closed {
		return
	}
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
	c.queueMu.Lock()
	if !c.closed {
		c.closed = true
		c.queue = nil
		c.cancel()
	}
	workerStarted := c.workerStarted
	c.queueMu.Unlock()
	c.signalWorker()

	if workerStarted {
		select {
		case <-c.workerDone:
		case <-ctx.Done():
			return errors.E("daemon.controller.close", ctx.Err())
		}
	}
	if err := c.stopCurrent(ctx); err != nil {
		return errors.E("daemon.controller.close", err)
	}
	return nil
}

func (c *Controller) launch(request RunRequest, mode ScanMode) {
	runCtx, cancel := context.WithCancel(c.ctx)
	done := make(chan struct{})
	startedAt := time.Now()
	c.mu.Lock()
	c.runCancel = cancel
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
			c.runCancel = nil
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
	cancel := c.runCancel
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

func (c *Controller) persistActiveReader(ctx context.Context, request RunRequest) {
	if err := c.store.SetActiveScanningReader(ctx, request.Tenant, request.Reader); err != nil {
		c.logger.Warn("failed to persist active reader", "error", err)
	}
}

func (c *Controller) enqueue(action func(context.Context)) bool {
	c.queueMu.Lock()
	if c.closed {
		c.queueMu.Unlock()
		return false
	}
	c.workerOnce.Do(func() {
		c.workerStarted = true
		go c.runActions()
	})
	c.queue = append(c.queue, action)
	c.queueMu.Unlock()
	c.signalWorker()
	return true
}

func (c *Controller) runActions() {
	defer close(c.workerDone)
	for {
		c.queueMu.Lock()
		if len(c.queue) > 0 {
			action := c.queue[0]
			c.queue = c.queue[1:]
			c.queueMu.Unlock()
			action(c.ctx)
			continue
		}
		closed := c.closed
		c.queueMu.Unlock()
		if closed || c.ctx.Err() != nil {
			return
		}
		select {
		case <-c.ctx.Done():
		case <-c.wake:
		}
	}
}

func (c *Controller) signalWorker() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}
