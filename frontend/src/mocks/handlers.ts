import { http, HttpResponse } from 'msw'
import type { ExtractionDiagnostic, ExtractionDiagnosticListStatus } from '@/api/types'
import {
  buildAnnualHeatmapData,
  dashboardData,
  heatmapData,
  monthlyBreakdownData,
  seededFacets,
  seededTransactions,
} from './fixtures/transactions'

function sourceLabel(source: { label: string }) {
  return source.label
}

function filterTransactions(url: URL) {
  const q = url.searchParams.get('q')?.toLowerCase() ?? ''
  const category = url.searchParams.get('category')
  const source = url.searchParams.get('source')
  const sourceType = url.searchParams.get('source_type')
  const bank = url.searchParams.get('bank')
  const label = url.searchParams.get('label')
  const dateFrom = url.searchParams.get('date_from')
  const dateTo = url.searchParams.get('date_to')

  return seededTransactions.filter((transaction) => {
    if (q) {
      const haystack = [
        transaction.description,
        transaction.merchant_info,
        transaction.category,
        sourceLabel(transaction.source),
        transaction.source.type,
        transaction.source.bank,
        ...transaction.labels,
      ]
        .join(' ')
        .toLowerCase()

      if (!haystack.includes(q)) {
        return false
      }
    }

    if (category && transaction.category !== category) {
      return false
    }

    if (source && sourceLabel(transaction.source) !== source) {
      return false
    }

    if (sourceType && transaction.source.type !== sourceType) {
      return false
    }

    if (bank && transaction.source.bank !== bank) {
      return false
    }

    if (label && !transaction.labels.includes(label)) {
      return false
    }

    if (dateFrom && new Date(transaction.timestamp) < new Date(dateFrom)) {
      return false
    }

    if (dateTo && new Date(transaction.timestamp) > new Date(dateTo)) {
      return false
    }

    return true
  })
}

const extractionDiagnostics: ExtractionDiagnostic[] = [
  {
    id: 'diag-1',
    status: 'open',
    reader: 'gmail',
    message_id: 'gmail-msg-1',
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
  {
    id: 'diag-2',
    status: 'resolved',
    reader: 'thunderbird',
    message_id: 'thunderbird-msg-1',
    source: 'Bank Account',
    sender: 'Bank Alerts <alerts@example.com>',
    sender_email: 'alerts@example.com',
    subject: 'Transaction Alert',
    email_body: 'You spent at using card',
    received_at: '2026-04-19T09:00:00Z',
    snippet: '',
    rule_id: 'rule-2',
    rule_name: 'Bank Rule',
    amount_regex: 'Rs\\.([\\d.]+)',
    merchant_regex: 'at (.*?) using',
    currency_regex: '',
    failure_reasons: ['merchant_empty'],
    created_at: '2026-04-19T09:01:00Z',
    updated_at: '2026-04-19T09:05:00Z',
    resolved_at: '2026-04-19T09:05:00Z',
  },
]

export const handlers = [
  http.get('/api/health', () => HttpResponse.json({ status: 'ok' })),
  http.get('/api/status', () =>
    HttpResponse.json({ daemon: { running: false }, stats: { base_currency: 'USD' } }),
  ),
  http.get('/api/version', () => HttpResponse.json({ version: 'test' })),
  http.get('/api/stats/dashboard', () => HttpResponse.json(dashboardData)),
  http.get('/api/stats/heatmap', () => HttpResponse.json(heatmapData)),
  http.get('/api/stats/heatmap/annual', ({ request }) => {
    const year = Number(new URL(request.url).searchParams.get('year') ?? '2026')
    return HttpResponse.json(buildAnnualHeatmapData(Number.isFinite(year) ? year : 2026))
  }),
  http.get('/api/stats/labels/monthly', () => HttpResponse.json(monthlyBreakdownData)),
  http.get('/api/config/setup-status', () => HttpResponse.json({ required: false, missing: [] })),
  http.get('/api/config/preferences', () =>
    HttpResponse.json({
      base_currency: 'USD',
      scan_interval: 60,
      lookback_days: 180,
      timezone: 'UTC',
      time_format: 'HH:mm',
    }),
  ),
  http.patch('/api/config/preferences', async () =>
    HttpResponse.json({
      base_currency: 'USD',
      scan_interval: 60,
      lookback_days: 180,
      timezone: 'UTC',
      time_format: 'HH:mm',
    }),
  ),
  http.get('/api/config/active-reader', () => HttpResponse.json({ reader: '' })),
  http.get('/api/config/readers/:reader/checkpoint', () =>
    HttpResponse.json({ last_scan_at: null }),
  ),
  http.get('/api/readers/:reader/status', () =>
    HttpResponse.json({
      credentials_uploaded: true,
      authenticated: true,
      config_present: true,
      auth_type: 'oauth',
      auth_state: 'connected',
      ready: true,
    }),
  ),
  http.delete('/api/config/readers/:reader/checkpoint', () => HttpResponse.json({})),
  http.post('/api/daemon/rescan', () => HttpResponse.json({ status: 'rescanning' })),
  http.get('/api/extraction-diagnostics', ({ request }) => {
    const status =
      (new URL(request.url).searchParams.get('status') as ExtractionDiagnosticListStatus | null) ??
      'open'
    const rows =
      status === 'all'
        ? extractionDiagnostics
        : extractionDiagnostics.filter((diagnostic) => diagnostic.status === status)
    return HttpResponse.json(rows)
  }),
  http.get('/api/extraction-diagnostics/:id', ({ params }) => {
    const diagnostic = extractionDiagnostics.find((item) => item.id === params.id)
    if (!diagnostic) {
      return HttpResponse.json({ error: 'extraction diagnostic not found' }, { status: 404 })
    }
    return HttpResponse.json(diagnostic)
  }),
  http.patch('/api/extraction-diagnostics/:id', async ({ params, request }) => {
    const body = (await request.json()) as { status?: ExtractionDiagnostic['status'] }
    const diagnostic = extractionDiagnostics.find((item) => item.id === params.id)
    if (!diagnostic) {
      return HttpResponse.json({ error: 'extraction diagnostic not found' }, { status: 404 })
    }
    diagnostic.status = body.status ?? diagnostic.status
    diagnostic.updated_at = new Date().toISOString()
    diagnostic.resolved_at = diagnostic.status === 'open' ? null : diagnostic.updated_at
    return HttpResponse.json(diagnostic)
  }),
  http.get('/api/config/sync/status', () =>
    HttpResponse.json({ last_synced_at: null, error: null, entries_updated: 0 }),
  ),
  http.post('/api/config/sync', () => HttpResponse.json({ status: 'queued' })),
  http.get('/api/config/labels', () =>
    HttpResponse.json([
      { name: 'Groceries', color: '#22c55e', created_at: '2026-04-10T12:30:00Z' },
      { name: 'Rent', color: '#3b82f6', created_at: '2026-04-01T08:00:00Z' },
    ]),
  ),
  http.get('/api/config/labels/mappings', () => HttpResponse.json({})),
  http.get('/api/config/labels/export', () => HttpResponse.json([])),
  http.post('/api/config/labels', async ({ request }) => {
    const body = (await request.json()) as { name?: string; color?: string }
    return HttpResponse.json({ name: body.name ?? '', color: body.color ?? '' })
  }),
  http.put('/api/config/labels/:name', async ({ params, request }) => {
    const body = (await request.json()) as { color?: string }
    return HttpResponse.json({ name: String(params.name), color: body.color ?? '' })
  }),
  http.delete('/api/config/labels/:name', () => HttpResponse.json({})),
  http.post('/api/config/labels/:name/apply', () => HttpResponse.json({ applied: 0 })),
  http.delete('/api/config/labels/:name/merchant', () => HttpResponse.json({})),
  http.get('/api/config/categories', () =>
    HttpResponse.json([
      { name: 'Food', description: 'Food spending', is_default: false },
      { name: 'Housing', description: 'Housing spending', is_default: false },
    ]),
  ),
  http.get('/api/config/categories/mappings', () => HttpResponse.json({ Food: ['swiggy'] })),
  http.get('/api/config/categories/export', () => HttpResponse.json([])),
  http.post('/api/config/categories', async ({ request }) => {
    const body = (await request.json()) as { name?: string }
    return HttpResponse.json({ name: body.name ?? '' })
  }),
  http.delete('/api/config/categories/:name', () => HttpResponse.json({})),
  http.post('/api/config/categories/:name/apply', () => HttpResponse.json({ applied: 0 })),
  http.delete('/api/config/categories/:name/merchant', () => HttpResponse.json({})),
  http.get('/api/config/buckets', () =>
    HttpResponse.json([{ name: 'Needs', description: 'Essential spending', is_default: false }]),
  ),
  http.get('/api/config/buckets/mappings', () => HttpResponse.json({ Needs: ['rent'] })),
  http.get('/api/config/buckets/export', () => HttpResponse.json([])),
  http.post('/api/config/buckets', async ({ request }) => {
    const body = (await request.json()) as { name?: string }
    return HttpResponse.json({ name: body.name ?? '' })
  }),
  http.delete('/api/config/buckets/:name', () => HttpResponse.json({})),
  http.post('/api/config/buckets/:name/apply', () => HttpResponse.json({ applied: 0 })),
  http.delete('/api/config/buckets/:name/merchant', () => HttpResponse.json({})),
  http.get('/api/config/banks', () => HttpResponse.json([])),
  http.get('/api/transactions/facets', () => HttpResponse.json(seededFacets)),
  http.get('/api/transactions', ({ request }) => {
    const url = new URL(request.url)
    const transactions = filterTransactions(url)
    const totalAmount = transactions.reduce((sum, transaction) => sum + transaction.amount, 0)

    return HttpResponse.json({
      transactions,
      total: transactions.length,
      total_amount: totalAmount,
      base_currency: 'USD',
    })
  }),
  http.post('/api/transactions/:id/labels', ({ params }) => {
    const transaction = seededTransactions.find((item) => item.id === params.id)
    return HttpResponse.json(transaction ?? {})
  }),
  http.delete('/api/transactions/:id/labels/:label', ({ params }) => {
    const transaction = seededTransactions.find((item) => item.id === params.id)
    return HttpResponse.json(transaction ?? {})
  }),
]
