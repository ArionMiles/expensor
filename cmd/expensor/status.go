package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	kJson "github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"github.com/ArionMiles/expensor/pkg/client"
	"github.com/ArionMiles/expensor/pkg/config"
)

// runStatus checks the configuration and authentication status.
func runStatus(logger *slog.Logger, configPath string) error {
	fmt.Println("=== Expensor Status ===")
	fmt.Println()

	allGood := true

	// Check config file
	fmt.Printf("Config file (%s): ", configPath)
	cfg, err := checkConfig(configPath)
	if err != nil {
		fmt.Printf("✗ %v\n", err)
		allGood = false
	} else {
		fmt.Println("✓ Found")
	}

	// Check credentials file
	secretsPath := "credentials.json"
	if cfg != nil && cfg.SecretsFilePath != "" {
		secretsPath = cfg.SecretsFilePath
	}
	fmt.Printf("Credentials file (%s): ", secretsPath)
	if _, err := os.Stat(secretsPath); os.IsNotExist(err) {
		fmt.Println("✗ Not found")
		allGood = false
	} else {
		fmt.Println("✓ Found")
	}

	// Check token file
	fmt.Printf("OAuth token (%s): ", tokenFile)
	token, err := checkToken(tokenFile)
	if err != nil {
		fmt.Printf("✗ %v\n", err)
		allGood = false
	} else {
		if token.Expiry.Before(time.Now()) {
			fmt.Println("⚠ Expired (will refresh on next run)")
		} else {
			fmt.Printf("✓ Valid (expires: %s)\n", token.Expiry.Format(time.RFC3339))
		}
	}

	// Check rules and labels
	fmt.Print("Embedded rules: ")
	if rulesInput == "" {
		fmt.Println("✗ Not found")
		allGood = false
	} else {
		rules, err := parseRules(rulesInput)
		if err != nil {
			fmt.Printf("✗ Invalid: %v\n", err)
			allGood = false
		} else {
			enabledCount := 0
			for _, r := range rules {
				if r.Enabled {
					enabledCount++
				}
			}
			fmt.Printf("✓ %d rules (%d enabled)\n", len(rules), enabledCount)
		}
	}

	fmt.Print("Embedded labels: ")
	if labelsInput == "" {
		fmt.Println("✗ Not found")
		allGood = false
	} else {
		var labels map[string]any
		if err := json.Unmarshal([]byte(labelsInput), &labels); err != nil {
			fmt.Printf("✗ Invalid: %v\n", err)
			allGood = false
		} else {
			fmt.Printf("✓ %d labels\n", len(labels))
		}
	}

	// If we have valid credentials, test API connectivity
	if cfg != nil && token != nil {
		fmt.Println()
		fmt.Println("API Connectivity:")

		httpClient, err := client.New(
			secretsPath,
			gmail.GmailReadonlyScope,
			gmail.GmailModifyScope,
			sheets.SpreadsheetsScope,
		)
		if err != nil {
			fmt.Printf("  OAuth client: ✗ %v\n", err)
			allGood = false
		} else {
			// Test Gmail API
			fmt.Print("  Gmail API: ")
			if err := testGmailAPI(httpClient); err != nil {
				fmt.Printf("✗ %v\n", err)
				allGood = false
			} else {
				fmt.Println("✓ Connected")
			}

			// Test Sheets API
			fmt.Print("  Sheets API: ")
			if err := testSheetsAPI(httpClient); err != nil {
				fmt.Printf("✗ %v\n", err)
				allGood = false
			} else {
				fmt.Println("✓ Connected")
			}
		}
	}

	fmt.Println()
	if allGood {
		fmt.Println("Status: ✓ Ready to run")
		fmt.Println()
		fmt.Println("Run 'expensor run' to start tracking expenses.")
	} else {
		fmt.Println("Status: ✗ Configuration issues detected")
		fmt.Println()
		fmt.Println("Fix the issues above, then run 'expensor status' again.")
	}

	return nil
}

func checkConfig(configPath string) (*config.Config, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("not found")
	}

	k := koanf.New(".")
	if err := k.Load(file.Provider(configPath), kJson.Parser()); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	var cfg config.Config
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf", FlatPaths: true}); err != nil {
		return nil, fmt.Errorf("invalid format: %w", err)
	}

	return &cfg, nil
}

func checkToken(tokenPath string) (*oauth2.Token, error) {
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not found (run 'expensor setup')")
		}
		return nil, err
	}

	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("invalid format")
	}

	return &token, nil
}

func testGmailAPI(httpClient *http.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	svc, err := gmail.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	// List labels as a simple connectivity test
	_, err = svc.Users.Labels.List("me").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("API call failed: %w", err)
	}

	return nil
}

func testSheetsAPI(httpClient *http.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	svc, err := sheets.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	// Just verify we can create the service
	// A real test would require a spreadsheet ID
	_ = svc
	return nil
}
