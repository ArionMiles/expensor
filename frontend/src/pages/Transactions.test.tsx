import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { useLocation } from 'react-router-dom'
import { Transactions } from './Transactions'
import { renderWithProviders } from '@/test/render'

const transactionFixtures = vi.hoisted(() => {
  const coffee = {
    id: 'tx-1',
    message_id: 'msg-1',
    amount: 1250,
    currency: 'USD',
    timestamp: '2026-04-10T12:30:00Z',
    merchant_info: 'Corner Coffee',
    category: 'Food',
    bucket: 'Needs',
    source: 'gmail',
    description: 'Coffee beans',
    labels: ['Groceries'],
    muted: false,
    muted_by_merchant: false,
    created_at: '2026-04-10T12:30:00Z',
    updated_at: '2026-04-10T12:30:00Z',
  }

  const rent = {
    id: 'tx-2',
    message_id: 'msg-2',
    amount: 240000,
    currency: 'USD',
    timestamp: '2026-04-01T08:00:00Z',
    merchant_info: 'City Apartments',
    category: 'Housing',
    bucket: 'Needs',
    source: 'thunderbird',
    description: 'April rent',
    labels: ['Rent'],
    muted: false,
    muted_by_merchant: false,
    created_at: '2026-04-01T08:00:00Z',
    updated_at: '2026-04-01T08:00:00Z',
  }

  return {
    allTransactions: [coffee, rent],
    foodTransactions: [coffee],
    housingTransactions: [rent],
    gmailTransactions: [coffee],
    searchCoffeeTransactions: [coffee],
    searchRentTransactions: [rent],
  }
})

vi.mock('@/api/queries', () => ({
  useBanks: () => ({ data: [], isLoading: false }),
  useBulkIgnoreMerchants: () => ({ mutate: vi.fn(), isPending: false }),
  useBulkIgnoreTransactions: () => ({ mutate: vi.fn(), isPending: false }),
  useBuckets: () => ({
    data: [{ name: 'Needs', description: 'Essential spending', is_default: false }],
    isLoading: false,
  }),
  useCategorizeMerchant: () => ({ mutate: vi.fn(), isPending: false }),
  useCategories: () => ({
    data: [
      { name: 'Food', description: 'Food spending', is_default: false },
      { name: 'Housing', description: 'Housing spending', is_default: false },
    ],
    isLoading: false,
  }),
  useFacets: () => ({
    data: {
      sources: ['gmail', 'thunderbird'],
      categories: ['Food', 'Housing'],
      currencies: ['USD'],
      labels: ['Groceries', 'Rent'],
      buckets: ['Needs'],
    },
    isLoading: false,
  }),
  useIgnoreByMerchant: () => ({ mutate: vi.fn(), isPending: false }),
  useIgnoreTransaction: () => ({ mutate: vi.fn(), isPending: false }),
  useTransactions: (
    filters: {
      category?: string
      source?: string
    },
    searchQuery: string,
  ) => {
    const normalizedQuery = searchQuery.toLowerCase()
    const transactions =
      normalizedQuery === 'coffee'
        ? transactionFixtures.searchCoffeeTransactions
        : normalizedQuery === 'rent'
          ? transactionFixtures.searchRentTransactions
          : filters.category === 'Housing'
            ? transactionFixtures.housingTransactions
            : filters.category === 'Food'
              ? transactionFixtures.foodTransactions
              : filters.source === 'gmail'
                ? transactionFixtures.gmailTransactions
                : transactionFixtures.allTransactions

    const totalAmount = transactions.reduce((sum, transaction) => sum + transaction.amount, 0)

    return {
      data: {
        transactions,
        total: transactions.length,
        total_amount: totalAmount,
        base_currency: 'USD',
      },
      isLoading: false,
      isFetching: false,
      error: null,
    }
  },
  useUpdateTransactionDescription: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateTransactionFields: () => ({ mutate: vi.fn(), isPending: false }),
  useTimeFormat: () => ({ data: 'HH:mm', isLoading: false }),
  useTimezone: () => ({ data: 'UTC', isLoading: false }),
}))

vi.mock('@/components/DateRangePicker', () => ({
  DateRangePicker: ({
    value,
    onChange,
  }: {
    value: { from?: Date; to?: Date }
    onChange: (range: { from?: Date; to?: Date }) => void
  }) => (
    <button type="button" aria-label="Select date range" onClick={() => onChange(value)}>
      Date range
    </button>
  ),
}))

vi.mock('@/components/FilterCombobox', () => ({
  FilterCombobox: ({
    value,
    onChange,
    placeholder,
    label,
  }: {
    value: string
    onChange: (value: string) => void
    placeholder: string
    label: string
  }) => (
    <input
      aria-label={label}
      placeholder={placeholder}
      value={value}
      onChange={(event) => onChange(event.target.value)}
    />
  ),
}))

vi.mock('@/components/LabelSearch', () => ({
  LabelSearch: ({ value, onChange }: { value: string; onChange: (value: string) => void }) => (
    <input
      aria-label="Filter by label"
      placeholder="Label"
      value={value}
      onChange={(event) => onChange(event.target.value)}
    />
  ),
}))

vi.mock('@/components/Pagination', () => ({
  Pagination: () => null,
}))

vi.mock('@/components/InlineSelect', () => ({
  InlineSelect: ({ value }: { value: string }) => <span>{value}</span>,
}))

vi.mock('@/components/LabelCombobox', () => ({
  LabelCombobox: () => <span>Labels</span>,
}))

function LocationProbe() {
  const location = useLocation()
  return <output data-testid="location">{`${location.pathname}${location.search}`}</output>
}

function renderTransactions(route = '/transactions') {
  return renderWithProviders(
    <>
      <Transactions />
      <LocationProbe />
    </>,
    { route },
  )
}

describe('Transactions', () => {
  it('renders mocked transactions and facets', async () => {
    renderTransactions('/transactions')

    expect(await screen.findByText('Corner Coffee')).toBeInTheDocument()
    expect(screen.getByText('City Apartments')).toBeInTheDocument()
    expect(screen.getByText('2 transactions')).toBeInTheDocument()
    const search = screen.getByRole('searchbox', { name: 'Search transactions' })
    expect(search).toHaveAttribute('type', 'search')
    expect(search).toHaveClass('no-native-search-clear')
    expect(screen.getByRole('table', { name: 'Transactions' })).toBeInTheDocument()
    expect(screen.getAllByRole('checkbox', { name: /select transaction/i }).length).toBeGreaterThan(
      0,
    )
  })

  it('keeps the transaction table on fixed non-overlapping columns', async () => {
    renderTransactions('/transactions')

    const table = await screen.findByRole('table', { name: 'Transactions' })

    expect(table).toHaveClass('table-fixed')
    expect(table).toHaveClass('min-w-[96rem]')
    expect(table.querySelector('colgroup col:nth-child(2)')).toHaveClass('w-52')
    expect(table.querySelector('colgroup col:nth-child(3)')).toHaveClass('w-72')
  })

  it('round-trips search and source filter state through URL query params', async () => {
    const user = userEvent.setup()

    renderTransactions('/transactions')

    await user.type(screen.getByRole('searchbox', { name: 'Search transactions' }), 'coffee')

    await waitFor(() => {
      expect(screen.getByTestId('location').textContent).toContain('q=coffee')
    })

    await user.click(screen.getByRole('button', { name: 'Filters' }))
    fireEvent.change(screen.getByRole('textbox', { name: 'Filter by source' }), {
      target: { value: 'gmail' },
    })

    await waitFor(() => {
      expect(screen.getByTestId('location').textContent).toContain('q=coffee')
      expect(screen.getByTestId('location').textContent).toContain('source=gmail')
    })
  })

  it('restores the initial URL state into the filter UI', async () => {
    renderTransactions('/transactions?q=rent&category=Housing')

    expect(await screen.findByDisplayValue('rent')).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Filter by category' })).toHaveValue('Housing')
    expect(screen.getByText('City Apartments')).toBeInTheDocument()
  })

  it('highlights matching transaction text safely', async () => {
    renderTransactions('/transactions?q=coffee')

    const matches = await screen.findAllByText('Coffee', { selector: 'mark' })

    expect(matches).toHaveLength(2)
    matches.forEach((match) => expect(match).toHaveClass('bg-primary/20'))
  })

  it('shows selected transaction sum when rows are selected', async () => {
    const user = userEvent.setup()

    renderTransactions('/transactions')

    await user.click(await screen.findByLabelText(/select transaction corner coffee/i))

    expect(screen.getByText('1 selected')).toBeInTheDocument()
    expect(screen.getByText(/selected total/i)).toBeInTheDocument()
    expect(screen.queryByText(/total spend/i)).not.toBeInTheDocument()
  })

  it('renders ignore tooltip through the document portal', async () => {
    const user = userEvent.setup()
    const { container } = renderTransactions('/transactions')

    const ignoreButtons = await screen.findAllByRole('button', {
      name: 'Ignore',
    })
    await user.hover(ignoreButtons[0])

    const tooltip = screen.getByText('Ignore')
    expect(tooltip).toBeInTheDocument()
    expect(tooltip.parentElement).toBe(document.body)
    expect(container).not.toContainElement(tooltip)
  })

  it('explains ignored transactions in the ignore confirmation modal', async () => {
    const user = userEvent.setup()

    renderTransactions('/transactions')

    const ignoreButtons = await screen.findAllByRole('button', { name: 'Ignore' })
    await user.click(ignoreButtons[0])

    expect(screen.getByRole('heading', { name: 'Ignore transaction' })).toBeInTheDocument()
    expect(
      screen.getByText('Ignored transactions are excluded from totals and dashboard charts.'),
    ).toBeInTheDocument()
    expect(screen.getByText('Merchant: Corner Coffee')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Ignore merchant' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Ignore this transaction' })).toBeInTheDocument()
  })
})
