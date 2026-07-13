package main

import (
	"embed"
	"fmt"
	"log/slog"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
	"github.com/ArionMiles/expensor/backend/pkg/reader/gmail"
	"github.com/ArionMiles/expensor/backend/pkg/reader/thunderbird"
)

// registerPlugins assembles the application's concrete provider catalog.
func registerPlugins(registry *plugins.Registry, fs embed.FS, logger *slog.Logger) error {
	var gmailGuide []byte
	if data, err := fs.ReadFile("content/readers/gmail/guide.json"); err == nil {
		gmailGuide = data
	} else {
		logger.Warn("could not load gmail guide", "error", err)
	}
	var thunderbirdGuide []byte
	if data, err := fs.ReadFile("content/readers/thunderbird/guide.json"); err == nil {
		thunderbirdGuide = data
	} else {
		logger.Warn("could not load thunderbird guide", "error", err)
	}

	for _, provider := range []plugins.Provider{gmail.Provider(gmailGuide), thunderbird.Provider(thunderbirdGuide)} {
		if err := registry.RegisterProvider(provider); err != nil {
			return errors.E("server.register_plugins", errors.Internal, fmt.Sprintf("registering provider %s", provider.Metadata.Name), err)
		}
	}
	return nil
}
