import { cn } from '@/lib/utils'
import { useTooltip } from '@/hooks/useTooltip'
import { useEffect, useId, useRef, useState } from 'react'

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
  const [dropdownPos, setDropdownPos] = useState<{
    top?: number
    bottom?: number
    left: number
    minWidth: number
    maxHeight: number
  } | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)
  const listboxId = useId()
  const { handlers: editTip, tip: editTipEl } = useTooltip()

  useEffect(() => {
    if (!open) return

    const updateDropdownPos = () => {
      const rect = buttonRef.current?.getBoundingClientRect()
      if (!rect) return

      const viewportPadding = 8
      const dropdownGap = 4
      const preferredMaxHeight = 192
      const spaceBelow = window.innerHeight - rect.bottom - viewportPadding
      const spaceAbove = rect.top - viewportPadding
      const openUpward = spaceBelow < 120 && spaceAbove > spaceBelow
      const maxHeight = Math.max(
        96,
        Math.min(
          preferredMaxHeight,
          openUpward ? spaceAbove - dropdownGap : spaceBelow - dropdownGap,
        ),
      )

      setDropdownPos({
        ...(openUpward
          ? { bottom: Math.max(viewportPadding, window.innerHeight - rect.top + dropdownGap) }
          : { top: rect.bottom + dropdownGap }),
        left: Math.max(viewportPadding, rect.left),
        minWidth: Math.max(140, rect.width),
        maxHeight,
      })
    }

    updateDropdownPos()
    window.addEventListener('resize', updateDropdownPos)
    window.addEventListener('scroll', updateDropdownPos, true)
    return () => {
      window.removeEventListener('resize', updateDropdownPos)
      window.removeEventListener('scroll', updateDropdownPos, true)
    }
  }, [open])

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
        ref={buttonRef}
        role="combobox"
        aria-label={value || placeholder}
        aria-expanded={open}
        aria-controls={open ? listboxId : undefined}
        aria-haspopup="listbox"
        onClick={() => {
          setOpen(!open)
          setHighlighted(-1)
        }}
        onKeyDown={handleKeyDown}
        {...editTip('Click to edit')}
        className={cn(
          'text-left text-xs transition-colors hover:text-primary focus:outline-none',
          className ?? 'text-foreground',
        )}
      >
        {value || <span className="opacity-30">{placeholder}</span>}
      </button>
      {editTipEl}

      {open && dropdownPos && (
        <ul
          id={listboxId}
          role="listbox"
          style={{
            position: 'fixed',
            top: dropdownPos.top,
            bottom: dropdownPos.bottom,
            left: dropdownPos.left,
            minWidth: dropdownPos.minWidth,
            maxHeight: dropdownPos.maxHeight,
          }}
          className="z-50 overflow-y-auto rounded-md border border-border bg-card shadow-lg"
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
