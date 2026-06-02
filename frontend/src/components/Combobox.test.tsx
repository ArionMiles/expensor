import { fireEvent, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useRef, useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { ComboboxListbox, comboboxOptionClass, useComboboxNavigation } from './Combobox'
import { renderWithProviders } from '@/test/render'

function TestCombobox({ onSelect = vi.fn() }: { onSelect?: (value: string) => void }) {
  const [open, setOpen] = useState(false)
  const anchorRef = useRef<HTMLInputElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const options = ['Food', 'Travel', 'Utilities']
  const navigation = useComboboxNavigation({
    open,
    optionCount: options.length,
    onOpenChange: setOpen,
    onSelectIndex: (index) => {
      onSelect(options[index])
      setOpen(false)
    },
  })

  return (
    <div ref={containerRef}>
      <input
        ref={anchorRef}
        aria-label="Category"
        onClick={() => setOpen(true)}
        onFocus={() => setOpen(true)}
        {...navigation.getComboboxProps({ listboxVisible: open })}
      />
      <ComboboxListbox
        open={open}
        anchorRef={anchorRef}
        containerRef={containerRef}
        listboxId={navigation.listboxId}
        label="Category options"
        onOpenChange={setOpen}
      >
        {options.map((option, index) => (
          <li
            key={option}
            {...navigation.getOptionProps(index, { selected: false })}
            className={comboboxOptionClass(index === navigation.highlightedIndex)}
          >
            {option}
          </li>
        ))}
      </ComboboxListbox>
    </div>
  )
}

describe('Combobox primitives', () => {
  it('renders listboxes in a fixed body portal', async () => {
    const user = userEvent.setup()

    renderWithProviders(<TestCombobox />)

    await user.click(screen.getByRole('combobox', { name: 'Category' }))

    const listbox = screen.getByRole('listbox', { name: 'Category options' })
    expect(listbox.parentElement).toBe(document.body)
    expect(listbox).toHaveClass('fixed')
    expect(screen.getByRole('option', { name: 'Food' })).toBeInTheDocument()
  })

  it('selects the highlighted option with keyboard navigation', async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()

    renderWithProviders(<TestCombobox onSelect={onSelect} />)

    await user.click(screen.getByRole('combobox', { name: 'Category' }))
    await user.keyboard('{ArrowDown}{ArrowDown}{Enter}')

    expect(onSelect).toHaveBeenCalledWith('Travel')
    expect(screen.queryByRole('listbox', { name: 'Category options' })).not.toBeInTheDocument()
  })

  it('closes on escape and outside click', async () => {
    const user = userEvent.setup()

    renderWithProviders(<TestCombobox />)

    const input = screen.getByRole('combobox', { name: 'Category' })
    await user.click(input)
    expect(screen.getByRole('listbox', { name: 'Category options' })).toBeInTheDocument()

    await user.keyboard('{Escape}')
    expect(screen.queryByRole('listbox', { name: 'Category options' })).not.toBeInTheDocument()

    await user.click(input)
    expect(screen.getByRole('listbox', { name: 'Category options' })).toBeInTheDocument()

    fireEvent.mouseDown(document.body)
    expect(screen.queryByRole('listbox', { name: 'Category options' })).not.toBeInTheDocument()
  })
})
