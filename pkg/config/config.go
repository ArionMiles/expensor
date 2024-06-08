package config

import "github.com/ArionMiles/expensor/pkg/api"

type Config struct {
	SecretsFilePath string `koanf:"secretsFilePath"`
	SheetTitle      string `koanf:"sheetTitle"`
	SheetID         string `koanf:"sheetId"`
	SheetName       string `koanf:"sheetName"`
	Rules           []api.Rule
}
