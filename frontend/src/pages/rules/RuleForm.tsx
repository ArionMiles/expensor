import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { LoaderCircle, Sparkles, X } from 'lucide-react'
import { ApiError } from '@/api/client'
import type { RuleDraftValidationIssue } from '@/api/types'
import {
  useActiveReader,
  useActiveLLMProviderStatus,
  useCreateRule,
  useDraftRule,
  useExtractionDiagnostic,
  useFacets,
  useRescan,
  useRules,
  useSession,
  useUpdateRule,
} from '@/api/queries'
import { ConfirmModal } from '@/components/ConfirmModal'
import { useTooltip } from '@/hooks/useTooltip'
import { useI18n } from '@/i18n/I18nProvider'
import { loadRuleEmailSearchDraft } from './emailSearchDraft'
import {
  HintDot,
  RULE_CONTRIBUTION_GUIDE_URL,
  ResultValue,
  SourceValueCombobox,
  blankSample,
  buildStoredZip,
  diagnosticSample,
  downloadBlob,
  emptyForm,
  inputClasses,
  isValidEmail,
  sampleHasValidationData,
  slug,
  sourceLabel,
  testRegex,
  uniqueSorted,
  yamlNumberOrScalar,
  yamlScalar,
} from './RuleFormSupport'
import type { FieldErrors, FormState, SampleState } from './RuleFormSupport'

const AI_DRAFT_PRIVACY_REMINDER_KEY_PREFIX = 'expensor.rule-draft-ai-privacy-reminder-dismissed'

function aiDraftPrivacyReminderKey(userID: string, providerName: string) {
  return `${AI_DRAFT_PRIVACY_REMINDER_KEY_PREFIX}:${encodeURIComponent(userID)}:${encodeURIComponent(providerName)}`
}

function aiDraftPrivacyReminderDismissed(userID: string, providerName: string) {
  try {
    return window.localStorage.getItem(aiDraftPrivacyReminderKey(userID, providerName)) === 'true'
  } catch {
    return false
  }
}

function dismissAIDraftPrivacyReminder(userID: string, providerName: string) {
  try {
    window.localStorage.setItem(aiDraftPrivacyReminderKey(userID, providerName), 'true')
    return true
  } catch {
    return false
  }
}

export function RuleForm() {
  const { t } = useI18n()
  const { id } = useParams<{ id: string }>()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const isCreate = !id
  const diagnosticID = searchParams.get('diagnostic')
  const draftID = searchParams.get('draft')

  const { data: rules = [], isLoading: rulesLoading } = useRules()
  const rule = id ? rules.find((candidate) => candidate.id === id) : null
  const { data: diagnostic } = useExtractionDiagnostic(diagnosticID)
  const { data: activeReader = '' } = useActiveReader()
  const { data: session } = useSession()
  const { data: facets } = useFacets()
  const { data: llmStatus, provider: llmProvider } = useActiveLLMProviderStatus()
  const { handlers: aiTipHandlers, tip: aiTip } = useTooltip()

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
  const [draftIssues, setDraftIssues] = useState<RuleDraftValidationIssue[]>([])
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})
  const [exportDialogOpen, setExportDialogOpen] = useState(false)
  const [saveDialogOpen, setSaveDialogOpen] = useState(false)
  const [aiPrivacyDialogOpen, setAIPrivacyDialogOpen] = useState(false)
  const [skipAIPrivacyReminder, setSkipAIPrivacyReminder] = useState(false)
  const [draftApplied, setDraftApplied] = useState(false)

  const { mutate: createRule, isPending: creating } = useCreateRule()
  const { mutate: updateRule, isPending: updating } = useUpdateRule()
  const { mutate: draftRule, isPending: drafting } = useDraftRule()
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

  useEffect(() => {
    if (!isCreate || diagnosticID || !draftID || draftApplied) return

    const draft = loadRuleEmailSearchDraft(draftID)
    if (!draft || draft.messages.length === 0) {
      setDraftApplied(true)
      return
    }

    const nextSamples = draft.messages.map((message, index) => ({
      name: t('rules.editor.sampleDefaultName', { index: index + 1 }),
      sender: message.sender_email,
      subject: message.subject,
      body: message.body,
      expected: {
        amount: '',
        merchant: '',
        currency: '',
      },
    }))
    const senders = uniqueSorted(draft.messages.map((message) => message.sender_email))
    const subjects = uniqueSorted(draft.messages.map((message) => message.subject))
    setSamples(nextSamples)
    setActiveSample(0)
    updateForm({
      subjectContains:
        subjects.length === 1 ? subjects[0] : draft.subjectQuery || form.subjectContains,
      senders,
      senderDraft: '',
    })
    setDraftApplied(true)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [diagnosticID, draftApplied, draftID, isCreate, t])

  const updateForm = (patch: Partial<FormState>) => {
    setDraftIssues([])
    setForm((current) => ({ ...current, ...patch }))
  }

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

  const updateSample = (patch: Partial<SampleState>) => {
    setDraftIssues([])
    setSamples((current) =>
      current.map((sample, index) => (index === activeSample ? { ...sample, ...patch } : sample)),
    )
  }

  const updateExpected = (patch: Partial<SampleState['expected']>) => {
    setDraftIssues([])
    setSamples((current) =>
      current.map((sample, index) =>
        index === activeSample ? { ...sample, expected: { ...sample.expected, ...patch } } : sample,
      ),
    )
  }

  const addSample = () => {
    setDraftIssues([])
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
    setDraftIssues([])
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
  const hasAIDraftSample = samples.some(
    (sample) =>
      sample.body.trim() && sample.expected.amount.trim() && sample.expected.merchant.trim(),
  )
  const aiReady = llmStatus?.ready ?? false
  const aiDraftDisabled = drafting || !hasAIDraftSample || !aiReady
  const aiDraftTooltip = drafting
    ? t('rules.editor.aiDraftInProgress')
    : aiReady
      ? t('rules.editor.aiDraftTooltip')
      : t('rules.editor.aiDraftSetupNeeded')
  const aiProviderName =
    llmProvider?.display_name || llmStatus?.name || t('rules.editor.aiProviderFallback')
  const aiProviderID = llmProvider?.name || llmStatus?.name || 'unknown'
  const privacyReminderUserID = session?.user_id || 'unknown'
  const issueCountsBySample = useMemo(() => {
    const counts = new Map<number, number>()
    draftIssues.forEach((issue) => {
      if (issue.sample_index < 0) return
      counts.set(issue.sample_index, (counts.get(issue.sample_index) ?? 0) + 1)
    })
    return counts
  }, [draftIssues])
  const live = useMemo(
    () => ({
      amount: testRegex(form.amountRegex, selectedSample?.body ?? ''),
      merchant: testRegex(form.merchantRegex, selectedSample?.body ?? ''),
      currency: testRegex(form.currencyRegex, selectedSample?.body ?? ''),
    }),
    [form.amountRegex, form.currencyRegex, form.merchantRegex, selectedSample?.body],
  )
  const shouldEvaluateExtraction =
    selectedSample.body.trim() !== '' &&
    (Boolean(form.amountRegex.trim() || form.merchantRegex.trim() || form.currencyRegex.trim()) ||
      draftIssues.length > 0)
  const selectedSampleSenderMatches =
    selectedSample.sender.trim() === '' ||
    (!selectedSampleSenderInvalid && form.senders.includes(selectedSample.sender))
  const selectedSampleSubjectMatches =
    selectedSample.subject.trim() === '' ||
    !form.subjectContains ||
    selectedSample.subject.includes(form.subjectContains)
  const selectedSampleIdentityNeedsAttention =
    !selectedSampleSenderMatches || !selectedSampleSubjectMatches
  const needsAttention =
    selectedSampleHasData &&
    (selectedSampleIdentityNeedsAttention ||
      (shouldEvaluateExtraction &&
        (live.amount.invalid ||
          live.merchant.invalid ||
          !live.amount.match ||
          live.amount.match.trim() === '' ||
          !live.merchant.match ||
          live.merchant.match.trim() === '' ||
          (selectedSample.expected.amount.trim() !== '' &&
            live.amount.match !== selectedSample.expected.amount.trim()) ||
          (selectedSample.expected.merchant.trim() !== '' &&
            live.merchant.match !== selectedSample.expected.merchant.trim()) ||
          (selectedSample.expected.currency.trim() !== '' &&
            live.currency.match !== selectedSample.expected.currency.trim()))))
  const showLiveStatus =
    selectedSampleHasData && (selectedSampleIdentityNeedsAttention || shouldEvaluateExtraction)

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

    if (error instanceof ApiError) {
      const sourceErrors: Pick<FieldErrors, 'sourceType' | 'bank'> = {}
      for (const validationError of error.validationErrors) {
        if (validationError.location !== 'body') continue
        if (validationError.field === 'source.type') {
          sourceErrors.sourceType = validationError.message
        }
        if (validationError.field === 'source.bank') {
          sourceErrors.bank = validationError.message
        }
      }
      if (Object.keys(sourceErrors).length > 0) {
        setFieldErrors((current) => ({ ...current, ...sourceErrors }))
      }
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

  const runAIDraft = () => {
    setFormError('')
    setDraftIssues([])
    draftRule(
      {
        ...buildRulePayload(),
        samples,
      },
      {
        onSuccess: (result) => {
          const draft = result.draft
          const nextIssues = result.validation_issues ?? []
          updateForm({
            name: draft.name || form.name,
            subjectContains: draft.subject_contains,
            amountRegex: draft.amount_regex,
            merchantRegex: draft.merchant_regex,
            currencyRegex: draft.currency_regex,
            sourceType: draft.source.type,
            bank: draft.source.bank,
            senders: draft.sender_emails,
            senderDraft: '',
          })
          if (draft.source.type)
            setCustomTypes((current) => uniqueSorted([...current, draft.source.type]))
          if (draft.source.bank)
            setCustomBanks((current) => uniqueSorted([...current, draft.source.bank]))
          setDraftIssues(nextIssues)
          const issueSampleIndexes = uniqueSorted(
            nextIssues
              .map((issue) => issue.sample_index)
              .filter((sampleIndex) => sampleIndex >= 0 && sampleIndex < samples.length)
              .map(String),
          ).map(Number)
          if (
            samples.length > 1 &&
            issueSampleIndexes.length === 1 &&
            issueSampleIndexes[0] !== activeSample
          ) {
            setActiveSample(issueSampleIndexes[0])
          }
        },
        onError: (error) => setFormError(error.message),
      },
    )
  }

  const handleAIDraft = () => {
    if (aiDraftPrivacyReminderDismissed(privacyReminderUserID, aiProviderID)) {
      runAIDraft()
      return
    }
    setAIPrivacyDialogOpen(true)
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
  const renderSampleTab = (sample: SampleState, index: number) => {
    const issueCount = issueCountsBySample.get(index) ?? 0
    return (
      <span
        key={`${sample.name}-${index}`}
        className={`inline-flex shrink-0 items-center overflow-hidden rounded-full border text-sm font-medium ${
          issueCount > 0 && index === activeSample
            ? 'border-destructive/50 bg-destructive/10 text-foreground'
            : issueCount > 0
              ? 'border-destructive/40 text-destructive hover:bg-destructive/10'
              : index === activeSample
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
          {issueCount > 0 && (
            <span className="ml-2 rounded-full bg-destructive/10 px-1.5 py-0.5 text-[10px] font-semibold text-destructive">
              {issueCount}
            </span>
          )}
        </button>
        <button
          type="button"
          aria-label={t('rules.editor.removeSample', { name: sample.name })}
          onClick={(event) => {
            event.stopPropagation()
            deleteSampleAt(index)
          }}
          className="pr-3 text-muted-foreground transition-colors hover:text-destructive"
        >
          <X aria-hidden="true" size={14} />
        </button>
      </span>
    )
  }

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
            disabled={isPending || drafting}
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
          {formError && (
            <div
              role="alert"
              className="rounded-lg border border-destructive/50 bg-destructive/10 px-3 py-2 text-sm text-destructive"
            >
              {formError}
            </div>
          )}
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
                    className="text-muted-foreground transition-colors hover:text-foreground"
                  >
                    <X aria-hidden="true" size={14} />
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
              <div className="flex items-center gap-2">
                <span className="inline-flex" {...aiTipHandlers(aiDraftTooltip)}>
                  <button
                    type="button"
                    onClick={handleAIDraft}
                    disabled={aiDraftDisabled}
                    aria-label={aiDraftTooltip}
                    className="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-primary/40 text-primary hover:bg-primary/10 disabled:cursor-not-allowed disabled:border-border disabled:text-muted-foreground disabled:opacity-60"
                  >
                    {drafting ? (
                      <LoaderCircle aria-hidden="true" size={15} className="animate-spin" />
                    ) : (
                      <Sparkles aria-hidden="true" size={15} />
                    )}
                  </button>
                </span>
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
              onChange={(value) => {
                updateForm({ sourceType: value })
                setFieldErrors((current) => ({ ...current, sourceType: undefined }))
              }}
              onAdd={(value) => setCustomTypes((current) => uniqueSorted([...current, value]))}
              addLabel={(value) => t('rules.editor.addOption', { value })}
              error={fieldErrors.sourceType}
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
              onChange={(value) => {
                updateForm({ bank: value })
                setFieldErrors((current) => ({ ...current, bank: undefined }))
              }}
              onAdd={(value) => setCustomBanks((current) => uniqueSorted([...current, value]))}
              addLabel={(value) => t('rules.editor.addOption', { value })}
              error={fieldErrors.bank}
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
                {samples.map(renderSampleTab)}
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
                {showLiveStatus && (
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
                          selectedSampleSenderMatches ? 'text-green-500' : 'text-destructive'
                        }
                      >
                        {selectedSampleSenderMatches ? t('common.matches') : t('common.missing')}
                      </dd>
                    </div>
                  )}
                  {selectedSample.subject.trim() !== '' && (
                    <div className="flex items-center justify-between gap-3 py-2">
                      <dt className="text-muted-foreground">{t('common.subject')}</dt>
                      <dd
                        className={
                          selectedSampleSubjectMatches ? 'text-green-500' : 'text-destructive'
                        }
                      >
                        {selectedSampleSubjectMatches ? t('common.matches') : t('common.missing')}
                      </dd>
                    </div>
                  )}
                  {selectedSample.body.trim() !== '' && (
                    <>
                      <div className="flex items-center justify-between gap-3 py-2">
                        <dt className="text-muted-foreground">{t('common.amount')}</dt>
                        <dd>
                          <ResultValue
                            result={live.amount}
                            expected={selectedSample.expected.amount}
                            active={shouldEvaluateExtraction}
                          />
                        </dd>
                      </div>
                      <div className="flex items-center justify-between gap-3 py-2">
                        <dt className="text-muted-foreground">{t('common.merchant')}</dt>
                        <dd>
                          <ResultValue
                            result={live.merchant}
                            expected={selectedSample.expected.merchant}
                            active={shouldEvaluateExtraction}
                          />
                        </dd>
                      </div>
                      <div className="flex items-center justify-between gap-3 py-2">
                        <dt className="text-muted-foreground">{t('common.currency')}</dt>
                        <dd>
                          <ResultValue
                            result={live.currency}
                            expected={selectedSample.expected.currency}
                            active={shouldEvaluateExtraction}
                            optional
                          />
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
      {aiPrivacyDialogOpen && (
        <ConfirmModal
          title={t('rules.editor.aiPrivacyTitle')}
          message={
            <div className="space-y-4">
              <p>{t('rules.editor.aiPrivacyDescription', { provider: aiProviderName })}</p>
              <label className="flex cursor-pointer items-center gap-2 text-foreground">
                <input
                  type="checkbox"
                  checked={skipAIPrivacyReminder}
                  onChange={(event) => setSkipAIPrivacyReminder(event.target.checked)}
                  className="accent-primary"
                />
                <span>{t('rules.editor.aiPrivacyDoNotRemind')}</span>
              </label>
            </div>
          }
          confirmLabel={t('rules.editor.aiPrivacyConfirm')}
          onConfirm={() => {
            if (skipAIPrivacyReminder) {
              void dismissAIDraftPrivacyReminder(privacyReminderUserID, aiProviderID)
            }
            setAIPrivacyDialogOpen(false)
            setSkipAIPrivacyReminder(false)
            runAIDraft()
          }}
          onCancel={() => {
            setAIPrivacyDialogOpen(false)
            setSkipAIPrivacyReminder(false)
          }}
        />
      )}
      {aiTip}
    </div>
  )
}
