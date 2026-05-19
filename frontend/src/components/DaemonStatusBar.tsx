import { useStatus } from '@/api/queries'
import { formatDuration } from '@/lib/utils'
import { cn } from '@/lib/utils'
import { useI18n } from '@/i18n/I18nProvider'
import { shortcutLabel } from '@/lib/shortcuts'

function CommandPaletteHint() {
  const { t } = useI18n()

  return (
    <div className="ml-auto flex flex-shrink-0 items-center gap-1.5 text-[11px] text-muted-foreground">
      <span>{t('command.palette')}</span>
      <kbd className="rounded border border-border bg-secondary px-1.5 py-0.5 font-mono text-[10px] leading-none text-foreground">
        {shortcutLabel('K')}
      </kbd>
    </div>
  )
}

export function DaemonStatusBar() {
  const { data, isLoading, error } = useStatus()

  if (isLoading) {
    return (
      <div className="flex w-full items-center gap-3 border-b border-border bg-card px-4 py-2">
        <span className="text-xs text-muted-foreground">checking daemon status...</span>
        <CommandPaletteHint />
      </div>
    )
  }

  if (error || !data) {
    return (
      <div className="flex w-full items-center gap-2 border-b border-border bg-card px-4 py-2">
        <div className="flex min-w-0 items-center gap-2">
          <span
            className="h-1.5 w-1.5 flex-shrink-0 rounded-full bg-destructive"
            aria-hidden="true"
          />
          <span className="text-xs text-destructive">backend unreachable</span>
        </div>
        <CommandPaletteHint />
      </div>
    )
  }

  const { daemon } = data

  return (
    <div className="flex w-full items-center gap-3 border-b border-border bg-card px-4 py-2">
      <div className="flex min-w-0 items-center gap-2">
        <span
          className={cn(
            'h-1.5 w-1.5 flex-shrink-0 rounded-full',
            daemon.running ? 'bg-success' : 'bg-destructive',
          )}
          aria-hidden="true"
        />
        {daemon.running ? (
          <span className="text-xs text-success">
            daemon running
            {daemon.started_at && (
              <span className="ml-2 text-muted-foreground">
                · uptime {formatDuration(daemon.started_at)}
              </span>
            )}
          </span>
        ) : (
          <span className="text-xs text-destructive">
            daemon stopped
            {daemon.last_error && (
              <span className="ml-2 text-muted-foreground">· {daemon.last_error}</span>
            )}
          </span>
        )}
      </div>
      <CommandPaletteHint />
    </div>
  )
}
