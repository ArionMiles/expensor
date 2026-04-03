import { useAddLabels, useCreateLabel, useLabels, useRemoveLabel } from '@/api/queries'
import type { Transaction } from '@/api/types'
import { LabelChip } from '@/components/LabelChip'
import { cn } from '@/lib/utils'
import { useEffect, useRef, useState } from 'react'

interface LabelComboboxProps {
  tx: Transaction
}

export function LabelCombobox({ tx }: LabelComboboxProps) {
  const [open, setOpen] = useState(false)
  const [input, setInput] = useState('')
  const containerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const { data: labels = [] } = useLabels()
  const { mutate: addLabels } = useAddLabels()
  const { mutate: removeLabel } = useRemoveLabel()
  const { mutate: createLabel } = useCreateLabel()

  useEffect(() => {
    if (open) inputRef.current?.focus()
  }, [open])

  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (!containerRef.current?.contains(e.target as Node)) {
        setOpen(false)
        setInput('')
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  const filtered = labels.filter((l) => l.name.toLowerCase().includes(input.toLowerCase()))

  const handleToggle = (name: string) => {
    if (tx.labels.includes(name)) {
      removeLabel({ id: tx.id, label: name })
    } else {
      addLabels({ id: tx.id, labels: [name] })
    }
    setInput('')
  }

  const handleCreate = () => {
    const name = input.trim()
    if (!name) return
    createLabel(
      { name, color: '#6366f1' },
      {
        onSuccess: () => {
          addLabels({ id: tx.id, labels: [name] })
          setInput('')
        },
      },
    )
  }

  const showCreate = input.trim().length > 0 && !labels.some((l) => l.name === input.trim())

  return (
    <div ref={containerRef} className="flex min-w-0 flex-wrap items-center gap-1">
      {tx.labels.map((label) => {
        const meta = labels.find((l) => l.name === label)
        return (
          <LabelChip
            key={label}
            label={label}
            color={meta?.color}
            onRemove={() => removeLabel({ id: tx.id, label })}
          />
        )
      })}

      {open ? (
        <div className="relative">
          <input
            ref={inputRef}
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                setOpen(false)
                setInput('')
              }
              if (e.key === 'Enter' && showCreate) handleCreate()
            }}
            placeholder="label..."
            className="w-24 rounded-sm border border-primary bg-accent px-1.5 py-0.5 text-xs text-foreground focus:outline-none"
          />
          {(filtered.length > 0 || showCreate) && (
            <ul className="absolute left-0 top-full z-50 mt-0.5 max-h-40 min-w-[140px] overflow-y-auto rounded-md border border-border bg-card shadow-lg">
              {filtered.map((l) => (
                <li
                  key={l.name}
                  onMouseDown={() => handleToggle(l.name)}
                  className="flex cursor-pointer items-center gap-2 px-2 py-1.5 text-xs hover:bg-accent"
                >
                  <span
                    className="h-2 w-2 flex-shrink-0 rounded-full"
                    style={{ background: l.color }}
                  />
                  <span className="flex-1">{l.name}</span>
                  {tx.labels.includes(l.name) && <span className="text-success">&#10003;</span>}
                </li>
              ))}
              {showCreate && (
                <li
                  onMouseDown={handleCreate}
                  className={cn(
                    'cursor-pointer px-2 py-1.5 text-xs text-primary hover:bg-accent',
                    filtered.length > 0 && 'border-t border-border',
                  )}
                >
                  + Create &quot;{input.trim()}&quot;
                </li>
              )}
            </ul>
          )}
        </div>
      ) : (
        <button
          onClick={() => setOpen(true)}
          className="rounded-sm border border-border px-1.5 py-0.5 text-[10px] text-muted-foreground transition-colors hover:border-primary hover:text-primary"
          aria-label="Add label"
        >
          +
        </button>
      )}
    </div>
  )
}
