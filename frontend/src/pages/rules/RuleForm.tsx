import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import {
  useActiveReader,
  useCreateRule,
  useExtractionDiagnostic,
  useFacets,
  useRescan,
  useRules,
  useUpdateRule,
} from '@/api/queries'
import { ConfirmModal } from '@/components/ConfirmModal'

interface RegexResult {
  match: string | null
  invalid: boolean
}

interface FormState {
  name: string
  subjectContains: string
  amountRegex: string
  merchantRegex: string
  currencyRegex: string
  sourceType: string
  bank: string
  senders: string[]
  senderDraft: string
}

interface SampleState {
  name: string
  sender: string
  subject: string
  body: string
  expected: {
    amount: string
    merchant: string
    currency: string
  }
}

interface FieldErrors {
  name?: string
  senders?: string
  amountRegex?: string
  merchantRegex?: string
  sampleSender?: string
}

const emptyForm: FormState = {
  name: '',
  subjectContains: '',
  amountRegex: '',
  merchantRegex: '',
  currencyRegex: '',
  sourceType: '',
  bank: '',
  senders: [],
  senderDraft: '',
}

function testRegex(pattern: string, body: string): RegexResult {
  if (!pattern) return { match: null, invalid: false }
  try {
    const match = new RegExp(pattern).exec(body)
    return { match: match?.[1] ?? null, invalid: false }
  } catch {
    return { match: null, invalid: true }
  }
}

function diagnosticSample(diagnostic: {
  sender_email: string
  subject: string
  email_body: string
}): SampleState {
  return {
    name: 'Diagnostic sample',
    sender: diagnostic.sender_email,
    subject: diagnostic.subject,
    body: diagnostic.email_body,
    expected: {
      amount: '',
      merchant: '',
      currency: '',
    },
  }
}

function blankSample(index: number): SampleState {
  return {
    name: `Sample ${index}`,
    sender: '',
    subject: '',
    body: '',
    expected: {
      amount: '',
      merchant: '',
      currency: '',
    },
  }
}

function sourceLabel(bank: string, sourceType: string) {
  return [bank.trim(), sourceType.trim()].filter(Boolean).join(' ')
}

function uniqueSorted(values: string[]) {
  return [...new Set(values.map((value) => value.trim()).filter(Boolean))].sort((a, b) =>
    a.localeCompare(b),
  )
}

function slug(value: string) {
  return (
    value
      .trim()
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '') || 'rule'
  )
}

function indentBlock(value: string) {
  return value
    .replace(/\r\n/g, '\n')
    .split('\n')
    .map((line) => `  ${line}`)
    .join('\n')
}

function isValidEmail(value: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim())
}

function sampleHasValidationData(sample?: SampleState) {
  if (!sample) return false
  return Boolean(
    sample.sender.trim() ||
    sample.subject.trim() ||
    sample.body.trim() ||
    sample.expected.amount.trim() ||
    sample.expected.merchant.trim() ||
    sample.expected.currency.trim(),
  )
}

function inputClasses(hasError = false, extra = '') {
  return [
    'mt-1 w-full rounded-lg border bg-input px-3 py-2 text-sm text-foreground',
    hasError ? 'border-destructive focus:border-destructive' : 'border-border',
    extra,
  ]
    .filter(Boolean)
    .join(' ')
}

function downloadText(filename: string, text: string, type: string) {
  const blob = new Blob([text], { type })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}

type ComboboxProps = {
  label: string
  value: string
  options: string[]
  customValues: string[]
  onChange: (value: string) => void
  onAdd: (value: string) => void
}

function SourceValueCombobox({
  label,
  value,
  options,
  customValues,
  onChange,
  onAdd,
}: ComboboxProps) {
  const [open, setOpen] = useState(false)
  const [readOnly, setReadOnly] = useState(true)
  const [rect, setRect] = useState<DOMRect | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const allOptions = uniqueSorted([...options, ...customValues])
  const filtered = allOptions.filter((option) => option.toLowerCase().includes(value.toLowerCase()))
  const exactMatch = allOptions.some(
    (option) => option.toLowerCase() === value.trim().toLowerCase(),
  )
  const canAdd = value.trim() !== '' && filtered.length === 0 && !exactMatch

  const openMenu = () => {
    const nextRect = inputRef.current?.getBoundingClientRect()
    if (nextRect) setRect(nextRect)
    setOpen(true)
  }

  const select = (nextValue: string) => {
    onChange(nextValue)
    setOpen(false)
  }

  const add = () => {
    const nextValue = value.trim()
    if (!nextValue) return
    onAdd(nextValue)
    onChange(nextValue)
    setOpen(false)
  }

  const onKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter' && canAdd) {
      event.preventDefault()
      add()
    }
  }

  return (
    <div>
      <label className="mb-1 block text-sm text-muted-foreground" htmlFor={`${label}-input`}>
        {label}
      </label>
      <div className="relative">
        <input
          ref={inputRef}
          id={`${label}-input`}
          value={value}
          onChange={(event) => {
            onChange(event.target.value)
            openMenu()
          }}
          onFocus={() => {
            setReadOnly(false)
            openMenu()
          }}
          onBlur={() => window.setTimeout(() => setOpen(false), 120)}
          onKeyDown={onKeyDown}
          readOnly={readOnly}
          autoComplete="off"
          data-1p-ignore="true"
          data-lpignore="true"
          data-form-type="other"
          className="w-full rounded-lg border border-border bg-input px-3 py-2 pr-8 text-sm text-foreground outline-none transition-colors focus:border-primary"
        />
        <span
          aria-hidden="true"
          className="pointer-events-none absolute right-3 top-1/2 h-2 w-2 -translate-y-1/2 rotate-45 border-b-2 border-r-2 border-muted-foreground"
        />
      </div>
      {open &&
        rect &&
        createPortal(
          <div
            role="listbox"
            aria-label={`${label} options`}
            className="fixed z-50 rounded-lg border border-border bg-card p-1 text-sm text-card-foreground shadow-xl"
            style={{ left: rect.left, top: rect.bottom + 6, width: rect.width }}
          >
            {filtered.map((option) => (
              <button
                key={option}
                type="button"
                role="option"
                aria-selected={value === option}
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => select(option)}
                className="block w-full rounded-md px-3 py-2 text-left text-muted-foreground hover:bg-secondary hover:text-foreground"
              >
                {option}
              </button>
            ))}
            {canAdd && (
              <button
                type="button"
                role="option"
                aria-selected={false}
                onMouseDown={(event) => event.preventDefault()}
                onClick={add}
                className="block w-full rounded-md px-3 py-2 text-left font-medium text-primary hover:bg-secondary"
              >
                Add &quot;{value.trim()}&quot;
              </button>
            )}
          </div>,
          document.body,
        )}
    </div>
  )
}

function ResultValue({ result, optional = false }: { result: RegexResult; optional?: boolean }) {
  if (result.invalid) return <span className="text-destructive">invalid</span>
  if (result.match !== null && result.match.trim() !== '') {
    return <span className="font-mono text-green-500">{result.match}</span>
  }
  if (optional) return <span className="text-muted-foreground">optional</span>
  return <span className="text-destructive">missing</span>
}

export function RuleForm() {
  const { id } = useParams<{ id: string }>()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const isCreate = !id
  const diagnosticID = searchParams.get('diagnostic')

  const { data: rules = [], isLoading: rulesLoading } = useRules()
  const rule = id ? rules.find((candidate) => candidate.id === id) : null
  const { data: diagnostic } = useExtractionDiagnostic(diagnosticID)
  const { data: activeReader = '' } = useActiveReader()
  const { data: facets } = useFacets()

  const [form, setForm] = useState<FormState>(emptyForm)
  const [lastSavedName, setLastSavedName] = useState('New Rule')
  const [samples, setSamples] = useState<SampleState[]>([blankSample(1)])
  const [activeSample, setActiveSample] = useState(0)
  const [customTypes, setCustomTypes] = useState<string[]>([])
  const [customBanks, setCustomBanks] = useState<string[]>([])
  const [toast, setToast] = useState('')
  const [formError, setFormError] = useState('')
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})
  const [saveDialogOpen, setSaveDialogOpen] = useState(false)

  const { mutate: createRule, isPending: creating } = useCreateRule()
  const { mutate: updateRule, isPending: updating } = useUpdateRule()
  const { mutate: triggerRescan } = useRescan()

  useEffect(() => {
    if (!rule) return
    const nextName = rule.name || 'New Rule'
    setLastSavedName(nextName)
    setForm({
      name: nextName,
      subjectContains: rule.subject_contains,
      amountRegex: rule.amount_regex || diagnostic?.amount_regex || '',
      merchantRegex: rule.merchant_regex || diagnostic?.merchant_regex || '',
      currencyRegex: rule.currency_regex || diagnostic?.currency_regex || '',
      sourceType: rule.source?.type || '',
      bank: rule.source?.bank || '',
      senders: rule.sender_emails?.length
        ? rule.sender_emails
        : [rule.sender_email ?? ''].filter(Boolean),
      senderDraft: '',
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rule?.id, diagnostic?.id])

  useEffect(() => {
    if (!diagnostic) return

    setSamples([diagnosticSample(diagnostic)])
    setActiveSample(0)
    if (!isCreate) return

    const nextName = diagnostic.rule_name || 'New Rule'
    setLastSavedName(nextName)
    setForm({
      name: nextName,
      subjectContains: diagnostic.subject,
      amountRegex: diagnostic.amount_regex,
      merchantRegex: diagnostic.merchant_regex,
      currencyRegex: diagnostic.currency_regex,
      sourceType: diagnostic.source,
      bank: '',
      senders: [diagnostic.sender_email].filter(Boolean),
      senderDraft: '',
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [diagnostic?.id, isCreate])

  const updateForm = (patch: Partial<FormState>) => setForm((current) => ({ ...current, ...patch }))

  const addSender = () => {
    const sender = form.senderDraft.trim().toLowerCase()
    if (!sender || form.senders.includes(sender)) {
      updateForm({ senderDraft: '' })
      return
    }
    updateForm({ senders: [...form.senders, sender], senderDraft: '' })
  }

  const removeSender = (sender: string) =>
    updateForm({ senders: form.senders.filter((candidate) => candidate !== sender) })

  const updateSample = (patch: Partial<SampleState>) =>
    setSamples((current) =>
      current.map((sample, index) => (index === activeSample ? { ...sample, ...patch } : sample)),
    )

  const updateExpected = (patch: Partial<SampleState['expected']>) =>
    setSamples((current) =>
      current.map((sample, index) =>
        index === activeSample ? { ...sample, expected: { ...sample.expected, ...patch } } : sample,
      ),
    )

  const addSample = () => {
    setSamples((current) => {
      const next = [...current, blankSample(current.length + 1)]
      setActiveSample(next.length - 1)
      return next
    })
  }

  const deleteSample = () => {
    setSamples((current) => {
      if (current.length === 1) {
        setActiveSample(0)
        return [blankSample(1)]
      }
      const next = current.filter((_, index) => index !== activeSample)
      setActiveSample(Math.max(0, Math.min(activeSample, next.length - 1)))
      return next
    })
  }

  const exportFixture = () => {
    const sample = selectedSample
    const bankSlug = slug(form.bank || 'bank')
    const typeSlug = slug(form.sourceType || 'source-type')
    const caseSlug = slug(sample.name || form.name || 'sample')
    const body = `rule: ${form.name || 'New Rule'}
sender: ${sample.sender}
subject: "${sample.subject.replace(/"/g, '\\"')}"
body: |
${indentBlock(sample.body || '')}
expected:
  amount: ${sample.expected.amount || '0.00'}
  merchant: ${sample.expected.merchant || ''}
  currency: ${sample.expected.currency || ''}
`
    downloadText(`${bankSlug}_${typeSlug}_${caseSlug}.yaml`, body, 'text/yaml')
  }

  const selectedSample = samples[activeSample] ?? samples[0]
  const selectedSampleHasData = sampleHasValidationData(selectedSample)
  const selectedSampleSenderInvalid =
    selectedSample?.sender.trim() !== '' && !isValidEmail(selectedSample.sender)
  const selectedSampleHasSenderAndBody = Boolean(
    selectedSample?.sender.trim() && selectedSample?.body.trim(),
  )
  const selectedSampleExpectedMissing =
    selectedSampleHasSenderAndBody &&
    (!selectedSample.expected.amount.trim() || !selectedSample.expected.merchant.trim())
  const selectedSampleExtractMissing =
    selectedSampleHasSenderAndBody &&
    !selectedSampleExpectedMissing &&
    (!form.amountRegex.trim() || !form.merchantRegex.trim())
  const live = useMemo(
    () => ({
      amount: testRegex(form.amountRegex, selectedSample?.body ?? ''),
      merchant: testRegex(form.merchantRegex, selectedSample?.body ?? ''),
      currency: testRegex(form.currencyRegex, selectedSample?.body ?? ''),
    }),
    [form.amountRegex, form.currencyRegex, form.merchantRegex, selectedSample?.body],
  )
  const needsAttention =
    selectedSampleHasData &&
    (selectedSampleSenderInvalid ||
      live.amount.invalid ||
      live.merchant.invalid ||
      (selectedSample.body.trim() !== '' &&
        (!live.amount.match ||
          live.amount.match.trim() === '' ||
          !live.merchant.match ||
          live.merchant.match.trim() === '')) ||
      (selectedSample.expected.amount.trim() !== '' &&
        live.amount.match !== selectedSample.expected.amount.trim()) ||
      (selectedSample.expected.merchant.trim() !== '' &&
        live.merchant.match !== selectedSample.expected.merchant.trim()) ||
      (selectedSample.expected.currency.trim() !== '' &&
        live.currency.match !== selectedSample.expected.currency.trim()))

  const validateForm = () => {
    setFormError('')
    const errors: FieldErrors = {}
    const name = form.name.trim()
    if (!name) {
      errors.name = 'Rule name is required.'
    }
    if (form.senders.length === 0) {
      errors.senders = 'Add at least one sender email.'
    }
    if (!form.amountRegex.trim()) {
      errors.amountRegex = 'Amount regex is required.'
    }
    if (!form.merchantRegex.trim()) {
      errors.merchantRegex = 'Merchant regex is required.'
    }
    if (selectedSampleSenderInvalid) {
      errors.sampleSender = 'Enter a valid sender email address.'
    }
    setFieldErrors(errors)
    return { valid: Object.keys(errors).length === 0, name }
  }

  const saveRule = (shouldRescan: boolean) => {
    const name = form.name.trim()

    const body = {
      name,
      sender_emails: form.senders,
      subject_contains: form.subjectContains,
      amount_regex: form.amountRegex,
      merchant_regex: form.merchantRegex,
      currency_regex: form.currencyRegex,
      source: {
        type: form.sourceType,
        bank: form.bank,
        label: sourceLabel(form.bank, form.sourceType),
      },
    }

    if (isCreate) {
      createRule(body, {
        onSuccess: () => navigate('/rules'),
        onError: (error) => setFormError(error.message),
      })
      return
    }

    updateRule(
      { id: id!, body },
      {
        onSuccess: () => {
          if (!shouldRescan || !activeReader) {
            navigate('/rules')
            return
          }
          triggerRescan(activeReader, {
            onSuccess: (data) => {
              const msg =
                data.status === 'rescanning'
                  ? 'Rule saved. Retroactive scan started.'
                  : 'Rule saved. Retroactive scan queued — will run on the next daemon start.'
              setToast(msg)
              setTimeout(() => navigate('/rules'), 2500)
            },
            onError: () => navigate('/rules'),
          })
        },
        onError: (error) => setFormError(error.message),
      },
    )
  }

  const handleSubmit = () => {
    const result = validateForm()
    if (!result.valid) return

    if (!isCreate) {
      setSaveDialogOpen(true)
      return
    }

    saveRule(false)
  }

  if (!isCreate && rulesLoading) {
    return (
      <div className="mx-auto w-full max-w-6xl px-6 py-6">
        <p className="text-xs text-muted-foreground">Loading...</p>
      </div>
    )
  }

  if (!isCreate && !rule) {
    return (
      <div className="mx-auto w-full max-w-6xl px-6 py-6">
        <p className="text-sm text-destructive">Rule not found.</p>
        <Link to="/rules" className="text-xs text-primary hover:underline">
          Back to rules
        </Link>
      </div>
    )
  }

  const isPending = creating || updating

  return (
    <div className="mx-auto w-full max-w-7xl space-y-4 px-6 py-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <nav className="flex items-center gap-2 text-sm text-muted-foreground">
            <Link to="/rules" className="hover:text-foreground">
              Rules
            </Link>
            <span aria-hidden="true">›</span>
            <span className="text-foreground">{isCreate ? 'New Rule' : 'Edit Rule'}</span>
          </nav>
          <input
            aria-label="Rule name"
            value={form.name}
            onChange={(event) => updateForm({ name: event.target.value })}
            onBlur={() => {
              if (form.name.trim() === '') updateForm({ name: lastSavedName })
            }}
            className={`mt-2 max-w-[46rem] border-0 border-b bg-transparent px-0 py-1 text-3xl font-semibold tracking-tight text-foreground outline-none transition-colors hover:border-border ${
              fieldErrors.name
                ? 'border-destructive focus:border-destructive'
                : 'border-transparent focus:border-primary'
            }`}
          />
          {fieldErrors.name && <p className="mt-1 text-xs text-destructive">{fieldErrors.name}</p>}
          <p className="mt-1 text-sm text-muted-foreground">
            Edit the rule once, switch samples freely, and watch match status update inline.
          </p>
        </div>
        <div aria-label="Rule editor actions" className="flex flex-wrap items-center gap-2">
          {rule?.predefined && (
            <span className="rounded-full border border-primary/40 px-3 py-1 text-xs font-medium text-primary">
              Predefined
            </span>
          )}
          <button
            type="button"
            onClick={exportFixture}
            className="rounded-lg border border-border px-4 py-2 text-sm font-semibold text-muted-foreground hover:text-foreground"
          >
            Export fixture
          </button>
          <button
            type="button"
            onClick={handleSubmit}
            disabled={isPending}
            className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
          >
            {isPending ? 'Saving...' : 'Save Rule'}
          </button>
          <Link
            to="/rules"
            className="inline-flex rounded-lg border border-border px-4 py-2 text-sm font-semibold text-muted-foreground hover:text-foreground"
          >
            Cancel
          </Link>
        </div>
      </div>

      {toast && (
        <div className="rounded-lg border border-border bg-secondary px-4 py-3 text-sm text-foreground">
          {toast}
        </div>
      )}

      <div className="grid min-h-[32rem] grid-cols-1 gap-4 lg:grid-cols-[24rem_minmax(0,1fr)]">
        <aside
          aria-label="Rule settings"
          className="space-y-4 rounded-xl border border-border bg-card p-4"
        >
          <section className="space-y-2 border-b border-border pb-4">
            <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Subject Contains
            </h2>
            <input
              aria-label="Subject contains"
              value={form.subjectContains}
              onChange={(event) => updateForm({ subjectContains: event.target.value })}
              className="w-full rounded-lg border border-border bg-input px-3 py-2 text-sm text-foreground"
            />
          </section>

          <section className="space-y-2 border-b border-border pb-4">
            <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Sender
            </h2>
            <div className="flex flex-wrap gap-2">
              {form.senders.map((sender) => (
                <span
                  key={sender}
                  className="inline-flex items-center gap-2 rounded-full border border-border px-3 py-1.5 font-mono text-xs text-muted-foreground"
                >
                  {sender}
                  <button
                    type="button"
                    onClick={() => removeSender(sender)}
                    aria-label={`Remove ${sender}`}
                    className="text-sm leading-none text-muted-foreground hover:text-foreground"
                  >
                    x
                  </button>
                </span>
              ))}
            </div>
            {fieldErrors.senders && (
              <p className="text-xs text-destructive">{fieldErrors.senders}</p>
            )}
            <input
              aria-label="Add sender"
              value={form.senderDraft}
              onChange={(event) => updateForm({ senderDraft: event.target.value })}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault()
                  addSender()
                }
              }}
              placeholder="alerts@example.com"
              className={`w-full rounded-lg border bg-input px-3 py-2 font-mono text-sm text-foreground ${
                fieldErrors.senders ? 'border-destructive' : 'border-border'
              }`}
            />
          </section>

          <section className="space-y-2 border-b border-border pb-4">
            <div className="flex items-center justify-between gap-2">
              <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Extract
              </h2>
              <div className="group relative">
                <button
                  type="button"
                  aria-label="Go-compatible regex help"
                  className="flex h-6 w-6 items-center justify-center rounded-full border border-border text-xs font-semibold text-muted-foreground hover:text-foreground"
                >
                  ?
                </button>
                <div className="bg-popover text-popover-foreground pointer-events-none absolute right-0 top-8 z-10 hidden w-64 rounded-lg border border-border p-3 text-xs normal-case leading-relaxed shadow-xl group-hover:block">
                  Use Go-compatible regular expressions. Put the extracted value in the first
                  capture group.
                </div>
              </div>
            </div>
            {selectedSampleExtractMissing && (
              <p className="rounded-lg border border-primary/30 bg-primary/10 px-3 py-2 text-xs text-primary">
                Expected values are present. Fill amount and merchant regex next so the sample can
                validate extraction.
              </p>
            )}
            <label className="block text-sm text-muted-foreground">
              Amount regex
              <input
                aria-label="Amount regex"
                value={form.amountRegex}
                onChange={(event) => updateForm({ amountRegex: event.target.value })}
                className={inputClasses(Boolean(fieldErrors.amountRegex), 'font-mono text-xs')}
              />
              {fieldErrors.amountRegex && (
                <span className="mt-1 block text-xs text-destructive">
                  {fieldErrors.amountRegex}
                </span>
              )}
            </label>
            <label className="block text-sm text-muted-foreground">
              Merchant regex
              <input
                aria-label="Merchant regex"
                value={form.merchantRegex}
                onChange={(event) => updateForm({ merchantRegex: event.target.value })}
                className={inputClasses(Boolean(fieldErrors.merchantRegex), 'font-mono text-xs')}
              />
              {fieldErrors.merchantRegex && (
                <span className="mt-1 block text-xs text-destructive">
                  {fieldErrors.merchantRegex}
                </span>
              )}
            </label>
            <label className="block text-sm text-muted-foreground">
              Currency regex
              <input
                aria-label="Currency regex"
                value={form.currencyRegex}
                onChange={(event) => updateForm({ currencyRegex: event.target.value })}
                placeholder="Optional"
                className={inputClasses(false, 'font-mono text-xs')}
              />
            </label>
          </section>

          <section className="space-y-3">
            <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Source Type
            </h2>
            <SourceValueCombobox
              label="Type"
              value={form.sourceType}
              options={facets?.source_types ?? []}
              customValues={customTypes}
              onChange={(value) => updateForm({ sourceType: value })}
              onAdd={(value) => setCustomTypes((current) => uniqueSorted([...current, value]))}
            />
            <h2 className="border-t border-border pt-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Bank
            </h2>
            <SourceValueCombobox
              label="Bank"
              value={form.bank}
              options={facets?.banks ?? []}
              customValues={customBanks}
              onChange={(value) => updateForm({ bank: value })}
              onAdd={(value) => setCustomBanks((current) => uniqueSorted([...current, value]))}
            />
          </section>
        </aside>

        <div
          aria-label="Sample workbench"
          className="grid min-w-0 overflow-hidden rounded-xl border border-border bg-card lg:grid-cols-[minmax(0,1fr)_20rem]"
        >
          <main className="min-w-0 border-b border-border lg:border-b-0 lg:border-r">
            <div className="flex items-center justify-between gap-3 border-b border-border p-4">
              <div role="tablist" aria-label="Samples" className="flex flex-wrap gap-2">
                {samples.map((sample, index) => (
                  <button
                    key={`${sample.name}-${index}`}
                    type="button"
                    role="tab"
                    aria-selected={index === activeSample}
                    onClick={() => setActiveSample(index)}
                    className={`rounded-full border px-3 py-1.5 text-sm font-medium ${
                      index === activeSample
                        ? 'border-primary/50 bg-primary/10 text-foreground'
                        : 'border-border text-muted-foreground hover:text-foreground'
                    }`}
                  >
                    {sample.name}
                  </button>
                ))}
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  onClick={deleteSample}
                  aria-label={`Delete sample ${selectedSample.name}`}
                  className="rounded-lg border border-border px-3 py-2 text-sm font-semibold text-muted-foreground hover:text-destructive"
                >
                  Delete sample
                </button>
                <button
                  type="button"
                  onClick={addSample}
                  className="rounded-lg border border-border px-3 py-2 text-sm font-semibold text-foreground hover:bg-secondary"
                >
                  + Add sample
                </button>
              </div>
            </div>

            <div className="grid gap-3 overflow-y-auto p-4">
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                <label className="text-sm text-muted-foreground">
                  Name
                  <input
                    value={selectedSample.name}
                    onChange={(event) => updateSample({ name: event.target.value })}
                    className="mt-1 w-full rounded-lg border border-border bg-input px-3 py-2 text-sm text-foreground"
                  />
                </label>
                <label className="text-sm text-muted-foreground">
                  Sender
                  <input
                    value={selectedSample.sender}
                    onChange={(event) => updateSample({ sender: event.target.value })}
                    className={inputClasses(
                      Boolean(fieldErrors.sampleSender || selectedSampleSenderInvalid),
                      'font-mono',
                    )}
                  />
                  {(fieldErrors.sampleSender || selectedSampleSenderInvalid) && (
                    <span className="mt-1 block text-xs text-destructive">
                      Enter a valid sender email address.
                    </span>
                  )}
                </label>
                <label className="text-sm text-muted-foreground md:col-span-2">
                  Subject
                  <input
                    value={selectedSample.subject}
                    onChange={(event) => updateSample({ subject: event.target.value })}
                    className="mt-1 w-full rounded-lg border border-border bg-input px-3 py-2 text-sm text-foreground"
                  />
                </label>
              </div>
              <label className="flex min-h-0 flex-col text-sm text-muted-foreground">
                Email body
                <textarea
                  value={selectedSample.body}
                  onChange={(event) => updateSample({ body: event.target.value })}
                  className="mt-1 h-[20rem] min-h-[14rem] resize-y rounded-lg border border-border bg-input px-3 py-3 font-mono text-xs text-foreground"
                />
              </label>
              {selectedSampleHasSenderAndBody && selectedSampleExpectedMissing && (
                <p className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-500">
                  Fill expected amount and merchant first. Once those are set, tune the extract
                  regex fields against this sample.
                </p>
              )}
            </div>
          </main>

          <aside className="space-y-4 p-4">
            <div className="rounded-xl border border-border p-4">
              <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Expected
              </h2>
              <div className="mt-3 space-y-3">
                <label className="block text-sm text-muted-foreground">
                  Amount
                  <input
                    aria-label="Expected amount"
                    value={selectedSample.expected.amount}
                    onChange={(event) => updateExpected({ amount: event.target.value })}
                    className="mt-1 w-full rounded-lg border border-border bg-input px-3 py-2 font-mono text-sm text-foreground"
                  />
                </label>
                <label className="block text-sm text-muted-foreground">
                  Merchant
                  <input
                    aria-label="Expected merchant"
                    value={selectedSample.expected.merchant}
                    onChange={(event) => updateExpected({ merchant: event.target.value })}
                    className="mt-1 w-full rounded-lg border border-border bg-input px-3 py-2 text-sm text-foreground"
                  />
                </label>
                <label className="block text-sm text-muted-foreground">
                  Currency
                  <input
                    aria-label="Expected currency"
                    value={selectedSample.expected.currency}
                    onChange={(event) => updateExpected({ currency: event.target.value })}
                    className="mt-1 w-full rounded-lg border border-border bg-input px-3 py-2 font-mono text-sm text-foreground"
                  />
                </label>
              </div>
            </div>

            <div className="rounded-xl border border-border p-4">
              <div className="flex items-center justify-between gap-3">
                <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                  Live Result
                </h2>
                {selectedSampleHasData && (
                  <span
                    className={`rounded-full border px-3 py-1 text-xs font-medium ${
                      needsAttention
                        ? 'border-destructive/40 text-destructive'
                        : 'border-green-500/40 text-green-500'
                    }`}
                  >
                    {needsAttention ? 'Needs attention' : 'All checks pass'}
                  </span>
                )}
              </div>
              {selectedSampleHasData ? (
                <dl className="mt-3 divide-y divide-border text-sm">
                  {selectedSample.sender.trim() !== '' && (
                    <div className="flex items-center justify-between gap-3 py-2">
                      <dt className="text-muted-foreground">Sender</dt>
                      <dd
                        className={
                          !selectedSampleSenderInvalid &&
                          form.senders.includes(selectedSample.sender)
                            ? 'text-green-500'
                            : 'text-destructive'
                        }
                      >
                        {!selectedSampleSenderInvalid &&
                        form.senders.includes(selectedSample.sender)
                          ? 'matches'
                          : 'missing'}
                      </dd>
                    </div>
                  )}
                  {selectedSample.subject.trim() !== '' && (
                    <div className="flex items-center justify-between gap-3 py-2">
                      <dt className="text-muted-foreground">Subject</dt>
                      <dd
                        className={
                          !form.subjectContains ||
                          selectedSample.subject.includes(form.subjectContains)
                            ? 'text-green-500'
                            : 'text-destructive'
                        }
                      >
                        {!form.subjectContains ||
                        selectedSample.subject.includes(form.subjectContains)
                          ? 'matches'
                          : 'missing'}
                      </dd>
                    </div>
                  )}
                  {selectedSample.body.trim() !== '' && (
                    <>
                      <div className="flex items-center justify-between gap-3 py-2">
                        <dt className="text-muted-foreground">Amount</dt>
                        <dd>
                          <ResultValue result={live.amount} />
                        </dd>
                      </div>
                      <div className="flex items-center justify-between gap-3 py-2">
                        <dt className="text-muted-foreground">Merchant</dt>
                        <dd>
                          <ResultValue result={live.merchant} />
                        </dd>
                      </div>
                      <div className="flex items-center justify-between gap-3 py-2">
                        <dt className="text-muted-foreground">Currency</dt>
                        <dd>
                          <ResultValue result={live.currency} optional />
                        </dd>
                      </div>
                    </>
                  )}
                </dl>
              ) : (
                <p className="mt-3 rounded-lg border border-border bg-secondary/30 px-3 py-3 text-xs text-muted-foreground">
                  No sample data yet
                </p>
              )}
            </div>

            {formError && <p className="text-xs text-destructive">{formError}</p>}
          </aside>
        </div>
      </div>

      {saveDialogOpen && (
        <ConfirmModal
          title="Save rule changes?"
          message={
            activeReader
              ? 'Save & Exit updates the rule and returns to the rules list. Save & Re-scan also re-processes emails from the configured lookback window using the updated rule.'
              : 'Save & Exit updates the rule and returns to the rules list. Save & Re-scan needs an active reader, so it is unavailable right now.'
          }
          confirmLabel="Save & Re-scan"
          secondaryLabel="Save & Exit"
          confirmDisabled={!activeReader}
          onConfirm={() => {
            if (!activeReader) return
            setSaveDialogOpen(false)
            saveRule(true)
          }}
          onSecondary={() => {
            setSaveDialogOpen(false)
            saveRule(false)
          }}
          onCancel={() => setSaveDialogOpen(false)}
        />
      )}
    </div>
  )
}
