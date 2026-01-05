package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/sheets/v4"

	"github.com/ArionMiles/expensor/pkg/api"
	"github.com/ArionMiles/expensor/pkg/client"
	"github.com/ArionMiles/expensor/pkg/config"
	gmailreader "github.com/ArionMiles/expensor/pkg/reader/gmail"
	sheetswriter "github.com/ArionMiles/expensor/pkg/writer/sheets"
)

var (
	//go:embed config/rules.json
	rulesInput string
	//go:embed config/labels.json
	labelsInput string
)

// runExpensor starts the expense tracking daemon.
func runExpensor(logger *slog.Logger) error {
	k := koanf.New(".")

	// Load configuration from environment variables
	if err := k.Load(env.Provider("", ".", nil), nil); err != nil {
		return fmt.Errorf("loading config from environment: %w", err)
	}

	var cfg config.Config
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf", FlatPaths: true}); err != nil {
		return fmt.Errorf("unmarshaling config: %w", err)
	}

	// Validate required configuration
	if cfg.GSheetsName == "" {
		return fmt.Errorf("GSHEETS_NAME environment variable is required")
	}
	if cfg.GSheetsID == "" && cfg.GSheetsTitle == "" {
		return fmt.Errorf("either GSHEETS_ID or GSHEETS_TITLE environment variable is required")
	}

	// Parse rules and labels
	rules, err := parseRules(rulesInput)
	if err != nil {
		return fmt.Errorf("parsing rules: %w", err)
	}
	cfg.Rules = rules

	var labels api.Labels
	if err := json.Unmarshal([]byte(labelsInput), &labels); err != nil {
		return fmt.Errorf("parsing labels: %w", err)
	}
	cfg.Labels = labels

	logger.Info("configuration loaded",
		"rules_count", len(cfg.Rules),
		"labels_count", len(cfg.Labels),
		"sheet_title", cfg.GSheetsTitle,
	)

	// Create OAuth client
	httpClient, err := client.New(
		config.ClientSecretFile,
		gmail.GmailReadonlyScope,
		gmail.GmailModifyScope,
		sheets.SpreadsheetsScope,
	)
	if err != nil {
		return fmt.Errorf("creating http client: %w", err)
	}

	// Create reader and writer
	reader, err := gmailreader.New(httpClient, gmailreader.Config{
		Rules:  cfg.Rules,
		Labels: cfg.Labels,
	}, logger.With("component", "gmail_reader"))
	if err != nil {
		return fmt.Errorf("creating gmail reader: %w", err)
	}

	writer, err := sheetswriter.New(httpClient, sheetswriter.Config{
		SheetTitle: cfg.GSheetsTitle,
		SheetID:    cfg.GSheetsID,
		SheetName:  cfg.GSheetsName,
	}, logger.With("component", "sheets_writer"))
	if err != nil {
		return fmt.Errorf("creating sheets writer: %w", err)
	}

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

	// Create transaction channel
	transactions := make(chan *api.TransactionDetails, 100)

	// Start writer in background
	writerDone := make(chan error, 1)
	go func() {
		writerDone <- writer.Write(ctx, transactions)
	}()

	// Start reader (blocks until context is canceled)
	logger.Info("starting expensor")
	if err := reader.Read(ctx, transactions); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("reader error", "error", err)
	}

	// Wait for writer to finish
	if err := <-writerDone; err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("writer error", "error", err)
	}

	logger.Info("expensor stopped")
	return nil
}

func parseRules(rulesInput string) ([]api.Rule, error) {
	var rawRules []map[string]any
	if err := json.Unmarshal([]byte(rulesInput), &rawRules); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	rules := make([]api.Rule, 0, len(rawRules))
	for _, raw := range rawRules {
		rule, err := parseRule(raw)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return rules, nil
}

func parseRule(raw map[string]any) (api.Rule, error) {
	getString := func(key string) (string, error) {
		val, ok := raw[key].(string)
		if !ok {
			return "", fmt.Errorf("%s: expected string", key)
		}
		return val, nil
	}

	getBool := func(key string) (bool, error) {
		val, ok := raw[key].(bool)
		if !ok {
			return false, fmt.Errorf("%s: expected bool", key)
		}
		return val, nil
	}

	amountRegexStr, err := getString("amountRegex")
	if err != nil {
		return api.Rule{}, err
	}
	amountRegex, err := regexp.Compile(amountRegexStr)
	if err != nil {
		return api.Rule{}, fmt.Errorf("compiling amountRegex: %w", err)
	}

	merchantRegexStr, err := getString("merchantInfoRegex")
	if err != nil {
		return api.Rule{}, err
	}
	merchantRegex, err := regexp.Compile(merchantRegexStr)
	if err != nil {
		return api.Rule{}, fmt.Errorf("compiling merchantInfoRegex: %w", err)
	}

	query, err := getString("query")
	if err != nil {
		return api.Rule{}, err
	}

	enabled, err := getBool("enabled")
	if err != nil {
		return api.Rule{}, err
	}

	source, err := getString("source")
	if err != nil {
		return api.Rule{}, err
	}

	name, _ := getString("name") // optional

	return api.Rule{
		Name:         name,
		Query:        query,
		Amount:       amountRegex,
		MerchantInfo: merchantRegex,
		Enabled:      enabled,
		Source:       source,
	}, nil
}
