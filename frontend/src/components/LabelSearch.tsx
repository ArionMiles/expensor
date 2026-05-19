import { cn } from '@/lib/utils'
import { useEffect, useRef, useState } from 'react'

interface LabelSearchProps {
  value: string
  onChange: (value: string) => void
  options: string[]
}

export function LabelSearch({ value, onChange, options }: LabelSearchProps) {
  const [input, setInput] = useState(value)
  const [open, setOpen] = useState(false)
  const [highlighted, setHighlighted] = useState(-1)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    setInput(value)
  }, [value])

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (!containerRef.current?.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  const filtered =
    input.length > 0 ? options.filter((o) => o.toLowerCase().includes(input.toLowerCase())) : []

  const select = (opt: string) => {
    onChange(opt)
    setInput(opt)
    setOpen(false)
    setHighlighted(-1)
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      setOpen(false)
      return
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setHighlighted((h) => Math.min(h + 1, filtered.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setHighlighted((h) => Math.max(h - 1, 0))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      if (highlighted >= 0 && filtered[highlighted]) {
        select(filtered[highlighted])
      } else {
        onChange(input)
        setOpen(false)
      }
    }
  }

  return (
    <div ref={containerRef} className="relative">
      <div className="relative">
        <input
          type="text"
          value={input}
          onChange={(e) => {
            setInput(e.target.value)
            setOpen(e.target.value.length > 0)
            setHighlighted(-1)
          }}
          onKeyDown={handleKeyDown}
          placeholder="Label"
          aria-label="Filter by label"
          className="w-full rounded-md border border-border bg-secondary py-1.5 pl-2 pr-6 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
        />
        {input && (
          <button
            onClick={() => {
              onChange('')
              setInput('')
              setOpen(false)
            }}
            className="absolute right-1.5 top-1/2 -translate-y-1/2 text-sm leading-none text-muted-foreground hover:text-foreground"
            aria-label="Clear label filter"
            tabIndex={-1}
          >
            ×
          </button>
        )}
      </div>

      {open && filtered.length > 0 && (
        <ul
          role="listbox"
          className="absolute z-50 mt-1 max-h-40 w-full overflow-y-auto rounded-md border border-border bg-card shadow-lg"
        >
          {filtered.map((opt, i) => (
            <li
              key={opt}
              role="option"
              aria-selected={opt === value}
              onMouseDown={() => select(opt)}
              className={cn(
                'cursor-pointer px-2 py-1.5 text-xs',
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
