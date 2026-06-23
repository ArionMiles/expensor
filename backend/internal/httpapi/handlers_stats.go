package httpapi

import (
	"net/http"

	"github.com/go-playground/validator/v10"
)

// GetChartData handles GET /api/stats/charts.
// @Summary Get chart data
// @Tags Stats
// @Produce json
// @Success 200 {object} ChartDataResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /stats/charts [get]
func (h *Handlers) GetChartData(w http.ResponseWriter, r *http.Request) {
	cd, err := h.analyticsStore.GetChartData(r.Context(), requestTenant(r))
	if err != nil {
		h.logger.Error("get chart data", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch chart data")
		return
	}
	writeJSON(w, http.StatusOK, cd)
}

// GetDashboardData handles GET /api/stats/dashboard.
// @Summary Get dashboard data
// @Tags Stats
// @Produce json
// @Success 200 {object} DashboardDataResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /stats/dashboard [get]
func (h *Handlers) GetDashboardData(w http.ResponseWriter, r *http.Request) {
	data, err := h.analyticsStore.GetDashboardData(r.Context(), requestTenant(r))
	if err != nil {
		h.logger.Error("get dashboard data", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch dashboard data")
		return
	}

	writeJSON(w, http.StatusOK, data)
}

// GetHeatmap handles GET /api/stats/heatmap.
// Optional query params: from=<RFC3339>, to=<RFC3339>, or year=<YYYY>.
// Returns 400 if either param is present but malformed.
// @Summary Get spending heatmap
// @Tags Stats
// @Produce json
// @Param from query string false "RFC3339 start timestamp" example(2026-05-01T00:00:00Z)
// @Param to query string false "RFC3339 end timestamp" example(2026-05-31T23:59:59Z)
// @Param year query int false "Calendar year; cannot be combined with from or to" example(2026)
// @Success 200 {object} HeatmapResponse
// @Failure 422 {object} ValidationErrorResponse
// @Failure 500 {object} ErrorResponse
// @Failure 503 {object} ErrorResponse
// @Router /stats/heatmap [get]
func (h *Handlers) GetHeatmap(w http.ResponseWriter, r *http.Request) {
	query, ok := decodeAndValidateQuery[heatmapQuery](h, w, r)
	if !ok {
		return
	}
	if query.Year != nil {
		h.getAnnualHeatmap(w, r, *query.Year)
		return
	}

	data, err := h.analyticsStore.GetSpendingHeatmap(r.Context(), requestTenant(r), query.From, query.To)
	if err != nil {
		h.logger.Error("get heatmap", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch heatmap data")
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (h *Handlers) getAnnualHeatmap(w http.ResponseWriter, r *http.Request, year int) {
	buckets, err := h.analyticsStore.GetAnnualSpend(r.Context(), requestTenant(r), year)
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

func validateHeatmapQuery(level validator.StructLevel) {
	query, ok := level.Current().Interface().(heatmapQuery)
	if !ok || query.Year == nil || (query.From == nil && query.To == nil) {
		return
	}
	level.ReportError(query.Year, "year", "Year", "heatmap_range", "")
}
