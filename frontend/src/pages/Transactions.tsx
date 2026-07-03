import {
  useBanks,
  useBulkIgnoreMerchants,
  useBulkIgnoreTransactions,
  useFacets,
  useTransactions,
} from '@/api/queries'
import type { TransactionFilters } from '@/api/types'
import { Pagination } from '@/components/Pagination'
import { useDisplay } from '@/contexts/DisplayContext'
import { useTooltip } from '@/hooks/useTooltip'
import { formatNumberForLocale } from '@/i18n/format'
import { useI18n } from '@/i18n/I18nProvider'
import { toggleOrderedSelection } from '@/lib/rangeSelection'
import { cn, formatCurrency } from '@/lib/utils'
import { ChevronDown, ChevronUp, Eye, EyeOff, Funnel, X } from 'lucide-react'
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  FilterPanel,
  IgnoreReasonModal,
  TransactionRow,
  useDebounce,
} from './transactions/TransactionsParts'

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
    source_type: searchParams.get('source_type') || undefined,
    exclude_source_types: parseCSVParam('exclude_source_types'),
    bank: searchParams.get('bank') || undefined,
    exclude_banks: parseCSVParam('exclude_banks'),
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
      Boolean(searchParams.get('source_type')) ||
      Boolean(searchParams.get('bank')) ||
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
  const selectionAnchorIdRef = useRef<string | null>(null)
  const [bulkIgnoreMode, setBulkIgnoreMode] = useState<'transactions' | 'merchants' | null>(null)
  const [bulkReason, setBulkReason] = useState('')

  useEffect(() => {
    const currentPageIDs = new Set(transactions.map((tx) => tx.id))
    if (selectionAnchorIdRef.current && !currentPageIDs.has(selectionAnchorIdRef.current)) {
      selectionAnchorIdRef.current = null
    }
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

  const toggleRowSelection = (id: string, extendRange: boolean) => {
    setSelectedIds((prev) => {
      const result = toggleOrderedSelection({
        orderedIds: transactions.map((tx) => tx.id),
        selectedIds: prev,
        id,
        anchorId: selectionAnchorIdRef.current,
        extendRange,
      })
      selectionAnchorIdRef.current = result.anchorId
      return result.selectedIds
    })
  }

  const toggleSelectAllCurrentPage = () => {
    selectionAnchorIdRef.current = null
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
    filters.source_type ||
    filters.bank ||
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
          className="min-w-[78rem] table-fixed"
          style={{ borderCollapse: 'collapse' }}
        >
          <colgroup>
            <col className="w-10" />
            <col className="w-40" />
            <col className="w-60" />
            <col className="w-28" />
            <col className="w-32" />
            <col className="w-44" />
            <col className="w-48" />
            <col className="w-52" />
            <col className="w-10" />
          </colgroup>
          <thead>
            <tr className="border-b border-border bg-secondary/50">
              <th scope="col" className="w-10 px-0 py-2.5 text-center">
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
                {t('transactions.columns.bankType')}
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
              <th scope="col" className="sticky right-0 bg-secondary/50 px-3 py-2.5" />
            </tr>
          </thead>
          <tbody className="[&_td]:py-1 [&_td]:text-xs">
            {isLoading
              ? Array.from({ length: 10 }).map((_, i) => (
                  <tr key={i} className="animate-pulse border-b border-border">
                    {Array.from({ length: 9 }).map((_, j) => (
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
                    onToggleSelect={(event) => toggleRowSelection(tx.id, event.shiftKey)}
                    searchQuery={debouncedSearch}
                  />
                ))}
            {!isLoading && transactions.length === 0 && (
              <tr>
                <td colSpan={9} className="px-3 py-12 text-center text-xs text-muted-foreground">
                  {hasActiveFilters
                    ? t('transactions.empty.filtered')
                    : t('transactions.empty.none')}
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
