import { fireEvent, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { InlineSelect } from './InlineSelect'
import { renderWithProviders } from '@/test/render'

describe('InlineSelect', () => {
  it('exposes combobox state and related listbox semantics', async () => {
    const user = userEvent.setup()

    renderWithProviders(
      <InlineSelect value="Food" options={['Food', 'Travel', 'Utilities']} onCommit={vi.fn()} />,
    )

    const trigger = screen.getByRole('combobox', { name: 'Food' })
    expect(trigger).toHaveAttribute('aria-expanded', 'false')

    await user.click(trigger)

    const listbox = screen.getByRole('listbox')
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    expect(trigger).toHaveAttribute('aria-controls', listbox.id)
    expect(screen.getByRole('option', { name: 'Food' })).toHaveAttribute('aria-selected', 'true')
  })

  it('supports keyboard navigation and enter to commit a new option', async () => {
    const user = userEvent.setup()
    const onCommit = vi.fn()

    renderWithProviders(
      <InlineSelect value="Food" options={['Food', 'Travel', 'Utilities']} onCommit={onCommit} />,
    )

    const trigger = screen.getByRole('combobox', { name: 'Food' })
    await user.click(trigger)
    await user.keyboard('{ArrowDown}{ArrowDown}{Enter}')

    expect(onCommit).toHaveBeenCalledWith('Travel')
  })

  it('closes the dropdown on outside click', async () => {
    const user = userEvent.setup()

    renderWithProviders(
      <InlineSelect value="Food" options={['Food', 'Travel', 'Utilities']} onCommit={vi.fn()} />,
    )

    await user.click(screen.getByRole('combobox', { name: 'Food' }))
    expect(screen.getByRole('listbox')).toBeInTheDocument()

    fireEvent.mouseDown(document.body)

    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('does not commit when selecting the current value', async () => {
    const user = userEvent.setup()
    const onCommit = vi.fn()

    renderWithProviders(
      <InlineSelect value="Food" options={['Food', 'Travel', 'Utilities']} onCommit={onCommit} />,
    )

    await user.click(screen.getByRole('combobox', { name: 'Food' }))
    fireEvent.mouseDown(screen.getByRole('option', { name: 'Food' }))

    expect(onCommit).not.toHaveBeenCalled()
  })
})
