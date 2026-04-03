import { format } from 'date-fns'
import { useEffect, useRef, useState } from 'react'
import { DayPicker } from 'react-day-picker'
import type { DateRange } from 'react-day-picker'
import { cn } from '@/lib/utils'

interface DateRangePickerProps {
  value: { from?: Date; to?: Date }
  onChange: (range: { from?: Date; to?: Date }) => void
}

function formatRange(from?: Date, to?: Date): string {
  if (!from && !to) return 'All time'
  if (from && !to) return format(from, 'dd MMM yyyy')
  if (from && to) return `${format(from, 'dd MMM yyyy')} – ${format(to, 'dd MMM yyyy')}`
  return 'All time'
}

export function DateRangePicker({ value, onChange }: DateRangePickerProps) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (!containerRef.current?.contains(e.target as Node)) setOpen(false)
    }
    const keyHandler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    document.addEventListener('keydown', keyHandler)
    return () => {
      document.removeEventListener('mousedown', handler)
      document.removeEventListener('keydown', keyHandler)
    }
  }, [open])

  const hasRange = value.from || value.to

  const handleSelect = (range: DateRange | undefined) => {
    onChange({ from: range?.from, to: range?.to })
    if (range?.from && range?.to) setOpen(false)
  }

  return (
    <div ref={containerRef} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className={cn(
          'flex items-center gap-1.5 rounded-md border px-2 py-1.5 text-xs transition-colors',
          hasRange
            ? 'border-primary bg-primary/10 text-primary'
            : 'border-border bg-secondary text-muted-foreground hover:text-foreground',
        )}
        aria-label="Select date range"
        aria-expanded={open}
      >
        <span>{formatRange(value.from, value.to)}</span>
        {hasRange && (
          <span
            role="button"
            aria-label="Clear date range"
            onClick={(e) => {
              e.stopPropagation()
              onChange({})
              setOpen(false)
            }}
            className="ml-1 text-sm leading-none hover:text-destructive"
          >
            ×
          </span>
        )}
      </button>

      {open && (
        <div className="absolute left-0 top-full z-50 mt-1 rounded-lg border border-border bg-card p-3 shadow-lg">
          <DayPicker
            mode="range"
            numberOfMonths={2}
            selected={{ from: value.from, to: value.to }}
            onSelect={handleSelect}
            classNames={{
              months: 'flex gap-4',
              month: 'space-y-2',
              month_caption: 'flex items-center justify-between px-1 py-1',
              caption_label: 'text-xs font-medium text-foreground',
              nav: 'flex items-center gap-1',
              button_next:
                'rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors text-xs',
              button_previous:
                'rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors text-xs',
              month_grid: 'w-full border-collapse',
              weekdays: 'flex',
              weekday: 'w-8 text-center text-[10px] font-normal text-muted-foreground',
              week: 'flex mt-0.5',
              day: 'relative w-8 text-center',
              day_button:
                'h-8 w-8 rounded-md text-xs transition-colors hover:bg-accent hover:text-accent-foreground focus:outline-none focus-visible:ring-1 focus-visible:ring-ring',
              selected:
                'bg-primary text-primary-foreground hover:bg-primary hover:text-primary-foreground',
              today: 'font-semibold text-primary',
              outside: 'text-muted-foreground opacity-30',
              disabled: 'opacity-20 cursor-not-allowed',
              range_middle: 'bg-accent text-accent-foreground rounded-none',
              range_start: 'bg-primary text-primary-foreground rounded-r-none',
              range_end: 'bg-primary text-primary-foreground rounded-l-none',
              hidden: 'invisible',
            }}
          />
          {hasRange && (
            <div className="mt-2 flex justify-end border-t border-border pt-2">
              <button
                onClick={() => {
                  onChange({})
                  setOpen(false)
                }}
                className="text-xs text-muted-foreground transition-colors hover:text-foreground"
              >
                Clear
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
