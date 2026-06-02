import { ComboboxListbox, comboboxOptionClass, useComboboxNavigation } from '@/components/Combobox'
import { useEffect, useRef, useState } from 'react'

interface LabelSearchProps {
  value: string
  onChange: (value: string) => void
  options: string[]
}

export function LabelSearch({ value, onChange, options }: LabelSearchProps) {
  const [input, setInput] = useState(value)
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    setInput(value)
  }, [value])

  const filtered =
    input.length > 0 ? options.filter((o) => o.toLowerCase().includes(input.toLowerCase())) : []
  const navigation = useComboboxNavigation({
    open,
    optionCount: filtered.length,
    onOpenChange: setOpen,
    onSelectIndex: (index) => {
      const selected = filtered[index]
      if (selected) select(selected)
    },
    onEnterWithoutSelection: () => {
      onChange(input)
      setOpen(false)
    },
  })
  const highlighted = navigation.highlightedIndex

  const select = (opt: string) => {
    onChange(opt)
    setInput(opt)
    setOpen(false)
    navigation.resetHighlight()
  }

  return (
    <div ref={containerRef} className="relative">
      <div className="relative">
        <input
          ref={inputRef}
          type="text"
          value={input}
          onChange={(e) => {
            setInput(e.target.value)
            setOpen(e.target.value.length > 0)
            navigation.resetHighlight()
          }}
          placeholder="Label"
          aria-label="Filter by label"
          {...navigation.getComboboxProps({ listboxVisible: open && filtered.length > 0 })}
          className="w-full rounded-md border border-border bg-secondary py-1.5 pl-2 pr-6 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
        />
        {input && (
          <button
            onClick={() => {
              onChange('')
              setInput('')
              setOpen(false)
              navigation.resetHighlight()
            }}
            className="absolute right-1.5 top-1/2 -translate-y-1/2 text-sm leading-none text-muted-foreground hover:text-foreground"
            aria-label="Clear label filter"
            tabIndex={-1}
          >
            ×
          </button>
        )}
      </div>

      <ComboboxListbox
        open={open && filtered.length > 0}
        anchorRef={inputRef}
        containerRef={containerRef}
        listboxId={navigation.listboxId}
        label="Label options"
        onOpenChange={setOpen}
      >
        {filtered.map((opt, i) => (
          <li
            key={opt}
            {...navigation.getOptionProps(i, {
              selected: opt === value,
              onMouseDown: () => select(opt),
            })}
            className={comboboxOptionClass(i === highlighted, opt === value)}
          >
            {opt}
          </li>
        ))}
      </ComboboxListbox>
    </div>
  )
}
