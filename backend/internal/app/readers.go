package app

import (
	"fmt"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
	"github.com/ArionMiles/expensor/backend/pkg/reader/gmail"
	"github.com/ArionMiles/expensor/backend/pkg/reader/thunderbird"
)

func registerReaders(registry *plugins.Registry, guides map[string][]byte) error {
	providers := []plugins.Provider{
		gmail.Provider(guides["gmail"]),
		thunderbird.Provider(guides["thunderbird"]),
	}
	for _, provider := range providers {
		if err := registry.RegisterProvider(provider); err != nil {
			return errors.E("app.readers.register", errors.Internal, fmt.Sprintf("registering provider %s", provider.Metadata.Name), err)
		}
	}
	return nil
}
