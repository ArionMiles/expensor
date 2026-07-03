//go:build component

package component_test

import (
	"net/http"
	"testing"

	"github.com/ArionMiles/expensor/tests/component/helpers"
)

func TestDaemonControlRequiresAuthAndPersistsActiveReader(t *testing.T) {
	helpers.WaitForHealthy(t)

	unauthenticated := helpers.NewUnauthenticatedClient(t)
	authenticated := helpers.NewClient(t)

	cases := []struct {
		name   string
		client *helpers.Client
		method string
		path   string
		body   any
		want   int
		assert func(t *testing.T, body map[string]string)
	}{
		{
			name:   "anonymous start rejected",
			client: unauthenticated,
			method: http.MethodPost,
			path:   "/api/daemon/start",
			body: map[string]string{
				"reader": "thunderbird",
			},
			want: http.StatusUnauthorized,
		},
		{
			name:   "authenticated start accepted",
			client: authenticated,
			method: http.MethodPost,
			path:   "/api/daemon/start",
			body: map[string]string{
				"reader": "gmail",
			},
			want: http.StatusAccepted,
			assert: func(t *testing.T, body map[string]string) {
				t.Helper()
				if body["status"] != "starting" {
					t.Fatalf("unexpected start response: %#v", body)
				}
			},
		},
		{
			name:   "active reader is tenant persisted",
			client: authenticated,
			method: http.MethodGet,
			path:   "/api/config/active-reader",
			want:   http.StatusOK,
			assert: func(t *testing.T, body map[string]string) {
				t.Helper()
				if body["reader"] != "gmail" {
					t.Fatalf("expected gmail active reader, got %#v", body)
				}
			},
		},
		{
			name:   "authenticated rescan accepted",
			client: authenticated,
			method: http.MethodPost,
			path:   "/api/daemon/rescan",
			body: map[string]string{
				"reader": "thunderbird",
			},
			want: http.StatusAccepted,
			assert: func(t *testing.T, body map[string]string) {
				t.Helper()
				if body["status"] != "rescanning" {
					t.Fatalf("unexpected rescan response: %#v", body)
				}
			},
		},
		{
			name:   "restore seeded active reader",
			client: authenticated,
			method: http.MethodPost,
			path:   "/api/daemon/start",
			body: map[string]string{
				"reader": "thunderbird",
			},
			want: http.StatusAccepted,
		},
		{
			name:   "active reader restored",
			client: authenticated,
			method: http.MethodGet,
			path:   "/api/config/active-reader",
			want:   http.StatusOK,
			assert: func(t *testing.T, body map[string]string) {
				t.Helper()
				if body["reader"] != "thunderbird" {
					t.Fatalf("expected restored thunderbird active reader, got %#v", body)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var resp *http.Response
			if tc.method == http.MethodGet {
				resp = tc.client.Get(t, tc.path)
			} else {
				resp = tc.client.JSON(t, tc.method, tc.path, tc.body)
			}
			helpers.RequireStatus(t, resp, tc.want)
			if tc.assert == nil {
				_ = resp.Body.Close()
				return
			}
			body := helpers.DecodeJSON[map[string]string](t, resp)
			tc.assert(t, body)
		})
	}
}
