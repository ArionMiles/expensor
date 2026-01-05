// Command emaildump fetches emails matching rules and dumps their bodies to files.
// This utility is used to collect email samples for unit testing.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	kJson "github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"github.com/ArionMiles/expensor/pkg/client"
	"github.com/ArionMiles/expensor/pkg/config"
	"github.com/ArionMiles/expensor/pkg/logging"
)

var k = koanf.New(".")

const dumpDir = "tests/data/dump"

func main() {
	logger := logging.Setup(logging.DefaultConfig())

	// Load configuration
	if err := k.Load(file.Provider("config.json"), kJson.Parser()); err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	var cfg config.Config
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf", FlatPaths: true}); err != nil {
		logger.Error("failed to unmarshal config", "error", err)
		os.Exit(1)
	}

	// Load rules from the same location as expensor
	rulesData, err := os.ReadFile("cmd/expensor/config/rules.json")
	if err != nil {
		logger.Error("failed to read rules file", "error", err)
		os.Exit(1)
	}

	rules, err := parseRules(string(rulesData))
	if err != nil {
		logger.Error("failed to parse rules", "error", err)
		os.Exit(1)
	}

	// Create OAuth client
	httpClient, err := client.New(
		cfg.SecretsFilePath,
		gmail.GmailReadonlyScope,
	)
	if err != nil {
		logger.Error("failed to create http client", "error", err)
		os.Exit(1)
	}

	// Create Gmail service
	gmailSvc, err := gmail.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		logger.Error("failed to create gmail service", "error", err)
		os.Exit(1)
	}

	// Create dump directory
	if err := os.MkdirAll(dumpDir, 0o755); err != nil {
		logger.Error("failed to create dump directory", "error", err)
		os.Exit(1)
	}

	// Process each rule
	totalDumped := 0
	for _, rule := range rules {
		if !rule.Enabled {
			logger.Debug("skipping disabled rule", "rule", rule.Name)
			continue
		}

		logger.Info("processing rule", "name", rule.Name, "query", rule.Query)

		count, err := dumpMessagesForRule(context.Background(), gmailSvc, rule, logger)
		if err != nil {
			logger.Error("failed to dump messages for rule", "rule", rule.Name, "error", err)
			continue
		}

		logger.Info("dumped messages for rule", "rule", rule.Name, "count", count)
		totalDumped += count
	}

	logger.Info("email dump complete", "total_dumped", totalDumped, "directory", dumpDir)
}

func dumpMessagesForRule(ctx context.Context, svc *gmail.Service, rule Rule, logger *slog.Logger) (int, error) {
	resp, err := svc.Users.Messages.List("me").Q(rule.Query).MaxResults(10).Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("listing messages: %w", err)
	}

	count := 0
	for _, msg := range resp.Messages {
		if err := dumpMessage(ctx, svc, msg.Id, rule.Source, logger); err != nil {
			logger.Warn("failed to dump message", "message_id", msg.Id, "error", err)
			continue
		}
		count++
	}

	return count, nil
}

func dumpMessage(ctx context.Context, svc *gmail.Service, msgID, source string, logger *slog.Logger) error {
	msg, err := svc.Users.Messages.Get("me", msgID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("getting message: %w", err)
	}

	// Extract subject and date
	var subject string
	var dateStr string
	for _, header := range msg.Payload.Headers {
		switch header.Name {
		case "Subject":
			subject = header.Value
		case "Date":
			dateStr = header.Value
		}
	}

	// Parse and format date
	receivedTime := time.Unix(msg.InternalDate/1000, 0)
	dateFormatted := receivedTime.Format("2006-01-02_150405")

	// Extract body
	body := extractBody(msg)
	if body == "" {
		return fmt.Errorf("empty message body")
	}

	// Create filename: source_date_subject.txt
	filename := sanitizeFilename(fmt.Sprintf("%s_%s_%s.txt", source, dateFormatted, subject))
	filePath := filepath.Join(dumpDir, filename)

	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		logger.Debug("file already exists, skipping", "file", filename)
		return nil
	}

	// Write to file
	if err := os.WriteFile(filePath, []byte(body), 0o644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	logger.Info("dumped email",
		"file", filename,
		"subject", subject,
		"date", dateStr,
	)

	return nil
}

func extractBody(msg *gmail.Message) string {
	// Try to find text/plain body first for easier testing
	for _, part := range msg.Payload.Parts {
		if part.MimeType == "text/plain" {
			bodyBytes, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err != nil {
				continue
			}
			return string(bodyBytes)
		}
	}

	// Try HTML body
	for _, part := range msg.Payload.Parts {
		if part.MimeType == "text/html" {
			bodyBytes, err := base64.URLEncoding.DecodeString(part.Body.Data)
			if err != nil {
				continue
			}
			return string(bodyBytes)
		}
	}

	// Fallback to direct body data
	if msg.Payload.Body != nil && msg.Payload.Body.Data != "" {
		bodyBytes, err := base64.URLEncoding.DecodeString(msg.Payload.Body.Data)
		if err == nil {
			return string(bodyBytes)
		}
	}

	return ""
}

func sanitizeFilename(name string) string {
	// Replace unsafe characters with underscores
	unsafe := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
	name = unsafe.ReplaceAllString(name, "_")

	// Collapse multiple underscores
	name = regexp.MustCompile(`_+`).ReplaceAllString(name, "_")

	// Trim underscores and limit length
	name = strings.Trim(name, "_")
	if len(name) > 200 {
		name = name[:200]
	}

	return name
}

// Rule defines an email matching rule.
type Rule struct {
	Name    string
	Query   string
	Enabled bool
	Source  string
}

func parseRules(rulesInput string) ([]Rule, error) {
	var rawRules []map[string]any
	if err := json.Unmarshal([]byte(rulesInput), &rawRules); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	rules := make([]Rule, 0, len(rawRules))
	for _, raw := range rawRules {
		rule := Rule{}

		if name, ok := raw["name"].(string); ok {
			rule.Name = name
		}
		if query, ok := raw["query"].(string); ok {
			rule.Query = query
		}
		if enabled, ok := raw["enabled"].(bool); ok {
			rule.Enabled = enabled
		}
		if source, ok := raw["source"].(string); ok {
			rule.Source = source
		}

		rules = append(rules, rule)
	}

	return rules, nil
}
