import { useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Link, useSearchParams } from 'react-router-dom'
import { useDeleteRule, useImportRules, useRules } from '@/api/queries'
import type { Rule, RuleDocument, RuleImport } from '@/api/types'
import { ConfirmModal } from '@/components/ConfirmModal'
import { Trash2 } from 'lucide-react'

type FilterKey = 'type' | 'bank' | 'origin'

type FilterButtonProps = {
  label: string
  value: string
  options: string[]
  allLabel?: string
  onChange: (value: string) => void
}

function sourceValue(rule: Rule, key: 'type' | 'bank') {
  return key === 'type' ? rule.source.type : rule.source.bank
}

function uniqueSorted(values: string[]) {
  return [...new Set(values.filter(Boolean))].sort((a, b) => a.localeCompare(b))
}

function FilterButton({ label, value, options, allLabel = 'All', onChange }: FilterButtonProps) {
  const [rect, setRect] = useState<DOMRect | null>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)
  const open = rect !== null

  const toggle = () => {
    if (open) {
      setRect(null)
      return
    }
    const nextRect = buttonRef.current?.getBoundingClientRect()
    if (nextRect) setRect(nextRect)
  }

  const selectValue = (nextValue: string) => {
    onChange(nextValue)
    setRect(null)
  }

  return (
    <>
      <button
        ref={buttonRef}
        type="button"
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={toggle}
        className="inline-flex min-w-36 items-center justify-between gap-3 rounded-lg border border-border bg-background px-3 py-2 text-left text-sm font-medium text-foreground shadow-sm transition-colors hover:bg-secondary"
      >
        <span>
          {label}: {value || allLabel}
        </span>
        <span
          aria-hidden="true"
          className={`h-2 w-2 rotate-45 border-b-2 border-r-2 border-muted-foreground transition-transform ${open ? 'rotate-[225deg]' : ''}`}
        />
      </button>

      {open &&
        rect &&
        createPortal(
          <div
            role="listbox"
            aria-label={`${label} filter options`}
            className="fixed z-50 min-w-44 rounded-lg border border-border bg-card p-1 text-sm text-card-foreground shadow-xl"
            style={{ top: rect.bottom + 6, left: rect.left, width: Math.max(rect.width, 176) }}
          >
            <button
              type="button"
              role="option"
              aria-selected={value === ''}
              onClick={() => selectValue('')}
              className={`block w-full rounded-md px-3 py-2 text-left hover:bg-secondary ${value === '' ? 'bg-secondary text-foreground' : 'text-muted-foreground'}`}
            >
              {allLabel}
            </button>
            {options.map((option) => (
              <button
                key={option}
                type="button"
                role="option"
                aria-selected={value === option}
                onClick={() => selectValue(option)}
                className={`block w-full rounded-md px-3 py-2 text-left hover:bg-secondary ${value === option ? 'bg-secondary text-foreground' : 'text-muted-foreground'}`}
              >
                {option}
              </button>
            ))}
          </div>,
          document.body,
        )}
    </>
  )
}

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
      source_types: uniqueSorted(toExport.map((rule) => rule.source.type)).map((value) => ({
        value,
        origin: 'custom',
      })),
      banks: uniqueSorted(toExport.map((rule) => rule.source.bank)).map((value) => ({
        value,
        origin: 'custom',
      })),
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

export default function Rules() {
  const { data: rules = [], isLoading } = useRules()
  const { mutate: deleteRule } = useDeleteRule()
  const { mutate: importRules, isPending: importing } = useImportRules()
  const [searchParams, setSearchParams] = useSearchParams()

  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [importMsg, setImportMsg] = useState('')
  const [predefinedTooltip, setPredefinedTooltip] = useState<{ x: number; y: number } | null>(null)
  const fileRef = useRef<HTMLInputElement>(null)
  const [confirmState, setConfirmState] = useState<{
    title: string
    message: string
    onConfirm: () => void
  } | null>(null)

  const filters = {
    q: searchParams.get('q') ?? '',
    type: searchParams.get('type') ?? '',
    bank: searchParams.get('bank') ?? '',
    origin: searchParams.get('origin') ?? '',
  }

  const options = useMemo(
    () => ({
      type: uniqueSorted(rules.map((rule) => sourceValue(rule, 'type'))),
      bank: uniqueSorted(rules.map((rule) => sourceValue(rule, 'bank'))),
      origin: ['predefined', 'custom'],
    }),
    [rules],
  )

  const visibleRules = useMemo(
    () =>
      rules.filter((rule) => {
        const query = filters.q.trim().toLowerCase()
        if (
          query &&
          ![
            rule.name,
            rule.subject_contains,
            rule.source.bank,
            rule.source.type,
            ...rule.sender_emails,
          ].some((value) => value.toLowerCase().includes(query))
        ) {
          return false
        }
        if (filters.type && rule.source.type !== filters.type) return false
        if (filters.bank && rule.source.bank !== filters.bank) return false
        if (filters.origin === 'predefined' && !rule.predefined) return false
        if (filters.origin === 'custom' && rule.predefined) return false
        return true
      }),
    [filters.bank, filters.origin, filters.q, filters.type, rules],
  )

  const allSelected = visibleRules.length > 0 && visibleRules.every((rule) => selected.has(rule.id))
  const noneSelected = selected.size === 0

  const setFilter = (key: FilterKey, value: string) => {
    const next = new URLSearchParams(searchParams)
    if (value) next.set(key, value)
    else next.delete(key)
    setSearchParams(next, { replace: true })
  }

  const setSearch = (value: string) => {
    const next = new URLSearchParams(searchParams)
    if (value) next.set('q', value)
    else next.delete('q')
    setSearchParams(next, { replace: true })
  }

  const toggleAll = () =>
    setSelected((prev) => {
      if (allSelected) {
        const next = new Set(prev)
        visibleRules.forEach((rule) => next.delete(rule.id))
        return next
      }
      return new Set([...prev, ...visibleRules.map((rule) => rule.id)])
    })

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
      <div className="mx-auto w-full max-w-6xl px-6 py-6">
        <p className="text-xs text-muted-foreground">Loading…</p>
      </div>
    )
  }

  const selectedDeletableCount = rules.filter((r) => selected.has(r.id) && !r.predefined).length
  const hasActiveFilter = Boolean(filters.q || filters.type || filters.bank || filters.origin)

  return (
    <div className="mx-auto w-full max-w-6xl space-y-4 px-6 py-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-xs font-medium text-muted-foreground">Rules</p>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight text-foreground">Rules</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Manage extraction rules, source classification, and sender matching.
          </p>
        </div>
        <div aria-label="Rule actions" className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={() => downloadRules(rules, selected)}
            disabled={noneSelected}
            className="rounded-lg border border-border px-3 py-2 text-sm font-semibold text-muted-foreground hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
          >
            {noneSelected ? 'Export' : `Export (${selected.size} selected)`}
          </button>
          <button
            type="button"
            onClick={() => fileRef.current?.click()}
            disabled={importing}
            className="rounded-lg border border-border px-3 py-2 text-sm font-semibold text-muted-foreground hover:text-foreground disabled:opacity-50"
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
          <Link
            to="/rules/new"
            className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground hover:bg-primary/90"
          >
            + New rule
          </Link>
        </div>
      </div>

      <div className="rounded-xl border border-border bg-card p-3">
        <div className="flex flex-wrap items-center gap-2">
          <input
            type="search"
            aria-label="Search rules"
            value={filters.q}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search rules, senders, subjects..."
            className="min-w-64 flex-1 rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          />
          <FilterButton
            label="Type"
            value={filters.type}
            options={options.type}
            onChange={(value) => setFilter('type', value)}
          />
          <FilterButton
            label="Bank"
            value={filters.bank}
            options={options.bank}
            onChange={(value) => setFilter('bank', value)}
          />
          <FilterButton
            label="Origin"
            value={filters.origin}
            options={options.origin}
            onChange={(value) => setFilter('origin', value)}
          />

          <div className="ml-auto flex flex-wrap items-center gap-2">
            {!noneSelected && selectedDeletableCount > 0 && (
              <button
                type="button"
                onClick={bulkDelete}
                className="rounded-lg border border-destructive/40 px-3 py-2 text-sm font-semibold text-destructive hover:bg-destructive/10"
              >
                Delete ({selectedDeletableCount})
              </button>
            )}
          </div>
        </div>
        {importMsg && <p className="mt-2 text-xs text-muted-foreground">{importMsg}</p>}
      </div>

      <div className="overflow-hidden rounded-xl border border-border bg-card">
        <table aria-label="Rules" className="w-full table-fixed text-sm">
          <thead>
            <tr className="border-b border-border bg-secondary/60">
              <td className="w-10 px-3 py-3">
                <input
                  type="checkbox"
                  checked={allSelected}
                  onChange={toggleAll}
                  aria-label="Select all"
                />
              </td>
              <th
                scope="col"
                className="w-20 px-3 py-3 text-left text-xs uppercase tracking-wider text-muted-foreground"
              >
                Bank
              </th>
              <th
                scope="col"
                className="w-44 px-3 py-3 text-left text-xs uppercase tracking-wider text-muted-foreground"
              >
                Name
              </th>
              <th
                scope="col"
                className="w-48 px-3 py-3 text-left text-xs uppercase tracking-wider text-muted-foreground"
              >
                Subject
              </th>
              <th
                scope="col"
                className="w-56 px-3 py-3 text-left text-xs uppercase tracking-wider text-muted-foreground"
              >
                Senders
              </th>
              <th
                scope="col"
                className="w-28 px-3 py-3 text-left text-xs uppercase tracking-wider text-muted-foreground"
              >
                Type
              </th>
              <th
                scope="col"
                className="w-24 px-3 py-3 text-left text-xs uppercase tracking-wider text-muted-foreground"
              >
                Origin
              </th>
              <th
                scope="col"
                className="w-12 px-2 py-3 text-center text-xs uppercase tracking-wider text-muted-foreground"
              />
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {visibleRules.map((rule) => (
              <tr
                key={rule.id}
                className={`hover:bg-secondary/50 ${selected.has(rule.id) ? 'bg-secondary/30' : ''}`}
              >
                <td className="px-3 py-3">
                  <input
                    type="checkbox"
                    checked={selected.has(rule.id)}
                    onChange={() => toggleRow(rule.id)}
                    aria-label={`Select ${rule.name}`}
                  />
                </td>
                <td className="px-3 py-3 font-medium text-foreground">{rule.source.bank || '—'}</td>
                <td className="px-3 py-3 font-medium">
                  <Link
                    to={`/rules/${rule.id}`}
                    className="inline-block max-w-full truncate rounded-md px-1 py-1 text-left text-sm font-semibold text-foreground transition-colors hover:bg-accent"
                  >
                    {rule.name}
                  </Link>
                </td>
                <td className="truncate px-3 py-3 text-xs text-muted-foreground">
                  {rule.subject_contains || '—'}
                </td>
                <td className="px-3 py-3">
                  <div className="flex flex-wrap gap-1.5">
                    {rule.sender_emails.length > 0 ? (
                      rule.sender_emails.map((sender) => (
                        <span
                          key={sender}
                          className="rounded-full border border-border bg-background px-2 py-1 font-mono text-[11px] text-muted-foreground"
                        >
                          {sender}
                        </span>
                      ))
                    ) : (
                      <span className="text-xs text-muted-foreground">—</span>
                    )}
                  </div>
                </td>
                <td className="px-3 py-3 text-sm text-muted-foreground">
                  {rule.source.type || '—'}
                </td>
                <td className="px-3 py-3">
                  {rule.predefined ? (
                    <span className="inline-flex shrink-0 items-center gap-1 whitespace-nowrap rounded-full border border-primary/40 px-2 py-1 text-xs font-medium text-primary">
                      Predefined
                    </span>
                  ) : (
                    <span className="inline-flex shrink-0 items-center gap-1 whitespace-nowrap rounded-full border border-green-500/40 px-2 py-1 text-xs font-medium text-green-500">
                      Custom
                    </span>
                  )}
                </td>
                <td className="px-2 py-3 text-center">
                  {rule.predefined ? (
                    <span
                      className="inline-flex cursor-not-allowed"
                      onMouseEnter={(e) => {
                        const deleteRect = e.currentTarget.getBoundingClientRect()
                        setPredefinedTooltip({
                          x: deleteRect.left + deleteRect.width / 2,
                          y: deleteRect.top - 6,
                        })
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
            {visibleRules.length === 0 && (
              <tr>
                <td colSpan={8} className="px-3 py-8 text-center text-xs text-muted-foreground">
                  {hasActiveFilter ? 'No rules match these filters.' : 'No rules yet.'}{' '}
                  {!hasActiveFilter && (
                    <Link to="/rules/new" className="text-primary hover:underline">
                      Create one
                    </Link>
                  )}
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
