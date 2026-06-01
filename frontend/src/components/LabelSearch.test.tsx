import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { LabelSearch } from './LabelSearch'
import { renderWithProviders } from '@/test/render'

describe('LabelSearch', () => {
  it('renders matching options in a fixed body portal', async () => {
    const user = userEvent.setup()

    renderWithProviders(
      <LabelSearch value="" onChange={vi.fn()} options={['Groceries', 'Gas', 'Travel']} />,
    )

    await user.type(screen.getByRole('textbox', { name: 'Filter by label' }), 'gro')

    const listbox = screen.getByRole('listbox')
    expect(listbox.parentElement).toBe(document.body)
    expect(listbox).toHaveClass('fixed')
    expect(screen.getByRole('option', { name: 'Groceries' })).toBeInTheDocument()
  })
})
