package store

import (
	"context"
	"time"
)

type ReadModelRepository interface {
	GetStats(ctx context.Context, baseCurrency string) (*Stats, error)
	GetChartData(ctx context.Context) (*ChartData, error)
	GetDashboardData(ctx context.Context) (*DashboardData, error)
	GetSpendingHeatmap(ctx context.Context, from, to *time.Time) (*HeatmapData, error)
	GetAnnualSpend(ctx context.Context, year int) ([]DailyBucket, error)
	GetMonthlyBreakdownSpend(ctx context.Context, dimension string, months int) (*MonthlyBreakdownData, error)
}

type pgReadModelRepository struct {
	legacy  *Store
	metrics *QueryInstrumentation
}

func NewReadModelRepository(deps repositoryDependencies, legacy *Store) ReadModelRepository {
	metrics := deps.metrics
	if metrics == nil {
		metrics = NewQueryInstrumentation(deps.logger)
	}
	return &pgReadModelRepository{
		legacy:  legacy,
		metrics: metrics,
	}
}

func (r *pgReadModelRepository) GetStats(ctx context.Context, baseCurrency string) (*Stats, error) {
	var stats *Stats
	err := r.metrics.Observe(ctx, "read_model.get_stats", func(ctx context.Context) error {
		var err error
		stats, err = r.statsReadModel(ctx, baseCurrency)
		return err
	})
	return stats, err
}

func (r *pgReadModelRepository) GetChartData(ctx context.Context) (*ChartData, error) {
	var data *ChartData
	err := r.metrics.Observe(ctx, "read_model.get_chart_data", func(ctx context.Context) error {
		var err error
		data, err = r.chartDataReadModel(ctx)
		return err
	})
	return data, err
}

func (r *pgReadModelRepository) GetDashboardData(ctx context.Context) (*DashboardData, error) {
	var data *DashboardData
	err := r.metrics.Observe(ctx, "read_model.get_dashboard_data", func(ctx context.Context) error {
		var err error
		data, err = r.dashboardDataReadModel(ctx)
		return err
	})
	return data, err
}

func (r *pgReadModelRepository) GetSpendingHeatmap(ctx context.Context, from, to *time.Time) (*HeatmapData, error) {
	var data *HeatmapData
	err := r.metrics.Observe(ctx, "read_model.get_spending_heatmap", func(ctx context.Context) error {
		var err error
		data, err = r.spendingHeatmapReadModel(ctx, from, to)
		return err
	})
	return data, err
}

func (r *pgReadModelRepository) GetAnnualSpend(ctx context.Context, year int) ([]DailyBucket, error) {
	var buckets []DailyBucket
	err := r.metrics.Observe(ctx, "read_model.get_annual_spend", func(ctx context.Context) error {
		var err error
		buckets, err = r.annualSpendReadModel(ctx, year)
		return err
	})
	return buckets, err
}

func (r *pgReadModelRepository) GetMonthlyBreakdownSpend(ctx context.Context, dimension string, months int) (*MonthlyBreakdownData, error) {
	var data *MonthlyBreakdownData
	err := r.metrics.Observe(ctx, "read_model.get_monthly_breakdown_spend", func(ctx context.Context) error {
		var err error
		data, err = r.monthlyBreakdownSpendReadModel(ctx, dimension, months)
		return err
	})
	return data, err
}
