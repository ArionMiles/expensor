// Package client provides OAuth2 client setup for Google APIs.
package client

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// callbackPort is the port for the local OAuth callback server.
	callbackPort = 8085
	// callbackPath is the path for the OAuth callback.
	callbackPath = "/callback"
	// serverTimeout is how long to wait for the OAuth callback.
	serverTimeout = 5 * time.Minute
)

const (
	// TokenFile is the path to the OAuth token file.
	TokenFile = "data/token.json"
)

// New creates a new HTTP client with OAuth2 credentials from a file path.
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

	client, err := getClient(config)
	if err != nil {
		return nil, fmt.Errorf("getting oauth client: %w", err)
	}

	return client, nil
}

func getClient(config *oauth2.Config) (*http.Client, error) {
	tok, err := tokenFromFile(TokenFile)
	if err != nil {
		slog.Info("no existing token found, initiating OAuth flow")
		tok, err = getTokenFromWeb(config)
		if err != nil {
			return nil, err
		}
		if err := saveToken(TokenFile, tok); err != nil {
			slog.Error("failed to save token", "error", err)
		}
	}
	return config.Client(context.Background(), tok), nil
}

func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	ctx := context.Background()

	// Set the redirect URI to our local callback server
	config.RedirectURL = fmt.Sprintf("http://localhost:%d%s", callbackPort, callbackPath)

	// Generate a random state token for CSRF protection
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("generating state token: %w", err)
	}

	// Create channels for the callback result
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Start local server to receive the callback
	server, err := startCallbackServer(ctx, state, codeChan, errChan)
	if err != nil {
		return nil, fmt.Errorf("starting callback server: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			slog.Warn("error shutting down callback server", "error", err)
		}
	}()

	// Generate auth URL and open browser
	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline)

	fmt.Printf("\nOpening browser for Google authentication...\n")
	fmt.Printf("If the browser doesn't open automatically, visit this URL:\n%s\n\n", authURL)

	if err := openBrowser(authURL); err != nil {
		slog.Warn("failed to open browser automatically", "error", err)
	}

	// Wait for callback or timeout
	select {
	case code := <-codeChan:
		tok, err := config.Exchange(context.Background(), code)
		if err != nil {
			return nil, fmt.Errorf("exchanging authorization code for token: %w", err)
		}
		fmt.Println("Authentication successful!")
		return tok, nil
	case err := <-errChan:
		return nil, fmt.Errorf("oauth callback error: %w", err)
	case <-time.After(serverTimeout):
		return nil, fmt.Errorf("oauth flow timed out after %v", serverTimeout)
	}
}

func startCallbackServer(ctx context.Context, expectedState string, codeChan chan<- string, errChan chan<- error) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		// Verify state to prevent CSRF
		if state := r.URL.Query().Get("state"); state != expectedState {
			errChan <- fmt.Errorf("invalid state parameter")
			http.Error(w, "Invalid state parameter", http.StatusBadRequest)
			return
		}

		// Check for errors from OAuth provider
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			errDesc := r.URL.Query().Get("error_description")
			errChan <- fmt.Errorf("%s: %s", errMsg, errDesc)
			http.Error(w, fmt.Sprintf("Authentication failed: %s", errMsg), http.StatusBadRequest)
			return
		}

		// Extract authorization code
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("no authorization code received")
			http.Error(w, "No authorization code received", http.StatusBadRequest)
			return
		}

		// Send success response to browser
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Authentication Successful</title></head>
<body style="font-family: sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0;">
<div style="text-align: center;">
<h1 style="color: #4CAF50;">✓ Authentication Successful</h1>
<p>You can close this window and return to the terminal.</p>
</div>
</body>
</html>`)

		// Send code to main goroutine
		codeChan <- code
	})

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", callbackPort),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Create listener to check if port is available
	lc := net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", server.Addr)
	if err != nil {
		return nil, fmt.Errorf("port %d unavailable: %w", callbackPort, err)
	}

	// Start server in background
	go func() {
		slog.Debug("starting OAuth callback server", "port", callbackPort)
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			slog.Error("callback server error", "error", err)
			errChan <- err
		}
	}()

	return server, nil
}

func openBrowser(url string) error {
	ctx := context.Background()
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", url)
	case "linux":
		cmd = exec.CommandContext(ctx, "xdg-open", url)
	case "windows":
		cmd = exec.CommandContext(ctx, "cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) error {
	slog.Info("saving credential file", "path", path)

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
