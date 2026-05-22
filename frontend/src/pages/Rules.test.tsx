import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { useLocation } from 'react-router-dom'
import Rules from './Rules'
import { renderWithProviders } from '@/test/render'

vi.mock('@/api/queries', () => ({
  useDeleteRule: () => ({ mutate: vi.fn() }),
  useImportRules: () => ({ mutate: vi.fn(), isPending: false }),
  useTimeFormat: () => ({ data: 'HH:mm', isLoading: false }),
  useTimezone: () => ({ data: 'UTC', isLoading: false }),
  useRules: () => ({
    data: [
      {
        id: 'rule-1',
        name: 'HDFC Credit Card',
        sender_emails: ['alerts@hdfcbank.net', 'alerts@hdfcbank.bank.in'],
        subject_contains: 'HDFC Credit Card',
        amount_regex: 'amount',
        merchant_regex: 'merchant',
        currency_regex: '',
        source: { type: 'Credit Card', label: 'HDFC Credit Card', bank: 'HDFC' },
        predefined: true,
      },
      {
        id: 'rule-2',
        name: 'ICICI UPI',
        sender_emails: ['alerts@icicibank.com'],
        subject_contains: 'UPI txn',
        amount_regex: 'amount',
        merchant_regex: 'merchant',
        currency_regex: '',
        source: { type: 'UPI', label: 'ICICI UPI', bank: 'ICICI' },
        predefined: false,
      },
    ],
    isLoading: false,
  }),
}))

function LocationProbe() {
  const location = useLocation()
  return <div data-testid="location">{location.pathname + location.search}</div>
}

describe('Rules', () => {
  it('renders the approved list columns without native selects', () => {
    const { container } = renderWithProviders(<Rules />, { route: '/rules' })

    expect(screen.getByRole('table', { name: 'Rules' })).toBeInTheDocument()
    expect(container.querySelector('select')).not.toBeInTheDocument()

    const headers = screen.getAllByRole('columnheader').map((header) => header.textContent)
    expect(headers).toEqual(['Bank', 'Name', 'Subject', 'Senders', 'Type', 'Origin', ''])
    expect(screen.getByRole('table', { name: 'Rules' })).not.toHaveClass('min-w-[980px]')
  })

  it('labels row selection controls', () => {
    renderWithProviders(<Rules />, { route: '/rules' })

    expect(screen.getByRole('checkbox', { name: 'Select HDFC Credit Card' })).toBeInTheDocument()
  })

  it('opens the rule editor from the rule name without a separate edit column', async () => {
    const user = userEvent.setup()

    renderWithProviders(
      <>
        <Rules />
        <LocationProbe />
      </>,
      { route: '/rules' },
    )

    expect(screen.queryByRole('columnheader', { name: 'Edit' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('link', { name: 'HDFC Credit Card' }))

    expect(screen.getByTestId('location')).toHaveTextContent('/rules/rule-1')
  })

  it('persists type filters in the URL', async () => {
    const user = userEvent.setup()

    renderWithProviders(
      <>
        <Rules />
        <LocationProbe />
      </>,
      { route: '/rules' },
    )

    await user.click(screen.getByRole('button', { name: 'Type: All' }))
    await user.click(screen.getByRole('option', { name: 'Credit Card' }))

    expect(screen.getByTestId('location')).toHaveTextContent('/rules?type=Credit+Card')
    expect(screen.getByRole('row', { name: /HDFC Credit Card/ })).toBeInTheDocument()
    expect(screen.queryByRole('row', { name: /ICICI UPI/ })).not.toBeInTheDocument()
  })

  it('filters rules by search and persists the search in the URL', async () => {
    const user = userEvent.setup()

    renderWithProviders(
      <>
        <Rules />
        <LocationProbe />
      </>,
      { route: '/rules' },
    )

    await user.type(screen.getByRole('searchbox', { name: 'Search rules' }), 'upi')

    expect(screen.getByTestId('location')).toHaveTextContent('/rules?q=upi')
    expect(screen.getByRole('row', { name: /ICICI UPI/ })).toBeInTheDocument()
    expect(screen.queryByRole('row', { name: /HDFC Credit Card/ })).not.toBeInTheDocument()
  })

  it('keeps import and export actions beside the new rule action', () => {
    renderWithProviders(<Rules />, { route: '/rules' })

    const actions = screen.getByLabelText('Rule actions')
    expect(within(actions).getByRole('button', { name: 'Export' })).toBeInTheDocument()
    expect(within(actions).getByRole('button', { name: 'Import' })).toBeInTheDocument()
    expect(within(actions).getByRole('link', { name: '+ New rule' })).toBeInTheDocument()
  })

  it('uses an opaque card surface for filter dropdowns', async () => {
    const user = userEvent.setup()

    renderWithProviders(<Rules />, { route: '/rules' })

    await user.click(screen.getByRole('button', { name: 'Type: All' }))

    expect(screen.getByRole('listbox', { name: 'Type filter options' })).toHaveClass('bg-card')
  })

  it('uses icon-only delete actions and keeps disabled delete icons readable', () => {
    renderWithProviders(<Rules />, { route: '/rules' })

    const customRow = screen.getByRole('row', { name: /ICICI UPI/ })
    expect(within(customRow).getByRole('button', { name: 'Delete ICICI UPI' })).toBeEnabled()

    const predefinedRow = screen.getByRole('row', { name: /HDFC Credit Card/ })
    const disabledDelete = within(predefinedRow).getByRole('button', {
      name: 'Delete HDFC Credit Card',
    })
    expect(disabledDelete).toBeDisabled()
    expect(disabledDelete).toHaveClass('disabled:text-destructive/60')
  })
})
