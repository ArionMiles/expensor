import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { useLocation } from 'react-router-dom'
import ExpenseGroupsPage from './ExpenseGroupsPage'
import { renderWithProviders } from '@/test/render'
import { server } from '@/test/server'

function LocationProbe() {
  const location = useLocation()
  return <div data-testid="location">{location.pathname + location.search}</div>
}

function findExpenseGroupsHeading() {
  return screen.findByRole('heading', { name: 'Expense Groups' }, { timeout: 5000 })
}

describe('ExpenseGroupsPage', () => {
  function visibleGroupNames() {
    return screen
      .getAllByRole('row')
      .slice(1)
      .map((row) => row.querySelector('td')?.textContent?.trim())
  }

  it('uses the tab query param and renders one shared expense groups surface', async () => {
    renderWithProviders(<ExpenseGroupsPage />, { route: '/expense-groups?tab=buckets' })

    expect(await findExpenseGroupsHeading()).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Categories/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Buckets/ })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('button', { name: /Labels/ })).toBeInTheDocument()
    expect((await screen.findAllByText('Needs')).length).toBeGreaterThan(0)
    expect(screen.getByText('rent')).toBeInTheDocument()
  })

  it('does not show transaction cleanup when deleting an unused item', async () => {
    server.use(
      http.get('/api/config/categories', () =>
        HttpResponse.json([{ name: 'Gifts', description: '', is_default: false }]),
      ),
      http.get('/api/config/categories/mappings', () => HttpResponse.json({})),
      http.get('/api/transactions/facets', () =>
        HttpResponse.json({
          sources: [],
          categories: ['Gifts'],
          category_counts: { Gifts: 0 },
          currencies: [],
          labels: [],
          label_counts: {},
          buckets: [],
          bucket_counts: {},
        }),
      ),
    )
    const user = userEvent.setup()

    renderWithProviders(<ExpenseGroupsPage />, { route: '/expense-groups?tab=categories' })

    const row = await screen.findByRole('row', { name: /Gifts/ })
    await user.click(within(row).getByRole('button', { name: 'Delete Gifts' }))

    expect(screen.getByRole('dialog', { name: 'Delete category' })).toBeInTheDocument()
    expect(
      screen.queryByRole('checkbox', { name: 'Remove this category from existing transactions' }),
    ).not.toBeInTheDocument()
  })

  it('keeps disabled delete icons readable for default items', async () => {
    server.use(
      http.get('/api/config/categories', () =>
        HttpResponse.json([{ name: 'Uncategorized', description: '', is_default: true }]),
      ),
      http.get('/api/config/categories/mappings', () => HttpResponse.json({})),
      http.get('/api/transactions/facets', () =>
        HttpResponse.json({
          sources: [],
          categories: ['Uncategorized'],
          category_counts: { Uncategorized: 0 },
          currencies: [],
          labels: [],
          label_counts: {},
          buckets: [],
          bucket_counts: {},
        }),
      ),
    )

    renderWithProviders(<ExpenseGroupsPage />, { route: '/expense-groups?tab=categories' })

    const row = await screen.findByRole('row', { name: /Uncategorized/ })
    const disabledDelete = within(row).getByRole('button', { name: 'Delete Uncategorized' })
    expect(disabledDelete).toBeDisabled()
    expect(disabledDelete).toHaveClass('disabled:text-destructive/60')
  })

  it('offers bulk actions before row-level import conflict choices are changed', async () => {
    const user = userEvent.setup()
    const { container } = renderWithProviders(<ExpenseGroupsPage />, {
      route: '/expense-groups?tab=categories',
    })
    await findExpenseGroupsHeading()

    const file = new File(
      [JSON.stringify([{ name: 'Food', merchants: ['zomato'] }])],
      'groups.json',
      {
        type: 'application/json',
      },
    )
    const input = container.querySelector(
      '[data-testid="expense-groups-import-file"]',
    ) as HTMLInputElement
    fireEvent.change(input, { target: { files: [file] } })

    const dialog = await screen.findByRole('dialog', { name: 'Import conflicts' })
    expect(dialog).toBeInTheDocument()
    expect(within(dialog).getByText('Food')).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Keep' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Overwrite' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Keep All' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Overwrite All' })).toBeInTheDocument()
    expect(within(dialog).queryByRole('button', { name: 'Apply changes' })).not.toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: 'Keep All' }))

    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: 'Import conflicts' })).not.toBeInTheDocument()
    })
  })

  it('switches to apply changes after a row-level import conflict choice changes', async () => {
    const user = userEvent.setup()
    const { container } = renderWithProviders(<ExpenseGroupsPage />, {
      route: '/expense-groups?tab=categories',
    })
    await findExpenseGroupsHeading()

    const file = new File(
      [JSON.stringify([{ name: 'Food', merchants: ['zomato'] }])],
      'groups.json',
      {
        type: 'application/json',
      },
    )
    const input = container.querySelector(
      '[data-testid="expense-groups-import-file"]',
    ) as HTMLInputElement
    fireEvent.change(input, { target: { files: [file] } })

    const dialog = await screen.findByRole('dialog', { name: 'Import conflicts' })

    await user.click(within(dialog).getByRole('button', { name: 'Overwrite' }))

    expect(within(dialog).queryByRole('button', { name: 'Keep All' })).not.toBeInTheDocument()
    expect(within(dialog).queryByRole('button', { name: 'Overwrite All' })).not.toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Apply changes' }))

    await waitFor(() => {
      expect(screen.queryByRole('dialog', { name: 'Import conflicts' })).not.toBeInTheDocument()
    })
  })

  it('opens transactions with the active group filter when a transaction count is clicked', async () => {
    const user = userEvent.setup()
    renderWithProviders(
      <>
        <ExpenseGroupsPage />
        <LocationProbe />
      </>,
      { route: '/expense-groups?tab=buckets' },
    )

    const row = await screen.findByRole('row', { name: /Needs/ })
    await user.click(within(row).getByRole('button', { name: 'View Needs transactions' }))

    expect(screen.getByTestId('location')).toHaveTextContent('/transactions?bucket=Needs')
  })

  it('suggests merchants from transaction facets even when they are not already linked', async () => {
    server.use(
      http.get('/api/transactions/facets', () =>
        HttpResponse.json({
          sources: [],
          categories: ['Food'],
          category_counts: { Food: 1 },
          currencies: [],
          merchants: ['Corner Coffee'],
          labels: [],
          label_counts: {},
          buckets: [],
          bucket_counts: {},
        }),
      ),
      http.get('/api/config/categories/mappings', () => HttpResponse.json({ Food: ['swiggy'] })),
    )
    const user = userEvent.setup()

    renderWithProviders(<ExpenseGroupsPage />, { route: '/expense-groups?tab=categories' })

    await findExpenseGroupsHeading()
    await user.clear(screen.getByRole('textbox', { name: 'Add merchant' }))
    await user.type(screen.getByRole('textbox', { name: 'Add merchant' }), 'corner')

    expect(await screen.findByRole('button', { name: 'Corner Coffee' })).toBeInTheDocument()
  })

  it('sorts transaction counts by toggling the Transactions column', async () => {
    server.use(
      http.get('/api/config/categories', () =>
        HttpResponse.json([
          { name: 'Food', description: '', is_default: false },
          { name: 'Housing', description: '', is_default: false },
          { name: 'Travel', description: '', is_default: false },
        ]),
      ),
      http.get('/api/config/categories/mappings', () => HttpResponse.json({})),
      http.get('/api/transactions/facets', () =>
        HttpResponse.json({
          sources: [],
          categories: ['Food', 'Housing', 'Travel'],
          category_counts: { Food: 2, Housing: 9, Travel: 5 },
          currencies: [],
          labels: [],
          label_counts: {},
          buckets: [],
          bucket_counts: {},
        }),
      ),
    )
    const user = userEvent.setup()

    renderWithProviders(<ExpenseGroupsPage />, { route: '/expense-groups?tab=categories' })
    await findExpenseGroupsHeading()

    await user.click(screen.getByRole('button', { name: /Sort by transactions/i }))
    expect(visibleGroupNames()).toEqual(['Housing', 'Travel', 'Food'])

    await user.click(screen.getByRole('button', { name: /Sort by transactions/i }))
    expect(visibleGroupNames()).toEqual(['Food', 'Travel', 'Housing'])
  })

  it('sorts merchant counts by toggling the Merchants column', async () => {
    server.use(
      http.get('/api/config/labels', () =>
        HttpResponse.json([
          { name: 'Groceries', color: '#22c55e', created_at: '2026-04-10T12:30:00Z' },
          { name: 'Rent', color: '#3b82f6', created_at: '2026-04-01T08:00:00Z' },
          { name: 'Travel', color: '#f59e0b', created_at: '2026-03-01T08:00:00Z' },
        ]),
      ),
      http.get('/api/config/labels/mappings', () =>
        HttpResponse.json({
          Groceries: ['swiggy', 'zomato'],
          Rent: ['landlord'],
          Travel: ['uber', 'irctc', 'air india'],
        }),
      ),
      http.get('/api/transactions/facets', () =>
        HttpResponse.json({
          sources: [],
          categories: [],
          category_counts: {},
          currencies: [],
          labels: ['Groceries', 'Rent', 'Travel'],
          label_counts: { Groceries: 1, Rent: 1, Travel: 1 },
          buckets: [],
          bucket_counts: {},
        }),
      ),
    )
    const user = userEvent.setup()

    renderWithProviders(<ExpenseGroupsPage />, { route: '/expense-groups?tab=labels' })
    await findExpenseGroupsHeading()

    await user.click(screen.getByRole('button', { name: /Sort by merchants/i }))
    expect(visibleGroupNames()).toEqual(['Travel', 'Groceries', 'Rent'])

    await user.click(screen.getByRole('button', { name: /Sort by merchants/i }))
    expect(visibleGroupNames()).toEqual(['Rent', 'Groceries', 'Travel'])
  })
})
