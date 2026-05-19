import {
  useAddLabels,
  useApplyLabel,
  useCreateLabel,
  useLabels,
  useRemoveLabel,
} from '@/api/queries'
import type { Transaction } from '@/api/types'
import { LabelChip } from '@/components/LabelChip'
import { SlideNotification } from '@/components/SlideNotification'
import { cn } from '@/lib/utils'
import { useEffect, useId, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

interface LabelComboboxProps {
  tx: Transaction
}

export function LabelCombobox({ tx }: LabelComboboxProps) {
  const [open, setOpen] = useState(false)
  const [input, setInput] = useState('')
  const [highlighted, setHighlighted] = useState(-1)
  const [dropdownPos, setDropdownPos] = useState<{
    top: number
    left: number
    minWidth: number
    maxHeight: number
  } | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const addButtonRef = useRef<HTMLButtonElement>(null)
  const optionRefs = useRef<Record<number, HTMLLIElement | null>>({})
  const escapeClosedRef = useRef(false)
  const listboxId = useId()
  const { data: labels = [] } = useLabels()
  const { mutate: addLabels } = useAddLabels()
  const { mutate: removeLabel } = useRemoveLabel()
  const { mutate: createLabel } = useCreateLabel()
  const { mutate: applyLabel } = useApplyLabel()
  const [propagatePrompt, setPropagatePrompt] = useState<{ label: string } | null>(null)

  useEffect(() => {
    if (!open) {
      escapeClosedRef.current = false
      return
    }
    escapeClosedRef.current = false
    inputRef.current?.focus()
  }, [open])

  useEffect(() => {
    if (!open) return

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      event.preventDefault()
      event.stopPropagation()
      if (escapeClosedRef.current) return
      escapeClosedRef.current = true
      closeInput({ restoreFocus: true })
    }

    window.addEventListener('keydown', handleEscape, true)
    window.addEventListener('keyup', handleEscape, true)
    return () => {
      window.removeEventListener('keydown', handleEscape, true)
      window.removeEventListener('keyup', handleEscape, true)
    }
  }, [open])

  useEffect(() => {
    if (!open) return

    const updateDropdownPos = () => {
      const rect = containerRef.current?.getBoundingClientRect()
      if (!rect) return

      const viewportPadding = 8
      const dropdownGap = 4
      const preferredMaxHeight = 160
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
        top: openUpward
          ? Math.max(viewportPadding, rect.top - maxHeight - dropdownGap)
          : rect.bottom + dropdownGap,
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
      addLabels(
        { id: tx.id, labels: [name] },
        {
          onSuccess: () => {
            const skipKey = `label-propagate-skip:${name}:${tx.merchant_info}`
            if (tx.merchant_info && !localStorage.getItem(skipKey)) {
              setPropagatePrompt({ label: name })
            }
          },
        },
      )
    }
    setOpen(false)
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
  const optionCount = filtered.length + (showCreate ? 1 : 0)
  const activeOptionId =
    highlighted >= 0 && dropdownPos ? `${listboxId}-option-${highlighted}` : undefined

  useEffect(() => {
    if (highlighted < 0 || !open) return
    optionRefs.current[highlighted]?.scrollIntoView({ block: 'nearest' })
  }, [highlighted, open])

  const closeInput = ({ restoreFocus = false } = {}) => {
    setOpen(false)
    setInput('')
    setHighlighted(-1)
    if (restoreFocus) {
      window.setTimeout(() => addButtonRef.current?.focus(), 0)
    }
  }

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
            onChange={(e) => {
              setInput(e.target.value)
              setHighlighted(-1)
            }}
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                e.preventDefault()
                e.stopPropagation()
                closeInput({ restoreFocus: true })
                return
              }
              if (e.key === 'ArrowDown') {
                e.preventDefault()
                setHighlighted((current) => Math.min(current + 1, optionCount - 1))
                return
              }
              if (e.key === 'ArrowUp') {
                e.preventDefault()
                setHighlighted((current) => Math.max(current - 1, 0))
                return
              }
              if (e.key === 'Enter' && highlighted >= 0) {
                e.preventDefault()
                const selected = filtered[highlighted]
                if (selected) handleToggle(selected.name)
                else if (showCreate && highlighted === filtered.length) handleCreate()
                return
              }
              if (e.key === 'Enter' && showCreate) handleCreate()
            }}
            placeholder="label..."
            role="combobox"
            aria-label="Add transaction label"
            autoComplete="off"
            spellCheck={false}
            aria-expanded={Boolean((filtered.length > 0 || showCreate) && dropdownPos)}
            aria-controls={
              (filtered.length > 0 || showCreate) && dropdownPos ? listboxId : undefined
            }
            aria-activedescendant={activeOptionId}
            aria-autocomplete="list"
            className="w-24 rounded-sm border border-primary bg-accent px-1.5 py-0.5 text-xs text-foreground focus:outline-none"
          />
          {(filtered.length > 0 || showCreate) &&
            dropdownPos &&
            createPortal(
              <ul
                id={listboxId}
                role="listbox"
                aria-label="Label options"
                style={{
                  position: 'fixed',
                  top: dropdownPos.top,
                  left: dropdownPos.left,
                  minWidth: dropdownPos.minWidth,
                  maxHeight: dropdownPos.maxHeight,
                }}
                className="z-50 overflow-y-auto rounded-md border border-border bg-card shadow-lg"
              >
                {filtered.map((l, index) => (
                  <li
                    key={l.name}
                    id={`${listboxId}-option-${index}`}
                    ref={(element) => {
                      optionRefs.current[index] = element
                      if (index === highlighted) element?.scrollIntoView({ block: 'nearest' })
                    }}
                    role="option"
                    aria-selected={tx.labels.includes(l.name)}
                    onMouseDown={() => handleToggle(l.name)}
                    onMouseEnter={() => setHighlighted(index)}
                    className={cn(
                      'flex cursor-pointer items-center gap-2 px-2 py-1.5 text-xs hover:bg-accent',
                      index === highlighted && 'bg-accent text-accent-foreground',
                    )}
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
                    id={`${listboxId}-option-${filtered.length}`}
                    ref={(element) => {
                      optionRefs.current[filtered.length] = element
                      if (filtered.length === highlighted) {
                        element?.scrollIntoView({ block: 'nearest' })
                      }
                    }}
                    role="option"
                    aria-selected={false}
                    onMouseDown={handleCreate}
                    onMouseEnter={() => setHighlighted(filtered.length)}
                    className={cn(
                      'cursor-pointer px-2 py-1.5 text-xs text-primary hover:bg-accent',
                      filtered.length > 0 && 'border-t border-border',
                      highlighted === filtered.length && 'bg-accent',
                    )}
                  >
                    + Create &quot;{input.trim()}&quot;
                  </li>
                )}
              </ul>,
              document.body,
            )}
        </div>
      ) : (
        <button
          ref={addButtonRef}
          onClick={() => {
            setOpen(true)
          }}
          className="rounded-sm border border-border px-1.5 py-0.5 text-[10px] text-muted-foreground transition-colors hover:border-primary hover:text-primary"
          aria-label="Add label"
        >
          +
        </button>
      )}

      {propagatePrompt && tx.merchant_info && (
        <SlideNotification
          key={`${propagatePrompt.label}:${tx.merchant_info}`}
          onAction={(applyToAll) => {
            if (applyToAll) {
              applyLabel({ name: propagatePrompt.label, pattern: tx.merchant_info })
            } else {
              localStorage.setItem(
                `label-propagate-skip:${propagatePrompt.label}:${tx.merchant_info}`,
                '1',
              )
            }
            setPropagatePrompt(null)
          }}
          actions={[
            { label: 'Apply to all', value: true, primary: true },
            { label: 'Just this one', value: false },
          ]}
        >
          Apply{' '}
          <span className="font-medium text-foreground">&quot;{propagatePrompt.label}&quot;</span>{' '}
          to all <span className="font-medium text-foreground">{tx.merchant_info}</span>{' '}
          transactions?
        </SlideNotification>
      )}
    </div>
  )
}
