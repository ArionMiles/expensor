import { useRef, useState } from 'react'
import {
  useCreateRule,
  useDeleteRule,
  useImportRules,
  useRules,
  useUpdateRule,
} from '@/api/queries'
import type { Rule, RuleImport } from '@/api/types'

// ─── Regex live tester ────────────────────────────────────────────────────────

function RegexTester({ pattern }: { pattern: string }) {
  const [sample, setSample] = useState('')

  let result: boolean | 'invalid' | null = null
  if (sample && pattern) {
    try {
      result = new RegExp(pattern).test(sample)
    } catch {
      result = 'invalid'
    }
  }

  return (
    <div className="mt-1">
      <input
        value={sample}
        onChange={(e) => setSample(e.target.value)}
        placeholder="Test against sample…"
        className="w-full rounded border border-border bg-input px-2 py-1 text-xs"
      />
      {result !== null && (
        <p
          className={`mt-0.5 text-xs ${
            result === true
              ? 'text-green-500'
              : result === false
                ? 'text-destructive'
                : 'text-warning-foreground'
          }`}
        >
          {result === true ? '✓ matches' : result === false ? '✗ no match' : 'Invalid regex'}
        </p>
      )}
    </div>
  )
}

// ─── Rule slide-over form ─────────────────────────────────────────────────────

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

function ruleToForm(r: Rule): FormState {
  return {
    name: r.name,
    senderEmail: r.sender_email,
    subjectContains: r.subject_contains,
    amountRegex: r.amount_regex,
    merchantRegex: r.merchant_regex,
    currencyRegex: r.currency_regex,
    enabled: r.enabled,
  }
}

interface SlideOverProps {
  open: boolean
  editRule: Rule | null
  onClose: () => void
}

function RuleSlideOver({ open, editRule, onClose }: SlideOverProps) {
  const [form, setForm] = useState<FormState>(editRule ? ruleToForm(editRule) : emptyForm)
  const [error, setError] = useState('')
  const { mutate: createRule, isPending: creating } = useCreateRule()
  const { mutate: updateRule, isPending: updating } = useUpdateRule()

  const isSystem = editRule?.source === 'system'

  const set = (k: keyof FormState) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setForm((f) => ({
      ...f,
      [k]: e.target.type === 'checkbox' ? e.target.checked : e.target.value,
    }))

  const handleSubmit = () => {
    setError('')
    if (!form.name) {
      setError('Name is required')
      return
    }
    if (!isSystem && !form.amountRegex) {
      setError('Amount regex is required')
      return
    }
    if (!isSystem && !form.merchantRegex) {
      setError('Merchant regex is required')
      return
    }

    if (editRule) {
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
      updateRule(
        { id: editRule.id, body },
        { onSuccess: onClose, onError: (e) => setError(e.message) },
      )
    } else {
      createRule(
        {
          name: form.name,
          sender_email: form.senderEmail,
          subject_contains: form.subjectContains,
          amount_regex: form.amountRegex,
          merchant_regex: form.merchantRegex,
          currency_regex: form.currencyRegex,
          enabled: form.enabled,
        },
        { onSuccess: onClose, onError: (e) => setError(e.message) },
      )
    }
  }

  return (
    <>
      {open && <div className="fixed inset-0 z-40 bg-black/40" onClick={onClose} />}
      <div
        className={`fixed inset-y-0 right-0 z-50 flex w-full max-w-md flex-col border-l border-border bg-card shadow-xl transition-transform duration-200 ${open ? 'translate-x-0' : 'translate-x-full'}`}
      >
        <div className="flex items-center justify-between border-b border-border px-6 py-4">
          <h2 className="text-sm font-semibold text-foreground">
            {editRule ? 'Edit Rule' : 'New Rule'}
          </h2>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            ✕
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-6 py-4">
          {isSystem && (
            <p className="mb-4 rounded border border-border bg-secondary px-3 py-2 text-xs text-muted-foreground">
              System rule — only the enabled toggle can be changed.
            </p>
          )}

          <div className="space-y-4">
            {(
              [
                { key: 'name' as const, label: 'Name', required: true, disabled: isSystem },
                {
                  key: 'senderEmail' as const,
                  label: 'Sender email',
                  required: false,
                  disabled: isSystem,
                },
                {
                  key: 'subjectContains' as const,
                  label: 'Subject contains',
                  required: false,
                  disabled: isSystem,
                },
              ] as const
            ).map(({ key, label, required, disabled }) => (
              <div key={key}>
                <label className="mb-1 block text-xs uppercase tracking-wider text-muted-foreground">
                  {label}
                  {required === true && ' *'}
                </label>
                <input
                  value={form[key]}
                  onChange={set(key)}
                  disabled={disabled}
                  className="w-full rounded border border-border bg-input px-2 py-1.5 text-sm disabled:opacity-50"
                />
              </div>
            ))}

            {(
              [
                { key: 'amountRegex' as const, label: 'Amount regex *', disabled: isSystem },
                { key: 'merchantRegex' as const, label: 'Merchant regex *', disabled: isSystem },
                { key: 'currencyRegex' as const, label: 'Currency regex', disabled: isSystem },
              ] as const
            ).map(({ key, label, disabled }) => (
              <div key={key}>
                <label className="mb-1 block text-xs uppercase tracking-wider text-muted-foreground">
                  {label}
                </label>
                <input
                  value={form[key]}
                  onChange={set(key)}
                  disabled={disabled}
                  className="w-full rounded border border-border bg-input px-2 py-1.5 font-mono text-xs disabled:opacity-50"
                />
                {!disabled && <RegexTester pattern={form[key]} />}
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
        </div>

        {error && <p className="px-6 pb-2 text-xs text-destructive">{error}</p>}

        <div className="flex justify-end gap-2 border-t border-border px-6 py-4">
          <button
            onClick={onClose}
            className="rounded border border-border px-3 py-1.5 text-xs text-muted-foreground hover:text-foreground"
          >
            Cancel
          </button>
          <button
            onClick={handleSubmit}
            disabled={creating || updating}
            className="rounded bg-primary px-3 py-1.5 text-xs text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
          >
            {creating || updating ? 'Saving…' : 'Save'}
          </button>
        </div>
      </div>
    </>
  )
}

// ─── Main component ───────────────────────────────────────────────────────────

export function RulesSettings() {
  const { data: rules = [], isLoading } = useRules()
  const { mutate: deleteRule } = useDeleteRule()
  const { mutate: importRules, isPending: importing } = useImportRules()
  const [panelOpen, setPanelOpen] = useState(false)
  const [editRule, setEditRule] = useState<Rule | null>(null)
  const [statusMsg, setStatusMsg] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)

  const openCreate = () => {
    setEditRule(null)
    setPanelOpen(true)
  }

  const openEdit = (r: Rule) => {
    setEditRule(r)
    setPanelOpen(true)
  }

  const closePanel = () => setPanelOpen(false)

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    void file.text().then((text) => {
      let parsed: RuleImport[]
      try {
        parsed = JSON.parse(text) as RuleImport[]
      } catch {
        setStatusMsg('Invalid JSON file')
        return
      }
      importRules(parsed, {
        onSuccess: (data) =>
          setStatusMsg(`${data.imported} rule${data.imported !== 1 ? 's' : ''} imported`),
        onError: (err) => setStatusMsg(err.message),
      })
    })
    e.target.value = ''
  }

  const handleExport = () => {
    window.location.href = '/api/rules/export'
  }

  const handleDelete = (r: Rule) => {
    if (!confirm(`Delete rule "${r.name}"?`)) return
    deleteRule(r.id)
  }

  if (isLoading) return <p className="text-xs text-muted-foreground">Loading…</p>

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <button
          onClick={openCreate}
          className="rounded bg-primary px-3 py-1.5 text-xs text-primary-foreground hover:bg-primary/90"
        >
          + New rule
        </button>
        <button
          onClick={handleExport}
          className="rounded border border-border px-3 py-1.5 text-xs text-muted-foreground hover:text-foreground"
        >
          Export
        </button>
        <button
          onClick={() => fileRef.current?.click()}
          disabled={importing}
          className="rounded border border-border px-3 py-1.5 text-xs text-muted-foreground hover:text-foreground disabled:opacity-50"
        >
          {importing ? 'Importing…' : 'Import'}
        </button>
        <input
          ref={fileRef}
          type="file"
          accept=".json"
          className="hidden"
          onChange={handleFileChange}
        />
        {statusMsg && <span className="text-xs text-muted-foreground">{statusMsg}</span>}
      </div>

      <p className="text-xs text-muted-foreground">
        Rule changes apply on the next daemon restart.
      </p>

      <div className="overflow-x-auto rounded-lg border border-border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border bg-secondary">
              {['Name', 'Sender', 'Subject', 'Enabled', 'Source', ''].map((h) => (
                <th
                  key={h}
                  className="px-3 py-2 text-left text-xs uppercase tracking-wider text-muted-foreground"
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {rules.map((rule) => (
              <tr key={rule.id} className="hover:bg-secondary/50">
                <td className="px-3 py-2 font-medium text-foreground">{rule.name}</td>
                <td className="px-3 py-2 font-mono text-xs text-muted-foreground">
                  {rule.sender_email || '—'}
                </td>
                <td className="max-w-[180px] truncate px-3 py-2 text-xs text-muted-foreground">
                  {rule.subject_contains || '—'}
                </td>
                <td className="px-3 py-2">
                  <span
                    className={`inline-block h-2 w-2 rounded-full ${rule.enabled ? 'bg-green-500' : 'bg-muted-foreground'}`}
                  />
                </td>
                <td className="px-3 py-2">
                  {rule.source === 'system' ? (
                    <span className="rounded border border-border px-1.5 py-0.5 text-xs text-muted-foreground">
                      🔒 System
                    </span>
                  ) : (
                    <span className="rounded border border-primary/40 px-1.5 py-0.5 text-xs text-primary">
                      User
                    </span>
                  )}
                </td>
                <td className="px-3 py-2">
                  <div className="flex items-center gap-3">
                    <button
                      onClick={() => openEdit(rule)}
                      className="text-xs text-muted-foreground hover:text-foreground"
                    >
                      Edit
                    </button>
                    {rule.source === 'user' && (
                      <button
                        onClick={() => handleDelete(rule)}
                        className="text-xs text-destructive hover:underline"
                      >
                        Delete
                      </button>
                    )}
                  </div>
                </td>
              </tr>
            ))}
            {rules.length === 0 && (
              <tr>
                <td colSpan={6} className="px-3 py-6 text-center text-xs text-muted-foreground">
                  No rules yet
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <RuleSlideOver open={panelOpen} editRule={editRule} onClose={closePanel} />
    </div>
  )
}
