package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// HandleGetChartData handles GET /api/stats/charts.
func (h *Handlers) HandleGetChartData(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}
	cd, err := h.store.GetChartData(r.Context())
	if err != nil {
		h.logger.Error("get chart data", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch chart data")
		return
	}
	writeJSON(w, http.StatusOK, cd)
}

// HandleGetDashboardData handles GET /api/stats/dashboard.
func (h *Handlers) HandleGetDashboardData(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	data, err := h.store.GetDashboardData(r.Context())
	if err != nil {
		h.logger.Error("get dashboard data", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch dashboard data")
		return
	}

	writeJSON(w, http.StatusOK, data)
}

// HandleGetHeatmap handles GET /api/stats/heatmap.
// Optional query params: from=<RFC3339>, to=<RFC3339> (both or neither).
// Returns 400 if either param is present but malformed.
func (h *Handlers) HandleGetHeatmap(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	from, to, err := parseHeatmapRange(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	data, err := h.store.GetSpendingHeatmap(r.Context(), from, to)
	if err != nil {
		h.logger.Error("get heatmap", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch heatmap data")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// HandleGetAnnualHeatmap handles GET /api/stats/heatmap/annual?year=YYYY.
// Returns per-day transaction totals for the requested calendar year.
// Defaults to the current year when ?year is absent or invalid.
func (h *Handlers) HandleGetAnnualHeatmap(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database not connected")
		return
	}

	yearStr := r.URL.Query().Get("year")
	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 1 {
		year = time.Now().Year()
	}

	buckets, err := h.store.GetAnnualSpend(r.Context(), year)
	if err != nil {
		h.logger.Error("get annual heatmap", "error", err, "year", year)
		writeError(w, http.StatusInternalServerError, "failed to fetch annual heatmap data")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"year":    year,
		"buckets": buckets,
	})
}

// parseHeatmapRange parses optional ?from= and ?to= RFC3339 query parameters.
// Returns nil, nil when neither is provided. Returns an error if either is
// present but cannot be parsed as RFC3339.
func parseHeatmapRange(r *http.Request) (from, to *time.Time, err error) {
	if v := r.URL.Query().Get("from"); v != "" {
		t, parseErr := time.Parse(time.RFC3339, v)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("invalid 'from' param: must be RFC3339 (e.g. 2026-04-01T00:00:00Z)")
		}
		from = &t
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, parseErr := time.Parse(time.RFC3339, v)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("invalid 'to' param: must be RFC3339 (e.g. 2026-04-30T23:59:59Z)")
		}
		to = &t
	}
	return from, to, nil
}
