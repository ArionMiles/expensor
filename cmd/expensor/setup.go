package main

import (
	"fmt"
	"log/slog"
	"os"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/sheets/v4"

	"github.com/ArionMiles/expensor/pkg/client"
)

const tokenFile = "token.json"

// runSetup handles the OAuth setup flow.
func runSetup(logger *slog.Logger, secretsPath string, force bool) error {
	fmt.Println("=== Expensor Setup ===")
	fmt.Println()

	// Check if credentials file exists
	if _, err := os.Stat(secretsPath); os.IsNotExist(err) {
		return fmt.Errorf("credentials file not found: %s\n\nTo get your credentials:\n"+
			"1. Go to https://console.cloud.google.com/apis/credentials\n"+
			"2. Create an OAuth 2.0 Client ID (Desktop application)\n"+
			"3. Download the JSON file and save it as '%s'", secretsPath, secretsPath)
	}

	// Check if already authenticated
	if !force {
		if _, err := os.Stat(tokenFile); err == nil {
			fmt.Printf("Already authenticated! Token file exists: %s\n", tokenFile)
			fmt.Println()
			fmt.Println("To re-authenticate, run: expensor setup --force")
			return nil
		}
	}

	// Remove existing token if force flag is set
	if force {
		if err := os.Remove(tokenFile); err != nil && !os.IsNotExist(err) {
			logger.Warn("failed to remove existing token", "error", err)
		}
		fmt.Println("Forcing re-authentication...")
		fmt.Println()
	}

	fmt.Println("This will set up OAuth authentication with Google.")
	fmt.Println()
	fmt.Println("Required permissions:")
	fmt.Println("  - Gmail: Read and modify emails (to mark processed emails as read)")
	fmt.Println("  - Sheets: Read and write spreadsheets")
	fmt.Println()
	fmt.Println("Starting authentication...")
	fmt.Println()

	// Trigger OAuth flow by creating client
	_, err := client.New(
		secretsPath,
		gmail.GmailReadonlyScope,
		gmail.GmailModifyScope,
		sheets.SpreadsheetsScope,
	)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	fmt.Println()
	fmt.Println("=== Setup Complete ===")
	fmt.Println()
	fmt.Printf("Token saved to: %s\n", tokenFile)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Create a config.json file (see README for format)")
	fmt.Println("  2. Run 'expensor run' to start tracking expenses")
	fmt.Println()

	return nil
}
