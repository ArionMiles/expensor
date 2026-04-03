import { cn } from '@/lib/utils'

interface StatusBadgeProps {
  status: 'running' | 'stopped' | 'error' | 'pending'
  label?: string
  className?: string
}

export function StatusBadge({ status, label, className }: StatusBadgeProps) {
  const dotColor = {
    running: 'bg-success',
    stopped: 'bg-muted-foreground',
    error: 'bg-destructive',
    pending: 'bg-warning',
  }[status]

  const textColor = {
    running: 'text-success',
    stopped: 'text-muted-foreground',
    error: 'text-destructive',
    pending: 'text-warning',
  }[status]

  const displayLabel =
    label ??
    {
      running: 'Running',
      stopped: 'Stopped',
      error: 'Error',
      pending: 'Pending',
    }[status]

  return (
    <span className={cn('inline-flex items-center gap-1.5 text-xs', className)}>
      <span className={cn('w-1.5 h-1.5 rounded-full flex-shrink-0', dotColor)} aria-hidden="true" />
      <span className={textColor}>{displayLabel}</span>
    </span>
  )
}
