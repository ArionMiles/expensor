import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import {
  useDashboardData,
  useHeatmapData,
  useMonthlyBreakdownSpend,
  useStatus,
  useTimezone,
  useTransactions,
} from '@/api/queries'
import type {
  CategoryMonthlyEntry,
  ChartData,
  MonthlyBreakdownData,
  Stats,
  TimeBucket,
} from '@/api/types'
import { AnnualCalendarHeatmap } from '@/components/AnnualCalendarHeatmap'
import { HeatmapLegend } from '@/components/HeatmapLegend'
import { WeekdayHourHeatmap } from '@/components/WeekdayHourHeatmap'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { formatCompactNumber, formatCurrency, formatDateShort, formatRelative } from '@/lib/utils'
import { formatMonthForLocale, formatNumberForLocale } from '@/i18n/format'
import { useI18n } from '@/i18n/I18nProvider'

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

const UNCATEGORIZED_LABEL = 'Uncategorized'

type BreakdownDimension = 'category' | 'bucket' | 'label' | 'source' | 'source_type' | 'bank'

type SpendBreakdownMode = 'labels' | 'categories' | 'buckets'

export const DEFAULT_SPEND_BREAKDOWN_MODE: SpendBreakdownMode = 'categories'
export const DEFAULT_HEATMAP_METRIC = 'count'

export function displayBucketLabel(label: string): string {
  return label
}

export function dashboardBreakdownData(
  data: Record<string, number>,
  dimension: BreakdownDimension = 'category',
): Record<string, number> {
  return Object.entries(data ?? {}).reduce<Record<string, number>>((acc, [rawLabel, value]) => {
    const normalized = rawLabel.trim() || UNCATEGORIZED_LABEL
    const label = dimension === 'bucket' ? displayBucketLabel(normalized) : normalized
    acc[label] = (acc[label] ?? 0) + value
    return acc
  }, {})
}

export function dashboardBreakdownParams(
  dimension: BreakdownDimension,
  label: string,
): Record<string, string> {
  if (label === UNCATEGORIZED_LABEL) {
    return { [`${dimension}_missing`]: '1', show_filters: '1' }
  }
  return { [dimension]: label, show_filters: '1' }
}

// ─── Donut chart (SVG) ───────────────────────────────────────────────────────

interface DonutSlice {
  label: string
  value: number
  color: string
}

type BreakdownSlice = DonutSlice

interface DonutTooltipState {
  x: number
  y: number
  label: string
  amount: number
  pct: number
  group?: string
}

function DonutChart({
  data,
  size = 120,
  currency,
  onSliceClick,
}: {
  data: DonutSlice[]
  size?: number
  currency: string
  onSliceClick?: (label: string) => void
}) {
  const [tooltip, setTooltip] = useState<DonutTooltipState | null>(null)

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
    <>
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
              style={{ cursor: onSliceClick ? 'pointer' : 'default' }}
              onMouseEnter={(e) =>
                setTooltip({
                  x: e.clientX + 12,
                  y: e.clientY + 12,
                  label: s.label,
                  amount: s.value,
                  pct: (s.value / total) * 100,
                })
              }
              onMouseMove={(e) =>
                setTooltip((prev) =>
                  prev ? { ...prev, x: e.clientX + 12, y: e.clientY + 12 } : prev,
                )
              }
              onMouseLeave={() => setTooltip(null)}
              onClick={() => onSliceClick?.(s.label)}
            />
          ))}
        </g>
      </svg>
      {tooltip &&
        createPortal(
          <div
            className="pointer-events-none fixed z-50 rounded border border-border bg-secondary px-2 py-1 text-xs shadow-lg"
            style={{ left: tooltip.x, top: tooltip.y }}
          >
            <span className="font-medium text-foreground">{tooltip.label}</span>
            <br />
            <span className="text-muted-foreground">
              {formatCurrency(tooltip.amount, currency)} · {tooltip.pct.toFixed(1)}%
            </span>
          </div>,
          document.body,
        )}
    </>
  )
}

export function topBreakdownSlices(data: Record<string, number>): BreakdownSlice[] {
  return Object.entries(data ?? {})
    .filter(([, value]) => value > 0)
    .sort(([, a], [, b]) => b - a)
    .map(([label, value], index) => ({
      label,
      value,
      color: chartColor(index),
    }))
}

// ─── Breakdown charts ────────────────────────────────────────────────────────

function BreakdownChart({
  title,
  data,
  currency,
  onSliceClick,
  embedded = false,
}: {
  title: string
  data: Record<string, number>
  currency: string
  onSliceClick?: (slice: BreakdownSlice) => void
  embedded?: boolean
}) {
  const slices = topBreakdownSlices(data)

  const total = slices.reduce((sum, s) => sum + s.value, 0)

  if (slices.length === 0) {
    return (
      <div
        className={[
          embedded ? '' : 'h-full',
          'flex min-h-[220px] flex-col rounded-lg border border-border bg-card p-4 shadow-sm',
        ].join(' ')}
      >
        <h3
          className={
            embedded
              ? 'mb-3 text-[10px] text-muted-foreground'
              : 'mb-3 text-xs uppercase tracking-wider text-muted-foreground'
          }
        >
          {title}
        </h3>
        <p className="flex flex-1 items-center justify-center py-4 text-center text-xs text-muted-foreground">
          No data
        </p>
      </div>
    )
  }

  return (
    <div
      className={[
        embedded ? '' : 'h-full',
        'flex min-h-[220px] flex-col rounded-lg border border-border bg-card p-4 shadow-sm',
      ].join(' ')}
    >
      <h3
        className={
          embedded
            ? 'mb-3 text-[10px] text-muted-foreground'
            : 'mb-3 text-xs uppercase tracking-wider text-muted-foreground'
        }
      >
        {title}
      </h3>
      <div className="flex flex-1 items-center justify-center py-2">
        <div className="flex flex-shrink-0 items-center justify-center">
          <DonutChart
            data={slices}
            size={156}
            currency={currency}
            onSliceClick={(label) => {
              const slice = slices.find((entry) => entry.label === label)
              if (slice) onSliceClick?.(slice)
            }}
          />
        </div>
      </div>
      <p className="mt-auto border-t border-border pt-3 text-xs text-muted-foreground">
        Total: <span className="font-mono text-foreground">{formatCurrency(total, currency)}</span>
      </p>
    </div>
  )
}

function ConcentricBreakdownChart({
  title,
  outerData,
  innerData,
  outerLabel,
  innerLabel,
  currency,
  onOuterSliceClick,
  onInnerSliceClick,
}: {
  title: string
  outerData: Record<string, number>
  innerData: Record<string, number>
  outerLabel: string
  innerLabel: string
  currency: string
  onOuterSliceClick?: (slice: BreakdownSlice) => void
  onInnerSliceClick?: (slice: BreakdownSlice) => void
}) {
  const [tooltip, setTooltip] = useState<DonutTooltipState | null>(null)
  const outerSlices = topBreakdownSlices(outerData)
  const innerSlices = topBreakdownSlices(innerData).map((slice, index) => ({
    ...slice,
    color: chartColor(index + 3),
  }))
  const total = outerSlices.reduce((sum, slice) => sum + slice.value, 0)
  const hasData = outerSlices.length > 0 || innerSlices.length > 0

  if (!hasData) {
    return (
      <div className="flex min-h-[220px] flex-col rounded-lg border border-border bg-card p-4 shadow-sm">
        <h3 className="mb-3 text-xs uppercase tracking-wider text-muted-foreground">{title}</h3>
        <p className="flex flex-1 items-center justify-center py-4 text-center text-xs text-muted-foreground">
          No data
        </p>
      </div>
    )
  }

  const size = 176
  const cx = size / 2
  const cy = size / 2
  const buildRing = (
    slices: BreakdownSlice[],
    radius: number,
    strokeWidth: number,
    group: string,
    onSliceClick?: (slice: BreakdownSlice) => void,
  ) => {
    const ringTotal = slices.reduce((sum, slice) => sum + slice.value, 0)
    const circumference = 2 * Math.PI * radius
    let cumulativeOffset = 0

    return slices.map((slice, index) => {
      const length = ringTotal > 0 ? (slice.value / ringTotal) * circumference : 0
      const dashOffset = circumference - cumulativeOffset
      cumulativeOffset += length

      return (
        <circle
          key={`${group}-${slice.label}-${index}`}
          cx={cx}
          cy={cy}
          r={radius}
          fill="none"
          stroke={slice.color}
          strokeWidth={strokeWidth}
          strokeDasharray={`${length} ${circumference - length}`}
          strokeDashoffset={dashOffset}
          tabIndex={onSliceClick ? 0 : -1}
          role={onSliceClick ? 'button' : undefined}
          aria-label={
            onSliceClick
              ? `${group}: ${slice.label}, ${formatCurrency(slice.value, currency)}, ${Math.round((slice.value / ringTotal) * 100)} percent`
              : undefined
          }
          style={{ cursor: onSliceClick ? 'pointer' : 'default' }}
          onMouseEnter={(event) =>
            setTooltip({
              x: event.clientX + 12,
              y: event.clientY + 12,
              label: slice.label,
              amount: slice.value,
              pct: ringTotal > 0 ? (slice.value / ringTotal) * 100 : 0,
              group,
            })
          }
          onMouseMove={(event) =>
            setTooltip((prev) =>
              prev ? { ...prev, x: event.clientX + 12, y: event.clientY + 12 } : prev,
            )
          }
          onMouseLeave={() => setTooltip(null)}
          onClick={() => onSliceClick?.(slice)}
          onKeyDown={(event) => {
            if (event.key === 'Enter' || event.key === ' ') {
              event.preventDefault()
              onSliceClick?.(slice)
            }
          }}
        />
      )
    })
  }

  return (
    <div className="flex min-h-[220px] flex-col rounded-lg border border-border bg-card p-4 shadow-sm">
      <h3 className="mb-3 text-xs uppercase tracking-wider text-muted-foreground">{title}</h3>
      <div className="flex flex-1 items-center justify-center py-1">
        <svg
          width={size}
          height={size}
          viewBox={`0 0 ${size} ${size}`}
          aria-label={title}
          className="overflow-visible"
        >
          <g transform={`rotate(-90, ${cx}, ${cy})`}>
            {buildRing(outerSlices, 68, 20, outerLabel, onOuterSliceClick)}
            {buildRing(innerSlices, 41, 18, innerLabel, onInnerSliceClick)}
          </g>
          <text
            x={cx}
            y={cy - 6}
            textAnchor="middle"
            fontSize={9}
            fill="currentColor"
            opacity={0.5}
          >
            Total
          </text>
          <text x={cx} y={cy + 8} textAnchor="middle" fontSize={11} fill="currentColor">
            {formatCurrency(total, currency)}
          </text>
        </svg>
      </div>
      <p className="mt-auto border-t border-border pt-3 text-xs text-muted-foreground">
        {innerLabel}: <span className="text-foreground">{innerSlices.length}</span> · {outerLabel}:{' '}
        <span className="text-foreground">{outerSlices.length}</span>
      </p>
      {tooltip &&
        createPortal(
          <div
            className="pointer-events-none fixed z-50 rounded border border-border bg-secondary px-2 py-1 text-xs shadow-lg"
            style={{ left: tooltip.x, top: tooltip.y }}
          >
            {tooltip.group && (
              <>
                <span className="text-[10px] uppercase tracking-wider text-muted-foreground">
                  {tooltip.group}
                </span>
                <br />
              </>
            )}
            <span className="font-medium text-foreground">{tooltip.label}</span>
            <br />
            <span className="text-muted-foreground">
              {formatCurrency(tooltip.amount, currency)} · {tooltip.pct.toFixed(1)}%
            </span>
          </div>,
          document.body,
        )}
    </div>
  )
}

const BUCKET_COLORS: Record<string, string> = {
  Needs: '#3b82f6',
  Wants: '#f59e0b',
  Investments: '#10b981',
  Income: '#8b5cf6',
}

function BucketRing({
  data,
  currency,
  onSliceClick,
}: {
  data: Record<string, number>
  currency: string
  onSliceClick?: (bucket: string) => void
}) {
  const [tooltip, setTooltip] = useState<DonutTooltipState | null>(null)

  const entries = Object.entries(data ?? {})
    .filter(([, value]) => value > 0)
    .sort(([, a], [, b]) => b - a)

  if (entries.length === 0) {
    return (
      <div className="flex h-full min-h-[220px] flex-col rounded-lg border border-border bg-card p-4 shadow-sm">
        <h3 className="mb-3 text-xs uppercase tracking-wider text-muted-foreground">
          Needs · Wants · Investments
        </h3>
        <p className="flex flex-1 items-center justify-center py-4 text-center text-xs text-muted-foreground">
          No data
        </p>
      </div>
    )
  }

  const total = entries.reduce((sum, [, value]) => sum + value, 0)
  const size = 156
  const r = size * 0.36
  const strokeWidth = size * 0.15
  const cx = size / 2
  const cy = size / 2
  const C = 2 * Math.PI * r

  let offset = 0
  const slices = entries.map(([label, value], index) => {
    const length = (value / total) * C
    const slice = {
      label,
      value,
      length,
      offset,
      color: BUCKET_COLORS[label] ?? chartColor(index),
    }
    offset += length
    return slice
  })

  return (
    <div className="flex h-full min-h-[220px] flex-col rounded-lg border border-border bg-card p-4 shadow-sm">
      <h3 className="mb-3 text-xs uppercase tracking-wider text-muted-foreground">
        Needs · Wants · Investments
      </h3>
      <div className="flex flex-1 flex-col items-center justify-center">
        <div className="relative flex items-center justify-center">
          <svg
            width={size}
            height={size}
            viewBox={`0 0 ${size} ${size}`}
            aria-label="Needs, wants, and investments breakdown"
          >
            <g transform={`rotate(-90, ${cx}, ${cy})`}>
              {slices.map((s) => (
                <circle
                  key={s.label}
                  cx={cx}
                  cy={cy}
                  r={r}
                  fill="none"
                  stroke={s.color}
                  strokeWidth={strokeWidth}
                  strokeDasharray={`${s.length} ${C - s.length}`}
                  strokeDashoffset={C - s.offset}
                  tabIndex={onSliceClick ? 0 : -1}
                  role={onSliceClick ? 'button' : undefined}
                  aria-label={
                    onSliceClick
                      ? `${displayBucketLabel(s.label)}, ${formatCurrency(s.value, currency)}, ${Math.round((s.value / total) * 100)} percent`
                      : undefined
                  }
                  style={{ cursor: onSliceClick ? 'pointer' : 'default' }}
                  onMouseEnter={(e) =>
                    setTooltip({
                      x: e.clientX + 12,
                      y: e.clientY + 12,
                      label: displayBucketLabel(s.label),
                      amount: s.value,
                      pct: (s.value / total) * 100,
                    })
                  }
                  onMouseMove={(e) =>
                    setTooltip((prev) =>
                      prev ? { ...prev, x: e.clientX + 12, y: e.clientY + 12 } : prev,
                    )
                  }
                  onMouseLeave={() => setTooltip(null)}
                  onClick={() => onSliceClick?.(s.label)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault()
                      onSliceClick?.(s.label)
                    }
                  }}
                />
              ))}
            </g>
            <text
              x={cx}
              y={cy - 6}
              textAnchor="middle"
              fontSize={9}
              fill="currentColor"
              opacity={0.5}
            >
              Total
            </text>
            <text x={cx} y={cy + 8} textAnchor="middle" fontSize={11} fill="currentColor">
              {formatCurrency(total, currency)}
            </text>
          </svg>
        </div>
      </div>
      {tooltip &&
        createPortal(
          <div
            className="pointer-events-none fixed z-50 rounded border border-border bg-secondary px-2 py-1 text-xs shadow-lg"
            style={{ left: tooltip.x, top: tooltip.y }}
          >
            <span className="font-medium text-foreground">{tooltip.label}</span>
            <br />
            <span className="text-muted-foreground">
              {formatCurrency(tooltip.amount, currency)} · {tooltip.pct.toFixed(1)}%
            </span>
          </div>,
          document.body,
        )}
    </div>
  )
}

function CategoryMonthlyCard({
  data,
  currency,
  monthLabel,
  locale,
  title,
  showPrior = true,
  onRowClick,
}: {
  data: Record<string, CategoryMonthlyEntry>
  currency: string
  monthLabel: string
  locale: string
  title?: string
  showPrior?: boolean
  onRowClick?: (category: string) => void
}) {
  const currentMonth = parseMonthLabel(monthLabel)
  const currentLabel = currentMonth
    ? formatMonthForLocale(new Date(currentMonth.year, currentMonth.month - 1, 1), locale)
    : monthLabel
  const priorMonth = currentMonth ? prevMonth(currentMonth) : null
  const priorLabel = priorMonth
    ? formatMonthForLocale(new Date(priorMonth.year, priorMonth.month - 1, 1), locale)
    : 'Prior'

  const cardTitle = title ?? `${currentLabel} vs ${priorLabel}`

  const rows = Object.entries(data)
    .filter(([, entry]) => entry.current > 0 || (showPrior && entry.prior > 0))
    .sort(([, a], [, b]) => b.current - a.current)
    .slice(0, 5)

  if (rows.length === 0) {
    return (
      <div className="flex h-full min-h-[220px] flex-col rounded-lg border border-border bg-card p-4 shadow-sm">
        <h3 className="mb-3 text-xs uppercase tracking-wider text-muted-foreground">{cardTitle}</h3>
        <p className="flex flex-1 items-center justify-center py-4 text-center text-xs text-muted-foreground">
          No data
        </p>
      </div>
    )
  }

  const maxVal = Math.max(
    ...rows.flatMap(([, entry]) => (showPrior ? [entry.current, entry.prior] : [entry.current])),
    1,
  )

  return (
    <div className="flex h-full min-h-[220px] flex-col rounded-lg border border-border bg-card p-4 shadow-sm">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-xs uppercase tracking-wider text-muted-foreground">{cardTitle}</h3>
        {showPrior && (
          <div className="flex items-center gap-3 text-[10px] text-muted-foreground">
            <span className="flex items-center gap-1">
              <span className="inline-block h-2 w-3 rounded-sm bg-primary/80" />
              {currentLabel}
            </span>
            <span className="flex items-center gap-1">
              <span className="inline-block h-2 w-3 rounded-sm bg-secondary-foreground/20" />
              {priorLabel}
            </span>
          </div>
        )}
      </div>
      <div className="space-y-2">
        {rows.map(([cat, entry]) => {
          const delta =
            showPrior && entry.prior > 0
              ? ((entry.current - entry.prior) / entry.prior) * 100
              : null
          const isClickable = Boolean(onRowClick) && entry.current > 0
          return (
            <button
              key={cat}
              type="button"
              className="group block w-full text-left"
              disabled={!isClickable}
              style={{ cursor: isClickable ? 'pointer' : 'default' }}
              onClick={() => {
                if (isClickable) onRowClick?.(cat)
              }}
            >
              <div className="mb-0.5 flex items-center justify-between gap-3">
                <span
                  className={[
                    'truncate text-xs text-muted-foreground',
                    isClickable ? 'group-hover:text-foreground' : 'opacity-70',
                  ].join(' ')}
                >
                  {cat}
                </span>
                <div className="flex items-center gap-2">
                  <span className="font-mono text-xs text-foreground">
                    {formatCurrency(entry.current, currency)}
                  </span>
                  {delta !== null && (
                    <span
                      className={[
                        'text-[10px] font-medium tabular-nums',
                        delta > 0 ? 'text-amber-500' : 'text-emerald-500',
                      ].join(' ')}
                    >
                      {delta > 0 ? '+' : ''}
                      {Math.round(delta)}%
                    </span>
                  )}
                </div>
              </div>
              <div className="flex flex-col gap-0.5">
                <div className="h-1.5 overflow-hidden rounded-full bg-secondary">
                  <div
                    className="h-full rounded-full bg-primary/80 transition-all"
                    style={{ width: `${(entry.current / maxVal) * 100}%` }}
                  />
                </div>
                <div
                  className={[
                    'h-1 overflow-hidden rounded-full bg-secondary',
                    !showPrior && 'invisible',
                  ].join(' ')}
                >
                  {showPrior && (
                    <div
                      className="h-full rounded-full bg-secondary-foreground/20 transition-all"
                      style={{ width: `${(entry.prior / maxVal) * 100}%` }}
                    />
                  )}
                </div>
              </div>
            </button>
          )
        })}
      </div>
    </div>
  )
}

const SPEND_BREAKDOWN_OPTIONS: Array<{
  key: SpendBreakdownMode
  label: string
  param: 'label' | 'category' | 'bucket'
}> = [
  { key: 'categories', label: 'Categories', param: 'category' },
  { key: 'buckets', label: 'Buckets', param: 'bucket' },
  { key: 'labels', label: 'Labels', param: 'label' },
]

// ─── Bar chart with MoM delta ─────────────────────────────────────────────────

function dayDateRange(period: string): { from: string; to: string } {
  const [y, mo, d] = period.split('-').map(Number)
  return {
    from: new Date(Date.UTC(y, mo - 1, d, 0, 0, 0)).toISOString().split('.')[0] + 'Z',
    to: new Date(Date.UTC(y, mo - 1, d, 23, 59, 59)).toISOString().split('.')[0] + 'Z',
  }
}

interface BarTooltipState {
  x: number
  y: number
  label: string
  amount: number
  count: number
  delta?: number // MoM % change
}

function BarChart({
  title,
  data,
  labelFormat,
  currency,
  showDelta,
  onBarClick,
}: {
  title: string
  data: TimeBucket[]
  labelFormat: (period: string) => string
  currency: string
  showDelta?: boolean
  onBarClick?: (period: string) => void
}) {
  const [tooltip, setTooltip] = useState<BarTooltipState | null>(null)

  if (data.length === 0) {
    return (
      <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
        <h3 className="mb-3 text-xs uppercase tracking-wider text-muted-foreground">{title}</h3>
        <p className="py-4 text-center text-xs text-muted-foreground">No data</p>
      </div>
    )
  }

  const maxAmount = Math.max(...data.map((d) => d.amount), 1)
  const visible = data.slice(-12)

  return (
    <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
      <h3 className="mb-4 text-xs uppercase tracking-wider text-muted-foreground">{title}</h3>
      <div className="flex h-24 items-end gap-1">
        {visible.map((d, idx) => {
          const pct = (d.amount / maxAmount) * 100
          const clickable = Boolean(onBarClick)
          const prev = visible[idx - 1]
          const delta =
            showDelta && prev && prev.amount > 0
              ? ((d.amount - prev.amount) / prev.amount) * 100
              : undefined

          return (
            <div
              key={d.period}
              className={[
                'group flex flex-1 flex-col items-center gap-0.5',
                clickable ? 'cursor-pointer' : '',
              ].join(' ')}
              onClick={() => onBarClick?.(d.period)}
              onMouseEnter={(e) =>
                setTooltip({
                  x: e.clientX,
                  y: e.clientY,
                  label: labelFormat(d.period),
                  amount: d.amount,
                  count: d.count,
                  delta,
                })
              }
              onMouseMove={(e) =>
                setTooltip((prev) => prev && { ...prev, x: e.clientX, y: e.clientY })
              }
              onMouseLeave={() => setTooltip(null)}
            >
              <div className="flex w-full items-end" style={{ height: '72px' }}>
                <div
                  className="w-full rounded-t-sm bg-primary/70 transition-colors group-hover:bg-primary"
                  style={{ height: `${Math.max(pct, 2)}%` }}
                />
              </div>
              {showDelta && delta !== undefined ? (
                <span
                  className={[
                    'w-full truncate text-center text-[7px] font-medium tabular-nums',
                    delta > 0 ? 'text-amber-500' : 'text-emerald-500',
                  ].join(' ')}
                >
                  {delta > 0 ? '+' : ''}
                  {Math.round(delta)}%
                </span>
              ) : (
                <span className="w-full truncate text-center text-[8px] text-muted-foreground">
                  {labelFormat(d.period)}
                </span>
              )}
            </div>
          )
        })}
      </div>
      {tooltip &&
        createPortal(
          <div
            className="pointer-events-none fixed z-50 rounded border border-border bg-secondary px-2 py-1 text-xs shadow-lg"
            style={{ left: tooltip.x + 10, top: tooltip.y - 10 }}
          >
            <span className="font-medium text-foreground">{tooltip.label}</span>
            <br />
            <span className="text-muted-foreground">
              {formatCurrency(tooltip.amount, currency)} · {tooltip.count} txn
              {tooltip.count !== 1 ? 's' : ''}
            </span>
            {tooltip.delta !== undefined && (
              <>
                <br />
                <span className={tooltip.delta > 0 ? 'text-amber-500' : 'text-emerald-500'}>
                  vs prior: {tooltip.delta > 0 ? '+' : ''}
                  {tooltip.delta.toFixed(1)}%
                </span>
              </>
            )}
          </div>,
          document.body,
        )}
    </div>
  )
}

// ─── Daily bar chart with 7-day rolling average line ─────────────────────────

function DailySpendChart({
  data,
  currency,
  title = 'Daily spend (30 days)',
  locale,
  onBarClick,
}: {
  data: TimeBucket[]
  currency: string
  title?: string
  locale: string
  onBarClick?: (period: string) => void
}) {
  const [tooltip, setTooltip] = useState<BarTooltipState | null>(null)

  if (data.length === 0) {
    return (
      <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
        <h3 className="mb-3 text-xs uppercase tracking-wider text-muted-foreground">{title}</h3>
        <p className="py-4 text-center text-xs text-muted-foreground">No data</p>
      </div>
    )
  }

  const visible = data.slice(-30)
  const maxAmount = Math.max(...visible.map((d) => d.amount), 1)

  // Compute 7-day rolling average
  const rolling = visible.map((_, i) => {
    const window = visible.slice(Math.max(0, i - 6), i + 1)
    return window.reduce((s, d) => s + d.amount, 0) / window.length
  })

  // Rolling average line in percentage coords (viewBox 0 0 100 100, preserveAspectRatio=none)
  const toX = (i: number) => ((i + 0.5) / visible.length) * 100
  const toY = (v: number) => (1 - v / maxAmount) * 100
  const pts = rolling.map((v, i) => ({ x: toX(i), y: toY(v) }))
  let linePath = ''
  if (pts.length >= 2) {
    linePath = `M${pts[0].x},${pts[0].y}`
    for (let i = 0; i < pts.length - 1; i++) {
      const cx = (pts[i].x + pts[i + 1].x) / 2
      linePath += ` C${cx},${pts[i].y} ${cx},${pts[i + 1].y} ${pts[i + 1].x},${pts[i + 1].y}`
    }
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
      <div className="mb-4 flex items-center justify-between">
        <h3 className="text-xs uppercase tracking-wider text-muted-foreground">{title}</h3>
        <span className="flex items-center gap-1.5 text-[10px] text-muted-foreground">
          <svg width="16" height="6" viewBox="0 0 16 6" aria-hidden="true">
            <path d="M0 3 Q4 0 8 3 Q12 6 16 3" stroke="#f59e0b" strokeWidth="1.5" fill="none" />
          </svg>
          7-day avg
        </span>
      </div>
      <div className="relative">
        {/* Flex bars — same layout as BarChart */}
        <div className="flex h-24 items-end gap-0.5">
          {visible.map((d) => {
            const pct = (d.amount / maxAmount) * 100
            return (
              <div
                key={d.period}
                className="group flex flex-1 items-end"
                style={{ height: '100%', cursor: onBarClick ? 'pointer' : 'default' }}
                onClick={() => onBarClick?.(d.period)}
                onMouseEnter={(e) =>
                  setTooltip({
                    x: e.clientX,
                    y: e.clientY,
                    label: formatDateShort(d.period, undefined, locale),
                    amount: d.amount,
                    count: d.count,
                  })
                }
                onMouseMove={(e) =>
                  setTooltip((prev) => prev && { ...prev, x: e.clientX, y: e.clientY })
                }
                onMouseLeave={() => setTooltip(null)}
              >
                <div
                  className="w-full rounded-t-sm bg-primary/50 transition-colors group-hover:bg-primary"
                  style={{ height: `${Math.max(pct, 2)}%` }}
                />
              </div>
            )
          })}
        </div>
        {/* Rolling average SVG overlay — pointer-events:none so bars remain hoverable */}
        {linePath && (
          <svg
            className="pointer-events-none absolute inset-0 h-full w-full overflow-visible"
            viewBox="0 0 100 100"
            preserveAspectRatio="none"
            aria-hidden="true"
          >
            <path
              d={linePath}
              fill="none"
              stroke="#f59e0b"
              strokeWidth="2"
              strokeLinecap="round"
              vectorEffect="non-scaling-stroke"
            />
          </svg>
        )}
      </div>
      {tooltip &&
        createPortal(
          <div
            className="pointer-events-none fixed z-50 rounded border border-border bg-secondary px-2 py-1 text-xs shadow-lg"
            style={{ left: tooltip.x + 10, top: tooltip.y - 10 }}
          >
            <span className="font-medium text-foreground">{tooltip.label}</span>
            <br />
            <span className="text-muted-foreground">
              {formatCurrency(tooltip.amount, currency)} · {tooltip.count} txn
              {tooltip.count !== 1 ? 's' : ''}
            </span>
          </div>,
          document.body,
        )}
    </div>
  )
}

type SummaryMode = 'current_month' | 'all_time'

const SUMMARY_MODE_OPTIONS: SummaryMode[] = ['current_month', 'all_time']

const MONTH_NAME_TO_NUMBER: Record<string, number> = {
  January: 1,
  February: 2,
  March: 3,
  April: 4,
  May: 5,
  June: 6,
  July: 7,
  August: 8,
  September: 9,
  October: 10,
  November: 11,
  December: 12,
}

function parseMonthLabel(label: string): MonthNav | null {
  const [monthName, yearValue] = label.trim().split(/\s+/)
  const month = MONTH_NAME_TO_NUMBER[monthName]
  const year = Number(yearValue)
  if (!month || !Number.isInteger(year)) return null
  return { year, month }
}

function timeZoneOffsetMinutes(date: Date, timeZone: string): number {
  const offsetLabel =
    new Intl.DateTimeFormat('en-US', {
      timeZone,
      timeZoneName: 'shortOffset',
      hour: '2-digit',
    })
      .formatToParts(date)
      .find((part) => part.type === 'timeZoneName')?.value ?? 'GMT'

  if (offsetLabel === 'GMT' || offsetLabel === 'UTC') return 0

  const match = offsetLabel.match(/^GMT([+-])(\d{1,2})(?::(\d{2}))?$/)
  if (!match) return 0

  const sign = match[1] === '-' ? -1 : 1
  const hours = Number(match[2] ?? '0')
  const minutes = Number(match[3] ?? '0')
  return sign * (hours * 60 + minutes)
}

function zonedDateTimeToUtcIso(
  nav: MonthNav,
  timeZone: string,
  day: number,
  hour: number,
  minute: number,
  second: number,
): string {
  const guess = Date.UTC(nav.year, nav.month - 1, day, hour, minute, second)
  const initialOffset = timeZoneOffsetMinutes(new Date(guess), timeZone)
  const adjusted = guess - initialOffset * 60 * 1000
  const correctedOffset = timeZoneOffsetMinutes(new Date(adjusted), timeZone)
  return new Date(guess - correctedOffset * 60 * 1000).toISOString().split('.')[0] + 'Z'
}

function buildMonthRangeParams(
  monthLabel: string,
  timeZone: string,
): { from: string; to: string } | null {
  const nav = parseMonthLabel(monthLabel)
  if (!nav) return null

  const lastDay = new Date(Date.UTC(nav.year, nav.month, 0)).getUTCDate()
  return {
    from: zonedDateTimeToUtcIso(nav, timeZone, 1, 0, 0, 0),
    to: zonedDateTimeToUtcIso(nav, timeZone, lastDay, 23, 59, 59),
  }
}

function buildTransactionSearch(
  params: Record<string, string | undefined>,
  monthRange?: { from: string; to: string } | null,
): string {
  const search = new URLSearchParams()

  Object.entries(params).forEach(([key, value]) => {
    if (value) search.set(key, value)
  })

  if (monthRange) {
    search.set('date_from', monthRange.from)
    search.set('date_to', monthRange.to)
  }

  return search.toString()
}

export function SummarySection({
  summary,
  currency,
  locale,
  summaryMode,
  currentMonthRange,
}: {
  summary: { label: string; stats: Stats; charts: ChartData }
  currency: string
  locale: string
  summaryMode: SummaryMode
  currentMonthRange?: { from: string; to: string } | null
}) {
  const navigate = useNavigate()
  const { t } = useI18n()

  const monthRange = summaryMode === 'current_month' ? currentMonthRange : null

  const goToTransactions = (params: Record<string, string | undefined> = {}) => {
    navigate(`/transactions?${buildTransactionSearch(params, monthRange)}`)
  }

  const goToBreakdownSlice = (slice: BreakdownSlice, includeKey: BreakdownDimension) => {
    goToTransactions(dashboardBreakdownParams(includeKey, slice.label))
  }

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
        <div className="md:col-span-1">
          <BucketRing
            data={summary.charts.by_bucket}
            currency={currency}
            onSliceClick={(label) =>
              goToTransactions(dashboardBreakdownParams('bucket', displayBucketLabel(label)))
            }
          />
        </div>
        <div className="md:col-span-2">
          <CategoryMonthlyCard
            data={summary.charts.by_category_monthly}
            currency={currency}
            locale={locale}
            monthLabel={summary.label}
            title={
              summaryMode === 'all_time' ? t('dashboard.breakdown.spendByCategory') : undefined
            }
            showPrior={summaryMode === 'current_month'}
            onRowClick={(category) =>
              goToTransactions(dashboardBreakdownParams('category', category))
            }
          />
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
        <BreakdownChart
          title={t('dashboard.breakdown.byCategory')}
          data={dashboardBreakdownData(summary.charts.by_category)}
          currency={currency}
          onSliceClick={(slice) => goToBreakdownSlice(slice, 'category')}
        />
        <BreakdownChart
          title={t('dashboard.breakdown.byLabel')}
          data={dashboardBreakdownData(summary.charts.by_label)}
          currency={currency}
          onSliceClick={(slice) => goToBreakdownSlice(slice, 'label')}
        />
        <ConcentricBreakdownChart
          title={t('dashboard.breakdown.bankAndType')}
          outerData={summary.charts.by_source_type}
          innerData={summary.charts.by_bank}
          outerLabel={t('common.type')}
          innerLabel={t('common.bank')}
          currency={currency}
          onOuterSliceClick={(slice) => goToBreakdownSlice(slice, 'source_type')}
          onInnerSliceClick={(slice) => goToBreakdownSlice(slice, 'bank')}
        />
      </div>
    </div>
  )
}

function SummaryOverviewCard({
  summary,
  currency,
  locale,
  summaryMode,
  currentMonthRange,
}: {
  summary: { label: string; stats: Stats; charts: ChartData }
  currency: string
  locale: string
  summaryMode: SummaryMode
  currentMonthRange?: { from: string; to: string } | null
}) {
  const navigate = useNavigate()
  const monthRange = summaryMode === 'current_month' ? currentMonthRange : null

  const goToTransactions = () => {
    navigate(`/transactions?${buildTransactionSearch({ show_filters: '1' }, monthRange)}`)
  }

  return (
    <div className="space-y-4">
      <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
        <div className="space-y-3">
          <button type="button" onClick={goToTransactions} className="block text-left">
            <span className="block text-xs uppercase tracking-wider text-muted-foreground">
              Total spend ({summary.stats.base_currency})
            </span>
            <span className="mt-1 block break-all font-mono text-3xl font-semibold tabular-nums text-primary transition-colors hover:text-primary/80">
              {formatCurrency(summary.stats.total_base, currency)}
            </span>
          </button>
          <button
            type="button"
            onClick={goToTransactions}
            className="block text-left text-sm text-muted-foreground transition-colors hover:text-foreground"
          >
            {formatNumberForLocale(summary.stats.total_count, locale)} transactions
          </button>
        </div>
      </div>
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

type HeatmapMetric = 'amount' | 'count'

export function MetricToggle({
  value,
  onChange,
}: {
  value: HeatmapMetric
  onChange: (v: HeatmapMetric) => void
}) {
  return (
    <div
      role="group"
      aria-label="Heatmap metric"
      className="flex rounded-md border border-border bg-card p-0.5 text-xs"
    >
      {(['amount', 'count'] as const).map((opt) => (
        <button
          key={opt}
          type="button"
          onClick={() => onChange(opt)}
          className={[
            'rounded px-3 py-1.5 transition-colors',
            value === opt
              ? 'bg-primary text-primary-foreground'
              : 'text-muted-foreground hover:text-foreground',
          ].join(' ')}
        >
          {opt === 'amount' ? 'Amount' : 'Count'}
        </button>
      ))}
    </div>
  )
}

// ─── Spending patterns ────────────────────────────────────────────────────────

function SpendingPatternsSection() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const [metric, setMetric] = useState<HeatmapMetric>(DEFAULT_HEATMAP_METRIC)
  const { data: status } = useStatus()
  const { data: timezone } = useTimezone()
  const currency = status?.stats?.base_currency ?? 'INR'
  const now = new Date()
  const currentYear = now.getFullYear()
  const heatmapTimezone = timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone

  // Item 4: year persisted in URL
  const yearParam = searchParams.get('heatmap_year')
  const year = yearParam ? parseInt(yearParam, 10) : currentYear
  const setYear = (y: number) => {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        next.set('heatmap_year', String(y))
        return next
      },
      { replace: true },
    )
  }

  // Item 5: month filter persisted in URL
  const monthParam = searchParams.get('heatmap_month') // format YYYY-MM or absent
  const monthNav: MonthNav | null = monthParam
    ? {
        year: parseInt(monthParam.split('-')[0]!, 10),
        month: parseInt(monthParam.split('-')[1]!, 10),
      }
    : null
  const setMonthNav = (nav: MonthNav | null) => {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (nav) {
          next.set('heatmap_month', `${nav.year}-${String(nav.month).padStart(2, '0')}`)
        } else {
          next.delete('heatmap_month')
        }
        return next
      },
      { replace: true },
    )
  }

  const dateRange = monthNav ? monthRangeISO(monthNav) : undefined
  const { data: heatmap, isLoading } = useHeatmapData(dateRange?.from, dateRange?.to)

  const monthLabel = monthNav ? `${MONTH_NAMES[monthNav.month - 1]} ${monthNav.year}` : undefined

  const isCurrentMonth =
    monthNav !== null &&
    monthNav.year === now.getFullYear() &&
    monthNav.month === now.getMonth() + 1

  if (isLoading && !heatmap) {
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
                setMonthNav(
                  prevMonth(monthNav ?? { year: now.getFullYear(), month: now.getMonth() + 1 }),
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
                setMonthNav(
                  nextMonth(monthNav ?? { year: now.getFullYear(), month: now.getMonth() + 1 }),
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
          currency={currency}
          monthLabel={monthLabel}
          onCellClick={(weekday, hour) => {
            const params = new URLSearchParams({
              weekday: String(weekday),
              hour_from: String(hour),
              hour_to: String(hour),
              tz: heatmapTimezone,
              show_filters: '1',
            })
            if (dateRange) {
              params.set('date_from', dateRange.from)
              params.set('date_to', dateRange.to)
            }
            navigate(`/transactions?${params.toString()}`)
          }}
        />
      </div>
      <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
        <div className="mb-3 flex items-center justify-between">
          <h3 className="text-xs uppercase tracking-wider text-muted-foreground">By day of year</h3>
          <div className="flex items-center gap-1.5">
            <button
              onClick={() => setYear(year - 1)}
              className="rounded border border-border px-1.5 py-0.5 text-xs text-muted-foreground hover:text-foreground"
              aria-label="Previous year"
            >
              ←
            </button>
            <span className="min-w-[3rem] text-center text-xs font-medium tabular-nums text-foreground">
              {year}
            </span>
            <button
              onClick={() => setYear(year + 1)}
              disabled={year >= currentYear}
              className="rounded border border-border px-1.5 py-0.5 text-xs text-muted-foreground hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40"
              aria-label="Next year"
            >
              →
            </button>
          </div>
        </div>
        <AnnualCalendarHeatmap
          year={year}
          metric={metric}
          currency={currency}
          onDayClick={(date) => {
            const from =
              new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate(), 0, 0, 0))
                .toISOString()
                .split('.')[0] + 'Z'
            const to =
              new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate(), 23, 59, 59))
                .toISOString()
                .split('.')[0] + 'Z'
            navigate(
              `/transactions?date_from=${encodeURIComponent(from)}&date_to=${encodeURIComponent(to)}&show_filters=1`,
            )
          }}
        />
      </div>
      <HeatmapLegend />
    </div>
  )
}

// ─── Charts section ────────────────────────────────────────────────────────────

function formatMonthLabel(period: string, locale: string): string {
  const [year, month] = period.split('-')
  const d = new Date(Number(year), Number(month) - 1, 1)
  return formatMonthForLocale(d, locale)
}

function ChartsSection({
  charts,
  currency,
  locale,
  monthlyTitle = 'Monthly spend',
  dailyTitle = 'Daily spend (30 days)',
}: {
  charts: ChartData
  currency: string
  locale: string
  monthlyTitle?: string
  dailyTitle?: string
}) {
  const navigate = useNavigate()

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <BarChart
          title={monthlyTitle}
          data={charts.monthly_spend ?? []}
          labelFormat={(period) => formatMonthLabel(period, locale)}
          currency={currency}
          showDelta
          onBarClick={(period) => {
            const [y, m] = period.split('-').map(Number)
            const { from, to } = monthRangeISO({ year: y, month: m })
            navigate(
              `/transactions?date_from=${encodeURIComponent(from)}&date_to=${encodeURIComponent(to)}&show_filters=1`,
            )
          }}
        />
        <DailySpendChart
          data={charts.daily_spend ?? []}
          currency={currency}
          locale={locale}
          title={dailyTitle}
          onBarClick={(period) => {
            const { from, to } = dayDateRange(period)
            navigate(
              `/transactions?date_from=${encodeURIComponent(from)}&date_to=${encodeURIComponent(to)}&show_filters=1`,
            )
          }}
        />
      </div>
    </div>
  )
}

// ─── Label spend timeline line chart ─────────────────────────────────────────

const SHORT_MONTHS = [
  '',
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
const shortMonth = (m: string) => SHORT_MONTHS[parseInt(m.split('-')[1], 10)]

function smoothPath(pts: { x: number; y: number }[]): string {
  if (pts.length < 2) return pts.length === 1 ? `M${pts[0].x},${pts[0].y}` : ''
  let d = `M${pts[0].x},${pts[0].y}`
  for (let i = 0; i < pts.length - 1; i++) {
    const cx = (pts[i].x + pts[i + 1].x) / 2
    d += ` C${cx},${pts[i].y} ${cx},${pts[i + 1].y} ${pts[i + 1].x},${pts[i + 1].y}`
  }
  return d
}

export function BreakdownTimeline({
  data,
  currency,
  mode,
  onModeChange,
}: {
  data: MonthlyBreakdownData
  currency: string
  mode: SpendBreakdownMode
  onModeChange: (mode: SpendBreakdownMode) => void
}) {
  const { months, series } = data
  const [hoveredMonth, setHoveredMonth] = useState<number | null>(null)
  const [tooltipPos, setTooltipPos] = useState<{ x: number; y: number } | null>(null)
  const [selectedLabels, setSelectedLabels] = useState<string[]>([])
  const visibleSeries = series.filter((s) => s.data.some((value) => value > 0))
  const normalizedSelectedLabels = selectedLabels.filter((label) =>
    visibleSeries.some((entry) => entry.label === label),
  )
  const displayedSeries =
    normalizedSelectedLabels.length > 0
      ? visibleSeries.filter((entry) => normalizedSelectedLabels.includes(entry.label))
      : visibleSeries

  const W = 720
  const H = 180
  const PAD = { top: 16, right: 16, bottom: 28, left: 56 }
  const chartW = W - PAD.left - PAD.right
  const chartH = H - PAD.top - PAD.bottom

  useEffect(() => {
    if (normalizedSelectedLabels.length !== selectedLabels.length) {
      setSelectedLabels(normalizedSelectedLabels)
    }
  }, [normalizedSelectedLabels, selectedLabels])

  if (visibleSeries.length === 0) {
    const activeLabel =
      SPEND_BREAKDOWN_OPTIONS.find((option) => option.key === mode)?.label ?? 'Labels'
    return (
      <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
        <div className="mb-4 flex items-center justify-between gap-3">
          <h2 className="text-xs uppercase tracking-wider text-muted-foreground">
            Spend breakdown (12 months)
          </h2>
          <div className="flex rounded-md border border-border p-0.5 text-[10px]">
            {SPEND_BREAKDOWN_OPTIONS.map((option) => (
              <button
                key={option.key}
                type="button"
                onClick={() => onModeChange(option.key)}
                className={[
                  'rounded px-2 py-1 transition-colors',
                  mode === option.key
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:text-foreground',
                ].join(' ')}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>
        <div className="rounded-lg border border-border/60 bg-card p-4 shadow-sm">
          <p className="py-4 text-center text-xs text-muted-foreground">No data</p>
          <p className="text-center text-xs text-muted-foreground/80">
            No {activeLabel.toLowerCase()} data in the last 12 months.
          </p>
        </div>
      </div>
    )
  }

  const maxVal = Math.max(...displayedSeries.flatMap((s) => s.data), 1)
  const xStep = months.length > 1 ? chartW / (months.length - 1) : chartW

  const toX = (i: number) => PAD.left + i * xStep
  const toY = (v: number) => PAD.top + chartH - (v / maxVal) * chartH

  const yTicks = [0, 0.25, 0.5, 0.75, 1].map((f) => ({
    y: PAD.top + chartH * (1 - f),
    label: formatCompactNumber(maxVal * f),
  }))

  const handleLegendClick = (label: string, event: { metaKey: boolean; ctrlKey: boolean }) => {
    const isMultiSelect = event.metaKey || event.ctrlKey
    if (isMultiSelect) {
      setSelectedLabels((prev) => {
        if (prev.length === 0) return [label]
        if (prev.includes(label)) {
          const next = prev.filter((entry) => entry !== label)
          return next.length === 0 ? [] : next
        }
        return [...prev, label]
      })
      return
    }

    setSelectedLabels((prev) => (prev.length === 1 && prev[0] === label ? [] : [label]))
  }

  const tooltipSeries =
    hoveredMonth === null
      ? []
      : displayedSeries.filter((entry) => (entry.data[hoveredMonth] ?? 0) > 0)

  return (
    <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
      <div className="mb-4 flex items-center justify-between gap-3">
        <h2 className="text-xs uppercase tracking-wider text-muted-foreground">
          Spend breakdown (12 months)
        </h2>
        <div className="flex rounded-md border border-border p-0.5 text-[10px]">
          {SPEND_BREAKDOWN_OPTIONS.map((option) => (
            <button
              key={option.key}
              type="button"
              onClick={() => onModeChange(option.key)}
              className={[
                'rounded px-2 py-1 transition-colors',
                mode === option.key
                  ? 'bg-primary text-primary-foreground'
                  : 'text-muted-foreground hover:text-foreground',
              ].join(' ')}
            >
              {option.label}
            </button>
          ))}
        </div>
      </div>
      <div>
        <div className="mb-4 flex flex-wrap gap-4">
          {visibleSeries.map((s, i) => (
            <button
              key={s.label}
              type="button"
              onClick={(event) => handleLegendClick(s.label, event)}
              className={[
                'flex items-center gap-1.5 text-xs transition-opacity',
                normalizedSelectedLabels.length === 0 || normalizedSelectedLabels.includes(s.label)
                  ? 'text-muted-foreground opacity-100'
                  : 'text-muted-foreground opacity-40',
              ].join(' ')}
            >
              <span
                className="h-2 w-6 rounded-full"
                style={{ background: CHART_COLORS[i % CHART_COLORS.length] }}
              />
              {s.label}
            </button>
          ))}
        </div>

        <div className="relative overflow-x-auto">
          <svg viewBox={`0 0 ${W} ${H}`} className="w-full" style={{ minWidth: 360 }}>
            {yTicks.map((t, i) => (
              <g key={i}>
                <line
                  x1={PAD.left}
                  y1={t.y}
                  x2={W - PAD.right}
                  y2={t.y}
                  stroke="currentColor"
                  strokeOpacity={0.08}
                  strokeWidth={1}
                />
                <text
                  x={PAD.left - 6}
                  y={t.y + 3.5}
                  textAnchor="end"
                  fontSize={9}
                  fill="currentColor"
                  opacity={0.4}
                >
                  {t.label}
                </text>
              </g>
            ))}

            {months.map((m, i) => (
              <text
                key={m}
                x={toX(i)}
                y={H - 6}
                textAnchor="middle"
                fontSize={9}
                fill="currentColor"
                opacity={0.5}
              >
                {shortMonth(m)}
              </text>
            ))}

            {visibleSeries.map((s, si) => {
              const pts = s.data.map((v, i) => ({ x: toX(i), y: toY(v) }))
              const color = CHART_COLORS[si % CHART_COLORS.length]
              const isActive =
                normalizedSelectedLabels.length === 0 || normalizedSelectedLabels.includes(s.label)
              return (
                <path
                  key={s.label}
                  d={smoothPath(pts)}
                  fill="none"
                  stroke={color}
                  strokeWidth={2}
                  strokeOpacity={isActive ? (hoveredMonth !== null ? 0.35 : 0.9) : 0.08}
                  strokeLinecap="round"
                />
              )
            })}

            {hoveredMonth !== null &&
              displayedSeries.map((s, si) => {
                const v = s.data[hoveredMonth] ?? 0
                if (v <= 0) return null
                return (
                  <circle
                    key={s.label}
                    cx={toX(hoveredMonth)}
                    cy={toY(v)}
                    r={4}
                    fill={CHART_COLORS[si % CHART_COLORS.length]}
                    stroke="var(--card)"
                    strokeWidth={2}
                  />
                )
              })}

            {months.map((_, i) => (
              <rect
                key={i}
                x={toX(i) - xStep / 2}
                y={PAD.top}
                width={xStep}
                height={chartH}
                fill="transparent"
                onMouseEnter={(e) => {
                  setHoveredMonth(i)
                  setTooltipPos({ x: e.clientX, y: e.clientY })
                }}
                onMouseMove={(e) => setTooltipPos({ x: e.clientX, y: e.clientY })}
                onMouseLeave={() => {
                  setHoveredMonth(null)
                  setTooltipPos(null)
                }}
              />
            ))}
          </svg>

          {hoveredMonth !== null &&
            tooltipPos &&
            createPortal(
              <div
                className="pointer-events-none fixed z-50 -translate-x-1/2 -translate-y-full rounded-lg border border-border bg-card px-3 py-2 shadow-xl"
                style={{ left: tooltipPos.x, top: tooltipPos.y - 8 }}
              >
                <p className="mb-1.5 text-[10px] font-medium text-muted-foreground">
                  {months[hoveredMonth].replace('-', ' ')}
                </p>
                {tooltipSeries.length > 0 ? (
                  tooltipSeries.map((s) => {
                    const colorIndex = visibleSeries.findIndex((entry) => entry.label === s.label)
                    return (
                      <div key={s.label} className="flex items-center gap-2 text-xs">
                        <span
                          className="h-1.5 w-1.5 flex-shrink-0 rounded-full"
                          style={{ background: CHART_COLORS[colorIndex % CHART_COLORS.length] }}
                        />
                        <span className="text-muted-foreground">{s.label}</span>
                        <span className="ml-auto font-mono text-foreground">
                          {formatCurrency(s.data[hoveredMonth] ?? 0, currency)}
                        </span>
                      </div>
                    )
                  })
                ) : (
                  <p className="text-xs text-muted-foreground">No spend</p>
                )}
              </div>,
              document.body,
            )}
        </div>
      </div>
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

export default function Dashboard() {
  const [searchParams, setSearchParams] = useSearchParams()
  const breakdownParam = searchParams.get('breakdown')
  const breakdownMode: SpendBreakdownMode =
    breakdownParam === 'labels' || breakdownParam === 'buckets'
      ? breakdownParam
      : DEFAULT_SPEND_BREAKDOWN_MODE
  const setBreakdownMode = (mode: SpendBreakdownMode) => {
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
  const requestedSummaryMode: SummaryMode =
    summaryParam === 'all_time' ? 'all_time' : 'current_month'
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
