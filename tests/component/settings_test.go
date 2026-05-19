//go:build component

package component_test

import (
	"net/http"
	"testing"

	"github.com/ArionMiles/expensor/tests/component/helpers"
)

func TestSettingsRoundTripAndCheckpointClear(t *testing.T) {
	helpers.WaitForHealthy(t)
	client := helpers.NewClient(t)

	readCases := []struct {
		name string
		path string
		key  string
		want string
	}{
		{name: "base currency", path: "/api/config/base-currency", key: "base_currency", want: "INR"},
		{name: "scan interval", path: "/api/config/scan-interval", key: "scan_interval", want: "120"},
		{name: "timezone", path: "/api/config/timezone", key: "timezone", want: "Asia/Kolkata"},
	}

	for _, tc := range readCases {
		t.Run(tc.name, func(t *testing.T) {
			resp := client.Get(t, tc.path)
			helpers.RequireStatus(t, resp, http.StatusOK)
			body := helpers.DecodeJSON[map[string]string](t, resp)
			if body[tc.key] != tc.want {
				t.Fatalf("unexpected payload for %s: %#v", tc.name, body)
			}
		})
	}

	t.Run("update base currency", func(t *testing.T) {
		updateCurrency := client.JSON(t, http.MethodPut, "/api/config/base-currency", map[string]string{"base_currency": "INR"})
		helpers.RequireStatus(t, updateCurrency, http.StatusOK)
	})

	t.Run("checkpoint exists then clears", func(t *testing.T) {
		checkpoint := client.Get(t, "/api/config/readers/gmail/checkpoint")
		helpers.RequireStatus(t, checkpoint, http.StatusOK)
		checkpointBody := helpers.DecodeJSON[map[string]any](t, checkpoint)
		if checkpointBody["last_scan_at"] == nil {
			t.Fatalf("expected seeded checkpoint, got %#v", checkpointBody)
		}

		clearCheckpoint := client.JSON(t, http.MethodDelete, "/api/config/readers/gmail/checkpoint", nil)
		helpers.RequireStatus(t, clearCheckpoint, http.StatusNoContent)

		checkpointAfter := client.Get(t, "/api/config/readers/gmail/checkpoint")
		helpers.RequireStatus(t, checkpointAfter, http.StatusOK)
		checkpointAfterBody := helpers.DecodeJSON[map[string]any](t, checkpointAfter)
		if checkpointAfterBody["last_scan_at"] != nil {
			t.Fatalf("expected null checkpoint after clear, got %#v", checkpointAfterBody)
		}
	})
}
