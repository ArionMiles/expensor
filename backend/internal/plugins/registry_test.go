package plugins

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"testing"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/state"
)

// mockReader implements api.Reader for testing.
type mockReader struct{}

func (m *mockReader) Read(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
	return nil
}

// mockWriter implements api.Writer for testing.
type mockWriter struct{}

func (m *mockWriter) Write(ctx context.Context, in <-chan *api.TransactionDetails, ackChan chan<- string) error {
	return nil
}

// mockReaderPlugin implements ReaderPlugin for testing.
type mockReaderPlugin struct {
	name        string
	description string
	scopes      []string
	createError error
}

func (m *mockReaderPlugin) Name() string                    { return m.name }
func (m *mockReaderPlugin) Description() string             { return m.description }
func (m *mockReaderPlugin) RequiredScopes() []string        { return m.scopes }
func (m *mockReaderPlugin) AuthType() AuthType              { return AuthTypeOAuth }
func (m *mockReaderPlugin) RequiresCredentialsUpload() bool { return false }
func (m *mockReaderPlugin) ConfigSchema() []ConfigField     { return nil }
func (m *mockReaderPlugin) NewReader( //nolint:revive // interface method; argument count dictated by ReaderPlugin
	httpClient *http.Client, cfg *config.Config, rules []api.Rule,
	labels api.Labels, stateManager *state.Manager, logger *slog.Logger,
) (api.Reader, error) {
	if m.createError != nil {
		return nil, m.createError
	}
	return &mockReader{}, nil
}

// mockWriterPlugin implements WriterPlugin for testing.
type mockWriterPlugin struct {
	name        string
	description string
	scopes      []string
	createError error
}

func (m *mockWriterPlugin) Name() string             { return m.name }
func (m *mockWriterPlugin) Description() string      { return m.description }
func (m *mockWriterPlugin) RequiredScopes() []string { return m.scopes }
func (m *mockWriterPlugin) NewWriter(httpClient *http.Client, cfg *config.Config, logger *slog.Logger) (api.Writer, error) {
	if m.createError != nil {
		return nil, m.createError
	}
	return &mockWriter{}, nil
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	if registry == nil {
		t.Fatal("expected non-nil registry")
	}
	if registry.readers == nil {
		t.Error("expected readers map to be initialized")
	}
	if registry.writers == nil {
		t.Error("expected writers map to be initialized")
	}
	if len(registry.readers) != 0 {
		t.Errorf("expected empty readers map, got %d entries", len(registry.readers))
	}
	if len(registry.writers) != 0 {
		t.Errorf("expected empty writers map, got %d entries", len(registry.writers))
	}
}

func TestRegisterReader(t *testing.T) {
	tests := []struct {
		name    string
		plugin  ReaderPlugin
		wantErr bool
	}{
		{
			name: "successful registration",
			plugin: &mockReaderPlugin{
				name:        "test-reader",
				description: "Test Reader",
				scopes:      []string{"scope1"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			err := registry.RegisterReader(tt.plugin)

			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterReader() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				plugin, err := registry.GetReader(tt.plugin.Name())
				if err != nil {
					t.Errorf("failed to get registered reader: %v", err)
				}
				if plugin.Name() != tt.plugin.Name() {
					t.Errorf("expected plugin name %q, got %q", tt.plugin.Name(), plugin.Name())
				}
			}
		})
	}
}

func TestRegisterReader_Duplicate(t *testing.T) {
	registry := NewRegistry()
	plugin := &mockReaderPlugin{name: "test-reader"}

	// First registration should succeed
	err := registry.RegisterReader(plugin)
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Second registration should fail
	err = registry.RegisterReader(plugin)
	if err == nil {
		t.Fatal("expected error for duplicate registration, got nil")
	}
	expectedErr := `reader plugin "test-reader" already registered`
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestRegisterWriter(t *testing.T) {
	tests := []struct {
		name    string
		plugin  WriterPlugin
		wantErr bool
	}{
		{
			name: "successful registration",
			plugin: &mockWriterPlugin{
				name:        "test-writer",
				description: "Test Writer",
				scopes:      []string{"scope1"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			err := registry.RegisterWriter(tt.plugin)

			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterWriter() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				plugin, err := registry.GetWriter(tt.plugin.Name())
				if err != nil {
					t.Errorf("failed to get registered writer: %v", err)
				}
				if plugin.Name() != tt.plugin.Name() {
					t.Errorf("expected plugin name %q, got %q", tt.plugin.Name(), plugin.Name())
				}
			}
		})
	}
}

func TestRegisterWriter_Duplicate(t *testing.T) {
	registry := NewRegistry()
	plugin := &mockWriterPlugin{name: "test-writer"}

	// First registration should succeed
	err := registry.RegisterWriter(plugin)
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Second registration should fail
	err = registry.RegisterWriter(plugin)
	if err == nil {
		t.Fatal("expected error for duplicate registration, got nil")
	}
	expectedErr := `writer plugin "test-writer" already registered`
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestGetReader(t *testing.T) {
	registry := NewRegistry()
	plugin := &mockReaderPlugin{name: "test-reader", description: "Test"}

	// Register plugin
	if err := registry.RegisterReader(plugin); err != nil {
		t.Fatalf("failed to register reader: %v", err)
	}

	tests := []struct {
		name       string
		pluginName string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "get existing reader",
			pluginName: "test-reader",
			wantErr:    false,
		},
		{
			name:       "get non-existent reader",
			pluginName: "unknown",
			wantErr:    true,
			errMsg:     `reader plugin "unknown" not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := registry.GetReader(tt.pluginName)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetReader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("expected error %q, got %q", tt.errMsg, err.Error())
			}

			if !tt.wantErr && got == nil {
				t.Error("expected plugin, got nil")
			}
			if !tt.wantErr && got != nil && got.Name() != tt.pluginName {
				t.Errorf("expected plugin name %q, got %q", tt.pluginName, got.Name())
			}
		})
	}
}

func TestGetWriter(t *testing.T) {
	registry := NewRegistry()
	plugin := &mockWriterPlugin{name: "test-writer", description: "Test"}

	// Register plugin
	if err := registry.RegisterWriter(plugin); err != nil {
		t.Fatalf("failed to register writer: %v", err)
	}

	tests := []struct {
		name       string
		pluginName string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "get existing writer",
			pluginName: "test-writer",
			wantErr:    false,
		},
		{
			name:       "get non-existent writer",
			pluginName: "unknown",
			wantErr:    true,
			errMsg:     `writer plugin "unknown" not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := registry.GetWriter(tt.pluginName)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetWriter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("expected error %q, got %q", tt.errMsg, err.Error())
			}

			if !tt.wantErr && got == nil {
				t.Error("expected plugin, got nil")
			}
			if !tt.wantErr && got != nil && got.Name() != tt.pluginName {
				t.Errorf("expected plugin name %q, got %q", tt.pluginName, got.Name())
			}
		})
	}
}

func TestListReaders(t *testing.T) {
	registry := NewRegistry()

	// Initially empty
	readers := registry.ListReaders()
	if len(readers) != 0 {
		t.Errorf("expected 0 readers, got %d", len(readers))
	}

	// Add readers
	plugin1 := &mockReaderPlugin{name: "reader1"}
	plugin2 := &mockReaderPlugin{name: "reader2"}

	if err := registry.RegisterReader(plugin1); err != nil {
		t.Fatalf("failed to register reader1: %v", err)
	}
	if err := registry.RegisterReader(plugin2); err != nil {
		t.Fatalf("failed to register reader2: %v", err)
	}

	readers = registry.ListReaders()
	if len(readers) != 2 {
		t.Errorf("expected 2 readers, got %d", len(readers))
	}

	// Verify both are present (order not guaranteed)
	names := make(map[string]bool)
	for _, r := range readers {
		names[r.Name()] = true
	}
	if !names["reader1"] {
		t.Error("reader1 not found in list")
	}
	if !names["reader2"] {
		t.Error("reader2 not found in list")
	}
}

func TestListWriters(t *testing.T) {
	registry := NewRegistry()

	// Initially empty
	writers := registry.ListWriters()
	if len(writers) != 0 {
		t.Errorf("expected 0 writers, got %d", len(writers))
	}

	// Add writers
	plugin1 := &mockWriterPlugin{name: "writer1"}
	plugin2 := &mockWriterPlugin{name: "writer2"}

	if err := registry.RegisterWriter(plugin1); err != nil {
		t.Fatalf("failed to register writer1: %v", err)
	}
	if err := registry.RegisterWriter(plugin2); err != nil {
		t.Fatalf("failed to register writer2: %v", err)
	}

	writers = registry.ListWriters()
	if len(writers) != 2 {
		t.Errorf("expected 2 writers, got %d", len(writers))
	}

	// Verify both are present (order not guaranteed)
	names := make(map[string]bool)
	for _, w := range writers {
		names[w.Name()] = true
	}
	if !names["writer1"] {
		t.Error("writer1 not found in list")
	}
	if !names["writer2"] {
		t.Error("writer2 not found in list")
	}
}

func TestGetAllScopes(t *testing.T) {
	tests := []struct {
		name           string
		readerScopes   []string
		writerScopes   []string
		expectedScopes []string
		wantErr        bool
	}{
		{
			name:           "combine scopes",
			readerScopes:   []string{"scope1", "scope2"},
			writerScopes:   []string{"scope3", "scope4"},
			expectedScopes: []string{"scope1", "scope2", "scope3", "scope4"},
			wantErr:        false,
		},
		{
			name:           "deduplicate scopes",
			readerScopes:   []string{"scope1", "scope2"},
			writerScopes:   []string{"scope2", "scope3"},
			expectedScopes: []string{"scope1", "scope2", "scope3"},
			wantErr:        false,
		},
		{
			name:           "empty scopes",
			readerScopes:   []string{},
			writerScopes:   []string{},
			expectedScopes: []string{},
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()

			reader := &mockReaderPlugin{name: "test-reader", scopes: tt.readerScopes}
			writer := &mockWriterPlugin{name: "test-writer", scopes: tt.writerScopes}

			if err := registry.RegisterReader(reader); err != nil {
				t.Fatalf("failed to register reader: %v", err)
			}
			if err := registry.RegisterWriter(writer); err != nil {
				t.Fatalf("failed to register writer: %v", err)
			}

			scopes, err := registry.GetAllScopes("test-reader", "test-writer")

			if (err != nil) != tt.wantErr {
				t.Errorf("GetAllScopes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Convert to map for comparison (order doesn't matter)
				scopeMap := make(map[string]bool)
				for _, s := range scopes {
					scopeMap[s] = true
				}

				expectedMap := make(map[string]bool)
				for _, s := range tt.expectedScopes {
					expectedMap[s] = true
				}

				if len(scopeMap) != len(expectedMap) {
					t.Errorf("expected %d scopes, got %d", len(expectedMap), len(scopeMap))
				}

				for scope := range expectedMap {
					if !scopeMap[scope] {
						t.Errorf("expected scope %q not found", scope)
					}
				}
			}
		})
	}
}

func TestGetAllScopes_ReaderNotFound(t *testing.T) {
	registry := NewRegistry()
	writer := &mockWriterPlugin{name: "test-writer"}
	if err := registry.RegisterWriter(writer); err != nil {
		t.Fatalf("RegisterWriter: %v", err)
	}

	_, err := registry.GetAllScopes("unknown-reader", "test-writer")
	if err == nil {
		t.Fatal("expected error for unknown reader, got nil")
	}
}

func TestGetAllScopes_WriterNotFound(t *testing.T) {
	registry := NewRegistry()
	reader := &mockReaderPlugin{name: "test-reader"}
	if err := registry.RegisterReader(reader); err != nil {
		t.Fatalf("RegisterReader: %v", err)
	}

	_, err := registry.GetAllScopes("test-reader", "unknown-writer")
	if err == nil {
		t.Fatal("expected error for unknown writer, got nil")
	}
}

func TestCreateReader(t *testing.T) {
	tests := []struct {
		name        string
		createError error
		wantErr     bool
	}{
		{
			name:        "successful creation",
			createError: nil,
			wantErr:     false,
		},
		{
			name:        "creation error",
			createError: errors.New("failed to create"),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			plugin := &mockReaderPlugin{
				name:        "test-reader",
				createError: tt.createError,
			}
			if err := registry.RegisterReader(plugin); err != nil {
				t.Fatalf("RegisterReader: %v", err)
			}

			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
			cfg := &config.Config{}
			reader, err := registry.CreateReader("test-reader", &http.Client{}, cfg, []api.Rule{}, make(api.Labels), nil, logger)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateReader() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && reader == nil {
				t.Error("expected reader, got nil")
			}
		})
	}
}

func TestCreateReader_NotFound(t *testing.T) {
	registry := NewRegistry()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Config{}

	_, err := registry.CreateReader("unknown", &http.Client{}, cfg, []api.Rule{}, make(api.Labels), nil, logger)
	if err == nil {
		t.Fatal("expected error for unknown reader, got nil")
	}
}

func TestCreateWriter(t *testing.T) {
	tests := []struct {
		name        string
		createError error
		wantErr     bool
	}{
		{
			name:        "successful creation",
			createError: nil,
			wantErr:     false,
		},
		{
			name:        "creation error",
			createError: errors.New("failed to create"),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			plugin := &mockWriterPlugin{
				name:        "test-writer",
				createError: tt.createError,
			}
			if err := registry.RegisterWriter(plugin); err != nil {
				t.Fatalf("RegisterWriter: %v", err)
			}

			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
			cfg := &config.Config{}
			writer, err := registry.CreateWriter("test-writer", &http.Client{}, cfg, logger)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateWriter() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && writer == nil {
				t.Error("expected writer, got nil")
			}
		})
	}
}

func TestCreateWriter_NotFound(t *testing.T) {
	registry := NewRegistry()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Config{}

	_, err := registry.CreateWriter("unknown", &http.Client{}, cfg, logger)
	if err == nil {
		t.Fatal("expected error for unknown writer, got nil")
	}
}
