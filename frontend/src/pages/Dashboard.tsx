import { useState } from 'react'
import { useChartData, useHeatmapData, useStatus, useTransactions } from '@/api/queries'
import type { ChartData, TimeBucket } from '@/api/types'
import { AnnualCalendarHeatmap } from '@/components/AnnualCalendarHeatmap'
import { HeatmapLegend } from '@/components/HeatmapLegend'
import { WeekdayHourHeatmap } from '@/components/WeekdayHourHeatmap'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { formatCurrency, formatRelative } from '@/lib/utils'
import { Link } from 'react-router-dom'

// ─── Chart palette ───────────────────────────────────────────────────────────

const CHART_COLORS = [
  '#3b82f6', // blue
  '#8b5cf6', // violet
  '#06b6d4', // cyan
  '#10b981', // emerald
  '#f59e0b', // amber
  '#ec4899', // pink
  '#6366f1', // indigo
  '#14b8a6', // teal
  '#f97316', // orange
  '#84cc16', // lime
]

function chartColor(index: number): string {
  return CHART_COLORS[index % CHART_COLORS.length]
}

// ─── Donut chart (SVG) ───────────────────────────────────────────────────────

interface DonutSlice {
  label: string
  value: number
  color: string
}

function DonutChart({ data, size = 120 }: { data: DonutSlice[]; size?: number }) {
  const total = data.reduce((sum, d) => sum + d.value, 0)
  if (total === 0) return null

  const r = size * 0.38
  const strokeWidth = size * 0.14
  const cx = size / 2
  const cy = size / 2
  const C = 2 * Math.PI * r

  let cumulativeOffset = 0
  const slices = data.map((d) => {
    const length = (d.value / total) * C
    const slice = { ...d, length, offset: cumulativeOffset }
    cumulativeOffset += length
    return slice
  })

  return (
    <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} aria-hidden="true">
      <g transform={`rotate(-90, ${cx}, ${cy})`}>
        {slices.map((s, i) => (
          <circle
            key={`${s.label}-${i}`}
            cx={cx}
            cy={cy}
            r={r}
            fill="none"
            stroke={s.color}
            strokeWidth={strokeWidth}
            strokeDasharray={`${s.length} ${C - s.length}`}
            strokeDashoffset={C - s.offset}
          />
        ))}
      </g>
    </svg>
  )
}

// ─── Breakdown chart (donut + legend) ────────────────────────────────────────

function BreakdownChart({
  title,
  data,
  currency,
}: {
  title: string
  data: Record<string, number>
  currency?: string
}) {
  const entries = Object.entries(data)
    .sort(([, a], [, b]) => b - a)
    .slice(0, 8)

  const slices: DonutSlice[] = entries.map(([label, value], i) => ({
    label,
    value,
    color: chartColor(i),
  }))

  const total = slices.reduce((sum, s) => sum + s.value, 0)

  if (entries.length === 0) {
    return (
      <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
        <h3 className="mb-3 text-xs uppercase tracking-wider text-muted-foreground">{title}</h3>
        <p className="py-4 text-center text-xs text-muted-foreground">No data</p>
      </div>
    )
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
      <h3 className="mb-4 text-xs uppercase tracking-wider text-muted-foreground">{title}</h3>
      <div className="flex items-start gap-4">
        <div className="flex-shrink-0">
          <DonutChart data={slices} size={100} />
        </div>
        <div className="min-w-0 flex-1 space-y-1.5">
          {slices.map((s) => (
            <div key={s.label} className="flex items-center gap-2">
              <span
                className="h-2 w-2 flex-shrink-0 rounded-full"
                style={{ backgroundColor: s.color }}
              />
              <span className="flex-1 truncate text-xs text-muted-foreground">{s.label}</span>
              <span className="flex-shrink-0 font-mono text-xs text-foreground">
                {total > 0 ? `${Math.round((s.value / total) * 100)}%` : '—'}
              </span>
            </div>
          ))}
        </div>
      </div>
      {currency && (
        <p className="mt-3 border-t border-border pt-3 text-xs text-muted-foreground">
          Total:{' '}
          <span className="font-mono text-foreground">{formatCurrency(total, currency)}</span>
        </p>
      )}
    </div>
  )
}

// ─── Bar chart (CSS) ─────────────────────────────────────────────────────────

function BarChart({
  title,
  data,
  labelFormat,
}: {
  title: string
  data: TimeBucket[]
  labelFormat: (period: string) => string
}) {
  if (data.length === 0) {
    return (
      <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
        <h3 className="mb-3 text-xs uppercase tracking-wider text-muted-foreground">{title}</h3>
        <p className="py-4 text-center text-xs text-muted-foreground">No data</p>
      </div>
    )
  }

  const maxAmount = Math.max(...data.map((d) => d.amount), 1)
  // Show at most 12 bars, prioritize recent periods
  const visible = data.slice(-12)

  return (
    <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
      <h3 className="mb-4 text-xs uppercase tracking-wider text-muted-foreground">{title}</h3>
      <div className="flex h-24 items-end gap-1">
        {visible.map((d) => {
          const pct = (d.amount / maxAmount) * 100
          return (
            <div
              key={d.period}
              className="group flex flex-1 flex-col items-center gap-1"
              title={`${labelFormat(d.period)}: ${d.amount.toLocaleString('en-IN')}`}
            >
              <div className="flex w-full items-end" style={{ height: '80px' }}>
                <div
                  className="w-full rounded-t-sm bg-primary/70 transition-colors group-hover:bg-primary"
                  style={{ height: `${Math.max(pct, 2)}%` }}
                />
              </div>
              <span className="w-full truncate text-center text-[8px] text-muted-foreground">
                {labelFormat(d.period)}
              </span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

// ─── Stats section ───────────────────────────────────────────────────────────

function CategoryBar({
  category,
  amount,
  maxAmount,
}: {
  category: string
  amount: number
  maxAmount: number
}) {
  const pct = maxAmount > 0 ? (amount / maxAmount) * 100 : 0
  return (
    <div className="flex items-center gap-3 py-1.5">
      <span className="w-28 flex-shrink-0 truncate text-xs text-muted-foreground">{category}</span>
      <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-secondary">
        <div className="h-full rounded-full bg-primary" style={{ width: `${pct}%` }} />
      </div>
      <span className="w-24 flex-shrink-0 text-right font-mono text-xs text-primary">
        {formatCurrency(amount, 'INR')}
      </span>
    </div>
  )
}

function StatsSection() {
  const { data, isLoading, error } = useStatus()

  if (isLoading) {
    return (
      <div className="space-y-6">
        <div className="grid grid-cols-2 gap-4">
          {[0, 1].map((i) => (
            <div
              key={i}
              className="animate-pulse rounded-lg border border-border bg-card p-4 shadow-sm"
            >
              <div className="mb-3 h-3 w-32 rounded bg-secondary" />
              <div className="h-8 w-20 rounded bg-secondary" />
            </div>
          ))}
        </div>
      </div>
    )
  }

  if (error || !data) {
    return (
      <div className="rounded-lg border border-destructive bg-card p-4">
        <p className="text-xs text-destructive">Failed to load stats</p>
      </div>
    )
  }

  const { stats, daemon } = data

  if (!daemon.running && !stats) {
    return (
      <div className="space-y-4 rounded-lg border border-border bg-card p-8 text-center shadow-sm">
        <p className="text-sm text-muted-foreground">No data yet.</p>
        <Link
          to="/setup"
          className="inline-block rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground transition-colors hover:bg-primary/90"
        >
          Get started →
        </Link>
      </div>
    )
  }

  if (!stats) {
    return <div className="py-4 text-xs text-muted-foreground">No stats available yet</div>
  }

  const sortedCategories = Object.entries(stats.total_by_category ?? {})
    .sort(([, a], [, b]) => b - a)
    .slice(0, 10)
  const maxCategoryAmount = sortedCategories[0]?.[1] ?? 1

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
          <p className="mb-2 text-xs uppercase tracking-wider text-muted-foreground">
            Total transactions
          </p>
          <p className="text-3xl font-semibold tabular-nums text-foreground">
            {stats.total_count.toLocaleString('en-IN')}
          </p>
        </div>
        <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
          <p className="mb-2 text-xs uppercase tracking-wider text-muted-foreground">
            Total spend ({stats.base_currency})
          </p>
          <p className="break-all font-mono text-2xl font-semibold tabular-nums text-primary">
            {formatCurrency(stats.total_base, stats.base_currency)}
          </p>
        </div>
      </div>

      {sortedCategories.length > 0 && (
        <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
          <h3 className="mb-4 text-xs uppercase tracking-wider text-muted-foreground">
            Spend by category
          </h3>
          <div className="divide-y divide-border">
            {sortedCategories.map(([category, amount]) => (
              <CategoryBar
                key={category}
                category={category}
                amount={amount}
                maxAmount={maxCategoryAmount}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

// ─── Month navigation ─────────────────────────────────────────────────────────

interface MonthNav {
  year: number
  month: number
}

const MONTH_NAMES = [
  'Jan',
  'Feb',
  'Mar',
  'Apr',
  'May',
  'Jun',
  'Jul',
  'Aug',
  'Sep',
  'Oct',
  'Nov',
  'Dec',
]

function monthRangeISO(nav: MonthNav): { from: string; to: string } {
  const from = new Date(Date.UTC(nav.year, nav.month - 1, 1))
  const to = new Date(Date.UTC(nav.year, nav.month, 0, 23, 59, 59))
  return {
    from: from.toISOString().split('.')[0] + 'Z',
    to: to.toISOString().split('.')[0] + 'Z',
  }
}

function prevMonth(n: MonthNav): MonthNav {
  return n.month === 1 ? { year: n.year - 1, month: 12 } : { year: n.year, month: n.month - 1 }
}

function nextMonth(n: MonthNav): MonthNav {
  return n.month === 12 ? { year: n.year + 1, month: 1 } : { year: n.year, month: n.month + 1 }
}

// ─── Metric toggle ────────────────────────────────────────────────────────────

function MetricToggle({
  value,
  onChange,
}: {
  value: 'amount' | 'count'
  onChange: (v: 'amount' | 'count') => void
}) {
  return (
    <div className="flex rounded-md border border-border text-xs">
      {(['amount', 'count'] as const).map((opt) => (
        <button
          key={opt}
          onClick={() => onChange(opt)}
          className={[
            'px-2 py-0.5 capitalize transition-colors first:rounded-l-md last:rounded-r-md',
            value === opt
              ? 'bg-primary text-primary-foreground'
              : 'text-muted-foreground hover:text-foreground',
          ].join(' ')}
        >
          {opt}
        </button>
      ))}
    </div>
  )
}

// ─── Spending patterns ────────────────────────────────────────────────────────

function SpendingPatternsSection() {
  const [metric, setMetric] = useState<'amount' | 'count'>('amount')
  const now = new Date()
  const [monthNav, setMonthNav] = useState<MonthNav | null>(null)
  const currentYear = now.getFullYear()
  const [year, setYear] = useState(currentYear)

  const dateRange = monthNav ? monthRangeISO(monthNav) : undefined
  const { data: heatmap, isLoading } = useHeatmapData(dateRange?.from, dateRange?.to)

  const monthLabel = monthNav ? `${MONTH_NAMES[monthNav.month - 1]} ${monthNav.year}` : undefined

  const isCurrentMonth =
    monthNav !== null &&
    monthNav.year === now.getFullYear() &&
    monthNav.month === now.getMonth() + 1

  if (isLoading) {
    return <div className="h-40 animate-pulse rounded-lg border border-border bg-card shadow-sm" />
  }

  if (!heatmap) return null

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-xs uppercase tracking-wider text-muted-foreground">
          Spending Patterns
        </h2>
        <MetricToggle value={metric} onChange={setMetric} />
      </div>

      <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
        <div className="mb-3 flex items-center justify-between">
          <h3 className="text-xs uppercase tracking-wider text-muted-foreground">
            By weekday &amp; hour
          </h3>
          <div className="flex items-center gap-1.5">
            {monthNav !== null && (
              <button
                onClick={() => setMonthNav(null)}
                className="rounded border border-border px-1.5 py-0.5 text-xs text-muted-foreground hover:text-foreground"
              >
                All time
              </button>
            )}
            <button
              onClick={() =>
                setMonthNav((p) =>
                  prevMonth(p ?? { year: now.getFullYear(), month: now.getMonth() + 1 }),
                )
              }
              className="rounded border border-border px-1.5 py-0.5 text-xs text-muted-foreground hover:text-foreground"
              aria-label="Previous month"
            >
              ←
            </button>
            <span className="min-w-[5rem] text-center text-xs font-medium text-foreground">
              {monthLabel ?? 'All time'}
            </span>
            <button
              onClick={() =>
                setMonthNav((p) =>
                  nextMonth(p ?? { year: now.getFullYear(), month: now.getMonth() + 1 }),
                )
              }
              disabled={isCurrentMonth}
              className="rounded border border-border px-1.5 py-0.5 text-xs text-muted-foreground hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40"
              aria-label="Next month"
            >
              →
            </button>
          </div>
        </div>
        <WeekdayHourHeatmap
          data={heatmap.by_weekday_hour}
          metric={metric}
          monthLabel={monthLabel}
        />
      </div>
      <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
        <div className="mb-3 flex items-center justify-between">
          <h3 className="text-xs uppercase tracking-wider text-muted-foreground">By day of year</h3>
          <div className="flex items-center gap-1.5">
            <button
              onClick={() => setYear((y) => y - 1)}
              className="rounded border border-border px-1.5 py-0.5 text-xs text-muted-foreground hover:text-foreground"
              aria-label="Previous year"
            >
              ←
            </button>
            <span className="min-w-[3rem] text-center text-xs font-medium tabular-nums text-foreground">
              {year}
            </span>
            <button
              onClick={() => setYear((y) => y + 1)}
              disabled={year >= currentYear}
              className="rounded border border-border px-1.5 py-0.5 text-xs text-muted-foreground hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40"
              aria-label="Next year"
            >
              →
            </button>
          </div>
        </div>
        <AnnualCalendarHeatmap year={year} metric={metric} />
      </div>
      <HeatmapLegend />
    </div>
  )
}

// ─── Chart section ────────────────────────────────────────────────────────────

function formatMonthLabel(period: string): string {
  // period = "2024-01"
  const [year, month] = period.split('-')
  const d = new Date(Number(year), Number(month) - 1, 1)
  return d.toLocaleString('en-IN', { month: 'short' })
}

function formatDayLabel(period: string): string {
  // period = "2024-01-15"
  const d = new Date(period)
  return `${d.getDate()}`
}

function ChartsSection({ charts }: { charts: ChartData }) {
  const hasMonthly = charts.monthly_spend.length > 0
  const hasDaily = charts.daily_spend.length > 0
  const hasCategory = Object.keys(charts.by_category).length > 0
  const hasBucket = Object.keys(charts.by_bucket).length > 0
  const hasLabel = Object.keys(charts.by_label).length > 0

  if (!hasMonthly && !hasDaily && !hasCategory && !hasBucket && !hasLabel) {
    return null
  }

  return (
    <div className="space-y-4">
      <h2 className="text-xs uppercase tracking-wider text-muted-foreground">Charts</h2>

      {/* Time-series bars */}
      {(hasMonthly || hasDaily) && (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
          {hasMonthly && (
            <BarChart
              title="Monthly spend (12 months)"
              data={charts.monthly_spend}
              labelFormat={formatMonthLabel}
            />
          )}
          {hasDaily && (
            <BarChart
              title="Daily spend (30 days)"
              data={charts.daily_spend}
              labelFormat={formatDayLabel}
            />
          )}
        </div>
      )}

      {/* Breakdown donuts */}
      {(hasCategory || hasBucket || hasLabel) && (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
          {hasCategory && <BreakdownChart title="By category" data={charts.by_category} />}
          {hasBucket && <BreakdownChart title="By bucket" data={charts.by_bucket} />}
          {hasLabel && <BreakdownChart title="By label" data={charts.by_label} />}
        </div>
      )}
    </div>
  )
}

// ─── Recent transactions ──────────────────────────────────────────────────────

function RecentTransactions() {
  const { data, isLoading, error } = useTransactions({ page: 1, page_size: 5 }, '')

  if (isLoading) {
    return (
      <div className="space-y-3">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="flex animate-pulse items-start justify-between gap-2 py-2.5">
            <div className="min-w-0 flex-1 space-y-1">
              <div className="h-3 w-32 rounded bg-secondary" />
              <div className="h-2.5 w-20 rounded bg-secondary" />
            </div>
            <div className="h-3 w-16 rounded bg-secondary" />
          </div>
        ))}
      </div>
    )
  }

  if (error) {
    return <p className="py-4 text-xs text-destructive">Failed to load transactions</p>
  }

  const transactions = data?.transactions ?? []

  if (transactions.length === 0) {
    return <p className="py-4 text-xs text-muted-foreground">No transactions yet</p>
  }

  return (
    <div className="divide-y divide-border">
      {transactions.map((tx) => (
        <div key={tx.id} className="flex items-start justify-between gap-2 py-2.5">
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm text-foreground">{tx.merchant_info}</p>
            <p className="text-xs text-muted-foreground">
              {tx.category} · {formatRelative(tx.timestamp)}
            </p>
          </div>
          <span className="flex-shrink-0 font-mono text-sm tabular-nums text-primary">
            {formatCurrency(tx.amount, tx.currency)}
          </span>
        </div>
      ))}
      <div className="pt-3">
        <Link to="/transactions" className="text-xs text-primary hover:underline">
          View all transactions →
        </Link>
      </div>
    </div>
  )
}

// ─── Dashboard ────────────────────────────────────────────────────────────────

export function Dashboard() {
  const { data: chartData } = useChartData()

  return (
    <div className="mx-auto w-full max-w-6xl space-y-6 px-6 py-6">
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2">
          <ErrorBoundary>
            <StatsSection />
          </ErrorBoundary>
        </div>
        <div className="lg:col-span-1">
          <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-xs uppercase tracking-wider text-muted-foreground">
                Recent transactions
              </h2>
            </div>
            <ErrorBoundary>
              <RecentTransactions />
            </ErrorBoundary>
          </div>
        </div>
      </div>
      {chartData && (
        <ErrorBoundary>
          <ChartsSection charts={chartData} />
        </ErrorBoundary>
      )}
      <ErrorBoundary>
        <SpendingPatternsSection />
      </ErrorBoundary>
    </div>
  )
}

export default Dashboard
