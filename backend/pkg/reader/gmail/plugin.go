// Package gmail provides the Gmail reader and its plugin integration.
package gmail

import (
	"time"

	gmailapi "google.golang.org/api/gmail/v1"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
)

// Plugin builds Gmail provider capabilities.
type Plugin struct {
	guideData []byte
}

// SetGuideData injects the setup guide content. Application composition calls it
// after loading backend/internal/catalog/content/readers/gmail/guide.json via go:embed.
func (p *Plugin) SetGuideData(data []byte) { p.guideData = data }

// Provider returns the Gmail provider registration.
func Provider(guideData []byte) plugins.Provider {
	plugin := &Plugin{guideData: guideData}
	return plugins.Provider{
		Metadata:         plugin.Metadata(),
		NewReader:        plugin.NewReader,
		NewEmailSearcher: plugin.NewEmailSearcher,
	}
}

// Metadata returns catalog metadata for the Gmail provider.
func (p *Plugin) Metadata() plugins.ProviderMetadata {
	return plugins.ProviderMetadata{
		Name:        "gmail",
		Description: "Read expense transactions from Gmail messages",
		Auth: plugins.AuthSpec{
			Type:                      plugins.AuthTypeOAuth,
			RequiredScopes:            []string{gmailapi.GmailReadonlyScope},
			RequiresCredentialsUpload: true,
		},
		ConfigSchema: []plugins.ConfigField{},
		SetupGuide:   p.guideData,
	}
}

// NewReader creates a new Gmail reader instance.
func (p *Plugin) NewReader(input plugins.ProviderInput) (api.Reader, error) {
	return p.buildReader(input)
}

// NewEmailSearcher creates a new Gmail email searcher instance.
func (p *Plugin) NewEmailSearcher(input plugins.ProviderInput) (api.EmailSearcher, error) {
	return p.buildReader(input)
}

func (p *Plugin) buildReader(input plugins.ProviderInput) (*Reader, error) {
	cfg := input.AppConfig
	if cfg == nil {
		cfg = &config.App{}
	}
	interval := time.Duration(cfg.ScanInterval) * time.Second
	if interval == 0 {
		interval = 60 * time.Second
	}
	readerCfg := Config{
		Rules:          input.Rules,
		Resolver:       input.Resolver,
		Interval:       interval,
		State:          input.StateManager,
		LookbackDays:   cfg.LookbackDays,
		LastScanAt:     cfg.LastScanAt,
		ForceFullScan:  cfg.ForceFullScan,
		RunOnce:        cfg.RunOnce,
		OnCheckpoint:   cfg.OnCheckpoint,
		DiagnosticSink: input.DiagnosticSink,
	}
	return New(input.HTTPClient, readerCfg, input.Logger)
}
