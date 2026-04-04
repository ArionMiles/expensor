// Package thunderbird provides a plugin wrapper for the Thunderbird reader.
package thunderbird

import (
	_ "embed"
	"log/slog"
	"net/http"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	tbreader "github.com/ArionMiles/expensor/backend/pkg/reader/thunderbird"
	"github.com/ArionMiles/expensor/backend/pkg/state"
)

//go:embed guide.json
var guideData []byte

// Plugin implements the ReaderPlugin interface for Thunderbird.
type Plugin struct{}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "thunderbird"
}

// Description returns a human-readable description.
func (p *Plugin) Description() string {
	return "Read expense transactions from Thunderbird mailbox files (MBOX format)"
}

// RequiredScopes returns the OAuth scopes needed by this plugin.
// Thunderbird reader doesn't need OAuth as it reads local files.
func (p *Plugin) RequiredScopes() []string {
	return []string{}
}

// AuthType returns the authentication type for Thunderbird (config-only, no OAuth).
func (p *Plugin) AuthType() plugins.AuthType {
	return plugins.AuthTypeConfig
}

// RequiresCredentialsUpload reports that Thunderbird does not need credentials upload.
func (p *Plugin) RequiresCredentialsUpload() bool {
	return false
}

// ConfigSchema returns the fields required to configure Thunderbird.
func (p *Plugin) ConfigSchema() []plugins.ConfigField {
	return []plugins.ConfigField{
		{
			Key:      "profilePath",
			Label:    "Thunderbird Profile Directory",
			Type:     "thunderbird-profile",
			Required: true,
			Help:     "Path to your Thunderbird profile directory (contains Mail/ and ImapMail/).",
		},
		{
			Key:       "mailboxes",
			Label:     "Mailboxes to scan",
			Type:      "thunderbird-mailboxes",
			Required:  true,
			DependsOn: "profilePath",
			Help:      "Select mailboxes to scan. Comma-separated if entering manually (e.g. INBOX,Sent).",
		},
	}
}

// SetupGuide returns the embedded setup guide for Thunderbird.
func (p *Plugin) SetupGuide() []byte { return guideData }

// ApplyConfig maps the web-UI-persisted JSON config onto config.Config.
// Keys match the ConfigSchema: "profilePath" and "mailboxes".
func (p *Plugin) ApplyConfig(cfg *config.Config, raw map[string]any) {
	if v, ok := raw["profilePath"].(string); ok {
		cfg.Thunderbird.ProfilePath = v
	}
	if v, ok := raw["mailboxes"].(string); ok {
		cfg.Thunderbird.Mailboxes = v
	}
}

// NewReader creates a new Thunderbird reader instance.
// The httpClient parameter is unused for Thunderbird (no OAuth needed).
func (p *Plugin) NewReader( //nolint:revive // interface method; argument count dictated by ReaderPlugin
	httpClient *http.Client, cfg *config.Config, rules []api.Rule,
	labels api.Labels, stateManager *state.Manager, logger *slog.Logger,
) (api.Reader, error) {
	readerCfg := tbreader.Config{
		ProfilePath: cfg.Thunderbird.ProfilePath,
		Mailboxes:   cfg.Thunderbird.GetMailboxes(),
		Rules:       rules,
		Labels:      labels,
		State:       stateManager,
		Interval:    time.Duration(cfg.ScanInterval) * time.Second,
	}

	return tbreader.New(readerCfg, logger)
}
