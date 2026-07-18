package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestGetPreferencesCombinesStoredValuesAndDefaults(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/preferences", nil)
	rr := httptest.NewRecorder()
	h.GetPreferences(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp PreferencesResponse
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.BaseCurrency != "INR" || resp.ScanInterval != 60 || resp.LookbackDays != 180 {
		t.Fatalf("unexpected configured defaults: %#v", resp)
	}
	if resp.Timezone != "" || resp.TimeFormat != "HH:mm" {
		t.Fatalf("unexpected display defaults: %#v", resp)
	}
}

func TestGetPreferencesUsesStoredValues(t *testing.T) {
	ms := &mockStore{appConfig: map[string]string{
		"base_currency":   "USD",
		"scan_interval":   "120",
		"lookback_days":   "365",
		"app.timezone":    "Asia/Kolkata",
		"app.time_format": "h:mm a",
	}}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/preferences", nil)
	rr := httptest.NewRecorder()
	h.GetPreferences(rr, req)

	var resp PreferencesResponse
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.BaseCurrency != "USD" || resp.ScanInterval != 120 || resp.LookbackDays != 365 {
		t.Fatalf("unexpected stored numeric preferences: %#v", resp)
	}
	if resp.Timezone != "Asia/Kolkata" || resp.TimeFormat != "h:mm a" {
		t.Fatalf("unexpected stored display preferences: %#v", resp)
	}
}

func TestGetSetupStatusRequiresMissingPreferences(t *testing.T) {
	h := newTestHandlers(t, &mockStore{appConfig: map[string]string{"scan_interval": "60"}}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/setup-status", nil)
	rr := httptest.NewRecorder()
	h.GetSetupStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Required bool     `json:"required"`
		Missing  []string `json:"missing"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Required {
		t.Fatalf("required = false, want true")
	}
	want := []string{"base_currency", "timezone", "time_format"}
	if !reflect.DeepEqual(resp.Missing, want) {
		t.Fatalf("missing = %#v, want %#v", resp.Missing, want)
	}
}

func TestGetSetupStatusCompleteWhenPreferencesExist(t *testing.T) {
	h := newTestHandlers(t, &mockStore{appConfig: map[string]string{
		"base_currency":   "USD",
		"app.timezone":    "America/New_York",
		"app.time_format": "h:mm a",
	}}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/setup-status", nil)
	rr := httptest.NewRecorder()
	h.GetSetupStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Required bool     `json:"required"`
		Missing  []string `json:"missing"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Required {
		t.Fatalf("required = true, want false")
	}
	if len(resp.Missing) != 0 {
		t.Fatalf("missing = %#v, want empty", resp.Missing)
	}
}

func TestPatchPreferencesUpdatesSuppliedFields(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})

	body := strings.NewReader(
		`{"base_currency":"usd","scan_interval":120,"lookback_days":365,"timezone":"Asia/Kolkata","time_format":"h:mm a"}`,
	)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/api/config/preferences", body)
	rr := httptest.NewRecorder()
	h.PatchPreferences(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp PreferencesResponse
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.BaseCurrency != "USD" || resp.ScanInterval != 120 || resp.LookbackDays != 365 {
		t.Fatalf("unexpected response: %#v", resp)
	}
	want := map[string]string{
		"base_currency":   "USD",
		"scan_interval":   "120",
		"lookback_days":   "365",
		"app.timezone":    "Asia/Kolkata",
		"app.time_format": "h:mm a",
	}
	if !reflect.DeepEqual(ms.appConfig, want) {
		t.Fatalf("stored preferences = %#v, want %#v", ms.appConfig, want)
	}
}

func TestPatchPreferencesRejectsInvalidFieldsBeforeWriting(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		field   string
		message string
	}{
		{
			name:    "currency",
			body:    `{"base_currency":"US1"}`,
			field:   "base_currency",
			message: "must be a 3-letter ISO 4217 code",
		},
		{
			name:    "scan interval",
			body:    `{"base_currency":"USD","scan_interval":5}`,
			field:   "scan_interval",
			message: "must be at least 10",
		},
		{
			name:    "lookback days",
			body:    `{"lookback_days":3651}`,
			field:   "lookback_days",
			message: "must be at most 3650",
		},
		{
			name:    "timezone",
			body:    `{"timezone":"Mars/Olympus"}`,
			field:   "timezone",
			message: "must be a valid IANA timezone",
		},
		{
			name:    "time format",
			body:    `{"time_format":"24h"}`,
			field:   "time_format",
			message: "must be one of: HH:mm, HH:mm:ss, h:mm a, h:mm:ss a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &mockStore{}
			h := newTestHandlers(t, ms, &mockDaemon{})
			req := httptest.NewRequestWithContext(
				context.Background(),
				http.MethodPatch,
				"/api/config/preferences",
				strings.NewReader(tt.body),
			)
			rr := httptest.NewRecorder()
			h.PatchPreferences(rr, req)

			assertValidationError(t, rr, tt.field, "body", tt.message)
			if len(ms.appConfig) != 0 {
				t.Fatalf("invalid patch persisted values: %#v", ms.appConfig)
			}
		})
	}
}

func TestPatchPreferencesRejectsInvalidJSON(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/config/preferences",
		strings.NewReader("not-json"),
	)
	rr := httptest.NewRecorder()
	h.PatchPreferences(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if len(ms.appConfig) != 0 {
		t.Fatalf("invalid patch persisted values: %#v", ms.appConfig)
	}
}

func TestGetReaderCheckpoint_EmptyValueReturnsNull(t *testing.T) {
	ms := &mockStore{appConfig: map[string]string{"reader.gmail.last_scan_at": ""}}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/providers/gmail/checkpoint", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.GetReaderCheckpoint(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if val, ok := resp["last_scan_at"]; !ok || val != nil {
		t.Fatalf("expected last_scan_at to be null, got %#v", resp["last_scan_at"])
	}
}

func TestListBanks(t *testing.T) {
	banksJSON := []byte(`[{"fragment":"hdfc","color":"#E31837","name":"HDFC Bank"}]`)

	tests := []struct {
		name       string
		banksData  []byte
		wantStatus int
		wantBody   string
	}{
		{
			name:       "returns banks JSON when populated",
			banksData:  banksJSON,
			wantStatus: http.StatusOK,
			wantBody:   string(banksJSON),
		},
		{
			name:       "returns empty array when no banks data",
			banksData:  nil,
			wantStatus: http.StatusOK,
			wantBody:   "[]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandlers(t, &mockStore{}, &mockDaemon{}, tc.banksData)
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/banks", nil)
			w := httptest.NewRecorder()
			h.ListBanks(w, req)
			if w.Code != tc.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tc.wantStatus)
			}
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("got Content-Type %q, want application/json", ct)
			}
			if got := strings.TrimSpace(w.Body.String()); got != tc.wantBody {
				t.Errorf("got body %q, want %q", got, tc.wantBody)
			}
		})
	}
}

func TestGetCommunitySyncSettingsDefaultsAutomaticSyncEnabled(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/config/sync/settings", nil)
	rr := httptest.NewRecorder()
	h.GetCommunitySyncSettings(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	var resp CommunitySyncSettingsResponse
	decodeJSON(t, rr.Body.String(), &resp)
	if !resp.AutomaticSyncEnabled {
		t.Fatalf("automatic_sync_enabled = false, want true")
	}
}

func TestPatchCommunitySyncSettingsPersistsAutomaticSyncEnabled(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPatch,
		"/api/config/sync/settings",
		strings.NewReader(`{"automatic_sync_enabled":false}`),
	)
	rr := httptest.NewRecorder()
	h.PatchCommunitySyncSettings(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rr.Code, rr.Body.String())
	}
	if ms.communitySyncSettingsPatch.AutomaticSyncEnabled == nil || *ms.communitySyncSettingsPatch.AutomaticSyncEnabled {
		t.Fatalf("stored patch = %#v, want automatic_sync_enabled=false", ms.communitySyncSettingsPatch)
	}
	var resp CommunitySyncSettingsResponse
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.AutomaticSyncEnabled {
		t.Fatalf("automatic_sync_enabled = true, want false")
	}
}
