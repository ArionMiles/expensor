import { useScanningStatus } from '@/api/queries'
import {
  isScanningStatusBreathingEnabled,
  subscribeScanningStatusBreathing,
} from '@/lib/scanningStatusIndicator'
import { cn } from '@/lib/utils'
import { useI18n } from '@/i18n/I18nProvider'
import { shortcutLabel } from '@/lib/shortcuts'
import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'

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
  const { t } = useI18n()
  const { data, isLoading, error } = useScanningStatus()
  const navigate = useNavigate()
  const [breathingEnabled, setBreathingEnabled] = useState(isScanningStatusBreathingEnabled)
  const indicatorRef = useRef<HTMLSpanElement>(null)
  const [tooltipPosition, setTooltipPosition] = useState<{ left: number; top: number } | null>(null)

  useEffect(() => subscribeScanningStatusBreathing(setBreathingEnabled), [])

  const showTooltip = () => {
    const rect = indicatorRef.current?.getBoundingClientRect()
    if (!rect) return
    setTooltipPosition({ left: rect.left, top: rect.bottom + 8 })
  }
  const hideTooltip = () => setTooltipPosition(null)

  if (isLoading) {
    return (
      <div className="flex w-full items-center gap-3 border-b border-border bg-card px-4 py-2">
        <span className="text-xs text-muted-foreground">{t('scanStatus.checking')}</span>
        <CommandPaletteHint />
      </div>
    )
  }

  if (error || !data) {
    return (
      <div className="flex w-full items-center gap-2 border-b border-border bg-card px-4 py-2">
        <div
          className="relative flex min-w-0 items-center gap-2"
          onMouseEnter={showTooltip}
          onMouseLeave={hideTooltip}
        >
          <span
            ref={indicatorRef}
            className="h-1.5 w-1.5 flex-shrink-0 rounded-full bg-destructive"
            aria-hidden="true"
          />
          <span className="text-xs text-destructive">{t('scanStatus.backendUnreachable')}</span>
          {tooltipPosition && (
            <span
              className="pointer-events-none fixed z-50 rounded-md border border-border bg-card px-2 py-1 text-xs text-card-foreground shadow-lg"
              style={{ left: tooltipPosition.left, top: tooltipPosition.top }}
            >
              {t('scanStatus.backendUnreachableTooltip')}
            </span>
          )}
        </div>
        <CommandPaletteHint />
      </div>
    )
  }

  const noReader = !data.active_reader
  const needsAction = data.state === 'needs_auth' || data.state === 'reader_not_configured'
  const transientError = data.state === 'backing_off'
  const healthy = !noReader && !needsAction && !transientError && data.enabled
  const canOpenSetup = needsAction
  const label = noReader
    ? t('scanStatus.noReader')
    : needsAction
      ? data.state === 'needs_auth'
        ? t('scanStatus.needsAuth')
        : t('scanStatus.setupNeeded')
      : transientError
        ? t('scanStatus.retrying')
        : data.enabled
          ? t('scanStatus.active')
          : t('scanStatus.paused')
  const tooltip = noReader
    ? t('scanStatus.noReaderTooltip')
    : data.public_message ||
      (healthy ? t('scanStatus.activeTooltip') : t('scanStatus.pausedTooltip'))

  return (
    <div className="flex w-full items-center gap-3 border-b border-border bg-card px-4 py-2">
      <button
        type="button"
        onClick={() => {
          if (canOpenSetup) navigate('/setup')
        }}
        onMouseEnter={showTooltip}
        onMouseLeave={hideTooltip}
        onFocus={showTooltip}
        onBlur={hideTooltip}
        className={cn(
          'relative flex min-w-0 items-center gap-2 text-left',
          canOpenSetup ? 'cursor-pointer' : 'cursor-default',
        )}
      >
        <span
          ref={indicatorRef}
          className="relative flex h-2 w-2 flex-shrink-0 items-center justify-center"
        >
          {healthy && breathingEnabled && (
            <span className="absolute h-3 w-3 rounded-full bg-success/35 motion-safe:animate-ping" />
          )}
          <span
            className={cn(
              'relative h-1.5 w-1.5 rounded-full',
              healthy && 'bg-success',
              (needsAction || transientError) && 'bg-destructive',
              (noReader || !data.enabled) && 'bg-muted-foreground',
            )}
            aria-hidden="true"
          />
        </span>
        <span
          className={cn(
            'truncate text-xs',
            healthy && 'text-success',
            (needsAction || transientError) && 'text-destructive',
            (noReader || !data.enabled) && 'text-muted-foreground',
          )}
        >
          {label}
        </span>
        {tooltipPosition && (
          <span
            className="pointer-events-none fixed z-50 max-w-xs rounded-md border border-border bg-card px-2 py-1 text-xs text-card-foreground shadow-lg"
            style={{ left: tooltipPosition.left, top: tooltipPosition.top }}
          >
            {tooltip}
          </span>
        )}
      </button>
      <CommandPaletteHint />
    </div>
  )
}
