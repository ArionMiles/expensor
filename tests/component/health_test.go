//go:build component

package component_test

import (
	"testing"

	"github.com/ArionMiles/expensor/tests/component/helpers"
)

func TestHealthVersionAndActiveReader(t *testing.T) {
	helpers.WaitForHealthy(t)
	client := helpers.NewClient(t)

	cases := []struct {
		name   string
		path   string
		assert func(t *testing.T, body map[string]string)
	}{
		{
			name: "health ok",
			path: "/api/health",
			assert: func(t *testing.T, body map[string]string) {
				t.Helper()
				if body["status"] != "ok" {
					t.Fatalf("unexpected health payload: %#v", body)
				}
			},
		},
		{
			name: "version present",
			path: "/api/version",
			assert: func(t *testing.T, body map[string]string) {
				t.Helper()
				if body["version"] == "" {
					t.Fatalf("expected non-empty version payload, got %#v", body)
				}
			},
		},
		{
			name: "active reader persisted",
			path: "/api/config/active-reader",
			assert: func(t *testing.T, body map[string]string) {
				t.Helper()
				if body["reader"] != "thunderbird" {
					t.Fatalf("expected thunderbird active reader, got %#v", body)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := client.Get(t, tc.path)
			helpers.RequireStatus(t, resp, 200)
			body := helpers.DecodeJSON[map[string]string](t, resp)
			tc.assert(t, body)
		})
	}
}
