import {
  useActiveReader,
  useAdminScanningSettings,
  useClearReaderCheckpoint,
  usePreferences,
  useReaderCheckpoint,
  useRescan,
  useSession,
  useUpdateAdminScanningSettings,
  useUpdatePreferences,
} from '@/api/queries'
import type { PreferencesPatch } from '@/api/types'
import { cn } from '@/lib/utils'
import { useCallback, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useDisplay } from '@/contexts/DisplayContext'
import { formatDate } from '@/lib/utils'
import { GeneralSettings } from './settings/GeneralSettings'
import { SyncSettings } from './settings/SyncSettings'
import { useI18n } from '@/i18n/I18nProvider'
import type { MessageKey } from '@/i18n/messages'
import { AccountSettings, AdminUsersSection } from './settings/AccountSettings'

type SettingsTab = 'general' | 'daemon' | 'sync' | 'account' | 'admin'

const TABS: { id: SettingsTab; labelKey: MessageKey }[] = [
  { id: 'general', labelKey: 'nav.settings.general.subtitle' },
  { id: 'daemon', labelKey: 'nav.settings.daemon.subtitle' },
  { id: 'account', labelKey: 'nav.settings.account.subtitle' },
  { id: 'sync', labelKey: 'nav.settings.sync.subtitle' },
  { id: 'admin', labelKey: 'nav.settings.admin.subtitle' },
]

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

function DaemonSettings() {
  const { t } = useI18n()
  const { data: activeReader } = useActiveReader()
  const {
    mutate: rescan,
    isPending: rescanning,
    data: rescanResult,
    reset: resetRescan,
  } = useRescan()
  const reader = activeReader ?? ''

  const { data: preferences } = usePreferences()
  const scanInterval = preferences?.scan_interval ?? 60
  const lookbackDays = preferences?.lookback_days ?? 180
  const { mutate: updatePreferences } = useUpdatePreferences()
  const { data: checkpoint } = useReaderCheckpoint(reader)
  const { mutate: clearCheckpoint, isPending: clearing } = useClearReaderCheckpoint()
  const { timezone, timeFormat } = useDisplay()

  const [intervalDraft, setIntervalDraft] = useState<string | null>(null)
  const [lookbackDraft, setLookbackDraft] = useState<string | null>(null)
  const [scanError, setScanError] = useState<string | null>(null)

  // Sync drafts when server values arrive
  const intervalVal = intervalDraft ?? String(scanInterval)
  const lookbackVal = lookbackDraft ?? String(lookbackDays)

  const save = useCallback(() => {
    setScanError(null)
    const interval = parseInt(intervalVal, 10)
    const lookback = parseInt(lookbackVal, 10)
    if (isNaN(interval) || interval < 10 || interval > 3600) {
      setScanError(t('settings.daemon.scanIntervalError'))
      return
    }
    if (isNaN(lookback) || lookback < 1 || lookback > 3650) {
      setScanError(t('settings.daemon.lookbackError'))
      return
    }
    const patch: PreferencesPatch = {}
    if (interval !== scanInterval) patch.scan_interval = interval
    if (lookback !== lookbackDays) patch.lookback_days = lookback
    if (Object.keys(patch).length > 0) updatePreferences(patch)
  }, [intervalVal, lookbackDays, lookbackVal, scanInterval, t, updatePreferences])

  const handleFieldChange = (setter: (value: string) => void, value: string) => {
    setter(value)
    const interval = parseInt(intervalVal, 10)
    const lookback = parseInt(lookbackVal, 10)
    if (
      scanError &&
      !isNaN(interval) &&
      interval >= 10 &&
      interval <= 3600 &&
      !isNaN(lookback) &&
      lookback >= 1 &&
      lookback <= 3650
    ) {
      setScanError(null)
    }
  }

  return (
    <div className="space-y-8">
      {/* Scan settings */}
      <div className="space-y-6">
        <SettingField
          label={t('settings.daemon.scanIntervalLabel')}
          hint={t('settings.daemon.scanIntervalHint')}
        >
          <div className="flex items-center gap-2">
            <input
              type="text"
              inputMode="numeric"
              value={intervalVal}
              onChange={(e) => {
                handleFieldChange(setIntervalDraft, e.target.value)
              }}
              onBlur={save}
              className="w-24 rounded-md border border-border bg-input px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring"
            />
            <span className="shrink-0 text-xs text-muted-foreground">
              {t('settings.daemon.scanUnit')}
            </span>
          </div>
        </SettingField>

        <SettingField
          label={t('settings.daemon.lookbackLabel')}
          hint={t('settings.daemon.lookbackHint')}
        >
          <div className="flex items-center gap-2">
            <input
              type="text"
              inputMode="numeric"
              value={lookbackVal}
              onChange={(e) => {
                handleFieldChange(setLookbackDraft, e.target.value)
              }}
              onBlur={save}
              className="w-24 rounded-md border border-border bg-input px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring"
            />
            <span className="shrink-0 text-xs text-muted-foreground">
              {t('settings.daemon.lookbackUnit')}
            </span>
          </div>
        </SettingField>

        {scanError && <p className="text-xs text-destructive">{scanError}</p>}
      </div>

      {/* Checkpoint */}
      <div>
        <h2 className="mb-1 text-sm font-medium text-foreground">
          {t('settings.daemon.checkpointTitle')}
        </h2>
        <p className="mb-4 text-xs text-muted-foreground">{t('settings.daemon.checkpointHint')}</p>
        <div className="rounded-lg border border-border p-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs text-muted-foreground">{t('settings.daemon.lastScan')}</p>
              <p className="font-mono text-sm text-foreground">
                {checkpoint
                  ? formatDate(checkpoint, true, timezone, timeFormat)
                  : t('settings.daemon.noCheckpoint')}
              </p>
            </div>
            {checkpoint && (
              <button
                onClick={() => clearCheckpoint(reader)}
                disabled={clearing || !reader}
                className={cn(
                  'rounded-md border border-border px-3 py-1.5 text-xs transition-colors',
                  clearing || !reader
                    ? 'cursor-not-allowed opacity-50'
                    : 'text-muted-foreground hover:border-destructive hover:text-destructive',
                )}
              >
                {clearing ? t('settings.daemon.clearing') : t('settings.daemon.clearCheckpoint')}
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Force rescan */}
      <div
        data-testid="force-rescan-row"
        className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between"
      >
        <div className="min-w-0">
          <h2 className="mb-1 text-sm font-medium text-foreground">
            {t('settings.daemon.forceRescanTitle')}
          </h2>
          <p className="text-xs text-muted-foreground">{t('settings.daemon.forceRescanHint')}</p>
        </div>
        <div className="flex flex-shrink-0 items-center gap-4">
          <button
            onClick={() => {
              resetRescan()
              rescan(reader)
            }}
            disabled={rescanning || !reader}
            className={cn(
              'rounded-md px-4 py-2 text-sm transition-colors',
              rescanning || !reader
                ? 'cursor-not-allowed bg-secondary text-muted-foreground opacity-50'
                : 'bg-primary text-primary-foreground hover:bg-primary/90',
            )}
          >
            {rescanning ? t('settings.daemon.requesting') : t('settings.daemon.forceRescan')}
          </button>
          {rescanResult && (
            <span className="text-xs text-muted-foreground">
              {rescanResult.status === 'rescanning'
                ? t('settings.daemon.rescanStarted')
                : t('settings.daemon.rescanQueued')}
            </span>
          )}
          {!reader && (
            <span className="text-xs text-muted-foreground">
              {t('settings.daemon.noActiveReader')}
            </span>
          )}
        </div>
      </div>
    </div>
  )
}

function AdminSettings() {
  const { t } = useI18n()
  const { data } = useAdminScanningSettings()
  const update = useUpdateAdminScanningSettings()
  const current = data?.max_concurrent_scans ?? 4
  const [draft, setDraft] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const value = draft ?? String(current)

  const save = useCallback(() => {
    setMessage(null)
    const parsed = parseInt(value, 10)
    if (isNaN(parsed) || parsed < 1 || parsed > 64) {
      setMessage(t('settings.admin.scanningConcurrencyError'))
      return
    }
    if (parsed === current) return
    update.mutate(
      { max_concurrent_scans: parsed },
      { onSuccess: () => setMessage(t('settings.admin.saved')) },
    )
  }, [current, t, update, value])

  return (
    <div className="space-y-8">
      <div>
        <h2 className="mb-1 text-sm font-medium text-foreground">
          {t('settings.admin.scanningTitle')}
        </h2>
        <p className="mb-4 text-xs text-muted-foreground">{t('settings.admin.scanningHint')}</p>
        <div className="flex items-center gap-3">
          <input
            type="text"
            inputMode="numeric"
            value={value}
            onChange={(event) => {
              setDraft(event.target.value)
              setMessage(null)
            }}
            onBlur={save}
            className="w-24 rounded-md border border-border bg-input px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring"
          />
          {update.isPending && (
            <span className="text-xs text-muted-foreground">{t('settings.admin.saving')}</span>
          )}
          {message && <span className="text-xs text-muted-foreground">{message}</span>}
        </div>
      </div>

      <AdminUsersSection />
    </div>
  )
}

export default function Settings() {
  const [searchParams, setSearchParams] = useSearchParams()
  const { t } = useI18n()
  const { data: session } = useSession()
  const tabs = session?.role === 'admin' ? TABS : TABS.filter((item) => item.id !== 'admin')
  const rawTab = searchParams.get('tab')
  const tab: SettingsTab = tabs.some((t) => t.id === rawTab) ? (rawTab as SettingsTab) : 'general'

  const setTab = (id: SettingsTab) => setSearchParams({ tab: id }, { replace: true })

  return (
    <div className="mx-auto w-full max-w-4xl px-6 py-6">
      <h1 className="mb-6 text-lg font-semibold text-foreground">{t('nav.settings')}</h1>
      <div className="mb-6 flex gap-1 border-b border-border">
        {tabs.map((item) => (
          <button
            key={item.id}
            onClick={() => setTab(item.id)}
            className={cn(
              '-mb-px border-b-2 px-4 py-2 text-sm transition-colors',
              tab === item.id
                ? 'border-primary font-medium text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            )}
          >
            {t(item.labelKey)}
          </button>
        ))}
      </div>
      {tab === 'general' && <GeneralSettings />}
      {tab === 'daemon' && <DaemonSettings />}
      {tab === 'sync' && <SyncSettings />}
      {tab === 'account' && <AccountSettings />}
      {tab === 'admin' && <AdminSettings />}
    </div>
  )
}
