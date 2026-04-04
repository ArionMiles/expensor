import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useActiveReader, useCreateRule, useRescan, useRules, useUpdateRule } from '@/api/queries'

// ─── Regex helpers ────────────────────────────────────────────────────────────

interface RegexResult {
  match: string | null
  invalid: boolean
}

function testRegex(pattern: string, body: string): RegexResult {
  if (!pattern) return { match: null, invalid: false }
  try {
    const m = new RegExp(pattern).exec(body)
    return { match: m?.[1] ?? null, invalid: false }
  } catch {
    return { match: null, invalid: true }
  }
}

function ResultCell({ result }: { result: RegexResult }) {
  if (result.invalid) {
    return <span className="text-xs text-warning-foreground">⚠ invalid</span>
  }
  if (result.match !== null) {
    return <span className="font-mono text-xs text-green-500">{result.match} ✓</span>
  }
  return <span className="text-xs text-muted-foreground">—</span>
}

// ─── Form state ───────────────────────────────────────────────────────────────

interface FormState {
  name: string
  senderEmail: string
  subjectContains: string
  amountRegex: string
  merchantRegex: string
  currencyRegex: string
  enabled: boolean
}

const emptyForm: FormState = {
  name: '',
  senderEmail: '',
  subjectContains: '',
  amountRegex: '',
  merchantRegex: '',
  currencyRegex: '',
  enabled: true,
}

// ─── Rule form page ───────────────────────────────────────────────────────────

export function RuleForm() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const isCreate = !id

  const { data: rules = [], isLoading: rulesLoading } = useRules()
  const rule = id ? rules.find((r) => r.id === id) : null

  const { data: activeReader = '' } = useActiveReader()

  const [form, setForm] = useState<FormState>(emptyForm)
  const [samples, setSamples] = useState<string[]>([''])
  const [rescan, setRescan] = useState(true)
  const [toast, setToast] = useState('')
  const [formError, setFormError] = useState('')

  const { mutate: createRule, isPending: creating } = useCreateRule()
  const { mutate: updateRule, isPending: updating } = useUpdateRule()
  const { mutate: triggerRescan } = useRescan()

  // Populate form when rule data arrives (edit mode)
  useEffect(() => {
    if (rule) {
      setForm({
        name: rule.name,
        senderEmail: rule.sender_email,
        subjectContains: rule.subject_contains,
        amountRegex: rule.amount_regex,
        merchantRegex: rule.merchant_regex,
        currencyRegex: rule.currency_regex,
        enabled: rule.enabled,
      })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rule?.id])

  const isSystem = rule?.source === 'system'

  const set =
    (k: keyof FormState) => (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
      setForm((f) => ({
        ...f,
        [k]: e.target.type === 'checkbox' ? (e.target as HTMLInputElement).checked : e.target.value,
      }))

  // Live regex results for each sample
  const results = useMemo(
    () =>
      samples.map((body) => ({
        body,
        amount: testRegex(form.amountRegex, body),
        merchant: testRegex(form.merchantRegex, body),
        currency: testRegex(form.currencyRegex, body),
      })),
    [samples, form.amountRegex, form.merchantRegex, form.currencyRegex],
  )

  const addSample = () => setSamples((s) => [...s, ''])
  const removeSample = (i: number) => setSamples((s) => s.filter((_, idx) => idx !== i))
  const updateSample = (i: number, val: string) =>
    setSamples((s) => s.map((v, idx) => (idx === i ? val : v)))

  const handleSubmit = () => {
    setFormError('')
    if (!form.name) {
      setFormError('Name is required')
      return
    }
    if (!isSystem && !form.amountRegex) {
      setFormError('Amount regex is required')
      return
    }
    if (!isSystem && !form.merchantRegex) {
      setFormError('Merchant regex is required')
      return
    }

    const body = isSystem
      ? { enabled: form.enabled }
      : {
          name: form.name,
          sender_email: form.senderEmail,
          subject_contains: form.subjectContains,
          amount_regex: form.amountRegex,
          merchant_regex: form.merchantRegex,
          currency_regex: form.currencyRegex,
          enabled: form.enabled,
        }

    if (isCreate) {
      createRule(body as Parameters<typeof createRule>[0], {
        onSuccess: () => navigate('/rules'),
        onError: (e) => setFormError(e.message),
      })
      return
    }

    updateRule(
      { id: id!, body },
      {
        onSuccess: () => {
          if (!rescan || !activeReader) {
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
        onError: (e) => setFormError(e.message),
      },
    )
  }

  if (!isCreate && rulesLoading) {
    return (
      <div className="mx-auto w-full max-w-2xl px-6 py-6">
        <p className="text-xs text-muted-foreground">Loading…</p>
      </div>
    )
  }

  if (!isCreate && !rule) {
    return (
      <div className="mx-auto w-full max-w-2xl px-6 py-6">
        <p className="text-sm text-destructive">Rule not found.</p>
        <Link to="/rules" className="text-xs text-primary hover:underline">
          ← Back to rules
        </Link>
      </div>
    )
  }

  const isPending = creating || updating

  return (
    <div className="mx-auto w-full max-w-2xl space-y-6 px-6 py-6">
      {/* Breadcrumb */}
      <nav className="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Link to="/rules" className="hover:text-foreground">
          Rules
        </Link>
        <span>›</span>
        <span className="text-foreground">{isCreate ? 'New Rule' : (rule?.name ?? id)}</span>
      </nav>

      {toast && (
        <div className="rounded border border-border bg-secondary px-4 py-3 text-sm text-foreground">
          {toast}
        </div>
      )}

      {isSystem && (
        <p className="rounded border border-border bg-secondary px-3 py-2 text-xs text-muted-foreground">
          System rule — only the enabled toggle can be changed.
        </p>
      )}

      {/* Core fields */}
      <div className="space-y-4">
        {(
          [
            { key: 'name' as const, label: 'Name', required: true },
            { key: 'senderEmail' as const, label: 'Sender email', required: false },
            { key: 'subjectContains' as const, label: 'Subject contains', required: false },
          ] as const
        ).map(({ key, label, required }) => (
          <div key={key}>
            <label className="mb-1 block text-xs uppercase tracking-wider text-muted-foreground">
              {label}
              {required && ' *'}
            </label>
            <input
              value={form[key]}
              onChange={set(key)}
              disabled={isSystem}
              className="w-full rounded border border-border bg-input px-2 py-1.5 text-sm disabled:opacity-50"
            />
          </div>
        ))}

        {(
          [
            { key: 'amountRegex' as const, label: 'Amount regex *' },
            { key: 'merchantRegex' as const, label: 'Merchant regex *' },
            { key: 'currencyRegex' as const, label: 'Currency regex' },
          ] as const
        ).map(({ key, label }) => (
          <div key={key}>
            <label className="mb-1 block text-xs uppercase tracking-wider text-muted-foreground">
              {label}
            </label>
            <input
              value={form[key]}
              onChange={set(key)}
              disabled={isSystem}
              className="w-full rounded border border-border bg-input px-2 py-1.5 font-mono text-xs disabled:opacity-50"
            />
          </div>
        ))}

        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            id="rule-enabled"
            checked={form.enabled}
            onChange={set('enabled')}
          />
          <label htmlFor="rule-enabled" className="text-sm text-foreground">
            Enabled
          </label>
        </div>
      </div>

      {/* Multi-sample regex tester */}
      <div className="space-y-3 rounded-lg border border-border p-4">
        <div className="flex items-center justify-between">
          <h3 className="text-xs uppercase tracking-wider text-muted-foreground">Test Regexes</h3>
          <button
            type="button"
            onClick={addSample}
            className="text-xs text-primary hover:underline"
          >
            + Add sample
          </button>
        </div>

        {samples.map((body, i) => (
          <div key={i} className="flex items-start gap-2">
            <textarea
              value={body}
              onChange={(e) => updateSample(i, e.target.value)}
              placeholder="Paste an email body here…"
              rows={3}
              className="flex-1 rounded border border-border bg-input px-2 py-1.5 font-mono text-xs"
            />
            {samples.length > 1 && (
              <button
                type="button"
                onClick={() => removeSample(i)}
                className="mt-1 text-xs text-muted-foreground hover:text-foreground"
                aria-label="Remove sample"
              >
                ✕
              </button>
            )}
          </div>
        ))}

        {/* Results table */}
        {samples.some((s) => s.trim()) && (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-border">
                  <th className="py-1 pr-3 text-left text-muted-foreground">#</th>
                  <th className="py-1 pr-3 text-left text-muted-foreground">Preview</th>
                  <th className="py-1 pr-3 text-left text-muted-foreground">Amount</th>
                  <th className="py-1 pr-3 text-left text-muted-foreground">Merchant</th>
                  <th className="py-1 text-left text-muted-foreground">Currency</th>
                </tr>
              </thead>
              <tbody>
                {results.map((r, i) => (
                  <tr key={i} className="border-b border-border/50 last:border-0">
                    <td className="py-1.5 pr-3 text-muted-foreground">{i + 1}</td>
                    <td className="max-w-[160px] truncate py-1.5 pr-3 font-mono text-muted-foreground">
                      {r.body.slice(0, 60) || '—'}
                    </td>
                    <td className="py-1.5 pr-3">
                      <ResultCell result={r.amount} />
                    </td>
                    <td className="py-1.5 pr-3">
                      <ResultCell result={r.merchant} />
                    </td>
                    <td className="py-1.5">
                      <ResultCell result={r.currency} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Retroactive scan (edit mode only) */}
      {!isCreate && (
        <div className="flex items-start gap-2 rounded-lg border border-border bg-secondary/40 px-4 py-3">
          <input
            type="checkbox"
            id="rescan"
            checked={rescan}
            onChange={(e) => setRescan(e.target.checked)}
            className="mt-0.5"
          />
          <label htmlFor="rescan" className="text-sm text-foreground">
            Retroactive scan — re-process emails from the lookback window
            <p className="mt-0.5 text-xs text-muted-foreground">
              Previously processed emails will be re-extracted using the updated regexes and their
              transaction records updated. If the daemon is running, the scan will be queued for the
              next daemon start.
            </p>
          </label>
        </div>
      )}

      {formError && <p className="text-xs text-destructive">{formError}</p>}

      {/* Save / Cancel */}
      <div className="flex gap-2">
        <button
          onClick={handleSubmit}
          disabled={isPending}
          className="rounded bg-primary px-4 py-2 text-sm text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
        >
          {isPending ? 'Saving…' : 'Save rule'}
        </button>
        <Link
          to="/rules"
          className="rounded border border-border px-4 py-2 text-sm text-muted-foreground hover:text-foreground"
        >
          Cancel
        </Link>
      </div>
    </div>
  )
}
