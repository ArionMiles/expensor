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
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/observability"
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

// mockWriter implements api.Writer for testing.
type mockWriter struct {
	writeFunc func(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error
	closeFunc func() error
}

func (m *mockWriter) Write(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
	if m.writeFunc != nil {
		return m.writeFunc(ctx, in, ackChan)
	}
	// Default: drain channel
	for range in {
	}
	return nil
}

func (m *mockWriter) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// mockReaderPlugin implements plugins.ReaderPlugin for testing.
type mockReaderPlugin struct {
	name        string
	reader      api.Reader
	createError error
	input       plugins.ReaderInput
}

func (m *mockReaderPlugin) Metadata() plugins.ReaderMetadata {
	return plugins.ReaderMetadata{
		Name:        m.name,
		Description: "mock reader",
		Auth: plugins.AuthSpec{
			Type:           plugins.AuthTypeOAuth,
			RequiredScopes: []string{"scope1"},
		},
	}
}

func (m *mockReaderPlugin) NewReader(input plugins.ReaderInput) (api.Reader, error) {
	m.input = input
	if m.createError != nil {
		return nil, m.createError
	}
	return m.reader, nil
}

// mockWriterPlugin implements plugins.WriterPlugin for testing.
type mockWriterPlugin struct {
	name        string
	writer      api.Writer
	createError error
	input       plugins.WriterInput
}

func (m *mockWriterPlugin) Metadata() plugins.WriterMetadata {
	return plugins.WriterMetadata{
		Name:           m.name,
		Description:    "mock writer",
		RequiredScopes: []string{"scope2"},
	}
}

func (m *mockWriterPlugin) NewWriter(input plugins.WriterInput) (api.Writer, error) {
	m.input = input
	if m.createError != nil {
		return nil, m.createError
	}
	return m.writer, nil
}

type mockRuntimeStore struct {
	readerConfig json.RawMessage
	hasConfig    bool
	err          error
}

func (m *mockRuntimeStore) GetReaderConfig(ctx context.Context, reader string) (json.RawMessage, bool, error) {
	return m.readerConfig, m.hasConfig, m.err
}

func TestNew(t *testing.T) {
	tests := []struct {
		name       string
		registry   *plugins.Registry
		httpClient *http.Client
		logger     *slog.Logger
	}{
		{
			name:       "with all parameters",
			registry:   plugins.NewRegistry(),
			httpClient: &http.Client{},
			logger:     slog.Default(),
		},
		{
			name:       "with nil logger",
			registry:   plugins.NewRegistry(),
			httpClient: &http.Client{},
			logger:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := New(tt.registry, tt.httpClient, tt.logger)

			if runner == nil {
				t.Fatal("expected non-nil runner")
			}
			if runner.registry == nil {
				t.Error("expected registry to be set")
			}
			if runner.logger == nil {
				t.Error("expected logger to be set (default if nil)")
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
	if err := registry.RegisterReader(&mockReaderPlugin{name: "test-reader", reader: &mockReader{}}); err != nil {
		t.Fatalf("RegisterReader() error = %v", err)
	}
	if err := registry.RegisterWriter(&mockWriterPlugin{name: "test-writer", writer: &mockWriter{}}); err != nil {
		t.Fatalf("RegisterWriter() error = %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	scope := observability.NewScope(logger, "test/daemon")
	runner := NewWithScope(registry, &http.Client{}, logger, scope)

	err := runner.Run(t.Context(), RunConfig{
		ReaderName: "test-reader",
		WriterName: "test-writer",
		Config:     &config.Config{},
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

	// Use channels to signal when reader/writer have started
	readerStartedCh := make(chan struct{})
	writerStartedCh := make(chan struct{})

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

	writer := &mockWriter{
		writeFunc: func(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
			close(writerStartedCh)

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
	if err := registry.RegisterReader(&mockReaderPlugin{name: "test-reader", reader: reader}); err != nil {
		t.Fatalf("RegisterReader: %v", err)
	}
	if err := registry.RegisterWriter(&mockWriterPlugin{name: "test-writer", writer: writer}); err != nil {
		t.Fatalf("RegisterWriter: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, &http.Client{}, logger)

	cfg := &config.Config{}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		WriterName:   "test-writer",
		Config:       cfg,
		Rules:        []api.Rule{},
		Resolver:     nil,
		StateManager: nil,
	}

	// Wait for reader and writer to start before Run completes
	go func() {
		select {
		case <-readerStartedCh:
		case <-time.After(100 * time.Millisecond):
			t.Error("reader did not start in time")
		}
		select {
		case <-writerStartedCh:
		case <-time.After(100 * time.Millisecond):
			t.Error("writer did not start in time")
		}
	}()

	err := runner.Run(ctx, runCfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_ReaderError(t *testing.T) {
	ctx := context.Background()

	reader := &mockReader{
		readFunc: func(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
			close(out)
			return errors.New("reader failed")
		},
	}

	writer := &mockWriter{
		writeFunc: func(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
			for range in {
			}
			return nil
		},
	}

	registry := plugins.NewRegistry()
	if err := registry.RegisterReader(&mockReaderPlugin{name: "test-reader", reader: reader}); err != nil {
		t.Fatalf("RegisterReader: %v", err)
	}
	if err := registry.RegisterWriter(&mockWriterPlugin{name: "test-writer", writer: writer}); err != nil {
		t.Fatalf("RegisterWriter: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, &http.Client{}, logger)

	cfg := &config.Config{}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		WriterName:   "test-writer",
		Config:       cfg,
		Rules:        []api.Rule{},
		Resolver:     nil,
		StateManager: nil,
	}

	// Run should complete without returning error (errors are logged)
	err := runner.Run(ctx, runCfg)
	if err != nil {
		t.Errorf("Run should not return error, got: %v", err)
	}
}

func TestRun_WriterError(t *testing.T) {
	ctx := context.Background()

	reader := &mockReader{
		readFunc: func(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
			close(out)
			return nil
		},
	}

	writer := &mockWriter{
		writeFunc: func(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
			for range in {
			}
			return errors.New("writer failed")
		},
	}

	registry := plugins.NewRegistry()
	if err := registry.RegisterReader(&mockReaderPlugin{name: "test-reader", reader: reader}); err != nil {
		t.Fatalf("RegisterReader: %v", err)
	}
	if err := registry.RegisterWriter(&mockWriterPlugin{name: "test-writer", writer: writer}); err != nil {
		t.Fatalf("RegisterWriter: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, &http.Client{}, logger)

	cfg := &config.Config{}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		WriterName:   "test-writer",
		Config:       cfg,
		Rules:        []api.Rule{},
		Resolver:     nil,
		StateManager: nil,
	}

	// Run should complete without returning error (errors are logged)
	err := runner.Run(ctx, runCfg)
	if err != nil {
		t.Errorf("Run should not return error, got: %v", err)
	}
}

func TestRun_PassesPersistedReaderConfigToPlugin(t *testing.T) {
	reader := &mockReader{
		readFunc: func(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
			close(out)
			return nil
		},
	}
	writer := &mockWriter{}
	readerPlugin := &mockReaderPlugin{name: "test-reader", reader: reader}
	writerPlugin := &mockWriterPlugin{name: "test-writer", writer: writer}
	registry := plugins.NewRegistry()
	if err := registry.RegisterReader(readerPlugin); err != nil {
		t.Fatalf("RegisterReader() error = %v", err)
	}
	if err := registry.RegisterWriter(writerPlugin); err != nil {
		t.Fatalf("RegisterWriter() error = %v", err)
	}

	runtimeStore := &mockRuntimeStore{
		readerConfig: json.RawMessage(`{"config":{"profilePath":"/tmp/profile","mailboxes":"Inbox"}}`),
		hasConfig:    true,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, &http.Client{}, logger)

	err := runner.Run(context.Background(), RunConfig{
		ReaderName:   "test-reader",
		WriterName:   "test-writer",
		Config:       &config.Config{},
		RuntimeStore: runtimeStore,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if string(readerPlugin.input.ReaderConfig) != string(runtimeStore.readerConfig) {
		t.Fatalf("ReaderConfig = %s, want %s", readerPlugin.input.ReaderConfig, runtimeStore.readerConfig)
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var readerStopped, writerStopped bool
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

	writer := &mockWriter{
		writeFunc: func(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
			for range in {
			}
			mu.Lock()
			writerStopped = true
			mu.Unlock()
			return context.Canceled
		},
	}

	registry := plugins.NewRegistry()
	if err := registry.RegisterReader(&mockReaderPlugin{name: "test-reader", reader: reader}); err != nil {
		t.Fatalf("RegisterReader: %v", err)
	}
	if err := registry.RegisterWriter(&mockWriterPlugin{name: "test-writer", writer: writer}); err != nil {
		t.Fatalf("RegisterWriter: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, &http.Client{}, logger)

	cfg := &config.Config{}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		WriterName:   "test-writer",
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
	if !writerStopped {
		t.Error("writer did not stop after reader completion")
	}
}

func TestRun_NewReaderError(t *testing.T) {
	registry := plugins.NewRegistry()
	if err := registry.RegisterReader(&mockReaderPlugin{
		name:        "test-reader",
		createError: errors.New("failed to create reader"),
	}); err != nil {
		t.Fatalf("RegisterReader: %v", err)
	}
	if err := registry.RegisterWriter(&mockWriterPlugin{name: "test-writer", writer: &mockWriter{}}); err != nil {
		t.Fatalf("RegisterWriter: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, &http.Client{}, logger)

	cfg := &config.Config{}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		WriterName:   "test-writer",
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

func TestRun_NewWriterError(t *testing.T) {
	reader := &mockReader{
		readFunc: func(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
			close(out)
			return nil
		},
	}

	registry := plugins.NewRegistry()
	if err := registry.RegisterReader(&mockReaderPlugin{name: "test-reader", reader: reader}); err != nil {
		t.Fatalf("RegisterReader: %v", err)
	}
	if err := registry.RegisterWriter(&mockWriterPlugin{
		name:        "test-writer",
		createError: errors.New("failed to create writer"),
	}); err != nil {
		t.Fatalf("RegisterWriter: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, &http.Client{}, logger)

	cfg := &config.Config{}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		WriterName:   "test-writer",
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
	if err.Error() != "creating writer: failed to create writer" {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRunnerWriterErrorDeadlock reproduces the deadlock: the writer exits with an error while the reader
// is still producing more transactions than the channel can buffer. Without errgroup the runner hangs
// forever because the external context is never canceled, so the reader never unblocks.
func TestRunnerWriterErrorDeadlock(t *testing.T) {
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

	// Writer returns an error after 5 transactions, leaving the reader blocked on a full channel.
	writer := &mockWriter{
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
			return errors.New("writer failed after 5 transactions")
		},
	}

	registry := plugins.NewRegistry()
	if err := registry.RegisterReader(&mockReaderPlugin{name: "test-reader", reader: reader}); err != nil {
		t.Fatalf("RegisterReader: %v", err)
	}
	if err := registry.RegisterWriter(&mockWriterPlugin{name: "test-writer", writer: writer}); err != nil {
		t.Fatalf("RegisterWriter: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, &http.Client{}, logger)

	runCfg := RunConfig{
		ReaderName: "test-reader",
		WriterName: "test-writer",
		Config:     &config.Config{},
		Rules:      []api.Rule{},
	}

	// Safety net: cancel context after 3s to prevent the goroutine hanging forever.
	time.AfterFunc(3*time.Second, cancel)

	done := make(chan struct{})
	go func() {
		runner.Run(ctx, runCfg) //nolint:errcheck
		close(done)
	}()

	// With errgroup the runner cancels the reader's context as soon as the writer
	// fails, so Run returns well within 1 second. Without the fix, the runner waits
	// for the reader which is blocked on a full channel until the 3s safety cancel
	// fires — missing this deadline.
	select {
	case <-done:
		// Run returned quickly — no deadlock.
	case <-time.After(1 * time.Second):
		t.Fatal("Run did not return: deadlock suspected")
	}
}

// TestRun_WriterResourceCleanup tests that the writer's Close method is called if it implements io.Closer.
// Note: This tests implementation details - the real CSV/JSON writers implement Close() properly.
func TestRun_WriterResourceCleanup(t *testing.T) {
	// This test verifies that if a writer implements Close(), it gets called.
	// The actual behavior is tested in integration tests with real writers.
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

	writer := &mockWriter{
		writeFunc: func(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
			for range in {
			}
			return nil
		},
	}

	registry := plugins.NewRegistry()
	if err := registry.RegisterReader(&mockReaderPlugin{name: "test-reader", reader: reader}); err != nil {
		t.Fatalf("RegisterReader: %v", err)
	}
	if err := registry.RegisterWriter(&mockWriterPlugin{name: "test-writer", writer: writer}); err != nil {
		t.Fatalf("RegisterWriter: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	runner := New(registry, &http.Client{}, logger)

	cfg := &config.Config{}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		WriterName:   "test-writer",
		Config:       cfg,
		Rules:        []api.Rule{},
		Resolver:     nil,
		StateManager: nil,
	}

	// Should not panic even if Close() exists on writer
	err := runner.Run(ctx, runCfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
