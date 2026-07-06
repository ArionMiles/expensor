import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { ArrowLeft, ChevronDown, Search, X } from 'lucide-react'
import { useActiveReader, useSearchReaderMessages } from '@/api/queries'
import type { ReaderMessageSample } from '@/api/types'
import { FloatingDropdown } from '@/components/Combobox'
import { useDisplay } from '@/contexts/DisplayContext'
import { useTooltip } from '@/hooks/useTooltip'
import { useI18n } from '@/i18n/I18nProvider'
import { cn, formatDate } from '@/lib/utils'
import { saveRuleEmailSearchDraft } from './emailSearchDraft'

const limitOptions = [10, 25, 50]
const relativeTimeThresholdMs = 2 * 24 * 60 * 60 * 1000

function normalizedLimit(value: string | null) {
  const parsed = Number(value)
  return limitOptions.includes(parsed) ? parsed : 10
}

function selectedMessages(
  results: ReaderMessageSample[] | undefined,
  selectedIDs: Set<string>,
): ReaderMessageSample[] {
  if (!results?.length || selectedIDs.size === 0) return []
  return results.filter((message) => selectedIDs.has(message.id))
}

function relativeReceivedLabel(value: Date, locale: string) {
  const diffMs = Date.now() - value.getTime()
  if (diffMs < 0 || diffMs >= relativeTimeThresholdMs) return null

  const formatter = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' })
  const diffSeconds = Math.floor(diffMs / 1000)
  if (diffSeconds < 60) return 'just now'

  const diffMinutes = Math.floor(diffSeconds / 60)
  if (diffMinutes < 60) return formatter.format(-diffMinutes, 'minute')

  const diffHours = Math.floor(diffMinutes / 60)
  if (diffHours < 24) return formatter.format(-diffHours, 'hour')

  const diffDays = Math.floor(diffHours / 24)
  return formatter.format(-diffDays, 'day')
}

function SkeletonRows() {
  return (
    <div className="divide-y divide-border rounded-lg border border-border bg-card">
      {Array.from({ length: 5 }).map((_, index) => (
        <div key={index} className="grid grid-cols-[1.5rem_1fr] gap-3 px-4 py-4">
          <div className="mt-1 h-4 w-4 rounded border border-border bg-secondary/60" />
          <div className="space-y-2">
            <div className="h-4 w-2/3 animate-pulse rounded bg-secondary" />
            <div className="h-3 w-48 animate-pulse rounded bg-secondary/80" />
            <div className="h-3 w-full animate-pulse rounded bg-secondary/60" />
          </div>
        </div>
      ))}
    </div>
  )
}

type ResultRowProps = {
  message: ReaderMessageSample
  checked: boolean
  expanded: boolean
  onToggleChecked: () => void
  onToggleExpanded: () => void
}

function ResultRow({
  message,
  checked,
  expanded,
  onToggleChecked,
  onToggleExpanded,
}: ResultRowProps) {
  const { locale } = useI18n()
  const { timezone, timeFormat } = useDisplay()
  const { handlers: tooltip, tip } = useTooltip()
  const receivedAt = message.received_at ? new Date(message.received_at) : null
  const receivedLabel =
    receivedAt && !Number.isNaN(receivedAt.getTime())
      ? (relativeReceivedLabel(receivedAt, locale) ??
        formatDate(message.received_at ?? '', true, timezone, timeFormat, locale))
      : ''
  const receivedFullLabel =
    receivedAt && !Number.isNaN(receivedAt.getTime())
      ? formatDate(message.received_at ?? '', true, timezone, timeFormat, locale)
      : ''

  return (
    <div className="grid grid-cols-[1.5rem_1fr] gap-3 px-4 py-4">
      <input
        type="checkbox"
        checked={checked}
        onChange={onToggleChecked}
        aria-label={`Select ${message.subject}`}
        className="mt-1 h-4 w-4 accent-primary"
      />
      <div className="min-w-0">
        <button
          type="button"
          onClick={onToggleExpanded}
          className="group flex w-full min-w-0 items-start justify-between gap-3 text-left"
          aria-expanded={expanded}
        >
          <span className="min-w-0">
            <span className="block truncate text-sm font-semibold text-foreground">
              {message.subject}
            </span>
            <span className="mt-1 block truncate font-mono text-xs text-muted-foreground">
              {message.sender_email}
            </span>
            {!expanded && (
              <span className="mt-2 block max-h-10 overflow-hidden text-xs leading-5 text-muted-foreground">
                {message.body}
              </span>
            )}
          </span>
          <span className="flex shrink-0 items-start gap-3">
            {receivedLabel && (
              <span
                className="mt-0.5 whitespace-nowrap text-xs font-medium text-muted-foreground"
                {...tooltip(receivedFullLabel)}
              >
                {receivedLabel}
              </span>
            )}
            <ChevronDown
              aria-hidden="true"
              className={`mt-0.5 h-4 w-4 shrink-0 text-muted-foreground transition-transform group-hover:text-foreground ${
                expanded ? 'rotate-180' : ''
              }`}
            />
          </span>
        </button>
        {tip}
        {expanded && (
          <pre className="mt-3 max-h-48 overflow-y-auto whitespace-pre-wrap rounded-lg border border-border bg-background px-3 py-3 font-mono text-xs leading-5 text-foreground">
            {message.body}
          </pre>
        )}
      </div>
    </div>
  )
}

type LimitControlProps = {
  limit: number
  menuLabel: string
  perPageLabel: string
  onLimit: (limit: number) => void
}

function LimitControl({ limit, menuLabel, perPageLabel, onLimit }: LimitControlProps) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)

  return (
    <div ref={containerRef} className="flex items-center gap-1.5 text-xs text-muted-foreground">
      <button
        ref={buttonRef}
        type="button"
        aria-label={`${menuLabel}: ${limit}`}
        aria-expanded={open}
        aria-haspopup="menu"
        onClick={() => setOpen((value) => !value)}
        className="rounded border border-primary/40 bg-primary/10 px-1.5 py-0.5 font-mono text-primary transition-colors hover:bg-primary/20"
      >
        {limit}
      </button>
      <span>{perPageLabel}</span>
      <FloatingDropdown
        open={open}
        anchorRef={buttonRef}
        containerRef={containerRef}
        onOpenChange={setOpen}
        minWidth={72}
        maxHeight={112}
      >
        {(style, setPortalNode) => (
          <div
            ref={setPortalNode}
            role="menu"
            aria-label={menuLabel}
            className="fixed z-50 rounded-md border border-border bg-card py-1 shadow-lg"
            style={style}
          >
            {limitOptions.map((option) => (
              <button
                key={option}
                type="button"
                role="menuitem"
                onMouseDown={() => {
                  onLimit(option)
                  setOpen(false)
                }}
                className={cn(
                  'block w-full px-3 py-1.5 text-left text-xs transition-colors hover:bg-accent hover:text-foreground',
                  option === limit ? 'text-primary' : 'text-muted-foreground',
                )}
              >
                {option}
              </button>
            ))}
          </div>
        )}
      </FloatingDropdown>
    </div>
  )
}

export function RuleEmailSearch() {
  const { t } = useI18n()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const { data: activeReader = '' } = useActiveReader()
  const { mutate: searchMessages, data, isPending, error } = useSearchReaderMessages()

  const subjectQuery = searchParams.get('q') ?? ''
  const limit = normalizedLimit(searchParams.get('limit'))
  const [inputValue, setInputValue] = useState(subjectQuery)
  const [selectedIDs, setSelectedIDs] = useState<Set<string>>(new Set())
  const [expandedIDs, setExpandedIDs] = useState<Set<string>>(new Set())

  useEffect(() => {
    setInputValue(subjectQuery)
  }, [subjectQuery])

  useEffect(() => {
    if (!activeReader || !subjectQuery.trim()) return
    setSelectedIDs(new Set())
    setExpandedIDs(new Set())
    searchMessages({ reader: activeReader, subject: subjectQuery.trim(), limit })
  }, [activeReader, limit, searchMessages, subjectQuery])

  const results = data?.results ?? []
  const selected = useMemo(() => selectedMessages(results, selectedIDs), [results, selectedIDs])

  const submitSearch = () => {
    const nextQuery = inputValue.trim()
    if (!nextQuery) return
    setSearchParams({ q: nextQuery, limit: String(limit) })
  }

  const setLimit = (nextLimit: number) => {
    const params: Record<string, string> = { limit: String(nextLimit) }
    if (inputValue.trim()) params.q = inputValue.trim()
    setSearchParams(params)
  }

  const toggleSelected = (id: string) => {
    setSelectedIDs((current) => {
      const next = new Set(current)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleExpanded = (id: string) => {
    setExpandedIDs((current) => {
      const next = new Set(current)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const startRule = () => {
    if (selected.length === 0) return
    const draftID = saveRuleEmailSearchDraft({
      subjectQuery: subjectQuery.trim(),
      messages: selected,
    })
    navigate(`/rules/new?draft=${encodeURIComponent(draftID)}`)
  }

  return (
    <div className="mx-auto flex min-h-full w-full max-w-6xl flex-col px-6 py-6">
      <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
        <div>
          <Link
            to="/rules"
            className="inline-flex items-center gap-2 text-xs font-medium text-muted-foreground hover:text-foreground"
          >
            <ArrowLeft aria-hidden="true" className="h-3.5 w-3.5" />
            {t('rules.backToRules')}
          </Link>
          <h1 className="mt-3 text-2xl font-semibold tracking-tight text-foreground">
            {t('rules.emailSearch.title')}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">{t('rules.emailSearch.summary')}</p>
        </div>
        {selected.length > 0 && (
          <div className="flex items-center gap-3 pt-8">
            <span className="text-sm font-medium text-muted-foreground">
              {t('rules.emailSearch.selected', { count: selected.length })}
            </span>
            <button
              type="button"
              onClick={startRule}
              className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground hover:bg-primary/90"
            >
              {t('rules.emailSearch.startRule')}
            </button>
          </div>
        )}
      </div>

      <form
        className="flex flex-col gap-3 border-y border-border bg-card/60 px-4 py-4 sm:flex-row sm:items-end"
        onSubmit={(event) => {
          event.preventDefault()
          submitSearch()
        }}
      >
        <label className="min-w-0 flex-1 text-xs font-medium text-muted-foreground">
          {t('rules.emailSearch.subjectLabel')}
          <div className="mt-1 flex items-center gap-2 rounded-lg border border-border bg-input px-3 py-2">
            <Search aria-hidden="true" className="h-4 w-4 shrink-0 text-muted-foreground" />
            <input
              type="search"
              value={inputValue}
              onChange={(event) => setInputValue(event.target.value)}
              placeholder={t('rules.emailSearch.subjectPlaceholder')}
              className="no-native-search-clear min-w-0 flex-1 bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground"
            />
            {inputValue && (
              <button
                type="button"
                onClick={() => setInputValue('')}
                aria-label={t('rules.emailSearch.clearSubject')}
                className="shrink-0 text-muted-foreground transition-colors hover:text-foreground"
              >
                <X aria-hidden="true" size={14} />
              </button>
            )}
          </div>
        </label>

        <div className="pb-2 sm:pb-2.5">
          <LimitControl
            limit={limit}
            menuLabel={t('rules.emailSearch.rowsPerPage')}
            perPageLabel={t('rules.emailSearch.perPage')}
            onLimit={setLimit}
          />
        </div>
      </form>

      {!activeReader && (
        <p className="mt-4 rounded-lg border border-border bg-card px-4 py-3 text-sm text-muted-foreground">
          {t('rules.emailSearch.noReader')}
        </p>
      )}

      <div className="mt-4 flex-1">
        {isPending ? (
          <SkeletonRows />
        ) : error ? (
          <p className="rounded-lg border border-destructive/40 bg-card px-4 py-3 text-sm text-destructive">
            {t('rules.emailSearch.error')}
          </p>
        ) : subjectQuery.trim() && data ? (
          results.length === 0 ? (
            <p className="rounded-lg border border-border bg-card px-4 py-8 text-center text-sm text-muted-foreground">
              {t('rules.emailSearch.empty')}
            </p>
          ) : (
            <div
              className="divide-y divide-border rounded-lg border border-border bg-card"
              aria-label={t('rules.emailSearch.results')}
            >
              {results.map((message) => (
                <ResultRow
                  key={message.id}
                  message={message}
                  checked={selectedIDs.has(message.id)}
                  expanded={expandedIDs.has(message.id)}
                  onToggleChecked={() => toggleSelected(message.id)}
                  onToggleExpanded={() => toggleExpanded(message.id)}
                />
              ))}
            </div>
          )
        ) : (
          <p className="rounded-lg border border-dashed border-border px-4 py-8 text-center text-sm text-muted-foreground">
            {t('rules.emailSearch.initial')}
          </p>
        )}
      </div>
    </div>
  )
}
