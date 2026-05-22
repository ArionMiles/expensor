import { cn } from '@/lib/utils'
import { useEffect, useId } from 'react'

interface ConfirmModalProps {
  title: string
  message: string
  confirmLabel?: string
  secondaryLabel?: string
  variant?: 'destructive' | 'default'
  onConfirm: () => void
  onSecondary?: () => void
  onCancel: () => void
  confirmDisabled?: boolean
  secondaryDisabled?: boolean
}

export function ConfirmModal({
  title,
  message,
  confirmLabel = 'Confirm',
  secondaryLabel,
  variant = 'default',
  onConfirm,
  onSecondary,
  onCancel,
  confirmDisabled = false,
  secondaryDisabled = false,
}: ConfirmModalProps) {
  const titleId = useId()

  // Close on Escape
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onCancel()
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onCancel])

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onCancel()
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        className="w-full max-w-sm space-y-4 rounded-lg border border-border bg-card p-6 shadow-xl"
      >
        <h2 id={titleId} className="text-sm font-semibold text-foreground">
          {title}
        </h2>
        <p className="text-xs leading-relaxed text-muted-foreground">{message}</p>
        <div className="flex flex-wrap justify-end gap-2 pt-1">
          <button
            onClick={onCancel}
            className="rounded-md px-4 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
          >
            Cancel
          </button>
          {secondaryLabel && onSecondary && (
            <button
              onClick={onSecondary}
              disabled={secondaryDisabled}
              className="rounded-md border border-border px-4 py-2 text-sm text-foreground transition-colors hover:bg-secondary disabled:cursor-not-allowed disabled:opacity-50"
            >
              {secondaryLabel}
            </button>
          )}
          <button
            onClick={onConfirm}
            disabled={confirmDisabled}
            className={cn(
              'rounded-md px-4 py-2 text-sm transition-colors disabled:cursor-not-allowed disabled:opacity-50',
              variant === 'destructive'
                ? 'bg-destructive text-destructive-foreground hover:bg-destructive/90'
                : 'bg-primary text-primary-foreground hover:bg-primary/90',
            )}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}
