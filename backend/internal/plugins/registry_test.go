package plugins

import (
	"context"
	"testing"

	"github.com/ArionMiles/expensor/backend/pkg/api"
)

// mockReader implements api.Reader for testing.
type mockReader struct{}

func (m *mockReader) Read(ctx context.Context, out chan<- *api.TransactionDetails, ackChan <-chan string) error {
	return nil
}

// mockReaderPlugin implements ReaderPlugin for testing.
type mockReaderPlugin struct {
	name        string
	description string
	scopes      []string
	guide       []byte
	reader      api.Reader
	err         error
	input       ReaderInput
}

func (m *mockReaderPlugin) Metadata() ReaderMetadata {
	return ReaderMetadata{
		Name:        m.name,
		Description: m.description,
		Auth: AuthSpec{
			Type:                      AuthTypeOAuth,
			RequiredScopes:            m.scopes,
			RequiresCredentialsUpload: false,
		},
		ConfigSchema: nil,
		SetupGuide:   m.guide,
	}
}

func (m *mockReaderPlugin) NewReader(input ReaderInput) (api.Reader, error) {
	m.input = input
	if m.err != nil {
		return nil, m.err
	}
	return m.reader, nil
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	if registry == nil {
		t.Fatal("expected non-nil registry")
	}
	if registry.readers == nil {
		t.Error("expected readers map to be initialized")
	}
	if len(registry.readers) != 0 {
		t.Errorf("expected empty readers map, got %d entries", len(registry.readers))
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
				plugin, err := registry.GetReader(tt.plugin.Metadata().Name)
				if err != nil {
					t.Errorf("failed to get registered reader: %v", err)
				}
				if plugin.Metadata().Name != tt.plugin.Metadata().Name {
					t.Errorf("expected plugin name %q, got %q", tt.plugin.Metadata().Name, plugin.Metadata().Name)
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

func TestRegisterReader_RejectsNilPlugin(t *testing.T) {
	registry := NewRegistry()

	assertNotPanics(t, func() {
		err := registry.RegisterReader(nil)
		if err == nil {
			t.Fatal("expected error for nil reader plugin, got nil")
		}
	})
}

func TestRegisterReader_RejectsTypedNilPlugin(t *testing.T) {
	registry := NewRegistry()
	var plugin *mockReaderPlugin

	assertNotPanics(t, func() {
		err := registry.RegisterReader(plugin)
		if err == nil {
			t.Fatal("expected error for typed nil reader plugin, got nil")
		}
	})
}

func TestRegisterReader_RejectsBlankName(t *testing.T) {
	tests := []struct {
		name       string
		readerName string
	}{
		{name: "empty", readerName: ""},
		{name: "whitespace", readerName: " \t\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			err := registry.RegisterReader(&mockReaderPlugin{name: tt.readerName})
			if err == nil {
				t.Fatal("expected error for blank reader name, got nil")
			}
		})
	}
}

func TestRegisterReader_RejectsInvalidSetupGuideJSON(t *testing.T) {
	registry := NewRegistry()
	err := registry.RegisterReader(&mockReaderPlugin{name: "test-reader", guide: []byte("{invalid")})
	if err == nil {
		t.Fatal("expected error for invalid setup guide JSON, got nil")
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
			if !tt.wantErr && got != nil && got.Metadata().Name != tt.pluginName {
				t.Errorf("expected plugin name %q, got %q", tt.pluginName, got.Metadata().Name)
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
		names[r.Metadata().Name] = true
	}
	if !names["reader1"] {
		t.Error("reader1 not found in list")
	}
	if !names["reader2"] {
		t.Error("reader2 not found in list")
	}
}

func TestGetAllScopes(t *testing.T) {
	tests := []struct {
		name           string
		readerScopes   []string
		expectedScopes []string
		wantErr        bool
	}{
		{
			name:           "reader scopes",
			readerScopes:   []string{"scope1", "scope2"},
			expectedScopes: []string{"scope1", "scope2"},
			wantErr:        false,
		},
		{
			name:           "empty scopes",
			readerScopes:   []string{},
			expectedScopes: []string{},
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()

			reader := &mockReaderPlugin{name: "test-reader", scopes: tt.readerScopes}

			if err := registry.RegisterReader(reader); err != nil {
				t.Fatalf("failed to register reader: %v", err)
			}

			scopes, err := registry.GetAllScopes("test-reader")

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

	_, err := registry.GetAllScopes("unknown-reader")
	if err == nil {
		t.Fatal("expected error for unknown reader, got nil")
	}
}

func TestRegistryIsCatalogOnly(t *testing.T) {
	registry := NewRegistry()
	reader := &mockReaderPlugin{name: "test-reader", reader: &mockReader{}}

	if err := registry.RegisterReader(reader); err != nil {
		t.Fatalf("RegisterReader() error = %v", err)
	}

	gotReader, err := registry.GetReader("test-reader")
	if err != nil {
		t.Fatalf("GetReader() error = %v", err)
	}
	if gotReader.Metadata().Name != "test-reader" {
		t.Fatalf("reader name = %q, want test-reader", gotReader.Metadata().Name)
	}
}

func assertNotPanics(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	fn()
}
