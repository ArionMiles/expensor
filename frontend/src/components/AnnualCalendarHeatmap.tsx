import { useState } from 'react'
import { useAnnualHeatmapData } from '@/api/queries'
import type { DailyBucket } from '@/api/types'
import { intensityColor } from '@/lib/heatmap'

// ─── Calendar grid builder ────────────────────────────────────────────────────

/**
 * Returns an array of week-columns. Each column has 7 cells (Mon=0 … Sun=6).
 * Cells before Jan 1 and after Dec 31 are null (padding).
 * ISO week convention: Mon=0, Sun=6.
 */
function buildCalendarGrid(year: number): { date: Date | null }[][] {
  const jan1 = new Date(year, 0, 1)
  // JS getDay(): 0=Sun … 6=Sat → shift so Mon=0, Sun=6
  const startDow = (jan1.getDay() + 6) % 7

  const allDates: Date[] = []
  const d = new Date(jan1)
  while (d.getFullYear() === year) {
    allDates.push(new Date(d))
    d.setDate(d.getDate() + 1)
  }

  // Pad to a multiple of 7
  const totalCells = Math.ceil((allDates.length + startDow) / 7) * 7
  const cells: (Date | null)[] = [
    ...Array(startDow).fill(null),
    ...allDates,
    ...Array(totalCells - allDates.length - startDow).fill(null),
  ]

  const numCols = totalCells / 7
  return Array.from({ length: numCols }, (_, col) =>
    Array.from({ length: 7 }, (_, row) => ({ date: cells[col * 7 + row] })),
  )
}

// ─── Constants ────────────────────────────────────────────────────────────────

const CELL = 14
const GAP = 2
const STEP = CELL + GAP
const DAY_LABEL_WIDTH = 28
const MONTH_LABEL_HEIGHT = 16

const LABELED_ROWS: { row: number; label: string }[] = [
  { row: 0, label: 'Mon' },
  { row: 2, label: 'Wed' },
  { row: 4, label: 'Fri' },
]

const MONTH_NAMES = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

// ─── Component ───────────────────────────────────────────────────────────────

interface TooltipState {
  x: number
  y: number
  date: Date
  amount: number
  count: number
}

interface Props {
  year: number
  metric: 'amount' | 'count'
}

export function AnnualCalendarHeatmap({ year, metric }: Props) {
  const [tooltip, setTooltip] = useState<TooltipState | null>(null)

  const { data, isLoading } = useAnnualHeatmapData(year)

  // Build lookup: "YYYY-MM-DD" → DailyBucket
  const lookup = new Map<string, DailyBucket>()
  if (data) {
    for (const b of data.buckets) {
      lookup.set(b.date.slice(0, 10), b)
    }
  }

  const maxValue = data
    ? Math.max(...data.buckets.map((b) => (metric === 'amount' ? b.amount : b.count)), 1)
    : 1
  const grid = buildCalendarGrid(year)
  const numCols = grid.length
  const svgWidth = DAY_LABEL_WIDTH + numCols * STEP - GAP
  const svgHeight = MONTH_LABEL_HEIGHT + 7 * STEP - GAP

  // Month label positions: first column whose first non-null date is the 1st of a month
  const monthLabels: { col: number; month: number }[] = []
  grid.forEach((col, colIdx) => {
    const firstDate = col.find((c) => c.date !== null)?.date
    if (firstDate && firstDate.getDate() === 1) {
      monthLabels.push({ col: colIdx, month: firstDate.getMonth() })
    }
  })

  function formatTooltipDate(d: Date): string {
    return d.toLocaleDateString('en-IN', {
      weekday: 'short',
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    })
  }

  if (isLoading) {
    return <div className="h-28 animate-pulse rounded bg-secondary" />
  }

  return (
    <div className="relative">
      <svg
        style={{ width: '100%', height: 'auto' }}
        viewBox={`0 0 ${svgWidth} ${svgHeight}`}
        aria-label={`Annual spending heatmap for ${year}`}
      >
        {/* Day-of-week labels: Mon, Wed, Fri */}
        {LABELED_ROWS.map(({ row, label }) => (
          <text
            key={label}
            x={DAY_LABEL_WIDTH - 4}
            y={MONTH_LABEL_HEIGHT + row * STEP + CELL / 2 + 3}
            textAnchor="end"
            fontSize={8}
            className="fill-muted-foreground"
          >
            {label}
          </text>
        ))}

        {/* Month labels */}
        {monthLabels.map(({ col, month }) => (
          <text
            key={`month-${month}`}
            x={DAY_LABEL_WIDTH + col * STEP}
            y={MONTH_LABEL_HEIGHT - 3}
            fontSize={8}
            className="fill-muted-foreground"
          >
            {MONTH_NAMES[month]}
          </text>
        ))}

        {/* Calendar cells */}
        {grid.map((col, colIdx) =>
          col.map(({ date }, rowIdx) => {
            const x = DAY_LABEL_WIDTH + colIdx * STEP
            const y = MONTH_LABEL_HEIGHT + rowIdx * STEP
            if (date === null) {
              return (
                <rect
                  key={`${colIdx}-${rowIdx}`}
                  x={x}
                  y={y}
                  width={CELL}
                  height={CELL}
                  rx={2}
                  fill="transparent"
                />
              )
            }
            const key = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
            const bucket = lookup.get(key)
            const amount = bucket?.amount ?? 0
            const value = metric === 'amount' ? amount : (bucket?.count ?? 0)
            return (
              <rect
                key={`${colIdx}-${rowIdx}`}
                x={x}
                y={y}
                width={CELL}
                height={CELL}
                rx={2}
                fill={intensityColor(value, maxValue)}
                onMouseEnter={(e) =>
                  setTooltip({ x: e.clientX, y: e.clientY, date, amount, count: bucket?.count ?? 0 })
                }
                onMouseLeave={() => setTooltip(null)}
              />
            )
          }),
        )}
      </svg>

      {tooltip && (
        <div
          className="pointer-events-none fixed z-50 rounded border border-border bg-secondary px-2 py-1 text-xs shadow-lg"
          style={{ left: tooltip.x + 10, top: tooltip.y - 10 }}
        >
          <span className="font-medium text-foreground">{formatTooltipDate(tooltip.date)}</span>
          <br />
          <span className="text-muted-foreground">
            {tooltip.amount > 0
              ? `₹${tooltip.amount.toLocaleString('en-IN')} · ${tooltip.count} txn${tooltip.count !== 1 ? 's' : ''}`
              : 'No transactions'}
          </span>
        </div>
      )}
    </div>
  )
}
