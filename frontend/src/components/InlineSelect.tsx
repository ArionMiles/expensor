import { cn } from '@/lib/utils'
import { useEffect, useRef, useState } from 'react'

interface InlineSelectProps {
  value: string
  options: string[]
  onCommit: (value: string) => void
  placeholder?: string
  className?: string
}

export function InlineSelect({
  value,
  options,
  onCommit,
  placeholder = '—',
  className,
}: InlineSelectProps) {
  const [open, setOpen] = useState(false)
  const [highlighted, setHighlighted] = useState(-1)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (!containerRef.current?.contains(e.target as Node)) setOpen(false)
    }
    const keyHandler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    document.addEventListener('keydown', keyHandler)
    return () => {
      document.removeEventListener('mousedown', handler)
      document.removeEventListener('keydown', keyHandler)
    }
  }, [open])

  const select = (opt: string) => {
    if (opt !== value) onCommit(opt)
    setOpen(false)
    setHighlighted(-1)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setHighlighted((h) => Math.min(h + 1, options.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setHighlighted((h) => Math.max(h - 1, 0))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      if (highlighted >= 0 && options[highlighted]) select(options[highlighted])
    }
  }

  return (
    <div ref={containerRef} className="relative">
      <button
        onClick={() => {
          setOpen(!open)
          setHighlighted(-1)
        }}
        onKeyDown={handleKeyDown}
        className={cn(
          'text-left text-xs transition-colors hover:text-primary focus:outline-none',
          className ?? 'text-foreground',
        )}
        title="Click to edit"
      >
        {value || <span className="opacity-30">{placeholder}</span>}
      </button>

      {open && (
        <ul
          role="listbox"
          className="absolute left-0 top-full z-50 mt-0.5 max-h-48 min-w-[140px] overflow-y-auto rounded-md border border-border bg-card shadow-lg"
        >
          {options.map((opt, i) => (
            <li
              key={opt}
              role="option"
              aria-selected={opt === value}
              onMouseDown={() => select(opt)}
              onMouseEnter={() => setHighlighted(i)}
              className={cn(
                'cursor-pointer px-3 py-1.5 text-xs',
                i === highlighted && 'bg-accent text-accent-foreground',
                opt === value && i !== highlighted && 'text-primary',
                i !== highlighted && opt !== value && 'text-foreground hover:bg-accent/50',
              )}
            >
              {opt}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
