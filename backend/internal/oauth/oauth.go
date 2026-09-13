// Package client provides OAuth2 client setup for Google APIs.
package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

var (
	KindCredentialsMissing = errors.Kind{Code: "oauth_credentials_missing", Status: http.StatusPreconditionFailed}
	KindTokenMissing       = errors.Kind{Code: "oauth_token_missing", Status: http.StatusPreconditionFailed}
)

// ReaderTokenStore is the DB persistence surface used by store-backed OAuth clients.
type ReaderTokenStore interface {
	GetReaderToken(ctx context.Context, tenant store.Tenant, reader string) ([]byte, bool, error)
	SetReaderToken(ctx context.Context, tenant store.Tenant, reader string, token []byte) error
}

// StoreClientInput contains the dependencies needed for a DB-backed OAuth client.
type StoreClientInput struct {
	SecretJSON []byte
	Store      ReaderTokenStore
	Tenant     store.Tenant
	Reader     string
	Scopes     []string
}

// NewFromJSONAndStore creates a new HTTP client with OAuth2 credentials from JSON content and a DB-backed token store.
func NewFromJSONAndStore(ctx context.Context, input StoreClientInput) (*http.Client, error) {
	const op = "oauth.NewFromJSONAndStore"

	config, err := google.ConfigFromJSON(input.SecretJSON, input.Scopes...)
	if err != nil {
		return nil, errors.B.Op(op).KindInvalidInput().Text("parsing client secret").Err(err).Build()
	}
	if input.Store == nil {
		return nil, errors.B.Op(op).KindInternal().Text("token store is nil").Build()
	}

	tokenJSON, ok, err := input.Store.GetReaderToken(ctx, input.Tenant, input.Reader)
	if err != nil {
		return nil, errors.B.Op(op).Text("loading token for reader " + input.Reader).Err(err).Build()
	}
	if !ok {
		return nil, errors.B.Op(op).Kind(KindTokenMissing).UserMsg("provider is not authenticated").Text("reader token missing").Build()
	}
	tok := &oauth2.Token{}
	if err := json.Unmarshal(tokenJSON, tok); err != nil {
		return nil, errors.B.Op(op).KindInvalidInput().Text("parsing token for reader " + input.Reader).Err(err).Build()
	}

	tokenSource := &persistingStoreTokenSource{
		ctx:         ctx,
		tenant:      input.Tenant,
		reader:      input.Reader,
		store:       input.Store,
		tokenSource: config.TokenSource(ctx, tok),
	}
	if _, err := tokenSource.Token(); err != nil {
		return nil, errors.B.Op(op).KindFailedPrecondition().Text("refreshing token for reader " + input.Reader).Err(err).Build()
	}
	return oauth2.NewClient(ctx, tokenSource), nil
}

// persistingStoreTokenSource wraps an oauth2.TokenSource and persists refreshed tokens to the runtime store.
type persistingStoreTokenSource struct {
	ctx         context.Context
	tenant      store.Tenant
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
		} else if saveErr := p.store.SetReaderToken(p.ctx, p.tenant, p.reader, tokenJSON); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to persist refreshed token: %v\n", saveErr)
		}
		p.cachedToken = tok
	}

	return tok, nil
}

// GetOAuthConfig creates an OAuth2 config from client secret JSON.
func GetOAuthConfig(secretJSON []byte, redirectURL string, scopes ...string) (*oauth2.Config, error) {
	const op = "oauth.GetOAuthConfig"

	config, err := google.ConfigFromJSON(secretJSON, scopes...)
	if err != nil {
		return nil, errors.B.Op(op).KindInvalidInput().Text("parsing client secret").Err(err).Build()
	}

	config.RedirectURL = redirectURL
	return config, nil
}

func IsInvalidGrant(err error) bool {
	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &retrieveErr) && retrieveErr.ErrorCode == "invalid_grant" {
		return true
	}
	return strings.Contains(err.Error(), "invalid_grant")
}
