import {
  useApplyBucket,
  useApplyCategory,
  useApplyLabel,
  useBucketMappings,
  useBuckets,
  useCategories,
  useCategoryMappings,
  useCreateBucket,
  useCreateCategory,
  useCreateLabel,
  useDeleteBucket,
  useDeleteCategory,
  useDeleteLabel,
  useFacets,
  useLabelMappings,
  useLabels,
  useRemoveBucketByMerchant,
  useRemoveCategoryByMerchant,
  useRemoveLabelByMerchant,
  useUpdateLabel,
} from '@/api/queries'
import { useI18n } from '@/i18n/I18nProvider'
import { cn } from '@/lib/utils'
import {
  ArrowDown,
  ArrowUp,
  ArrowUpDown,
  Check,
  FolderOpen,
  Layers,
  Search,
  Tag,
  Trash2,
  X,
  type LucideIcon,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate, useSearchParams } from 'react-router-dom'

type GroupTab = 'categories' | 'buckets' | 'labels'
type Decision = 'keep' | 'overwrite'
type SortField = 'merchants' | 'transactions'
type SortDirection = 'asc' | 'desc'

type GroupItem = {
  name: string
  color?: string
  isDefault?: boolean
  transactionCount: number
  merchants: string[]
}

type TabMeta = {
  id: GroupTab
  icon: LucideIcon
  labelKey:
    | 'expenseGroups.tabs.categories'
    | 'expenseGroups.tabs.buckets'
    | 'expenseGroups.tabs.labels'
  singularKey:
    | 'expenseGroups.singular.category'
    | 'expenseGroups.singular.bucket'
    | 'expenseGroups.singular.label'
  newPlaceholderKey:
    | 'expenseGroups.placeholders.category'
    | 'expenseGroups.placeholders.bucket'
    | 'expenseGroups.placeholders.label'
  exportUrl: string
  exportName: string
}

type ImportConflict = {
  imported: GroupItem[]
  conflicts: string[]
  decisions: Record<string, Decision>
  fileName: string
}

type ColorPickerState =
  | { kind: 'new'; rect: DOMRect }
  | { kind: 'item'; name: string; rect: DOMRect }
  | null

type MerchantSuggestState = { rect: DOMRect; value: string } | null

const TABS: TabMeta[] = [
  {
    id: 'categories',
    icon: FolderOpen,
    labelKey: 'expenseGroups.tabs.categories',
    singularKey: 'expenseGroups.singular.category',
    newPlaceholderKey: 'expenseGroups.placeholders.category',
    exportUrl: '/api/config/categories/export',
    exportName: 'expensor-categories.json',
  },
  {
    id: 'buckets',
    icon: Layers,
    labelKey: 'expenseGroups.tabs.buckets',
    singularKey: 'expenseGroups.singular.bucket',
    newPlaceholderKey: 'expenseGroups.placeholders.bucket',
    exportUrl: '/api/config/buckets/export',
    exportName: 'expensor-buckets.json',
  },
  {
    id: 'labels',
    icon: Tag,
    labelKey: 'expenseGroups.tabs.labels',
    singularKey: 'expenseGroups.singular.label',
    newPlaceholderKey: 'expenseGroups.placeholders.label',
    exportUrl: '/api/config/labels/export',
    exportName: 'expensor-labels.json',
  },
]

const PRESET_COLORS = [
  '#f59e0b',
  '#3b82f6',
  '#8b5cf6',
  '#06b6d4',
  '#10b981',
  '#ec4899',
  '#f97316',
  '#6366f1',
]

function isTab(value: string | null): value is GroupTab {
  return value === 'categories' || value === 'buckets' || value === 'labels'
}

function isSortField(value: string | null): value is SortField {
  return value === 'merchants' || value === 'transactions'
}

function isSortDirection(value: string | null): value is SortDirection {
  return value === 'asc' || value === 'desc'
}

function parseImportRows(payload: unknown, tab: GroupTab): GroupItem[] {
  const rows = Array.isArray(payload)
    ? payload
    : payload &&
        typeof payload === 'object' &&
        Array.isArray((payload as Record<string, unknown>)[tab])
      ? ((payload as Record<string, unknown>)[tab] as unknown[])
      : null
  if (!rows) throw new Error('Invalid import file')

  return rows
    .filter((row): row is Record<string, unknown> => Boolean(row) && typeof row === 'object')
    .map((row) => ({
      name: typeof row.name === 'string' ? row.name.trim() : '',
      color: tab === 'labels' && typeof row.color === 'string' ? row.color : undefined,
      merchants: Array.isArray(row.merchants)
        ? row.merchants.filter((merchant): merchant is string => typeof merchant === 'string')
        : [],
      transactionCount: 0,
    }))
    .filter((row) => row.name)
}

function readFileText(file: File) {
  if (typeof file.text === 'function') return file.text()
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result ?? ''))
    reader.onerror = () => reject(reader.error)
    reader.readAsText(file)
  })
}

function colorPickerKey(state: ColorPickerState) {
  if (!state) return ''
  return state.kind === 'new' ? 'new' : `item:${state.name}`
}

function SortHeaderButton({
  active,
  direction,
  label,
  onClick,
}: {
  active: boolean
  direction: SortDirection
  label: string
  onClick: () => void
}) {
  const Icon = active ? (direction === 'asc' ? ArrowUp : ArrowDown) : ArrowUpDown

  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={`Sort by ${label.toLowerCase()}`}
      className="mx-auto inline-flex items-center justify-center gap-1 rounded px-1.5 py-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
    >
      <span>{label}</span>
      <Icon className="h-3 w-3" aria-hidden="true" />
    </button>
  )
}

export default function ExpenseGroupsPage() {
  const { t } = useI18n()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const tabParam = searchParams.get('tab')
  const tab: GroupTab = isTab(tabParam) ? tabParam : 'categories'
  const tabMeta = TABS.find((item) => item.id === tab) ?? TABS[0]
  const sortFieldParam = searchParams.get('sort')
  const sortField: SortField | null = isSortField(sortFieldParam) ? sortFieldParam : null
  const sortDirectionParam = searchParams.get('sort_dir')
  const sortDirection: SortDirection = isSortDirection(sortDirectionParam)
    ? sortDirectionParam
    : 'desc'

  const { data: categories = [], isLoading: loadingCategories } = useCategories()
  const { data: buckets = [], isLoading: loadingBuckets } = useBuckets()
  const { data: labels = [], isLoading: loadingLabels } = useLabels()
  const { data: categoryMappings = {} } = useCategoryMappings()
  const { data: bucketMappings = {} } = useBucketMappings()
  const { data: labelMappings = {} } = useLabelMappings()
  const { data: facets } = useFacets()

  const createCategory = useCreateCategory()
  const createBucket = useCreateBucket()
  const createLabel = useCreateLabel()
  const updateLabel = useUpdateLabel()
  const deleteCategory = useDeleteCategory()
  const deleteBucket = useDeleteBucket()
  const deleteLabel = useDeleteLabel()
  const applyCategory = useApplyCategory()
  const applyBucket = useApplyBucket()
  const applyLabel = useApplyLabel()
  const removeCategory = useRemoveCategoryByMerchant()
  const removeBucket = useRemoveBucketByMerchant()
  const removeLabel = useRemoveLabelByMerchant()

  const [newName, setNewName] = useState('')
  const [newColor, setNewColor] = useState('#6366f1')
  const [search, setSearch] = useState('')
  const [selectedNames, setSelectedNames] = useState<Record<GroupTab, string | null>>({
    categories: null,
    buckets: null,
    labels: null,
  })
  const [merchantPattern, setMerchantPattern] = useState('')
  const [merchantSuggest, setMerchantSuggest] = useState<MerchantSuggestState>(null)
  const [colorPicker, setColorPicker] = useState<ColorPickerState>(null)
  const [deleteItem, setDeleteItem] = useState<GroupItem | null>(null)
  const [removeExistingTransactions, setRemoveExistingTransactions] = useState(false)
  const [importConflict, setImportConflict] = useState<ImportConflict | null>(null)
  const [note, setNote] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const addMerchantRef = useRef<HTMLInputElement>(null)

  const isLoading = loadingCategories || loadingBuckets || loadingLabels

  const items = useMemo<GroupItem[]>(() => {
    if (tab === 'categories') {
      return categories.map((category) => ({
        name: category.name,
        isDefault: category.is_default,
        merchants: categoryMappings[category.name] ?? [],
        transactionCount: facets?.category_counts?.[category.name] ?? 0,
      }))
    }
    if (tab === 'buckets') {
      return buckets.map((bucket) => ({
        name: bucket.name,
        isDefault: bucket.is_default,
        merchants: bucketMappings[bucket.name] ?? [],
        transactionCount: facets?.bucket_counts?.[bucket.name] ?? 0,
      }))
    }
    return labels.map((label) => ({
      name: label.name,
      color: label.color,
      merchants: labelMappings[label.name] ?? [],
      transactionCount: facets?.label_counts?.[label.name] ?? 0,
    }))
  }, [bucketMappings, buckets, categories, categoryMappings, facets, labelMappings, labels, tab])

  const selectedItem = items.find((item) => item.name === selectedNames[tab]) ?? items[0] ?? null

  useEffect(() => {
    if (!selectedItem && items[0]) {
      setSelectedNames((current) => ({ ...current, [tab]: items[0].name }))
    }
  }, [items, selectedItem, tab])

  useEffect(() => {
    const handlePointerDown = (event: MouseEvent) => {
      if (
        !(event.target as Element).closest('[data-color-picker]') &&
        !(event.target as Element).closest('[data-color-trigger]')
      ) {
        setColorPicker(null)
      }
      if (!(event.target as Element).closest('[data-merchant-suggestions]')) {
        setMerchantSuggest(null)
      }
    }
    document.addEventListener('mousedown', handlePointerDown)
    return () => document.removeEventListener('mousedown', handlePointerDown)
  }, [])

  const filteredItems = useMemo(() => {
    const query = search.trim().toLowerCase()
    if (!query) return items
    return items.filter(
      (item) =>
        item.name.toLowerCase().includes(query) ||
        item.merchants.some((merchant) => merchant.toLowerCase().includes(query)),
    )
  }, [items, search])

  const sortedItems = useMemo(() => {
    if (!sortField) return filteredItems
    const multiplier = sortDirection === 'asc' ? 1 : -1
    return [...filteredItems].sort((a, b) => {
      const aValue = sortField === 'merchants' ? a.merchants.length : a.transactionCount
      const bValue = sortField === 'merchants' ? b.merchants.length : b.transactionCount
      const countComparison = (aValue - bValue) * multiplier
      if (countComparison !== 0) return countComparison
      return a.name.localeCompare(b.name)
    })
  }, [filteredItems, sortDirection, sortField])

  const merchantSuggestions = useMemo(() => {
    const allMerchants = new Set(facets?.merchants ?? [])
    for (const item of items) {
      for (const merchant of item.merchants) allMerchants.add(merchant)
    }
    const query = merchantPattern.trim().toLowerCase()
    return [...allMerchants]
      .filter((merchant) => merchant.toLowerCase().includes(query) && merchant !== query)
      .slice(0, 6)
  }, [facets?.merchants, items, merchantPattern])

  const transactionFilterParam =
    tab === 'categories' ? 'category' : tab === 'buckets' ? 'bucket' : 'label'

  const setTab = (next: GroupTab) => {
    setSearchParams({ tab: next }, { replace: true })
    setSearch('')
    setNewName('')
    setMerchantPattern('')
  }

  const toggleSort = (field: SortField) => {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        next.set('sort', field)
        next.set('sort_dir', sortField === field && sortDirection === 'desc' ? 'asc' : 'desc')
        return next
      },
      { replace: true },
    )
  }

  const openColorPicker = (next: NonNullable<ColorPickerState>) => {
    setColorPicker((current) => (colorPickerKey(current) === colorPickerKey(next) ? null : next))
  }

  const selectItem = (name: string, field?: 'merchant') => {
    setSelectedNames((current) => ({ ...current, [tab]: name }))
    setMerchantPattern('')
    window.setTimeout(() => {
      if (field === 'merchant') addMerchantRef.current?.focus()
    }, 0)
  }

  const createItem = () => {
    const trimmed = newName.trim()
    if (!trimmed) return
    if (tab === 'categories') createCategory.mutate({ name: trimmed })
    if (tab === 'buckets') createBucket.mutate({ name: trimmed })
    if (tab === 'labels') createLabel.mutate({ name: trimmed, color: newColor })
    setSelectedNames((current) => ({ ...current, [tab]: trimmed }))
    setNewName('')
    setNewColor('#6366f1')
  }

  const applyMerchant = (pattern = merchantPattern) => {
    if (!selectedItem || !pattern.trim()) return
    const payload = { name: selectedItem.name, pattern: pattern.trim() }
    if (tab === 'categories') applyCategory.mutate(payload)
    if (tab === 'buckets') applyBucket.mutate(payload)
    if (tab === 'labels') applyLabel.mutate(payload)
    setMerchantPattern('')
    setMerchantSuggest(null)
  }

  const removeMerchant = (name: string, pattern: string) => {
    const payload = { name, pattern }
    if (tab === 'categories') removeCategory.mutate(payload)
    if (tab === 'buckets') removeBucket.mutate(payload)
    if (tab === 'labels') removeLabel.mutate(payload)
  }

  const runImport = (imported: GroupItem[], decisions: Record<string, Decision>) => {
    const existing = new Map(items.map((item) => [item.name.toLowerCase(), item]))
    for (const row of imported) {
      const match = existing.get(row.name.toLowerCase())
      if (match && decisions[row.name] !== 'overwrite') continue

      if (!match) {
        if (tab === 'categories') createCategory.mutate({ name: row.name })
        if (tab === 'buckets') createBucket.mutate({ name: row.name })
        if (tab === 'labels') createLabel.mutate({ name: row.name, color: row.color ?? '#6366f1' })
      } else if (tab === 'labels' && row.color) {
        updateLabel.mutate({ name: match.name, color: row.color })
      }

      if (match && decisions[row.name] === 'overwrite') {
        for (const merchant of match.merchants) removeMerchant(match.name, merchant)
      }
      for (const merchant of row.merchants) applyMerchantTo(row.name, merchant)
    }
    setImportConflict(null)
    setNote(t('expenseGroups.importComplete'))
  }

  const applyMerchantTo = (name: string, pattern: string) => {
    const payload = { name, pattern }
    if (tab === 'categories') applyCategory.mutate(payload)
    if (tab === 'buckets') applyBucket.mutate(payload)
    if (tab === 'labels') applyLabel.mutate(payload)
  }

  const importFile = async (file: File) => {
    try {
      const imported = parseImportRows(JSON.parse(await readFileText(file)), tab)
      const existingNames = new Set(items.map((item) => item.name.toLowerCase()))
      const conflicts = imported
        .filter((item) => existingNames.has(item.name.toLowerCase()))
        .map((item) => item.name)
      if (conflicts.length > 0) {
        setImportConflict({
          imported,
          conflicts,
          fileName: file.name,
          decisions: Object.fromEntries(conflicts.map((name) => [name, 'keep'])),
        })
      } else {
        runImport(imported, {})
      }
    } catch {
      setNote(t('expenseGroups.importInvalid'))
    } finally {
      if (fileInputRef.current) fileInputRef.current.value = ''
    }
  }

  const deleteSelectedItem = () => {
    if (!deleteItem) return
    const payload = { name: deleteItem.name, removeFromTransactions: removeExistingTransactions }
    if (tab === 'categories') deleteCategory.mutate(payload)
    if (tab === 'buckets') deleteBucket.mutate(payload)
    if (tab === 'labels') deleteLabel.mutate(payload)
    setDeleteItem(null)
    setRemoveExistingTransactions(false)
  }

  if (isLoading) return <p className="p-6 text-xs text-muted-foreground">{t('common.loading')}</p>

  return (
    <div className="mx-auto w-full max-w-6xl px-8 py-6">
      <h1 className="mb-6 text-lg font-semibold text-foreground">{t('nav.expenseGroups')}</h1>

      <div className="mb-5 border-b border-border">
        <div className="flex flex-wrap gap-1">
          {TABS.map((item) => (
            <button
              key={item.id}
              type="button"
              onClick={() => setTab(item.id)}
              className={cn(
                '-mb-px inline-flex items-center gap-2 border-b-2 px-4 py-2 text-sm transition-colors',
                tab === item.id
                  ? 'border-primary font-medium text-foreground'
                  : 'border-transparent text-muted-foreground hover:text-foreground',
              )}
              aria-current={tab === item.id ? 'page' : undefined}
            >
              <item.icon size={15} aria-hidden="true" />
              {t(item.labelKey)}
            </button>
          ))}
        </div>
      </div>

      <div className="space-y-4">
        <div className="rounded-lg border border-border bg-card p-2">
          <div className="relative mb-2">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t('expenseGroups.searchPlaceholder', {
                group: t(tabMeta.labelKey).toLowerCase(),
              })}
              aria-label={t('expenseGroups.searchLabel', {
                group: t(tabMeta.labelKey).toLowerCase(),
              })}
              className="h-10 w-full rounded-md border border-border bg-input pl-9 pr-3 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <input
              value={newName}
              onChange={(event) => setNewName(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') createItem()
              }}
              placeholder={t(tabMeta.newPlaceholderKey)}
              aria-label={t(tabMeta.newPlaceholderKey)}
              className="h-10 w-full rounded-md border border-border bg-input px-3 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring sm:w-56"
            />
            {tab === 'labels' && (
              <button
                type="button"
                data-color-trigger
                onClick={(event) =>
                  openColorPicker({
                    kind: 'new',
                    rect: event.currentTarget.getBoundingClientRect(),
                  })
                }
                className="flex h-10 w-10 items-center justify-center rounded-md border border-border bg-input transition-colors hover:bg-accent"
                aria-label={t('expenseGroups.color.chooseNew')}
              >
                <span className="h-5 w-5 rounded-full" style={{ background: newColor }} />
              </button>
            )}
            <button
              type="button"
              disabled={!newName.trim()}
              onClick={createItem}
              className="h-10 rounded-md bg-primary px-4 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-40"
            >
              {t('expenseGroups.newButton', { group: t(tabMeta.singularKey) })}
            </button>
            <div className="ml-auto flex gap-2">
              <input
                ref={fileInputRef}
                type="file"
                data-testid="expense-groups-import-file"
                accept="application/json,.json"
                className="sr-only"
                onChange={(event) => {
                  const file = event.target.files?.[0]
                  if (file) void importFile(file)
                }}
              />
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                className="h-10 rounded-md px-3 text-sm text-muted-foreground transition-colors hover:text-foreground"
              >
                {t('common.import')}
              </button>
              <a
                href={tabMeta.exportUrl}
                download={tabMeta.exportName}
                className="flex h-10 items-center rounded-md px-3 text-sm text-muted-foreground transition-colors hover:text-foreground"
              >
                {t('common.export')}
              </a>
            </div>
          </div>
        </div>

        {note && <p className="text-xs text-muted-foreground">{note}</p>}

        <div
          className={cn(
            'grid items-start gap-4',
            selectedItem
              ? 'grid-cols-[minmax(0,1fr)_minmax(18rem,24rem)] max-lg:grid-cols-1'
              : 'grid-cols-1',
          )}
        >
          <div className="self-start overflow-hidden rounded-lg border border-border">
            <table className="w-full table-fixed">
              <colgroup>
                <col />
                <col className="w-28" />
                <col className="w-32" />
                <col className="w-16" />
              </colgroup>
              <thead>
                <tr className="border-b border-border bg-secondary/50">
                  <th className="px-4 py-3 text-left text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                    {t('expenseGroups.columns.name')}
                  </th>
                  <th
                    aria-sort={
                      sortField === 'merchants'
                        ? sortDirection === 'asc'
                          ? 'ascending'
                          : 'descending'
                        : undefined
                    }
                    className="px-3 py-3 text-center text-[10px] font-semibold uppercase tracking-wider text-muted-foreground"
                  >
                    <SortHeaderButton
                      active={sortField === 'merchants'}
                      direction={sortDirection}
                      label={t('expenseGroups.columns.merchants')}
                      onClick={() => toggleSort('merchants')}
                    />
                  </th>
                  <th
                    aria-sort={
                      sortField === 'transactions'
                        ? sortDirection === 'asc'
                          ? 'ascending'
                          : 'descending'
                        : undefined
                    }
                    className="px-3 py-3 text-center text-[10px] font-semibold uppercase tracking-wider text-muted-foreground"
                  >
                    <SortHeaderButton
                      active={sortField === 'transactions'}
                      direction={sortDirection}
                      label={t('expenseGroups.columns.transactions')}
                      onClick={() => toggleSort('transactions')}
                    />
                  </th>
                  <th className="px-2 py-3 text-center text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                    {t('expenseGroups.columns.delete')}
                  </th>
                </tr>
              </thead>
              <tbody>
                {sortedItems.map((item) => (
                  <tr
                    key={item.name}
                    onClick={(event) => {
                      if (!(event.target as Element).closest('button')) selectItem(item.name)
                    }}
                    className={cn(
                      'border-b border-border last:border-0 hover:bg-accent/40',
                      selectedItem?.name === item.name && 'bg-accent/60',
                    )}
                  >
                    <td className="px-4 py-2.5">
                      <div className="flex min-w-0 items-center gap-3">
                        {tab === 'labels' && (
                          <button
                            type="button"
                            data-color-trigger
                            onClick={(event) =>
                              openColorPicker({
                                kind: 'item',
                                name: item.name,
                                rect: event.currentTarget.getBoundingClientRect(),
                              })
                            }
                            className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md border border-border bg-input transition-colors hover:bg-accent"
                            aria-label={t('expenseGroups.color.change', { name: item.name })}
                          >
                            <span
                              className="h-5 w-5 rounded-full"
                              style={{ background: item.color }}
                            />
                          </button>
                        )}
                        <button
                          type="button"
                          onClick={() => selectItem(item.name)}
                          className="min-w-0 truncate rounded-md px-1 py-1 text-left text-sm font-medium text-foreground transition-colors hover:bg-accent"
                        >
                          {item.name}
                        </button>
                        {item.isDefault && (
                          <span className="rounded-sm border border-border px-1 py-0.5 text-[10px] text-muted-foreground">
                            {t('expenseGroups.default')}
                          </span>
                        )}
                      </div>
                    </td>
                    <td className="px-3 py-2.5 text-center">
                      <button
                        type="button"
                        onClick={() => selectItem(item.name, 'merchant')}
                        className="inline-flex min-w-8 justify-center rounded-full border border-primary/30 bg-primary/10 px-2 py-1 text-xs text-primary transition-colors hover:bg-primary/15"
                        aria-label={t('expenseGroups.editMerchants', {
                          count: item.merchants.length,
                          name: item.name,
                        })}
                      >
                        {item.merchants.length}
                      </button>
                    </td>
                    <td className="px-3 py-2.5 text-center">
                      <button
                        type="button"
                        onClick={() =>
                          navigate(
                            `/transactions?${transactionFilterParam}=${encodeURIComponent(item.name)}`,
                          )
                        }
                        className="inline-flex min-w-8 justify-center rounded-full border border-primary/30 bg-primary/10 px-2 py-1 text-xs text-primary transition-colors hover:bg-primary/15"
                        aria-label={t('expenseGroups.viewTransactions', { name: item.name })}
                      >
                        {item.transactionCount}
                      </button>
                    </td>
                    <td className="px-2 py-2.5 text-center">
                      <button
                        type="button"
                        disabled={item.isDefault}
                        onClick={() => {
                          setDeleteItem(item)
                          setRemoveExistingTransactions(false)
                        }}
                        className="inline-flex h-8 w-8 items-center justify-center rounded-md text-destructive transition-colors hover:bg-destructive/10 hover:ring-1 hover:ring-destructive/30 disabled:cursor-not-allowed disabled:text-destructive/60"
                        aria-label={t('expenseGroups.deleteAria', { name: item.name })}
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </td>
                  </tr>
                ))}
                {sortedItems.length === 0 && (
                  <tr>
                    <td colSpan={4} className="px-4 py-8 text-center text-xs text-muted-foreground">
                      {t('expenseGroups.emptySearch', { group: t(tabMeta.labelKey).toLowerCase() })}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          {selectedItem && (
            <aside
              role="region"
              aria-label={t('expenseGroups.inspectorAria', { name: selectedItem.name })}
              className="self-start rounded-lg border border-border bg-card p-4"
            >
              <div className="mb-4 flex min-w-0 items-center gap-2">
                {tab === 'labels' && (
                  <span
                    className="h-5 w-5 rounded-full"
                    style={{ background: selectedItem.color }}
                  />
                )}
                <h2 className="truncate text-base font-semibold text-foreground">
                  {selectedItem.name}
                </h2>
              </div>
              <label className="mb-1 block text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                {t('expenseGroups.columns.name')}
              </label>
              <input
                value={selectedItem.name}
                readOnly
                aria-label={t('expenseGroups.nameAria')}
                className="mb-4 h-9 w-full rounded-md border border-border bg-input px-3 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
              />
              <div className="mb-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                {t('expenseGroups.linkedMerchants')}
              </div>
              <div className="mb-4 flex min-h-8 flex-wrap gap-1.5">
                {selectedItem.merchants.length > 0 ? (
                  selectedItem.merchants.map((merchant) => (
                    <span
                      key={merchant}
                      className="inline-flex items-center gap-1 rounded-full border border-border bg-secondary px-2 py-1 text-xs text-muted-foreground"
                    >
                      {merchant}
                      <button
                        type="button"
                        onClick={() => removeMerchant(selectedItem.name, merchant)}
                        className="rounded-full text-muted-foreground hover:text-foreground"
                        aria-label={t('expenseGroups.removeMerchant', {
                          merchant,
                          name: selectedItem.name,
                        })}
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </span>
                  ))
                ) : (
                  <span className="text-xs text-muted-foreground">
                    {t('expenseGroups.noMerchants')}
                  </span>
                )}
              </div>
              <label className="mb-1 block text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
                {t('expenseGroups.addMerchant')}
              </label>
              <input
                ref={addMerchantRef}
                value={merchantPattern}
                onChange={(event) => {
                  setMerchantPattern(event.target.value)
                  setMerchantSuggest({
                    value: event.target.value,
                    rect: event.currentTarget.getBoundingClientRect(),
                  })
                }}
                onFocus={(event) =>
                  setMerchantSuggest({
                    value: merchantPattern,
                    rect: event.currentTarget.getBoundingClientRect(),
                  })
                }
                onKeyDown={(event) => {
                  if (event.key === 'Enter') applyMerchant()
                }}
                aria-label={t('expenseGroups.addMerchant')}
                placeholder={t('expenseGroups.placeholders.merchant')}
                className="h-9 w-full rounded-md border border-border bg-input px-3 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
              />
            </aside>
          )}
        </div>
      </div>

      {merchantSuggest &&
        merchantPattern.trim() &&
        merchantSuggestions.length > 0 &&
        createPortal(
          <div
            data-merchant-suggestions
            className="fixed z-50 rounded-lg border border-border bg-card p-1 shadow-xl"
            style={{
              left: merchantSuggest.rect.left,
              top: merchantSuggest.rect.bottom + 6,
              width: merchantSuggest.rect.width,
            }}
          >
            {merchantSuggestions.map((merchant) => (
              <button
                key={merchant}
                type="button"
                onClick={() => applyMerchant(merchant)}
                className="block w-full rounded-md px-3 py-2 text-left text-xs text-foreground hover:bg-accent"
              >
                {merchant}
              </button>
            ))}
          </div>,
          document.body,
        )}

      {colorPicker &&
        createPortal(
          <div
            data-color-picker
            role="menu"
            aria-label={t('expenseGroups.color.menu')}
            className="fixed z-50 grid grid-cols-4 gap-2 rounded-lg border border-border bg-card p-3 shadow-xl"
            style={{
              left: Math.min(colorPicker.rect.left, window.innerWidth - 164),
              top: colorPicker.rect.bottom + 6,
            }}
          >
            {PRESET_COLORS.map((color) => {
              const selected =
                colorPicker.kind === 'new'
                  ? newColor === color
                  : items.find((item) => item.name === colorPicker.name)?.color === color
              return (
                <button
                  key={color}
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    if (colorPicker.kind === 'new') setNewColor(color)
                    else updateLabel.mutate({ name: colorPicker.name, color })
                    setColorPicker(null)
                  }}
                  className="flex h-7 w-7 items-center justify-center rounded-full focus:outline-none focus:ring-1 focus:ring-ring"
                  aria-label={t('expenseGroups.color.set', { color })}
                >
                  <span
                    className={cn(
                      'flex h-5 w-5 items-center justify-center rounded-full',
                      selected && 'ring-2 ring-ring ring-offset-2 ring-offset-card',
                    )}
                    style={{ background: color }}
                  >
                    {selected && <Check className="h-3 w-3 text-white" />}
                  </span>
                </button>
              )
            })}
          </div>,
          document.body,
        )}

      {deleteItem && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) setDeleteItem(null)
          }}
        >
          <div
            role="dialog"
            aria-modal="true"
            aria-labelledby="delete-group-title"
            className="w-full max-w-sm space-y-4 rounded-lg border border-border bg-card p-6 shadow-xl"
          >
            <h2 id="delete-group-title" className="text-sm font-semibold text-foreground">
              {t('expenseGroups.deleteTitle', { group: t(tabMeta.singularKey) })}
            </h2>
            <p className="text-xs leading-relaxed text-muted-foreground">
              {t('expenseGroups.deleteBody', { name: deleteItem.name })}
            </p>
            {deleteItem.transactionCount > 0 && (
              <label className="flex items-start gap-3 rounded-md border border-border bg-secondary/50 p-3 text-xs text-muted-foreground">
                <input
                  type="checkbox"
                  aria-label={t('expenseGroups.removeExisting', { group: t(tabMeta.singularKey) })}
                  checked={removeExistingTransactions}
                  onChange={(event) => setRemoveExistingTransactions(event.target.checked)}
                  className="mt-0.5 h-4 w-4 rounded border-border bg-input accent-primary"
                />
                <span>
                  <span className="block font-medium text-foreground">
                    {t('expenseGroups.removeExisting', { group: t(tabMeta.singularKey) })}
                  </span>
                  <span className="mt-1 block">
                    {t('expenseGroups.removeExistingHelp', { group: t(tabMeta.singularKey) })}
                  </span>
                </span>
              </label>
            )}
            <div className="flex justify-end gap-2 pt-1">
              <button
                type="button"
                onClick={() => setDeleteItem(null)}
                className="rounded-md px-4 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
              >
                {t('common.cancel')}
              </button>
              <button
                type="button"
                onClick={deleteSelectedItem}
                className="rounded-md bg-destructive px-4 py-2 text-sm text-destructive-foreground transition-colors hover:bg-destructive/90"
              >
                {t('expenseGroups.deleteConfirm', { group: t(tabMeta.singularKey) })}
              </button>
            </div>
          </div>
        </div>
      )}

      {importConflict && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm">
          <div
            role="dialog"
            aria-modal="true"
            aria-labelledby="import-conflicts-title"
            className="w-full max-w-lg space-y-4 rounded-lg border border-border bg-card p-6 shadow-xl"
          >
            <h2 id="import-conflicts-title" className="text-sm font-semibold text-foreground">
              {t('expenseGroups.importConflictsTitle')}
            </h2>
            <p className="text-xs leading-relaxed text-muted-foreground">
              {t('expenseGroups.importConflictsBody')}
            </p>
            <div className="max-h-72 space-y-2 overflow-y-auto">
              {importConflict.conflicts.map((name) => (
                <div
                  key={name}
                  className="flex items-center justify-between gap-3 rounded-md border border-border p-2"
                >
                  <span className="truncate text-sm text-foreground">{name}</span>
                  <div className="flex rounded-md border border-border bg-secondary p-0.5">
                    {(['keep', 'overwrite'] as Decision[]).map((decision) => (
                      <button
                        key={decision}
                        type="button"
                        onClick={() =>
                          setImportConflict(
                            (current) =>
                              current && {
                                ...current,
                                decisions: { ...current.decisions, [name]: decision },
                              },
                          )
                        }
                        className={cn(
                          'rounded px-2 py-1 text-xs capitalize',
                          importConflict.decisions[name] === decision
                            ? 'bg-background text-foreground'
                            : 'text-muted-foreground hover:text-foreground',
                        )}
                      >
                        {t(
                          decision === 'keep'
                            ? 'expenseGroups.importKeep'
                            : 'expenseGroups.importOverwrite',
                        )}
                      </button>
                    ))}
                  </div>
                </div>
              ))}
            </div>
            <div className="flex justify-end gap-2 pt-1">
              <button
                type="button"
                onClick={() => setImportConflict(null)}
                className="rounded-md px-4 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
              >
                {t('common.cancel')}
              </button>
              <button
                type="button"
                onClick={() => runImport(importConflict.imported, importConflict.decisions)}
                className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
              >
                {t('expenseGroups.importApply')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
