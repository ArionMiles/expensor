import {
  useActiveReader,
  useClearReaderCheckpoint,
  useLookbackDays,
  useReaderCheckpoint,
  useRescan,
  useScanInterval,
  useSetLookbackDays,
  useSetScanInterval,
} from '@/api/queries'
import { cn } from '@/lib/utils'
import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useDisplay } from '@/contexts/DisplayContext'
import { formatDate } from '@/lib/utils'
import { GeneralSettings } from './settings/GeneralSettings'
import { SyncSettings } from './settings/SyncSettings'
import { useI18n } from '@/i18n/I18nProvider'
import type { MessageKey } from '@/i18n/messages'

type SettingsTab = 'general' | 'daemon' | 'sync'

const TABS: { id: SettingsTab; labelKey: MessageKey }[] = [
  { id: 'general', labelKey: 'nav.settings.general.subtitle' },
  { id: 'daemon', labelKey: 'nav.settings.daemon.subtitle' },
  { id: 'sync', labelKey: 'nav.settings.sync.subtitle' },
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

  const { data: scanInterval = 60 } = useScanInterval()
  const { data: lookbackDays = 180 } = useLookbackDays()
  const { mutate: setScanInterval } = useSetScanInterval()
  const { mutate: setLookbackDays } = useSetLookbackDays()
  const { data: checkpoint } = useReaderCheckpoint(reader)
  const { mutate: clearCheckpoint, isPending: clearing } = useClearReaderCheckpoint()
  const { timezone, timeFormat } = useDisplay()

  const [intervalDraft, setIntervalDraft] = useState<string>('')
  const [lookbackDraft, setLookbackDraft] = useState<string>('')
  const [scanSaved, setScanSaved] = useState(false)
  const [scanError, setScanError] = useState<string | null>(null)

  // Sync drafts when server values arrive
  const intervalVal = intervalDraft !== '' ? intervalDraft : String(scanInterval)
  const lookbackVal = lookbackDraft !== '' ? lookbackDraft : String(lookbackDays)

  const handleScanSave = () => {
    setScanSaved(false)
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
    if (interval !== scanInterval) setScanInterval(interval)
    if (lookback !== lookbackDays) setLookbackDays(lookback)
    setScanSaved(true)
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
                setIntervalDraft(e.target.value)
                setScanSaved(false)
              }}
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
                setLookbackDraft(e.target.value)
                setScanSaved(false)
              }}
              className="w-24 rounded-md border border-border bg-input px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring"
            />
            <span className="shrink-0 text-xs text-muted-foreground">
              {t('settings.daemon.lookbackUnit')}
            </span>
          </div>
        </SettingField>

        <div className="space-y-2">
          <button
            onClick={handleScanSave}
            className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground transition-colors hover:bg-primary/90"
          >
            {t('common.save')}
          </button>
          {scanSaved && <p className="text-xs text-success">{t('settings.daemon.saved')}</p>}
          {scanError && <p className="text-xs text-destructive">{scanError}</p>}
        </div>
      </div>

      {/* Scanning checkpoint */}
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
      <div>
        <h2 className="mb-1 text-sm font-medium text-foreground">
          {t('settings.daemon.forceRescanTitle')}
        </h2>
        <p className="mb-4 text-xs text-muted-foreground">{t('settings.daemon.forceRescanHint')}</p>
        <div className="flex items-center gap-4">
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

export default function Settings() {
  const [searchParams, setSearchParams] = useSearchParams()
  const { t } = useI18n()
  const rawTab = searchParams.get('tab')
  const tab: SettingsTab = TABS.some((t) => t.id === rawTab) ? (rawTab as SettingsTab) : 'general'

  const setTab = (id: SettingsTab) => setSearchParams({ tab: id }, { replace: true })

  return (
    <div className="mx-auto w-full max-w-4xl px-6 py-6">
      <h1 className="mb-6 text-lg font-semibold text-foreground">{t('nav.settings')}</h1>
      <div className="mb-6 flex gap-1 border-b border-border">
        {TABS.map((item) => (
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
    </div>
  )
}
