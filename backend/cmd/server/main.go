package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"

	"github.com/ArionMiles/expensor/backend/internal/daemon"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/client"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/logging"
	gmailplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/readers/gmail"
	csvplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/writers/csv"
	jsonplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/writers/json"
	postgresplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/writers/postgres"
	sheetsplugin "github.com/ArionMiles/expensor/backend/pkg/plugins/writers/sheets"
)

var (
	//go:embed content/rules.json
	rulesInput string
	//go:embed content/labels.json
	labelsInput string
)

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

	// Register writer plugins
	if err := registry.RegisterWriter(&sheetsplugin.Plugin{}); err != nil {
		logger.Error("failed to register sheets plugin", "error", err)
		os.Exit(1)
	}
	if err := registry.RegisterWriter(&csvplugin.Plugin{}); err != nil {
		logger.Error("failed to register csv plugin", "error", err)
		os.Exit(1)
	}
	if err := registry.RegisterWriter(&jsonplugin.Plugin{}); err != nil {
		logger.Error("failed to register json plugin", "error", err)
		os.Exit(1)
	}
	if err := registry.RegisterWriter(&postgresplugin.Plugin{}); err != nil {
		logger.Error("failed to register postgres plugin", "error", err)
		os.Exit(1)
	}

	logger.Info("plugins registered",
		"readers", len(registry.ListReaders()),
		"writers", len(registry.ListWriters()),
	)

	// Load configuration from environment
	k := koanf.New(".")
	if err := k.Load(env.Provider("", ".", nil), nil); err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	var cfg config.Config
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf", FlatPaths: true}); err != nil {
		logger.Error("failed to unmarshal config", "error", err)
		os.Exit(1)
	}

	// If no plugin config provided, build default config from embedded files
	if cfg.ReaderPlugin == "" {
		cfg.ReaderPlugin = "gmail"
	}
	if len(cfg.ReaderConfig) == 0 {
		readerCfg, err := buildDefaultReaderConfig()
		if err != nil {
			logger.Error("failed to build default reader config", "error", err)
			os.Exit(1)
		}
		cfg.ReaderConfig = readerCfg
	}

	if cfg.WriterPlugin == "" {
		cfg.WriterPlugin = "sheets"
	}
	if len(cfg.WriterConfig) == 0 {
		writerCfg, err := buildDefaultWriterConfig(k)
		if err != nil {
			logger.Error("failed to build default writer config", "error", err)
			os.Exit(1)
		}
		cfg.WriterConfig = writerCfg
	}

	logger.Info("configuration loaded",
		"reader", cfg.ReaderPlugin,
		"writer", cfg.WriterPlugin,
	)

	// Get required OAuth scopes from plugins
	scopes, err := registry.GetAllScopes(cfg.ReaderPlugin, cfg.WriterPlugin)
	if err != nil {
		logger.Error("failed to get required scopes", "error", err)
		os.Exit(1)
	}

	logger.Info("OAuth scopes required", "scopes", scopes)

	// Create OAuth client
	httpClient, err := client.New(config.ClientSecretFile, scopes...)
	if err != nil {
		logger.Error("failed to create http client", "error", err)
		os.Exit(1)
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
	if err := runner.Run(ctx, cfg); err != nil {
		logger.Error("daemon failed", "error", err)
		os.Exit(1)
	}
}

// buildDefaultReaderConfig builds default reader config from embedded files.
func buildDefaultReaderConfig() (json.RawMessage, error) {
	// Parse rules
	var rawRules []map[string]any
	if err := json.Unmarshal([]byte(rulesInput), &rawRules); err != nil {
		return nil, fmt.Errorf("parsing rules: %w", err)
	}

	// Parse labels
	var labels map[string]any
	if err := json.Unmarshal([]byte(labelsInput), &labels); err != nil {
		return nil, fmt.Errorf("parsing labels: %w", err)
	}

	// Build config
	cfg := map[string]any{
		"rules":    rawRules,
		"labels":   labels,
		"interval": 10,
	}

	return json.Marshal(cfg)
}

// buildDefaultWriterConfig builds default writer config from environment.
func buildDefaultWriterConfig(k *koanf.Koanf) (json.RawMessage, error) {
	sheetTitle := k.String("GSHEETS_TITLE")
	sheetID := k.String("GSHEETS_ID")
	sheetName := k.String("GSHEETS_NAME")

	if sheetName == "" {
		return nil, fmt.Errorf("GSHEETS_NAME is required")
	}
	if sheetID == "" && sheetTitle == "" {
		return nil, fmt.Errorf("either GSHEETS_ID or GSHEETS_TITLE is required")
	}

	cfg := map[string]any{
		"sheetName":     sheetName,
		"batchSize":     10,
		"flushInterval": 30,
	}

	if sheetTitle != "" {
		cfg["sheetTitle"] = sheetTitle
	}
	if sheetID != "" {
		cfg["sheetId"] = sheetID
	}

	return json.Marshal(cfg)
}
