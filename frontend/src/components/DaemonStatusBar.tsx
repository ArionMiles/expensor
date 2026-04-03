import { useStatus } from '@/api/queries'
import { formatDuration } from '@/lib/utils'
import { cn } from '@/lib/utils'

export function DaemonStatusBar() {
  const { data, isLoading, error } = useStatus()

  if (isLoading) {
    return (
      <div className="w-full border-b border-border bg-card px-4 py-2">
        <span className="text-xs text-muted-foreground">checking daemon status...</span>
      </div>
    )
  }

  if (error || !data) {
    return (
      <div className="flex w-full items-center gap-2 border-b border-border bg-card px-4 py-2">
        <span
          className="h-1.5 w-1.5 flex-shrink-0 rounded-full bg-destructive"
          aria-hidden="true"
        />
        <span className="text-xs text-destructive">backend unreachable</span>
      </div>
    )
  }

  const { daemon } = data

  return (
    <div className="flex w-full items-center gap-3 border-b border-border bg-card px-4 py-2">
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
  )
}
