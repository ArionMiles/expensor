import { useChartData, useStatus, useTransactions } from '@/api/queries'
import type { ChartData, TimeBucket } from '@/api/types'
import { DaemonStatusBar } from '@/components/DaemonStatusBar'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { cn, formatCurrency, formatRelative } from '@/lib/utils'
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
        <h3 className="text-xs text-muted-foreground uppercase tracking-wider mb-3">{title}</h3>
        <p className="text-xs text-muted-foreground py-4 text-center">No data</p>
      </div>
    )
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
      <h3 className="text-xs text-muted-foreground uppercase tracking-wider mb-4">{title}</h3>
      <div className="flex items-start gap-4">
        <div className="flex-shrink-0">
          <DonutChart data={slices} size={100} />
        </div>
        <div className="flex-1 min-w-0 space-y-1.5">
          {slices.map((s) => (
            <div key={s.label} className="flex items-center gap-2">
              <span
                className="w-2 h-2 rounded-full flex-shrink-0"
                style={{ backgroundColor: s.color }}
              />
              <span className="text-xs text-muted-foreground truncate flex-1">{s.label}</span>
              <span className="text-xs font-mono text-foreground flex-shrink-0">
                {total > 0 ? `${Math.round((s.value / total) * 100)}%` : '—'}
              </span>
            </div>
          ))}
        </div>
      </div>
      {currency && (
        <p className="text-xs text-muted-foreground mt-3 pt-3 border-t border-border">
          Total: <span className="font-mono text-foreground">{formatCurrency(total, currency)}</span>
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
        <h3 className="text-xs text-muted-foreground uppercase tracking-wider mb-3">{title}</h3>
        <p className="text-xs text-muted-foreground py-4 text-center">No data</p>
      </div>
    )
  }

  const maxAmount = Math.max(...data.map((d) => d.amount), 1)
  // Show at most 12 bars, prioritize recent periods
  const visible = data.slice(-12)

  return (
    <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
      <h3 className="text-xs text-muted-foreground uppercase tracking-wider mb-4">{title}</h3>
      <div className="flex items-end gap-1 h-24">
        {visible.map((d) => {
          const pct = (d.amount / maxAmount) * 100
          return (
            <div
              key={d.period}
              className="flex-1 flex flex-col items-center gap-1 group"
              title={`${labelFormat(d.period)}: ${d.amount.toLocaleString('en-IN')}`}
            >
              <div className="w-full flex items-end" style={{ height: '80px' }}>
                <div
                  className="w-full bg-primary/70 rounded-t-sm group-hover:bg-primary transition-colors"
                  style={{ height: `${Math.max(pct, 2)}%` }}
                />
              </div>
              <span className="text-[8px] text-muted-foreground truncate w-full text-center">
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
      <span className="w-28 text-xs text-muted-foreground truncate flex-shrink-0">{category}</span>
      <div className="flex-1 h-1.5 bg-secondary rounded-full overflow-hidden">
        <div className="h-full bg-primary rounded-full" style={{ width: `${pct}%` }} />
      </div>
      <span className="w-24 text-right text-xs font-mono text-primary flex-shrink-0">
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
              className="rounded-lg border border-border bg-card p-4 shadow-sm animate-pulse"
            >
              <div className="h-3 w-32 bg-secondary rounded mb-3" />
              <div className="h-8 w-20 bg-secondary rounded" />
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
      <div className="rounded-lg border border-border bg-card p-8 text-center space-y-4 shadow-sm">
        <p className="text-sm text-muted-foreground">No data yet.</p>
        <Link
          to="/setup"
          className="inline-block px-4 py-2 text-sm rounded-md bg-primary text-primary-foreground hover:bg-primary/90 transition-colors"
        >
          Get started →
        </Link>
      </div>
    )
  }

  if (!stats) {
    return <div className="text-xs text-muted-foreground py-4">No stats available yet</div>
  }

  const sortedCategories = Object.entries(stats.total_by_category ?? {})
    .sort(([, a], [, b]) => b - a)
    .slice(0, 10)
  const maxCategoryAmount = sortedCategories[0]?.[1] ?? 1

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
          <p className="text-xs text-muted-foreground uppercase tracking-wider mb-2">
            Total transactions
          </p>
          <p className="text-3xl font-semibold text-foreground tabular-nums">
            {stats.total_count.toLocaleString('en-IN')}
          </p>
        </div>
        <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
          <p className="text-xs text-muted-foreground uppercase tracking-wider mb-2">
            Total spend (INR)
          </p>
          <p className="text-2xl font-semibold text-primary font-mono tabular-nums break-all">
            {formatCurrency(stats.total_inr, 'INR')}
          </p>
        </div>
      </div>

      {sortedCategories.length > 0 && (
        <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
          <h3 className="text-xs text-muted-foreground uppercase tracking-wider mb-4">
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
      <h2 className="text-xs text-muted-foreground uppercase tracking-wider">Charts</h2>

      {/* Time-series bars */}
      {(hasMonthly || hasDaily) && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
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
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {hasCategory && (
            <BreakdownChart title="By category" data={charts.by_category} />
          )}
          {hasBucket && (
            <BreakdownChart title="By bucket" data={charts.by_bucket} />
          )}
          {hasLabel && (
            <BreakdownChart title="By label" data={charts.by_label} />
          )}
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
          <div key={i} className="py-2.5 flex items-start justify-between gap-2 animate-pulse">
            <div className="min-w-0 flex-1 space-y-1">
              <div className="h-3 w-32 bg-secondary rounded" />
              <div className="h-2.5 w-20 bg-secondary rounded" />
            </div>
            <div className="h-3 w-16 bg-secondary rounded" />
          </div>
        ))}
      </div>
    )
  }

  if (error) {
    return <p className="text-xs text-destructive py-4">Failed to load transactions</p>
  }

  const transactions = data?.transactions ?? []

  if (transactions.length === 0) {
    return <p className="text-xs text-muted-foreground py-4">No transactions yet</p>
  }

  return (
    <div className="divide-y divide-border">
      {transactions.map((tx) => (
        <div key={tx.id} className="py-2.5 flex items-start justify-between gap-2">
          <div className="min-w-0 flex-1">
            <p className="text-sm text-foreground truncate">{tx.merchant_info}</p>
            <p className="text-xs text-muted-foreground">
              {tx.category} · {formatRelative(tx.timestamp)}
            </p>
          </div>
          <span className="text-sm font-mono text-primary flex-shrink-0 tabular-nums">
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

// ─── Nav ──────────────────────────────────────────────────────────────────────

function NavLink({
  to,
  children,
  active,
}: {
  to: string
  children: React.ReactNode
  active?: boolean
}) {
  return (
    <Link
      to={to}
      className={cn(
        'text-xs transition-colors',
        active ? 'text-foreground font-medium' : 'text-muted-foreground hover:text-foreground',
      )}
    >
      {children}
    </Link>
  )
}

// ─── Dashboard ────────────────────────────────────────────────────────────────

export function Dashboard() {
  const { data: chartData } = useChartData()

  return (
    <div className="min-h-screen bg-background flex flex-col">
      <header className="border-b border-border px-6 py-3 flex items-center justify-between bg-card">
        <Link
          to="/"
          className="text-sm font-semibold text-primary tracking-wide hover:text-primary/80 transition-colors"
        >
          Expensor
        </Link>
        <nav className="flex items-center gap-4">
          <NavLink to="/transactions">Transactions</NavLink>
          <NavLink to="/setup">Setup</NavLink>
        </nav>
      </header>

      <DaemonStatusBar />

      <main className="flex-1 px-6 py-6 max-w-6xl mx-auto w-full space-y-6">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="lg:col-span-2">
            <ErrorBoundary>
              <StatsSection />
            </ErrorBoundary>
          </div>

          <div className="lg:col-span-1">
            <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-xs text-muted-foreground uppercase tracking-wider">
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
      </main>
    </div>
  )
}

export default Dashboard
