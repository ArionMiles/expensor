package helpers

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func WaitForHealthy(t *testing.T) string {
	t.Helper()

	baseURL := strings.TrimRight(os.Getenv("COMPONENT_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://backend:8080"
	}

	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, baseURL+"/api/health", nil)
		if err == nil {
			resp, doErr := client.Do(req)
			if doErr == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return baseURL
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("backend did not become healthy before timeout")
	return ""
}
