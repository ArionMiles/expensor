// Package client provides OAuth2 client setup for Google APIs.
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// ReaderTokenStore is the DB persistence surface used by store-backed OAuth clients.
type ReaderTokenStore interface {
	GetReaderToken(ctx context.Context, reader string) ([]byte, bool, error)
	SetReaderToken(ctx context.Context, reader string, token []byte) error
}

// NewFromJSONAndStore creates a new HTTP client with OAuth2 credentials from JSON content and a DB-backed token store.
func NewFromJSONAndStore(ctx context.Context, secretJSON []byte, store ReaderTokenStore, reader string, scope ...string) (*http.Client, error) {
	config, err := google.ConfigFromJSON(secretJSON, scope...)
	if err != nil {
		return nil, fmt.Errorf("parsing client secret: %w", err)
	}
	if store == nil {
		return nil, fmt.Errorf("token store is nil")
	}

	tokenJSON, ok, err := store.GetReaderToken(ctx, reader)
	if err != nil {
		return nil, fmt.Errorf("loading token for reader %q: %w", reader, err)
	}
	if !ok {
		return nil, fmt.Errorf("loading token for reader %q: token missing (use web interface to authenticate)", reader)
	}
	tok := &oauth2.Token{}
	if err := json.Unmarshal(tokenJSON, tok); err != nil {
		return nil, fmt.Errorf("parsing token for reader %q: %w", reader, err)
	}

	tokenSource := &persistingStoreTokenSource{
		ctx:         ctx,
		reader:      reader,
		store:       store,
		tokenSource: config.TokenSource(ctx, tok),
	}
	return oauth2.NewClient(ctx, tokenSource), nil
}

// persistingStoreTokenSource wraps an oauth2.TokenSource and persists refreshed tokens to the runtime store.
type persistingStoreTokenSource struct {
	ctx         context.Context
	reader      string
	store       ReaderTokenStore
	tokenSource oauth2.TokenSource
	cachedToken *oauth2.Token
}

func (p *persistingStoreTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.tokenSource.Token()
	if err != nil {
		return nil, err
	}

	if p.cachedToken == nil || tok.AccessToken != p.cachedToken.AccessToken {
		tokenJSON, marshalErr := json.Marshal(tok) //nolint:gosec // OAuth tokens are intentionally serialized into the runtime store.
		if marshalErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to marshal refreshed token: %v\n", marshalErr)
		} else if saveErr := p.store.SetReaderToken(p.ctx, p.reader, tokenJSON); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to persist refreshed token: %v\n", saveErr)
		}
		p.cachedToken = tok
	}

	return tok, nil
}

// GetOAuthConfig creates an OAuth2 config from client secret JSON.
func GetOAuthConfig(secretJSON []byte, redirectURL string, scopes ...string) (*oauth2.Config, error) {
	config, err := google.ConfigFromJSON(secretJSON, scopes...)
	if err != nil {
		return nil, fmt.Errorf("parsing client secret: %w", err)
	}

	config.RedirectURL = redirectURL
	return config, nil
}
