import { screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { Diagnostics } from './Diagnostics'
import { renderWithProviders } from '@/test/render'

const updateStatus = vi.fn()

vi.mock('@/api/queries', () => ({
  useExtractionDiagnostics: (status: string) => ({
    data:
      status === 'ignored'
        ? [
            {
              id: 'diag-ignored',
              status: 'ignored',
              reader: 'gmail',
              message_id: 'msg-ignored',
              source: 'Credit Card',
              sender: 'Bank Alerts <alerts@example.com>',
              sender_email: 'alerts@example.com',
              subject: 'Ignored alert',
              email_body: 'Ignored body',
              received_at: '2026-04-20T10:00:00Z',
              snippet: 'Ignored body',
              rule_id: 'rule-2',
              rule_name: 'Ignored Rule',
              amount_regex: 'Rs\\.([\\d.]+)',
              merchant_regex: 'at (.*?) on',
              currency_regex: '',
              failure_reasons: ['merchant_empty'],
              created_at: '2026-04-20T10:01:00Z',
              updated_at: '2026-04-20T10:01:00Z',
              resolved_at: '2026-04-20T10:01:00Z',
            },
          ]
        : [
            {
              id: 'diag-1',
              status: 'open',
              reader: 'gmail',
              message_id: 'msg-1',
              source: 'Credit Card',
              sender: 'Bank Alerts <alerts@example.com>',
              sender_email: 'alerts@example.com',
              subject: 'Card alert',
              email_body: 'Paid at on card',
              received_at: '2026-04-20T10:00:00Z',
              snippet: 'Paid at on card',
              rule_id: 'rule-1',
              rule_name: 'Card Rule',
              amount_regex: 'Rs\\.([\\d.]+)',
              merchant_regex: 'at (.*?) on',
              currency_regex: '',
              failure_reasons: ['amount_zero', 'merchant_empty'],
              created_at: '2026-04-20T10:01:00Z',
              updated_at: '2026-04-20T10:01:00Z',
              resolved_at: null,
            },
          ],
    isLoading: false,
    isFetching: false,
    error: null,
  }),
  useUpdateExtractionDiagnosticStatus: () => ({
    mutate: updateStatus,
    isPending: false,
  }),
  useTimeFormat: () => ({ data: 'HH:mm', isLoading: false }),
  useTimezone: () => ({ data: 'UTC', isLoading: false }),
}))

describe('Diagnostics', () => {
  it('renders diagnostics and persists status filter in the URL', async () => {
    renderWithProviders(<Diagnostics />, { route: '/diagnostics?status=ignored' })

    expect(await screen.findByText('Extraction diagnostics')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Ignored' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('table', { name: 'Extraction diagnostics' })).toBeInTheDocument()
  })

  it('links fix rule to the rule editor with the diagnostic id', async () => {
    renderWithProviders(<Diagnostics />, { route: '/diagnostics' })

    const link = await screen.findByRole('link', { name: /fix rule/i })
    expect(link).toHaveAttribute('href', '/rules/rule-1?diagnostic=diag-1')
  })

  it('uses readable resolved action styling in light and dark mode', async () => {
    renderWithProviders(<Diagnostics />, { route: '/diagnostics' })

    const button = await screen.findByRole('button', { name: /mark resolved/i })
    expect(button).toHaveClass('bg-emerald-50')
    expect(button).toHaveClass('text-emerald-800')
    expect(button).toHaveClass('dark:bg-emerald-500/15')
    expect(button).toHaveClass('dark:text-emerald-100')
    expect(button).not.toHaveClass('text-emerald-200')
  })

  it('uses readable open status styling in light and dark mode', async () => {
    renderWithProviders(<Diagnostics />, { route: '/diagnostics' })

    const status = await screen.findByText('open')
    expect(status).toHaveClass('bg-amber-50')
    expect(status).toHaveClass('text-amber-800')
    expect(status).toHaveClass('dark:bg-amber-500/10')
    expect(status).toHaveClass('dark:text-amber-200')
    expect(status).not.toHaveClass('text-amber-200')
  })

  it('uses the same readable amber for the page heading icon', async () => {
    renderWithProviders(<Diagnostics />, { route: '/diagnostics' })

    expect(await screen.findByText('Extraction diagnostics')).toBeInTheDocument()
    const icon = screen.getByTestId('diagnostics-heading-icon')
    expect(icon).toHaveClass('text-amber-800')
    expect(icon).toHaveClass('dark:text-amber-200')
    expect(icon).not.toHaveClass('text-amber-300')
  })
})
