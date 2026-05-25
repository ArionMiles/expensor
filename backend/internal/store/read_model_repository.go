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
	legacy *Store
}

func NewReadModelRepository(deps repositoryDependencies, legacy *Store) ReadModelRepository {
	return &pgReadModelRepository{
		legacy: legacy,
	}
}

func (r *pgReadModelRepository) GetStats(ctx context.Context, baseCurrency string) (*Stats, error) {
	return r.statsReadModel(ctx, baseCurrency)
}

func (r *pgReadModelRepository) GetChartData(ctx context.Context) (*ChartData, error) {
	return r.chartDataReadModel(ctx)
}

func (r *pgReadModelRepository) GetDashboardData(ctx context.Context) (*DashboardData, error) {
	return r.dashboardDataReadModel(ctx)
}

func (r *pgReadModelRepository) GetSpendingHeatmap(ctx context.Context, from, to *time.Time) (*HeatmapData, error) {
	return r.spendingHeatmapReadModel(ctx, from, to)
}

func (r *pgReadModelRepository) GetAnnualSpend(ctx context.Context, year int) ([]DailyBucket, error) {
	return r.annualSpendReadModel(ctx, year)
}

func (r *pgReadModelRepository) GetMonthlyBreakdownSpend(ctx context.Context, dimension string, months int) (*MonthlyBreakdownData, error) {
	return r.monthlyBreakdownSpendReadModel(ctx, dimension, months)
}
