//go:build component

package component_test

import (
	"net/http"
	"testing"

	"github.com/ArionMiles/expensor/tests/component/helpers"
)

func TestTaxonomyListAndLabelMutationFlow(t *testing.T) {
	helpers.WaitForHealthy(t)
	client := helpers.NewClient(t)

	listCases := []struct {
		name string
		path string
	}{
		{name: "labels list", path: "/api/config/labels"},
		{name: "categories list", path: "/api/config/categories"},
		{name: "buckets list", path: "/api/config/buckets"},
	}

	for _, tc := range listCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := client.Get(t, tc.path)
			helpers.RequireStatus(t, resp, http.StatusOK)
			rows := helpers.DecodeJSON[[]map[string]any](t, resp)
			if len(rows) == 0 {
				t.Fatalf("expected seeded rows for %s", tc.name)
			}
		})
	}

	t.Run("create and apply label", func(t *testing.T) {
		createLabel := client.JSON(t, http.MethodPost, "/api/config/labels", map[string]string{
			"name":  "ComponentLabel",
			"color": "#22c55e",
		})
		helpers.RequireStatus(t, createLabel, http.StatusCreated)

		applyLabel := client.JSON(t, http.MethodPost, "/api/config/labels/ComponentLabel/apply", map[string]string{
			"merchant_pattern": "Swiggy",
		})
		helpers.RequireStatus(t, applyLabel, http.StatusOK)
		applyBody := helpers.DecodeJSON[map[string]any](t, applyLabel)
		if applyBody["applied"] == nil {
			t.Fatalf("expected applied count, got %#v", applyBody)
		}
	})
}
