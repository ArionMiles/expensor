package httpapi

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
)

func TestGetDashboardData_Success(t *testing.T) {
	ms := &mockStore{
		dashboardData: &store.DashboardData{
			CurrentMonth: store.DashboardSection{
				Label: "April 2026",
				Stats: store.Stats{TotalCount: 1, TotalBase: 1000, BaseCurrency: "INR"},
				Charts: store.ChartData{
					MonthlySpend:      []store.TimeBucket{},
					DailySpend:        []store.TimeBucket{},
					ByCategory:        map[string]float64{"Shopping": 1000},
					ByBucket:          map[string]float64{},
					ByLabel:           map[string]float64{},
					BySource:          map[string]float64{},
					ByCategoryMonthly: map[string]store.CategoryMonthlyEntry{},
				},
			},
			AllTime: store.DashboardSection{
				Label: "All Time",
				Stats: store.Stats{TotalCount: 3, TotalBase: 3000, BaseCurrency: "INR"},
				Charts: store.ChartData{
					MonthlySpend:      []store.TimeBucket{},
					DailySpend:        []store.TimeBucket{},
					ByCategory:        map[string]float64{"Shopping": 3000},
					ByBucket:          map[string]float64{},
					ByLabel:           map[string]float64{},
					BySource:          map[string]float64{},
					ByCategoryMonthly: map[string]store.CategoryMonthlyEntry{},
				},
			},
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/dashboard", nil)
	rr := httptest.NewRecorder()
	h.GetDashboardData(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	var resp map[string]json.RawMessage
	decodeJSON(t, rr.Body.String(), &resp)
	if _, ok := resp["current_month"]; !ok {
		t.Fatalf("expected current_month section in response: %s", rr.Body.String())
	}
	if _, ok := resp["all_time"]; !ok {
		t.Fatalf("expected all_time section in response: %s", rr.Body.String())
	}
}

func TestGetMonthlyBreakdown_InvalidDimension(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/stats/labels/monthly?dimension=nope",
		nil,
	)
	rr := httptest.NewRecorder()
	h.GetLabelMonthlySpend(rr, req)

	assertValidationError(t, rr, "dimension", "query", "must be one of: labels, categories, buckets")
}

func TestGetFacets_ReturnsEmptySlices(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/facets", nil)
	rr := httptest.NewRecorder()
	h.GetFacets(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp struct {
		Sources     []string       `json:"sources"`
		Categories  []string       `json:"categories"`
		Currencies  []string       `json:"currencies"`
		Labels      []string       `json:"labels"`
		LabelCounts map[string]int `json:"label_counts"`
		Buckets     []string       `json:"buckets"`
	}
	decodeJSON(t, rr.Body.String(), &resp)
	emptySlices := map[string][]string{
		"sources":    resp.Sources,
		"categories": resp.Categories,
		"currencies": resp.Currencies,
		"labels":     resp.Labels,
		"buckets":    resp.Buckets,
	}
	for key, value := range emptySlices {
		if value == nil {
			t.Errorf("expected %q to be an empty slice, got nil", key)
		}
	}
	if resp.LabelCounts == nil {
		t.Error("expected label_counts to be an empty object, got nil")
	}
}

func TestGetFacets_ReturnsLabelCounts(t *testing.T) {
	h := newTestHandlers(
		t,
		&mockStore{facets: &store.Facets{Labels: []string{"Food"}, LabelCounts: map[string]int{"Food": 3}}},
		&mockDaemon{},
	)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/facets", nil)
	rr := httptest.NewRecorder()

	h.GetFacets(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp struct {
		LabelCounts map[string]int `json:"label_counts"`
	}
	decodeJSON(t, rr.Body.String(), &resp)
	if got := resp.LabelCounts["Food"]; got != 3 {
		t.Fatalf("expected Food label count 3, got %d", got)
	}
}

func TestGetFacets_StoreError(t *testing.T) {
	h := newTestHandlers(t, &mockStore{getFacetsErr: stderrors.New("db error")}, &mockDaemon{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/transactions/facets", nil)
	rr := httptest.NewRecorder()
	h.GetFacets(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestGetHeatmap_Success(t *testing.T) {
	ms := &mockStore{
		heatmapData: &store.HeatmapData{
			ByWeekdayHour: []store.WeekdayHourBucket{
				{Weekday: 1, Hour: 14, Amount: 500.0, Count: 3},
			},
			ByDayOfMonth: []store.DayOfMonthBucket{
				{Day: 15, Amount: 1200.0, Count: 5},
			},
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/heatmap", nil)
	rr := httptest.NewRecorder()
	h.GetHeatmap(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp store.HeatmapData
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp.ByWeekdayHour) != 1 {
		t.Errorf("expected 1 weekday/hour bucket, got %d", len(resp.ByWeekdayHour))
	}
	if resp.ByWeekdayHour[0].Hour != 14 {
		t.Errorf("expected Hour=14, got %d", resp.ByWeekdayHour[0].Hour)
	}
	if len(resp.ByDayOfMonth) != 1 {
		t.Errorf("expected 1 day-of-month bucket, got %d", len(resp.ByDayOfMonth))
	}
}

func TestGetHeatmap_StoreError_Returns500(t *testing.T) {
	ms := &mockStore{heatmapErr: stderrors.New("db connection lost")}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/heatmap", nil)
	rr := httptest.NewRecorder()
	h.GetHeatmap(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestGetHeatmap_WithFromTo_Returns200(t *testing.T) {
	ms := &mockStore{
		heatmapData: &store.HeatmapData{
			ByWeekdayHour: []store.WeekdayHourBucket{{Weekday: 0, Hour: 10, Amount: 100, Count: 1}},
			ByDayOfMonth:  []store.DayOfMonthBucket{},
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet,
		"/api/stats/heatmap?from=2026-04-01T00:00:00Z&to=2026-04-30T23:59:59Z",
		nil,
	)
	rr := httptest.NewRecorder()
	h.GetHeatmap(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp store.HeatmapData
	decodeJSON(t, rr.Body.String(), &resp)
	if len(resp.ByWeekdayHour) != 1 {
		t.Errorf("expected 1 bucket, got %d", len(resp.ByWeekdayHour))
	}
}

func TestGetHeatmap_InvalidFrom_Returns400(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})

	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet,
		"/api/stats/heatmap?from=not-a-date",
		nil,
	)
	rr := httptest.NewRecorder()
	h.GetHeatmap(rr, req)

	assertValidationError(t, rr, "from", "query", "must be an RFC3339 timestamp")
}

func TestGetHeatmap_RejectsInvalidYear(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	rr := get(h.GetHeatmap, "/api/stats/heatmap?year=invalid")

	assertValidationError(t, rr, "year", "query", "must be an integer")
}

func TestGetHeatmap_WithYear_ReturnsAnnualData(t *testing.T) {
	ms := &mockStore{
		annualData: []store.DailyBucket{
			{Date: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Amount: 1500.0, Count: 3},
		},
	}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/heatmap?year=2026", nil)
	rr := httptest.NewRecorder()
	h.GetHeatmap(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp struct {
		Year    int                 `json:"year"`
		Buckets []store.DailyBucket `json:"buckets"`
	}
	decodeJSON(t, rr.Body.String(), &resp)
	if resp.Year != 2026 {
		t.Errorf("expected year=2026, got %d", resp.Year)
	}
	if len(resp.Buckets) != 1 {
		t.Errorf("expected 1 bucket, got %d", len(resp.Buckets))
	}
	if resp.Buckets[0].Amount != 1500.0 {
		t.Errorf("expected Amount=1500, got %f", resp.Buckets[0].Amount)
	}
}

func TestGetHeatmap_WithYearStoreError_Returns500(t *testing.T) {
	ms := &mockStore{annualErr: stderrors.New("db connection lost")}
	h := newTestHandlers(t, ms, &mockDaemon{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/stats/heatmap?year=2026", nil)
	rr := httptest.NewRecorder()
	h.GetHeatmap(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestGetHeatmap_RejectsYearWithRange(t *testing.T) {
	h := newTestHandlers(t, &mockStore{}, &mockDaemon{})
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"/api/stats/heatmap?year=2026&from=2026-01-01T00:00:00Z",
		nil,
	)
	rr := httptest.NewRecorder()
	h.GetHeatmap(rr, req)

	assertValidationError(t, rr, "year", "query", "cannot be combined with from or to")
}
