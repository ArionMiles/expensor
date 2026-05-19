import { useSyncStatus, useTriggerSync } from '@/api/queries'
import { useDisplay } from '@/contexts/DisplayContext'
import { cn, formatDate } from '@/lib/utils'
import { useState } from 'react'

export function SyncSettings() {
  const { data: status, isLoading: statusLoading } = useSyncStatus()
  const { mutate: triggerSync, isPending: syncing } = useTriggerSync()
  const [triggered, setTriggered] = useState(false)
  const { timezone, timeFormat } = useDisplay()

  const handleTrigger = () => {
    setTriggered(false)
    triggerSync(undefined, {
      onSuccess: () => setTriggered(true),
    })
  }

  return (
    <div className="space-y-8">
      {/* Last sync status */}
      <div>
        <h2 className="mb-1 text-sm font-medium text-foreground">Sync status</h2>
        <p className="mb-4 text-xs text-muted-foreground">
          Expensor syncs community MCC codes and merchant category mappings at startup and every 24
          hours.
        </p>
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
      <div>
        <h2 className="mb-1 text-sm font-medium text-foreground">Manual sync</h2>
        <p className="mb-4 text-xs text-muted-foreground">
          Fetch the latest MCC codes and merchant category mappings immediately. The sync runs in
          the background; refresh the status above to see results.
        </p>
        <div className="flex items-center gap-4">
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
