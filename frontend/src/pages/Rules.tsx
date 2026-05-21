import { useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { useDeleteRule, useImportRules, useRules } from '@/api/queries'
import type { Rule, RuleDocument, RuleImport } from '@/api/types'
import { ConfirmModal } from '@/components/ConfirmModal'
import { Trash2 } from 'lucide-react'

// ─── Client-side export ───────────────────────────────────────────────────────

function downloadRules(rules: Rule[], selectedIds: Set<string>) {
  const toExport: RuleImport[] = rules
    .filter((r) => selectedIds.has(r.id))
    .map((r) => ({
      name: r.name,
      sender_emails: r.sender_emails,
      subject_contains: r.subject_contains,
      amount_regex: r.amount_regex,
      merchant_regex: r.merchant_regex,
      currency_regex: r.currency_regex || '',
      source: r.source,
    }))
  const doc: RuleDocument = {
    version: 2,
    presets: {
      source_types: [...new Set(toExport.map((rule) => rule.source.type).filter(Boolean))].map(
        (value) => ({ value, origin: 'custom' }),
      ),
      banks: [...new Set(toExport.map((rule) => rule.source.bank).filter(Boolean))].map(
        (value) => ({ value, origin: 'custom' }),
      ),
    },
    rules: toExport,
  }
  const blob = new Blob([JSON.stringify(doc, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'expensor-rules.json'
  a.click()
  URL.revokeObjectURL(url)
}

// ─── Rules list page ──────────────────────────────────────────────────────────

export default function Rules() {
  const { data: rules = [], isLoading } = useRules()
  const { mutate: deleteRule } = useDeleteRule()
  const { mutate: importRules, isPending: importing } = useImportRules()

  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [importMsg, setImportMsg] = useState('')
  const [predefinedTooltip, setPredefinedTooltip] = useState<{ x: number; y: number } | null>(null)
  const fileRef = useRef<HTMLInputElement>(null)
  const [confirmState, setConfirmState] = useState<{
    title: string
    message: string
    onConfirm: () => void
  } | null>(null)

  const allSelected = rules.length > 0 && selected.size === rules.length
  const noneSelected = selected.size === 0

  const toggleAll = () => setSelected(allSelected ? new Set() : new Set(rules.map((r) => r.id)))

  const toggleRow = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })

  const bulkDelete = () => {
    const deletable = rules.filter((r) => selected.has(r.id) && !r.predefined)
    if (deletable.length === 0) return
    setConfirmState({
      title: `Delete ${deletable.length} rule${deletable.length !== 1 ? 's' : ''}`,
      message: `Delete ${deletable.length} rule${deletable.length !== 1 ? 's' : ''}? This cannot be undone.`,
      onConfirm: () => {
        deletable.forEach((r) =>
          deleteRule(r.id, {
            onSuccess: () =>
              setSelected((s) => {
                const n = new Set(s)
                n.delete(r.id)
                return n
              }),
          }),
        )
        setConfirmState(null)
      },
    })
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    void file.text().then((text) => {
      let parsed: RuleDocument
      try {
        parsed = JSON.parse(text) as RuleDocument
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
    setConfirmState({
      title: 'Delete rule',
      message: `Delete rule "${r.name}"? This cannot be undone.`,
      onConfirm: () => {
        deleteRule(r.id)
        setConfirmState(null)
      },
    })
  }

  if (isLoading) {
    return (
      <div className="mx-auto w-full max-w-5xl px-6 py-6">
        <p className="text-xs text-muted-foreground">Loading…</p>
      </div>
    )
  }

  const selectedDeletableCount = rules.filter((r) => selected.has(r.id) && !r.predefined).length

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

        {/* Bulk delete — right side */}
        {!noneSelected && selectedDeletableCount > 0 && (
          <div className="ml-auto flex items-center gap-2">
            <span className="text-xs text-muted-foreground">{selected.size} selected</span>
            <button
              onClick={bulkDelete}
              className="rounded border border-destructive/40 px-2 py-1 text-xs text-destructive hover:bg-destructive/10"
            >
              Delete ({selectedDeletableCount})
            </button>
          </div>
        )}
      </div>

      {/* Table */}
      <div className="overflow-x-auto rounded-lg border border-border">
        <table aria-label="Rules" className="w-full table-fixed text-sm">
          <thead>
            <tr className="border-b border-border bg-secondary">
              <th scope="col" className="w-10 px-3 py-2">
                <input
                  type="checkbox"
                  checked={allSelected}
                  onChange={toggleAll}
                  aria-label="Select all"
                />
              </th>
              <th
                scope="col"
                className="px-3 py-2 text-left text-xs uppercase tracking-wider text-muted-foreground"
              >
                Name
              </th>
              <th
                scope="col"
                className="w-48 px-3 py-2 text-left text-xs uppercase tracking-wider text-muted-foreground"
              >
                Sender
              </th>
              <th
                scope="col"
                className="w-44 px-3 py-2 text-left text-xs uppercase tracking-wider text-muted-foreground"
              >
                Subject
              </th>
              <th
                scope="col"
                className="w-24 px-3 py-2 text-left text-xs uppercase tracking-wider text-muted-foreground"
              >
                Type
              </th>
              <th
                scope="col"
                className="w-12 px-2 py-2 text-center text-xs uppercase tracking-wider text-muted-foreground"
              />
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
                <td className="px-3 py-2 font-medium">
                  <Link
                    to={`/rules/${rule.id}`}
                    className="inline-block max-w-full truncate rounded-md px-1 py-1 text-left text-sm font-medium text-foreground transition-colors hover:bg-accent"
                  >
                    {rule.name}
                  </Link>
                </td>
                <td className="truncate px-3 py-2 font-mono text-xs text-muted-foreground">
                  {rule.sender_emails.join(', ') || '—'}
                </td>
                <td className="truncate px-3 py-2 text-xs text-muted-foreground">
                  {rule.subject_contains || '—'}
                </td>
                <td className="px-3 py-2">
                  {rule.predefined ? (
                    <span className="inline-flex shrink-0 items-center gap-1 whitespace-nowrap rounded border border-border px-1.5 py-0.5 text-xs text-muted-foreground">
                      Predefined
                    </span>
                  ) : (
                    <span className="inline-flex shrink-0 items-center gap-1 whitespace-nowrap rounded border border-primary/40 px-1.5 py-0.5 text-xs text-primary">
                      Custom
                    </span>
                  )}
                </td>
                <td className="px-2 py-2.5 text-center">
                  {rule.predefined ? (
                    <span
                      className="inline-flex cursor-not-allowed"
                      onMouseEnter={(e) => {
                        const rect = e.currentTarget.getBoundingClientRect()
                        setPredefinedTooltip({ x: rect.left + rect.width / 2, y: rect.top - 6 })
                      }}
                      onMouseLeave={() => setPredefinedTooltip(null)}
                    >
                      <button
                        type="button"
                        aria-label={`Delete ${rule.name}`}
                        disabled
                        className="inline-flex h-8 w-8 items-center justify-center rounded-md text-destructive transition-colors hover:bg-destructive/10 hover:ring-1 hover:ring-destructive/30 disabled:cursor-not-allowed disabled:text-destructive/60"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </span>
                  ) : (
                    <button
                      type="button"
                      aria-label={`Delete ${rule.name}`}
                      onClick={() => handleDelete(rule)}
                      className="inline-flex h-8 w-8 items-center justify-center rounded-md text-destructive transition-colors hover:bg-destructive/10 hover:ring-1 hover:ring-destructive/30"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  )}
                </td>
              </tr>
            ))}
            {rules.length === 0 && (
              <tr>
                <td colSpan={6} className="px-3 py-6 text-center text-xs text-muted-foreground">
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

      {predefinedTooltip && (
        <div
          className="pointer-events-none fixed z-50 -translate-x-1/2 -translate-y-full whitespace-nowrap rounded bg-foreground px-2 py-1 text-xs text-background shadow-md"
          style={{ left: predefinedTooltip.x, top: predefinedTooltip.y }}
        >
          Predefined rules cannot be deleted
        </div>
      )}

      {confirmState && (
        <ConfirmModal
          title={confirmState.title}
          message={confirmState.message}
          confirmLabel="Delete"
          variant="destructive"
          onConfirm={confirmState.onConfirm}
          onCancel={() => setConfirmState(null)}
        />
      )}
    </div>
  )
}
