import { QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { DisplayProvider } from '@/contexts/DisplayContext'
import { I18nProvider } from '@/i18n/I18nProvider'
import { createTestQueryClient } from '@/test/render'
import { saveRuleEmailSearchDraft } from './emailSearchDraft'
import { RuleForm } from './RuleForm'

const queryMocks = vi.hoisted(() => ({
  activeReader: 'gmail',
  createRule: vi.fn(),
  updateRule: vi.fn(),
  rescan: vi.fn(),
}))

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
  useActiveReader: () => ({ data: queryMocks.activeReader }),
  useCreateRule: () => ({ mutate: queryMocks.createRule, isPending: false }),
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
  useRescan: () => ({ mutate: queryMocks.rescan }),
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
  useUpdateRule: () => ({ mutate: queryMocks.updateRule, isPending: false }),
}))

function renderRuleForm(route: string, path: string) {
  const queryClient = createTestQueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      <I18nProvider>
        <DisplayProvider>
          <MemoryRouter initialEntries={[route]}>
            <Routes>
              <Route path={path} element={<RuleForm />} />
            </Routes>
          </MemoryRouter>
        </DisplayProvider>
      </I18nProvider>
    </QueryClientProvider>,
  )
}

describe('RuleForm diagnostics', () => {
  beforeEach(() => {
    queryMocks.activeReader = 'gmail'
    queryMocks.createRule.mockReset()
    queryMocks.updateRule.mockReset()
    queryMocks.rescan.mockReset()
    sessionStorage.clear()
  })

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

  it('loads selected email search results into a new rule workbench', async () => {
    const user = userEvent.setup()
    const draftID = saveRuleEmailSearchDraft({
      subjectQuery: 'Card spend',
      messages: [
        {
          id: 'message-1',
          sender_email: 'alerts@example.com',
          subject: 'Card spend approved',
          body: 'INR 42.00 at Coffee',
        },
        {
          id: 'message-2',
          sender_email: 'alerts-alt@example.com',
          subject: 'Card spend approved',
          body: 'INR 99.00 at Books',
        },
      ],
    })

    renderRuleForm(`/rules/new?draft=${draftID}`, '/rules/new')

    expect(screen.getByLabelText('Subject contains')).toHaveValue('Card spend approved')
    expect(screen.getByRole('button', { name: 'Remove alerts@example.com' })).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Remove alerts-alt@example.com' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Sample 1' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Sample 2' })).toBeInTheDocument()
    expect(screen.getByDisplayValue('INR 42.00 at Coffee')).toBeInTheDocument()

    await user.click(screen.getByRole('tab', { name: 'Sample 2' }))

    expect(screen.getByDisplayValue('INR 99.00 at Books')).toBeInTheDocument()
  })

  it('shows extract labels and expected sample assertions', async () => {
    renderRuleForm('/rules/new', '/rules/new')

    expect(screen.queryByText('Go regexp syntax')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Go-compatible regex help' })).toBeInTheDocument()
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

  it('marks missing live extraction values in red when sample data exists', async () => {
    const user = userEvent.setup()

    renderRuleForm('/rules/new?diagnostic=diag-1', '/rules/new')

    await screen.findByDisplayValue(/Amount: 0/)
    expect(screen.getByText('Needs attention')).toBeInTheDocument()
    expect(
      screen.getAllByText('missing').some((node) => node.classList.contains('text-destructive')),
    ).toBe(true)

    await user.click(screen.getByRole('button', { name: '+ Add sample' }))

    expect(screen.getByRole('tab', { name: 'Sample 2' })).toBeInTheDocument()
    expect(screen.getByText('No sample data yet')).toBeInTheDocument()
  })

  it('does not show missing live results or block saving when the sample is empty', async () => {
    const user = userEvent.setup()

    renderRuleForm('/rules/new', '/rules/new')

    expect(screen.getByText('No sample data yet')).toBeInTheDocument()
    expect(screen.queryByText('missing')).not.toBeInTheDocument()

    await user.type(screen.getByRole('textbox', { name: 'Rule name' }), 'HDFC Credit Card')
    await user.type(screen.getByLabelText('Subject contains'), 'Credit Card')
    await user.type(screen.getByLabelText('Add sender'), 'alerts@hdfcbank.net{Enter}')
    fireEvent.change(screen.getByLabelText('Amount regex'), {
      target: { value: 'Amount: ([0-9.]+)' },
    })
    fireEvent.change(screen.getByLabelText('Merchant regex'), { target: { value: 'at (.*)' } })
    await user.click(screen.getByRole('button', { name: 'Save Rule' }))

    expect(queryMocks.createRule).toHaveBeenCalledTimes(1)
  })

  it('deletes samples from the workbench while keeping one blank sample', async () => {
    const user = userEvent.setup()

    renderRuleForm('/rules/new', '/rules/new')

    await user.click(screen.getByRole('button', { name: '+ Add sample' }))
    expect(screen.getByRole('tab', { name: 'Sample 2' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Delete sample Sample 2' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Remove sample Sample 2' }))

    expect(screen.queryByRole('tab', { name: 'Sample 2' })).not.toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Sample 1' })).toBeInTheDocument()
  })

  it('moves sample guidance hints to Expected before Extract', async () => {
    const user = userEvent.setup()

    renderRuleForm('/rules/new', '/rules/new')

    await user.type(screen.getByLabelText('Sender'), 'alerts@example.com')
    await user.type(screen.getByLabelText('Email body'), 'Amount: 999 paid at SWIGGY')

    expect(screen.getByRole('button', { name: 'Expected values needed' })).toBeInTheDocument()
    expect(screen.getByText('Fill expected amount and merchant')).toBeInTheDocument()
    expect(screen.queryByText(/Fill expected amount and merchant first/)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Extract regex needed' })).not.toBeInTheDocument()

    await user.type(screen.getByLabelText('Expected amount'), '999')
    await user.type(screen.getByLabelText('Expected merchant'), 'SWIGGY')

    expect(screen.queryByRole('button', { name: 'Expected values needed' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Extract regex needed' })).toBeInTheDocument()
  })

  it('validates sample sender email inline', async () => {
    const user = userEvent.setup()

    renderRuleForm('/rules/new', '/rules/new')

    await user.type(screen.getByLabelText('Sender'), 'not-an-email')

    expect(screen.getByText('Enter a valid sender email address.')).toBeInTheDocument()
  })

  it('shows inline rule field errors on save', () => {
    renderRuleForm('/rules/new', '/rules/new')

    fireEvent.change(screen.getByRole('textbox', { name: 'Rule name' }), {
      target: { value: '' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save Rule' }))

    expect(screen.getByText('Rule name is required.')).toBeInTheDocument()
    expect(screen.getByText('Add at least one sender email.')).toBeInTheDocument()
    expect(screen.getByText('Amount regex is required.')).toBeInTheDocument()
    expect(screen.getByText('Merchant regex is required.')).toBeInTheDocument()
    expect(screen.queryByText('Name is required')).not.toBeInTheDocument()
    expect(queryMocks.createRule).not.toHaveBeenCalled()
  })

  it('shows duplicate rule name save failures beside the rule name', async () => {
    const user = userEvent.setup()
    queryMocks.createRule.mockImplementation((_body, options) =>
      options?.onError?.(new Error('rule name already exists')),
    )

    renderRuleForm('/rules/new', '/rules/new')

    const title = screen.getByRole('textbox', { name: 'Rule name' })
    await user.type(title, 'Existing rule name')
    await user.type(screen.getByLabelText('Subject contains'), 'Credit Card')
    await user.type(screen.getByLabelText('Add sender'), 'alerts@hdfcbank.net{Enter}')
    fireEvent.change(screen.getByLabelText('Amount regex'), {
      target: { value: 'Amount: ([0-9.]+)' },
    })
    fireEvent.change(screen.getByLabelText('Merchant regex'), { target: { value: 'at (.*)' } })
    await user.click(screen.getByRole('button', { name: 'Save Rule' }))

    expect(screen.getByText('Rule name already exists.')).toBeInTheDocument()
    expect(title).toHaveClass('border-destructive')
    expect(screen.queryByText('rule name already exists')).not.toBeInTheDocument()
  })

  it('asks how to save existing rules and can start a retroactive scan', async () => {
    const user = userEvent.setup()
    queryMocks.updateRule.mockImplementation((_variables, options) => options?.onSuccess?.())
    queryMocks.rescan.mockImplementation((_reader, options) =>
      options?.onSuccess?.({ status: 'rescanning' }),
    )

    renderRuleForm('/rules/rule-1', '/rules/:id')

    await screen.findByDisplayValue('Existing rule name')
    await user.click(screen.getByRole('button', { name: 'Save Rule' }))

    expect(
      screen.queryByRole('dialog', { name: 'Export contribution files?' }),
    ).not.toBeInTheDocument()
    expect(screen.getByRole('dialog', { name: 'Save rule changes?' })).toBeInTheDocument()
    expect(
      screen.getByText(/Save & Exit updates the rule and returns to the rules list/),
    ).toBeInTheDocument()
    expect(screen.getByText(/Save & Re-scan also re-processes emails/)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Save & Re-scan' }))

    expect(queryMocks.updateRule).toHaveBeenCalledTimes(1)
    expect(queryMocks.rescan).toHaveBeenCalledWith('gmail', expect.any(Object))
  })

  it('offers fixture and rule export before saving when samples contain data', async () => {
    const user = userEvent.setup()
    const anchorClick = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    const realBlob = Blob
    const textDecoder = new TextDecoder()
    const createdBlobs: Array<{ text: string; type: string }> = []
    class TestBlob {
      readonly type: string

      constructor(parts: BlobPart[], options?: BlobPropertyBag) {
        this.type = options?.type ?? ''
        createdBlobs.push({
          text: parts
            .map((part) => (part instanceof Uint8Array ? textDecoder.decode(part) : String(part)))
            .join(''),
          type: this.type,
        })
      }
    }
    const createObjectURL = vi.fn((_blob: Blob) => 'blob:test')
    vi.stubGlobal('Blob', TestBlob)
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: createObjectURL,
    })
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: vi.fn(),
    })

    queryMocks.updateRule.mockImplementation((_variables, options) => options?.onSuccess?.())

    renderRuleForm('/rules/rule-1?diagnostic=diag-1', '/rules/:id')

    await screen.findByDisplayValue(/Amount: 0/)
    await user.click(screen.getByRole('button', { name: 'Save Rule' }))

    expect(screen.getByRole('dialog', { name: 'Export contribution files?' })).toBeInTheDocument()
    expect(screen.getByText(/helping others with similar bank emails/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'See where these files go' })).toHaveAttribute(
      'href',
      expect.stringContaining('.github/CONTRIBUTING.md#adding-bank-support'),
    )
    expect(screen.queryByRole('dialog', { name: 'Save rule changes?' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Export & Continue' }))

    expect(anchorClick).toHaveBeenCalledTimes(1)
    const archiveBlob = createdBlobs.find((blob) => blob.type === 'application/zip')
    const archiveText = archiveBlob?.text ?? ''
    expect(archiveText).toContain('existing-rule-name.rule.json')
    expect(archiveText).toContain('hdfc_credit-card_diagnostic-sample.rule.fixture')
    expect(archiveText).toContain('rule: "Existing rule name"')
    expect(archiveText).toContain('Amount: 0 paid at')
    expect(archiveText).not.toContain('body: |')
    expect(screen.getByRole('dialog', { name: 'Save rule changes?' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Save & Exit' }))
    expect(queryMocks.updateRule).toHaveBeenCalledTimes(1)

    vi.stubGlobal('Blob', realBlob)
    anchorClick.mockRestore()
  })

  it('can save existing rules without re-scanning', async () => {
    const user = userEvent.setup()

    renderRuleForm('/rules/rule-1', '/rules/:id')

    await screen.findByDisplayValue('Existing rule name')
    await user.click(screen.getByRole('button', { name: 'Save Rule' }))
    await user.click(screen.getByRole('button', { name: 'Save & Exit' }))

    expect(queryMocks.updateRule).toHaveBeenCalledTimes(1)
    expect(queryMocks.rescan).not.toHaveBeenCalled()
  })

  it('keeps cancel next to save rule without a standalone fixture export', () => {
    renderRuleForm('/rules/new', '/rules/new')

    const actions = screen.getByLabelText('Rule editor actions')
    const saveButton = within(actions).getByRole('button', { name: 'Save Rule' })
    const cancelLink = within(actions).getByRole('link', { name: 'Cancel' })

    expect(
      within(actions).queryByRole('button', { name: 'Export fixture' }),
    ).not.toBeInTheDocument()
    expect(saveButton.compareDocumentPosition(cancelLink)).toBe(Node.DOCUMENT_POSITION_FOLLOWING)
  })

  it('renders the rule settings as a separate bordered pane', () => {
    renderRuleForm('/rules/new', '/rules/new')

    expect(screen.getByLabelText('Rule settings')).toHaveClass('rounded-xl', 'border', 'bg-card')
    expect(screen.getByLabelText('Sample workbench')).toHaveClass('rounded-xl', 'border', 'bg-card')
  })

  it('uses an opaque card surface for source dropdowns', async () => {
    const user = userEvent.setup()

    renderRuleForm('/rules/new', '/rules/new')

    await user.click(screen.getByLabelText('Type'))

    expect(screen.getByRole('listbox', { name: 'Type options' })).toHaveClass('bg-card')
  })
})
