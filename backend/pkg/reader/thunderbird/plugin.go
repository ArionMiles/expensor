// Package thunderbird provides the Thunderbird reader and its plugin integration.
package thunderbird

import (
	"encoding/json"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
)

// Plugin builds Thunderbird provider capabilities.
type Plugin struct {
	guideData []byte
}

// SetGuideData injects the setup guide content. Called by main.go after loading
// the backend-owned content/readers/thunderbird/guide.json via go:embed.
func (p *Plugin) SetGuideData(data []byte) { p.guideData = data }

// Provider returns the Thunderbird provider registration.
func Provider(guideData []byte) plugins.Provider {
	plugin := &Plugin{guideData: guideData}
	return plugins.Provider{
		Metadata:         plugin.Metadata(),
		NewReader:        plugin.NewReader,
		NewEmailSearcher: plugin.NewEmailSearcher,
	}
}

// Metadata returns catalog metadata for the Thunderbird provider.
func (p *Plugin) Metadata() plugins.ProviderMetadata {
	return plugins.ProviderMetadata{
		Name:        "thunderbird",
		Description: "Read expense transactions from Thunderbird mailbox files (MBOX format)",
		Auth: plugins.AuthSpec{
			Type:                      plugins.AuthTypeConfig,
			RequiredScopes:            []string{},
			RequiresCredentialsUpload: false,
		},
		ConfigSchema: p.ConfigSchema(),
		SetupGuide:   p.guideData,
	}
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

// SetupGuide returns the injected setup guide for Thunderbird.
func (p *Plugin) SetupGuide() []byte { return p.guideData }

// ApplyConfig maps the web-UI-persisted JSON config onto config.App.
// The frontend wraps fields under a "config" key: {"config":{"profilePath":...}}.
func (p *Plugin) ApplyConfig(cfg *config.App, raw map[string]any) {
	fields, ok := raw["config"].(map[string]any)
	if !ok {
		return
	}
	if v, ok := fields["profilePath"].(string); ok {
		cfg.Thunderbird.ProfilePath = v
	}
	if v, ok := fields["mailboxes"].(string); ok {
		cfg.Thunderbird.Mailboxes = v
	}
}

// NewReader creates a new Thunderbird reader instance.
func (p *Plugin) NewReader(input plugins.ProviderInput) (api.Reader, error) {
	return p.buildReader(input)
}

// NewEmailSearcher creates a new Thunderbird email searcher instance.
func (p *Plugin) NewEmailSearcher(input plugins.ProviderInput) (api.EmailSearcher, error) {
	return p.buildReader(input)
}

func (p *Plugin) buildReader(input plugins.ProviderInput) (*Reader, error) {
	cfg := config.App{}
	if input.AppConfig != nil {
		cfg = *input.AppConfig
	}
	if len(input.ReaderConfig) > 0 {
		var raw map[string]any
		if err := json.Unmarshal(input.ReaderConfig, &raw); err == nil {
			p.ApplyConfig(&cfg, raw)
		}
	}

	readerCfg := Config{
		ProfilePath:    cfg.Thunderbird.ProfilePath,
		Mailboxes:      cfg.Thunderbird.GetMailboxes(),
		Rules:          input.Rules,
		Resolver:       input.Resolver,
		State:          input.StateManager,
		Interval:       time.Duration(cfg.ScanInterval) * time.Second,
		LastScanAt:     cfg.LastScanAt,
		ForceFullScan:  cfg.ForceFullScan,
		RunOnce:        cfg.RunOnce,
		OnCheckpoint:   cfg.OnCheckpoint,
		DiagnosticSink: input.DiagnosticSink,
	}
	return New(readerCfg, input.Logger)
}
