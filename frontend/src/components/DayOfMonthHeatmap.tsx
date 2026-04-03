import { useState } from 'react'
import type { DayOfMonthBucket } from '@/api/types'
import { intensityColor } from '@/lib/heatmap'

const CELL_W = 20
const CELL_H = 40
const CELL_GAP = 2

interface TooltipState {
  x: number
  y: number
  day: number
  amount: number
  count: number
}

interface Props {
  data: DayOfMonthBucket[]
  metric: 'amount' | 'count'
}

export function DayOfMonthHeatmap({ data, metric }: Props) {
  const [tooltip, setTooltip] = useState<TooltipState | null>(null)

  // Build a lookup: buckets[day] = bucket (days 1–31)
  const buckets: Record<number, DayOfMonthBucket> = {}
  for (const b of data) {
    buckets[b.day] = b
  }

  const maxAmount = Math.max(...data.map((b) => b.amount), 1)
  const maxCount = Math.max(...data.map((b) => b.count), 1)
  const allZero = data.length === 0

  const totalWidth = 31 * (CELL_W + CELL_GAP) - CELL_GAP

  return (
    <div className="relative overflow-x-auto">
      {allZero && (
        <div className="absolute inset-0 flex items-center justify-center bg-card/80">
          <span className="text-xs text-muted-foreground">No transaction data yet</span>
        </div>
      )}
      <svg
        width={totalWidth}
        height={CELL_H}
        viewBox={`0 0 ${totalWidth} ${CELL_H}`}
        aria-label="Spending heatmap by day of month"
      >
        {Array.from({ length: 31 }, (_, i) => {
          const day = i + 1
          const bucket = buckets[day]
          const value = bucket ? (metric === 'amount' ? bucket.amount : bucket.count) : 0
          const max = metric === 'amount' ? maxAmount : maxCount
          const x = i * (CELL_W + CELL_GAP)
          return (
            <g key={day}>
              <rect
                x={x}
                y={0}
                width={CELL_W}
                height={CELL_H}
                rx={2}
                // Days with no data (e.g. day 31 in a 30-day month) shown at
                // 20% opacity so they visually recede without disappearing.
                fill={bucket ? intensityColor(value, max) : 'hsl(var(--muted))'}
                opacity={bucket ? 1 : 0.2}
                onMouseEnter={(e) => {
                  if (!bucket) return
                  setTooltip({
                    x: e.clientX,
                    y: e.clientY,
                    day,
                    amount: bucket.amount,
                    count: bucket.count,
                  })
                }}
                onMouseLeave={() => setTooltip(null)}
              />
              <text
                x={x + CELL_W / 2}
                y={CELL_H / 2 + 4}
                textAnchor="middle"
                fontSize="9"
                fill="currentColor"
                className="select-none fill-foreground/60"
              >
                {day}
              </text>
            </g>
          )
        })}
      </svg>

      {tooltip && (
        <div
          className="pointer-events-none fixed z-50 rounded border border-border bg-popover px-2 py-1 text-xs shadow-sm"
          style={{ left: tooltip.x + 8, top: tooltip.y - 8 }}
        >
          <span className="font-medium text-foreground">Day {tooltip.day}</span>
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
