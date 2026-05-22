import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { DisplayProvider } from '@/contexts/DisplayContext'
import { createTestQueryClient } from '@/test/render'
import { RuleForm } from './RuleForm'

const diagnostic = {
  id: 'diag-1',
  status: 'open' as const,
  reader: 'gmail',
  message_id: 'msg-1',
  source: 'Card',
  sender: 'Bank Alerts <alerts@example.com>',
  sender_email: 'alerts@example.com',
  subject: 'Card alert',
  email_body: 'Amount: 0 paid at',
  received_at: '2026-04-20T10:00:00Z',
  snippet: 'Amount: 0 paid at',
  rule_id: 'rule-1',
  rule_name: 'Card',
  amount_regex: 'Amount: ([\\d.]+)',
  merchant_regex: 'paid at (.*)',
  currency_regex: 'Currency: ([A-Z]{3})',
  failure_reasons: ['amount_zero', 'merchant_empty'],
  created_at: '2026-04-20T10:01:00Z',
  updated_at: '2026-04-20T10:01:00Z',
  resolved_at: null,
}

vi.mock('@/api/queries', () => ({
  useActiveReader: () => ({ data: 'gmail' }),
  useCreateRule: () => ({ mutate: vi.fn(), isPending: false }),
  useExtractionDiagnostic: (id: string | null) => ({
    data: id === 'diag-1' ? diagnostic : undefined,
    isLoading: false,
  }),
  useFacets: () => ({
    data: {
      sources: ['Card', 'Existing Source'],
      source_types: ['Credit Card', 'Debit Card', 'UPI'],
      banks: ['HDFC', 'ICICI'],
      categories: [],
      currencies: [],
      labels: [],
      buckets: [],
    },
  }),
  useRescan: () => ({ mutate: vi.fn() }),
  useRules: () => ({
    data: [
      {
        id: 'rule-1',
        name: 'Existing rule name',
        sender_email: 'existing@example.com',
        sender_emails: ['existing@example.com'],
        subject_contains: 'Existing subject',
        amount_regex: 'Existing amount',
        merchant_regex: 'Existing merchant',
        currency_regex: '',
        transaction_source: 'Existing Source',
        source: { type: 'Credit Card', label: 'Existing Source', bank: 'HDFC' },
        predefined: false,
        created_at: '2026-04-19T10:00:00Z',
        updated_at: '2026-04-19T10:00:00Z',
      },
    ],
    isLoading: false,
  }),
  useTimeFormat: () => ({ data: 'HH:mm', isLoading: false }),
  useTimezone: () => ({ data: 'UTC', isLoading: false }),
  useUpdateRule: () => ({ mutate: vi.fn(), isPending: false }),
}))

function renderRuleForm(route: string, path: string) {
  const queryClient = createTestQueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      <DisplayProvider>
        <MemoryRouter initialEntries={[route]}>
          <Routes>
            <Route path={path} element={<RuleForm />} />
          </Routes>
        </MemoryRouter>
      </DisplayProvider>
    </QueryClientProvider>,
  )
}

describe('RuleForm diagnostics', () => {
  it('loads diagnostic email body into the first test sample', async () => {
    renderRuleForm('/rules/new?diagnostic=diag-1', '/rules/new')

    expect(await screen.findByDisplayValue(/Amount: 0/)).toBeInTheDocument()
    expect(screen.getAllByDisplayValue('Card').length).toBeGreaterThan(0)
  })

  it('keeps existing edit rule fields and loads diagnostic sample', async () => {
    renderRuleForm('/rules/rule-1?diagnostic=diag-1', '/rules/:id')

    expect(await screen.findByDisplayValue('Existing rule name')).toBeInTheDocument()
    expect(screen.getByDisplayValue(/Amount: 0/)).toBeInTheDocument()
  })

  it('reverts a blank rule title on blur', async () => {
    const user = userEvent.setup()

    renderRuleForm('/rules/rule-1', '/rules/:id')

    const title = await screen.findByRole('textbox', { name: 'Rule name' })
    await user.clear(title)
    await user.tab()

    expect(title).toHaveValue('Existing rule name')
  })

  it('adds exact sender emails with Enter and has no add sender button', async () => {
    const user = userEvent.setup()

    renderRuleForm('/rules/new', '/rules/new')

    expect(screen.queryByRole('button', { name: /add sender/i })).not.toBeInTheDocument()

    await user.type(screen.getByLabelText('Add sender'), 'alerts@hdfcbank.net{Enter}')

    expect(screen.getByText('alerts@hdfcbank.net')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Remove alerts@hdfcbank.net' })).toBeInTheDocument()
  })

  it('shows extract labels and expected sample assertions', async () => {
    renderRuleForm('/rules/new', '/rules/new')

    expect(screen.getByText(/Use Go-compatible regular expressions/)).toBeInTheDocument()
    expect(screen.getByLabelText('Amount regex')).toBeInTheDocument()
    expect(screen.getByLabelText('Merchant regex')).toBeInTheDocument()
    expect(screen.getByLabelText('Currency regex')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Expected' })).toBeInTheDocument()
    expect(screen.getByLabelText('Expected amount')).toBeInTheDocument()
    expect(screen.getByLabelText('Expected merchant')).toBeInTheDocument()
    expect(screen.getByLabelText('Expected currency')).toBeInTheDocument()
  })

  it('shows add options only for source type and bank values with no matches', async () => {
    const user = userEvent.setup()

    renderRuleForm('/rules/new', '/rules/new')

    const type = screen.getByLabelText('Type')
    await user.type(type, 'Cre')
    expect(screen.getByRole('option', { name: 'Credit Card' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: /Add "Cre"/ })).not.toBeInTheDocument()

    await user.clear(type)
    await user.type(type, 'Wallet')
    await user.click(screen.getByRole('option', { name: 'Add "Wallet"' }))

    expect(type).toHaveValue('Wallet')
  })

  it('adds samples and marks missing live extraction values in red', async () => {
    const user = userEvent.setup()

    renderRuleForm('/rules/new?diagnostic=diag-1', '/rules/new')

    await user.click(await screen.findByRole('button', { name: '+ Add sample' }))

    expect(screen.getByRole('tab', { name: 'Sample 2' })).toBeInTheDocument()
    expect(screen.getByText('Needs attention')).toBeInTheDocument()
    expect(
      screen.getAllByText('missing').some((node) => node.classList.contains('text-destructive')),
    ).toBe(true)
  })

  it('keeps cancel next to save rule and exposes fixture export', () => {
    renderRuleForm('/rules/new', '/rules/new')

    const actions = screen.getByLabelText('Rule editor actions')
    expect(within(actions).getByRole('link', { name: 'Cancel' })).toBeInTheDocument()
    expect(within(actions).getByRole('button', { name: 'Export fixture' })).toBeInTheDocument()
    expect(within(actions).getByRole('button', { name: 'Save Rule' })).toBeInTheDocument()
  })

  it('uses an opaque card surface for source dropdowns', async () => {
    const user = userEvent.setup()

    renderRuleForm('/rules/new', '/rules/new')

    await user.click(screen.getByLabelText('Type'))

    expect(screen.getByRole('listbox', { name: 'Type options' })).toHaveClass('bg-card')
  })
})
