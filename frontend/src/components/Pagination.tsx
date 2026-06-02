import { FloatingDropdown } from '@/components/Combobox'
import { cn } from '@/lib/utils'
import { useRef, useState } from 'react'

interface PaginationProps {
  page: number
  pageSize: number
  total: number
  onPage: (page: number) => void
  onPageSize?: (pageSize: number) => void
}

const PAGE_SIZE_OPTIONS = [20, 50, 100] as const

export function Pagination({ page, pageSize, total, onPage, onPageSize }: PaginationProps) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)
  const totalPages = Math.ceil(total / pageSize)
  const start = (page - 1) * pageSize + 1
  const end = Math.min(page * pageSize, total)

  if (total <= 0) return null

  return (
    <div
      ref={containerRef}
      className="flex items-center justify-between border-t border-border px-4 py-3"
    >
      <div className="text-xs text-muted-foreground">
        {start}-{end} of {total.toLocaleString('en-IN')}
        {onPageSize && (
          <>
            <span className="px-1.5">·</span>
            <button
              ref={buttonRef}
              type="button"
              aria-label={`Rows per page: ${pageSize}`}
              aria-expanded={open}
              aria-haspopup="menu"
              onClick={() => setOpen((value) => !value)}
              className="rounded border border-primary/40 bg-primary/10 px-1.5 py-0.5 font-mono text-primary transition-colors hover:bg-primary/20"
            >
              {pageSize}
            </button>
            <span className="pl-1">per page</span>
          </>
        )}
      </div>
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
            aria-label="Rows per page"
            className="fixed z-50 rounded-md border border-border bg-card py-1 shadow-lg"
            style={style}
          >
            {PAGE_SIZE_OPTIONS.map((option) => (
              <button
                key={option}
                type="button"
                role="menuitem"
                onMouseDown={() => {
                  onPageSize?.(option)
                  setOpen(false)
                }}
                className={cn(
                  'block w-full px-3 py-1.5 text-left text-xs transition-colors hover:bg-accent hover:text-foreground',
                  option === pageSize ? 'text-primary' : 'text-muted-foreground',
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
