package httpapi

import (
	"net/http"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
)

// ProviderInfo is the API representation of an email provider.
type ProviderInfo struct {
	Name                      string                `json:"name"`
	Description               string                `json:"description"`
	AuthType                  plugins.AuthType      `json:"auth_type"`
	RequiresCredentialsUpload bool                  `json:"requires_credentials_upload"`
	ConfigSchema              []plugins.ConfigField `json:"config_schema"`
}

// ListProviders handles GET /api/providers.
// @Summary List providers
// @Tags Providers
// @Produce json
// @Success 200 {array} ProviderInfoResponse
// @Router /providers [get]
func (h *Handlers) ListProviders(w http.ResponseWriter, _ *http.Request) {
	providers := h.registry.ListProviders()
	infos := make([]ProviderInfo, 0, len(providers))
	for _, provider := range providers {
		metadata := provider.Metadata
		configSchema := metadata.ConfigSchema
		if configSchema == nil {
			configSchema = []plugins.ConfigField{}
		}
		infos = append(infos, ProviderInfo{
			Name:                      metadata.Name,
			Description:               metadata.Description,
			AuthType:                  metadata.Auth.Type,
			RequiresCredentialsUpload: metadata.Auth.RequiresCredentialsUpload,
			ConfigSchema:              configSchema,
		})
	}
	writeJSON(w, http.StatusOK, infos)
}
