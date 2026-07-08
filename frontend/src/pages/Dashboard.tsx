import { Link, useSearchParams } from 'react-router-dom'
import { useDashboardData, useMonthlyBreakdownSpend, useStatus, useTimezone } from '@/api/queries'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { useI18n } from '@/i18n/I18nProvider'
import {
  BreakdownTimeline,
  ChartsSection,
  DASHBOARD_PREF_KEYS,
  DEFAULT_HEATMAP_METRIC,
  DEFAULT_SPEND_BREAKDOWN_MODE,
  RecentTransactions,
  SUMMARY_MODE_OPTIONS,
  SpendingPatternsSection,
  SummaryOverviewCard,
  SummarySection,
  buildMonthRangeParams,
  buildZonedDayRangeParams,
  dashboardBreakdownData,
  dashboardBreakdownParams,
  displayBucketLabel,
  isSpendBreakdownMode,
  isSummaryMode,
  readDashboardPreference,
  topBreakdownSlices,
  writeDashboardPreference,
} from './dashboard/DashboardSections'
import type { SpendBreakdownMode, SummaryMode } from './dashboard/DashboardSections'

export {
  BreakdownTimeline,
  DEFAULT_HEATMAP_METRIC,
  DEFAULT_SPEND_BREAKDOWN_MODE,
  SummarySection,
  buildZonedDayRangeParams,
  dashboardBreakdownData,
  dashboardBreakdownParams,
  displayBucketLabel,
  topBreakdownSlices,
}

// ─── Dashboard ────────────────────────────────────────────────────────────────

export default function Dashboard() {
  const [searchParams, setSearchParams] = useSearchParams()
  const breakdownParam = searchParams.get('breakdown')
  const storedBreakdownMode = readDashboardPreference(DASHBOARD_PREF_KEYS.spendBreakdownMode)
  const breakdownMode: SpendBreakdownMode = isSpendBreakdownMode(breakdownParam)
    ? breakdownParam
    : isSpendBreakdownMode(storedBreakdownMode)
      ? storedBreakdownMode
      : DEFAULT_SPEND_BREAKDOWN_MODE
  const setBreakdownMode = (mode: SpendBreakdownMode) => {
    writeDashboardPreference(DASHBOARD_PREF_KEYS.spendBreakdownMode, mode)
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (mode === DEFAULT_SPEND_BREAKDOWN_MODE) next.delete('breakdown')
        else next.set('breakdown', mode)
        return next
      },
      { replace: true },
    )
  }
  const { locale } = useI18n()
  const { data: dashboardData, isLoading, error } = useDashboardData()
  const { data: monthlyBreakdown } = useMonthlyBreakdownSpend(breakdownMode)
  const { data: status } = useStatus()
  const { data: timezone } = useTimezone()

  if (isLoading) {
    return (
      <div className="mx-auto w-full max-w-6xl space-y-6 px-6 py-6">
        <div className="space-y-2">
          <div className="h-4 w-40 animate-pulse rounded bg-secondary" />
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
            <div className="space-y-4 lg:col-span-2">
              {[0, 1].map((i) => (
                <div
                  key={i}
                  className="h-40 animate-pulse rounded-lg border border-border bg-card shadow-sm"
                />
              ))}
            </div>
            <div className="h-64 animate-pulse rounded-lg border border-border bg-card shadow-sm" />
          </div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="mx-auto w-full max-w-6xl px-6 py-6">
        <div className="rounded-lg border border-destructive bg-card p-4">
          <p className="text-xs text-destructive">Failed to load dashboard</p>
        </div>
      </div>
    )
  }

  const daemon = status?.daemon
  const currentMonth = dashboardData?.current_month
  const allTime = dashboardData?.all_time
  const summaryParam = searchParams.get('summary')
  const storedSummaryMode = readDashboardPreference(DASHBOARD_PREF_KEYS.summaryMode)
  const requestedSummaryMode: SummaryMode = isSummaryMode(summaryParam)
    ? summaryParam
    : isSummaryMode(storedSummaryMode)
      ? storedSummaryMode
      : 'current_month'
  const effectiveSummaryMode: SummaryMode =
    requestedSummaryMode === 'current_month'
      ? currentMonth
        ? 'current_month'
        : 'all_time'
      : allTime
        ? 'all_time'
        : 'current_month'
  const fallbackCurrency =
    currentMonth?.stats.base_currency ??
    allTime?.stats.base_currency ??
    status?.stats?.base_currency ??
    'INR'
  const activeSummary =
    effectiveSummaryMode === 'current_month' ? (currentMonth ?? allTime) : (allTime ?? currentMonth)
  const summaryTimezone = timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone
  const currentMonthRange = currentMonth
    ? buildMonthRangeParams(currentMonth.label, summaryTimezone)
    : null
  const summaryModeLabels: Record<SummaryMode, string> = {
    current_month: currentMonth?.label ?? 'Current Month',
    all_time: 'All Time',
  }
  const setSummaryMode = (mode: SummaryMode) => {
    writeDashboardPreference(DASHBOARD_PREF_KEYS.summaryMode, mode)
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        next.set('summary', mode)
        return next
      },
      { replace: true },
    )
  }

  if (!currentMonth && !allTime && !daemon?.running) {
    return (
      <div className="mx-auto w-full max-w-6xl px-6 py-6">
        <div className="space-y-4 rounded-lg border border-border bg-card p-8 text-center shadow-sm">
          <p className="text-sm text-muted-foreground">No data yet.</p>
          <Link
            to="/setup"
            className="inline-block rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground transition-colors hover:bg-primary/90"
          >
            Get started →
          </Link>
        </div>
      </div>
    )
  }

  return (
    <div className="mx-auto w-full max-w-6xl space-y-6 px-6 py-6">
      {activeSummary && (
        <section className="space-y-4">
          <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div>
              <h1 className="text-sm font-semibold uppercase tracking-[0.18em] text-muted-foreground">
                Dashboard Summary
              </h1>
              <p className="mt-1 text-sm text-muted-foreground">{activeSummary.label}</p>
            </div>
            <div className="flex rounded-md border border-border bg-card p-0.5 text-xs">
              {SUMMARY_MODE_OPTIONS.map((option) => {
                const disabled =
                  (option === 'current_month' && !currentMonth) ||
                  (option === 'all_time' && !allTime)
                return (
                  <button
                    key={option}
                    type="button"
                    disabled={disabled}
                    onClick={() => setSummaryMode(option)}
                    className={[
                      'rounded px-3 py-1.5 transition-colors',
                      effectiveSummaryMode === option
                        ? 'bg-primary text-primary-foreground'
                        : 'text-muted-foreground hover:text-foreground',
                      disabled ? 'cursor-not-allowed opacity-50' : '',
                    ].join(' ')}
                  >
                    {summaryModeLabels[option]}
                  </button>
                )
              })}
            </div>
          </div>

          <div className="grid grid-cols-1 gap-4 xl:grid-cols-12">
            <div className="self-start xl:col-span-7">
              <ErrorBoundary>
                <SummaryOverviewCard
                  summary={activeSummary}
                  currency={fallbackCurrency}
                  locale={locale}
                  summaryMode={effectiveSummaryMode}
                  currentMonthRange={currentMonthRange}
                />
              </ErrorBoundary>
            </div>
            <div className="xl:col-span-5">
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
          <ErrorBoundary>
            <SummarySection
              summary={activeSummary}
              currency={fallbackCurrency}
              locale={locale}
              summaryMode={effectiveSummaryMode}
              currentMonthRange={currentMonthRange}
            />
          </ErrorBoundary>
          <ErrorBoundary>
            <ChartsSection
              charts={allTime?.charts ?? activeSummary.charts}
              currency={fallbackCurrency}
              locale={locale}
              monthlyTitle="Monthly spend (12 months)"
              dailyTitle="Daily spend (30 days)"
            />
          </ErrorBoundary>
        </section>
      )}

      {monthlyBreakdown && (
        <ErrorBoundary>
          <BreakdownTimeline
            data={monthlyBreakdown}
            currency={fallbackCurrency}
            mode={breakdownMode}
            onModeChange={setBreakdownMode}
          />
        </ErrorBoundary>
      )}

      <ErrorBoundary>
        <SpendingPatternsSection />
      </ErrorBoundary>
    </div>
  )
}
