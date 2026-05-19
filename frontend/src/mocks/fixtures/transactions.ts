import type {
  AnnualHeatmapData,
  ChartData,
  DashboardData,
  HeatmapData,
  MonthlyBreakdownData,
  Transaction,
} from '@/api/types'

export const seededTransactions: Transaction[] = [
  {
    id: 'tx-1',
    message_id: 'msg-1',
    amount: 1250,
    currency: 'USD',
    timestamp: '2026-04-10T12:30:00Z',
    merchant_info: 'Corner Coffee',
    category: 'Food',
    bucket: 'Needs',
    source: 'gmail',
    description: 'Coffee beans',
    labels: ['Groceries'],
    muted: false,
    muted_by_merchant: false,
    created_at: '2026-04-10T12:30:00Z',
    updated_at: '2026-04-10T12:30:00Z',
  },
  {
    id: 'tx-2',
    message_id: 'msg-2',
    amount: 240000,
    currency: 'USD',
    timestamp: '2026-04-01T08:00:00Z',
    merchant_info: 'City Apartments',
    category: 'Housing',
    bucket: 'Needs',
    source: 'thunderbird',
    description: 'April rent',
    labels: ['Rent'],
    muted: false,
    muted_by_merchant: false,
    created_at: '2026-04-01T08:00:00Z',
    updated_at: '2026-04-01T08:00:00Z',
  },
]

export const seededFacets = {
  sources: ['gmail', 'thunderbird'],
  categories: ['Food', 'Housing'],
  category_counts: {
    Food: 1,
    Housing: 1,
  },
  currencies: ['USD'],
  merchants: ['City Apartments', 'Corner Coffee'],
  labels: ['Groceries', 'Rent'],
  label_counts: {
    Groceries: 1,
    Rent: 1,
  },
  buckets: ['Needs'],
  bucket_counts: {
    Needs: 2,
  },
}

const sharedCharts: ChartData = {
  monthly_spend: [
    { period: '2026-03', amount: 95000, count: 1 },
    { period: '2026-04', amount: 241250, count: 2 },
  ],
  daily_spend: [
    { period: '2026-04-01', amount: 240000, count: 1 },
    { period: '2026-04-10', amount: 1250, count: 1 },
  ],
  by_category: {
    Food: 1250,
    Housing: 240000,
  },
  by_bucket: {
    Needs: 241250,
  },
  by_label: {
    Groceries: 1250,
    Rent: 240000,
  },
  by_source: {
    gmail: 1250,
    thunderbird: 240000,
  },
  by_category_monthly: {
    Food: { current: 1250, prior: 950 },
    Housing: { current: 240000, prior: 238000 },
  },
}

export const dashboardData: DashboardData = {
  current_month: {
    label: 'April 2026',
    stats: {
      total_count: 2,
      total_base: 241250,
      base_currency: 'USD',
      total_by_category: {
        Food: 1250,
        Housing: 240000,
      },
      total_category_count: {
        Food: 1,
        Housing: 1,
      },
      top_merchants: [
        { merchant: 'City Apartments', amount: 240000, count: 1 },
        { merchant: 'Corner Coffee', amount: 1250, count: 1 },
      ],
    },
    charts: sharedCharts,
  },
  all_time: {
    label: 'All time',
    stats: {
      total_count: 2,
      total_base: 241250,
      base_currency: 'USD',
      total_by_category: {
        Food: 1250,
        Housing: 240000,
      },
      total_category_count: {
        Food: 1,
        Housing: 1,
      },
      top_merchants: [
        { merchant: 'City Apartments', amount: 240000, count: 1 },
        { merchant: 'Corner Coffee', amount: 1250, count: 1 },
      ],
    },
    charts: sharedCharts,
  },
}

export const heatmapData: HeatmapData = {
  by_weekday_hour: [
    { weekday: 2, hour: 9, amount: 1250, count: 1 },
    { weekday: 4, hour: 18, amount: 240000, count: 1 },
  ],
  by_day_of_month: [
    { day: 1, amount: 240000, count: 1 },
    { day: 10, amount: 1250, count: 1 },
  ],
}

export const monthlyBreakdownData: MonthlyBreakdownData = {
  labels: ['Groceries', 'Rent'],
  months: ['2026-03', '2026-04'],
  series: [
    { label: 'Groceries', data: [950, 1250] },
    { label: 'Rent', data: [238000, 240000] },
  ],
}

export function buildAnnualHeatmapData(year: number): AnnualHeatmapData {
  return {
    year,
    buckets: [
      { date: `${year}-04-01`, amount: 240000, count: 1 },
      { date: `${year}-04-10`, amount: 1250, count: 1 },
    ],
  }
}
