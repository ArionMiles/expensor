import { api } from '@/api/client'
import { useEffect, useState } from 'react'

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

const numberInput =
  'w-full rounded-md border border-border bg-input px-3 py-2 text-sm ' +
  '[appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none'

export function AppearanceSettings() {
  const [currency, setCurrency] = useState('')
  const [scanInterval, setScanInterval] = useState('60')
  const [lookbackDays, setLookbackDays] = useState('180')
  const [loading, setLoading] = useState(true)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    Promise.allSettled([
      api.status.get(),
      api.config.getScanInterval(),
      api.config.getLookbackDays(),
    ]).then(([statusRes, intervalRes, lookbackRes]) => {
      if (statusRes.status === 'fulfilled') {
        const c = statusRes.value.data.stats?.base_currency
        if (c) setCurrency(c)
      }
      if (intervalRes.status === 'fulfilled' && intervalRes.value.data.scan_interval)
        setScanInterval(intervalRes.value.data.scan_interval)
      if (lookbackRes.status === 'fulfilled' && lookbackRes.value.data.lookback_days)
        setLookbackDays(lookbackRes.value.data.lookback_days)
      setLoading(false)
    })
  }, [])

  const interval = parseInt(scanInterval, 10)
  const lookback = parseInt(lookbackDays, 10)

  const isValid =
    currency.length === 3 &&
    !isNaN(interval) &&
    interval >= 10 &&
    interval <= 3600 &&
    !isNaN(lookback) &&
    lookback >= 1 &&
    lookback <= 3650

  const handleSave = async () => {
    setSaved(false)
    setError(null)
    try {
      await Promise.all([
        api.config.setBaseCurrency(currency.toUpperCase().trim()),
        api.config.setScanInterval(interval),
        api.config.setLookbackDays(lookback),
      ])
      setSaved(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save')
    }
  }

  if (loading) return <p className="text-xs text-muted-foreground">Loading...</p>

  return (
    <div className="max-w-sm space-y-6">
      <SettingField
        label="Base currency"
        hint="3-letter ISO 4217 code used for aggregate totals (e.g. INR, USD, EUR)."
      >
        <input
          value={currency}
          onChange={(e) => {
            setCurrency(e.target.value)
            setSaved(false)
          }}
          placeholder="INR"
          maxLength={3}
          className="w-full rounded-md border border-border bg-input px-3 py-2 font-mono text-sm uppercase"
        />
      </SettingField>

      <SettingField
        label="Scan interval"
        hint="How often the daemon checks for new emails (10–3600 seconds)."
      >
        <div className="flex items-center gap-2">
          <input
            type="number"
            value={scanInterval}
            onChange={(e) => {
              setScanInterval(e.target.value)
              setSaved(false)
            }}
            min={10}
            max={3600}
            className={numberInput}
          />
          <span className="flex-shrink-0 text-xs text-muted-foreground">seconds</span>
        </div>
      </SettingField>

      <SettingField
        label="Lookback days"
        hint="How far back to import emails on the first run (1–3650 days)."
      >
        <div className="flex items-center gap-2">
          <input
            type="number"
            value={lookbackDays}
            onChange={(e) => {
              setLookbackDays(e.target.value)
              setSaved(false)
            }}
            min={1}
            max={3650}
            className={numberInput}
          />
          <span className="flex-shrink-0 text-xs text-muted-foreground">days</span>
        </div>
      </SettingField>

      <div className="space-y-2">
        <button
          onClick={handleSave}
          disabled={!isValid}
          className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-40"
        >
          Save
        </button>
        {saved && (
          <p className="text-xs text-success">
            Saved. Scan interval and lookback days take effect on next daemon start.
          </p>
        )}
        {error && <p className="text-xs text-destructive">{error}</p>}
      </div>
    </div>
  )
}
