package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"regexp"

	kJson "github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/pkg/errors"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/sheets/v4"

	"github.com/ArionMiles/expensor/pkg/api"
	"github.com/ArionMiles/expensor/pkg/client"
	"github.com/ArionMiles/expensor/pkg/config"
	"github.com/ArionMiles/expensor/pkg/orchestrator"
)

var (
	//go:embed config/rules.json
	rulesInput string
	rules      []api.Rule
	//go:embed config/labels.json
	labelsInput string
	labels      api.Labels
	k           = koanf.New(".")
)

func init() {
	err := generateRules(rulesInput)
	if err != nil {
		log.Fatalf("Failed to generate rules: %s", err)
	}

	if err := json.Unmarshal([]byte(labelsInput), &labels); err != nil {
		log.Fatalf("failed to read labels JSON: %s", err)
	}

}

func main() {

	// Load JSON config.
	if err := k.Load(file.Provider("config.json"), kJson.Parser()); err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	var cfg config.Config
	k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf", FlatPaths: true})
	fmt.Println(cfg)
	cfg.Rules = rules
	cfg.Labels = labels

	client, err := client.NewClient(
		cfg.SecretsFilePath,
		gmail.GmailReadonlyScope,
		gmail.GmailModifyScope,
		sheets.SpreadsheetsScope,
	)

	if err != nil {
		log.Fatalf("Failed to initialize http client %v", err)
	}

	expensor, err := orchestrator.NewExpensor(client, &cfg)
	if err != nil {
		log.Fatalf("Failed to create expensor %v", err)
	}

	// Start Writer
	expensor.Write(context.Background())

	expensor.Read(context.Background())

}

func generateRules(rulesInput string) error {
	// Parse the JSON into a slice of map[string]string to handle custom unmarshalling
	var rawRules []map[string]any
	if err := json.Unmarshal([]byte(rulesInput), &rawRules); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Convert the raw rules to the []api.Rule slice
	for _, rawRule := range rawRules {
		amountRegexString, ok := rawRule["amountRegex"].(string)
		if !ok {
			return errors.New("amountRegex incorrect")
		}
		amountRegex, err := regexp.Compile(amountRegexString)
		if err != nil {
			return fmt.Errorf("invalid amount regex: %w", err)
		}

		merchantInfoRegexString, ok := rawRule["merchantInfoRegex"].(string)
		if !ok {
			return errors.New("merchantInfoRegex incorrect")
		}

		merchantInfoRegex, err := regexp.Compile(merchantInfoRegexString)
		if err != nil {
			return fmt.Errorf("invalid merchant info regex: %w", err)
		}

		query, ok := rawRule["query"].(string)
		if !ok {
			return errors.New("query incorrect")
		}

		enabled, ok := rawRule["enabled"].(bool)
		if !ok {
			return errors.New("query incorrect")
		}

		source, ok := rawRule["source"].(string)
		if !ok {
			return errors.New("source incorrect")
		}

		rule := api.Rule{
			Query:        query,
			Amount:       amountRegex,
			MerchantInfo: merchantInfoRegex,
			Enabled:      enabled,
			Source:       source,
		}
		rules = append(rules, rule)
	}

	return nil
}
