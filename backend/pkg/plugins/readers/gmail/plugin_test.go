package gmail_test

import (
	"log/slog"
	"os"
	"testing"

	gmailapi "google.golang.org/api/gmail/v1"

	"github.com/ArionMiles/expensor/backend/pkg/config"
	gmailplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/readers/gmail"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestPlugin_Name(t *testing.T) {
	p := &gmailplugin.Plugin{}
	if got := p.Name(); got != "gmail" {
		t.Errorf("Name: got %q, want \"gmail\"", got)
	}
}

func TestPlugin_Description(t *testing.T) {
	p := &gmailplugin.Plugin{}
	if got := p.Description(); got == "" {
		t.Error("Description: got empty string, want non-empty")
	}
}

func TestPlugin_RequiredScopes(t *testing.T) {
	p := &gmailplugin.Plugin{}
	scopes := p.RequiredScopes()

	want := []string{
		gmailapi.GmailReadonlyScope,
		gmailapi.GmailModifyScope,
	}

	if len(scopes) != len(want) {
		t.Fatalf("RequiredScopes: got %d scopes, want %d — got %v", len(scopes), len(want), scopes)
	}
	for i, s := range scopes {
		if s != want[i] {
			t.Errorf("scope[%d]: got %q, want %q", i, s, want[i])
		}
	}
}

func TestPlugin_NewReader_NilHTTPClient(t *testing.T) {
	p := &gmailplugin.Plugin{}
	cfg := &config.Config{
		Gmail: config.GmailConfig{Interval: 60},
	}

	_, err := p.NewReader(nil, cfg, nil, nil, nil, testLogger())
	if err == nil {
		t.Error("expected error with nil http client, got nil")
	}
}
