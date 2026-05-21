import {
  useBanks,
  useBulkIgnoreMerchants,
  useBulkIgnoreTransactions,
  useBuckets,
  useCategorizeMerchant,
  useCategories,
  useFacets,
  useIgnoreByMerchant,
  useIgnoreTransaction,
  useTransactions,
  useUpdateTransactionDescription,
  useUpdateTransactionFields,
} from '@/api/queries'
import type { BankColor, Transaction, TransactionFilters } from '@/api/types'
import { DateRangePicker } from '@/components/DateRangePicker'
import { FilterCombobox } from '@/components/FilterCombobox'
import { InlineSelect } from '@/components/InlineSelect'
import { LabelCombobox } from '@/components/LabelCombobox'
import { LabelSearch } from '@/components/LabelSearch'
import { Pagination } from '@/components/Pagination'
import { SlideNotification } from '@/components/SlideNotification'
import { useDisplay } from '@/contexts/DisplayContext'
import { cn, formatCurrency, formatDate, getSourceColor } from '@/lib/utils'
import { formatNumberForLocale } from '@/i18n/format'
import { useI18n } from '@/i18n/I18nProvider'
import { ChevronDown, ChevronUp, EyeOff, Eye, Funnel, Plus, X } from 'lucide-react'
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useSearchParams } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { useTooltip } from '@/hooks/useTooltip'

function sourceDisplay(source: Transaction['source']) {
  return source.label || [source.bank, source.type].filter(Boolean).join(' ')
}

function useDebounce<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(timer)
  }, [value, delay])
  return debounced
}

function AmountCell({ tx }: { tx: Transaction }) {
  const { locale } = useI18n()
  const hasOriginal =
    tx.original_amount !== undefined &&
    tx.original_currency !== undefined &&
    tx.original_currency !== tx.currency

  return (
    <div className="text-right">
      <div className="font-mono text-sm tabular-nums text-primary">
        {formatCurrency(tx.amount, tx.currency, locale)}
      </div>
      {hasOriginal && (
        <div className="font-mono text-[10px] tabular-nums text-muted-foreground">
          {formatCurrency(tx.original_amount!, tx.original_currency!, locale)}
          {tx.exchange_rate !== undefined && ` @ ${tx.exchange_rate.toFixed(2)}`}
        </div>
      )}
    </div>
  )
}

function HighlightedText({ text, query }: { text: string; query: string }) {
  const normalizedQuery = query.trim()
  if (!normalizedQuery) return <>{text}</>

  const index = text.toLowerCase().indexOf(normalizedQuery.toLowerCase())
  if (index === -1) return <>{text}</>

  return (
    <>
      {text.slice(0, index)}
      <mark className="rounded bg-primary/20 px-0.5 text-primary">
        {text.slice(index, index + normalizedQuery.length)}
      </mark>
      {text.slice(index + normalizedQuery.length)}
    </>
  )
}

function DescriptionCell({ tx, searchQuery }: { tx: Transaction; searchQuery: string }) {
  const [editing, setEditing] = useState(false)
  const [value, setValue] = useState(tx.description)
  const inputRef = useRef<HTMLInputElement>(null)
  const { mutate: updateDesc } = useUpdateTransactionDescription()
  const { handlers: descTip, tip: descTipEl } = useTooltip()

  useEffect(() => {
    setValue(tx.description)
  }, [tx.description])

  useEffect(() => {
    if (editing) inputRef.current?.focus()
  }, [editing])

  const commit = useCallback(() => {
    if (value !== tx.description) {
      updateDesc({ id: tx.id, description: value })
    }
    setEditing(false)
  }, [value, tx.description, tx.id, updateDesc])

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') commit()
    if (e.key === 'Escape') {
      setValue(tx.description)
      setEditing(false)
    }
  }

  if (editing) {
    return (
      <input
        ref={inputRef}
        type="text"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onBlur={commit}
        onKeyDown={handleKeyDown}
        className="w-full rounded-sm border border-primary bg-accent px-2 py-1 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
        aria-label="Edit description"
      />
    )
  }

  return (
    <>
      <button
        onClick={() => setEditing(true)}
        {...descTip(value || 'Click to add description')}
        className="w-full truncate text-left text-xs text-muted-foreground transition-colors hover:text-foreground"
        aria-label={`Edit description: ${value || 'empty'}`}
      >
        {value ? (
          <HighlightedText text={value} query={searchQuery} />
        ) : (
          <span className="opacity-30">—</span>
        )}
      </button>
      {descTipEl}
    </>
  )
}

function LabelsCell({ tx }: { tx: Transaction }) {
  return <LabelCombobox tx={tx} />
}

type IgnoreReasonAction = {
  label: string
  variant?: 'primary' | 'secondary'
  onClick: () => void
  disabled?: boolean
}

function IgnoreReasonModal({
  title,
  subtitle,
  context,
  placeholder,
  reason,
  onReasonChange,
  actions,
  onCancel,
}: {
  title: string
  subtitle?: string
  context?: string
  placeholder?: string
  reason: string
  onReasonChange: (value: string) => void
  actions: IgnoreReasonAction[]
  onCancel: () => void
}) {
  const { t } = useI18n()
  const inputPlaceholder = placeholder ?? t('common.reasonOptional')

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onCancel()
      }}
    >
      <div className="w-full max-w-sm space-y-4 rounded-lg border border-border bg-card p-6 shadow-xl">
        <h2 className="text-sm font-semibold text-foreground">{title}</h2>
        {subtitle && <p className="text-xs text-muted-foreground">{subtitle}</p>}
        {context && <p className="text-xs text-muted-foreground">{context}</p>}
        <input
          autoFocus
          value={reason}
          onChange={(event) => onReasonChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Escape') onCancel()
          }}
          placeholder={inputPlaceholder}
          className="w-full rounded border border-border bg-secondary px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring"
        />
        <div className="flex flex-col gap-2">
          {actions.map((action) => (
            <button
              key={action.label}
              onClick={action.onClick}
              disabled={action.disabled}
              className={cn(
                'w-full rounded-md px-4 py-2 text-sm transition-colors',
                action.variant === 'primary'
                  ? 'bg-primary text-primary-foreground hover:bg-primary/90'
                  : 'border border-border text-muted-foreground hover:text-foreground',
                action.disabled && 'cursor-not-allowed opacity-50',
              )}
            >
              {action.label}
            </button>
          ))}
          <button
            onClick={onCancel}
            className="w-full px-4 py-1 text-xs text-muted-foreground hover:text-foreground"
          >
            {t('common.cancel')}
          </button>
        </div>
      </div>
    </div>,
    document.body,
  )
}

function CategoryBucketCell({ tx }: { tx: Transaction }) {
  const { data: categories = [] } = useCategories()
  const { data: buckets = [] } = useBuckets()
  const { mutate: updateFields } = useUpdateTransactionFields()
  const { mutate: categorizeMerchant } = useCategorizeMerchant()
  const [pendingMerchantCat, setPendingMerchantCat] = useState<{
    merchant: string
    category: string
    bucket: string
  } | null>(null)
  const [applyError, setApplyError] = useState(false)

  return (
    <div className="flex flex-col gap-0.5">
      <InlineSelect
        value={tx.category}
        options={categories.map((c) => c.name)}
        onCommit={(category) =>
          updateFields(
            { id: tx.id, patch: { category } },
            {
              onSuccess: () => {
                if (tx.merchant_info) {
                  setPendingMerchantCat({ merchant: tx.merchant_info, category, bucket: tx.bucket })
                }
              },
            },
          )
        }
        className="text-foreground"
      />
      <InlineSelect
        value={tx.bucket}
        options={buckets.map((b) => b.name)}
        onCommit={(bucket) =>
          updateFields(
            { id: tx.id, patch: { bucket } },
            {
              onSuccess: () => {
                if (tx.merchant_info) {
                  setPendingMerchantCat({
                    merchant: tx.merchant_info,
                    category: tx.category,
                    bucket,
                  })
                }
              },
            },
          )
        }
        className="text-muted-foreground"
      />

      {pendingMerchantCat && (
        <SlideNotification
          key={`${pendingMerchantCat.merchant}:${pendingMerchantCat.category}:${pendingMerchantCat.bucket}`}
          actions={[
            { label: 'Dismiss', value: false },
            { label: 'Apply', value: true, primary: true },
          ]}
          defaultActionIndex={0}
          onAction={(apply) => {
            if (apply && pendingMerchantCat) {
              categorizeMerchant(
                {
                  merchant: pendingMerchantCat.merchant,
                  category: pendingMerchantCat.category,
                  bucket: pendingMerchantCat.bucket,
                },
                {
                  onSuccess: () => setPendingMerchantCat(null),
                  onError: () => {
                    setPendingMerchantCat(null)
                    setApplyError(true)
                  },
                },
              )
            } else {
              setPendingMerchantCat(null)
            }
          }}
        >
          Apply{' '}
          <span className="font-medium text-foreground">
            &quot;{pendingMerchantCat.category || '(none)'}&quot;
          </span>
          {' / '}
          <span className="font-medium text-foreground">
            {pendingMerchantCat.bucket || '(none)'}
          </span>{' '}
          to all <span className="font-medium text-foreground">{pendingMerchantCat.merchant}</span>{' '}
          transactions?
        </SlideNotification>
      )}

      {applyError && (
        <SlideNotification
          key="categorize-error"
          actions={[{ label: 'Dismiss', value: false }]}
          defaultActionIndex={0}
          onAction={() => setApplyError(false)}
        >
          Failed to apply category to all transactions. Please try again.
        </SlideNotification>
      )}
    </div>
  )
}

function IgnoreButton({ tx }: { tx: Transaction }) {
  const { t } = useI18n()
  const { mutate: ignoreTransaction, isPending } = useIgnoreTransaction()
  const { mutate: ignoreByMerchant } = useIgnoreByMerchant()
  const qc = useQueryClient()
  const { handlers: ignoreTip, tip: ignoreTipEl } = useTooltip()
  const [showModal, setShowModal] = useState(false)
  const [reason, setReason] = useState('')

  const handleIconClick = () => {
    if (tx.muted) {
      ignoreTransaction(
        { id: tx.id, muted: false },
        { onSuccess: () => qc.invalidateQueries({ queryKey: ['transactions'] }) },
      )
      return
    }
    setReason('')
    setShowModal(true)
  }

  const handleJustThis = () => {
    setShowModal(false)
    ignoreTransaction(
      { id: tx.id, muted: true, reason },
      { onSuccess: () => qc.invalidateQueries({ queryKey: ['transactions'] }) },
    )
  }

  const handleIgnoreMerchant = () => {
    setShowModal(false)
    // Also ignore this individual transaction.
    ignoreTransaction({ id: tx.id, muted: true, reason })
    // Ignore all same-merchant transactions.
    ignoreByMerchant({ pattern: tx.merchant_info, reason })
  }

  const label = tx.muted
    ? t('transactions.ignore.tooltip.restore')
    : t('transactions.ignore.tooltip.ignore')

  return (
    <>
      <button
        onClick={handleIconClick}
        disabled={isPending}
        aria-label={label}
        {...(!showModal ? ignoreTip(label) : {})}
        className={cn(
          'flex items-center rounded p-1 transition-colors',
          tx.muted
            ? 'text-warning hover:text-foreground'
            : 'text-muted-foreground/30 hover:text-muted-foreground',
        )}
      >
        {tx.muted ? <EyeOff size={13} /> : <Eye size={13} />}
      </button>
      {!showModal && ignoreTipEl}

      {showModal && (
        <IgnoreReasonModal
          title={t('transactions.ignore.transactionTitle')}
          subtitle={t('transactions.ignore.transactionDescription')}
          context={tx.merchant_info ? `${t('common.merchant')}: ${tx.merchant_info}` : undefined}
          reason={reason}
          onReasonChange={setReason}
          onCancel={() => setShowModal(false)}
          actions={[
            ...(tx.merchant_info
              ? [
                  {
                    label: t('transactions.ignore.merchantAction'),
                    variant: 'primary' as const,
                    onClick: handleIgnoreMerchant,
                  },
                ]
              : []),
            {
              label: t('transactions.ignore.thisTransaction'),
              onClick: handleJustThis,
            },
          ]}
        />
      )}
    </>
  )
}

function TransactionRow({
  tx,
  banks,
  selected,
  onToggleSelect,
  searchQuery,
}: {
  tx: Transaction
  banks?: BankColor[]
  selected: boolean
  onToggleSelect: () => void
  searchQuery: string
}) {
  const { timezone, timeFormat } = useDisplay()
  const { locale } = useI18n()
  return (
    <tr
      className={cn(
        'border-b border-border transition-colors hover:bg-accent/50',
        selected && 'bg-secondary/30',
        tx.muted && 'opacity-40',
      )}
    >
      <td className="px-3 py-2.5">
        <input
          type="checkbox"
          checked={selected}
          onChange={onToggleSelect}
          aria-label={`Select transaction ${tx.merchant_info || tx.message_id}`}
        />
      </td>
      <td className="overflow-hidden whitespace-nowrap px-3 py-2.5">
        <span className="block truncate font-mono text-xs text-muted-foreground">
          {formatDate(tx.timestamp, true, timezone, timeFormat, locale)}
        </span>
      </td>
      <td className="overflow-hidden px-3 py-2.5">
        <span className="block truncate text-sm text-foreground">
          <HighlightedText text={tx.merchant_info} query={searchQuery} />
        </span>
      </td>
      <td className="overflow-hidden whitespace-nowrap px-3 py-2.5">
        {tx.source && (
          <span
            className="inline-block max-w-full truncate rounded-sm border border-border py-0.5 pl-1.5 pr-2 font-mono text-[10px] text-muted-foreground"
            style={{
              borderLeftColor: getSourceColor(sourceDisplay(tx.source), banks),
              borderLeftWidth: '2px',
            }}
          >
            {sourceDisplay(tx.source)}
          </span>
        )}
      </td>
      <td className="px-3 py-2.5">
        <AmountCell tx={tx} />
      </td>
      <td className="overflow-hidden px-3 py-2.5">
        <CategoryBucketCell tx={tx} />
      </td>
      <td className="overflow-hidden px-3 py-2.5">
        <LabelsCell tx={tx} />
      </td>
      <td className="overflow-hidden px-3 py-2.5">
        <DescriptionCell tx={tx} searchQuery={searchQuery} />
      </td>
      <td className="px-3 py-2.5">
        <IgnoreButton tx={tx} />
      </td>
    </tr>
  )
}

// ─── Filter panel with default + "Add filter" extras ────────────────────────

type FilterPanelProps = {
  filters: TransactionFilters
  facets:
    | {
        sources: string[]
        categories: string[]
        currencies: string[]
        labels: string[]
        buckets: string[]
      }
    | undefined
  hourFrom: number | undefined
  hourTo: number | undefined
  updateFilter: (key: keyof Omit<TransactionFilters, 'page' | 'page_size'>, value: string) => void
  updateParams: (updates: Record<string, string | undefined>) => void
}

const EXTRA_FILTER_KEYS = ['category', 'bucket', 'currency', 'hour_from', 'hour_to'] as const
type ExtraFilterKey = (typeof EXTRA_FILTER_KEYS)[number]

const EXTRA_FILTER_LABELS: Record<ExtraFilterKey, string> = {
  category: 'Category',
  bucket: 'Bucket',
  currency: 'Currency',
  hour_from: 'Hour from',
  hour_to: 'Hour to',
}

function FilterPanel({
  filters,
  facets,
  hourFrom,
  hourTo,
  updateFilter,
  updateParams,
}: FilterPanelProps) {
  const [addOpen, setAddOpen] = useState(false)
  const [addHighlighted, setAddHighlighted] = useState(-1)
  const addBtnRef = useRef<HTMLButtonElement>(null)
  const [addPos, setAddPos] = useState<{ x: number; y: number } | null>(null)
  const { handlers: addTip, tip: addTipEl } = useTooltip()

  // Which extra filters are visible — an extra filter is visible if it has an active value,
  // or if the user has explicitly added it via "Add filter".
  const [addedFilters, setAddedFilters] = useState<Set<ExtraFilterKey>>(() => {
    const pre = new Set<ExtraFilterKey>()
    if (filters.category) pre.add('category')
    if (filters.bucket) pre.add('bucket')
    if (filters.currency) pre.add('currency')
    if (hourFrom !== undefined) pre.add('hour_from')
    if (hourTo !== undefined) pre.add('hour_to')
    return pre
  })

  const visibleExtras = EXTRA_FILTER_KEYS.filter(
    (k) =>
      addedFilters.has(k) ||
      (k === 'category' && filters.category) ||
      (k === 'bucket' && filters.bucket) ||
      (k === 'currency' && filters.currency) ||
      (k === 'hour_from' && hourFrom !== undefined) ||
      (k === 'hour_to' && hourTo !== undefined),
  )

  const hiddenExtras = EXTRA_FILTER_KEYS.filter((k) => !visibleExtras.includes(k))

  const openAdd = () => {
    const rect = addBtnRef.current?.getBoundingClientRect()
    if (rect) setAddPos({ x: rect.left, y: rect.bottom + 4 })
    setAddOpen(true)
    setAddHighlighted(-1)
  }

  const closeAdd = () => {
    setAddOpen(false)
    setAddHighlighted(-1)
  }

  const selectExtraFilter = (key: ExtraFilterKey) => {
    setAddedFilters((s) => new Set([...s, key]))
    closeAdd()
  }

  useEffect(() => {
    if (!addOpen) return
    const handler = (e: MouseEvent) => {
      const portal = document.getElementById('add-filter-portal')
      if (!portal?.contains(e.target as Node) && !addBtnRef.current?.contains(e.target as Node)) {
        closeAdd()
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [addOpen])

  return (
    <div className="flex flex-wrap items-center gap-2 rounded-lg border border-border bg-card p-3">
      {/* Always-visible: Date */}
      <DateRangePicker
        value={{
          from: filters.date_from ? new Date(filters.date_from) : undefined,
          to: filters.date_to ? new Date(filters.date_to) : undefined,
        }}
        onChange={(range) =>
          updateParams({
            date_from: range.from ? range.from.toISOString() : undefined,
            date_to: range.to ? range.to.toISOString() : undefined,
            page: '1',
          })
        }
      />

      {/* Always-visible: Source */}
      <FilterCombobox
        value={filters.source ?? ''}
        onChange={(v) => updateFilter('source', v)}
        options={facets?.sources ?? []}
        placeholder="Source"
        label="Filter by source"
      />

      {/* Always-visible: Label */}
      <LabelSearch
        value={filters.label ?? ''}
        onChange={(v) => updateFilter('label', v)}
        options={facets?.labels ?? []}
      />

      {/* Extra filters added by user */}
      {visibleExtras.includes('category') && (
        <FilterCombobox
          value={filters.category ?? ''}
          onChange={(v) => updateFilter('category', v)}
          options={facets?.categories ?? []}
          placeholder="Category"
          label="Filter by category"
        />
      )}
      {visibleExtras.includes('bucket') && (
        <FilterCombobox
          value={filters.bucket ?? ''}
          onChange={(v) => updateFilter('bucket', v)}
          options={facets?.buckets ?? []}
          placeholder="Bucket"
          label="Filter by bucket"
        />
      )}
      {visibleExtras.includes('currency') && (
        <FilterCombobox
          value={filters.currency ?? ''}
          onChange={(v) => updateFilter('currency', v)}
          options={facets?.currencies ?? []}
          placeholder="Currency"
          label="Filter by currency"
        />
      )}
      {visibleExtras.includes('hour_from') && (
        <FilterCombobox
          value={hourFrom !== undefined ? `${String(hourFrom).padStart(2, '0')}:00` : ''}
          onChange={(v) =>
            updateParams({
              hour_from: v ? String(parseInt(v.split(':')[0], 10)) : undefined,
              page: '1',
            })
          }
          options={Array.from({ length: 24 }, (_, i) => `${String(i).padStart(2, '0')}:00`)}
          placeholder="Hour from"
          label="Filter from hour (DB timezone)"
        />
      )}
      {visibleExtras.includes('hour_to') && (
        <FilterCombobox
          value={hourTo !== undefined ? `${String(hourTo).padStart(2, '0')}:59` : ''}
          onChange={(v) =>
            updateParams({
              hour_to: v ? String(parseInt(v.split(':')[0], 10)) : undefined,
              page: '1',
            })
          }
          options={Array.from({ length: 24 }, (_, i) => `${String(i).padStart(2, '0')}:59`)}
          placeholder="Hour to"
          label="Filter to hour (DB timezone)"
        />
      )}

      {/* Add filter button */}
      {hiddenExtras.length > 0 && (
        <>
          <button
            ref={addBtnRef}
            onClick={openAdd}
            onKeyDown={(event) => {
              if (event.key === 'Escape') {
                event.preventDefault()
                closeAdd()
                return
              }
              if (event.key === 'ArrowDown') {
                event.preventDefault()
                if (!addOpen) {
                  openAdd()
                  setAddHighlighted(0)
                  return
                }
                setAddHighlighted((current) => Math.min(current + 1, hiddenExtras.length - 1))
                return
              }
              if (event.key === 'ArrowUp') {
                event.preventDefault()
                if (!addOpen) {
                  openAdd()
                  setAddHighlighted(hiddenExtras.length - 1)
                  return
                }
                setAddHighlighted((current) => Math.max(current - 1, 0))
                return
              }
              if (event.key === 'Enter' && addOpen && addHighlighted >= 0) {
                event.preventDefault()
                selectExtraFilter(hiddenExtras[addHighlighted])
              }
            }}
            aria-haspopup="menu"
            aria-expanded={addOpen}
            {...addTip('Add filter')}
            className="flex items-center gap-1 rounded-md border border-dashed border-border px-2 py-1.5 text-xs text-muted-foreground transition-colors hover:border-primary hover:text-primary"
          >
            <Plus size={11} />
            Add filter
          </button>
          {addTipEl}
        </>
      )}

      {/* Add-filter dropdown portal */}
      {addOpen &&
        addPos &&
        createPortal(
          <div
            id="add-filter-portal"
            role="menu"
            aria-label="Add filter options"
            className="fixed z-50 min-w-[160px] rounded-lg border border-border bg-card py-1 shadow-xl"
            style={{ left: addPos.x, top: addPos.y }}
          >
            {hiddenExtras.map((key, index) => (
              <button
                key={key}
                type="button"
                role="menuitem"
                onClick={() => {
                  selectExtraFilter(key)
                }}
                onMouseEnter={() => setAddHighlighted(index)}
                className={cn(
                  'flex w-full items-center gap-2 px-3 py-2 text-xs text-muted-foreground hover:bg-accent hover:text-foreground',
                  index === addHighlighted && 'bg-accent text-foreground',
                )}
              >
                {EXTRA_FILTER_LABELS[key]}
              </button>
            ))}
          </div>,
          document.body,
        )}
    </div>
  )
}

export function Transactions() {
  const [searchParams, setSearchParams] = useSearchParams()
  const { locale, t } = useI18n()

  // Local state for the raw search input (controlled input); debounced value syncs to URL.
  const [inputValue, setInputValue] = useState(() => searchParams.get('q') ?? '')
  const debouncedSearch = useDebounce(inputValue, 300)
  const isFirstRender = useRef(true)
  const { timezone: displayTimezone } = useDisplay()

  const parseCSVParam = (key: string) =>
    (searchParams.get(key) ?? '')
      .split(',')
      .map((value) => value.trim())
      .filter(Boolean)

  // Derive all other state from URL params.
  const page = Math.max(1, parseInt(searchParams.get('page') ?? '1', 10))
  const rawPageSize = parseInt(searchParams.get('page_size') ?? '20', 10)
  const pageSize = ([20, 50, 100] as const).includes(rawPageSize as 20 | 50 | 100)
    ? (rawPageSize as 20 | 50 | 100)
    : 20
  const sortDir = searchParams.get('sort_dir') === 'asc' ? ('asc' as const) : ('desc' as const)
  const showMuted = searchParams.get('show_muted') === '1'
  const mutedOnly = searchParams.get('muted_only') === '1'
  const rawWeekday = searchParams.get('weekday')
  const rawHourFrom = searchParams.get('hour_from')
  const rawHourTo = searchParams.get('hour_to')
  const weekday = rawWeekday !== null ? parseInt(rawWeekday, 10) : undefined
  const hourFrom = rawHourFrom !== null ? parseInt(rawHourFrom, 10) : undefined
  const hourTo = rawHourTo !== null ? parseInt(rawHourTo, 10) : undefined
  const timezone = searchParams.get('tz') || displayTimezone
  const filters = {
    merchant: searchParams.get('merchant') || undefined,
    category: searchParams.get('category') || undefined,
    category_missing: searchParams.get('category_missing') === '1' || undefined,
    exclude_categories: parseCSVParam('exclude_categories'),
    currency: searchParams.get('currency') || undefined,
    source: searchParams.get('source') || undefined,
    exclude_sources: parseCSVParam('exclude_sources'),
    bucket: searchParams.get('bucket') || undefined,
    bucket_missing: searchParams.get('bucket_missing') === '1' || undefined,
    exclude_buckets: parseCSVParam('exclude_buckets'),
    label: searchParams.get('label') || undefined,
    label_missing: searchParams.get('label_missing') === '1' || undefined,
    exclude_labels: parseCSVParam('exclude_labels'),
    date_from: searchParams.get('date_from') || undefined,
    date_to: searchParams.get('date_to') || undefined,
    show_muted: showMuted || undefined,
    muted_only: mutedOnly || undefined,
    weekday,
    hour_from: hourFrom,
    hour_to: hourTo,
    tz: timezone,
  }

  // Auto-open filter panel on load when URL contains active filters.
  const { handlers: toolTip, tip: toolTipEl } = useTooltip()

  const [showFilters, setShowFilters] = useState(
    () =>
      Boolean(searchParams.get('category')) ||
      Boolean(searchParams.get('category_missing')) ||
      Boolean(searchParams.get('currency')) ||
      Boolean(searchParams.get('source')) ||
      Boolean(searchParams.get('bucket')) ||
      Boolean(searchParams.get('bucket_missing')) ||
      Boolean(searchParams.get('label')) ||
      Boolean(searchParams.get('label_missing')) ||
      Boolean(searchParams.get('exclude_categories')) ||
      Boolean(searchParams.get('exclude_sources')) ||
      Boolean(searchParams.get('exclude_buckets')) ||
      Boolean(searchParams.get('exclude_labels')) ||
      Boolean(searchParams.get('date_from')) ||
      Boolean(searchParams.get('date_to')) ||
      Boolean(searchParams.get('weekday')) ||
      Boolean(searchParams.get('hour_from')) ||
      Boolean(searchParams.get('hour_to')) ||
      searchParams.get('show_filters') === '1',
  )

  // Remove the show_filters transient param on first render — it only controls initial open state.
  useLayoutEffect(() => {
    if (searchParams.get('show_filters') !== '1') return
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        next.delete('show_filters')
        return next
      },
      { replace: true },
    )
  }, []) // intentionally empty — runs once on mount to consume the navigation hint

  // Sync debounced search to URL (skip the initial mount to avoid a spurious write).
  useEffect(() => {
    if (isFirstRender.current) {
      isFirstRender.current = false
      return
    }
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (debouncedSearch) next.set('q', debouncedSearch)
        else next.delete('q')
        next.set('page', '1')
        return next
      },
      { replace: true },
    )
  }, [debouncedSearch])

  const { data: facets } = useFacets()

  // Helper: update one or more URL params at once (pass undefined to delete a key).
  const updateParams = useCallback(
    (updates: Record<string, string | undefined>) => {
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev)
          Object.entries(updates).forEach(([k, v]) => {
            if (v !== undefined && v !== '') next.set(k, v)
            else next.delete(k)
          })
          return next
        },
        { replace: true },
      )
    },
    [setSearchParams],
  )

  const activeFilters: TransactionFilters = {
    ...filters,
    page,
    page_size: pageSize,
    sort_by: 'timestamp',
    sort_dir: sortDir,
  }
  const { data, isLoading, isFetching, error } = useTransactions(activeFilters, debouncedSearch)
  const { data: banks } = useBanks()
  const { mutate: bulkIgnoreTransactions, isPending: isBulkIgnoringTransactions } =
    useBulkIgnoreTransactions()
  const { mutate: bulkIgnoreMerchants, isPending: isBulkIgnoringMerchants } =
    useBulkIgnoreMerchants()

  const transactions = data?.transactions ?? []
  const total = data?.total ?? 0
  const totalAmount = data?.total_amount ?? 0
  const baseCurrency = data?.base_currency ?? 'INR'
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [bulkIgnoreMode, setBulkIgnoreMode] = useState<'transactions' | 'merchants' | null>(null)
  const [bulkReason, setBulkReason] = useState('')

  useEffect(() => {
    const currentPageIDs = new Set(transactions.map((tx) => tx.id))
    setSelectedIds((prev) => new Set([...prev].filter((id) => currentPageIDs.has(id))))
  }, [transactions])

  const selectedTransactions = transactions.filter((tx) => selectedIds.has(tx.id))
  const selectedMerchantPatterns = [
    ...new Set(selectedTransactions.map((tx) => tx.merchant_info).filter(Boolean)),
  ]
  const allSelected = transactions.length > 0 && selectedIds.size === transactions.length
  const selectedCount = selectedIds.size
  const selectedMerchantCount = selectedMerchantPatterns.length
  const selectedAmount = selectedTransactions.reduce((sum, tx) => sum + tx.amount, 0)
  const displayedAmount = selectedCount > 0 ? selectedAmount : totalAmount
  const displayedAmountLabel = selectedCount > 0 ? 'Selected total' : 'Total spend'

  const updateFilter = (
    key: keyof Omit<TransactionFilters, 'page' | 'page_size'>,
    value: string,
  ) => {
    updateParams({ [key]: value || undefined, page: '1' })
  }

  const clearFilters = () => {
    setInputValue('')
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams()
        if (prev.get('page_size')) next.set('page_size', prev.get('page_size')!)
        if (prev.get('sort_dir')) next.set('sort_dir', prev.get('sort_dir')!)
        return next
      },
      { replace: true },
    )
  }

  const toggleSort = () =>
    updateParams({ sort_dir: sortDir === 'desc' ? 'asc' : 'desc', page: '1' })

  const toggleRowSelection = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleSelectAllCurrentPage = () => {
    setSelectedIds(() => {
      if (allSelected) return new Set()
      return new Set(transactions.map((tx) => tx.id))
    })
  }

  const closeBulkIgnoreModal = () => {
    setBulkIgnoreMode(null)
    setBulkReason('')
  }

  const completeBulkIgnore = () => {
    setSelectedIds(new Set())
    closeBulkIgnoreModal()
  }

  const hasActiveFilters = Boolean(
    inputValue ||
    filters.merchant ||
    filters.category ||
    filters.category_missing ||
    filters.currency ||
    filters.source ||
    filters.bucket ||
    filters.bucket_missing ||
    filters.label ||
    filters.label_missing ||
    filters.date_from ||
    filters.date_to ||
    filters.weekday !== undefined ||
    filters.hour_from !== undefined ||
    filters.hour_to !== undefined,
  )

  return (
    <div className="flex min-h-0 max-w-full flex-1 flex-col px-6 py-4">
      {/* Toolbar */}
      <div className="mb-4 space-y-3">
        <div className="flex items-center gap-3">
          <div className="relative max-w-md flex-1">
            <input
              type="search"
              value={inputValue}
              onChange={(e) => setInputValue(e.target.value)}
              placeholder="Search transactions..."
              className="no-native-search-clear w-full rounded-md border border-border bg-secondary py-2 pl-3 pr-8 text-sm text-foreground placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring"
              aria-label="Search transactions"
            />
            {inputValue && (
              <button
                onClick={() => setInputValue('')}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-base leading-none text-muted-foreground hover:text-foreground"
                aria-label="Clear search"
              >
                ×
              </button>
            )}
          </div>
          <button
            onClick={() => setShowFilters(!showFilters)}
            {...toolTip(showFilters ? 'Hide filters' : 'Filters')}
            className={cn(
              'rounded-md border p-2 text-xs transition-colors',
              showFilters || (hasActiveFilters && !inputValue)
                ? 'border-primary bg-primary/10 text-primary'
                : 'border-border text-muted-foreground hover:border-border hover:text-foreground',
            )}
            aria-label={showFilters ? 'Hide filters' : 'Filters'}
            aria-expanded={showFilters}
          >
            <Funnel size={16} />
          </button>
          <button
            onClick={() => updateParams({ show_muted: showMuted ? undefined : '1', page: '1' })}
            {...toolTip(
              showMuted
                ? t('transactions.ignore.hideIgnored')
                : mutedOnly
                  ? t('transactions.ignore.ignoredOnly')
                  : t('transactions.ignore.showIgnored'),
            )}
            className={cn(
              'rounded-md border p-2 text-xs transition-colors',
              showMuted || mutedOnly
                ? 'border-warning/60 bg-warning/10 text-warning'
                : 'border-border text-muted-foreground hover:border-border hover:text-foreground',
            )}
            aria-label={
              showMuted
                ? t('transactions.ignore.hideIgnored')
                : t('transactions.ignore.showIgnored')
            }
          >
            {showMuted || mutedOnly ? <EyeOff size={16} /> : <Eye size={16} />}
          </button>
          {hasActiveFilters && (
            <button
              onClick={clearFilters}
              className="text-xs text-muted-foreground transition-colors hover:text-destructive"
              aria-label="Clear all filters"
            >
              Clear all
            </button>
          )}
        </div>

        {showFilters && (
          <FilterPanel
            filters={filters}
            facets={facets}
            hourFrom={hourFrom}
            hourTo={hourTo}
            updateFilter={updateFilter}
            updateParams={updateParams}
          />
        )}

        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground">
              {isLoading
                ? 'Loading...'
                : `${formatNumberForLocale(total, locale)} ${total === 1 ? 'transaction' : 'transactions'}`}
            </span>
            {!isLoading && (
              <span className="text-xs text-muted-foreground" aria-hidden="true">
                ·
              </span>
            )}
            {!isLoading && (
              <span className="text-xs text-muted-foreground">{displayedAmountLabel}</span>
            )}
            {!isLoading && (
              <span className="font-mono text-xs tabular-nums text-primary">
                {formatCurrency(displayedAmount, baseCurrency, locale)}
              </span>
            )}
            {isFetching && !isLoading && (
              <span className="text-xs text-muted-foreground">· Refreshing...</span>
            )}
          </div>
          <div className="ml-auto flex flex-wrap items-center justify-end gap-3">
            <div
              aria-hidden={selectedCount === 0}
              className={cn(
                'flex min-h-8 flex-wrap items-center gap-2',
                selectedCount === 0 && 'pointer-events-none invisible',
              )}
            >
              <span className="text-xs text-muted-foreground" aria-live="polite">
                {selectedCount} selected
              </span>
              <button
                onClick={() => {
                  setBulkReason('')
                  setBulkIgnoreMode('transactions')
                }}
                disabled={
                  selectedCount === 0 || isBulkIgnoringTransactions || isBulkIgnoringMerchants
                }
                className="rounded border border-warning/40 px-2 py-1 text-xs text-warning hover:bg-warning/10 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {t('transactions.ignore.bulkTransactions')}
              </button>
              <button
                onClick={() => {
                  setBulkReason('')
                  setBulkIgnoreMode('merchants')
                }}
                disabled={
                  selectedCount === 0 ||
                  selectedMerchantCount === 0 ||
                  isBulkIgnoringTransactions ||
                  isBulkIgnoringMerchants
                }
                className="rounded border border-warning/40 px-2 py-1 text-xs text-warning hover:bg-warning/10 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {t('transactions.ignore.bulkMerchants')}
              </button>
              <span {...toolTip('Clear selection')} className="inline-flex">
                <button
                  onClick={() => setSelectedIds(new Set())}
                  disabled={selectedCount === 0}
                  aria-label="Clear selection"
                  className="rounded-md p-1.5 text-muted-foreground transition-colors hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <X size={14} />
                </button>
              </span>
            </div>
          </div>
        </div>
      </div>

      {error && (
        <div className="mb-4 rounded-lg border border-destructive p-4 text-xs text-destructive">
          {error instanceof Error ? error.message : 'Failed to load transactions'}
        </div>
      )}

      <div className="flex-1 overflow-x-auto rounded-lg border border-border bg-card shadow-sm">
        <table
          aria-label="Transactions"
          className="min-w-[96rem] table-fixed"
          style={{ borderCollapse: 'collapse' }}
        >
          <colgroup>
            <col className="w-10" />
            <col className="w-52" />
            <col className="w-72" />
            <col className="w-48" />
            <col className="w-32" />
            <col className="w-56" />
            <col className="w-44" />
            <col className="w-64" />
            <col className="w-10" />
          </colgroup>
          <thead>
            <tr className="border-b border-border bg-secondary/50">
              <th scope="col" className="w-10 px-3 py-2.5">
                <input
                  type="checkbox"
                  checked={allSelected}
                  onChange={toggleSelectAllCurrentPage}
                  aria-label="Select all transactions on current page"
                />
              </th>
              <th
                scope="col"
                className="whitespace-nowrap px-3 py-2.5 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground"
              >
                <button
                  onClick={toggleSort}
                  className="flex items-center gap-1 transition-colors hover:text-foreground"
                  aria-label={`Sort by date ${sortDir === 'desc' ? 'ascending' : 'descending'}`}
                >
                  Date
                  {sortDir === 'desc' ? <ChevronDown size={10} /> : <ChevronUp size={10} />}
                </button>
              </th>
              <th
                scope="col"
                className="px-3 py-2.5 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground"
              >
                Merchant
              </th>
              <th
                scope="col"
                className="whitespace-nowrap px-3 py-2.5 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground"
              >
                Source
              </th>
              <th
                scope="col"
                className="whitespace-nowrap px-3 py-2.5 text-right text-[10px] font-medium uppercase tracking-wider text-muted-foreground"
              >
                Amount
              </th>
              <th
                scope="col"
                className="px-3 py-2.5 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground"
              >
                Category / Bucket
              </th>
              <th
                scope="col"
                className="px-3 py-2.5 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground"
              >
                Labels
              </th>
              <th
                scope="col"
                className="px-3 py-2.5 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground"
              >
                Description
              </th>
              <th scope="col" className="px-3 py-2.5" />
            </tr>
          </thead>
          <tbody className="[&_td]:py-1 [&_td]:text-xs">
            {isLoading
              ? Array.from({ length: 10 }).map((_, i) => (
                  <tr key={i} className="animate-pulse border-b border-border">
                    {Array.from({ length: 8 }).map((_, j) => (
                      <td key={j} className="px-3 py-3">
                        <div className="h-3 rounded-sm bg-secondary" />
                      </td>
                    ))}
                  </tr>
                ))
              : transactions.map((tx) => (
                  <TransactionRow
                    key={tx.id}
                    tx={tx}
                    banks={banks}
                    selected={selectedIds.has(tx.id)}
                    onToggleSelect={() => toggleRowSelection(tx.id)}
                    searchQuery={debouncedSearch}
                  />
                ))}
            {!isLoading && transactions.length === 0 && (
              <tr>
                <td colSpan={9} className="px-3 py-12 text-center text-xs text-muted-foreground">
                  {hasActiveFilters
                    ? 'No transactions match the current filters'
                    : 'No transactions found'}
                </td>
              </tr>
            )}
          </tbody>
        </table>

        <Pagination
          page={page}
          pageSize={pageSize}
          total={total}
          onPage={(n) => updateParams({ page: String(n) })}
          onPageSize={(n) => updateParams({ page_size: String(n), page: '1' })}
        />
      </div>

      {bulkIgnoreMode === 'transactions' && (
        <IgnoreReasonModal
          title={t('transactions.ignore.bulkTransactionsTitle')}
          subtitle={`${t('transactions.ignore.action')} ${selectedCount} selected ${selectedCount === 1 ? 'transaction' : 'transactions'} only.`}
          reason={bulkReason}
          onReasonChange={setBulkReason}
          onCancel={closeBulkIgnoreModal}
          actions={[
            {
              label: `${t('transactions.ignore.bulkSelectedAction')} ${selectedCount}`,
              variant: 'primary',
              disabled: isBulkIgnoringTransactions,
              onClick: () =>
                bulkIgnoreTransactions(
                  { ids: selectedTransactions.map((tx) => tx.id), reason: bulkReason },
                  { onSuccess: completeBulkIgnore },
                ),
            },
          ]}
        />
      )}

      {bulkIgnoreMode === 'merchants' && (
        <IgnoreReasonModal
          title={t('transactions.ignore.bulkMerchantsTitle')}
          subtitle={`${t('transactions.ignore.action')} all transactions from ${selectedMerchantCount} selected ${
            selectedMerchantCount === 1 ? 'merchant' : 'merchants'
          } across the dataset.`}
          reason={bulkReason}
          onReasonChange={setBulkReason}
          onCancel={closeBulkIgnoreModal}
          actions={[
            {
              label: `${t('transactions.ignore.action')} ${selectedMerchantCount} merchants`,
              variant: 'primary',
              disabled: selectedMerchantCount === 0 || isBulkIgnoringMerchants,
              onClick: () =>
                bulkIgnoreMerchants(
                  { patterns: selectedMerchantPatterns, reason: bulkReason },
                  { onSuccess: completeBulkIgnore },
                ),
            },
          ]}
        />
      )}

      {toolTipEl}
    </div>
  )
}

export default Transactions
