import { cn } from '@/lib/utils'
import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

interface PaginationProps {
  page: number
  pageSize: number
  total: number
  onPage: (page: number) => void
  onPageSize?: (pageSize: number) => void
}

const PAGE_SIZE_OPTIONS = [20, 50, 100] as const
const MENU_ESTIMATED_HEIGHT = 112

type PageSizeMenuPosition = {
  top?: number
  bottom?: number
  left: number
  minWidth: number
  maxHeight: number
}

export function pageSizeMenuPosition(
  rect: Pick<DOMRect, 'top' | 'bottom' | 'left' | 'right' | 'width'>,
  viewport: { width: number; height: number },
): PageSizeMenuPosition {
  const viewportPadding = 8
  const menuGap = 4
  const spaceBelow = viewport.height - rect.bottom - viewportPadding
  const spaceAbove = rect.top - viewportPadding
  const openUpward = spaceBelow < MENU_ESTIMATED_HEIGHT && spaceAbove > spaceBelow
  const maxHeight = Math.max(
    80,
    Math.min(MENU_ESTIMATED_HEIGHT, openUpward ? spaceAbove - menuGap : spaceBelow - menuGap),
  )
  const minWidth = Math.max(72, rect.width)
  const left = Math.min(
    Math.max(viewportPadding, rect.left),
    Math.max(viewportPadding, viewport.width - minWidth - viewportPadding),
  )

  return {
    ...(openUpward
      ? { bottom: Math.max(viewportPadding, viewport.height - rect.top + menuGap) }
      : { top: rect.bottom + menuGap }),
    left,
    minWidth,
    maxHeight,
  }
}

export function Pagination({ page, pageSize, total, onPage, onPageSize }: PaginationProps) {
  const [open, setOpen] = useState(false)
  const [menuPos, setMenuPos] = useState<PageSizeMenuPosition | null>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)
  const totalPages = Math.ceil(total / pageSize)
  const start = (page - 1) * pageSize + 1
  const end = Math.min(page * pageSize, total)

  useEffect(() => {
    if (!open) return
    const update = () => {
      const rect = buttonRef.current?.getBoundingClientRect()
      if (!rect) return
      setMenuPos(
        pageSizeMenuPosition(rect, { width: window.innerWidth, height: window.innerHeight }),
      )
    }
    update()
    window.addEventListener('resize', update)
    window.addEventListener('scroll', update, true)
    return () => {
      window.removeEventListener('resize', update)
      window.removeEventListener('scroll', update, true)
    }
  }, [open])

  useEffect(() => {
    if (!open) return
    const close = (event: MouseEvent) => {
      if (!buttonRef.current?.contains(event.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', close)
    return () => document.removeEventListener('mousedown', close)
  }, [open])

  if (total <= 0) return null

  return (
    <div className="flex items-center justify-between border-t border-border px-4 py-3">
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
      {open &&
        menuPos &&
        createPortal(
          <div
            role="menu"
            aria-label="Rows per page"
            className="fixed z-50 rounded-md border border-border bg-card py-1 shadow-lg"
            style={{
              top: menuPos.top,
              bottom: menuPos.bottom,
              left: menuPos.left,
              minWidth: menuPos.minWidth,
              maxHeight: menuPos.maxHeight,
            }}
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
          </div>,
          document.body,
        )}
    </div>
  )
}
