//go:build component

package component_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/ArionMiles/expensor/tests/component/helpers"
)

func TestRuleDraftRequests(t *testing.T) {
	helpers.WaitForHealthy(t)
	client := helpers.NewClient(t)

	cases := []struct {
		name           string
		body           map[string]any
		expectedStatus int
		assertBody     func(t *testing.T, body map[string]any)
	}{
		{
			name: "valid request requires configured LLM provider",
			body: map[string]any{
				"samples": []map[string]any{{
					"name":    "Sample 1",
					"sender":  "alerts@example.com",
					"subject": "Card alert",
					"body":    "INR 10 at Cafe",
					"expected": map[string]string{
						"amount":   "10",
						"merchant": "Cafe",
						"currency": "INR",
					},
				}},
			},
			expectedStatus: http.StatusConflict,
			assertBody: func(t *testing.T, body map[string]any) {
				t.Helper()
				if !strings.Contains(body["error"].(string), "Configure an LLM provider") {
					t.Fatalf("unexpected error body: %#v", body)
				}
			},
		},
		{
			name: "sample expectations are validated before provider access",
			body: map[string]any{
				"samples": []map[string]any{{
					"name": "Sample 1",
					"body": "INR 10 at Cafe",
					"expected": map[string]string{
						"amount":   "10",
						"merchant": "",
					},
				}},
			},
			expectedStatus: http.StatusUnprocessableEntity,
			assertBody: func(t *testing.T, body map[string]any) {
				t.Helper()
				details, ok := body["details"].([]any)
				if !ok || len(details) == 0 {
					t.Fatalf("expected validation details, got %#v", body)
				}
				first, ok := details[0].(map[string]any)
				if !ok || first["field"] != "samples.expected" || first["location"] != "body" {
					t.Fatalf("unexpected validation details: %#v", details)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := client.JSON(t, http.MethodPost, "/api/rule-drafts", tc.body)
			helpers.RequireStatus(t, resp, tc.expectedStatus)
			tc.assertBody(t, helpers.DecodeJSON[map[string]any](t, resp))
		})
	}
}
