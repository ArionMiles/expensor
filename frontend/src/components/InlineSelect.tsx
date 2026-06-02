import { ComboboxListbox, comboboxOptionClass, useComboboxNavigation } from '@/components/Combobox'
import { cn } from '@/lib/utils'
import { useTooltip } from '@/hooks/useTooltip'
import { useRef, useState } from 'react'

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
  const containerRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)
  const { handlers: editTip, tip: editTipEl } = useTooltip()
  const navigation = useComboboxNavigation({
    open,
    optionCount: options.length,
    onOpenChange: setOpen,
    onSelectIndex: (index) => {
      const selected = options[index]
      if (selected) select(selected)
    },
  })
  const highlighted = navigation.highlightedIndex

  const select = (opt: string) => {
    if (opt !== value) onCommit(opt)
    setOpen(false)
    navigation.resetHighlight()
  }

  return (
    <div ref={containerRef} className="relative">
      <button
        ref={buttonRef}
        aria-label={value || placeholder}
        onClick={() => {
          setOpen(!open)
          navigation.resetHighlight()
        }}
        {...navigation.getComboboxProps({ listboxVisible: open })}
        {...editTip('Click to edit')}
        className={cn(
          'text-left text-xs transition-colors hover:text-primary focus:outline-none',
          className ?? 'text-foreground',
        )}
      >
        {value || <span className="opacity-30">{placeholder}</span>}
      </button>
      {editTipEl}

      <ComboboxListbox
        open={open}
        anchorRef={buttonRef}
        containerRef={containerRef}
        listboxId={navigation.listboxId}
        label={`${value || placeholder} options`}
        onOpenChange={setOpen}
      >
        {options.map((opt, i) => (
          <li
            key={opt}
            {...navigation.getOptionProps(i, {
              selected: opt === value,
              onMouseDown: () => select(opt),
            })}
            className={comboboxOptionClass(i === highlighted, opt === value, 'px-3')}
          >
            {opt}
          </li>
        ))}
      </ComboboxListbox>
    </div>
  )
}
