package oauth_test

import (
	"context"
	"testing"

	"github.com/ArionMiles/expensor/backend/internal/oauth"
	storepkg "github.com/ArionMiles/expensor/backend/internal/store"
)

const testSecretJSON = `{
  "installed": {
    "client_id": "test-client-id.apps.googleusercontent.com",
    "client_secret": "test-client-secret",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token",
    "redirect_uris": ["http://localhost"]
  }
}`

type fakeTokenStore struct {
	token []byte
}

func (f *fakeTokenStore) GetReaderToken(_ context.Context, _ storepkg.Tenant, _ string) (token []byte, found bool, err error) {
	return f.token, len(f.token) > 0, nil
}

func (f *fakeTokenStore) SetReaderToken(_ context.Context, _ storepkg.Tenant, _ string, token []byte) error {
	f.token = token
	return nil
}

func TestNewFromJSONAndStoreLoadsToken(t *testing.T) {
	store := &fakeTokenStore{token: []byte(`{"access_token":"a","token_type":"Bearer","expiry":"2999-01-01T00:00:00Z"}`)}
	client, err := oauth.NewFromJSONAndStore(context.Background(), oauth.StoreClientInput{
		SecretJSON: []byte(testSecretJSON),
		Store:      store,
		Tenant:     storepkg.Tenant{},
		Reader:     "gmail",
		Scopes:     []string{"https://www.googleapis.com/auth/gmail.readonly"},
	})
	if err != nil {
		t.Fatalf("NewFromJSONAndStore: %v", err)
	}
	if client == nil {
		t.Fatal("client is nil")
	}
}
