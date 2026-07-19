//go:build component

package component_test

import (
	"encoding/json"
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

func TestLLMProviderRuntimeLifecycle(t *testing.T) {
	helpers.WaitForHealthy(t)
	client := helpers.NewClient(t)

	type modelOption struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		Quality     string `json:"quality"`
		Cost        string `json:"cost"`
	}
	type provider struct {
		Name           string         `json:"name"`
		DisplayName    string         `json:"display_name"`
		APIKeyURL      string         `json:"api_key_url"`
		APIKeyLinkText string         `json:"api_key_link_text"`
		DataUse        map[string]any `json:"data_use"`
		AuthType       string         `json:"auth_type"`
		Capabilities   []string       `json:"capabilities"`
		ConfigSchema   map[string]any `json:"config_schema"`
		ModelOptions   []modelOption  `json:"model_options"`
	}
	type status struct {
		Name              string          `json:"name"`
		Config            json.RawMessage `json:"config"`
		ConfigPresent     bool            `json:"config_present"`
		CredentialsStored bool            `json:"credentials_stored"`
		Active            bool            `json:"active"`
		Ready             bool            `json:"ready"`
	}

	providersResp := client.Get(t, "/api/llm/providers")
	helpers.RequireStatus(t, providersResp, http.StatusOK)
	providers := helpers.DecodeJSON[[]provider](t, providersResp)
	if len(providers) != 2 || providers[0].Name != "gemini" || providers[1].Name != "openai" {
		t.Fatalf("unexpected providers: %#v", providers)
	}
	gemini, openAI := providers[0], providers[1]
	if gemini.DisplayName != "Gemini" || gemini.APIKeyLinkText != "Google AI dashboard" ||
		gemini.DataUse["mode"] != "free_tier_improvement" || len(gemini.ModelOptions) == 0 {
		t.Fatalf("unexpected Gemini provider metadata: %#v", gemini)
	}
	if _, ok := gemini.ConfigSchema["properties"].(map[string]any)["base_url"]; ok {
		t.Fatalf("Gemini config schema unexpectedly exposes base_url: %#v", gemini.ConfigSchema)
	}
	if openAI.DisplayName != "OpenAI" || openAI.AuthType != "api_key" || openAI.APIKeyURL != "https://platform.openai.com/api-keys" ||
		openAI.APIKeyLinkText != "OpenAI dashboard" || openAI.DataUse["mode"] != "no_training_by_default" || len(openAI.ModelOptions) == 0 {
		t.Fatalf("unexpected OpenAI provider metadata: %#v", openAI)
	}

	cases := []struct {
		name              string
		configPresent     bool
		credentialsStored bool
		ready             bool
		assertConfig       func(t *testing.T, raw json.RawMessage)
	}{
		{
			name: "initially unconfigured",
			assertConfig: func(t *testing.T, raw json.RawMessage) {
				t.Helper()
				if string(raw) != "{}" {
					t.Fatalf("initial config = %s, want empty object", raw)
				}
			},
		},
		{
			name:              "configured with stored credentials but inactive",
			configPresent:     true,
			credentialsStored: true,
			assertConfig: func(t *testing.T, raw json.RawMessage) {
				t.Helper()
				var config map[string]string
				if err := json.Unmarshal(raw, &config); err != nil {
					t.Fatalf("decode config: %v", err)
				}
				if config["model"] != "gpt-5.4-mini" || config["base_url"] != "https://api.openai.com/v1" {
					t.Fatalf("unexpected config: %#v", config)
				}
			},
		},
		{
			name: "disconnected",
			assertConfig: func(t *testing.T, raw json.RawMessage) {
				t.Helper()
				if string(raw) != "{}" {
					t.Fatalf("disconnected config = %s, want empty object", raw)
				}
			},
		},
	}

	statusResp := client.Get(t, "/api/llm/providers/openai/status")
	helpers.RequireStatus(t, statusResp, http.StatusOK)
	assertLLMStatus(t, helpers.DecodeJSON[status](t, statusResp), cases[0])

	saveConfig := client.JSON(t, http.MethodPut, "/api/llm/providers/openai/config", map[string]any{
		"config": map[string]string{
			"model":    "gpt-5.4-mini",
			"base_url": "https://api.openai.com/v1",
		},
	})
	helpers.RequireStatus(t, saveConfig, http.StatusOK)

	saveCredentials := client.JSON(t, http.MethodPut, "/api/llm/providers/openai/credentials", map[string]string{
		"api_key": "  sk-component-test  ",
	})
	helpers.RequireStatus(t, saveCredentials, http.StatusOK)

	configuredResp := client.Get(t, "/api/llm/providers/openai/status")
	helpers.RequireStatus(t, configuredResp, http.StatusOK)
	assertLLMStatus(t, helpers.DecodeJSON[status](t, configuredResp), cases[1])

	disconnect := client.JSON(t, http.MethodDelete, "/api/llm/providers/openai", nil)
	helpers.RequireStatus(t, disconnect, http.StatusNoContent)

	disconnectedResp := client.Get(t, "/api/llm/providers/openai/status")
	helpers.RequireStatus(t, disconnectedResp, http.StatusOK)
	assertLLMStatus(t, helpers.DecodeJSON[status](t, disconnectedResp), cases[2])
}

func assertLLMStatus(t *testing.T, got struct {
	Name              string          `json:"name"`
	Config            json.RawMessage `json:"config"`
	ConfigPresent     bool            `json:"config_present"`
	CredentialsStored bool            `json:"credentials_stored"`
	Active            bool            `json:"active"`
	Ready             bool            `json:"ready"`
}, want struct {
	name              string
	configPresent     bool
	credentialsStored bool
	ready             bool
	assertConfig       func(t *testing.T, raw json.RawMessage)
}) {
	t.Helper()
	if got.Name != "openai" {
		t.Fatalf("%s status provider = %q, want openai", want.name, got.Name)
	}
	if got.ConfigPresent != want.configPresent || got.CredentialsStored != want.credentialsStored || got.Ready != want.ready || got.Active {
		t.Fatalf("%s status = %#v", want.name, got)
	}
	want.assertConfig(t, got.Config)
}
