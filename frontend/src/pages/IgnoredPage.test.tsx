import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import IgnoredPage from './IgnoredPage'
import { renderWithProviders } from '@/test/render'

const mockMuteByMerchant = vi.hoisted(() => vi.fn())
const mockMutedMerchants = vi.hoisted(() => ({
  data: [] as {
    id: string
    pattern: string
    reason?: string
    created_at: string
    muted_count: number
  }[],
}))
const mockTransactions = vi.hoisted(() => ({
  data: [] as {
    id: string
    merchant_info: string
    amount: number
    currency: string
    timestamp: string
    mute_reason?: string
  }[],
}))

vi.mock('@/api/queries', () => ({
  useDeleteMutedMerchant: () => ({ mutate: vi.fn(), isPending: false }),
  useFacets: () => ({
    data: {
      sources: [],
      categories: [],
      category_counts: {},
      currencies: [],
      merchants: ['Corner Coffee'],
      labels: [],
      label_counts: {},
      buckets: [],
      bucket_counts: {},
    },
  }),
  useIgnoreByMerchant: () => ({ mutate: mockMuteByMerchant, isPending: false }),
  useIgnoredMerchants: () => ({ data: mockMutedMerchants.data, isLoading: false }),
  useIgnoreTransaction: () => ({ mutate: vi.fn(), isPending: false }),
  useTransactions: () => ({
    data: {
      transactions: mockTransactions.data,
      total: mockTransactions.data.length,
      total_amount: 0,
      base_currency: 'INR',
    },
    isLoading: false,
  }),
  useTimeFormat: () => ({ data: '12h' }),
  useTimezone: () => ({ data: 'Asia/Kolkata' }),
  useUpdateMerchantReason: () => ({ mutate: vi.fn(), isPending: false }),
  useUpdateIgnoreReason: () => ({ mutate: vi.fn(), isPending: false }),
}))

describe('IgnoredPage', () => {
  beforeEach(() => {
    mockMutedMerchants.data = []
    mockTransactions.data = []
    mockMuteByMerchant.mockClear()
  })

  it('renders merchant-wide as the first default tab', () => {
    renderWithProviders(<IgnoredPage />, { route: '/ignored' })

    expect(screen.getByRole('heading', { name: 'Ignored' })).toBeInTheDocument()
    expect(
      screen.getByText('Transactions matching these patterns are excluded from totals and charts.'),
    ).toBeInTheDocument()

    const tabs = screen.getAllByRole('button', { name: /Merchant-wide|Individual transactions/ })
    expect(tabs.map((tab) => tab.textContent)).toEqual(['Merchant-wide', 'Individual transactions'])
    expect(screen.getByRole('button', { name: 'Merchant-wide' })).toHaveClass('border-primary')
    expect(screen.getByText('No merchant-wide ignore patterns yet.')).toBeInTheDocument()
  })

  it('suggests transaction merchants while adding a merchant-wide pattern', async () => {
    const user = userEvent.setup()

    renderWithProviders(<IgnoredPage />, { route: '/ignored?tab=merchant' })

    await user.type(screen.getByRole('textbox'), 'corner')

    expect(await screen.findByRole('button', { name: 'Corner Coffee' })).toBeInTheDocument()
  })

  it('orders individual transaction columns by date, merchant, amount, reason, and action', () => {
    mockTransactions.data = [
      {
        id: 'tx-1',
        merchant_info: 'Corner Coffee',
        amount: 120.5,
        currency: 'INR',
        timestamp: '2026-05-17T08:30:00Z',
        mute_reason: 'duplicate',
      },
    ]

    renderWithProviders(<IgnoredPage />, { route: '/ignored?tab=individual' })

    expect(screen.getAllByRole('columnheader').map((header) => header.textContent)).toEqual([
      'Date',
      'Merchant',
      'Amount',
      'Reason',
      '',
    ])

    const row = screen.getByRole('row', { name: /Corner Coffee/ })
    const cells = within(row).getAllByRole('cell')
    expect(cells[0]).toHaveTextContent('2026')
    expect(cells[1]).toHaveTextContent('Corner Coffee')
    expect(cells[2]).toHaveTextContent('120.5 INR')
    expect(cells[3]).toHaveTextContent('duplicate')
    expect(within(cells[4]).getByRole('button', { name: /Restore/ })).toBeInTheDocument()
  })
})
