// Package thunderbird provides a plugin wrapper for the Thunderbird reader.
package thunderbird

import (
	"log/slog"
	"net/http"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	tbreader "github.com/ArionMiles/expensor/backend/pkg/reader/thunderbird"
	"github.com/ArionMiles/expensor/backend/pkg/state"
)

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
		Interval:    cfg.Thunderbird.GetInterval(),
	}

	return tbreader.New(readerCfg, logger)
}
