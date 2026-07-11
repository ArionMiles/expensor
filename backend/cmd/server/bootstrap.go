package main

import (
	"fmt"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
	"github.com/ArionMiles/expensor/backend/pkg/reader/gmail"
	"github.com/ArionMiles/expensor/backend/pkg/reader/thunderbird"
)

// registerPlugins assembles the application's concrete provider catalog.
func registerPlugins(registry *plugins.Registry, guides map[string][]byte) error {
	for _, provider := range []plugins.Provider{gmail.Provider(guides["gmail"]), thunderbird.Provider(guides["thunderbird"])} {
		if err := registry.RegisterProvider(provider); err != nil {
			return errors.E("server.register_plugins", errors.Internal, fmt.Sprintf("registering provider %s", provider.Metadata.Name), err)
		}
	}
	return nil
}
