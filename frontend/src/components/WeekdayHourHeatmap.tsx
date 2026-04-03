import { useState } from 'react'
import type { WeekdayHourBucket } from '@/api/types'
import { intensityColor } from '@/lib/heatmap'

const DAY_LABELS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

const CELL_SIZE = 14
const CELL_GAP = 2
const ROW_LABEL_WIDTH = 32
const COL_LABEL_HEIGHT = 16

interface TooltipState {
  x: number
  y: number
  day: string
  hour: number
  amount: number
  count: number
}

interface Props {
  data: WeekdayHourBucket[]
  metric: 'amount' | 'count'
  /** When set, shown as the first line of the hover tooltip, e.g. "Apr 2026". */
  monthLabel?: string
}

export function WeekdayHourHeatmap({ data, metric, monthLabel }: Props) {
  const [tooltip, setTooltip] = useState<TooltipState | null>(null)

  const grid: Record<number, Record<number, WeekdayHourBucket>> = {}
  for (const b of data) {
    if (!grid[b.weekday]) grid[b.weekday] = {}
    grid[b.weekday][b.hour] = b
  }

  const maxAmount = Math.max(...data.map((b) => b.amount), 1)
  const maxCount = Math.max(...data.map((b) => b.count), 1)
  const allZero = data.length === 0

  const totalWidth = ROW_LABEL_WIDTH + 24 * (CELL_SIZE + CELL_GAP) - CELL_GAP
  const totalHeight = COL_LABEL_HEIGHT + 7 * (CELL_SIZE + CELL_GAP) - CELL_GAP

  return (
    <div className="relative w-full">
      {allZero && (
        <div className="absolute inset-0 flex items-center justify-center bg-card/80">
          <span className="text-xs text-muted-foreground">No transaction data yet</span>
        </div>
      )}
      <svg
        width="100%"
        height={totalHeight}
        viewBox={`0 0 ${totalWidth} ${totalHeight}`}
        preserveAspectRatio="xMinYMid meet"
        aria-label="Spending heatmap by weekday and hour"
      >
        {[0, 6, 12, 18].map((hour) => (
          <text
            key={hour}
            x={ROW_LABEL_WIDTH + hour * (CELL_SIZE + CELL_GAP) + CELL_SIZE / 2}
            y={COL_LABEL_HEIGHT - 3}
            textAnchor="middle"
            fontSize="8"
            fill="currentColor"
            className="fill-muted-foreground"
          >
            {String(hour).padStart(2, '0')}
          </text>
        ))}

        {Array.from({ length: 7 }, (_, weekday) =>
          Array.from({ length: 24 }, (_, hour) => {
            const bucket = grid[weekday]?.[hour]
            const value = bucket ? (metric === 'amount' ? bucket.amount : bucket.count) : 0
            const max = metric === 'amount' ? maxAmount : maxCount
            const x = ROW_LABEL_WIDTH + hour * (CELL_SIZE + CELL_GAP)
            const y = COL_LABEL_HEIGHT + weekday * (CELL_SIZE + CELL_GAP)
            return (
              <rect
                key={`${weekday}-${hour}`}
                x={x}
                y={y}
                width={CELL_SIZE}
                height={CELL_SIZE}
                rx={2}
                fill={intensityColor(value, max)}
                onMouseEnter={(e) => {
                  if (!bucket) return
                  setTooltip({ x: e.clientX, y: e.clientY, day: DAY_LABELS[weekday], hour,
                    amount: bucket.amount, count: bucket.count })
                }}
                onMouseLeave={() => setTooltip(null)}
              />
            )
          }),
        )}

        {DAY_LABELS.map((label, weekday) => (
          <text
            key={label}
            x={ROW_LABEL_WIDTH - 4}
            y={COL_LABEL_HEIGHT + weekday * (CELL_SIZE + CELL_GAP) + CELL_SIZE / 2 + 3}
            textAnchor="end"
            fontSize="8"
            fill="currentColor"
            className="fill-muted-foreground"
          >
            {label}
          </text>
        ))}
      </svg>

      {tooltip && (
        <div
          className="pointer-events-none fixed z-50 rounded border border-border bg-secondary px-2 py-1 text-xs shadow-lg"
          style={{ left: tooltip.x + 8, top: tooltip.y - 8 }}
        >
          {monthLabel && (
            <span className="block text-muted-foreground">{monthLabel}</span>
          )}
          <span className="font-medium text-foreground">
            {tooltip.day} {String(tooltip.hour).padStart(2, '0')}:00
          </span>
          <br />
          <span className="text-muted-foreground">
            ₹{tooltip.amount.toLocaleString('en-IN')} &middot; {tooltip.count} txn
            {tooltip.count !== 1 ? 's' : ''}
          </span>
        </div>
      )}
    </div>
  )
}
