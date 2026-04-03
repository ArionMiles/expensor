import { cn } from '@/lib/utils'
import { useEffect, useRef, useState } from 'react'

interface FilterComboboxProps {
  value: string
  onChange: (value: string) => void
  options: string[]
  placeholder: string
  label: string
}

export function FilterCombobox({
  value,
  onChange,
  options,
  placeholder,
  label,
}: FilterComboboxProps) {
  const [open, setOpen] = useState(false)
  const [input, setInput] = useState(value)
  const [highlighted, setHighlighted] = useState(-1)
  const containerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  // Keep input in sync with controlled value
  useEffect(() => {
    setInput(value)
  }, [value])

  // Close on outside click
  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (!containerRef.current?.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  const filtered = options.filter((o) => o.toLowerCase().includes(input.toLowerCase()))

  const select = (opt: string) => {
    onChange(opt)
    setInput(opt)
    setOpen(false)
    setHighlighted(-1)
  }

  const clear = () => {
    onChange('')
    setInput('')
    setOpen(false)
    inputRef.current?.focus()
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
          ref={inputRef}
          type="text"
          value={input}
          onChange={(e) => {
            const v = e.target.value
            setInput(v)
            setOpen(true)
            setHighlighted(-1)
            onChange(v)
          }}
          onFocus={() => setOpen(true)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          aria-label={label}
          role="combobox"
          aria-expanded={open}
          aria-autocomplete="list"
          className="w-full rounded-md border border-border bg-secondary py-1.5 pl-2 pr-6 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
        />
        {(input || value) && (
          <button
            onClick={clear}
            className="absolute right-1.5 top-1/2 -translate-y-1/2 text-sm leading-none text-muted-foreground hover:text-foreground"
            aria-label={`Clear ${label}`}
            tabIndex={-1}
          >
            ×
          </button>
        )}
      </div>

      {open && filtered.length > 0 && (
        <ul
          role="listbox"
          aria-label={`${label} options`}
          className="absolute z-50 mt-1 max-h-48 w-full overflow-y-auto rounded-md border border-border bg-card shadow-lg"
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
