package config

import "github.com/ArionMiles/expensor/pkg/api"

// ClientSecretFile is the default path to the Google OAuth credentials JSON file.
const ClientSecretFile = "data/client_secret.json"

// Config holds the application configuration loaded from environment variables.
type Config struct {
	// GSheetsTitle is the title for a new Google Sheet (used when creating).
	// Environment variable: GSHEETS_TITLE
	GSheetsTitle string `koanf:"GSHEETS_TITLE"`

	// GSheetsID is the ID of an existing Google Sheet to use.
	// Environment variable: GSHEETS_ID
	GSheetsID string `koanf:"GSHEETS_ID"`

	// GSheetsName is the name of the sheet/tab within the spreadsheet.
	// Environment variable: GSHEETS_NAME
	GSheetsName string `koanf:"GSHEETS_NAME"`

	// Rules and Labels are populated from embedded config, not environment variables.
	Rules  []api.Rule
	Labels api.Labels
}
