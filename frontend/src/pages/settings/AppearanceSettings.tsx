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

export function AppearanceSettings() {
  const [currency, setCurrency] = useState('')
  const [scanInterval, setScanInterval] = useState('')
  const [lookbackDays, setLookbackDays] = useState('')
  const [loading, setLoading] = useState(true)
  const [saved, setSaved] = useState<string | null>(null)
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
      if (intervalRes.status === 'fulfilled') setScanInterval(intervalRes.value.data.scan_interval)
      if (lookbackRes.status === 'fulfilled') setLookbackDays(lookbackRes.value.data.lookback_days)
      setLoading(false)
    })
  }, [])

  const saveField = async (key: string, action: () => Promise<unknown>) => {
    setSaved(null)
    setError(null)
    try {
      await action()
      setSaved(key)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save')
    }
  }

  if (loading) return <p className="text-xs text-muted-foreground">Loading...</p>

  return (
    <div className="max-w-sm space-y-6">
      {/* Base currency */}
      <div className="space-y-3">
        <SettingField
          label="Base currency"
          hint="3-letter ISO 4217 code used for aggregate totals (e.g. INR, USD, EUR)."
        >
          <input
            value={currency}
            onChange={(e) => {
              setCurrency(e.target.value)
              setSaved(null)
            }}
            placeholder="INR"
            maxLength={3}
            className="w-full rounded-md border border-border bg-input px-3 py-2 font-mono text-sm uppercase"
          />
        </SettingField>
        <button
          onClick={() =>
            saveField('currency', () => api.config.setBaseCurrency(currency.toUpperCase().trim()))
          }
          disabled={currency.length !== 3}
          className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-40"
        >
          Save
        </button>
        {saved === 'currency' && <p className="text-xs text-success">Saved.</p>}
      </div>

      <div className="space-y-3 border-t border-border pt-6">
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
                setSaved(null)
              }}
              min={10}
              max={3600}
              className="w-full rounded-md border border-border bg-input px-3 py-2 text-sm"
            />
            <span className="flex-shrink-0 text-xs text-muted-foreground">seconds</span>
          </div>
        </SettingField>
        <button
          onClick={() => {
            const n = parseInt(scanInterval, 10)
            saveField('scan', () => api.config.setScanInterval(n))
          }}
          disabled={
            !scanInterval ||
            isNaN(parseInt(scanInterval, 10)) ||
            parseInt(scanInterval, 10) < 10 ||
            parseInt(scanInterval, 10) > 3600
          }
          className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-40"
        >
          Save
        </button>
        {saved === 'scan' && (
          <p className="text-xs text-success">Saved. Takes effect on next daemon start.</p>
        )}
      </div>

      <div className="space-y-3 border-t border-border pt-6">
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
                setSaved(null)
              }}
              min={1}
              max={3650}
              className="w-full rounded-md border border-border bg-input px-3 py-2 text-sm"
            />
            <span className="flex-shrink-0 text-xs text-muted-foreground">days</span>
          </div>
        </SettingField>
        <button
          onClick={() => {
            const n = parseInt(lookbackDays, 10)
            saveField('lookback', () => api.config.setLookbackDays(n))
          }}
          disabled={
            !lookbackDays ||
            isNaN(parseInt(lookbackDays, 10)) ||
            parseInt(lookbackDays, 10) < 1 ||
            parseInt(lookbackDays, 10) > 3650
          }
          className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-40"
        >
          Save
        </button>
        {saved === 'lookback' && (
          <p className="text-xs text-success">Saved. Takes effect on next daemon start.</p>
        )}
      </div>

      {error && <p className="text-xs text-destructive">{error}</p>}
    </div>
  )
}
