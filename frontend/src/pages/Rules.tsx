import { useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useDeleteRule, useImportRules, useRules, useUpdateRule } from '@/api/queries'
import type { Rule, RuleImport } from '@/api/types'

// ─── Client-side export ───────────────────────────────────────────────────────

function downloadRules(rules: Rule[], selectedIds: Set<string>) {
  const toExport: RuleImport[] = rules
    .filter((r) => selectedIds.has(r.id))
    .map((r) => ({
      name: r.name,
      senderEmail: r.sender_email,
      subjectContains: r.subject_contains,
      amountRegex: r.amount_regex,
      merchantInfoRegex: r.merchant_regex,
      currencyRegex: r.currency_regex || undefined,
      enabled: r.enabled,
    }))
  const blob = new Blob([JSON.stringify(toExport, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'expensor-rules.json'
  a.click()
  URL.revokeObjectURL(url)
}

// ─── Rules list page ──────────────────────────────────────────────────────────

export default function Rules() {
  const navigate = useNavigate()
  const { data: rules = [], isLoading } = useRules()
  const { mutate: updateRule } = useUpdateRule()
  const { mutate: deleteRule } = useDeleteRule()
  const { mutate: importRules, isPending: importing } = useImportRules()

  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [importMsg, setImportMsg] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)

  const allSelected = rules.length > 0 && selected.size === rules.length
  const noneSelected = selected.size === 0

  const toggleAll = () => setSelected(allSelected ? new Set() : new Set(rules.map((r) => r.id)))

  const toggleRow = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })

  const bulkEnable = () =>
    rules
      .filter((r) => selected.has(r.id))
      .forEach((r) => updateRule({ id: r.id, body: { enabled: true } }))

  const bulkDisable = () =>
    rules
      .filter((r) => selected.has(r.id))
      .forEach((r) => updateRule({ id: r.id, body: { enabled: false } }))

  const bulkDelete = () => {
    const userRules = rules.filter((r) => selected.has(r.id) && r.source === 'user')
    if (userRules.length === 0) return
    if (!confirm(`Delete ${userRules.length} user rule${userRules.length !== 1 ? 's' : ''}?`))
      return
    userRules.forEach((r) =>
      deleteRule(r.id, {
        onSuccess: () =>
          setSelected((s) => {
            const n = new Set(s)
            n.delete(r.id)
            return n
          }),
      }),
    )
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    void file.text().then((text) => {
      let parsed: RuleImport[]
      try {
        parsed = JSON.parse(text) as RuleImport[]
      } catch {
        setImportMsg('Invalid JSON file')
        return
      }
      importRules(parsed, {
        onSuccess: (data) =>
          setImportMsg(`${data.imported} rule${data.imported !== 1 ? 's' : ''} imported`),
        onError: (err) => setImportMsg(err.message),
      })
    })
    e.target.value = ''
  }

  const handleDelete = (r: Rule) => {
    if (!confirm(`Delete rule "${r.name}"?`)) return
    deleteRule(r.id)
  }

  if (isLoading) {
    return (
      <div className="mx-auto w-full max-w-5xl px-6 py-6">
        <p className="text-xs text-muted-foreground">Loading…</p>
      </div>
    )
  }

  const selectedUserRuleCount = rules.filter(
    (r) => selected.has(r.id) && r.source === 'user',
  ).length

  return (
    <div className="mx-auto w-full max-w-5xl space-y-4 px-6 py-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-foreground">Rules</h1>
      </div>

      {/* Action bar */}
      <div className="flex flex-wrap items-center gap-2">
        <Link
          to="/rules/new"
          className="rounded bg-primary px-3 py-1.5 text-xs text-primary-foreground hover:bg-primary/90"
        >
          + New rule
        </Link>
        <button
          onClick={() => downloadRules(rules, selected)}
          disabled={noneSelected}
          className="rounded border border-border px-3 py-1.5 text-xs text-muted-foreground hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
        >
          {noneSelected ? 'Export' : `Export (${selected.size} selected)`}
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
        {importMsg && <span className="text-xs text-muted-foreground">{importMsg}</span>}

        {/* Bulk actions — right side */}
        {!noneSelected && (
          <div className="ml-auto flex items-center gap-2">
            <span className="text-xs text-muted-foreground">{selected.size} selected</span>
            <button
              onClick={bulkEnable}
              className="rounded border border-border px-2 py-1 text-xs text-muted-foreground hover:text-foreground"
            >
              Enable ({selected.size})
            </button>
            <button
              onClick={bulkDisable}
              className="rounded border border-border px-2 py-1 text-xs text-muted-foreground hover:text-foreground"
            >
              Disable ({selected.size})
            </button>
            {selectedUserRuleCount > 0 && (
              <button
                onClick={bulkDelete}
                className="rounded border border-destructive/40 px-2 py-1 text-xs text-destructive hover:bg-destructive/10"
              >
                Delete ({selectedUserRuleCount})
              </button>
            )}
          </div>
        )}
      </div>

      {/* Table */}
      <div className="overflow-x-auto rounded-lg border border-border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border bg-secondary">
              <th className="w-10 px-3 py-2">
                <input
                  type="checkbox"
                  checked={allSelected}
                  onChange={toggleAll}
                  aria-label="Select all"
                />
              </th>
              {['Enabled', 'Name', 'Sender', 'Subject', 'Source', '', ''].map((h, i) => (
                <th
                  key={i}
                  className="px-3 py-2 text-left text-xs uppercase tracking-wider text-muted-foreground"
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {rules.map((rule) => (
              <tr
                key={rule.id}
                className={`hover:bg-secondary/50 ${selected.has(rule.id) ? 'bg-secondary/30' : ''}`}
              >
                <td className="px-3 py-2">
                  <input
                    type="checkbox"
                    checked={selected.has(rule.id)}
                    onChange={() => toggleRow(rule.id)}
                    aria-label={`Select ${rule.name}`}
                  />
                </td>
                <td className="px-3 py-2">
                  <span
                    className={`inline-block h-2 w-2 rounded-full ${rule.enabled ? 'bg-green-500' : 'bg-muted-foreground'}`}
                  />
                </td>
                <td className="px-3 py-2 font-medium text-foreground">{rule.name}</td>
                <td className="px-3 py-2 font-mono text-xs text-muted-foreground">
                  {rule.sender_email || '—'}
                </td>
                <td className="max-w-[200px] truncate px-3 py-2 text-xs text-muted-foreground">
                  {rule.subject_contains || '—'}
                </td>
                <td className="px-3 py-2">
                  {rule.source === 'system' ? (
                    <span className="inline-flex shrink-0 items-center gap-1 whitespace-nowrap rounded border border-border px-1.5 py-0.5 text-xs text-muted-foreground">
                      🔒 System
                    </span>
                  ) : (
                    <span className="inline-flex shrink-0 items-center whitespace-nowrap rounded border border-primary/40 px-1.5 py-0.5 text-xs text-primary">
                      User
                    </span>
                  )}
                </td>
                <td className="px-3 py-2">
                  <button
                    onClick={() => navigate(`/rules/${rule.id}`)}
                    className="text-xs text-muted-foreground hover:text-foreground"
                  >
                    Edit
                  </button>
                </td>
                <td className="px-3 py-2">
                  <button
                    onClick={() => handleDelete(rule)}
                    disabled={rule.source === 'system'}
                    title={rule.source === 'system' ? 'System rules cannot be deleted' : undefined}
                    className="text-xs text-destructive hover:underline disabled:cursor-not-allowed disabled:opacity-30"
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
            {rules.length === 0 && (
              <tr>
                <td colSpan={8} className="px-3 py-6 text-center text-xs text-muted-foreground">
                  No rules yet.{' '}
                  <Link to="/rules/new" className="text-primary hover:underline">
                    Create one
                  </Link>
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
