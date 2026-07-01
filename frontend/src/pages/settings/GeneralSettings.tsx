import { usePreferences, useUpdatePreferences, useVersion } from '@/api/queries'
import { TIME_FORMATS, type TimeFormatValue } from '@/contexts/DisplayContext'
import { FloatingDropdown, comboboxOptionClass, useComboboxNavigation } from '@/components/Combobox'
import { getBrowserTimezone, getTimezoneOptions, normalizeTimezone } from '@/lib/timezone'
import { useCopyTooltip } from '@/hooks/useCopyTooltip'
import { useI18n } from '@/i18n/I18nProvider'
import { cn } from '@/lib/utils'
import { Check, Copy } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'

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
  const containerRef = useRef<HTMLDivElement>(null)
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
  const navigation = useComboboxNavigation({
    open,
    optionCount: filtered.length,
    onOpenChange: setOpen,
    onSelectIndex: (index) => {
      const selectedCurrency = filtered[index]
      if (!selectedCurrency) return
      onChange(selectedCurrency.code)
      setOpen(false)
    },
  })
  const highlighted = navigation.highlightedIndex

  const openDropdown = () => {
    setQuery('')
    setOpen(true)
    navigation.resetHighlight()
  }

  return (
    <div ref={containerRef}>
      <button
        ref={btnRef}
        onClick={() => (open ? setOpen(false) : openDropdown())}
        {...navigation.getComboboxProps({
          'aria-label': selected?.name
            ? `Base currency ${value} ${selected.name}`
            : `Base currency ${value}`,
          listboxVisible: open,
        })}
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

      <FloatingDropdown
        open={open}
        anchorRef={btnRef}
        containerRef={containerRef}
        onOpenChange={setOpen}
        minWidth={256}
      >
        {(style, setPortalNode) => (
          <div
            ref={setPortalNode}
            className="fixed z-50 w-64 rounded-lg border border-border bg-card shadow-xl"
            style={style}
          >
            <div className="border-b border-border px-3 py-2">
              <input
                autoFocus
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value)
                  navigation.resetHighlight()
                }}
                onKeyDown={navigation.handleKeyDown}
                placeholder="Search currency…"
                className="w-full bg-transparent text-sm text-foreground placeholder:text-muted-foreground focus:outline-none"
              />
            </div>
            <ul id={navigation.listboxId} role="listbox" className="max-h-48 overflow-y-auto py-1">
              {filtered.map((c, index) => (
                <li
                  key={c.code}
                  {...navigation.getOptionProps(index, {
                    selected: c.code === value,
                    onMouseDown: () => {
                      onChange(c.code)
                      setOpen(false)
                    },
                  })}
                  className={cn(
                    comboboxOptionClass(index === highlighted, c.code === value),
                    'px-3 text-sm',
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
          </div>
        )}
      </FloatingDropdown>
    </div>
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
  const containerRef = useRef<HTMLDivElement>(null)
  const btnRef = useRef<HTMLButtonElement>(null)
  const normalizedValue = normalizeTimezone(value)

  const filtered = query
    ? ALL_TIMEZONES.filter((tz) => tz.toLowerCase().includes(query.toLowerCase()))
    : ALL_TIMEZONES
  const visibleOptions = filtered.slice(0, 100)
  const selected = ALL_TIMEZONES.find((tz) => tz === normalizedValue)
  const navigation = useComboboxNavigation({
    open,
    optionCount: visibleOptions.length,
    onOpenChange: setOpen,
    onSelectIndex: (index) => {
      const selectedTimezone = visibleOptions[index]
      if (!selectedTimezone) return
      onChange(selectedTimezone)
      setOpen(false)
    },
  })
  const highlighted = navigation.highlightedIndex

  const openDropdown = () => {
    setQuery('')
    setOpen(true)
    navigation.resetHighlight()
  }

  return (
    <div ref={containerRef}>
      <button
        ref={btnRef}
        onClick={() => (open ? setOpen(false) : openDropdown())}
        {...navigation.getComboboxProps({
          'aria-label': `Timezone ${selected ?? normalizedValue}`,
          listboxVisible: open,
        })}
        className="flex w-full items-center justify-between rounded-md border border-border bg-input px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring"
      >
        <span className="font-mono">{selected ?? normalizedValue}</span>
        <span className="text-xs text-muted-foreground">▾</span>
      </button>

      <FloatingDropdown
        open={open}
        anchorRef={btnRef}
        containerRef={containerRef}
        onOpenChange={setOpen}
        minWidth={288}
        maxHeight={224}
      >
        {(style, setPortalNode) => (
          <div
            ref={setPortalNode}
            className="fixed z-50 w-72 rounded-lg border border-border bg-card shadow-xl"
            style={style}
          >
            <div className="border-b border-border px-3 py-2">
              <input
                autoFocus
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value)
                  navigation.resetHighlight()
                }}
                onKeyDown={navigation.handleKeyDown}
                placeholder="Search timezone…"
                className="w-full bg-transparent text-sm text-foreground placeholder:text-muted-foreground focus:outline-none"
              />
            </div>
            <ul id={navigation.listboxId} role="listbox" className="max-h-56 overflow-y-auto py-1">
              {visibleOptions.map((tz, index) => (
                <li
                  key={tz}
                  {...navigation.getOptionProps(index, {
                    selected: tz === normalizedValue,
                    onMouseDown: () => {
                      onChange(tz)
                      setOpen(false)
                    },
                  })}
                  className={cn(
                    comboboxOptionClass(index === highlighted, tz === normalizedValue),
                    'px-3 text-sm',
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
          </div>
        )}
      </FloatingDropdown>
    </div>
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
  const containerRef = useRef<HTMLDivElement>(null)
  const btnRef = useRef<HTMLButtonElement>(null)

  const selected = TIME_FORMATS.find((f) => f.value === value) ?? TIME_FORMATS[0]
  const navigation = useComboboxNavigation({
    open,
    optionCount: TIME_FORMATS.length,
    onOpenChange: setOpen,
    onSelectIndex: (index) => {
      const selectedFormat = TIME_FORMATS[index]
      if (!selectedFormat) return
      onChange(selectedFormat.value)
      setOpen(false)
    },
  })
  const highlighted = navigation.highlightedIndex

  const openDropdown = () => {
    setOpen(true)
    navigation.resetHighlight()
  }

  return (
    <div ref={containerRef}>
      <button
        ref={btnRef}
        onClick={() => (open ? setOpen(false) : openDropdown())}
        {...navigation.getComboboxProps({
          'aria-label': `Time format ${selected.label}`,
          listboxVisible: open,
        })}
        className="flex w-full items-center justify-between rounded-md border border-border bg-input px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring"
      >
        <span className="font-mono">{selected.label}</span>
        <span className="text-xs text-muted-foreground">▾</span>
      </button>

      <FloatingDropdown
        open={open}
        anchorRef={btnRef}
        containerRef={containerRef}
        onOpenChange={setOpen}
        minWidth={288}
      >
        {(style, setPortalNode) => (
          <div
            ref={setPortalNode}
            className="fixed z-50 w-72 rounded-lg border border-border bg-card shadow-xl"
            style={style}
          >
            <ul id={navigation.listboxId} role="listbox" className="py-1">
              {TIME_FORMATS.map((f, index) => (
                <li
                  key={f.value}
                  {...navigation.getOptionProps(index, {
                    selected: f.value === value,
                    onMouseDown: () => {
                      onChange(f.value)
                      setOpen(false)
                    },
                  })}
                  className={cn(
                    comboboxOptionClass(index === highlighted, f.value === value),
                    'px-3 text-sm',
                  )}
                >
                  <span className="font-mono">{f.label}</span>
                </li>
              ))}
            </ul>
          </div>
        )}
      </FloatingDropdown>
    </div>
  )
}

function VersionSection() {
  const { t } = useI18n()
  const { data: version } = useVersion()
  const [copied, setCopied] = useState(false)
  const timerRef = useRef<number | null>(null)
  const { handlers, tip } = useCopyTooltip(
    copied ? t('settings.general.version.copied') : t('settings.general.version.copy'),
  )

  useEffect(
    () => () => {
      if (timerRef.current !== null) window.clearTimeout(timerRef.current)
    },
    [],
  )

  const handleCopy = () => {
    if (!version) return
    void navigator.clipboard?.writeText(version).then(() => {
      setCopied(true)
      if (timerRef.current !== null) window.clearTimeout(timerRef.current)
      timerRef.current = window.setTimeout(() => {
        setCopied(false)
        timerRef.current = null
      }, 2000)
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
            type="button"
            onClick={handleCopy}
            aria-label={t('settings.general.version.copy')}
            disabled={!version}
            className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40"
            {...handlers}
          >
            {copied ? <Check size={15} /> : <Copy size={15} />}
          </button>
          {tip}
        </div>
      </div>
    </div>
  )
}

export function GeneralSettings() {
  const [currency, setCurrency] = useState('INR')
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const { data: preferences, isLoading } = usePreferences()
  const updatePreferences = useUpdatePreferences()

  const [timezone, setTimezoneDraft] = useState<string>(getBrowserTimezone())
  const [timeFormat, setTimeFormatDraft] = useState<TimeFormatValue>('HH:mm')

  useEffect(() => {
    if (!preferences) return
    setCurrency(preferences.base_currency)
    if (preferences.timezone) setTimezoneDraft(normalizeTimezone(preferences.timezone))
    setTimeFormatDraft(preferences.time_format as TimeFormatValue)
  }, [preferences])

  const handleSave = async () => {
    setSaved(false)
    setError(null)
    try {
      await updatePreferences.mutateAsync({
        base_currency: currency,
        timezone,
        time_format: timeFormat,
      })
      setSaved(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save')
    }
  }

  if (isLoading) return <p className="text-xs text-muted-foreground">Loading...</p>

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
