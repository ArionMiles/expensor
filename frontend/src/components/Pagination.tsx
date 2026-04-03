import { cn } from '@/lib/utils'

interface PaginationProps {
  page: number
  pageSize: number
  total: number
  onPage: (page: number) => void
}

export function Pagination({ page, pageSize, total, onPage }: PaginationProps) {
  const totalPages = Math.ceil(total / pageSize)
  const start = (page - 1) * pageSize + 1
  const end = Math.min(page * pageSize, total)

  if (totalPages <= 1) return null

  return (
    <div className="flex items-center justify-between border-t border-border px-4 py-3">
      <span className="text-xs text-muted-foreground">
        {start}–{end} of {total.toLocaleString('en-IN')}
      </span>
      <div className="flex items-center gap-1">
        <button
          onClick={() => onPage(page - 1)}
          disabled={page <= 1}
          className={cn(
            'rounded-md border border-border bg-card px-3 py-1.5 text-xs',
            'text-foreground transition-colors hover:bg-accent hover:text-accent-foreground',
            'disabled:cursor-not-allowed disabled:opacity-40',
          )}
          aria-label="Previous page"
        >
          ← prev
        </button>
        <span className="px-3 py-1.5 text-xs text-muted-foreground">
          {page} / {totalPages}
        </span>
        <button
          onClick={() => onPage(page + 1)}
          disabled={page >= totalPages}
          className={cn(
            'rounded-md border border-border bg-card px-3 py-1.5 text-xs',
            'text-foreground transition-colors hover:bg-accent hover:text-accent-foreground',
            'disabled:cursor-not-allowed disabled:opacity-40',
          )}
          aria-label="Next page"
        >
          next →
        </button>
      </div>
    </div>
  )
}
