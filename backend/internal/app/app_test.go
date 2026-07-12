package app

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClosesStoreAfterPartialConstructionFailure(t *testing.T) {
	original := openApplicationStore
	t.Cleanup(func() { openApplicationStore = original })
	var closes atomic.Int32
	openApplicationStore = func(context.Context, StoreOptions) (Store, error) {
		return Store{close: func() { closes.Add(1) }}, nil
	}
	if _, err := New(context.Background(), Options{Logger: discardLogger()}); err == nil {
		t.Fatal("New() error = nil")
	}
	if closes.Load() != 1 {
		t.Fatalf("store closes = %d, want 1", closes.Load())
	}
}

func TestRunCancelsWorkers(t *testing.T) {
	workerStopped := make(chan struct{}, 2)
	waitForCancel := func(ctx context.Context) error {
		<-ctx.Done()
		workerStopped <- struct{}{}
		return ctx.Err()
	}
	application := &App{
		logger: discardLogger(), schedulerRun: waitForCancel, communityRun: waitForCancel,
		serverRun: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() },
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	<-workerStopped
	<-workerStopped
}

func TestRunLogsWorkerFailureWhileHTTPContinues(t *testing.T) {
	var logs lockedBuffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	serverStarted := make(chan struct{})
	application := &App{
		logger:       logger,
		schedulerRun: func(context.Context) error { return errors.New("scheduler failed") },
		communityRun: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() },
		serverRun:    func(ctx context.Context) error { close(serverStarted); <-ctx.Done(); return ctx.Err() },
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	<-serverStarted
	deadline := time.Now().Add(time.Second)
	for !strings.Contains(logs.String(), "scheduler stopped with error") && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !strings.Contains(logs.String(), "scheduler stopped with error") {
		t.Fatalf("logs = %s", logs.String())
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunPropagatesHTTPFailure(t *testing.T) {
	httpErr := errors.New("listen failed")
	waitForCancel := func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
	application := &App{
		logger: discardLogger(), schedulerRun: waitForCancel, communityRun: waitForCancel,
		serverRun: func(context.Context) error { return httpErr },
	}
	if err := application.Run(context.Background()); !errors.Is(err, httpErr) {
		t.Fatalf("Run() error = %v, want wrapped HTTP error", err)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	var controllerCloses atomic.Int32
	var communityCloses atomic.Int32
	var storeCloses atomic.Int32
	application := &App{
		controllerClose: func(context.Context) error { controllerCloses.Add(1); return nil },
		communityClose:  func(context.Context) error { communityCloses.Add(1); return nil },
		storeClose:      func() { storeCloses.Add(1) },
	}
	if err := application.Close(context.Background()); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := application.Close(context.Background()); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if controllerCloses.Load() != 1 || communityCloses.Load() != 1 || storeCloses.Load() != 1 {
		t.Fatalf("controller closes = %d, community closes = %d, store closes = %d",
			controllerCloses.Load(), communityCloses.Load(), storeCloses.Load())
	}
}

func TestCloseHonorsContextAndStillClosesStore(t *testing.T) {
	var storeCloses atomic.Int32
	application := &App{
		controllerClose: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() },
		storeClose:      func() { storeCloses.Add(1) },
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if err := application.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close() error = %v, want deadline exceeded", err)
	}
	if storeCloses.Load() != 1 {
		t.Fatalf("store closes = %d, want 1", storeCloses.Load())
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(data)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}
