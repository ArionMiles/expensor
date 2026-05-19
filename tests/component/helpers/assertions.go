package helpers

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func RequireStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("status=%d want=%d body=%s", resp.StatusCode, want, string(body))
	}
}

func DecodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()

	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response JSON: %v", err)
	}
	return out
}
