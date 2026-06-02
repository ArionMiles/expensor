package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

// --- helpers ---

// currentBaseCurrency returns the base currency from the DB, falling back to INR.
func (h *Handlers) currentBaseCurrency(ctx context.Context) string {
	if val, err := h.settingsStore.GetAppConfig(ctx, "base_currency"); err == nil && val != "" {
		return val
	}
	return defaultBaseCurrency
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func uuidPathValue(w http.ResponseWriter, r *http.Request, name, label string) (string, bool) {
	value := r.PathValue(name)
	if _, err := uuid.Parse(value); err != nil {
		writeError(w, http.StatusBadRequest, "invalid "+label+" id")
		return "", false
	}
	return value, true
}

// --- helpers ---

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return def
	}
	switch key {
	case "page":
		if n > maxPageParam {
			return def
		}
	case "page_size":
		if n > maxPageSizeParam {
			return def
		}
	}
	return n
}

// queryHour parses an hour filter (0–23) from a query parameter.
// Returns nil when the parameter is absent or out of range.
func queryHour(r *http.Request, key string) *int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 || n > 23 {
		return nil
	}
	return &n
}

// queryWeekday parses a PostgreSQL DOW weekday filter (0–6) from a query parameter.
// Returns nil when the parameter is absent or out of range.
func queryWeekday(r *http.Request, key string) *int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 || n > 6 {
		return nil
	}
	return &n
}
