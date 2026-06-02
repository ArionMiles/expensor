import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { FilterCombobox } from './FilterCombobox'
import { renderWithProviders } from '@/test/render'

describe('FilterCombobox', () => {
  it('exposes stable combobox and listbox semantics', async () => {
    const user = userEvent.setup()

    renderWithProviders(
      <FilterCombobox
        value=""
        onChange={vi.fn()}
        options={['Food', 'Travel', 'Utilities']}
        placeholder="Category"
        label="Category filter"
      />,
    )

    const input = screen.getByRole('combobox', { name: 'Category filter' })
    expect(input).toHaveAttribute('aria-expanded', 'false')
    expect(input).toHaveAttribute('autocomplete', 'off')

    await user.click(input)

    const listbox = screen.getByRole('listbox', { name: 'Category filter options' })
    expect(input).toHaveAttribute('aria-expanded', 'true')
    expect(input).toHaveAttribute('aria-controls', listbox.id)
    expect(listbox.parentElement).toBe(document.body)
    expect(listbox).toHaveClass('fixed')
    expect(screen.getByRole('option', { name: 'Food' })).toBeInTheDocument()
  })

  it('closes the option list on escape', async () => {
    const user = userEvent.setup()

    renderWithProviders(
      <FilterCombobox
        value=""
        onChange={vi.fn()}
        options={['Food', 'Travel', 'Utilities']}
        placeholder="Category"
        label="Category filter"
      />,
    )

    const input = screen.getByRole('combobox', { name: 'Category filter' })
    await user.click(input)
    expect(screen.getByRole('listbox', { name: 'Category filter options' })).toBeInTheDocument()

    await user.keyboard('{Escape}')

    expect(
      screen.queryByRole('listbox', { name: 'Category filter options' }),
    ).not.toBeInTheDocument()
    expect(input).toHaveAttribute('aria-expanded', 'false')
  })

  it('closes the option list when focus moves out', async () => {
    const user = userEvent.setup()

    renderWithProviders(
      <>
        <FilterCombobox
          value=""
          onChange={vi.fn()}
          options={['Food', 'Travel', 'Utilities']}
          placeholder="Category"
          label="Category filter"
        />
        <button type="button">Next field</button>
      </>,
    )

    const input = screen.getByRole('combobox', { name: 'Category filter' })
    await user.click(input)
    expect(screen.getByRole('listbox', { name: 'Category filter options' })).toBeInTheDocument()

    await user.tab()

    expect(
      screen.queryByRole('listbox', { name: 'Category filter options' }),
    ).not.toBeInTheDocument()
  })
})
