import { queryKeys, useUpdatePreferences } from '@/api/queries'
import type { TimeFormatValue } from '@/contexts/DisplayContext'
import { useI18n } from '@/i18n/I18nProvider'
import { getBrowserTimezone, normalizeTimezone } from '@/lib/timezone'
import {
  CurrencyCombobox,
  TimeFormatSelect,
  TimezoneCombobox,
} from '@/pages/settings/GeneralSettings'
import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

function PreferenceField({
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
      <p className="mb-1.5 text-xs uppercase tracking-wider text-muted-foreground">{label}</p>
      {children}
      <p className="mt-1 text-xs text-muted-foreground">{hint}</p>
    </div>
  )
}

export function PreferencesStep({ onNext }: { onNext: () => void }) {
  const { t } = useI18n()
  const qc = useQueryClient()
  const updatePreferences = useUpdatePreferences()
  const [currency, setCurrency] = useState('USD')
  const [timezone, setTimezoneDraft] = useState(normalizeTimezone(getBrowserTimezone()))
  const [timeFormat, setTimeFormatDraft] = useState<TimeFormatValue>('h:mm a')
  const [scanInterval, setScanInterval] = useState('60')
  const [lookbackDays, setLookbackDays] = useState('180')
  const [error, setError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const handleSave = async () => {
    setError(null)
    const interval = Number.parseInt(scanInterval, 10)
    const lookback = Number.parseInt(lookbackDays, 10)
    if (Number.isNaN(interval) || interval < 10 || interval > 3600) {
      setError(t('settings.daemon.scanIntervalError'))
      return
    }
    if (Number.isNaN(lookback) || lookback < 1 || lookback > 3650) {
      setError(t('settings.daemon.lookbackError'))
      return
    }

    setSaving(true)
    try {
      await updatePreferences.mutateAsync({
        base_currency: currency,
        timezone,
        time_format: timeFormat,
        scan_interval: interval,
        lookback_days: lookback,
      })
      await qc.invalidateQueries({ queryKey: queryKeys.setupStatus })
      onNext()
    } catch (err) {
      setError(err instanceof Error ? err.message : t('setup.preferences.error'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="w-full max-w-lg">
      <div className="mb-8">
        <p className="mb-2 text-xs uppercase tracking-widest text-muted-foreground">Setup</p>
        <h1 className="mb-1 text-lg font-semibold text-foreground">
          {t('setup.preferences.title')}
        </h1>
        <p className="text-sm text-muted-foreground">{t('setup.preferences.summary')}</p>
      </div>

      <div className="rounded-lg border border-border bg-card p-6 shadow-sm">
        <div className="space-y-5">
          <PreferenceField
            label={t('setup.preferences.baseCurrency')}
            hint={t('setup.preferences.baseCurrencyHint')}
          >
            <CurrencyCombobox value={currency} onChange={setCurrency} />
          </PreferenceField>

          <PreferenceField
            label={t('setup.preferences.timezone')}
            hint={t('setup.preferences.timezoneHint')}
          >
            <TimezoneCombobox value={timezone} onChange={setTimezoneDraft} />
          </PreferenceField>

          <PreferenceField
            label={t('setup.preferences.timeFormat')}
            hint={t('setup.preferences.timeFormatHint')}
          >
            <TimeFormatSelect value={timeFormat} onChange={setTimeFormatDraft} />
          </PreferenceField>

          <PreferenceField
            label={t('settings.daemon.scanIntervalLabel')}
            hint={t('settings.daemon.scanIntervalHint')}
          >
            <div className="flex items-center gap-2">
              <input
                type="text"
                inputMode="numeric"
                value={scanInterval}
                onChange={(event) => setScanInterval(event.currentTarget.value)}
                className="w-24 rounded-md border border-border bg-input px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring"
              />
              <span className="shrink-0 text-xs text-muted-foreground">
                {t('settings.daemon.scanUnit')}
              </span>
            </div>
          </PreferenceField>

          <PreferenceField
            label={t('settings.daemon.lookbackLabel')}
            hint={t('settings.daemon.lookbackHint')}
          >
            <div className="flex items-center gap-2">
              <input
                type="text"
                inputMode="numeric"
                value={lookbackDays}
                onChange={(event) => setLookbackDays(event.currentTarget.value)}
                className="w-24 rounded-md border border-border bg-input px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring"
              />
              <span className="shrink-0 text-xs text-muted-foreground">
                {t('settings.daemon.lookbackUnit')}
              </span>
            </div>
          </PreferenceField>

          <div className="space-y-2">
            <button
              onClick={handleSave}
              disabled={
                saving || !currency || !timezone || !timeFormat || !scanInterval || !lookbackDays
              }
              className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-40"
            >
              {saving ? t('setup.preferences.saving') : t('setup.preferences.continue')}
            </button>
            {error && (
              <p className="text-xs text-destructive" role="alert">
                {error}
              </p>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
