import { cn } from '@/lib/utils'
import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

export interface SlideNotificationAction {
  label: string
  primary?: boolean
  /** Value passed to onAction when this button is clicked. */
  value: boolean
}

interface SlideNotificationProps {
  /** Duration in ms before auto-dismissing via the default action. */
  duration?: number
  /** Index of the action to fire when the timer expires (defaults to the last action). */
  defaultActionIndex?: number
  actions: SlideNotificationAction[]
  onAction: (value: boolean) => void
  children: React.ReactNode
}

export function SlideNotification({
  duration = 5000,
  defaultActionIndex,
  actions,
  onAction,
  children,
}: SlideNotificationProps) {
  const [visible, setVisible] = useState(false)
  const timerRef = useRef<ReturnType<typeof setTimeout>>()
  const defaultIdx = defaultActionIndex ?? actions.length - 1

  useEffect(() => {
    const raf = requestAnimationFrame(() => setVisible(true))
    timerRef.current = setTimeout(() => dismiss(actions[defaultIdx].value), duration)
    return () => {
      cancelAnimationFrame(raf)
      clearTimeout(timerRef.current)
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const dismiss = (value: boolean) => {
    clearTimeout(timerRef.current)
    setVisible(false)
    setTimeout(() => onAction(value), 300)
  }

  return createPortal(
    <div
      className={cn(
        'fixed right-6 top-6 z-50 w-72 overflow-hidden rounded-lg border border-border bg-card shadow-xl',
        'transition-all duration-300 ease-out',
        visible ? 'translate-x-0 opacity-100' : 'translate-x-full opacity-0',
      )}
    >
      <div className="space-y-3 px-4 pb-3 pt-4">
        <div className="text-sm text-muted-foreground">{children}</div>
        <div className="flex gap-2">
          {actions.map((a) => (
            <button
              key={a.label}
              onClick={() => dismiss(a.value)}
              className={cn(
                'flex-1 rounded-md px-3 py-1.5 text-xs transition-colors',
                a.primary
                  ? 'bg-primary text-primary-foreground hover:bg-primary/90'
                  : 'border border-border text-muted-foreground hover:text-foreground',
              )}
            >
              {a.label}
            </button>
          ))}
        </div>
      </div>
      {/* Countdown bar */}
      <div className="h-0.5 w-full bg-border">
        <div
          className="h-full origin-left bg-primary"
          style={{ animation: `shrink-width ${duration}ms linear forwards` }}
        />
      </div>
    </div>,
    document.body,
  )
}
