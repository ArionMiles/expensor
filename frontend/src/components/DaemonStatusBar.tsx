import { useStatus } from '@/api/queries'
import { formatDuration } from '@/lib/utils'
import { cn } from '@/lib/utils'

export function DaemonStatusBar() {
  const { data, isLoading, error } = useStatus()

  if (isLoading) {
    return (
      <div className="w-full px-4 py-2 bg-card border-b border-border">
        <span className="text-xs text-muted-foreground">checking daemon status...</span>
      </div>
    )
  }

  if (error || !data) {
    return (
      <div className="w-full px-4 py-2 bg-card border-b border-border flex items-center gap-2">
        <span className="w-1.5 h-1.5 rounded-full bg-destructive flex-shrink-0" aria-hidden="true" />
        <span className="text-xs text-destructive">backend unreachable</span>
      </div>
    )
  }

  const { daemon } = data

  return (
    <div className="w-full px-4 py-2 bg-card border-b border-border flex items-center gap-3">
      <span
        className={cn(
          'w-1.5 h-1.5 rounded-full flex-shrink-0',
          daemon.running ? 'bg-success' : 'bg-destructive',
        )}
        aria-hidden="true"
      />
      {daemon.running ? (
        <span className="text-xs text-success">
          daemon running
          {daemon.started_at && (
            <span className="text-muted-foreground ml-2">
              · uptime {formatDuration(daemon.started_at)}
            </span>
          )}
        </span>
      ) : (
        <span className="text-xs text-destructive">
          daemon stopped
          {daemon.last_error && (
            <span className="text-muted-foreground ml-2">· {daemon.last_error}</span>
          )}
        </span>
      )}
    </div>
  )
}
