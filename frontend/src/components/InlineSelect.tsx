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
  const [editing, setEditing] = useState(false)
  const selectRef = useRef<HTMLSelectElement>(null)

  useEffect(() => {
    if (editing) selectRef.current?.focus()
  }, [editing])

  if (editing) {
    return (
      <select
        ref={selectRef}
        value={value}
        onChange={(e) => {
          if (e.target.value !== value) onCommit(e.target.value)
          setEditing(false)
        }}
        onBlur={() => setEditing(false)}
        onKeyDown={(e) => {
          if (e.key === 'Escape') setEditing(false)
        }}
        className="rounded-sm border border-primary bg-accent px-1 py-0.5 text-xs text-foreground focus:outline-none"
      >
        <option value="">—</option>
        {options.map((o) => (
          <option key={o} value={o}>
            {o}
          </option>
        ))}
      </select>
    )
  }

  return (
    <button
      onClick={() => setEditing(true)}
      className={`text-left text-xs transition-colors hover:text-primary ${className ?? 'text-foreground'}`}
      title="Click to edit"
    >
      {value || <span className="opacity-30">{placeholder}</span>}
    </button>
  )
}
