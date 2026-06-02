import {
  useAddLabels,
  useApplyLabel,
  useCreateLabel,
  useLabels,
  useRemoveLabel,
} from '@/api/queries'
import type { Transaction } from '@/api/types'
import { ComboboxListbox, comboboxOptionClass, useComboboxNavigation } from '@/components/Combobox'
import { LabelChip } from '@/components/LabelChip'
import { SlideNotification } from '@/components/SlideNotification'
import { useI18n } from '@/i18n/I18nProvider'
import { LABEL_SWATCH_COLORS, cn } from '@/lib/utils'
import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

interface LabelComboboxProps {
  tx: Transaction
  maxVisibleLabels?: number
}

export function LabelCombobox({ tx, maxVisibleLabels = 2 }: LabelComboboxProps) {
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const [overflowOpen, setOverflowOpen] = useState(false)
  const [input, setInput] = useState('')
  const [overflowPos, setOverflowPos] = useState<{ top: number; left: number } | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const addButtonRef = useRef<HTMLButtonElement>(null)
  const overflowButtonRef = useRef<HTMLButtonElement>(null)
  const overflowPortalRef = useRef<HTMLDivElement>(null)
  const optionRefs = useRef<Record<number, HTMLLIElement | null>>({})
  const { data: labels = [] } = useLabels()
  const { mutate: addLabels } = useAddLabels()
  const { mutate: removeLabel } = useRemoveLabel()
  const { mutate: createLabel } = useCreateLabel()
  const { mutate: applyLabel } = useApplyLabel()
  const [propagatePrompt, setPropagatePrompt] = useState<{ label: string } | null>(null)

  useEffect(() => {
    if (!open) return
    inputRef.current?.focus()
  }, [open])

  useEffect(() => {
    if (!overflowOpen) return

    const updateOverflowPos = () => {
      const rect = overflowButtonRef.current?.getBoundingClientRect()
      if (!rect) return

      const viewportPadding = 8
      const width = 224
      setOverflowPos({
        top: rect.bottom + 4,
        left: Math.min(
          Math.max(viewportPadding, rect.left),
          window.innerWidth - width - viewportPadding,
        ),
      })
    }

    updateOverflowPos()
    window.addEventListener('resize', updateOverflowPos)
    window.addEventListener('scroll', updateOverflowPos, true)
    return () => {
      window.removeEventListener('resize', updateOverflowPos)
      window.removeEventListener('scroll', updateOverflowPos, true)
    }
  }, [overflowOpen])

  useEffect(() => {
    if (!overflowOpen) return
    const handler = (event: MouseEvent) => {
      const target = event.target as Node
      if (
        !overflowButtonRef.current?.contains(target) &&
        !overflowPortalRef.current?.contains(target)
      ) {
        setOverflowOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [overflowOpen])

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
    const color = LABEL_SWATCH_COLORS[labels.length % LABEL_SWATCH_COLORS.length]
    createLabel(
      { name, color },
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
  const navigation = useComboboxNavigation({
    open,
    optionCount,
    onOpenChange: setOpen,
    onSelectIndex: (index) => {
      const selected = filtered[index]
      if (selected) handleToggle(selected.name)
      else if (showCreate && index === filtered.length) handleCreate()
    },
    onEnterWithoutSelection: () => {
      if (showCreate) handleCreate()
    },
    onEscape: () => closeInput({ restoreFocus: true }),
  })
  const highlighted = navigation.highlightedIndex
  const visibleLabels = tx.labels.slice(0, maxVisibleLabels)
  const hiddenLabels = tx.labels.slice(visibleLabels.length)
  const hiddenLabelCount = hiddenLabels.length

  useEffect(() => {
    if (highlighted < 0 || !open) return
    optionRefs.current[highlighted]?.scrollIntoView({ block: 'nearest' })
  }, [highlighted, open])

  const closeInput = ({ restoreFocus = false } = {}) => {
    setOpen(false)
    setInput('')
    navigation.resetHighlight()
    if (restoreFocus) {
      window.setTimeout(() => addButtonRef.current?.focus(), 0)
    }
  }

  return (
    <div ref={containerRef} className="flex min-w-0 flex-nowrap items-center gap-1">
      {visibleLabels.map((label) => {
        const meta = labels.find((l) => l.name === label)
        return (
          <LabelChip
            key={label}
            label={label}
            color={meta?.color}
            className="max-w-[7.5rem] overflow-hidden text-ellipsis"
            onRemove={() => removeLabel({ id: tx.id, label })}
          />
        )
      })}

      {hiddenLabelCount > 0 && (
        <>
          <button
            ref={overflowButtonRef}
            type="button"
            onClick={() => setOverflowOpen((current) => !current)}
            className="rounded-sm border border-border px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground transition-colors hover:border-primary hover:text-primary"
            aria-label={t('labels.moreAria', { count: hiddenLabelCount })}
            aria-expanded={overflowOpen}
          >
            +{hiddenLabelCount}
          </button>
          {overflowOpen &&
            overflowPos &&
            createPortal(
              <div
                ref={overflowPortalRef}
                role="dialog"
                aria-label={t('labels.hiddenLabels')}
                className="fixed z-50 w-56 rounded-md border border-border bg-card p-2 shadow-lg"
                style={{ top: overflowPos.top, left: overflowPos.left }}
              >
                <div className="mb-1 px-1 text-[10px] uppercase tracking-wider text-muted-foreground">
                  {t('labels.hiddenLabels')}
                </div>
                <div className="flex max-h-40 flex-wrap gap-1 overflow-y-auto">
                  {hiddenLabels.map((label) => {
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
                </div>
              </div>,
              document.body,
            )}
        </>
      )}

      {open ? (
        <div className="relative">
          <input
            ref={inputRef}
            type="text"
            value={input}
            onChange={(e) => {
              setInput(e.target.value)
              navigation.resetHighlight()
            }}
            placeholder="label..."
            aria-label="Add transaction label"
            autoComplete="off"
            spellCheck={false}
            aria-autocomplete="list"
            {...navigation.getComboboxProps({
              listboxVisible: open && (filtered.length > 0 || showCreate),
            })}
            className="w-24 rounded-sm border border-primary bg-accent px-1.5 py-0.5 text-xs text-foreground focus:outline-none"
          />
          <ComboboxListbox
            open={open && (filtered.length > 0 || showCreate)}
            anchorRef={inputRef}
            containerRef={containerRef}
            listboxId={navigation.listboxId}
            label="Label options"
            maxHeight={160}
            onOpenChange={(nextOpen) => {
              setOpen(nextOpen)
              if (!nextOpen) {
                setInput('')
                navigation.resetHighlight()
              }
            }}
          >
            {filtered.map((l, index) => (
              <li
                key={l.name}
                {...navigation.getOptionProps(index, {
                  selected: tx.labels.includes(l.name),
                  onMouseDown: () => handleToggle(l.name),
                })}
                ref={(element) => {
                  optionRefs.current[index] = element
                  if (index === highlighted) element?.scrollIntoView({ block: 'nearest' })
                }}
                className={cn(
                  comboboxOptionClass(index === highlighted, tx.labels.includes(l.name)),
                  'flex items-center gap-2',
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
                {...navigation.getOptionProps(filtered.length, {
                  selected: false,
                  onMouseDown: handleCreate,
                })}
                ref={(element) => {
                  optionRefs.current[filtered.length] = element
                  if (filtered.length === highlighted) {
                    element?.scrollIntoView({ block: 'nearest' })
                  }
                }}
                className={cn(
                  comboboxOptionClass(highlighted === filtered.length, false),
                  'text-primary',
                  filtered.length > 0 && 'border-t border-border',
                )}
              >
                + Create &quot;{input.trim()}&quot;
              </li>
            )}
          </ComboboxListbox>
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
