import { useMemo, useRef, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { useDeleteRule, useImportRules, useRules } from '@/api/queries'
import type { Rule, RuleDocument, RuleImport } from '@/api/types'
import { ComboboxListbox, comboboxOptionClass, useComboboxNavigation } from '@/components/Combobox'
import { ConfirmModal } from '@/components/ConfirmModal'
import { useI18n } from '@/i18n/I18nProvider'
import { Trash2 } from 'lucide-react'

type FilterKey = 'type' | 'bank' | 'origin'

type FilterButtonProps = {
  id: FilterKey
  label: string
  listboxLabel: string
  value: string
  options: string[]
  openFilter: FilterKey | null
  allLabel?: string
  optionLabel?: (value: string) => string
  onChange: (value: string) => void
  onOpenChange: (value: FilterKey | null) => void
}

function sourceValue(rule: Rule, key: 'type' | 'bank') {
  return key === 'type' ? rule.source.type : rule.source.bank
}

function uniqueSorted(values: string[]) {
  return [...new Set(values.filter(Boolean))].sort((a, b) => a.localeCompare(b))
}

function FilterButton({
  id,
  label,
  listboxLabel,
  value,
  options,
  openFilter,
  allLabel = 'All',
  optionLabel = (option) => option,
  onChange,
  onOpenChange,
}: FilterButtonProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)
  const open = openFilter === id
  const menuOptions = ['', ...options]
  const navigation = useComboboxNavigation({
    open,
    optionCount: menuOptions.length,
    onOpenChange: (nextOpen) => onOpenChange(nextOpen ? id : null),
    onSelectIndex: (index) => {
      const selected = menuOptions[index]
      if (selected !== undefined) selectValue(selected)
    },
  })
  const highlighted = navigation.highlightedIndex

  const toggle = () => {
    if (open) {
      onOpenChange(null)
      return
    }
    navigation.resetHighlight()
    onOpenChange(id)
  }

  const selectValue = (nextValue: string) => {
    onChange(nextValue)
    navigation.resetHighlight()
    onOpenChange(null)
  }

  return (
    <div ref={containerRef} className="inline-flex">
      <button
        ref={buttonRef}
        type="button"
        onClick={toggle}
        {...navigation.getComboboxProps({
          'aria-label': `${label}: ${value ? optionLabel(value) : allLabel}`,
          listboxVisible: open,
        })}
        className="inline-flex min-w-36 items-center justify-between gap-3 rounded-lg border border-border bg-background px-3 py-2 text-left text-sm font-medium text-foreground shadow-sm transition-colors hover:bg-secondary"
      >
        <span>
          {label}: {value ? optionLabel(value) : allLabel}
        </span>
        <span
          aria-hidden="true"
          className={`h-2 w-2 rotate-45 border-b-2 border-r-2 border-muted-foreground transition-transform ${open ? 'rotate-[225deg]' : ''}`}
        />
      </button>

      <ComboboxListbox
        open={open}
        anchorRef={buttonRef}
        containerRef={containerRef}
        listboxId={navigation.listboxId}
        label={listboxLabel}
        onOpenChange={(nextOpen) => onOpenChange(nextOpen ? id : null)}
        className="min-w-44 rounded-lg p-1 text-sm text-card-foreground shadow-xl"
        minWidth={176}
      >
        {menuOptions.map((option, index) => (
          <li
            key={option || '__all'}
            {...navigation.getOptionProps(index, {
              selected: value === option,
              onMouseDown: () => selectValue(option),
            })}
            className={comboboxOptionClass(
              index === highlighted,
              value === option,
              'rounded-md px-3 py-2 text-sm',
            )}
          >
            {option ? optionLabel(option) : allLabel}
          </li>
        ))}
      </ComboboxListbox>
    </div>
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
  const { t } = useI18n()
  const { data: rules = [], isLoading } = useRules()
  const { mutate: deleteRule } = useDeleteRule()
  const { mutate: importRules, isPending: importing } = useImportRules()
  const [searchParams, setSearchParams] = useSearchParams()

  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [importMsg, setImportMsg] = useState('')
  const [openFilter, setOpenFilter] = useState<FilterKey | null>(null)
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
    const countKey = deletable.length === 1 ? 'rules.bulkDeleteTitle.one' : 'rules.bulkDeleteTitle'
    const messageKey =
      deletable.length === 1 ? 'rules.bulkDeleteMessage.one' : 'rules.bulkDeleteMessage'
    setConfirmState({
      title: t(countKey, { count: deletable.length }),
      message: t(messageKey, { count: deletable.length }),
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
        setImportMsg(t('rules.importInvalid'))
        return
      }
      importRules(parsed, {
        onSuccess: (data) => {
          const key = data.imported === 1 ? 'rules.imported.one' : 'rules.imported'
          setImportMsg(t(key, { count: data.imported }))
        },
        onError: (err) => setImportMsg(err.message),
      })
    })
    e.target.value = ''
  }

  const handleDelete = (r: Rule) => {
    setConfirmState({
      title: t('rules.deleteRule'),
      message: t('rules.deleteRuleMessage', { name: r.name }),
      onConfirm: () => {
        deleteRule(r.id)
        setConfirmState(null)
      },
    })
  }

  if (isLoading) {
    return (
      <div className="mx-auto w-full max-w-6xl px-6 py-6">
        <p className="text-xs text-muted-foreground">{t('common.loading')}</p>
      </div>
    )
  }

  const selectedDeletableCount = rules.filter((r) => selected.has(r.id) && !r.predefined).length
  const hasActiveFilter = Boolean(filters.q || filters.type || filters.bank || filters.origin)

  return (
    <div className="mx-auto w-full max-w-6xl space-y-4 px-6 py-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-xs font-medium text-muted-foreground">{t('rules.pageTitle')}</p>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight text-foreground">
            {t('rules.pageTitle')}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">{t('rules.listSummary')}</p>
        </div>
        <div aria-label={t('rules.actions')} className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={() => downloadRules(rules, selected)}
            disabled={noneSelected}
            className="rounded-lg border border-border px-3 py-2 text-sm font-semibold text-muted-foreground hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
          >
            {noneSelected ? t('rules.export') : t('rules.exportSelected', { count: selected.size })}
          </button>
          <button
            type="button"
            onClick={() => fileRef.current?.click()}
            disabled={importing}
            className="rounded-lg border border-border px-3 py-2 text-sm font-semibold text-muted-foreground hover:text-foreground disabled:opacity-50"
          >
            {importing ? t('rules.importing') : t('rules.import')}
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
            {t('rules.newRule')}
          </Link>
        </div>
      </div>

      <div className="rounded-xl border border-border bg-card p-3">
        <div className="flex flex-wrap items-center gap-2">
          <input
            type="search"
            aria-label={t('rules.searchAria')}
            value={filters.q}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t('rules.searchPlaceholder')}
            className="min-w-64 flex-1 rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          />
          <FilterButton
            id="type"
            label={t('common.type')}
            listboxLabel={t('rules.typeFilterOptions')}
            value={filters.type}
            options={options.type}
            openFilter={openFilter}
            allLabel={t('common.all')}
            onChange={(value) => setFilter('type', value)}
            onOpenChange={setOpenFilter}
          />
          <FilterButton
            id="bank"
            label={t('common.bank')}
            listboxLabel={t('rules.bankFilterOptions')}
            value={filters.bank}
            options={options.bank}
            openFilter={openFilter}
            allLabel={t('common.all')}
            onChange={(value) => setFilter('bank', value)}
            onOpenChange={setOpenFilter}
          />
          <FilterButton
            id="origin"
            label={t('rules.columns.origin')}
            listboxLabel={t('rules.originFilterOptions')}
            value={filters.origin}
            options={options.origin}
            openFilter={openFilter}
            allLabel={t('common.all')}
            optionLabel={(value) =>
              value === 'predefined' ? t('common.predefined') : t('common.custom')
            }
            onChange={(value) => setFilter('origin', value)}
            onOpenChange={setOpenFilter}
          />

          <div className="ml-auto flex flex-wrap items-center gap-2">
            {!noneSelected && selectedDeletableCount > 0 && (
              <button
                type="button"
                onClick={bulkDelete}
                className="rounded-lg border border-destructive/40 px-3 py-2 text-sm font-semibold text-destructive hover:bg-destructive/10"
              >
                {t('common.delete')} ({selectedDeletableCount})
              </button>
            )}
          </div>
        </div>
        {importMsg && <p className="mt-2 text-xs text-muted-foreground">{importMsg}</p>}
      </div>

      <div className="overflow-hidden rounded-xl border border-border bg-card">
        <table aria-label={t('rules.tableAria')} className="w-full table-fixed text-sm">
          <thead>
            <tr className="border-b border-border bg-secondary/60">
              <td className="w-10 px-3 py-3">
                <input
                  type="checkbox"
                  checked={allSelected}
                  onChange={toggleAll}
                  aria-label={t('rules.selectAll')}
                />
              </td>
              <th
                scope="col"
                className="w-20 px-3 py-3 text-left text-xs uppercase tracking-wider text-muted-foreground"
              >
                {t('common.bank')}
              </th>
              <th
                scope="col"
                className="w-44 px-3 py-3 text-left text-xs uppercase tracking-wider text-muted-foreground"
              >
                {t('common.name')}
              </th>
              <th
                scope="col"
                className="w-48 px-3 py-3 text-left text-xs uppercase tracking-wider text-muted-foreground"
              >
                {t('common.subject')}
              </th>
              <th
                scope="col"
                className="w-56 px-3 py-3 text-left text-xs uppercase tracking-wider text-muted-foreground"
              >
                {t('rules.columns.senders')}
              </th>
              <th
                scope="col"
                className="w-28 px-3 py-3 text-left text-xs uppercase tracking-wider text-muted-foreground"
              >
                {t('common.type')}
              </th>
              <th
                scope="col"
                className="w-24 px-3 py-3 text-left text-xs uppercase tracking-wider text-muted-foreground"
              >
                {t('rules.columns.origin')}
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
                    aria-label={t('rules.selectRule', { name: rule.name })}
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
                      {t('common.predefined')}
                    </span>
                  ) : (
                    <span className="inline-flex shrink-0 items-center gap-1 whitespace-nowrap rounded-full border border-green-500/40 px-2 py-1 text-xs font-medium text-green-500">
                      {t('common.custom')}
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
                        aria-label={t('rules.deleteRuleAria', { name: rule.name })}
                        disabled
                        className="inline-flex h-8 w-8 items-center justify-center rounded-md text-destructive transition-colors hover:bg-destructive/10 hover:ring-1 hover:ring-destructive/30 disabled:cursor-not-allowed disabled:text-destructive/60"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </span>
                  ) : (
                    <button
                      type="button"
                      aria-label={t('rules.deleteRuleAria', { name: rule.name })}
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
                  {hasActiveFilter ? t('rules.empty.filtered') : t('rules.empty.none')}{' '}
                  {!hasActiveFilter && (
                    <Link to="/rules/new" className="text-primary hover:underline">
                      {t('common.createOne')}
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
          {t('rules.deletePredefinedTooltip')}
        </div>
      )}

      {confirmState && (
        <ConfirmModal
          title={confirmState.title}
          message={confirmState.message}
          confirmLabel={t('common.delete')}
          variant="destructive"
          onConfirm={confirmState.onConfirm}
          onCancel={() => setConfirmState(null)}
        />
      )}
    </div>
  )
}
