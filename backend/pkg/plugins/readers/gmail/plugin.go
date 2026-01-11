// Package gmail provides a plugin wrapper for the Gmail reader.
package gmail

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	gmailapi "google.golang.org/api/gmail/v1"

	"github.com/ArionMiles/expensor/backend/pkg/api"
	gmailreader "github.com/ArionMiles/expensor/backend/pkg/reader/gmail"
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

// ConfigSchema returns a JSON schema describing the plugin's configuration.
func (p *Plugin) ConfigSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"rules": map[string]any{
				"type":        "array",
				"description": "Transaction extraction rules",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type":        "string",
							"description": "Rule name for identification",
						},
						"query": map[string]any{
							"type":        "string",
							"description": "Gmail search query to match messages",
						},
						"amountRegex": map[string]any{
							"type":        "string",
							"description": "Regex pattern to extract amount",
						},
						"merchantInfoRegex": map[string]any{
							"type":        "string",
							"description": "Regex pattern to extract merchant info",
						},
						"enabled": map[string]any{
							"type":        "boolean",
							"description": "Whether this rule is enabled",
						},
						"source": map[string]any{
							"type":        "string",
							"description": "Transaction source identifier",
						},
					},
					"required": []string{"query", "amountRegex", "merchantInfoRegex", "enabled", "source"},
				},
			},
			"labels": map[string]any{
				"type":        "object",
				"description": "Merchant to category/bucket mappings",
				"additionalProperties": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"category": map[string]any{
							"type":        "string",
							"description": "Expense category",
						},
						"bucket": map[string]any{
							"type":        "string",
							"description": "Expense bucket (Need/Want/Investment)",
						},
					},
					"required": []string{"category", "bucket"},
				},
			},
			"interval": map[string]any{
				"type":        "integer",
				"description": "Interval in seconds between rule evaluations (default: 10)",
				"default":     10,
			},
		},
		"required": []string{"rules", "labels"},
	}
}

// Config represents the Gmail reader configuration.
type Config struct {
	Rules    []RuleConfig `json:"rules"`
	Labels   api.Labels   `json:"labels"`
	Interval int          `json:"interval,omitempty"` // in seconds
}

// RuleConfig represents a single rule configuration.
type RuleConfig struct {
	Name              string `json:"name"`
	Query             string `json:"query"`
	AmountRegex       string `json:"amountRegex"`
	MerchantInfoRegex string `json:"merchantInfoRegex"`
	Enabled           bool   `json:"enabled"`
	Source            string `json:"source"`
}

// NewReader creates a new Gmail reader instance.
func (p *Plugin) NewReader(httpClient *http.Client, configData json.RawMessage, logger *slog.Logger) (api.Reader, error) {
	var cfg Config
	if err := json.Unmarshal(configData, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling gmail config: %w", err)
	}

	// Parse rules
	rules := make([]api.Rule, 0, len(cfg.Rules))
	for i, ruleConfig := range cfg.Rules {
		rule, err := parseRule(ruleConfig)
		if err != nil {
			return nil, fmt.Errorf("parsing rule %d (%s): %w", i, ruleConfig.Name, err)
		}
		rules = append(rules, rule)
	}

	// Convert interval to duration
	interval := time.Duration(cfg.Interval) * time.Second
	if interval == 0 {
		interval = 10 * time.Second
	}

	readerCfg := gmailreader.Config{
		Rules:    rules,
		Labels:   cfg.Labels,
		Interval: interval,
	}

	return gmailreader.New(httpClient, readerCfg, logger)
}

func parseRule(cfg RuleConfig) (api.Rule, error) {
	amountRegex, err := regexp.Compile(cfg.AmountRegex)
	if err != nil {
		return api.Rule{}, fmt.Errorf("compiling amountRegex: %w", err)
	}

	merchantRegex, err := regexp.Compile(cfg.MerchantInfoRegex)
	if err != nil {
		return api.Rule{}, fmt.Errorf("compiling merchantInfoRegex: %w", err)
	}

	return api.Rule{
		Name:         cfg.Name,
		Query:        cfg.Query,
		Amount:       amountRegex,
		MerchantInfo: merchantRegex,
		Enabled:      cfg.Enabled,
		Source:       cfg.Source,
	}, nil
}
