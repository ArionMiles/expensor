package httpapi

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
	"github.com/ArionMiles/expensor/backend/pkg/reader/thunderbird"
)

// DiscoverProfiles handles GET /api/providers/thunderbird/discover/profiles.
// Returns discovered Thunderbird profile directories from platform paths,
// the Docker mount /thunderbird-profile, and THUNDERBIRD_DATA_DIR env var.
// @Summary Discover Thunderbird profiles
// @Tags Providers
// @Produce json
// @Success 200 {object} ThunderbirdProfilesResponse
// @Router /providers/thunderbird/discover/profiles [get]
func (h *Handlers) DiscoverProfiles(w http.ResponseWriter, _ *http.Request) {
	var paths []string
	seen := make(map[string]struct{})

	addIfExists := func(p string) {
		if p == "" {
			return
		}
		if _, err := os.Stat(p); err == nil {
			if _, exists := seen[p]; !exists {
				seen[p] = struct{}{}
				paths = append(paths, p)
			}
		}
	}

	if discovered, err := thunderbird.FindProfiles(); err == nil {
		for _, p := range discovered {
			addIfExists(p)
		}
	}
	addIfExists("/thunderbird-profile")
	addIfExists(h.thunderbirdDataDir)

	if paths == nil {
		paths = []string{}
	}
	writeJSON(w, http.StatusOK, map[string][]string{"profiles": paths})
}

// DiscoverMailboxes handles GET /api/providers/thunderbird/discover/mailboxes?profile=<path>.
// Returns available MBOX mailbox names within the given Thunderbird profile directory.
// @Summary Discover Thunderbird mailboxes
// @Tags Providers
// @Produce json
// @Param profile query string true "Thunderbird profile path"
// @Success 200 {object} ThunderbirdMailboxesResponse
// @Failure 422 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /providers/thunderbird/discover/mailboxes [get]
func (h *Handlers) DiscoverMailboxes(w http.ResponseWriter, r *http.Request) {
	query, ok := decodeAndValidateQuery[mailboxDiscoveryQuery](h, w, r)
	if !ok {
		return
	}
	profile := query.Profile
	if _, err := os.Stat(profile); err != nil {
		if os.IsNotExist(err) {
			writeError(w, r, errors.E(errors.NotFound, errors.User("profile directory not found")))
		} else {
			writeError(w, r, err)
		}
		return
	}
	mailboxes, err := thunderbird.ListMailboxes(profile)
	if err != nil {
		writeError(w, r, err)
		return
	}
	if mailboxes == nil {
		mailboxes = []string{}
	}
	writeJSON(w, http.StatusOK, map[string][]string{"mailboxes": mailboxes})
}

// GetProviderGuide handles GET /api/providers/{name}/guide.
// Returns the structured setup guide for a provider when metadata includes one.
// @Summary Get provider setup guide
// @Tags Providers
// @Produce json
// @Param name path string true "Provider name" example(thunderbird)
// @Success 200 {object} ProviderGuideResponse
// @Failure 404 {object} ErrorResponse
// @Router /providers/{name}/guide [get]
func (h *Handlers) GetProviderGuide(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	provider, err := h.registry.GetProvider(name)
	if err != nil {
		writeError(w, r, err)
		return
	}
	guideData := provider.Metadata.SetupGuide
	if len(guideData) == 0 {
		writeError(w, r, errors.E(errors.NotFound, errors.User("no setup guide available for this provider")))
		return
	}
	var guide plugins.ProviderGuide
	if err := json.Unmarshal(guideData, &guide); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, guide)
}
