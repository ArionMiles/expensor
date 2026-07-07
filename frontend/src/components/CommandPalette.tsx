import { LogOut, type LucideIcon } from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import type { NavigationTarget } from '@/lib/navigation'
import { cn } from '@/lib/utils'
import { useI18n } from '@/i18n/I18nProvider'
import type { MessageKey } from '@/i18n/messages'

export type CommandPaletteAction = {
  id: string
  titleKey: MessageKey
  descriptionKey: MessageKey
  icon?: LucideIcon
  keywords?: string[]
  variant?: 'default' | 'destructive'
  disabled?: boolean
}

type CommandPaletteItem =
  | {
      type: 'navigation'
      target: NavigationTarget
      score: number
      order: number
    }
  | {
      type: 'action'
      action: CommandPaletteAction
      score: number
      order: number
    }

function scoreCandidate({
  query,
  primary,
  secondary = [],
  tertiary = [],
}: {
  query: string
  primary: string[]
  secondary?: string[]
  tertiary?: string[]
}) {
  const normalized = query.trim().toLowerCase()
  if (normalized === '') return 0

  const cleanPrimary = primary.map((value) => value.toLowerCase()).filter(Boolean)
  const cleanSecondary = secondary.map((value) => value.toLowerCase()).filter(Boolean)
  const cleanTertiary = tertiary.map((value) => value.toLowerCase()).filter(Boolean)
  if (cleanPrimary.some((value) => value === normalized)) return 0
  if (cleanPrimary.some((value) => value.startsWith(normalized))) return 1
  if (cleanSecondary.some((value) => value === normalized)) return 2
  if (cleanSecondary.some((value) => value.startsWith(normalized))) return 3
  if (cleanPrimary.some((value) => value.includes(normalized))) return 4
  if (cleanSecondary.some((value) => value.includes(normalized))) return 5
  if (cleanTertiary.some((value) => value.includes(normalized))) return 6
  return Number.POSITIVE_INFINITY
}

function targetScore(
  target: NavigationTarget,
  query: string,
  t: (key: MessageKey) => string,
): number {
  return scoreCandidate({
    query,
    primary: [t(target.titleKey), target.subtitleKey ? t(target.subtitleKey) : ''],
    secondary: target.keywords ?? [],
    tertiary: [target.id, t(target.descriptionKey)],
  })
}

function actionScore(
  action: CommandPaletteAction,
  query: string,
  t: (key: MessageKey) => string,
): number {
  return scoreCandidate({
    query,
    primary: [t(action.titleKey), ...(action.keywords ?? [])],
    secondary: [action.id],
    tertiary: [t(action.descriptionKey)],
  })
}

export function CommandPalette({
  open,
  targets,
  actions = [],
  onClose,
  onNavigate,
  onAction,
}: {
  open: boolean
  targets: NavigationTarget[]
  actions?: CommandPaletteAction[]
  onClose: () => void
  onNavigate: (path: string) => void
  onAction?: (id: string) => void
}) {
  const { t } = useI18n()
  const [query, setQuery] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const inputRef = useRef<HTMLInputElement | null>(null)
  const escapeClosedRef = useRef(false)

  const rankedTargets = useMemo(
    () =>
      targets
        .map((target, index) => ({
          type: 'navigation' as const,
          target,
          score: targetScore(target, query, t),
          order: index + actions.length,
        }))
        .filter((item) => Number.isFinite(item.score)),
    [actions.length, targets, query, t],
  )
  const rankedActions = useMemo(
    () =>
      actions
        .map((action, index) => ({
          type: 'action' as const,
          action,
          score: actionScore(action, query, t),
          order: index,
        }))
        .filter((item) => Number.isFinite(item.score)),
    [actions, query, t],
  )
  const items = useMemo<CommandPaletteItem[]>(
    () =>
      [...rankedActions, ...rankedTargets].sort(
        (left, right) => left.score - right.score || left.order - right.order,
      ),
    [rankedActions, rankedTargets],
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

  const activeItem = items[selectedIndex]
  const hasMatches = items.length > 0

  const runItem = (item: CommandPaletteItem) => {
    if (item.type === 'navigation') {
      onNavigate(item.target.path)
      return
    }
    if (!item.action.disabled) onAction?.(item.action.id)
  }

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
                setSelectedIndex((prev) => Math.min(prev + 1, items.length - 1))
                return
              }
              if (event.key === 'ArrowUp') {
                event.preventDefault()
                setSelectedIndex((prev) => Math.max(prev - 1, 0))
                return
              }
              if (event.key === 'Enter' && activeItem) {
                event.preventDefault()
                runItem(activeItem)
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
          {!hasMatches ? (
            <p className="px-3 py-6 text-center text-sm text-muted-foreground">
              {t('command.noMatches')}
            </p>
          ) : (
            <>
              {items.map((item, index) => {
                const previous = items[index - 1]
                const showSection = !previous || previous.type !== item.type
                const option =
                  item.type === 'action' ? (
                    <CommandPaletteActionOption
                      key={item.action.id}
                      action={item.action}
                      selected={index === selectedIndex}
                      onMouseEnter={() => setSelectedIndex(index)}
                      onAction={onAction}
                    />
                  ) : (
                    <CommandPaletteOption
                      key={item.target.id}
                      target={item.target}
                      selected={index === selectedIndex}
                      onMouseEnter={() => setSelectedIndex(index)}
                      onNavigate={onNavigate}
                    />
                  )
                return (
                  <CommandPaletteSection
                    key={item.type === 'action' ? item.action.id : item.target.id}
                    title={
                      item.type === 'action'
                        ? t('command.sections.actions')
                        : t('command.sections.navigation')
                    }
                    compact={!showSection}
                  >
                    {option}
                  </CommandPaletteSection>
                )
              })}
            </>
          )}
        </div>
      </div>
    </div>,
    document.body,
  )
}

function CommandPaletteSection({
  title,
  compact = false,
  children,
}: {
  title: string
  compact?: boolean
  children: ReactNode
}) {
  return (
    <div className={compact ? 'py-0' : 'py-1 first:pt-0'}>
      {!compact && (
        <div className="px-3 pb-1 pt-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
          {title}
        </div>
      )}
      <div className="space-y-1">{children}</div>
    </div>
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

function CommandPaletteActionOption({
  action,
  selected,
  onMouseEnter,
  onAction,
}: {
  action: CommandPaletteAction
  selected: boolean
  onMouseEnter: () => void
  onAction?: (id: string) => void
}) {
  const { t } = useI18n()
  const title = t(action.titleKey)
  const description = t(action.descriptionKey)
  const destructive = action.variant === 'destructive'
  const Icon = action.icon ?? LogOut

  return (
    <button
      type="button"
      disabled={action.disabled}
      onMouseEnter={onMouseEnter}
      onClick={() => onAction?.(action.id)}
      className={cn(
        'flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-50',
        destructive
          ? selected
            ? 'bg-destructive/10 text-destructive'
            : 'text-destructive hover:bg-destructive/10'
          : selected
            ? 'bg-accent text-accent-foreground'
            : 'text-foreground hover:bg-accent/70',
      )}
    >
      <span
        className={cn(
          'flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md border',
          destructive
            ? 'border-destructive/20 bg-destructive/10 text-destructive'
            : 'border-border bg-secondary text-muted-foreground',
        )}
      >
        <Icon size={15} />
      </span>
      <span className="min-w-0">
        <span className="block truncate text-sm font-medium">{title}</span>
        <span className="block truncate text-xs text-muted-foreground">{description}</span>
      </span>
    </button>
  )
}
