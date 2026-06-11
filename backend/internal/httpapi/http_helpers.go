package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

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
