package daemon

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

type controllerScannerStub struct {
	run        func(context.Context, ScanRequest) error
	refreshErr error
	mu         sync.Mutex
	refreshes  int
}

func (s *controllerScannerStub) Run(ctx context.Context, request ScanRequest) error {
	return s.run(ctx, request)
}

func (s *controllerScannerStub) RefreshResolver(context.Context) error {
	s.mu.Lock()
	s.refreshes++
	s.mu.Unlock()
	return s.refreshErr
}

type activeReaderStoreStub struct {
	mu     sync.Mutex
	writes []RunRequest
}

func (s *activeReaderStoreStub) SetActiveScanningReader(_ context.Context, tenant store.Tenant, reader string) error {
	s.mu.Lock()
	s.writes = append(s.writes, RunRequest{Tenant: tenant, Reader: reader})
	s.mu.Unlock()
	return nil
}

func TestControllerSwitchesReadersAndPersistsSelection(t *testing.T) {
	started := make(chan ScanRequest, 2)
	scanner := &controllerScannerStub{run: func(ctx context.Context, request ScanRequest) error {
		started <- request
		<-ctx.Done()
		return ctx.Err()
	}}
	st := &activeReaderStoreStub{}
	controller := newTestController(t, scanner, st)
	tenant := store.Tenant{ID: "tenant-a"}

	controller.Start(RunRequest{Tenant: tenant, Reader: "gmail"})
	if request := <-started; request.Reader != "gmail" || request.Mode != ScanContinuous {
		t.Fatalf("first request = %#v", request)
	}
	controller.Start(RunRequest{Tenant: tenant, Reader: "thunderbird"})
	if request := <-started; request.Reader != "thunderbird" || request.Mode != ScanContinuous {
		t.Fatalf("second request = %#v", request)
	}
	if len(st.writes) != 2 || st.writes[1].Reader != "thunderbird" {
		t.Fatalf("active reader writes = %#v", st.writes)
	}
	closeController(t, controller)
}

func TestControllerRescanUsesRescanModeAndPersistsReader(t *testing.T) {
	started := make(chan ScanRequest, 1)
	scanner := &controllerScannerStub{run: func(ctx context.Context, request ScanRequest) error {
		started <- request
		<-ctx.Done()
		return ctx.Err()
	}}
	st := &activeReaderStoreStub{}
	controller := newTestController(t, scanner, st)
	controller.Rescan(RunRequest{Tenant: store.Tenant{ID: "tenant-a"}, Reader: "gmail"})
	if request := <-started; request.Mode != ScanRescan {
		t.Fatalf("scan mode = %v, want ScanRescan", request.Mode)
	}
	if len(st.writes) != 1 || st.writes[0].Reader != "gmail" {
		t.Fatalf("active reader writes = %#v", st.writes)
	}
	closeController(t, controller)
}

func TestControllerCanceledRunDoesNotSetLastError(t *testing.T) {
	started := make(chan struct{})
	scanner := &controllerScannerStub{run: func(ctx context.Context, _ ScanRequest) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}}
	controller := newTestController(t, scanner, &activeReaderStoreStub{})
	controller.Start(RunRequest{Tenant: store.Tenant{ID: "tenant-a"}, Reader: "gmail"})
	<-started
	controller.Stop()
	if status := controller.Status(); status.Running || status.LastError != "" {
		t.Fatalf("status = %#v", status)
	}
}

func TestControllerRecordsRunError(t *testing.T) {
	scanner := &controllerScannerStub{run: func(context.Context, ScanRequest) error { return errors.New("reader failed") }}
	controller := newTestController(t, scanner, &activeReaderStoreStub{})
	controller.Start(RunRequest{Tenant: store.Tenant{ID: "tenant-a"}, Reader: "gmail"})
	waitForStopped(t, controller)
	if status := controller.Status(); status.LastError != "reader failed" {
		t.Fatalf("last error = %q", status.LastError)
	}
}

func TestControllerCloseHonorsContextTimeout(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	scanner := &controllerScannerStub{run: func(context.Context, ScanRequest) error {
		close(started)
		<-release
		return nil
	}}
	controller := newTestController(t, scanner, &activeReaderStoreStub{})
	controller.Start(RunRequest{Tenant: store.Tenant{ID: "tenant-a"}, Reader: "gmail"})
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if err := controller.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close() error = %v, want deadline exceeded", err)
	}
	close(release)
	waitForStopped(t, controller)
}

func TestControllerRefreshesResolverAndRestartsActiveRun(t *testing.T) {
	started := make(chan ScanRequest, 2)
	scanner := &controllerScannerStub{run: func(ctx context.Context, request ScanRequest) error {
		started <- request
		<-ctx.Done()
		return ctx.Err()
	}}
	controller := newTestController(t, scanner, &activeReaderStoreStub{})
	request := RunRequest{Tenant: store.Tenant{ID: "tenant-a"}, Reader: "gmail"}
	controller.Start(request)
	<-started
	controller.RefreshResolver(context.Background())
	if restarted := <-started; restarted.Reader != "gmail" || restarted.Mode != ScanContinuous {
		t.Fatalf("restarted request = %#v", restarted)
	}
	scanner.mu.Lock()
	refreshes := scanner.refreshes
	scanner.mu.Unlock()
	if refreshes != 1 {
		t.Fatalf("resolver refreshes = %d, want 1", refreshes)
	}
	closeController(t, controller)
}

func newTestController(t *testing.T, scanner scanExecutor, st activeReaderStore) *Controller {
	t.Helper()
	controller, err := NewController(ControllerDependencies{
		Context: context.Background(), Scanner: scanner, Store: st,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("NewController() error = %v", err)
	}
	return controller
}

func closeController(t *testing.T, controller *Controller) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := controller.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func waitForStopped(t *testing.T, controller *Controller) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for controller.Status().Running && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if controller.Status().Running {
		t.Fatal("controller did not stop")
	}
}
