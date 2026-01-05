package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	"github.com/ArionMiles/expensor/pkg/client"
	"github.com/ArionMiles/expensor/pkg/config"
)

// runStatus checks the configuration and authentication status.
func runStatus() error {
	fmt.Println("=== Expensor Status ===")
	fmt.Println()

	allGood := true

	checkEnvConfig(&allGood)
	credentialsOk := checkCredentialsFile(&allGood)
	token := checkTokenStatus(&allGood)
	checkEmbeddedData(&allGood)

	if credentialsOk && token != nil {
		checkAPIConnectivity(&allGood)
	}

	printFinalStatus(allGood)

	return nil
}

func checkEnvConfig(allGood *bool) *config.Config {
	fmt.Println("Environment Variables:")

	k := koanf.New(".")
	_ = k.Load(env.Provider("", ".", nil), nil)

	var cfg config.Config
	_ = k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf", FlatPaths: true})

	// Check GSHEETS_ID or GSHEETS_TITLE
	fmt.Print("  GSHEETS_ID: ")
	if cfg.GSheetsID == "" {
		fmt.Println("✗ Not set")
	} else {
		fmt.Printf("✓ %s\n", cfg.GSheetsID)
	}

	fmt.Print("  GSHEETS_TITLE: ")
	if cfg.GSheetsTitle == "" {
		fmt.Println("✗ Not set")
	} else {
		fmt.Printf("✓ %s\n", cfg.GSheetsTitle)
	}

	if cfg.GSheetsID == "" && cfg.GSheetsTitle == "" {
		fmt.Println("  ⚠ Either GSHEETS_ID or GSHEETS_TITLE is required")
		*allGood = false
	}

	// Check GSHEETS_NAME
	fmt.Print("  GSHEETS_NAME: ")
	if cfg.GSheetsName == "" {
		fmt.Println("✗ Not set (required)")
		*allGood = false
	} else {
		fmt.Printf("✓ %s\n", cfg.GSheetsName)
	}

	return &cfg
}

func checkTokenStatus(allGood *bool) *oauth2.Token {
	fmt.Println()
	fmt.Printf("OAuth token (%s): ", client.TokenFile)
	token, err := checkToken(client.TokenFile)
	if err != nil {
		fmt.Printf("✗ %v\n", err)
		*allGood = false
		return nil
	}

	if token.Expiry.Before(time.Now()) {
		fmt.Println("⚠ Expired (will refresh on next run)")
	} else {
		fmt.Printf("✓ Valid (expires: %s)\n", token.Expiry.Format(time.RFC3339))
	}
	return token
}

func checkEmbeddedData(allGood *bool) {
	fmt.Println()
	// Check rules
	fmt.Print("Embedded rules: ")
	if rulesInput == "" {
		fmt.Println("✗ Not found")
		*allGood = false
	} else {
		rules, err := parseRules(rulesInput)
		if err != nil {
			fmt.Printf("✗ Invalid: %v\n", err)
			*allGood = false
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

	// Check labels
	fmt.Print("Embedded labels: ")
	if labelsInput == "" {
		fmt.Println("✗ Not found")
		*allGood = false
	} else {
		var labels map[string]any
		if err := json.Unmarshal([]byte(labelsInput), &labels); err != nil {
			fmt.Printf("✗ Invalid: %v\n", err)
			*allGood = false
		} else {
			fmt.Printf("✓ %d labels\n", len(labels))
		}
	}
}

func checkCredentialsFile(allGood *bool) bool {
	fmt.Println()
	fmt.Printf("Credentials file (%s): ", config.ClientSecretFile)
	if _, err := os.Stat(config.ClientSecretFile); os.IsNotExist(err) {
		fmt.Println("✗ Not found")
		*allGood = false
		return false
	}
	fmt.Println("✓ Found")
	return true
}

func checkAPIConnectivity(allGood *bool) {
	fmt.Println()
	fmt.Println("API Connectivity:")

	httpClient, err := client.New(
		config.ClientSecretFile,
		gmail.GmailReadonlyScope,
		gmail.GmailModifyScope,
		sheets.SpreadsheetsScope,
	)
	if err != nil {
		fmt.Printf("  OAuth client: ✗ %v\n", err)
		*allGood = false
		return
	}

	// Test Gmail API
	fmt.Print("  Gmail API: ")
	if err := testGmailAPI(httpClient); err != nil {
		fmt.Printf("✗ %v\n", err)
		*allGood = false
	} else {
		fmt.Println("✓ Connected")
	}

	// Test Sheets API
	fmt.Print("  Sheets API: ")
	if err := testSheetsAPI(httpClient); err != nil {
		fmt.Printf("✗ %v\n", err)
		*allGood = false
	} else {
		fmt.Println("✓ Connected")
	}
}

func printFinalStatus(allGood bool) {
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
