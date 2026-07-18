package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/ArionMiles/expensor/backend/internal/auth"
	"github.com/ArionMiles/expensor/backend/internal/daemon"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
	"github.com/ArionMiles/expensor/backend/internal/store"
)

func TestCredentialsStatus_Missing(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/gmail/credentials/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.CredentialsStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]bool
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["exists"] {
		t.Error("expected exists=false")
	}
}

func TestCredentialsStatus_Present(t *testing.T) {
	ms := &mockStore{
		readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(`{"installed":{}}`)},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/gmail/credentials/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.CredentialsStatus(rr, req)

	var resp map[string]bool
	decodeJSON(t, rr.Body.String(), &resp)
	if !resp["exists"] {
		t.Error("expected exists=true")
	}
}

func TestCredentialsStatus_UnknownReader(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/providers/noexist/credentials/status", nil)
	req.SetPathValue("name", "noexist")
	rr := httptest.NewRecorder()
	h.CredentialsStatus(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestUploadCredentials_SavesToStore(t *testing.T) {
	ms := &mockStore{}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/providers/gmail/credentials", strings.NewReader(`{"installed":{}}`))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.UploadCredentials(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["path"] != "db://reader_runtime/gmail/client_secret" {
		t.Fatalf("path = %q", resp["path"])
	}
	if string(ms.readerSecrets["tenant-a/gmail"]) != `{"installed":{}}` {
		t.Fatalf("secret was not persisted to store: %s", ms.readerSecrets["tenant-a/gmail"])
	}
}

func TestAuthStart_UsesMetadataScopes(t *testing.T) {
	const expectedScope = "https://www.googleapis.com/auth/gmail.readonly"
	secretJSON := `{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-client-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": "https://oauth2.googleapis.com/token",
			"redirect_uris": ["http://localhost:8080/api/auth/callback"]
		}
	}`
	st := &mockStore{readerSecrets: map[string][]byte{"tenant-a/scoped": []byte(secretJSON)}}
	h := newTestHandlers(t, st, &mockDaemon{})
	registry := plugins.NewRegistry()
	if err := registry.RegisterProvider((&testProvider{
		name:          "scoped",
		authType:      plugins.AuthTypeOAuth,
		requiresCreds: true,
		scopes:        []string{expectedScope},
	}).provider()); err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	h.registry = registry

	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/providers/scoped/auth/start", nil)
	req.SetPathValue("name", "scoped")
	rr := httptest.NewRecorder()

	h.AuthStart(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rr.Body.String(), &resp)
	authURL, err := url.Parse(resp["url"])
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	if got := authURL.Query().Get("scope"); got != expectedScope {
		t.Fatalf("scope = %q, want %q (url: %s)", got, expectedScope, resp["url"])
	}
}

func TestAuthStatus_NoToken(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/gmail/auth/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["authenticated"] != false {
		t.Errorf("expected authenticated=false")
	}
}

func TestAuthStatus_ConfigReader(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/providers/thunderbird/auth/status", nil)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()
	h.AuthStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["authenticated"] != true {
		t.Errorf("config-only reader should always be authenticated, got %v", resp["authenticated"])
	}
}

func TestAuthStatus_UsesStoreToken(t *testing.T) {
	ms := &mockStore{
		readerTokens: map[string][]byte{
			"tenant-a/gmail": []byte(`{"access_token":"a","token_type":"Bearer","expiry":"2999-01-01T00:00:00Z"}`),
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/gmail/auth/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.AuthStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["authenticated"] != true {
		t.Fatalf("expected authenticated=true, got %v", resp)
	}
	if resp["auth_state"] != "connected" {
		t.Fatalf("expected auth_state=connected, got %v", resp)
	}
}

func TestAuthStatus_RefreshesExpiredAccessTokenWithRefreshToken(t *testing.T) {
	tokenClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			return nil, fmt.Errorf("token endpoint method = %s, want POST", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			return nil, fmt.Errorf("ParseForm: %w", err)
		}
		if got := r.Form.Get("grant_type"); got != "refresh_token" {
			return nil, fmt.Errorf("grant_type = %q, want refresh_token", got)
		}
		if got := r.Form.Get("refresh_token"); got != "old-refresh" {
			return nil, fmt.Errorf("refresh_token = %q, want old-refresh", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-access","token_type":"Bearer","expires_in":3600}`)),
			Request:    r,
		}, nil
	})}

	secretJSON := fmt.Sprintf(`{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-client-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": %q,
			"redirect_uris": ["http://localhost:8080/api/auth/callback"]
		}
	}`, "https://oauth.test/token")
	ms := &mockStore{
		readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(secretJSON)},
		readerTokens: map[string][]byte{
			"tenant-a/gmail": []byte(`{"access_token":"old-access","refresh_token":"old-refresh","token_type":"Bearer","expiry":"2000-01-01T00:00:00Z"}`),
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.WithValue(context.Background(), oauth2.HTTPClient, tokenClient), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/gmail/auth/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.AuthStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["authenticated"] != true {
		t.Fatalf("expected authenticated=true, got %v", resp)
	}
	if resp["auth_state"] != "connected" {
		t.Fatalf("expected auth_state=connected, got %v", resp)
	}
	if !strings.Contains(string(ms.readerTokens["tenant-a/gmail"]), "new-access") {
		t.Fatalf("saved token = %s, want refreshed access token", ms.readerTokens["tenant-a/gmail"])
	}
	if !strings.Contains(string(ms.readerTokens["tenant-a/gmail"]), "old-refresh") {
		t.Fatalf("saved token = %s, want refresh token preserved", ms.readerTokens["tenant-a/gmail"])
	}
}

func TestAuthStatus_ExpiredAccessTokenWithoutRefreshTokenRequiresAuth(t *testing.T) {
	ms := &mockStore{
		readerTokens: map[string][]byte{
			"tenant-a/gmail": []byte(`{"access_token":"old-access","token_type":"Bearer","expiry":"2000-01-01T00:00:00Z"}`),
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/gmail/auth/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.AuthStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["authenticated"] != false {
		t.Fatalf("expected authenticated=false, got %v", resp)
	}
	if resp["auth_state"] != "reauthorization_required" {
		t.Fatalf("expected auth_state=reauthorization_required, got %v", resp)
	}
}

func TestAuthStatus_InvalidRefreshTokenRequiresAuth(t *testing.T) {
	tokenClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`)),
			Request:    r,
		}, nil
	})}

	secretJSON := fmt.Sprintf(`{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-client-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": %q,
			"redirect_uris": ["http://localhost:8080/api/auth/callback"]
		}
	}`, "https://oauth.test/token")
	ms := &mockStore{
		readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(secretJSON)},
		readerTokens: map[string][]byte{
			"tenant-a/gmail": []byte(`{"access_token":"old-access","refresh_token":"old-refresh","token_type":"Bearer","expiry":"2000-01-01T00:00:00Z"}`),
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.WithValue(context.Background(), oauth2.HTTPClient, tokenClient), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/providers/gmail/auth/status", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.AuthStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	decodeJSON(t, rr.Body.String(), &resp)
	if resp["authenticated"] != false {
		t.Fatalf("expected authenticated=false, got %v", resp)
	}
	if resp["auth_state"] != "reauthorization_required" {
		t.Fatalf("expected auth_state=reauthorization_required, got %v", resp)
	}
}

func TestRevokeToken_DeletesStoreToken(t *testing.T) {
	ms := &mockStore{readerTokens: map[string][]byte{"tenant-a/gmail": []byte(`{"access_token":"a"}`)}}
	h := newTestHandlers(t, ms, &mockDaemon{})
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/providers/gmail/auth/token", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.RevokeToken(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if _, ok := ms.readerTokens["tenant-a/gmail"]; ok {
		t.Fatal("token was not deleted from store")
	}
}

func TestDisconnectReader_StopsDaemonWhenActiveReaderIsRemoved(t *testing.T) {
	ms := &mockStore{
		scanningState: store.TenantScanningState{TenantID: "tenant-a", ActiveReader: "gmail", Enabled: true, State: store.ScanningStateRunning},
		readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(`{"installed":{}}`)},
		readerTokens:  map[string][]byte{"tenant-a/gmail": []byte(`{"access_token":"a"}`)},
	}
	var stopCalls int
	h := newTestHandlers(t, ms, &mockDaemon{status: DaemonStatus{Running: true}})
	h.daemon.(*mockDaemon).stopFn = func() { stopCalls++ }
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/providers/gmail", nil)
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()

	h.DisconnectReader(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if stopCalls != 1 {
		t.Fatalf("stop calls = %d, want 1", stopCalls)
	}
	if ms.scanningState.ActiveReader != "" {
		t.Fatalf("active scanning reader = %q, want cleared", ms.scanningState.ActiveReader)
	}
}

func TestDisconnectReader_DoesNotStopDaemonWhenInactiveReaderIsRemoved(t *testing.T) {
	ms := &mockStore{
		scanningState: store.TenantScanningState{TenantID: "tenant-a", ActiveReader: "gmail", Enabled: true, State: store.ScanningStateRunning},
		readerConfigs: map[string]json.RawMessage{"tenant-a/thunderbird": json.RawMessage(`{"mailbox":"Inbox"}`)},
		readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(`{"installed":{}}`)},
		readerTokens:  map[string][]byte{"tenant-a/gmail": []byte(`{"access_token":"a"}`)},
	}
	var stopCalls int
	h := newTestHandlers(t, ms, &mockDaemon{status: DaemonStatus{Running: true}})
	h.daemon.(*mockDaemon).stopFn = func() { stopCalls++ }
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/providers/thunderbird", nil)
	req.SetPathValue("name", "thunderbird")
	rr := httptest.NewRecorder()

	h.DisconnectReader(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	if stopCalls != 0 {
		t.Fatalf("stop calls = %d, want 0", stopCalls)
	}
	if ms.scanningState.ActiveReader != "gmail" {
		t.Fatalf("active scanning reader = %q, want gmail", ms.scanningState.ActiveReader)
	}
}

func TestGenerateState_IsUnique(t *testing.T) {
	s1, err := generateState("gmail")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s2, err := generateState("gmail")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s1 == s2 {
		t.Error("generateState must return unique values on each call")
	}
}

func TestGenerateState_IsNotPureTimestamp(t *testing.T) {
	s, err := generateState("gmail")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify the suffix after the last colon is a 32-char hex string.
	parts := strings.Split(s, ":")
	suffix := parts[len(parts)-1]
	if len(suffix) != 32 {
		t.Errorf("expected 32-char hex suffix, got %q (len=%d)", suffix, len(suffix))
	}
	for _, c := range suffix {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("suffix %q contains non-hex character %q", suffix, c)
		}
	}
}

func TestGenerateState_ContainsReaderName(t *testing.T) {
	s, err := generateState("thunderbird")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(s, "thunderbird") {
		t.Errorf("state %q should contain the reader name", s)
	}
}

func TestAuthCallback_RejectsUnknownState(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/auth/callback?state=doesnotexist&code=xyz", nil)
	rr := httptest.NewRecorder()
	h.AuthCallback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown state, got %d", rr.Code)
	}
}

func TestAuthCallback_RejectsExpiredState(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	// Inject an already-expired entry directly into the map.
	expiredState := "reader:gmail:expiredtoken"
	h.mu.Lock()
	h.oauthStates[expiredState] = oauthStateEntry{
		readerName: "gmail",
		expiresAt:  time.Now().Add(-1 * time.Second), // already expired
	}
	h.mu.Unlock()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/auth/callback?state="+expiredState+"&code=xyz", nil)
	rr := httptest.NewRecorder()
	h.AuthCallback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for expired state, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestAuthCallback_ReturnsClosePageAfterTokenSaved(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("token endpoint method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenServer.Close()

	secretJSON := fmt.Sprintf(`{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-client-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": %q,
			"redirect_uris": ["http://localhost:8080/api/auth/callback"]
		}
	}`, tokenServer.URL)
	st := &mockStore{readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(secretJSON)}}
	h := newTestHandlers(t, st, &mockDaemon{})

	state := "reader:gmail:validtoken"
	h.mu.Lock()
	h.oauthStates[state] = oauthStateEntry{
		readerName: "gmail",
		tenant:     store.Tenant{ID: "tenant-a"},
		expiresAt:  time.Now().Add(time.Minute),
	}
	h.mu.Unlock()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/auth/callback?state="+state+"&code=4%2F0Acode", nil)
	rr := httptest.NewRecorder()
	h.AuthCallback(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", got)
	}
	if location := rr.Header().Get("Location"); location != "" {
		t.Fatalf("Location header = %q, want no redirect", location)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "window.close()") {
		t.Fatalf("body should close the OAuth tab, got: %s", body)
	}
	if !strings.Contains(body, "http://localhost:5173/setup?auth=success&amp;reader=gmail") {
		t.Fatalf("body should include escaped fallback setup link, got: %s", body)
	}
	if !strings.Contains(string(st.readerTokens["tenant-a/gmail"]), "new-refresh") {
		t.Fatalf("saved token = %s, want refresh token from re-grant", st.readerTokens["tenant-a/gmail"])
	}
}

func TestAuthCallback_UsesTenantFromOAuthStartState(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("token endpoint method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenServer.Close()

	secretJSON := fmt.Sprintf(`{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-client-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": %q,
			"redirect_uris": ["http://localhost:8080/api/auth/callback"]
		}
	}`, tokenServer.URL)
	st := &mockStore{
		readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(secretJSON)},
	}
	h := newTestHandlers(t, st, &mockDaemon{})

	startCtx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	startReq := httptest.NewRequestWithContext(startCtx, http.MethodPost, "/api/providers/gmail/auth/start", nil)
	startReq.SetPathValue("name", "gmail")
	startRR := httptest.NewRecorder()
	h.AuthStart(startRR, startReq)

	if startRR.Code != http.StatusOK {
		t.Fatalf("start status = %d body=%s", startRR.Code, startRR.Body.String())
	}
	var startResp map[string]string
	decodeJSON(t, startRR.Body.String(), &startResp)
	authURL, err := url.Parse(startResp["url"])
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	state := authURL.Query().Get("state")
	if state == "" {
		t.Fatalf("auth URL missing state: %s", startResp["url"])
	}

	callbackReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/auth/callback?state="+url.QueryEscape(state)+"&code=4%2F0Acode", nil)
	callbackRR := httptest.NewRecorder()
	h.AuthCallback(callbackRR, callbackReq)

	if callbackRR.Code != http.StatusOK {
		t.Fatalf("callback status = %d body=%s", callbackRR.Code, callbackRR.Body.String())
	}
	if !strings.Contains(string(st.readerTokens["tenant-a/gmail"]), "new-refresh") {
		t.Fatalf("saved tenant token = %s, want refresh token from re-grant", st.readerTokens["tenant-a/gmail"])
	}
}

func TestAuthExchange_MissingURL_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/providers/gmail/auth/exchange", strings.NewReader(`{}`))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthExchange(rr, req)

	assertValidationError(t, rr, "url", "body", "is required")
}

func TestAuthExchange_MalformedURL_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/providers/gmail/auth/exchange", strings.NewReader(`{"url":":::not-a-url"}`))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthExchange(rr, req)

	assertValidationError(t, rr, "url", "body", "must be a valid URL")
}

func TestAuthExchange_MissingCode_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	body := `{"url":"http://localhost:8080/api/auth/callback?state=somestate"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/providers/gmail/auth/exchange", strings.NewReader(body))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthExchange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing code, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestAuthExchange_MissingState_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	body := `{"url":"http://localhost:8080/api/auth/callback?code=4%2F0Acode"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/providers/gmail/auth/exchange", strings.NewReader(body))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthExchange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing state, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestAuthExchange_UnknownState_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})
	body := `{"url":"http://localhost:8080/api/auth/callback?code=4%2F0Acode&state=doesnotexist"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/providers/gmail/auth/exchange", strings.NewReader(body))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthExchange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown state, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestAuthExchange_ExpiredState_Returns400(t *testing.T) {
	h := newTestHandlers(t, nil, &mockDaemon{})

	expiredState := "reader:gmail:expiredtoken"
	h.mu.Lock()
	h.oauthStates[expiredState] = oauthStateEntry{
		readerName: "gmail",
		expiresAt:  time.Now().Add(-1 * time.Second),
	}
	h.mu.Unlock()

	body := `{"url":"http://localhost:8080/api/auth/callback?code=4%2F0Acode&state=` + expiredState + `"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/providers/gmail/auth/exchange", strings.NewReader(body))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthExchange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for expired state, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestAuthExchange_RejectsStateFromDifferentTenant(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("token endpoint method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenServer.Close()

	secretJSON := fmt.Sprintf(`{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-client-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": %q,
			"redirect_uris": ["http://localhost:8080/api/auth/callback"]
		}
	}`, tokenServer.URL)
	h := newTestHandlers(t, &mockStore{
		readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(secretJSON)},
	}, &mockDaemon{})

	state := "reader:gmail:tenant-a"
	h.mu.Lock()
	h.oauthStates[state] = oauthStateEntry{
		readerName: "gmail",
		tenant:     store.Tenant{ID: "tenant-a"},
		expiresAt:  time.Now().Add(time.Minute),
	}
	h.mu.Unlock()

	body := `{"url":"http://localhost:8080/api/auth/callback?code=4%2F0Acode&state=` + state + `"}`
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-b", TenantID: "tenant-b", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/providers/gmail/auth/exchange", strings.NewReader(body))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthExchange(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for tenant mismatch, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestAuthExchange_RestartsRunningDaemonAfterTokenSaved(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("token endpoint method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenServer.Close()

	secretJSON := fmt.Sprintf(`{
		"installed": {
			"client_id": "test-client-id.apps.googleusercontent.com",
			"client_secret": "test-client-secret",
			"auth_uri": "https://accounts.google.com/o/oauth2/auth",
			"token_uri": %q,
			"redirect_uris": ["http://localhost:8080/api/auth/callback"]
		}
	}`, tokenServer.URL)
	st := &mockStore{readerSecrets: map[string][]byte{"tenant-a/gmail": []byte(secretJSON)}}
	dm := &mockDaemon{status: DaemonStatus{Running: true}}
	h := newTestHandlers(t, st, dm)
	var restarted daemon.RunRequest
	h.daemon.(*mockDaemon).restartFn = func(req daemon.RunRequest) { restarted = req }

	state := "reader:gmail:validtoken"
	h.mu.Lock()
	h.oauthStates[state] = oauthStateEntry{
		readerName: "gmail",
		tenant:     store.Tenant{ID: "tenant-a"},
		expiresAt:  time.Now().Add(time.Minute),
	}
	h.mu.Unlock()

	body := `{"url":"http://localhost:8080/api/auth/callback?code=4%2F0Acode&state=` + state + `"}`
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: "user-a", TenantID: "tenant-a", Role: auth.RoleUser})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/providers/gmail/auth/exchange", strings.NewReader(body))
	req.SetPathValue("name", "gmail")
	rr := httptest.NewRecorder()
	h.AuthExchange(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if restarted.Reader != "gmail" {
		t.Fatalf("restartFn reader = %q, want gmail", restarted.Reader)
	}
	if restarted.Tenant.ID != "tenant-a" {
		t.Fatalf("restartFn tenant = %q, want tenant-a", restarted.Tenant.ID)
	}
	if !strings.Contains(string(st.readerTokens["tenant-a/gmail"]), "new-refresh") {
		t.Fatalf("saved token = %s, want refresh token from re-grant", st.readerTokens["tenant-a/gmail"])
	}
}
