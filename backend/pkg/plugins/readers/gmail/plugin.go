// Package gmail provides a plugin wrapper for the Gmail reader.
package gmail

import (
	"log/slog"
	"net/http"
	"time"

	gmailapi "google.golang.org/api/gmail/v1"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	gmailreader "github.com/ArionMiles/expensor/backend/pkg/reader/gmail"
	"github.com/ArionMiles/expensor/backend/pkg/state"
)

// Plugin implements the ReaderPlugin interface for Gmail.
type Plugin struct{}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "gmail"
}

// Description returns a human-readable description.
func (p *Plugin) Description() string {
	return "Read expense transactions from Gmail messages"
}

// RequiredScopes returns the OAuth scopes needed by this plugin.
func (p *Plugin) RequiredScopes() []string {
	return []string{
		gmailapi.GmailReadonlyScope,
		gmailapi.GmailModifyScope,
	}
}

// AuthType returns the authentication type for Gmail (OAuth2).
func (p *Plugin) AuthType() plugins.AuthType {
	return plugins.AuthTypeOAuth
}

// RequiresCredentialsUpload reports that Gmail needs a client_secret.json upload.
func (p *Plugin) RequiresCredentialsUpload() bool {
	return true
}

// ConfigSchema returns an empty schema — Gmail is configured via OAuth, not form fields.
func (p *Plugin) ConfigSchema() []plugins.ConfigField {
	return []plugins.ConfigField{}
}

// NewReader creates a new Gmail reader instance.
func (p *Plugin) NewReader( //nolint:revive // interface method; argument count dictated by ReaderPlugin
	httpClient *http.Client, cfg *config.Config, rules []api.Rule,
	labels api.Labels, stateManager *state.Manager, logger *slog.Logger,
) (api.Reader, error) {
	// Get interval from config
	interval := time.Duration(cfg.ScanInterval) * time.Second
	if interval == 0 {
		interval = 60 * time.Second
	}

	readerCfg := gmailreader.Config{
		Rules:        rules,
		Labels:       labels,
		Interval:     interval,
		State:        stateManager,
		LookbackDays: cfg.LookbackDays,
	}

	return gmailreader.New(httpClient, readerCfg, logger)
}
