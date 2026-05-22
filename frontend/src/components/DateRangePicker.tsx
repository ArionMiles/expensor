import {
  addDays,
  addMonths,
  endOfMonth,
  format,
  isAfter,
  isBefore,
  isSameDay,
  isSameMonth,
  setHours,
  setMilliseconds,
  setMinutes,
  setSeconds,
  startOfMonth,
  subDays,
  subMonths,
} from 'date-fns'
import { ChevronLeft, ChevronRight, X } from 'lucide-react'
import { type KeyboardEvent, useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { cn } from '@/lib/utils'

interface DateRangePickerProps {
  value: { from?: Date; to?: Date }
  onChange: (range: { from?: Date; to?: Date }) => void
}

type TimePart = 'hour' | 'minute'
type TimeTarget = 'from' | 'to'

const WEEKDAYS = ['S', 'M', 'T', 'W', 'T', 'F', 'S']
const PRESETS = ['7D', '1M', 'This month', 'Last month', 'FY', 'Custom'] as const

function hasNonTrivialTime(from?: Date, to?: Date): boolean {
  if (from && (from.getHours() !== 0 || from.getMinutes() !== 0)) return true
  if (to && (to.getHours() !== 23 || to.getMinutes() !== 59)) return true
  return false
}

function formatRange(from?: Date, to?: Date): string {
  if (!from && !to) return 'All'
  const withTime = hasNonTrivialTime(from, to)
  if (from && !to) return format(from, withTime ? 'dd MMM HH:mm' : 'dd MMM yyyy')
  if (from && to) {
    const sameDay = format(from, 'dd MMM yyyy') === format(to, 'dd MMM yyyy')
    if (sameDay) {
      if (withTime)
        return `${format(from, 'dd MMM')} ${format(from, 'HH:mm')}-${format(to, 'HH:mm')}`
      return format(from, 'dd MMM yyyy')
    }
    if (withTime) return `${format(from, 'dd MMM HH:mm')} - ${format(to, 'dd MMM HH:mm')}`
    return `${format(from, 'dd MMM yyyy')} - ${format(to, 'dd MMM yyyy')}`
  }
  return 'All'
}

function padTwo(n: number) {
  return String(n).padStart(2, '0')
}

function withStartTime(date: Date, source?: Date) {
  return setMilliseconds(
    setSeconds(setMinutes(setHours(date, source?.getHours() ?? 0), source?.getMinutes() ?? 0), 0),
    0,
  )
}

function withEndTime(date: Date, source?: Date) {
  return setMilliseconds(
    setSeconds(
      setMinutes(setHours(date, source?.getHours() ?? 23), source?.getMinutes() ?? 59),
      59,
    ),
    999,
  )
}

function setDateTimePart(date: Date, part: TimePart, value: number, isEnd: boolean) {
  const bounded =
    part === 'hour' ? Math.min(Math.max(value, 0), 23) : Math.min(Math.max(value, 0), 59)
  const next = part === 'hour' ? setHours(date, bounded) : setMinutes(date, bounded)
  return setMilliseconds(setSeconds(next, isEnd ? 59 : 0), isEnd ? 999 : 0)
}

function fiscalYearRange(today = new Date()) {
  const year = today.getMonth() >= 3 ? today.getFullYear() : today.getFullYear() - 1
  return {
    from: new Date(year, 3, 1, 0, 0, 0, 0),
    to: new Date(year + 1, 2, 31, 23, 59, 59, 999),
  }
}

function monthDays(month: Date) {
  const start = startOfMonth(month)
  const blanks = Array.from({ length: start.getDay() }, () => null)
  const days = Array.from({ length: endOfMonth(month).getDate() }, (_, i) => addDays(start, i))
  return [...blanks, ...days]
}

function dropdownPosition(rect: DOMRect) {
  const margin = 16
  const width = Math.min(704, window.innerWidth - margin * 2)
  return {
    x: Math.min(Math.max(margin, rect.left), window.innerWidth - width - margin),
    y: rect.bottom + 4,
  }
}

function TimeSegmentInput({
  date,
  label,
  part,
  target,
  onChange,
  inputRef,
  onMoveFocus,
}: {
  date: Date
  label: string
  part: TimePart
  target: TimeTarget
  onChange: (target: TimeTarget, part: TimePart, value: number) => void
  inputRef: (node: HTMLInputElement | null) => void
  onMoveFocus: (target: TimeTarget, part: TimePart, backwards: boolean) => void
}) {
  const value = part === 'hour' ? date.getHours() : date.getMinutes()
  const [draft, setDraft] = useState<string | null>(null)
  const displayValue = draft ?? padTwo(value)

  const commit = (rawValue: string) => {
    setDraft(null)
    onChange(target, part, Number.parseInt(rawValue || '0', 10))
  }

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
      event.preventDefault()
      const input = event.currentTarget
      setDraft(null)
      onChange(target, part, value + (event.key === 'ArrowUp' ? 1 : -1))
      window.requestAnimationFrame(() => {
        input.focus()
        input.select()
      })
      return
    }
    if (event.key === 'Tab') {
      event.preventDefault()
      onMoveFocus(target, part, event.shiftKey)
      return
    }
    if (event.key === 'Enter') {
      event.currentTarget.blur()
    }
  }

  return (
    <input
      ref={inputRef}
      aria-label={label}
      inputMode="numeric"
      maxLength={2}
      value={displayValue}
      onFocus={(event) => {
        const input = event.currentTarget
        window.requestAnimationFrame(() => input.select())
      }}
      onChange={(event) => {
        const digits = event.currentTarget.value.replace(/\D/g, '').slice(0, 2)
        setDraft(digits)
        if (digits.length === 2) commit(digits)
      }}
      onBlur={() => commit(draft ?? padTwo(value))}
      onKeyDown={onKeyDown}
      className="h-7 w-7 rounded-md border-0 bg-transparent p-0 text-center font-mono text-xs tabular-nums text-foreground outline-none transition-colors hover:bg-accent focus:bg-primary focus:text-primary-foreground"
    />
  )
}

function TimeRangeEditor({
  from,
  to,
  onChange,
}: {
  from: Date
  to?: Date
  onChange: (range: { from?: Date; to?: Date }) => void
}) {
  const fallbackTo = to ?? new Date(new Date().setHours(23, 59, 59, 999))
  const refs = useRef<Record<string, HTMLInputElement | null>>({
    'from-hour': null,
    'from-minute': null,
    'to-hour': null,
    'to-minute': null,
  })

  const setRef = (target: TimeTarget, part: TimePart) => (node: HTMLInputElement | null) => {
    refs.current[`${target}-${part}`] = node
  }

  const moveFocus = (target: TimeTarget, part: TimePart, backwards: boolean) => {
    const order = ['from-hour', 'from-minute', 'to-hour', 'to-minute']
    const current = order.indexOf(`${target}-${part}`)
    const next = order[(current + (backwards ? -1 : 1) + order.length) % order.length]
    const input = refs.current[next]
    input?.focus()
    input?.select()
  }

  const updateTime = (target: TimeTarget, part: TimePart, value: number) => {
    onChange({
      from: target === 'from' ? setDateTimePart(from, part, value, false) : from,
      to: target === 'to' && to ? setDateTimePart(to, part, value, true) : to,
    })
  }

  const nudge = (target: TimeTarget, delta: number) => {
    onChange({
      from:
        target === 'from'
          ? setDateTimePart(
              addMinutes(from, delta),
              'minute',
              addMinutes(from, delta).getMinutes(),
              false,
            )
          : from,
      to:
        target === 'to' && to
          ? setDateTimePart(
              addMinutes(to, delta),
              'minute',
              addMinutes(to, delta).getMinutes(),
              true,
            )
          : to,
    })
  }

  return (
    <div className="flex min-w-0 flex-wrap items-center gap-2">
      <span className="text-xs text-muted-foreground">Time</span>
      <div className="flex items-center gap-1 rounded-lg border border-border bg-secondary p-1">
        <button
          type="button"
          aria-label="Decrease start time by 15 minutes"
          onClick={() => nudge('from', -15)}
          className="grid h-7 w-7 place-items-center rounded-md text-xs text-foreground transition-colors hover:bg-accent"
        >
          -
        </button>
        <span className="flex items-center gap-0.5">
          <TimeSegmentInput
            date={from}
            label="Start hour"
            part="hour"
            target="from"
            onChange={updateTime}
            inputRef={setRef('from', 'hour')}
            onMoveFocus={moveFocus}
          />
          <span className="text-xs text-muted-foreground">:</span>
          <TimeSegmentInput
            date={from}
            label="Start minute"
            part="minute"
            target="from"
            onChange={updateTime}
            inputRef={setRef('from', 'minute')}
            onMoveFocus={moveFocus}
          />
        </span>
        <button
          type="button"
          aria-label="Increase start time by 15 minutes"
          onClick={() => nudge('from', 15)}
          className="grid h-7 w-7 place-items-center rounded-md text-xs text-foreground transition-colors hover:bg-accent"
        >
          +
        </button>
      </div>
      <span className="text-xs text-muted-foreground">to</span>
      <div className="flex items-center gap-1 rounded-lg border border-border bg-secondary p-1">
        <button
          type="button"
          aria-label="Decrease end time by 15 minutes"
          onClick={() => nudge('to', -15)}
          disabled={!to}
          className="grid h-7 w-7 place-items-center rounded-md text-xs text-foreground transition-colors hover:bg-accent disabled:cursor-not-allowed disabled:opacity-40"
        >
          -
        </button>
        <span className="flex items-center gap-0.5">
          <TimeSegmentInput
            date={fallbackTo}
            label="End hour"
            part="hour"
            target="to"
            onChange={updateTime}
            inputRef={setRef('to', 'hour')}
            onMoveFocus={moveFocus}
          />
          <span className="text-xs text-muted-foreground">:</span>
          <TimeSegmentInput
            date={fallbackTo}
            label="End minute"
            part="minute"
            target="to"
            onChange={updateTime}
            inputRef={setRef('to', 'minute')}
            onMoveFocus={moveFocus}
          />
        </span>
        <button
          type="button"
          aria-label="Increase end time by 15 minutes"
          onClick={() => nudge('to', 15)}
          disabled={!to}
          className="grid h-7 w-7 place-items-center rounded-md text-xs text-foreground transition-colors hover:bg-accent disabled:cursor-not-allowed disabled:opacity-40"
        >
          +
        </button>
      </div>
    </div>
  )
}

function addMinutes(date: Date, minutes: number) {
  return new Date(date.getTime() + minutes * 60_000)
}

function CalendarMonth({
  month,
  pending,
  onSelect,
}: {
  month: Date
  pending: { from?: Date; to?: Date }
  onSelect: (day: Date) => void
}) {
  return (
    <div className="min-w-0">
      <div className="mb-2 grid grid-cols-7">
        {WEEKDAYS.map((day, index) => (
          <span key={`${day}-${index}`} className="text-center text-[11px] text-muted-foreground">
            {day}
          </span>
        ))}
      </div>
      <div className="grid grid-cols-7">
        {monthDays(month).map((day, index) => {
          if (!day) return <span key={`blank-${index}`} className="h-8" />
          const isStart = pending.from && isSameDay(day, pending.from)
          const isEnd = pending.to && isSameDay(day, pending.to)
          const inRange =
            pending.from && pending.to && isAfter(day, pending.from) && isBefore(day, pending.to)
          return (
            <button
              key={day.toISOString()}
              type="button"
              aria-label={format(day, 'EEEE, MMMM do, yyyy')}
              onClick={() => onSelect(day)}
              className={cn(
                'relative grid h-8 place-items-center text-sm text-foreground transition-colors hover:rounded-full hover:bg-accent focus:outline-none focus-visible:ring-1 focus-visible:ring-ring',
                inRange && 'bg-accent/70 hover:rounded-none',
                isStart &&
                  'z-10 rounded-full bg-primary text-primary-foreground shadow-[0_0_0_4px_hsl(var(--accent)/0.7)] hover:bg-primary',
                isEnd &&
                  'z-10 rounded-full bg-primary text-primary-foreground shadow-[0_0_0_4px_hsl(var(--accent)/0.7)] hover:bg-primary',
                isStart && isEnd && 'shadow-none',
                isSameDay(day, new Date()) && !isStart && !isEnd && 'font-semibold text-primary',
                !isSameMonth(day, month) && 'text-muted-foreground opacity-40',
              )}
            >
              {format(day, 'd')}
            </button>
          )
        })}
      </div>
    </div>
  )
}

export function DateRangePicker({ value, onChange }: DateRangePickerProps) {
  const [open, setOpen] = useState(false)
  const [dropdownPos, setDropdownPos] = useState<{ x: number; y: number } | null>(null)
  const [pending, setPending] = useState<{ from?: Date; to?: Date }>({})
  const [viewMonth, setViewMonth] = useState(() => startOfMonth(value.from ?? new Date()))
  const [activePreset, setActivePreset] = useState<(typeof PRESETS)[number] | null>(null)
  const btnRef = useRef<HTMLButtonElement>(null)

  const closePicker = ({ restoreFocus = false } = {}) => {
    setOpen(false)
    if (restoreFocus) {
      btnRef.current?.focus()
    }
  }

  const openPicker = () => {
    const rect = btnRef.current?.getBoundingClientRect()
    if (rect) setDropdownPos(dropdownPosition(rect))
    setPending({ from: value.from, to: value.to })
    setViewMonth(startOfMonth(value.from ?? new Date()))
    setActivePreset(value.from || value.to ? 'Custom' : null)
    setOpen(true)
  }

  useEffect(() => {
    if (!open) return
    const updateDropdownPos = () => {
      const rect = btnRef.current?.getBoundingClientRect()
      if (rect) setDropdownPos(dropdownPosition(rect))
    }
    const handler = (e: MouseEvent) => {
      const target = e.target as Node
      const portal = document.getElementById('date-picker-portal')
      const btn = btnRef.current
      if (!portal?.contains(target) && !btn?.contains(target)) closePicker()
    }
    const keyHandler = (e: globalThis.KeyboardEvent) => {
      if (e.key === 'Escape') closePicker({ restoreFocus: true })
    }
    updateDropdownPos()
    window.addEventListener('resize', updateDropdownPos)
    window.addEventListener('scroll', updateDropdownPos, true)
    document.addEventListener('mousedown', handler)
    document.addEventListener('keydown', keyHandler)
    return () => {
      window.removeEventListener('resize', updateDropdownPos)
      window.removeEventListener('scroll', updateDropdownPos, true)
      document.removeEventListener('mousedown', handler)
      document.removeEventListener('keydown', keyHandler)
    }
  }, [open])

  const hasRange = value.from || value.to
  const rightMonth = useMemo(() => addMonths(viewMonth, 1), [viewMonth])

  const selectDay = (day: Date) => {
    setActivePreset('Custom')
    if (!pending.from || (pending.from && pending.to)) {
      setPending({ from: withStartTime(day, pending.from), to: undefined })
      return
    }
    if (isBefore(day, pending.from)) {
      setPending({
        from: withStartTime(day, pending.from),
        to: withEndTime(pending.from, pending.to),
      })
      return
    }
    setPending({ from: pending.from, to: withEndTime(day, pending.to) })
  }

  const applyPreset = (preset: (typeof PRESETS)[number]) => {
    const today = new Date()
    setActivePreset(preset)
    if (preset === '7D') {
      const to = withEndTime(today)
      setPending({ from: withStartTime(subDays(today, 6)), to })
      setViewMonth(startOfMonth(subDays(today, 6)))
      return
    }
    if (preset === '1M') {
      const from = subDays(today, 29)
      setPending({ from: withStartTime(from), to: withEndTime(today) })
      setViewMonth(startOfMonth(from))
      return
    }
    if (preset === 'This month') {
      const from = startOfMonth(today)
      setPending({ from: withStartTime(from), to: withEndTime(endOfMonth(today)) })
      setViewMonth(from)
      return
    }
    if (preset === 'Last month') {
      const from = startOfMonth(subMonths(today, 1))
      setPending({ from: withStartTime(from), to: withEndTime(endOfMonth(from)) })
      setViewMonth(from)
      return
    }
    if (preset === 'FY') {
      const range = fiscalYearRange(today)
      setPending(range)
      setViewMonth(startOfMonth(range.from))
      return
    }
    setViewMonth(startOfMonth(pending.from ?? today))
  }

  const apply = () => {
    onChange(pending)
    setOpen(false)
  }

  const clear = () => {
    onChange({})
    setOpen(false)
  }

  const clearPending = () => {
    setPending({})
    setActivePreset(null)
    onChange({})
  }

  const pendingFrom = pending.from ?? new Date(new Date().setHours(0, 0, 0, 0))

  return (
    <>
      <button
        ref={btnRef}
        onClick={() => (open ? setOpen(false) : openPicker())}
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
              clear()
            }}
            className="ml-1 grid h-4 w-4 place-items-center rounded-full text-muted-foreground transition-colors hover:bg-primary/15 hover:text-primary"
          >
            <X size={12} strokeWidth={2.5} />
          </span>
        )}
      </button>

      {open &&
        dropdownPos &&
        createPortal(
          <div
            id="date-picker-portal"
            className="fixed z-50 w-[min(44rem,calc(100vw-2rem))] rounded-xl border border-border bg-card shadow-xl"
            style={{ left: dropdownPos.x, top: dropdownPos.y }}
          >
            <div className="flex flex-wrap items-center gap-2 border-b border-border bg-secondary/60 px-3 py-2">
              {PRESETS.map((preset) => (
                <button
                  key={preset}
                  type="button"
                  aria-pressed={activePreset === preset}
                  onClick={() => applyPreset(preset)}
                  className={cn(
                    'rounded-full border px-3 py-1.5 text-xs transition-colors hover:border-primary hover:bg-accent',
                    activePreset === preset
                      ? 'border-primary bg-primary/10 text-primary'
                      : 'border-border bg-card text-foreground',
                  )}
                >
                  {preset}
                </button>
              ))}
            </div>

            <div className="p-4">
              <div className="mb-3 grid grid-cols-1 gap-5 md:grid-cols-2">
                <div className="grid grid-cols-[2.5rem_1fr_2.5rem] items-center gap-2">
                  <button
                    type="button"
                    aria-label="Previous month"
                    onClick={() => setViewMonth((month) => subMonths(month, 1))}
                    className="grid h-10 w-10 place-items-center rounded-full border border-border bg-secondary text-foreground transition-colors hover:bg-accent"
                  >
                    <ChevronLeft className="h-5 w-5" />
                  </button>
                  <h3 className="text-center text-base font-semibold text-foreground">
                    {format(viewMonth, 'MMMM yyyy')}
                  </h3>
                  <span aria-hidden="true" />
                </div>
                <div className="hidden grid-cols-[2.5rem_1fr_2.5rem] items-center gap-2 md:grid">
                  <span aria-hidden="true" />
                  <h3 className="text-center text-base font-semibold text-foreground">
                    {format(rightMonth, 'MMMM yyyy')}
                  </h3>
                  <button
                    type="button"
                    aria-label="Next month"
                    onClick={() => setViewMonth((month) => addMonths(month, 1))}
                    className="grid h-10 w-10 place-items-center rounded-full border border-border bg-secondary text-foreground transition-colors hover:bg-accent"
                  >
                    <ChevronRight className="h-5 w-5" />
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-1 gap-5 md:grid-cols-2">
                <CalendarMonth month={viewMonth} pending={pending} onSelect={selectDay} />
                <div className="hidden md:block">
                  <CalendarMonth month={rightMonth} pending={pending} onSelect={selectDay} />
                </div>
              </div>

              <div
                className={cn(
                  'mt-4 flex flex-wrap items-center gap-3 border-t border-border pt-3',
                  pending.from ? 'justify-between' : 'justify-end',
                )}
              >
                {pending.from && (
                  <TimeRangeEditor
                    from={pendingFrom}
                    to={pending.to}
                    onChange={(range) => {
                      setActivePreset('Custom')
                      setPending(range)
                    }}
                  />
                )}
                <div className="flex items-center gap-3">
                  <button
                    type="button"
                    onClick={clearPending}
                    disabled={!pending.from}
                    className="text-xs text-muted-foreground transition-colors hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    Clear
                  </button>
                  <button
                    type="button"
                    onClick={apply}
                    disabled={!pending.from}
                    className={cn(
                      'rounded-md px-4 py-2 text-xs font-medium transition-colors',
                      pending.from
                        ? 'bg-primary text-primary-foreground hover:bg-primary/90'
                        : 'cursor-not-allowed bg-secondary text-muted-foreground opacity-50',
                    )}
                  >
                    Apply
                  </button>
                </div>
              </div>
            </div>
          </div>,
          document.body,
        )}
    </>
  )
}
