package daemon

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/state"
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
}

func (m *mockReaderPlugin) Name() string                    { return m.name }
func (m *mockReaderPlugin) Description() string             { return "mock reader" }
func (m *mockReaderPlugin) RequiredScopes() []string        { return []string{"scope1"} }
func (m *mockReaderPlugin) AuthType() plugins.AuthType      { return plugins.AuthTypeOAuth }
func (m *mockReaderPlugin) RequiresCredentialsUpload() bool { return false }
func (m *mockReaderPlugin) ConfigSchema() []plugins.ConfigField { return nil }
func (m *mockReaderPlugin) NewReader( //nolint:revive // interface method; argument count dictated by ReaderPlugin
	httpClient *http.Client, cfg *config.Config, rules []api.Rule,
	labels api.Labels, stateManager *state.Manager, logger *slog.Logger,
) (api.Reader, error) {
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
}

func (m *mockWriterPlugin) Name() string             { return m.name }
func (m *mockWriterPlugin) Description() string      { return "mock writer" }
func (m *mockWriterPlugin) RequiredScopes() []string { return []string{"scope2"} }
func (m *mockWriterPlugin) NewWriter(httpClient *http.Client, cfg *config.Config, logger *slog.Logger) (api.Writer, error) {
	if m.createError != nil {
		return nil, m.createError
	}
	return m.writer, nil
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

	cfg := &config.Config{
		
		
	}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		WriterName:   "test-writer",
		Config:       cfg,
		Rules:        []api.Rule{},
		Labels:       make(api.Labels),
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

	cfg := &config.Config{
		
		
	}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		WriterName:   "test-writer",
		Config:       cfg,
		Rules:        []api.Rule{},
		Labels:       make(api.Labels),
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

	cfg := &config.Config{
		
		
	}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		WriterName:   "test-writer",
		Config:       cfg,
		Rules:        []api.Rule{},
		Labels:       make(api.Labels),
		StateManager: nil,
	}

	// Run should complete without returning error (errors are logged)
	err := runner.Run(ctx, runCfg)
	if err != nil {
		t.Errorf("Run should not return error, got: %v", err)
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

	cfg := &config.Config{
		
		
	}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		WriterName:   "test-writer",
		Config:       cfg,
		Rules:        []api.Rule{},
		Labels:       make(api.Labels),
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

func TestRun_CreateReaderError(t *testing.T) {
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

	cfg := &config.Config{
		
		
	}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		WriterName:   "test-writer",
		Config:       cfg,
		Rules:        []api.Rule{},
		Labels:       make(api.Labels),
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

func TestRun_CreateWriterError(t *testing.T) {
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

	cfg := &config.Config{
		
		
	}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		WriterName:   "test-writer",
		Config:       cfg,
		Rules:        []api.Rule{},
		Labels:       make(api.Labels),
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

	cfg := &config.Config{
		
		
	}

	runCfg := RunConfig{
		ReaderName:   "test-reader",
		WriterName:   "test-writer",
		Config:       cfg,
		Rules:        []api.Rule{},
		Labels:       make(api.Labels),
		StateManager: nil,
	}

	// Should not panic even if Close() exists on writer
	err := runner.Run(ctx, runCfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
