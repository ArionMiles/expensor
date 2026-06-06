// Package gmail provides a plugin wrapper for the Gmail reader.
package gmail

import (
	"time"

	gmailapi "google.golang.org/api/gmail/v1"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	gmailreader "github.com/ArionMiles/expensor/backend/pkg/reader/gmail"
)

// Plugin implements the ReaderPlugin interface for Gmail.
type Plugin struct {
	guideData []byte
}

// SetGuideData injects the setup guide content. Called by main.go after loading
// the centralized content/readers/gmail/guide.json via go:embed.
func (p *Plugin) SetGuideData(data []byte) { p.guideData = data }

// Metadata returns catalog metadata for the Gmail reader plugin.
func (p *Plugin) Metadata() plugins.ReaderMetadata {
	return plugins.ReaderMetadata{
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
func (p *Plugin) NewReader(input plugins.ReaderInput) (api.Reader, error) {
	cfg := input.AppConfig
	if cfg == nil {
		cfg = &config.App{}
	}
	interval := time.Duration(cfg.ScanInterval) * time.Second
	if interval == 0 {
		interval = 60 * time.Second
	}
	readerCfg := gmailreader.Config{
		Rules:          input.Rules,
		Resolver:       input.Resolver,
		Interval:       interval,
		State:          input.StateManager,
		LookbackDays:   cfg.LookbackDays,
		LastScanAt:     cfg.LastScanAt,
		ForceFullScan:  cfg.ForceFullScan,
		OnCheckpoint:   cfg.OnCheckpoint,
		DiagnosticSink: input.DiagnosticSink,
	}
	return gmailreader.New(input.HTTPClient, readerCfg, input.Logger)
}
