import { api } from '@/api/client'
import {
  useSetTimeFormat,
  useSetTimezone,
  useTimeFormat,
  useTimezone,
  useVersion,
} from '@/api/queries'
import { TIME_FORMATS, type TimeFormatValue } from '@/contexts/DisplayContext'
import { getBrowserTimezone, getTimezoneOptions, normalizeTimezone } from '@/lib/timezone'
import { cn } from '@/lib/utils'
import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

export const COMMON_CURRENCIES = [
  { code: 'INR', name: 'Indian Rupee', symbol: '₹' },
  { code: 'USD', name: 'US Dollar', symbol: '$' },
  { code: 'EUR', name: 'Euro', symbol: '€' },
  { code: 'GBP', name: 'British Pound', symbol: '£' },
  { code: 'AUD', name: 'Australian Dollar', symbol: 'A$' },
  { code: 'CAD', name: 'Canadian Dollar', symbol: 'C$' },
  { code: 'SGD', name: 'Singapore Dollar', symbol: 'S$' },
  { code: 'AED', name: 'UAE Dirham', symbol: 'د.إ' },
  { code: 'JPY', name: 'Japanese Yen', symbol: '¥' },
  { code: 'CHF', name: 'Swiss Franc', symbol: 'CHF' },
]

const ALL_TIMEZONES = getTimezoneOptions()

function SettingField({
  label,
  hint,
  children,
}: {
  label: string
  hint: string
  children: React.ReactNode
}) {
  return (
    <div>
      <label className="mb-1.5 block text-xs uppercase tracking-wider text-muted-foreground">
        {label}
      </label>
      {children}
      <p className="mt-1 text-xs text-muted-foreground">{hint}</p>
    </div>
  )
}

export function CurrencyCombobox({
  value,
  onChange,
}: {
  value: string
  onChange: (v: string) => void
}) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [pos, setPos] = useState<{ x: number; y: number } | null>(null)
  const btnRef = useRef<HTMLButtonElement>(null)

  const allOptions = COMMON_CURRENCIES
  const filtered = query
    ? allOptions.filter(
        (c) =>
          c.code.toLowerCase().includes(query.toLowerCase()) ||
          c.name.toLowerCase().includes(query.toLowerCase()),
      )
    : allOptions

  const selected = allOptions.find((c) => c.code === value)

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      const portal = document.getElementById('currency-portal')
      if (!portal?.contains(e.target as Node) && !btnRef.current?.contains(e.target as Node))
        setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  const openDropdown = () => {
    const rect = btnRef.current?.getBoundingClientRect()
    if (rect) setPos({ x: rect.left, y: rect.bottom + 4 })
    setQuery('')
    setOpen(true)
  }

  return (
    <>
      <button
        ref={btnRef}
        onClick={() => (open ? setOpen(false) : openDropdown())}
        className="flex w-full items-center justify-between rounded-md border border-border bg-input px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring"
      >
        <span>
          {selected?.symbol && (
            <span className="mr-1.5 font-mono text-muted-foreground">{selected.symbol}</span>
          )}
          <span className="font-mono">{value}</span>
          {selected?.name && <span className="ml-2 text-muted-foreground">— {selected.name}</span>}
        </span>
        <span className="text-xs text-muted-foreground">▾</span>
      </button>

      {open &&
        pos &&
        createPortal(
          <div
            id="currency-portal"
            className="fixed z-50 w-64 rounded-lg border border-border bg-card shadow-xl"
            style={{ left: pos.x, top: pos.y }}
          >
            <div className="border-b border-border px-3 py-2">
              <input
                autoFocus
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Search currency…"
                className="w-full bg-transparent text-sm text-foreground placeholder:text-muted-foreground focus:outline-none"
              />
            </div>
            <ul className="max-h-48 overflow-y-auto py-1">
              {filtered.map((c) => (
                <li
                  key={c.code}
                  onMouseDown={() => {
                    onChange(c.code)
                    setOpen(false)
                  }}
                  className={cn(
                    'cursor-pointer px-3 py-1.5 text-sm hover:bg-accent',
                    c.code === value && 'text-primary',
                  )}
                >
                  <span className="mr-1.5 font-mono text-muted-foreground">{c.symbol}</span>
                  <span className="font-mono">{c.code}</span>
                  <span className="ml-2 text-xs text-muted-foreground">{c.name}</span>
                </li>
              ))}
              {filtered.length === 0 && (
                <li className="px-3 py-2 text-xs text-muted-foreground">No match</li>
              )}
            </ul>
          </div>,
          document.body,
        )}
    </>
  )
}

export function TimezoneCombobox({
  value,
  onChange,
}: {
  value: string
  onChange: (v: string) => void
}) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [pos, setPos] = useState<{ x: number; y: number } | null>(null)
  const btnRef = useRef<HTMLButtonElement>(null)
  const normalizedValue = normalizeTimezone(value)

  const filtered = query
    ? ALL_TIMEZONES.filter((tz) => tz.toLowerCase().includes(query.toLowerCase()))
    : ALL_TIMEZONES
  const selected = ALL_TIMEZONES.find((tz) => tz === normalizedValue)

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      const portal = document.getElementById('timezone-portal')
      if (!portal?.contains(e.target as Node) && !btnRef.current?.contains(e.target as Node))
        setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  const openDropdown = () => {
    const rect = btnRef.current?.getBoundingClientRect()
    if (rect) setPos({ x: rect.left, y: rect.bottom + 4 })
    setQuery('')
    setOpen(true)
  }

  return (
    <>
      <button
        ref={btnRef}
        onClick={() => (open ? setOpen(false) : openDropdown())}
        className="flex w-full items-center justify-between rounded-md border border-border bg-input px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring"
      >
        <span className="font-mono">{selected ?? normalizedValue}</span>
        <span className="text-xs text-muted-foreground">▾</span>
      </button>

      {open &&
        pos &&
        createPortal(
          <div
            id="timezone-portal"
            className="fixed z-50 w-72 rounded-lg border border-border bg-card shadow-xl"
            style={{ left: pos.x, top: pos.y }}
          >
            <div className="border-b border-border px-3 py-2">
              <input
                autoFocus
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Search timezone…"
                className="w-full bg-transparent text-sm text-foreground placeholder:text-muted-foreground focus:outline-none"
              />
            </div>
            <ul className="max-h-56 overflow-y-auto py-1">
              {filtered.slice(0, 100).map((tz) => (
                <li
                  key={tz}
                  onMouseDown={() => {
                    onChange(tz)
                    setOpen(false)
                  }}
                  className={cn(
                    'cursor-pointer px-3 py-1.5 text-sm hover:bg-accent',
                    tz === normalizedValue && 'text-primary',
                  )}
                >
                  {tz}
                </li>
              ))}
              {filtered.length > 100 && (
                <li className="px-3 py-2 text-xs text-muted-foreground">
                  {filtered.length - 100} more — refine your search
                </li>
              )}
              {filtered.length === 0 && (
                <li className="px-3 py-2 text-xs text-muted-foreground">No match</li>
              )}
            </ul>
          </div>,
          document.body,
        )}
    </>
  )
}

export function TimeFormatSelect({
  value,
  onChange,
}: {
  value: TimeFormatValue
  onChange: (v: TimeFormatValue) => void
}) {
  const [open, setOpen] = useState(false)
  const [pos, setPos] = useState<{ x: number; y: number } | null>(null)
  const btnRef = useRef<HTMLButtonElement>(null)

  const selected = TIME_FORMATS.find((f) => f.value === value) ?? TIME_FORMATS[0]

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      const portal = document.getElementById('timeformat-portal')
      if (!portal?.contains(e.target as Node) && !btnRef.current?.contains(e.target as Node))
        setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  const openDropdown = () => {
    const rect = btnRef.current?.getBoundingClientRect()
    if (rect) setPos({ x: rect.left, y: rect.bottom + 4 })
    setOpen(true)
  }

  return (
    <>
      <button
        ref={btnRef}
        onClick={() => (open ? setOpen(false) : openDropdown())}
        className="flex w-full items-center justify-between rounded-md border border-border bg-input px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring"
      >
        <span className="font-mono">{selected.label}</span>
        <span className="text-xs text-muted-foreground">▾</span>
      </button>

      {open &&
        pos &&
        createPortal(
          <div
            id="timeformat-portal"
            className="fixed z-50 w-72 rounded-lg border border-border bg-card shadow-xl"
            style={{ left: pos.x, top: pos.y }}
          >
            <ul className="py-1">
              {TIME_FORMATS.map((f) => (
                <li
                  key={f.value}
                  onMouseDown={() => {
                    onChange(f.value)
                    setOpen(false)
                  }}
                  className={cn(
                    'cursor-pointer px-3 py-1.5 text-sm hover:bg-accent',
                    f.value === value && 'text-primary',
                  )}
                >
                  <span className="font-mono">{f.label}</span>
                </li>
              ))}
            </ul>
          </div>,
          document.body,
        )}
    </>
  )
}

function VersionSection() {
  const { data: version } = useVersion()
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    if (!version) return
    void navigator.clipboard.writeText(version).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  return (
    <div className="border-t border-border pt-6">
      <p className="mb-3 text-xs uppercase tracking-wider text-muted-foreground">Version</p>
      <div className="rounded-lg border border-border bg-card px-4 py-3">
        <div className="flex items-center justify-between">
          <div>
            <span className="text-sm font-medium text-foreground">Expensor</span>
            <span className="ml-2 font-mono text-sm text-muted-foreground">{version ?? '—'}</span>
          </div>
          <button
            onClick={handleCopy}
            disabled={!version}
            className="rounded border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:border-primary hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40"
          >
            {copied ? 'Copied!' : 'Copy'}
          </button>
        </div>
      </div>
    </div>
  )
}

export function GeneralSettings() {
  const [currency, setCurrency] = useState('INR')
  const [loading, setLoading] = useState(true)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const { data: savedTimezone } = useTimezone()
  const { data: savedTimeFormat } = useTimeFormat()
  const { mutate: setTimezone } = useSetTimezone()
  const { mutate: setTimeFormat } = useSetTimeFormat()

  const [timezone, setTimezoneDraft] = useState<string>(getBrowserTimezone())
  const [timeFormat, setTimeFormatDraft] = useState<TimeFormatValue>('HH:mm')

  useEffect(() => {
    if (savedTimezone) setTimezoneDraft(normalizeTimezone(savedTimezone))
  }, [savedTimezone])

  useEffect(() => {
    if (savedTimeFormat) setTimeFormatDraft(savedTimeFormat as TimeFormatValue)
  }, [savedTimeFormat])

  useEffect(() => {
    api.status.get().then((r) => {
      const c = r.data.stats?.base_currency
      if (c) setCurrency(c)
      setLoading(false)
    })
  }, [])

  const handleSave = async () => {
    setSaved(false)
    setError(null)
    try {
      await api.config.setBaseCurrency(currency)
      setTimezone(timezone)
      setTimeFormat(timeFormat)
      setSaved(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save')
    }
  }

  if (loading) return <p className="text-xs text-muted-foreground">Loading...</p>

  return (
    <div className="space-y-6">
      <SettingField label="Base currency" hint="Used for aggregate totals on the Dashboard.">
        <CurrencyCombobox
          value={currency}
          onChange={(v) => {
            setCurrency(v)
            setSaved(false)
          }}
        />
      </SettingField>

      <SettingField
        label="Timezone"
        hint="Used for date display and hour-of-day filtering across the app."
      >
        <TimezoneCombobox
          value={timezone}
          onChange={(v) => {
            setTimezoneDraft(v)
            setSaved(false)
          }}
        />
      </SettingField>

      <SettingField label="Time format" hint="Controls how times are displayed throughout the app.">
        <TimeFormatSelect
          value={timeFormat}
          onChange={(v) => {
            setTimeFormatDraft(v)
            setSaved(false)
          }}
        />
      </SettingField>

      <div className="space-y-2">
        <button
          onClick={handleSave}
          disabled={!currency}
          className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-40"
        >
          Save
        </button>
        {saved && <p className="text-xs text-success">Saved.</p>}
        {error && <p className="text-xs text-destructive">{error}</p>}
      </div>

      <VersionSection />
    </div>
  )
}
