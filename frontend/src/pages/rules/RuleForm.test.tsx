import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
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
  useFacets: () => ({ data: { sources: ['Card', 'Existing Source'] } }),
  useRescan: () => ({ mutate: vi.fn() }),
  useRules: () => ({
    data: [
      {
        id: 'rule-1',
        name: 'Existing rule name',
        sender_email: 'existing@example.com',
        subject_contains: 'Existing subject',
        amount_regex: 'Existing amount',
        merchant_regex: 'Existing merchant',
        currency_regex: '',
        transaction_source: 'Existing Source',
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
})
