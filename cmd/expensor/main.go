package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ArionMiles/expensor/pkg/logging"
)

const usage = `expensor - Extract expense transactions from Gmail to Google Sheets

Usage:
  expensor <command> [options]

Commands:
  run      Start the expense tracking daemon (default)
  setup    Configure Gmail and Sheets integration
  status   Check authentication and configuration status

Options:
  -h, --help    Show this help message

Examples:
  expensor run              # Start tracking expenses
  expensor setup            # Set up OAuth authentication
  expensor status           # Check if everything is configured

For more information, visit: https://github.com/ArionMiles/expensor
`

func main() {
	// Setup logging early
	logger := logging.Setup(logging.DefaultConfig())

	// Handle no arguments - show usage
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(0)
	}

	// Parse command
	cmd := os.Args[1]

	// Handle help flags
	if cmd == "-h" || cmd == "--help" || cmd == "help" {
		fmt.Print(usage)
		os.Exit(0)
	}

	// Execute command
	var err error
	switch cmd {
	case "run":
		err = runCmd(logger, os.Args[2:])
	case "setup":
		err = setupCmd(logger, os.Args[2:])
	case "status":
		err = statusCmd(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		fmt.Print(usage)
		os.Exit(1)
	}

	if err != nil {
		logger.Error("command failed", "command", cmd, "error", err)
		os.Exit(1)
	}
}

// runCmd starts the expense tracking daemon
func runCmd(logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	return runExpensor(logger)
}

// setupCmd handles OAuth setup
func setupCmd(logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	force := fs.Bool("force", false, "Force re-authentication even if token exists")
	if err := fs.Parse(args); err != nil {
		return err
	}

	return runSetup(logger, *force)
}

// statusCmd checks configuration and authentication status
func statusCmd(args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	return runStatus()
}
