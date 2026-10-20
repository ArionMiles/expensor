package sqlite

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const (
	monthlyDimensionLabels     = "labels"
	monthlyDimensionCategories = "categories"
	monthlyDimensionBuckets    = "buckets"
)

func (s *Store) GetStats(ctx context.Context, tenant store.Tenant, baseCurrency string) (*store.Stats, error) {
	records, err := s.analyticsRecords(ctx, tenant)
	if err != nil {
		return nil, err
	}
	return analyticsStats(records, baseCurrency), nil
}

func (s *Store) GetChartData(ctx context.Context, tenant store.Tenant) (*store.ChartData, error) {
	records, err := s.analyticsRecords(ctx, tenant)
	if err != nil {
		return nil, err
	}
	return analyticsCharts(records, s.analyticsLocation(ctx, tenant), s.repositories.now()), nil
}

func (s *Store) GetDashboardData(ctx context.Context, tenant store.Tenant) (*store.DashboardData, error) {
	records, err := s.analyticsRecords(ctx, tenant)
	if err != nil {
		return nil, err
	}
	location := s.analyticsLocation(ctx, tenant)
	now := s.repositories.now()
	localNow := now.In(location)
	start := time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, location)
	end := start.AddDate(0, 1, 0)
	current := filterAnalyticsRecords(records, func(record storedTransaction) bool {
		instant := record.transaction.Timestamp
		return !instant.Before(start) && instant.Before(end)
	})
	baseCurrency := "INR"
	if configured, configErr := s.GetAppConfig(ctx, tenant, "base_currency"); configErr == nil && configured != "" {
		baseCurrency = configured
	}
	currentCharts := analyticsCharts(current, location, now)
	allCharts := analyticsCharts(records, location, now)
	monthly := categoryMonthly(records, location, now)
	currentCharts.ByCategoryMonthly = monthly
	allCharts.ByCategoryMonthly = monthly
	return &store.DashboardData{
		CurrentMonth: store.DashboardSection{
			Label: localNow.Format("January 2006"), Stats: *analyticsStats(current, baseCurrency), Charts: *currentCharts,
		},
		AllTime: store.DashboardSection{
			Label: "All Time", Stats: *analyticsStats(records, baseCurrency), Charts: *allCharts,
		},
	}, nil
}

func (s *Store) GetSpendingHeatmap(ctx context.Context, tenant store.Tenant, from, to *time.Time) (*store.HeatmapData, error) {
	records, err := s.analyticsRecords(ctx, tenant)
	if err != nil {
		return nil, err
	}
	location := s.analyticsLocation(ctx, tenant)
	type key struct{ first, second int }
	weekday := map[key]*store.WeekdayHourBucket{}
	day := map[int]*store.DayOfMonthBucket{}
	for _, record := range records {
		instant := record.transaction.Timestamp
		if from != nil && instant.Before(*from) || to != nil && instant.After(*to) {
			continue
		}
		local := instant.In(location)
		weekdayKey := key{int(local.Weekday()), local.Hour()}
		if weekday[weekdayKey] == nil {
			weekday[weekdayKey] = &store.WeekdayHourBucket{Weekday: weekdayKey.first, Hour: weekdayKey.second}
		}
		weekday[weekdayKey].Amount += record.transaction.Amount
		weekday[weekdayKey].Count++
		if day[local.Day()] == nil {
			day[local.Day()] = &store.DayOfMonthBucket{Day: local.Day()}
		}
		day[local.Day()].Amount += record.transaction.Amount
		day[local.Day()].Count++
	}
	result := &store.HeatmapData{ByWeekdayHour: []store.WeekdayHourBucket{}, ByDayOfMonth: []store.DayOfMonthBucket{}}
	for _, value := range weekday {
		result.ByWeekdayHour = append(result.ByWeekdayHour, *value)
	}
	for _, value := range day {
		result.ByDayOfMonth = append(result.ByDayOfMonth, *value)
	}
	sort.Slice(result.ByWeekdayHour, func(i, j int) bool {
		left, right := result.ByWeekdayHour[i], result.ByWeekdayHour[j]
		return left.Weekday < right.Weekday || left.Weekday == right.Weekday && left.Hour < right.Hour
	})
	sort.Slice(result.ByDayOfMonth, func(i, j int) bool { return result.ByDayOfMonth[i].Day < result.ByDayOfMonth[j].Day })
	return result, nil
}

func (s *Store) GetAnnualSpend(ctx context.Context, tenant store.Tenant, year int) ([]store.DailyBucket, error) {
	records, err := s.analyticsRecords(ctx, tenant)
	if err != nil {
		return nil, err
	}
	location := s.analyticsLocation(ctx, tenant)
	byDay := map[string]*store.DailyBucket{}
	for _, record := range records {
		local := record.transaction.Timestamp.In(location)
		if local.Year() != year {
			continue
		}
		key := local.Format(time.DateOnly)
		if byDay[key] == nil {
			byDay[key] = &store.DailyBucket{Date: time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)}
		}
		byDay[key].Amount += record.transaction.Amount
		byDay[key].Count++
	}
	result := make([]store.DailyBucket, 0, len(byDay))
	for _, value := range byDay {
		result = append(result, *value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Date.Before(result[j].Date) })
	return result, nil
}

func (s *Store) GetMonthlyBreakdownSpend(ctx context.Context, tenant store.Tenant, dimension string, months int) (*store.MonthlyBreakdownData, error) {
	result := &store.MonthlyBreakdownData{Labels: []string{}, Months: []string{}, Series: []store.MonthlyBreakdownSeries{}}
	if months <= 0 {
		return result, nil
	}
	if dimension == "" {
		dimension = monthlyDimensionLabels
	}
	if dimension != monthlyDimensionLabels &&
		dimension != monthlyDimensionCategories &&
		dimension != monthlyDimensionBuckets {
		return nil, errors.B.Op("store.analytics.monthly_breakdown").KindInvalidInput().Text("unsupported breakdown dimension").Build()
	}
	records, err := s.analyticsRecords(ctx, tenant)
	if err != nil {
		return nil, err
	}
	location := s.analyticsLocation(ctx, tenant)
	now := s.repositories.now().In(location)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location).AddDate(0, -(months - 1), 0)
	monthIndex := make(map[string]int, months)
	for i := range months {
		month := start.AddDate(0, i, 0).Format("2006-01")
		result.Months = append(result.Months, month)
		monthIndex[month] = i
	}
	series := map[string][]float64{}
	for _, record := range records {
		index, ok := monthIndex[record.transaction.Timestamp.In(location).Format("2006-01")]
		if !ok {
			continue
		}
		for _, value := range monthlyBreakdownValues(record.transaction, dimension) {
			if value == "" {
				continue
			}
			if series[value] == nil {
				series[value] = make([]float64, months)
			}
			series[value][index] += record.transaction.Amount
		}
	}
	for label := range series {
		result.Labels = append(result.Labels, label)
	}
	sort.Strings(result.Labels)
	for _, label := range result.Labels {
		result.Series = append(result.Series, store.MonthlyBreakdownSeries{Label: label, Data: series[label]})
	}
	return result, nil
}

func monthlyBreakdownValues(transaction store.Transaction, dimension string) []string {
	switch dimension {
	case monthlyDimensionLabels:
		return transaction.Labels
	case monthlyDimensionCategories:
		return []string{analyticsName(transaction.Category)}
	case monthlyDimensionBuckets:
		return []string{analyticsName(transaction.Bucket)}
	default:
		return nil
	}
}

func (s *Store) analyticsRecords(ctx context.Context, tenant store.Tenant) ([]storedTransaction, error) {
	records, err := s.loadStoredTransactions(ctx, tenant)
	if err != nil {
		return nil, err
	}
	return filterAnalyticsRecords(records, func(record storedTransaction) bool { return !record.transaction.Muted }), nil
}

func (s *Store) analyticsLocation(ctx context.Context, tenant store.Tenant) *time.Location {
	name, err := s.GetAppConfig(ctx, tenant, "app.timezone")
	if err == nil && name != "" {
		if location, loadErr := time.LoadLocation(name); loadErr == nil {
			return location
		}
	}
	return time.UTC
}

func filterAnalyticsRecords(records []storedTransaction, keep func(storedTransaction) bool) []storedTransaction {
	result := make([]storedTransaction, 0, len(records))
	for _, record := range records {
		if keep(record) {
			result = append(result, record)
		}
	}
	return result
}

func analyticsStats(records []storedTransaction, baseCurrency string) *store.Stats {
	result := &store.Stats{BaseCurrency: baseCurrency, TotalByCategory: map[string]float64{}, TotalCategoryCount: map[string]int{}}
	var baseTotal scaledAmountTotal
	categoryTotals := map[string]*scaledAmountTotal{}
	for _, record := range records {
		result.TotalCount++
		if record.transaction.Currency == baseCurrency {
			baseTotal.add(record.amount)
		}
		category := analyticsName(record.transaction.Category)
		if categoryTotals[category] == nil {
			categoryTotals[category] = &scaledAmountTotal{}
		}
		categoryTotals[category].add(record.amount)
		result.TotalCategoryCount[category]++
	}
	result.TotalBase = baseTotal.float64()
	for category, total := range categoryTotals {
		result.TotalByCategory[category] = total.float64()
	}
	return result
}

func analyticsCharts(records []storedTransaction, location *time.Location, now time.Time) *store.ChartData {
	result := &store.ChartData{
		MonthlySpend: []store.TimeBucket{}, DailySpend: []store.TimeBucket{}, ByCategory: map[string]float64{},
		ByBucket: map[string]float64{}, ByLabel: map[string]float64{}, BySource: map[string]float64{},
		BySourceType: map[string]float64{}, ByBank: map[string]float64{}, ByCategoryMonthly: map[string]store.CategoryMonthlyEntry{},
	}
	monthly := map[string]*store.TimeBucket{}
	daily := map[string]*store.TimeBucket{}
	for _, record := range records {
		transaction := record.transaction
		local := transaction.Timestamp.In(location)
		if !transaction.Timestamp.Before(now.AddDate(-1, 0, 0)) {
			addTimeBucket(monthly, local.Format("2006-01"), transaction.Amount)
		}
		if !transaction.Timestamp.Before(now.AddDate(0, 0, -30)) {
			addTimeBucket(daily, local.Format(time.DateOnly), transaction.Amount)
		}
		result.ByCategory[analyticsName(transaction.Category)] += transaction.Amount
		result.ByBucket[analyticsName(transaction.Bucket)] += transaction.Amount
		if len(transaction.Labels) == 0 {
			result.ByLabel["Uncategorized"] += transaction.Amount
		} else {
			for _, label := range transaction.Labels {
				result.ByLabel[analyticsName(label)] += transaction.Amount
			}
		}
		if source := transaction.Source.Display(); source != "" {
			result.BySource[source] += transaction.Amount
		}
		if transaction.Source.Type != "" {
			result.BySourceType[transaction.Source.Type] += transaction.Amount
		}
		if transaction.Source.Bank != "" {
			result.ByBank[transaction.Source.Bank] += transaction.Amount
		}
	}
	result.MonthlySpend = sortedTimeBuckets(monthly)
	result.DailySpend = sortedTimeBuckets(daily)
	result.ByCategoryMonthly = categoryMonthly(records, location, now)
	return result
}

func categoryMonthly(records []storedTransaction, location *time.Location, now time.Time) map[string]store.CategoryMonthlyEntry {
	result := map[string]store.CategoryMonthlyEntry{}
	localNow := now.In(location)
	current := localNow.Format("2006-01")
	prior := localNow.AddDate(0, -1, 0).Format("2006-01")
	for _, record := range records {
		month := record.transaction.Timestamp.In(location).Format("2006-01")
		if month != current && month != prior {
			continue
		}
		category := analyticsName(record.transaction.Category)
		entry := result[category]
		if month == current {
			entry.Current += record.transaction.Amount
		} else {
			entry.Prior += record.transaction.Amount
		}
		result[category] = entry
	}
	return result
}

func analyticsName(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Uncategorized"
	}
	return value
}

func addTimeBucket(values map[string]*store.TimeBucket, period string, amount float64) {
	if values[period] == nil {
		values[period] = &store.TimeBucket{Period: period}
	}
	values[period].Amount += amount
	values[period].Count++
}

func sortedTimeBuckets(values map[string]*store.TimeBucket) []store.TimeBucket {
	result := make([]store.TimeBucket, 0, len(values))
	for _, value := range values {
		result = append(result, *value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Period < result[j].Period })
	return result
}
