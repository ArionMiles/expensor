import {
  useCommunitySyncSettings,
  useSyncStatus,
  useTriggerSync,
  useUpdateCommunitySyncSettings,
} from '@/api/queries'
import { useDisplay } from '@/contexts/DisplayContext'
import { useI18n } from '@/i18n/I18nProvider'
import { cn, formatDate } from '@/lib/utils'
import { useState } from 'react'

export function SyncSettings() {
  const { t } = useI18n()
  const { data: status, isLoading: statusLoading } = useSyncStatus()
  const { data: settings } = useCommunitySyncSettings()
  const updateSettings = useUpdateCommunitySyncSettings()
  const { mutate: triggerSync, isPending: syncing } = useTriggerSync()
  const [triggered, setTriggered] = useState(false)
  const { timezone, timeFormat } = useDisplay()
  const automaticSyncEnabled = settings?.automatic_sync_enabled ?? true

  const handleTrigger = () => {
    setTriggered(false)
    triggerSync(undefined, {
      onSuccess: () => setTriggered(true),
    })
  }

  return (
    <div className="space-y-8">
      <div
        data-testid="automatic-sync-row"
        className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between"
      >
        <div className="min-w-0">
          <h2 className="mb-1 text-sm font-medium text-foreground">
            {t('settings.sync.automaticTitle')}
          </h2>
          <p className="text-xs text-muted-foreground">{t('settings.sync.automaticHint')}</p>
        </div>
        <button
          type="button"
          role="switch"
          aria-label={t('settings.sync.automaticTitle')}
          aria-checked={automaticSyncEnabled}
          disabled={updateSettings.isPending}
          onClick={() => updateSettings.mutate({ automatic_sync_enabled: !automaticSyncEnabled })}
          className={cn(
            'inline-flex h-7 w-12 flex-shrink-0 items-center rounded-full border border-border p-0.5 transition-colors disabled:cursor-not-allowed disabled:opacity-60',
            automaticSyncEnabled ? 'bg-primary' : 'bg-secondary',
          )}
        >
          <span
            className={cn(
              'h-5 w-5 rounded-full bg-background shadow-sm transition-transform',
              automaticSyncEnabled && 'translate-x-5',
            )}
          />
        </button>
      </div>

      {/* Last sync status */}
      <div>
        <h2 className="mb-1 text-sm font-medium text-foreground">Sync status</h2>
        <p className="mb-4 text-xs text-muted-foreground">{t('settings.sync.statusHint')}</p>
        <div className="rounded-lg border border-border p-4">
          {statusLoading ? (
            <p className="text-xs text-muted-foreground">Loading…</p>
          ) : (
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <span className="text-xs text-muted-foreground">Last sync</span>
                <span className="font-mono text-sm text-foreground">
                  {status?.last_synced_at
                    ? formatDate(status.last_synced_at, true, timezone, timeFormat)
                    : 'Never'}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-xs text-muted-foreground">Entries updated</span>
                <span className="font-mono text-sm text-foreground">
                  {status?.entries_updated ?? 0}
                </span>
              </div>
              {status?.error && (
                <div className="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2">
                  <p className="text-xs text-destructive">{status.error}</p>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Trigger manual sync */}
      <div
        data-testid="manual-sync-row"
        className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between"
      >
        <div className="min-w-0">
          <h2 className="mb-1 text-sm font-medium text-foreground">Manual sync</h2>
          <p className="text-xs text-muted-foreground">
            Fetch the latest MCC codes and merchant category mappings immediately. The sync runs in
            the background; refresh the status above to see results.
          </p>
        </div>
        <div className="flex flex-shrink-0 items-center gap-4">
          <button
            onClick={handleTrigger}
            disabled={syncing}
            className={cn(
              'rounded-md px-4 py-2 text-sm transition-colors',
              syncing
                ? 'cursor-not-allowed bg-secondary text-muted-foreground opacity-50'
                : 'bg-primary text-primary-foreground hover:bg-primary/90',
            )}
          >
            {syncing ? 'Requesting…' : 'Sync now'}
          </button>
          {triggered && (
            <span className="text-xs text-muted-foreground">✓ Sync started in background</span>
          )}
        </div>
      </div>
    </div>
  )
}
