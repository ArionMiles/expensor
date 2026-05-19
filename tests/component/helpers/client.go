package helpers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient(t *testing.T) *Client {
	t.Helper()

	baseURL := strings.TrimRight(os.Getenv("COMPONENT_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://backend:8080"
	}

	return &Client{
		BaseURL: baseURL,
		HTTP: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) Get(t *testing.T, path string) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		t.Fatalf("new GET request: %v", err)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		t.Fatalf("do GET %s: %v", path, err)
	}
	return resp
}

func (c *Client) JSON(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}

	req, err := http.NewRequestWithContext(t.Context(), method, c.BaseURL+path, &buf)
	if err != nil {
		t.Fatalf("new %s request: %v", method, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		t.Fatalf("do %s %s: %v", method, path, err)
	}
	return resp
}
