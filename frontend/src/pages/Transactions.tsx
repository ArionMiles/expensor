import {
  useAddLabels,
  useRemoveLabel,
  useTransactions,
  useUpdateTransactionDescription,
} from '@/api/queries'
import type { Transaction, TransactionFilters } from '@/api/types'
import { DaemonStatusBar } from '@/components/DaemonStatusBar'
import { LabelChip } from '@/components/LabelChip'
import { Pagination } from '@/components/Pagination'
import { cn, formatCurrency, formatDate } from '@/lib/utils'
import { useCallback, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'

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
      <div className="text-sm font-mono text-primary tabular-nums">
        {formatCurrency(tx.amount, tx.currency)}
      </div>
      {hasOriginal && (
        <div className="text-[10px] font-mono text-muted-foreground tabular-nums">
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
        className="w-full px-2 py-1 text-xs rounded-sm bg-accent border border-primary text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
        aria-label="Edit description"
      />
    )
  }

  return (
    <button
      onClick={() => setEditing(true)}
      className="w-full text-left text-xs text-muted-foreground hover:text-foreground truncate transition-colors"
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
    <div className="flex flex-wrap items-center gap-1 min-w-0">
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
          className="px-1.5 py-0.5 text-xs rounded-sm bg-accent border border-primary text-foreground focus:outline-none w-20"
          aria-label="New label"
        />
      ) : (
        <button
          onClick={() => setAdding(true)}
          className="text-[10px] text-muted-foreground hover:text-primary border border-border hover:border-primary rounded-sm px-1.5 py-0.5 transition-colors"
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
    <tr className="border-b border-border hover:bg-accent/50 transition-colors">
      <td className="px-3 py-2.5 whitespace-nowrap">
        <span className="text-xs font-mono text-muted-foreground">{formatDate(tx.timestamp)}</span>
      </td>
      <td className="px-3 py-2.5 max-w-[200px]">
        <span className="text-sm text-foreground truncate block">{tx.merchant_info}</span>
      </td>
      <td className="px-3 py-2.5">
        <AmountCell tx={tx} />
      </td>
      <td className="px-3 py-2.5">
        <span className="text-xs text-foreground block">{tx.category}</span>
        {tx.bucket && (
          <span className="text-[10px] text-muted-foreground">{tx.bucket}</span>
        )}
      </td>
      <td className="px-3 py-2.5 min-w-[120px] max-w-[200px]">
        <LabelsCell tx={tx} />
      </td>
      <td className="px-3 py-2.5 min-w-[140px] max-w-[250px]">
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

  useEffect(() => {
    setPage(1)
  }, [debouncedSearch, filters])

  const activeFilters: TransactionFilters = { ...filters, page, page_size: 20 }
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

  const hasActiveFilters = Boolean(
    searchInput ||
      filters.category ||
      filters.currency ||
      filters.label ||
      filters.date_from ||
      filters.date_to,
  )

  return (
    <div className="min-h-screen bg-background flex flex-col">
      <header className="border-b border-border px-6 py-3 flex items-center justify-between bg-card">
        <Link
          to="/"
          className="text-sm font-semibold text-primary tracking-wide hover:text-primary/80 transition-colors"
        >
          Expensor
        </Link>
        <nav className="flex items-center gap-4">
          <Link to="/transactions" className="text-xs text-foreground font-medium">
            Transactions
          </Link>
          <Link
            to="/setup"
            className="text-xs text-muted-foreground hover:text-foreground transition-colors"
          >
            Setup
          </Link>
        </nav>
      </header>

      <DaemonStatusBar />

      <main className="flex-1 flex flex-col px-6 py-4 max-w-full">
        {/* Toolbar */}
        <div className="mb-4 space-y-3">
          <div className="flex items-center gap-3">
            <div className="relative flex-1 max-w-md">
              <input
                type="text"
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                placeholder="Search transactions..."
                className="w-full pl-3 pr-8 py-2 text-sm rounded-md bg-secondary border border-border text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring focus:border-primary"
                aria-label="Search transactions"
              />
              {searchInput && (
                <button
                  onClick={() => setSearchInput('')}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground text-base leading-none"
                  aria-label="Clear search"
                >
                  ×
                </button>
              )}
            </div>
            <button
              onClick={() => setShowFilters(!showFilters)}
              className={cn(
                'px-3 py-2 text-xs rounded-md border transition-colors',
                showFilters || (hasActiveFilters && !searchInput)
                  ? 'border-primary text-primary bg-primary/10'
                  : 'border-border text-muted-foreground hover:text-foreground hover:border-border',
              )}
              aria-label="Toggle filters"
              aria-expanded={showFilters}
            >
              Filters{hasActiveFilters ? ' ●' : ''}
            </button>
            {hasActiveFilters && (
              <button
                onClick={clearFilters}
                className="text-xs text-muted-foreground hover:text-destructive transition-colors"
                aria-label="Clear all filters"
              >
                Clear all
              </button>
            )}
          </div>

          {showFilters && (
            <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2 p-3 rounded-lg border border-border bg-card">
              <input
                type="date"
                value={filters.date_from ?? ''}
                onChange={(e) => updateFilter('date_from', e.target.value)}
                className="px-2 py-1.5 text-xs rounded-md bg-secondary border border-border text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
                aria-label="From date"
                title="From date"
              />
              <input
                type="date"
                value={filters.date_to ?? ''}
                onChange={(e) => updateFilter('date_to', e.target.value)}
                className="px-2 py-1.5 text-xs rounded-md bg-secondary border border-border text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
                aria-label="To date"
                title="To date"
              />
              <input
                type="text"
                value={filters.category ?? ''}
                onChange={(e) => updateFilter('category', e.target.value)}
                placeholder="Category"
                className="px-2 py-1.5 text-xs rounded-md bg-secondary border border-border text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
                aria-label="Filter by category"
              />
              <input
                type="text"
                value={filters.currency ?? ''}
                onChange={(e) => updateFilter('currency', e.target.value)}
                placeholder="Currency"
                className="px-2 py-1.5 text-xs rounded-md bg-secondary border border-border text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
                aria-label="Filter by currency"
              />
              <input
                type="text"
                value={filters.label ?? ''}
                onChange={(e) => updateFilter('label', e.target.value)}
                placeholder="Label"
                className="px-2 py-1.5 text-xs rounded-md bg-secondary border border-border text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
                aria-label="Filter by label"
              />
            </div>
          )}

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
        </div>

        {error && (
          <div className="p-4 rounded-lg border border-destructive text-xs text-destructive mb-4">
            {error instanceof Error ? error.message : 'Failed to load transactions'}
          </div>
        )}

        <div className="flex-1 rounded-lg border border-border overflow-x-auto bg-card shadow-sm">
          <table className="w-full" style={{ borderCollapse: 'collapse' }}>
            <thead>
              <tr className="border-b border-border bg-secondary/50">
                <th className="px-3 py-2.5 text-left text-[10px] font-medium text-muted-foreground uppercase tracking-wider whitespace-nowrap">
                  Date
                </th>
                <th className="px-3 py-2.5 text-left text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
                  Merchant
                </th>
                <th className="px-3 py-2.5 text-right text-[10px] font-medium text-muted-foreground uppercase tracking-wider whitespace-nowrap">
                  Amount
                </th>
                <th className="px-3 py-2.5 text-left text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
                  Category / Bucket
                </th>
                <th className="px-3 py-2.5 text-left text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
                  Labels
                </th>
                <th className="px-3 py-2.5 text-left text-[10px] font-medium text-muted-foreground uppercase tracking-wider">
                  Description
                </th>
              </tr>
            </thead>
            <tbody>
              {isLoading
                ? Array.from({ length: 10 }).map((_, i) => (
                    <tr key={i} className="border-b border-border animate-pulse">
                      {Array.from({ length: 6 }).map((_, j) => (
                        <td key={j} className="px-3 py-3">
                          <div className="h-3 bg-secondary rounded-sm" />
                        </td>
                      ))}
                    </tr>
                  ))
                : transactions.map((tx) => <TransactionRow key={tx.id} tx={tx} />)}
              {!isLoading && transactions.length === 0 && (
                <tr>
                  <td
                    colSpan={6}
                    className="px-3 py-12 text-center text-xs text-muted-foreground"
                  >
                    {hasActiveFilters
                      ? 'No transactions match the current filters'
                      : 'No transactions found'}
                  </td>
                </tr>
              )}
            </tbody>
          </table>

          <Pagination page={page} pageSize={20} total={total} onPage={setPage} />
        </div>
      </main>
    </div>
  )
}

export default Transactions
