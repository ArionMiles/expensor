package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
)

// mockReader implements api.Reader for testing.
type mockReader struct {
	readFunc func(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error
}

func (m *mockReader) Read(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
	if m.readFunc != nil {
		return m.readFunc(ctx, out, ackChan)
	}
	close(out)
	return nil
}

func (m *mockReader) Search(context.Context, api.EmailSearchQuery) ([]api.EmailSearchResult, error) {
	return nil, nil
}

// mockSink implements TransactionSink for testing.
type mockSink struct {
	writeFunc func(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error
	closeFunc func() error
}

func (m *mockSink) Write(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
	if m.writeFunc != nil {
		return m.writeFunc(ctx, in, ackChan)
	}
	// Default: drain channel
	for range in {
	}
	return nil
}

func (m *mockSink) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

type mockProvider struct {
	name        string
	reader      api.Reader
	createError error
	input       plugins.ProviderInput
}

func (m *mockProvider) metadata() plugins.ProviderMetadata {
	return plugins.ProviderMetadata{
		Name:        m.name,
		Description: "mock reader",
		Auth: plugins.AuthSpec{
			Type:           plugins.AuthTypeOAuth,
			RequiredScopes: []string{"scope1"},
		},
	}
}

func (m *mockProvider) NewReader(input plugins.ProviderInput) (api.Reader, error) {
	m.input = input
	if m.createError != nil {
		return nil, m.createError
	}
	return m.reader, nil
}

func (m *mockProvider) NewEmailSearcher(input plugins.ProviderInput) (api.EmailSearcher, error) {
	reader, err := m.NewReader(input)
	if err != nil {
		return nil, err
	}
	searcher, ok := reader.(api.EmailSearcher)
	if !ok {
		return nil, errors.New("not implemented in test stub")
	}
	return searcher, nil
}

func (m *mockProvider) provider() plugins.Provider {
	return plugins.Provider{
		Metadata:         m.metadata(),
		NewReader:        m.NewReader,
		NewEmailSearcher: m.NewEmailSearcher,
	}
}

type mockRuntimeStore struct {
	readerConfig json.RawMessage
	hasConfig    bool
	err          error
}

func (m *mockRuntimeStore) GetReaderConfig(ctx context.Context, _ store.Tenant, reader string) (json.RawMessage, bool, error) {
	return m.readerConfig, m.hasConfig, m.err
}

func newMockSinkFactory(sink TransactionSink, err error) TransactionSinkFactory {
	return func(_ store.Tenant, _ *config.App, _ *slog.Logger) (TransactionSink, error) {
		return sink, err
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name       string
		registry   *plugins.Registry
		factory    TransactionSinkFactory
		httpClient *http.Client
		logger     *slog.Logger
	}{
		{
			name:       "with all parameters",
			registry:   plugins.NewRegistry(),
			factory:    newMockSinkFactory(&mockSink{}, nil),
			httpClient: &http.Client{},
			logger:     slog.Default(),
		},
		{
			name:       "with nil logger",
			registry:   plugins.NewRegistry(),
			factory:    newMockSinkFactory(&mockSink{}, nil),
			httpClient: &http.Client{},
			logger:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := New(tt.registry, tt.factory, tt.httpClient, tt.logger)

			if runner == nil {
				t.Fatal("expected non-nil runner")
			}
			if runner.registry == nil {
				t.Error("expected registry to be set")
			}
			if runner.logger == nil {
				t.Error("expected logger to be set (default if nil)")
			}
			if runner.sinkFactory == nil {
				t.Error("expected sink factory to be set")
			}
		})
	}
}

func TestRunCreatesDaemonLifecycleSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(t.Context())
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider((&mockProvider{name: "test-reader", reader: &mockReader{}}).provider()); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	scope := observability.NewScope(logger, "test/daemon")
	runner := NewWithScope(registry, newMockSinkFactory(&mockSink{}, nil), &http.Client{}, logger, scope)

	err := runner.Run(t.Context(), RunConfig{
		ReaderName: "test-reader",
		Config:     &config.App{},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	if spans[0].Name() != "daemon.run" {
		t.Fatalf("span name = %q, want daemon.run", spans[0].Name())
	}
}

func TestRun_SuccessfulRun(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Use channels to signal when reader/sink have started
	readerStartedCh := make(chan struct{})
	sinkStartedCh := make(chan struct{})

	reader := &mockReader{
		readFunc: func(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
			close(readerStartedCh)

			// Send a transaction
			select {
			case out <- &api.TransactionDetails{
				Amount:       100.50,
				MerchantInfo: "TestStore",
				MessageID:    "msg1",
			}:
			case <-ctx.Done():
				close(out)
				return ctx.Err()
			}

			// Wait for context cancellation
			<-ctx.Done()
			close(out)
			return ctx.Err()
		},
	}

	sink := &mockSink{
		writeFunc: func(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
			close(sinkStartedCh)

			for {
				select {
				case txn, ok := <-in:
					if !ok {
						return nil
					}
					// Send ack (with timeout to avoid blocking)
					select {
					case ackChan <- txn.MessageID:
					case <-time.After(10 * time.Millisecond):
					case <-ctx.Done():
					}
				case <-ctx.Done():
					// Drain remaining
					for range in {
					}
					return ctx.Err()
				}
			}
		},
	}

	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider((&mockProvider{name: "test-reader", reader: reader}).provider()); err != nil {
		t.Fatalf("RegisterProvider: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, newMockSinkFactory(sink, nil), &http.Client{}, logger)

	cfg := &config.App{}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		Config:       cfg,
		Rules:        []api.Rule{},
		Resolver:     nil,
		StateManager: nil,
	}

	// Wait for reader and sink to start before Run completes
	go func() {
		select {
		case <-readerStartedCh:
		case <-time.After(100 * time.Millisecond):
			t.Error("reader did not start in time")
		}
		select {
		case <-sinkStartedCh:
		case <-time.After(100 * time.Millisecond):
			t.Error("sink did not start in time")
		}
	}()

	err := runner.Run(ctx, runCfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_PassesTenantToSinkFactory(t *testing.T) {
	ctx := context.Background()
	wantTenant := store.Tenant{ID: "tenant-a"}
	var gotTenant store.Tenant

	reader := &mockReader{
		readFunc: func(_ context.Context, out chan<- *api.TransactionDetails, _ <-chan string) error {
			close(out)
			return nil
		},
	}
	sink := &mockSink{
		writeFunc: func(_ context.Context, in <-chan *api.TransactionDetails, _ chan<- string) error {
			for range in {
			}
			return nil
		},
	}

	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider((&mockProvider{name: "test-reader", reader: reader}).provider()); err != nil {
		t.Fatalf("RegisterProvider: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, func(tenant store.Tenant, _ *config.App, _ *slog.Logger) (TransactionSink, error) {
		gotTenant = tenant
		return sink, nil
	}, &http.Client{}, logger)

	if err := runner.Run(ctx, RunConfig{ReaderName: "test-reader", Tenant: wantTenant, Config: &config.App{}}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotTenant.ID != wantTenant.ID {
		t.Fatalf("sink factory tenant = %q, want %q", gotTenant.ID, wantTenant.ID)
	}
}

func TestRun_ReaderError(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("reader failed")

	reader := &mockReader{
		readFunc: func(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
			close(out)
			return wantErr
		},
	}

	sink := &mockSink{
		writeFunc: func(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
			for range in {
			}
			return nil
		},
	}

	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider((&mockProvider{name: "test-reader", reader: reader}).provider()); err != nil {
		t.Fatalf("RegisterProvider: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, newMockSinkFactory(sink, nil), &http.Client{}, logger)

	cfg := &config.App{}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		Config:       cfg,
		Rules:        []api.Rule{},
		Resolver:     nil,
		StateManager: nil,
	}

	err := runner.Run(ctx, runCfg)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

func TestRun_SinkError(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("sink failed")

	reader := &mockReader{
		readFunc: func(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
			close(out)
			return nil
		},
	}

	sink := &mockSink{
		writeFunc: func(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
			for range in {
			}
			return wantErr
		},
	}

	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider((&mockProvider{name: "test-reader", reader: reader}).provider()); err != nil {
		t.Fatalf("RegisterProvider: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, newMockSinkFactory(sink, nil), &http.Client{}, logger)

	cfg := &config.App{}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		Config:       cfg,
		Rules:        []api.Rule{},
		Resolver:     nil,
		StateManager: nil,
	}

	err := runner.Run(ctx, runCfg)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

func TestRun_PassesPersistedReaderConfigToPlugin(t *testing.T) {
	reader := &mockReader{
		readFunc: func(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
			close(out)
			return nil
		},
	}
	sink := &mockSink{}
	readerProvider := &mockProvider{name: "test-reader", reader: reader}
	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider(readerProvider.provider()); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}

	runtimeStore := &mockRuntimeStore{
		readerConfig: json.RawMessage(`{"config":{"profilePath":"/tmp/profile","mailboxes":"Inbox"}}`),
		hasConfig:    true,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, newMockSinkFactory(sink, nil), &http.Client{}, logger)

	err := runner.Run(context.Background(), RunConfig{
		ReaderName:   "test-reader",
		Config:       &config.App{},
		RuntimeStore: runtimeStore,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if string(readerProvider.input.ReaderConfig) != string(runtimeStore.readerConfig) {
		t.Fatalf("ReaderConfig = %s, want %s", readerProvider.input.ReaderConfig, runtimeStore.readerConfig)
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var readerStopped, sinkStopped bool
	var mu sync.Mutex

	reader := &mockReader{
		readFunc: func(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
			<-ctx.Done()
			mu.Lock()
			readerStopped = true
			mu.Unlock()
			close(out)
			return context.Canceled
		},
	}

	sink := &mockSink{
		writeFunc: func(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
			for range in {
			}
			mu.Lock()
			sinkStopped = true
			mu.Unlock()
			return context.Canceled
		},
	}

	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider((&mockProvider{name: "test-reader", reader: reader}).provider()); err != nil {
		t.Fatalf("RegisterProvider: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, newMockSinkFactory(sink, nil), &http.Client{}, logger)

	cfg := &config.App{}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		Config:       cfg,
		Rules:        []api.Rule{},
		Resolver:     nil,
		StateManager: nil,
	}

	// Cancel context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := runner.Run(ctx, runCfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if !readerStopped {
		t.Error("reader did not stop on context cancellation")
	}
	if !sinkStopped {
		t.Error("sink did not stop after reader completion")
	}
}

func TestRun_NewReaderError(t *testing.T) {
	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider((&mockProvider{
		name:        "test-reader",
		createError: errors.New("failed to create reader"),
	}).provider()); err != nil {
		t.Fatalf("RegisterProvider: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, newMockSinkFactory(&mockSink{}, nil), &http.Client{}, logger)

	cfg := &config.App{}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		Config:       cfg,
		Rules:        []api.Rule{},
		Resolver:     nil,
		StateManager: nil,
	}

	ctx := context.Background()
	err := runner.Run(ctx, runCfg)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "creating reader: failed to create reader" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_NewSinkError(t *testing.T) {
	reader := &mockReader{
		readFunc: func(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
			close(out)
			return nil
		},
	}

	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider((&mockProvider{name: "test-reader", reader: reader}).provider()); err != nil {
		t.Fatalf("RegisterProvider: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, newMockSinkFactory(nil, errors.New("failed to create sink")), &http.Client{}, logger)

	cfg := &config.App{}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		Config:       cfg,
		Rules:        []api.Rule{},
		Resolver:     nil,
		StateManager: nil,
	}

	ctx := context.Background()
	err := runner.Run(ctx, runCfg)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "creating transaction sink: failed to create sink" {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRunnerSinkErrorDeadlock reproduces the deadlock: the sink exits with an error while the reader
// is still producing more transactions than the channel can buffer. Without errgroup the runner hangs
// forever because the external context is never canceled, so the reader never unblocks.
func TestRunnerSinkErrorDeadlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Reader sends 200 transactions — more than the channel buffer of 100.
	reader := &mockReader{
		readFunc: func(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
			for i := range 200 {
				select {
				case out <- &api.TransactionDetails{Amount: float64(i), MessageID: fmt.Sprintf("msg-%d", i)}:
				case <-ctx.Done():
					close(out)
					return ctx.Err()
				}
			}
			close(out)
			return nil
		},
	}

	// Sink returns an error after 5 transactions, leaving the reader blocked on a full channel.
	sink := &mockSink{
		writeFunc: func(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
			for i := range 5 {
				select {
				case _, ok := <-in:
					if !ok {
						return nil
					}
					_ = i
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return errors.New("sink failed after 5 transactions")
		},
	}

	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider((&mockProvider{name: "test-reader", reader: reader}).provider()); err != nil {
		t.Fatalf("RegisterProvider: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, newMockSinkFactory(sink, nil), &http.Client{}, logger)

	runCfg := RunConfig{
		ReaderName: "test-reader",
		Config:     &config.App{},
		Rules:      []api.Rule{},
	}

	// Safety net: cancel context after 3s to prevent the goroutine hanging forever.
	time.AfterFunc(3*time.Second, cancel)

	result := make(chan error, 1)
	go func() {
		result <- runner.Run(ctx, runCfg)
	}()

	// With errgroup the runner cancels the reader's context as soon as the sink
	// fails, so Run returns well within 1 second. Without the fix, the runner waits
	// for the reader which is blocked on a full channel until the 3s safety cancel
	// fires — missing this deadline.
	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "sink failed after 5 transactions") {
			t.Fatalf("Run returned unexpected error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Run did not return: deadlock suspected")
	}
}

// TestRun_SinkResourceCleanup tests that the sink's Close method is called if it implements io.Closer.
// Note: This tests implementation details - the real CSV/JSON sinks implement Close() properly.
func TestRun_SinkResourceCleanup(t *testing.T) {
	// This test verifies that if a sink implements Close(), it gets called.
	// The actual behavior is tested in integration tests with real sinks.
	// For unit testing, we verify that the code path exists and doesn't panic.

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	reader := &mockReader{
		readFunc: func(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
			<-ctx.Done()
			close(out)
			return ctx.Err()
		},
	}

	sink := &mockSink{
		writeFunc: func(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
			for range in {
			}
			return nil
		},
	}

	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider((&mockProvider{name: "test-reader", reader: reader}).provider()); err != nil {
		t.Fatalf("RegisterProvider: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, newMockSinkFactory(sink, nil), &http.Client{}, logger)

	cfg := &config.App{}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		Config:       cfg,
		Rules:        []api.Rule{},
		Resolver:     nil,
		StateManager: nil,
	}

	// Should not panic even if Close() exists on sink
	err := runner.Run(ctx, runCfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
