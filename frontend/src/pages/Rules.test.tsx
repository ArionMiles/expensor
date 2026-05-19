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
        name: 'Coffee alerts',
        sender_email: 'alerts@example.com',
        subject_contains: 'spent',
        amount_regex: 'amount',
        merchant_regex: 'merchant',
        currency_regex: '',
        source: 'user',
        predefined: false,
      },
      {
        id: 'rule-2',
        name: 'Bank template',
        sender_email: 'bank@example.com',
        subject_contains: 'debited',
        amount_regex: 'amount',
        merchant_regex: 'merchant',
        currency_regex: '',
        source: 'system',
        predefined: true,
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
  it('labels the rules table and row selection controls', () => {
    renderWithProviders(<Rules />, { route: '/rules' })

    expect(screen.getByRole('table', { name: 'Rules' })).toBeInTheDocument()
    expect(screen.getByRole('checkbox', { name: 'Select Coffee alerts' })).toBeInTheDocument()
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

    await user.click(screen.getByRole('link', { name: 'Coffee alerts' }))

    expect(screen.getByTestId('location')).toHaveTextContent('/rules/rule-1')
  })

  it('uses icon-only delete actions and keeps disabled delete icons readable', () => {
    renderWithProviders(<Rules />, { route: '/rules' })

    const customRow = screen.getByRole('row', { name: /Coffee alerts/ })
    expect(within(customRow).getByRole('button', { name: 'Delete Coffee alerts' })).toBeEnabled()

    const predefinedRow = screen.getByRole('row', { name: /Bank template/ })
    const disabledDelete = within(predefinedRow).getByRole('button', {
      name: 'Delete Bank template',
    })
    expect(disabledDelete).toBeDisabled()
    expect(disabledDelete).toHaveClass('disabled:text-destructive/60')
  })
})
