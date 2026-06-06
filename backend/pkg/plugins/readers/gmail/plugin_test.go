package gmail_test

import (
	"log/slog"
	"os"
	"testing"

	gmailapi "google.golang.org/api/gmail/v1"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	gmailplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/readers/gmail"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestPlugin_Metadata(t *testing.T) {
	p := &gmailplugin.Plugin{}
	p.SetGuideData([]byte(`{"sections":[]}`))

	metadata := p.Metadata()
	if metadata.Name != "gmail" {
		t.Errorf("Name: got %q, want \"gmail\"", metadata.Name)
	}
	if metadata.Description == "" {
		t.Error("Description: got empty string, want non-empty")
	}
	if metadata.Auth.Type != plugins.AuthTypeOAuth {
		t.Errorf("Auth.Type: got %q, want %q", metadata.Auth.Type, plugins.AuthTypeOAuth)
	}
	if !metadata.Auth.RequiresCredentialsUpload {
		t.Error("RequiresCredentialsUpload: got false, want true")
	}

	want := []string{
		gmailapi.GmailReadonlyScope,
	}

	scopes := metadata.Auth.RequiredScopes
	if len(scopes) != len(want) {
		t.Fatalf("RequiredScopes: got %d scopes, want %d — got %v", len(scopes), len(want), scopes)
	}
	for i, s := range scopes {
		if s != want[i] {
			t.Errorf("scope[%d]: got %q, want %q", i, s, want[i])
		}
	}
	if len(metadata.ConfigSchema) != 0 {
		t.Errorf("ConfigSchema: got %v, want empty", metadata.ConfigSchema)
	}
	if len(metadata.SetupGuide) == 0 {
		t.Error("SetupGuide: got empty, want guide data")
	}
}

func TestPlugin_NewReader_NilHTTPClient(t *testing.T) {
	p := &gmailplugin.Plugin{}
	cfg := &config.App{
		ScanInterval: 60,
	}

	_, err := p.NewReader(plugins.ReaderInput{
		AppConfig: cfg,
		Logger:    testLogger(),
	})
	if err == nil {
		t.Error("expected error with nil http client, got nil")
	}
}
