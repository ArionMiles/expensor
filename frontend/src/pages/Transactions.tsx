import {
  useAddLabels,
  useFacets,
  useRemoveLabel,
  useTransactions,
  useUpdateTransactionDescription,
} from '@/api/queries'
import type { Transaction, TransactionFilters } from '@/api/types'
import { DateRangePicker } from '@/components/DateRangePicker'
import { FilterCombobox } from '@/components/FilterCombobox'
import { LabelChip } from '@/components/LabelChip'
import { LabelSearch } from '@/components/LabelSearch'
import { Pagination } from '@/components/Pagination'
import { cn, formatCurrency, formatDate } from '@/lib/utils'
import { ChevronDown, ChevronUp } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'

function useDebounce<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(timer)
  }, [value, delay])
  return debounced
}

function AmountCell({ tx }: { tx: Transaction }) {
  const hasOriginal =
    tx.original_amount !== undefined &&
    tx.original_currency !== undefined &&
    tx.original_currency !== tx.currency

  return (
    <div className="text-right">
      <div className="font-mono text-sm tabular-nums text-primary">
        {formatCurrency(tx.amount, tx.currency)}
      </div>
      {hasOriginal && (
        <div className="font-mono text-[10px] tabular-nums text-muted-foreground">
          {formatCurrency(tx.original_amount!, tx.original_currency!)}
          {tx.exchange_rate !== undefined && ` @ ${tx.exchange_rate.toFixed(2)}`}
        </div>
      )}
    </div>
  )
}

function DescriptionCell({ tx }: { tx: Transaction }) {
  const [editing, setEditing] = useState(false)
  const [value, setValue] = useState(tx.description)
  const inputRef = useRef<HTMLInputElement>(null)
  const { mutate: updateDesc } = useUpdateTransactionDescription()

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
    <button
      onClick={() => setEditing(true)}
      className="w-full truncate text-left text-xs text-muted-foreground transition-colors hover:text-foreground"
      title={value || 'click to add description'}
      aria-label={`Edit description: ${value || 'empty'}`}
    >
      {value || <span className="opacity-30">—</span>}
    </button>
  )
}

function LabelsCell({ tx }: { tx: Transaction }) {
  const [adding, setAdding] = useState(false)
  const [newLabel, setNewLabel] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const { mutate: addLabels } = useAddLabels()
  const { mutate: removeLabel } = useRemoveLabel()

  useEffect(() => {
    if (adding) inputRef.current?.focus()
  }, [adding])

  const commitAdd = useCallback(() => {
    const trimmed = newLabel.trim()
    if (trimmed && !tx.labels.includes(trimmed)) {
      addLabels({ id: tx.id, labels: [trimmed] })
    }
    setNewLabel('')
    setAdding(false)
  }, [newLabel, tx.labels, tx.id, addLabels])

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') commitAdd()
    if (e.key === 'Escape') {
      setNewLabel('')
      setAdding(false)
    }
  }

  return (
    <div className="flex min-w-0 flex-wrap items-center gap-1">
      {tx.labels.map((label) => (
        <LabelChip key={label} label={label} onRemove={() => removeLabel({ id: tx.id, label })} />
      ))}
      {adding ? (
        <input
          ref={inputRef}
          type="text"
          value={newLabel}
          onChange={(e) => setNewLabel(e.target.value)}
          onBlur={commitAdd}
          onKeyDown={handleKeyDown}
          placeholder="label..."
          className="w-20 rounded-sm border border-primary bg-accent px-1.5 py-0.5 text-xs text-foreground focus:outline-none"
          aria-label="New label"
        />
      ) : (
        <button
          onClick={() => setAdding(true)}
          className="rounded-sm border border-border px-1.5 py-0.5 text-[10px] text-muted-foreground transition-colors hover:border-primary hover:text-primary"
          aria-label="Add label"
        >
          +
        </button>
      )}
    </div>
  )
}

function TransactionRow({ tx }: { tx: Transaction }) {
  return (
    <tr className="border-b border-border transition-colors hover:bg-accent/50">
      <td className="whitespace-nowrap px-3 py-2.5">
        <span className="font-mono text-xs text-muted-foreground">
          {formatDate(tx.timestamp, true)}
        </span>
      </td>
      <td className="max-w-[200px] px-3 py-2.5">
        <span className="block truncate text-sm text-foreground">{tx.merchant_info}</span>
      </td>
      <td className="whitespace-nowrap px-3 py-2.5">
        {tx.source && (
          <span className="inline-block rounded-sm border border-border px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground">
            {tx.source}
          </span>
        )}
      </td>
      <td className="px-3 py-2.5">
        <AmountCell tx={tx} />
      </td>
      <td className="px-3 py-2.5">
        <span className="block text-xs text-foreground">{tx.category}</span>
        {tx.bucket && <span className="text-[10px] text-muted-foreground">{tx.bucket}</span>}
      </td>
      <td className="min-w-[120px] max-w-[200px] px-3 py-2.5">
        <LabelsCell tx={tx} />
      </td>
      <td className="min-w-[140px] max-w-[250px] px-3 py-2.5">
        <DescriptionCell tx={tx} />
      </td>
    </tr>
  )
}

export function Transactions() {
  const [searchInput, setSearchInput] = useState('')
  const debouncedSearch = useDebounce(searchInput, 300)
  const [page, setPage] = useState(1)
  const [filters, setFilters] = useState<Omit<TransactionFilters, 'page' | 'page_size'>>({})
  const [showFilters, setShowFilters] = useState(false)
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')
  const [pageSize, setPageSize] = useState(20)

  const { data: facets } = useFacets()

  useEffect(() => {
    setPage(1)
  }, [debouncedSearch, filters, pageSize])

  const activeFilters: TransactionFilters = {
    ...filters,
    page,
    page_size: pageSize,
    sort_by: 'timestamp',
    sort_dir: sortDir,
  }
  const { data, isLoading, isFetching, error } = useTransactions(activeFilters, debouncedSearch)

  const transactions = data?.transactions ?? []
  const total = data?.total ?? 0

  const updateFilter = (
    key: keyof Omit<TransactionFilters, 'page' | 'page_size'>,
    value: string,
  ) => {
    setFilters((prev) => ({ ...prev, [key]: value || undefined }))
  }

  const clearFilters = () => {
    setFilters({})
    setSearchInput('')
  }

  const toggleSort = () => setSortDir((d) => (d === 'desc' ? 'asc' : 'desc'))

  const hasActiveFilters = Boolean(
    searchInput ||
    filters.category ||
    filters.currency ||
    filters.source ||
    filters.label ||
    filters.date_from ||
    filters.date_to,
  )

  return (
    <div className="flex min-h-0 max-w-full flex-1 flex-col px-6 py-4">
      {/* Toolbar */}
      <div className="mb-4 space-y-3">
        <div className="flex items-center gap-3">
          <div className="relative max-w-md flex-1">
            <input
              type="text"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder="Search transactions..."
              className="w-full rounded-md border border-border bg-secondary py-2 pl-3 pr-8 text-sm text-foreground placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring"
              aria-label="Search transactions"
            />
            {searchInput && (
              <button
                onClick={() => setSearchInput('')}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-base leading-none text-muted-foreground hover:text-foreground"
                aria-label="Clear search"
              >
                ×
              </button>
            )}
          </div>
          <button
            onClick={() => setShowFilters(!showFilters)}
            className={cn(
              'rounded-md border px-3 py-2 text-xs transition-colors',
              showFilters || (hasActiveFilters && !searchInput)
                ? 'border-primary bg-primary/10 text-primary'
                : 'border-border text-muted-foreground hover:border-border hover:text-foreground',
            )}
            aria-label="Toggle filters"
            aria-expanded={showFilters}
          >
            Filters{hasActiveFilters ? ' ●' : ''}
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
          <div className="grid grid-cols-2 gap-2 rounded-lg border border-border bg-card p-3 sm:grid-cols-3 lg:grid-cols-5">
            <DateRangePicker
              value={{
                from: filters.date_from ? new Date(filters.date_from) : undefined,
                to: filters.date_to ? new Date(filters.date_to) : undefined,
              }}
              onChange={(range) => {
                let dateTo: string | undefined
                if (range.to) {
                  const end = new Date(range.to)
                  end.setHours(23, 59, 59, 999)
                  dateTo = end.toISOString()
                }
                setFilters((prev) => ({
                  ...prev,
                  date_from: range.from ? range.from.toISOString() : undefined,
                  date_to: dateTo,
                }))
              }}
            />
            <FilterCombobox
              value={filters.source ?? ''}
              onChange={(v) => updateFilter('source', v)}
              options={facets?.sources ?? []}
              placeholder="Source"
              label="Filter by source"
            />
            <FilterCombobox
              value={filters.category ?? ''}
              onChange={(v) => updateFilter('category', v)}
              options={facets?.categories ?? []}
              placeholder="Category"
              label="Filter by category"
            />
            <FilterCombobox
              value={filters.currency ?? ''}
              onChange={(v) => updateFilter('currency', v)}
              options={facets?.currencies ?? []}
              placeholder="Currency"
              label="Filter by currency"
            />
            <LabelSearch
              value={filters.label ?? ''}
              onChange={(v) => updateFilter('label', v)}
              options={facets?.labels ?? []}
            />
          </div>
        )}

        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground">
              {isLoading
                ? 'Loading...'
                : `${total.toLocaleString('en-IN')} ${total === 1 ? 'transaction' : 'transactions'}`}
            </span>
            {isFetching && !isLoading && (
              <span className="text-xs text-muted-foreground">· Refreshing...</span>
            )}
          </div>
          <div className="flex items-center gap-1">
            <span className="text-xs text-muted-foreground">Per page:</span>
            {([20, 50, 100] as const).map((n) => (
              <button
                key={n}
                onClick={() => setPageSize(n)}
                className={cn(
                  'rounded px-2 py-0.5 text-xs transition-colors',
                  pageSize === n
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:text-foreground',
                )}
                aria-pressed={pageSize === n}
              >
                {n}
              </button>
            ))}
          </div>
        </div>
      </div>

      {error && (
        <div className="mb-4 rounded-lg border border-destructive p-4 text-xs text-destructive">
          {error instanceof Error ? error.message : 'Failed to load transactions'}
        </div>
      )}

      <div className="flex-1 overflow-x-auto rounded-lg border border-border bg-card shadow-sm">
        <table className="w-full" style={{ borderCollapse: 'collapse' }}>
          <thead>
            <tr className="border-b border-border bg-secondary/50">
              <th className="whitespace-nowrap px-3 py-2.5 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                <button
                  onClick={toggleSort}
                  className="flex items-center gap-1 transition-colors hover:text-foreground"
                  aria-label={`Sort by date ${sortDir === 'desc' ? 'ascending' : 'descending'}`}
                >
                  Date
                  {sortDir === 'desc' ? <ChevronDown size={10} /> : <ChevronUp size={10} />}
                </button>
              </th>
              <th className="px-3 py-2.5 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                Merchant
              </th>
              <th className="whitespace-nowrap px-3 py-2.5 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                Source
              </th>
              <th className="whitespace-nowrap px-3 py-2.5 text-right text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                Amount
              </th>
              <th className="px-3 py-2.5 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                Category / Bucket
              </th>
              <th className="px-3 py-2.5 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                Labels
              </th>
              <th className="px-3 py-2.5 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                Description
              </th>
            </tr>
          </thead>
          <tbody>
            {isLoading
              ? Array.from({ length: 10 }).map((_, i) => (
                  <tr key={i} className="animate-pulse border-b border-border">
                    {Array.from({ length: 7 }).map((_, j) => (
                      <td key={j} className="px-3 py-3">
                        <div className="h-3 rounded-sm bg-secondary" />
                      </td>
                    ))}
                  </tr>
                ))
              : transactions.map((tx) => <TransactionRow key={tx.id} tx={tx} />)}
            {!isLoading && transactions.length === 0 && (
              <tr>
                <td colSpan={7} className="px-3 py-12 text-center text-xs text-muted-foreground">
                  {hasActiveFilters
                    ? 'No transactions match the current filters'
                    : 'No transactions found'}
                </td>
              </tr>
            )}
          </tbody>
        </table>

        <Pagination page={page} pageSize={pageSize} total={total} onPage={setPage} />
      </div>
    </div>
  )
}

export default Transactions
