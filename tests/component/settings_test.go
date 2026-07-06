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

	type preferences struct {
		BaseCurrency string `json:"base_currency"`
		ScanInterval int    `json:"scan_interval"`
		LookbackDays int    `json:"lookback_days"`
		Timezone     string `json:"timezone"`
		TimeFormat   string `json:"time_format"`
	}

	resp := client.Get(t, "/api/config/preferences")
	helpers.RequireStatus(t, resp, http.StatusOK)
	body := helpers.DecodeJSON[preferences](t, resp)
	if body.BaseCurrency != "INR" || body.ScanInterval != 120 || body.LookbackDays != 365 {
		t.Fatalf("unexpected preferences: %#v", body)
	}
	if body.Timezone != "Asia/Kolkata" || body.TimeFormat != "HH:mm" {
		t.Fatalf("unexpected display preferences: %#v", body)
	}

	update := client.JSON(t, http.MethodPatch, "/api/config/preferences", map[string]any{
		"scan_interval": 180,
		"time_format":   "h:mm a",
	})
	helpers.RequireStatus(t, update, http.StatusOK)
	updated := helpers.DecodeJSON[preferences](t, update)
	if updated.ScanInterval != 180 || updated.TimeFormat != "h:mm a" {
		t.Fatalf("unexpected updated preferences: %#v", updated)
	}

	t.Run("checkpoint exists then clears", func(t *testing.T) {
		checkpoint := client.Get(t, "/api/config/providers/gmail/checkpoint")
		helpers.RequireStatus(t, checkpoint, http.StatusOK)
		checkpointBody := helpers.DecodeJSON[map[string]any](t, checkpoint)
		if checkpointBody["last_scan_at"] == nil {
			t.Fatalf("expected seeded checkpoint, got %#v", checkpointBody)
		}

		clearCheckpoint := client.JSON(t, http.MethodDelete, "/api/config/providers/gmail/checkpoint", nil)
		helpers.RequireStatus(t, clearCheckpoint, http.StatusNoContent)

		checkpointAfter := client.Get(t, "/api/config/providers/gmail/checkpoint")
		helpers.RequireStatus(t, checkpointAfter, http.StatusOK)
		checkpointAfterBody := helpers.DecodeJSON[map[string]any](t, checkpointAfter)
		if checkpointAfterBody["last_scan_at"] != nil {
			t.Fatalf("expected null checkpoint after clear, got %#v", checkpointAfterBody)
		}
	})
}
