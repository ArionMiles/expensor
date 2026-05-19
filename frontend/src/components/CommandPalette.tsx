import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import type { NavigationTarget } from '@/lib/navigation'
import { cn } from '@/lib/utils'
import { useI18n } from '@/i18n/I18nProvider'
import type { MessageKey } from '@/i18n/messages'

function matchesTarget(
  target: NavigationTarget,
  query: string,
  t: (key: MessageKey) => string,
): boolean {
  const normalized = query.trim().toLowerCase()
  if (normalized === '') return true

  const haystack = [
    t(target.titleKey),
    target.subtitleKey ? t(target.subtitleKey) : '',
    t(target.descriptionKey),
    ...(target.keywords ?? []),
  ]
    .join(' ')
    .toLowerCase()

  return haystack.includes(normalized)
}

export function CommandPalette({
  open,
  targets,
  onClose,
  onNavigate,
}: {
  open: boolean
  targets: NavigationTarget[]
  onClose: () => void
  onNavigate: (path: string) => void
}) {
  const { t } = useI18n()
  const [query, setQuery] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const inputRef = useRef<HTMLInputElement | null>(null)
  const escapeClosedRef = useRef(false)

  const filteredTargets = useMemo(
    () => targets.filter((target) => matchesTarget(target, query, t)),
    [targets, query, t],
  )

  useEffect(() => {
    if (!open) {
      setQuery('')
      setSelectedIndex(0)
      escapeClosedRef.current = false
      return
    }
    escapeClosedRef.current = false
    inputRef.current?.focus()
  }, [open])

  useEffect(() => {
    setSelectedIndex(0)
  }, [query])

  useEffect(() => {
    if (!open) return

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      event.preventDefault()
      event.stopPropagation()
      if (escapeClosedRef.current) return
      escapeClosedRef.current = true
      onClose()
    }

    window.addEventListener('keydown', handleEscape, true)
    window.addEventListener('keyup', handleEscape, true)
    return () => {
      window.removeEventListener('keydown', handleEscape, true)
      window.removeEventListener('keyup', handleEscape, true)
    }
  }, [onClose, open])

  if (!open) return null

  const activeTarget = filteredTargets[selectedIndex]

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-start justify-center bg-background/50 px-4 pt-[12vh] backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Command palette"
        className="w-full max-w-2xl rounded-2xl border border-border bg-card shadow-2xl"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="border-b border-border px-4 py-3">
          <input
            ref={inputRef}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Escape') {
                event.preventDefault()
                event.stopPropagation()
                onClose()
                return
              }
              if (event.key === 'ArrowDown') {
                event.preventDefault()
                setSelectedIndex((prev) => Math.min(prev + 1, filteredTargets.length - 1))
                return
              }
              if (event.key === 'ArrowUp') {
                event.preventDefault()
                setSelectedIndex((prev) => Math.max(prev - 1, 0))
                return
              }
              if (event.key === 'Enter' && activeTarget) {
                event.preventDefault()
                onNavigate(activeTarget.path)
              }
            }}
            placeholder={t('command.navigatePlaceholder')}
            aria-label={t('command.search')}
            autoComplete="off"
            spellCheck={false}
            className="w-full border-0 bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground"
          />
        </div>
        <div className="max-h-[50vh] overflow-y-auto p-2">
          {filteredTargets.length === 0 ? (
            <p className="px-3 py-6 text-center text-sm text-muted-foreground">
              {t('command.noMatches')}
            </p>
          ) : (
            filteredTargets.map((target, index) => (
              <CommandPaletteOption
                key={target.id}
                target={target}
                selected={index === selectedIndex}
                onMouseEnter={() => setSelectedIndex(index)}
                onNavigate={onNavigate}
              />
            ))
          )}
        </div>
      </div>
    </div>,
    document.body,
  )
}

function CommandPaletteOption({
  target,
  selected,
  onMouseEnter,
  onNavigate,
}: {
  target: NavigationTarget
  selected: boolean
  onMouseEnter: () => void
  onNavigate: (path: string) => void
}) {
  const { t } = useI18n()
  const title = t(target.titleKey)
  const subtitle = target.subtitleKey ? t(target.subtitleKey) : undefined
  const description = t(target.descriptionKey)

  return (
    <button
      type="button"
      onMouseEnter={onMouseEnter}
      onClick={() => onNavigate(target.path)}
      className={cn(
        'flex w-full items-center justify-between rounded-xl px-3 py-2.5 text-left transition-colors',
        selected ? 'bg-accent text-accent-foreground' : 'text-foreground hover:bg-accent/70',
      )}
    >
      <div className="min-w-0">
        <div className="truncate text-sm">{subtitle ? `${title} / ${subtitle}` : title}</div>
        <div className="truncate text-xs text-muted-foreground">{description}</div>
      </div>
    </button>
  )
}
