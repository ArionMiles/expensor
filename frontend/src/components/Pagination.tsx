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
    <div className="flex items-center justify-between px-4 py-3 border-t border-border">
      <span className="text-xs text-muted-foreground">
        {start}–{end} of {total.toLocaleString('en-IN')}
      </span>
      <div className="flex items-center gap-1">
        <button
          onClick={() => onPage(page - 1)}
          disabled={page <= 1}
          className={cn(
            'px-3 py-1.5 text-xs rounded-md border border-border bg-card',
            'text-foreground hover:bg-accent hover:text-accent-foreground transition-colors',
            'disabled:opacity-40 disabled:cursor-not-allowed',
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
            'px-3 py-1.5 text-xs rounded-md border border-border bg-card',
            'text-foreground hover:bg-accent hover:text-accent-foreground transition-colors',
            'disabled:opacity-40 disabled:cursor-not-allowed',
          )}
          aria-label="Next page"
        >
          next →
        </button>
      </div>
    </div>
  )
}
