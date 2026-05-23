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
import { useI18n } from '@/i18n/I18nProvider'
import type { ReactNode } from 'react'

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

const RULE_CONTRIBUTION_GUIDE_URL =
  'https://github.com/ArionMiles/expensor/blob/main/.github/CONTRIBUTING.md#adding-bank-support'

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

function diagnosticSample(
  diagnostic: {
    sender_email: string
    subject: string
    email_body: string
  },
  name: string,
): SampleState {
  return {
    name,
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

function blankSample(name: string): SampleState {
  return {
    name,
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

function yamlScalar(value: string) {
  return JSON.stringify(value)
}

function yamlNumberOrScalar(value: string) {
  const trimmed = value.trim()
  return /^-?\d+(?:\.\d+)?$/.test(trimmed) ? trimmed : yamlScalar(trimmed)
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

type ZipEntry = {
  filename: string
  content: string
}

const textEncoder = new TextEncoder()
let crcTable: Uint32Array | null = null

function downloadBlob(filename: string, blob: Blob) {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}

function crc32(bytes: Uint8Array) {
  if (!crcTable) {
    crcTable = new Uint32Array(256)
    for (let n = 0; n < 256; n += 1) {
      let value = n
      for (let k = 0; k < 8; k += 1) {
        value = value & 1 ? 0xedb88320 ^ (value >>> 1) : value >>> 1
      }
      crcTable[n] = value >>> 0
    }
  }

  let crc = 0xffffffff
  for (const byte of bytes) {
    crc = crcTable[(crc ^ byte) & 0xff] ^ (crc >>> 8)
  }
  return (crc ^ 0xffffffff) >>> 0
}

function writeUint16(bytes: number[], value: number) {
  bytes.push(value & 0xff, (value >>> 8) & 0xff)
}

function writeUint32(bytes: number[], value: number) {
  bytes.push(value & 0xff, (value >>> 8) & 0xff, (value >>> 16) & 0xff, (value >>> 24) & 0xff)
}

function writeBytes(bytes: number[], value: Uint8Array) {
  for (const byte of value) bytes.push(byte)
}

function buildStoredZip(entries: ZipEntry[]) {
  const output: number[] = []
  const centralDirectory: number[] = []

  for (const entry of entries) {
    const filename = textEncoder.encode(entry.filename)
    const content = textEncoder.encode(entry.content)
    const checksum = crc32(content)
    const localHeaderOffset = output.length

    writeUint32(output, 0x04034b50)
    writeUint16(output, 20)
    writeUint16(output, 0)
    writeUint16(output, 0)
    writeUint16(output, 0)
    writeUint16(output, 0)
    writeUint32(output, checksum)
    writeUint32(output, content.length)
    writeUint32(output, content.length)
    writeUint16(output, filename.length)
    writeUint16(output, 0)
    writeBytes(output, filename)
    writeBytes(output, content)

    writeUint32(centralDirectory, 0x02014b50)
    writeUint16(centralDirectory, 20)
    writeUint16(centralDirectory, 20)
    writeUint16(centralDirectory, 0)
    writeUint16(centralDirectory, 0)
    writeUint16(centralDirectory, 0)
    writeUint16(centralDirectory, 0)
    writeUint32(centralDirectory, checksum)
    writeUint32(centralDirectory, content.length)
    writeUint32(centralDirectory, content.length)
    writeUint16(centralDirectory, filename.length)
    writeUint16(centralDirectory, 0)
    writeUint16(centralDirectory, 0)
    writeUint16(centralDirectory, 0)
    writeUint16(centralDirectory, 0)
    writeUint32(centralDirectory, 0)
    writeUint32(centralDirectory, localHeaderOffset)
    writeBytes(centralDirectory, filename)
  }

  const centralDirectoryOffset = output.length
  writeBytes(output, new Uint8Array(centralDirectory))
  writeUint32(output, 0x06054b50)
  writeUint16(output, 0)
  writeUint16(output, 0)
  writeUint16(output, entries.length)
  writeUint16(output, entries.length)
  writeUint32(output, centralDirectory.length)
  writeUint32(output, centralDirectoryOffset)
  writeUint16(output, 0)

  return new Blob([new Uint8Array(output)], { type: 'application/zip' })
}

type ComboboxProps = {
  label: string
  listboxLabel: string
  value: string
  options: string[]
  customValues: string[]
  onChange: (value: string) => void
  onAdd: (value: string) => void
  addLabel: (value: string) => string
}

function SourceValueCombobox({
  label,
  listboxLabel,
  value,
  options,
  customValues,
  onChange,
  onAdd,
  addLabel,
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
            aria-label={listboxLabel}
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
                {addLabel(value.trim())}
              </button>
            )}
          </div>,
          document.body,
        )}
    </div>
  )
}

function ResultValue({ result, optional = false }: { result: RegexResult; optional?: boolean }) {
  const { t } = useI18n()
  if (result.invalid) return <span className="text-destructive">{t('common.invalid')}</span>
  if (result.match !== null && result.match.trim() !== '') {
    return <span className="font-mono text-green-500">{result.match}</span>
  }
  if (optional) return <span className="text-muted-foreground">{t('common.optional')}</span>
  return <span className="text-destructive">{t('common.missing')}</span>
}

function HintDot({ label, children }: { label: string; children: ReactNode }) {
  return (
    <span className="group relative inline-flex items-center">
      <button
        type="button"
        aria-label={label}
        className="h-2.5 w-2.5 rounded-full bg-amber-500 ring-4 ring-amber-500/15"
      />
      <span className="pointer-events-none absolute right-0 top-4 z-50 hidden w-56 rounded-lg border border-border bg-card p-2 text-xs normal-case leading-relaxed text-card-foreground shadow-xl ring-1 ring-border group-hover:block">
        {children}
      </span>
    </span>
  )
}

export function RuleForm() {
  const { t } = useI18n()
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
  const [lastSavedName, setLastSavedName] = useState(t('rules.editor.newRuleName'))
  const [samples, setSamples] = useState<SampleState[]>([
    blankSample(t('rules.editor.sampleDefaultName', { index: 1 })),
  ])
  const [activeSample, setActiveSample] = useState(0)
  const [customTypes, setCustomTypes] = useState<string[]>([])
  const [customBanks, setCustomBanks] = useState<string[]>([])
  const [toast, setToast] = useState('')
  const [formError, setFormError] = useState('')
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})
  const [exportDialogOpen, setExportDialogOpen] = useState(false)
  const [saveDialogOpen, setSaveDialogOpen] = useState(false)

  const { mutate: createRule, isPending: creating } = useCreateRule()
  const { mutate: updateRule, isPending: updating } = useUpdateRule()
  const { mutate: triggerRescan } = useRescan()

  useEffect(() => {
    if (!rule) return
    const nextName = rule.name || t('rules.editor.newRuleName')
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

    setSamples([diagnosticSample(diagnostic, t('rules.editor.diagnosticSample'))])
    setActiveSample(0)
    if (!isCreate) return

    const nextName = diagnostic.rule_name || t('rules.editor.newRuleName')
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
      const nextIndex = current.length + 1
      const next = [
        ...current,
        blankSample(t('rules.editor.sampleDefaultName', { index: nextIndex })),
      ]
      setActiveSample(next.length - 1)
      return next
    })
  }

  const deleteSampleAt = (sampleIndex: number) => {
    setSamples((current) => {
      if (current.length === 1) {
        setActiveSample(0)
        return [blankSample(t('rules.editor.sampleDefaultName', { index: 1 }))]
      }
      const next = current.filter((_, index) => index !== sampleIndex)
      setActiveSample((currentActive) => {
        if (currentActive === sampleIndex)
          return Math.max(0, Math.min(sampleIndex, next.length - 1))
        if (currentActive > sampleIndex) return currentActive - 1
        return currentActive
      })
      return next
    })
  }

  const buildRulePayload = () => ({
    name: form.name.trim(),
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
  })

  const buildFixtureFile = (sample: SampleState) => `---
rule: ${yamlScalar(form.name || t('rules.editor.newRuleName'))}
sender: ${yamlScalar(sample.sender)}
subject: ${yamlScalar(sample.subject)}
expected:
  amount: ${yamlNumberOrScalar(sample.expected.amount || '')}
  merchant: ${yamlScalar(sample.expected.merchant || '')}
  currency: ${yamlScalar(sample.expected.currency || '')}
---
${sample.body.replace(/\r\n/g, '\n')}`

  const buildFixtureEntries = () => {
    const bankSlug = slug(form.bank || 'bank')
    const typeSlug = slug(form.sourceType || 'source-type')
    const populatedSamples = samples.filter(sampleHasValidationData)
    const seen = new Map<string, number>()
    return populatedSamples.map((sample) => {
      const caseSlug = slug(sample.name || form.name || 'sample')
      const baseName = `${bankSlug}_${typeSlug}_${caseSlug}`
      const count = seen.get(baseName) ?? 0
      seen.set(baseName, count + 1)
      const filename = `${baseName}${count === 0 ? '' : `-${count + 1}`}.rule.fixture`
      return { filename, content: buildFixtureFile(sample) }
    })
  }

  const exportRuleAndFixtures = () => {
    const rulePayload = buildRulePayload()
    const sourceTypes = uniqueSorted([...(facets?.source_types ?? []), form.sourceType])
    const banks = uniqueSorted([...(facets?.banks ?? []), form.bank])
    const ruleFile = {
      version: 2,
      presets: {
        source_types: sourceTypes.map((value) => ({
          value,
          origin: facets?.source_types?.includes(value) ? 'predefined' : 'custom',
        })),
        banks: banks.map((value) => ({
          value,
          origin: facets?.banks?.includes(value) ? 'predefined' : 'custom',
        })),
      },
      rules: [rulePayload],
    }

    const baseName = slug(form.name || 'rule')
    const entries = [
      {
        filename: `${baseName}.rule.json`,
        content: JSON.stringify(ruleFile, null, 2),
      },
      ...buildFixtureEntries(),
    ]

    downloadBlob(`${baseName}.contribution.zip`, buildStoredZip(entries))
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
  const hasExportableSamples = samples.some(sampleHasValidationData)
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
      errors.name = t('rules.editor.ruleNameRequired')
    }
    if (form.senders.length === 0) {
      errors.senders = t('rules.editor.senderRequired')
    }
    if (!form.amountRegex.trim()) {
      errors.amountRegex = t('rules.editor.amountRegexRequired')
    }
    if (!form.merchantRegex.trim()) {
      errors.merchantRegex = t('rules.editor.merchantRegexRequired')
    }
    if (selectedSampleSenderInvalid) {
      errors.sampleSender = t('rules.editor.senderEmailInvalid')
    }
    setFieldErrors(errors)
    return { valid: Object.keys(errors).length === 0, name }
  }

  const handleSaveError = (error: Error) => {
    if (error.message === 'rule name already exists') {
      setFormError('')
      setFieldErrors((current) => ({
        ...current,
        name: t('rules.editor.ruleNameExists'),
      }))
      return
    }
    setFormError(error.message)
  }

  const saveRule = (shouldRescan: boolean) => {
    const body = buildRulePayload()

    if (isCreate) {
      createRule(body, {
        onSuccess: () => navigate('/rules'),
        onError: handleSaveError,
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
                  ? t('rules.editor.toastRescanStarted')
                  : t('rules.editor.toastRescanQueued')
              setToast(msg)
              setTimeout(() => navigate('/rules'), 2500)
            },
            onError: () => navigate('/rules'),
          })
        },
        onError: handleSaveError,
      },
    )
  }

  const handleSubmit = () => {
    const result = validateForm()
    if (!result.valid) return

    if (hasExportableSamples) {
      setExportDialogOpen(true)
      return
    }

    continueSaveFlow()
  }

  const continueSaveFlow = () => {
    if (!isCreate) {
      setSaveDialogOpen(true)
      return
    }

    saveRule(false)
  }

  if (!isCreate && rulesLoading) {
    return (
      <div className="mx-auto w-full max-w-6xl px-6 py-6">
        <p className="text-xs text-muted-foreground">{t('rules.editor.loading')}</p>
      </div>
    )
  }

  if (!isCreate && !rule) {
    return (
      <div className="mx-auto w-full max-w-6xl px-6 py-6">
        <p className="text-sm text-destructive">{t('rules.editor.ruleNotFound')}</p>
        <Link to="/rules" className="text-xs text-primary hover:underline">
          {t('rules.backToRules')}
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
              {t('rules.pageTitle')}
            </Link>
            <span aria-hidden="true">›</span>
            <span className="text-foreground">
              {isCreate ? t('rules.editor.newRuleName') : t('rules.editor.editRule')}
            </span>
          </nav>
          <input
            aria-label={t('rules.editor.ruleName')}
            value={form.name}
            onChange={(event) => {
              updateForm({ name: event.target.value })
              if (fieldErrors.name === t('rules.editor.ruleNameExists')) {
                setFieldErrors((current) => ({ ...current, name: undefined }))
              }
            }}
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
          <p className="mt-1 text-sm text-muted-foreground">{t('rules.editor.formSummary')}</p>
        </div>
        <div aria-label={t('rules.editor.actions')} className="flex flex-wrap items-center gap-2">
          {rule?.predefined && (
            <span className="rounded-full border border-primary/40 px-3 py-1 text-xs font-medium text-primary">
              {t('common.predefined')}
            </span>
          )}
          <button
            type="button"
            onClick={handleSubmit}
            disabled={isPending}
            className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
          >
            {isPending ? t('common.saving') : t('rules.editor.saveRule')}
          </button>
          <Link
            to="/rules"
            className="inline-flex rounded-lg border border-border px-4 py-2 text-sm font-semibold text-muted-foreground hover:text-foreground"
          >
            {t('rules.editor.cancel')}
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
          aria-label={t('rules.editor.ruleSettings')}
          className="space-y-4 rounded-xl border border-border bg-card p-4"
        >
          <section className="space-y-2 border-b border-border pb-4">
            <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {t('rules.editor.subjectContains')}
            </h2>
            <input
              aria-label={t('rules.editor.subjectContainsInput')}
              value={form.subjectContains}
              onChange={(event) => updateForm({ subjectContains: event.target.value })}
              className="w-full rounded-lg border border-border bg-input px-3 py-2 text-sm text-foreground"
            />
          </section>

          <section className="space-y-2 border-b border-border pb-4">
            <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {t('common.sender')}
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
                    aria-label={t('rules.editor.removeSender', { sender })}
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
              aria-label={t('rules.addSender')}
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
              <div className="flex items-center gap-2">
                <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                  {t('rules.editor.extract')}
                </h2>
                {selectedSampleExtractMissing && (
                  <HintDot label={t('rules.editor.extractRegexNeeded')}>
                    {t('rules.editor.extractRegexNeededHint')}
                  </HintDot>
                )}
              </div>
              <div className="group relative">
                <button
                  type="button"
                  aria-label={t('rules.editor.extractSyntaxHelp')}
                  className="flex h-6 w-6 items-center justify-center rounded-full border border-border text-xs font-semibold text-muted-foreground hover:text-foreground"
                >
                  ?
                </button>
                <div className="pointer-events-none absolute right-0 top-8 z-50 hidden w-72 rounded-lg border border-border bg-card p-3 text-xs normal-case leading-relaxed text-card-foreground shadow-xl ring-1 ring-border group-hover:block">
                  {t('rules.editor.extractSyntaxHelpText')}
                </div>
              </div>
            </div>
            <label className="block text-sm text-muted-foreground">
              {t('rules.editor.amountRegex')}
              <input
                aria-label={t('rules.editor.amountRegex')}
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
              {t('rules.editor.merchantRegex')}
              <input
                aria-label={t('rules.editor.merchantRegex')}
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
              {t('rules.editor.currencyRegex')}
              <input
                aria-label={t('rules.editor.currencyRegex')}
                value={form.currencyRegex}
                onChange={(event) => updateForm({ currencyRegex: event.target.value })}
                placeholder={t('common.optional')}
                className={inputClasses(false, 'font-mono text-xs')}
              />
            </label>
          </section>

          <section className="space-y-3">
            <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {t('rules.editor.sourceTypeSection')}
            </h2>
            <SourceValueCombobox
              label={t('common.type')}
              listboxLabel={t('rules.editor.typeOptions')}
              value={form.sourceType}
              options={facets?.source_types ?? []}
              customValues={customTypes}
              onChange={(value) => updateForm({ sourceType: value })}
              onAdd={(value) => setCustomTypes((current) => uniqueSorted([...current, value]))}
              addLabel={(value) => t('rules.editor.addOption', { value })}
            />
            <h2 className="border-t border-border pt-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {t('rules.editor.bankSection')}
            </h2>
            <SourceValueCombobox
              label={t('common.bank')}
              listboxLabel={t('rules.editor.bankOptions')}
              value={form.bank}
              options={facets?.banks ?? []}
              customValues={customBanks}
              onChange={(value) => updateForm({ bank: value })}
              onAdd={(value) => setCustomBanks((current) => uniqueSorted([...current, value]))}
              addLabel={(value) => t('rules.editor.addOption', { value })}
            />
          </section>
        </aside>

        <div
          aria-label={t('rules.editor.sampleWorkbench')}
          className="grid min-w-0 overflow-hidden rounded-xl border border-border bg-card lg:grid-cols-[minmax(0,1fr)_20rem]"
        >
          <main className="min-w-0 border-b border-border lg:border-b-0 lg:border-r">
            <div className="flex min-w-0 items-center gap-3 border-b border-border p-4">
              <div
                role="tablist"
                aria-label={t('rules.editor.sampleTabs')}
                className="flex min-w-0 flex-1 flex-nowrap gap-2 overflow-x-auto pb-1"
              >
                {samples.map((sample, index) => (
                  <span
                    key={`${sample.name}-${index}`}
                    className={`inline-flex shrink-0 items-center overflow-hidden rounded-full border text-sm font-medium ${
                      index === activeSample
                        ? 'border-primary/50 bg-primary/10 text-foreground'
                        : 'border-border text-muted-foreground hover:text-foreground'
                    }`}
                  >
                    <button
                      type="button"
                      role="tab"
                      aria-selected={index === activeSample}
                      onClick={() => setActiveSample(index)}
                      className="px-3 py-1.5"
                    >
                      {sample.name}
                    </button>
                    <button
                      type="button"
                      aria-label={t('rules.editor.removeSample', { name: sample.name })}
                      onClick={(event) => {
                        event.stopPropagation()
                        deleteSampleAt(index)
                      }}
                      className="pr-3 text-sm leading-none text-muted-foreground hover:text-destructive"
                    >
                      x
                    </button>
                  </span>
                ))}
              </div>
              <button
                type="button"
                onClick={addSample}
                className="shrink-0 rounded-lg border border-border px-3 py-2 text-sm font-semibold text-foreground hover:bg-secondary"
              >
                {t('rules.addSample')}
              </button>
            </div>

            <div className="grid gap-3 overflow-y-auto p-4">
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                <label className="text-sm text-muted-foreground">
                  {t('common.name')}
                  <input
                    value={selectedSample.name}
                    onChange={(event) => updateSample({ name: event.target.value })}
                    className="mt-1 w-full rounded-lg border border-border bg-input px-3 py-2 text-sm text-foreground"
                  />
                </label>
                <label className="text-sm text-muted-foreground">
                  {t('common.sender')}
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
                      {t('rules.editor.senderEmailInvalid')}
                    </span>
                  )}
                </label>
                <label className="text-sm text-muted-foreground md:col-span-2">
                  {t('common.subject')}
                  <input
                    value={selectedSample.subject}
                    onChange={(event) => updateSample({ subject: event.target.value })}
                    className="mt-1 w-full rounded-lg border border-border bg-input px-3 py-2 text-sm text-foreground"
                  />
                </label>
              </div>
              <label className="flex min-h-0 flex-col text-sm text-muted-foreground">
                {t('rules.editor.emailBody')}
                <textarea
                  value={selectedSample.body}
                  onChange={(event) => updateSample({ body: event.target.value })}
                  className="mt-1 h-[20rem] min-h-[14rem] resize-y rounded-lg border border-border bg-input px-3 py-3 font-mono text-xs text-foreground"
                />
              </label>
            </div>
          </main>

          <aside className="space-y-4 p-4">
            <div className="rounded-xl border border-border p-4">
              <div className="flex items-center gap-2">
                <h2 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                  {t('rules.editor.expected')}
                </h2>
                {selectedSampleExpectedMissing && (
                  <HintDot label={t('rules.editor.expectedValuesNeeded')}>
                    {t('rules.editor.fillExpectedHint')}
                  </HintDot>
                )}
              </div>
              <div className="mt-3 space-y-3">
                <label className="block text-sm text-muted-foreground">
                  {t('common.amount')}
                  <input
                    aria-label={t('rules.editor.expectedAmount')}
                    value={selectedSample.expected.amount}
                    onChange={(event) => updateExpected({ amount: event.target.value })}
                    className="mt-1 w-full rounded-lg border border-border bg-input px-3 py-2 font-mono text-sm text-foreground"
                  />
                </label>
                <label className="block text-sm text-muted-foreground">
                  {t('common.merchant')}
                  <input
                    aria-label={t('rules.editor.expectedMerchant')}
                    value={selectedSample.expected.merchant}
                    onChange={(event) => updateExpected({ merchant: event.target.value })}
                    className="mt-1 w-full rounded-lg border border-border bg-input px-3 py-2 text-sm text-foreground"
                  />
                </label>
                <label className="block text-sm text-muted-foreground">
                  {t('common.currency')}
                  <input
                    aria-label={t('rules.editor.expectedCurrency')}
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
                  {t('rules.editor.liveResult')}
                </h2>
                {selectedSampleHasData && (
                  <span
                    className={`rounded-full border px-3 py-1 text-xs font-medium ${
                      needsAttention
                        ? 'border-destructive/40 text-destructive'
                        : 'border-green-500/40 text-green-500'
                    }`}
                  >
                    {needsAttention
                      ? t('rules.editor.needsAttention')
                      : t('rules.editor.allChecksPass')}
                  </span>
                )}
              </div>
              {selectedSampleHasData ? (
                <dl className="mt-3 divide-y divide-border text-sm">
                  {selectedSample.sender.trim() !== '' && (
                    <div className="flex items-center justify-between gap-3 py-2">
                      <dt className="text-muted-foreground">{t('common.sender')}</dt>
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
                          ? t('common.matches')
                          : t('common.missing')}
                      </dd>
                    </div>
                  )}
                  {selectedSample.subject.trim() !== '' && (
                    <div className="flex items-center justify-between gap-3 py-2">
                      <dt className="text-muted-foreground">{t('common.subject')}</dt>
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
                          ? t('common.matches')
                          : t('common.missing')}
                      </dd>
                    </div>
                  )}
                  {selectedSample.body.trim() !== '' && (
                    <>
                      <div className="flex items-center justify-between gap-3 py-2">
                        <dt className="text-muted-foreground">{t('common.amount')}</dt>
                        <dd>
                          <ResultValue result={live.amount} />
                        </dd>
                      </div>
                      <div className="flex items-center justify-between gap-3 py-2">
                        <dt className="text-muted-foreground">{t('common.merchant')}</dt>
                        <dd>
                          <ResultValue result={live.merchant} />
                        </dd>
                      </div>
                      <div className="flex items-center justify-between gap-3 py-2">
                        <dt className="text-muted-foreground">{t('common.currency')}</dt>
                        <dd>
                          <ResultValue result={live.currency} optional />
                        </dd>
                      </div>
                    </>
                  )}
                </dl>
              ) : (
                <p className="mt-3 rounded-lg border border-border bg-secondary/30 px-3 py-3 text-xs text-muted-foreground">
                  {t('rules.editor.noSampleData')}
                </p>
              )}
            </div>

            {formError && <p className="text-xs text-destructive">{formError}</p>}
          </aside>
        </div>
      </div>

      {exportDialogOpen && (
        <ConfirmModal
          title={t('rules.editor.exportContributionTitle')}
          message={
            <div className="space-y-3">
              <p>{t('rules.editor.exportContributionBody')}</p>
              <a
                href={RULE_CONTRIBUTION_GUIDE_URL}
                target="_blank"
                rel="noreferrer"
                className="inline-flex font-medium text-primary hover:underline"
              >
                {t('rules.editor.contributionGuide')}
              </a>
            </div>
          }
          confirmLabel={t('rules.editor.exportContinue')}
          secondaryLabel={t('rules.editor.skipExport')}
          onConfirm={() => {
            exportRuleAndFixtures()
            setExportDialogOpen(false)
            continueSaveFlow()
          }}
          onSecondary={() => {
            setExportDialogOpen(false)
            continueSaveFlow()
          }}
          onCancel={() => setExportDialogOpen(false)}
        />
      )}

      {saveDialogOpen && (
        <ConfirmModal
          title={t('rules.editor.saveDialogTitle')}
          message={
            activeReader
              ? t('rules.editor.saveDialogWithReader')
              : t('rules.editor.saveDialogNoReader')
          }
          confirmLabel={t('rules.editor.saveAndRescan')}
          secondaryLabel={t('rules.editor.saveAndExit')}
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
