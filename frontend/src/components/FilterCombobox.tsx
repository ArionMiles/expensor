import { ComboboxListbox, comboboxOptionClass, useComboboxNavigation } from '@/components/Combobox'
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
  const containerRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const filtered = options.filter((o) => o.toLowerCase().includes(input.toLowerCase()))
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

  // Keep input in sync with controlled value
  useEffect(() => {
    setInput(value)
  }, [value])

  const select = (opt: string) => {
    onChange(opt)
    setInput(opt)
    setOpen(false)
    navigation.resetHighlight()
  }

  const clear = () => {
    onChange('')
    setInput('')
    setOpen(false)
    navigation.resetHighlight()
    inputRef.current?.focus()
  }

  return (
    <div
      ref={containerRef}
      className="relative"
      onBlur={(event) => {
        if (!containerRef.current?.contains(event.relatedTarget as Node | null)) {
          setOpen(false)
        }
      }}
    >
      <div className="relative">
        <input
          ref={inputRef}
          type="text"
          value={input}
          onChange={(e) => {
            const v = e.target.value
            setInput(v)
            setOpen(true)
            navigation.resetHighlight()
            onChange(v)
          }}
          onFocus={() => setOpen(true)}
          placeholder={placeholder}
          aria-label={label}
          autoComplete="off"
          spellCheck={false}
          aria-autocomplete="list"
          {...navigation.getComboboxProps({ listboxVisible: open && filtered.length > 0 })}
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

      <ComboboxListbox
        open={open && filtered.length > 0}
        anchorRef={inputRef}
        containerRef={containerRef}
        listboxId={navigation.listboxId}
        label={`${label} options`}
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
