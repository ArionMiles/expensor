package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"

	"github.com/ArionMiles/expensor/backend/internal/daemon"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/client"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/logging"
	gmailplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/readers/gmail"
	thunderbirdplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/readers/thunderbird"
	postgresplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/writers/postgres"
	"github.com/ArionMiles/expensor/backend/pkg/state"
)

var (
	//go:embed content/rules.json
	rulesInput string
	//go:embed content/labels.json
	labelsInput string
)

// RuleJSON represents a rule in JSON format for parsing.
type RuleJSON struct {
	Name            string `json:"name"`
	SenderEmail     string `json:"senderEmail"`
	SubjectContains string `json:"subjectContains"`
	AmountRegex     string `json:"amountRegex"`
	MerchantRegex   string `json:"merchantInfoRegex"`
	Enabled         bool   `json:"enabled"`
	Source          string `json:"source"`
}

func main() {
	// Setup logging
	logger := logging.Setup(logging.DefaultConfig())

	// Create plugin registry
	registry := plugins.NewRegistry()

	// Register reader plugins
	if err := registry.RegisterReader(&gmailplugin.Plugin{}); err != nil {
		logger.Error("failed to register gmail plugin", "error", err)
		os.Exit(1)
	}
	if err := registry.RegisterReader(&thunderbirdplugin.Plugin{}); err != nil {
		logger.Error("failed to register thunderbird plugin", "error", err)
		os.Exit(1)
	}

	// Register writer plugins
	if err := registry.RegisterWriter(&postgresplugin.Plugin{}); err != nil {
		logger.Error("failed to register postgres plugin", "error", err)
		os.Exit(1)
	}

	logger.Info("plugins registered",
		"readers", len(registry.ListReaders()),
		"writers", len(registry.ListWriters()),
	)

	// Load configuration from environment
	// Use a callback to keep env var names unchanged (no delimiter transformation)
	k := koanf.New(".")
	envProvider := env.Provider("", ".", func(s string) string {
		return s // Keep env var names as-is
	})
	if err := k.Load(envProvider, nil); err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	var cfg config.Config
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf", FlatPaths: true}); err != nil {
		logger.Error("failed to unmarshal config", "error", err)
		os.Exit(1)
	}

	// Apply default values
	cfg.ApplyDefaults()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	logger.Info("configuration loaded",
		"reader", cfg.ReaderPlugin,
		"writer", cfg.WriterPlugin,
		"state_file", cfg.StateFile,
	)

	// Parse rules from embedded file
	rules, err := parseRules(rulesInput)
	if err != nil {
		logger.Error("failed to parse rules", "error", err)
		os.Exit(1)
	}
	logger.Info("loaded rules", "count", len(rules))

	// Parse labels from embedded file
	var labels api.Labels
	if err := json.Unmarshal([]byte(labelsInput), &labels); err != nil {
		logger.Error("failed to parse labels", "error", err)
		os.Exit(1)
	}
	logger.Info("loaded labels", "count", len(labels))

	// Create state manager
	stateManager, err := state.New(cfg.StateFile, logger)
	if err != nil {
		logger.Error("failed to create state manager", "error", err)
		os.Exit(1)
	}
	logger.Info("state manager initialized", "processed_messages", stateManager.Count())

	// Get required OAuth scopes from plugins
	scopes, err := registry.GetAllScopes(cfg.ReaderPlugin, cfg.WriterPlugin)
	if err != nil {
		logger.Error("failed to get required scopes", "error", err)
		os.Exit(1)
	}

	var httpClient *http.Client
	if len(scopes) > 0 {
		logger.Info("OAuth scopes required", "scopes", scopes)

		// Create OAuth client
		httpClient, err = client.New(config.ClientSecretFile, scopes...)
		if err != nil {
			logger.Error("failed to create http client", "error", err)
			os.Exit(1)
		}
	} else {
		logger.Info("no OAuth scopes required, skipping OAuth client creation")
	}

	// Create daemon runner
	runner := daemon.New(registry, httpClient, logger)

	// Setup context with cancellation on SIGINT/SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Info("received shutdown signal", "signal", sig)
		cancel()
	}()

	// Run daemon
	runCfg := daemon.RunConfig{
		Config:       &cfg,
		Rules:        rules,
		Labels:       labels,
		StateManager: stateManager,
	}

	if err := runner.Run(ctx, runCfg); err != nil {
		logger.Error("daemon failed", "error", err)
		os.Exit(1)
	}
}

// parseRules parses the rules JSON into []api.Rule.
func parseRules(rulesJSON string) ([]api.Rule, error) {
	var rawRules []RuleJSON
	if err := json.Unmarshal([]byte(rulesJSON), &rawRules); err != nil {
		return nil, fmt.Errorf("parsing rules JSON: %w", err)
	}

	rules := make([]api.Rule, 0, len(rawRules))
	for i, raw := range rawRules {
		amountRegex, err := regexp.Compile(raw.AmountRegex)
		if err != nil {
			return nil, fmt.Errorf("compiling amountRegex for rule %d (%s): %w", i, raw.Name, err)
		}

		merchantRegex, err := regexp.Compile(raw.MerchantRegex)
		if err != nil {
			return nil, fmt.Errorf("compiling merchantInfoRegex for rule %d (%s): %w", i, raw.Name, err)
		}

		rules = append(rules, api.Rule{
			Name:            raw.Name,
			SenderEmail:     raw.SenderEmail,
			SubjectContains: raw.SubjectContains,
			Amount:          amountRegex,
			MerchantInfo:    merchantRegex,
			Enabled:         raw.Enabled,
			Source:          raw.Source,
		})
	}

	return rules, nil
}
