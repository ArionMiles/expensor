import { http, HttpResponse } from 'msw'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { LabelCombobox } from './LabelCombobox'
import { renderWithProviders } from '@/test/render'
import { server } from '@/test/server'
import type { Transaction } from '@/api/types'

const baseTransaction: Transaction = {
  id: 'tx-1',
  message_id: 'message-1',
  amount: 1200,
  currency: 'USD',
  timestamp: '2026-04-14T10:00:00Z',
  merchant_info: '',
  category: 'Food',
  bucket: 'Needs',
  source: 'gmail',
  description: 'Lunch',
  labels: [],
  muted: false,
  muted_by_merchant: false,
  created_at: '2026-04-14T10:00:00Z',
  updated_at: '2026-04-14T10:00:00Z',
}

function stubLabels() {
  let addLabelCalls = 0

  server.use(
    http.get('/api/config/labels', () =>
      HttpResponse.json([
        { name: 'Groceries', color: '#22c55e' },
        { name: 'Gas', color: '#f59e0b' },
      ]),
    ),
    http.post('/api/transactions/:id/labels', async () => {
      addLabelCalls += 1
      return HttpResponse.json({})
    }),
  )

  return {
    getAddLabelCalls: () => addLabelCalls,
  }
}

describe('LabelCombobox', () => {
  it('opens the input from the add button', async () => {
    const user = userEvent.setup()
    stubLabels()

    renderWithProviders(<LabelCombobox tx={baseTransaction} />)

    await user.click(screen.getByRole('button', { name: 'Add label' }))

    expect(screen.getByPlaceholderText('label...')).toHaveAttribute('autocomplete', 'off')
  })

  it('closes the label input on escape', async () => {
    const user = userEvent.setup()
    stubLabels()

    renderWithProviders(<LabelCombobox tx={baseTransaction} />)

    await user.click(screen.getByRole('button', { name: 'Add label' }))
    const input = screen.getByRole('combobox', { name: 'Add transaction label' })
    await user.type(input, 'gro')

    await user.keyboard('{Escape}')

    expect(
      screen.queryByRole('combobox', { name: 'Add transaction label' }),
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add label' })).toHaveFocus()
  })

  it('filters the visible labels as the user types', async () => {
    const user = userEvent.setup()
    stubLabels()

    renderWithProviders(<LabelCombobox tx={baseTransaction} />)

    await user.click(screen.getByRole('button', { name: 'Add label' }))
    await user.type(screen.getByPlaceholderText('label...'), 'gro')

    expect(await screen.findByText('Groceries')).toBeInTheDocument()
    expect(screen.queryByText('Gas')).not.toBeInTheDocument()
  })

  it('selects an existing label through the mutation path', async () => {
    const user = userEvent.setup()
    const state = stubLabels()

    renderWithProviders(<LabelCombobox tx={baseTransaction} />)

    await user.click(screen.getByRole('button', { name: 'Add label' }))
    await user.click(await screen.findByText('Groceries'))

    await waitFor(() => {
      expect(state.getAddLabelCalls()).toBe(1)
    })
  })

  it('selects an existing label by keyboard', async () => {
    const user = userEvent.setup()
    const state = stubLabels()

    renderWithProviders(<LabelCombobox tx={baseTransaction} />)

    await user.click(screen.getByRole('button', { name: 'Add label' }))
    await user.type(screen.getByRole('combobox', { name: 'Add transaction label' }), 'gro')
    await user.keyboard('{ArrowDown}{Enter}')

    await waitFor(() => {
      expect(state.getAddLabelCalls()).toBe(1)
    })
  })

  it('exposes the highlighted option while navigating labels by keyboard', async () => {
    const user = userEvent.setup()
    stubLabels()

    renderWithProviders(<LabelCombobox tx={baseTransaction} />)

    await user.click(screen.getByRole('button', { name: 'Add label' }))
    const combobox = screen.getByRole('combobox', { name: 'Add transaction label' })
    await user.type(combobox, 'gro')
    await user.keyboard('{ArrowDown}')

    const option = await screen.findByRole('option', { name: 'Groceries' })
    expect(combobox).toHaveAttribute('aria-activedescendant', option.id)
  })

  it('scrolls the highlighted option into view while navigating labels by keyboard', async () => {
    const user = userEvent.setup()
    const scrollIntoView = vi.fn()
    HTMLElement.prototype.scrollIntoView = scrollIntoView
    stubLabels()

    renderWithProviders(<LabelCombobox tx={baseTransaction} />)

    await user.click(screen.getByRole('button', { name: 'Add label' }))
    await user.type(screen.getByRole('combobox', { name: 'Add transaction label' }), 'gro')
    await user.keyboard('{ArrowDown}')

    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'nearest' })
  })

  it('shows the create affordance for a new label', async () => {
    const user = userEvent.setup()
    stubLabels()

    renderWithProviders(<LabelCombobox tx={baseTransaction} />)

    await user.click(screen.getByRole('button', { name: 'Add label' }))
    await user.type(screen.getByPlaceholderText('label...'), 'Travel')

    expect(screen.getByText('+ Create "Travel"')).toBeInTheDocument()
  })
})
