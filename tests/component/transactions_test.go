//go:build component

package component_test

import (
	"net/http"
	"testing"

	"github.com/ArionMiles/expensor/tests/component/helpers"
)

func TestTransactionsSeededFiltersAndMutations(t *testing.T) {
	helpers.WaitForHealthy(t)
	client := helpers.NewClient(t)

	readCases := []struct {
		name   string
		path   string
		assert func(t *testing.T, body map[string]any)
	}{
		{
			name: "category filter",
			path: "/api/transactions?page=1&page_size=10&category=Food%20%26%20Dining",
			assert: func(t *testing.T, body map[string]any) {
				t.Helper()
				if int(body["total"].(float64)) == 0 {
					t.Fatalf("expected visible Food & Dining transactions, got %#v", body)
				}
			},
		},
		{
			name: "facets available",
			path: "/api/transactions/facets",
			assert: func(t *testing.T, body map[string]any) {
				t.Helper()
				if len(body["categories"].([]any)) == 0 {
					t.Fatalf("expected seeded categories in facets, got %#v", body)
				}
			},
		},
	}

	for _, tc := range readCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := client.Get(t, tc.path)
			helpers.RequireStatus(t, resp, http.StatusOK)
			body := helpers.DecodeJSON[map[string]any](t, resp)
			tc.assert(t, body)
		})
	}

	mutationCases := []struct {
		name   string
		method string
		path   string
		body   any
		want   int
	}{
		{
			name:   "update transaction",
			method: http.MethodPatch,
			path:   "/api/transactions/00000000-0000-0000-0000-000000000001",
			body: map[string]string{
				"description": "Updated seeded purchase",
				"category":    "Food & Dining",
				"bucket":      "Needs",
			},
			want: http.StatusOK,
		},
		{
			name:   "add label",
			method: http.MethodPost,
			path:   "/api/transactions/00000000-0000-0000-0000-000000000001/labels",
			body: map[string]any{
				"labels": []string{"Office"},
			},
			want: http.StatusOK,
		},
		{
			name:   "mute transaction",
			method: http.MethodPatch,
			path:   "/api/transactions/00000000-0000-0000-0000-000000000001",
			body: map[string]any{
				"muted":      true,
				"mute_reason": "component test mute",
			},
			want: http.StatusOK,
		},
		{
			name:   "update mute reason",
			method: http.MethodPatch,
			path:   "/api/transactions/00000000-0000-0000-0000-000000000001",
			body: map[string]string{
				"mute_reason": "component test reason",
			},
			want: http.StatusOK,
		},
	}

	for _, tc := range mutationCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := client.JSON(t, tc.method, tc.path, tc.body)
			helpers.RequireStatus(t, resp, tc.want)
		})
	}
}
