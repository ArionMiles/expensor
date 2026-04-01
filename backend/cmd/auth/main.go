// Package main provides a simple CLI to obtain OAuth2 tokens for Google APIs.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

const (
	clientSecretFile = "../data/client_secret.json"
	tokenFile        = "../data/token.json"
)

func main() {
	b, err := os.ReadFile(clientSecretFile)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	scopes := []string{
		gmail.GmailReadonlyScope,
		gmail.GmailModifyScope,
	}

	config, err := google.ConfigFromJSON(b, scopes...)
	if err != nil {
		log.Fatalf("Unable to parse client secret: %v", err)
	}

	// Use localhost callback for CLI flow
	config.RedirectURL = "http://localhost:8085/callback"

	// Start local server to receive callback
	codeChan := make(chan string)
	server := &http.Server{Addr: ":8085"}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "No code in callback", http.StatusBadRequest)
			return
		}
		fmt.Fprintln(w, "Authentication successful! You can close this window.")
		codeChan <- code
	})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("Go to the following link in your browser:\n\n%s\n\n", authURL)

	// Wait for callback
	code := <-codeChan
	if shutdownErr := server.Shutdown(context.Background()); shutdownErr != nil {
		log.Printf("HTTP server shutdown error: %v", shutdownErr)
	}

	tok, err := config.Exchange(context.Background(), code)
	if err != nil {
		log.Fatalf("Unable to exchange code for token: %v", err)
	}

	if err := saveToken(tokenFile, tok); err != nil {
		log.Fatalf("Unable to save token: %v", err)
	}

	fmt.Printf("Token saved to %s\n", tokenFile)
}

func saveToken(path string, token *oauth2.Token) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}
