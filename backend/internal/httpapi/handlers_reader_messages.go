package httpapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ArionMiles/expensor/backend/internal/oauth"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/config"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const defaultProviderMessageSearchLimit = 10

// SearchProviderMessages handles GET /api/providers/{name}/messages.
// @Summary Search provider messages for rule samples
// @Tags Providers
// @Produce json
// @Param name path string true "Provider name" example(gmail)
// @Param subject query string true "Subject substring"
// @Param limit query int false "Maximum messages to return" minimum(1) maximum(50)
// @Success 200 {object} ProviderSearchResponse
// @Failure 404 {object} ErrorResponse
// @Failure 412 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /providers/{name}/messages [get]
func (h *Handlers) SearchProviderMessages(w http.ResponseWriter, r *http.Request) {
	query, ok := decodeAndValidateQuery[providerMessagesQuery](h, w, r)
	if !ok {
		return
	}
	if query.Limit == 0 {
		query.Limit = defaultProviderMessageSearchLimit
	}

	name := r.PathValue("name")
	searcher, err := h.newEmailSearcher(r.Context(), requestTenant(r), name)
	if err != nil {
		writeError(w, r, err)
		return
	}

	results, err := searcher.Search(r.Context(), api.EmailSearchQuery{
		SubjectQuery: query.Subject,
		Limit:        query.Limit,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ProviderSearchResponse{Results: providerSearchResultsToHTTP(results)})
}

func (h *Handlers) newEmailSearcher(ctx context.Context, tenant store.Tenant, name string) (api.EmailSearcher, error) {
	const op = "httpapi.Handlers.newEmailSearcher"

	provider, err := h.registry.GetProvider(name)
	if err != nil {
		return nil, errors.E(op, errors.NotFound, fmt.Sprintf("reader %q is no longer registered", name), err)
	}

	var httpClient *http.Client
	meta := provider.Metadata
	if meta.Auth.Type == plugins.AuthTypeOAuth {
		secretJSON, ok, err := h.readerRuntimeStore.GetReaderSecret(ctx, tenant, name)
		if err != nil {
			return nil, errors.E("httpapi.handlers_readers.new_email_searcher", fmt.Sprintf("loading credentials for provider %q", name), err)
		}
		if !ok {
			return nil, errors.E(
				op,
				oauth.KindCredentialsMissing,
				errors.User("provider is not authenticated"),
				"credentials file missing",
			)
		}
		httpClient, err = oauth.NewFromJSONAndStore(ctx, oauth.StoreClientInput{
			SecretJSON: secretJSON,
			Store:      h.readerRuntimeStore,
			Tenant:     tenant,
			Reader:     name,
			Scopes:     meta.Auth.RequiredScopes,
		})
		if err != nil {
			return nil, err
		}
	}

	readerConfig, _, err := h.readerRuntimeStore.GetReaderConfig(ctx, tenant, name)
	if err != nil {
		return nil, errors.E("httpapi.handlers_readers.new_email_searcher", fmt.Sprintf("loading provider config for %q", name), err)
	}
	return provider.NewEmailSearcher(plugins.ProviderInput{
		HTTPClient:   httpClient,
		AppConfig:    &config.App{ScanInterval: h.scanInterval, LookbackDays: h.lookbackDays},
		ReaderConfig: readerConfig,
		Logger:       h.logger,
	})
}

func providerSearchResultsToHTTP(results []api.EmailSearchResult) []ProviderSearchResultResponse {
	out := make([]ProviderSearchResultResponse, 0, len(results))
	for _, result := range results {
		out = append(out, ProviderSearchResultResponse{
			ID:          result.ID,
			SenderEmail: result.SenderEmail,
			Subject:     result.Subject,
			Body:        result.Body,
			ReceivedAt:  result.ReceivedAt,
		})
	}
	return out
}
