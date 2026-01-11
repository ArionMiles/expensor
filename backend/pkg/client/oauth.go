// Package client provides OAuth2 client setup for Google APIs.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// TokenFile is the path to the OAuth token file.
	TokenFile = "data/token.json"
)

// New creates a new HTTP client with OAuth2 credentials from a file path.
// This version is designed for web-based OAuth flows - tokens must be managed
// by the web application, not via CLI callback.
func New(secretFilePath string, scope ...string) (*http.Client, error) {
	b, err := os.ReadFile(secretFilePath)
	if err != nil {
		return nil, fmt.Errorf("reading client secret file: %w", err)
	}

	return NewFromJSON(b, scope...)
}

// NewFromJSON creates a new HTTP client with OAuth2 credentials from JSON content.
func NewFromJSON(secretJSON []byte, scope ...string) (*http.Client, error) {
	config, err := google.ConfigFromJSON(secretJSON, scope...)
	if err != nil {
		return nil, fmt.Errorf("parsing client secret: %w", err)
	}

	tok, err := TokenFromFile(TokenFile)
	if err != nil {
		return nil, fmt.Errorf("loading token: %w (use web interface to authenticate)", err)
	}

	return config.Client(context.Background(), tok), nil
}

// TokenFromFile retrieves a token from a local file.
func TokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// SaveToken saves a token to a file path.
func SaveToken(path string, token *oauth2.Token) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating token directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("creating token file: %w", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(token); err != nil {
		return fmt.Errorf("encoding token: %w", err)
	}
	return nil
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
