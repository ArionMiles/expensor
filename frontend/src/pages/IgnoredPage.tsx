import {
  useDeleteMutedMerchant,
  useFacets,
  useIgnoreByMerchant,
  useIgnoredMerchants,
  useIgnoreTransaction,
  useTransactions,
  useUpdateMerchantReason,
  useUpdateIgnoreReason,
} from '@/api/queries'
import type { MutedMerchantWithCount } from '@/api/types'
import { useDisplay } from '@/contexts/DisplayContext'
import { cn, formatDate } from '@/lib/utils'
import { Eye, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { createPortal } from 'react-dom'
import { useQueryClient } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { ConfirmModal } from '@/components/ConfirmModal'
import { useI18n } from '@/i18n/I18nProvider'
import { useTooltip } from '@/hooks/useTooltip'

type MerchantSuggestState = { rect: DOMRect; value: string } | null

// ─── Inline editable reason ──────────────────────────────────────────────────

function EditableReason({
  value,
  onSave,
  placeholder,
}: {
  value?: string
  onSave: (reason: string) => void
  placeholder?: string
}) {
  const { t } = useI18n()
  const placeholderText = placeholder ?? t('common.reason')
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(value ?? '')
  const { handlers: reasonTip, tip: reasonTipEl } = useTooltip()

  const commit = () => {
    setEditing(false)
    if (draft !== (value ?? '')) onSave(draft)
  }

  if (editing) {
    return (
      <input
        autoFocus
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === 'Enter') commit()
          if (e.key === 'Escape') {
            setDraft(value ?? '')
            setEditing(false)
          }
        }}
        placeholder={placeholderText}
        className="w-full rounded border border-primary bg-accent px-2 py-0.5 text-xs text-foreground focus:outline-none"
      />
    )
  }

  return (
    <>
      <button
        onClick={() => {
          setDraft(value ?? '')
          setEditing(true)
        }}
        {...reasonTip(value || placeholderText)}
        className="block w-full max-w-full truncate text-left text-xs text-muted-foreground hover:text-foreground"
        aria-label={value || placeholderText}
      >
        {value || <span className="opacity-30">—</span>}
      </button>
      {reasonTipEl}
    </>
  )
}

// ─── Delete merchant modal ────────────────────────────────────────────────────

function DeleteMerchantModal({
  merchant,
  onCancel,
  onConfirm,
}: {
  merchant: MutedMerchantWithCount
  onCancel: () => void
  onConfirm: (unmute: boolean) => void
}) {
  const { t } = useI18n()

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm">
      <div className="w-full max-w-sm space-y-4 rounded-lg border border-border bg-card p-6 shadow-xl">
        <h2 className="text-sm font-semibold text-foreground">
          {t('ignored.merchant.delete.title')}
        </h2>
        <p className="text-xs text-muted-foreground">
          {t('ignored.merchant.delete.messagePrefix')}{' '}
          <span className="font-medium text-foreground">&quot;{merchant.pattern}&quot;</span>?
        </p>
        {merchant.muted_count > 0 && (
          <p className="text-xs text-muted-foreground">
            {merchant.muted_count} transaction{merchant.muted_count !== 1 ? 's are' : ' is'}{' '}
            {t('ignored.merchant.delete.currentlyIgnoredSuffix')}
          </p>
        )}
        <div className="flex flex-col gap-2">
          <button
            onClick={() => onConfirm(true)}
            className="w-full rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground hover:bg-primary/90"
          >
            {t('ignored.merchant.delete.restoreExisting')}
          </button>
          <button
            onClick={() => onConfirm(false)}
            className="w-full rounded-md border border-border px-4 py-2 text-sm text-muted-foreground hover:text-foreground"
          >
            {t('ignored.merchant.delete.keepIgnored')}
          </button>
          <button
            onClick={onCancel}
            className="w-full px-4 py-2 text-xs text-muted-foreground hover:text-foreground"
          >
            {t('common.cancel')}
          </button>
        </div>
      </div>
    </div>
  )
}

// ─── Individual ignored transactions tab ──────────────────────────────────────

function IndividualTab() {
  const { t } = useI18n()
  const { data, isLoading } = useTransactions({ individual_only: true, page_size: 100 }, '')
  const { mutate: ignoreTransaction } = useIgnoreTransaction()
  const { mutate: updateReason } = useUpdateIgnoreReason()
  const qc = useQueryClient()
  const { timezone, timeFormat } = useDisplay()
  const [confirmUnmute, setConfirmUnmute] = useState<string | null>(null)

  const muted = data?.transactions ?? []

  if (isLoading) return <p className="text-xs text-muted-foreground">{t('common.loading')}</p>

  if (muted.length === 0)
    return <p className="text-xs text-muted-foreground">{t('ignored.individual.empty')}</p>

  return (
    <>
      <div className="rounded-lg border border-border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border bg-secondary/50">
              <th className="px-3 py-2 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                {t('common.date')}
              </th>
              <th className="px-3 py-2 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                {t('common.merchant')}
              </th>
              <th className="px-3 py-2 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                {t('common.amount')}
              </th>
              <th className="px-3 py-2 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                {t('common.reason')}
              </th>
              <th className="px-3 py-2" />
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {muted.map((tx) => (
              <tr key={tx.id} className="hover:bg-secondary/30">
                <td className="px-3 py-2 text-xs text-muted-foreground">
                  {formatDate(tx.timestamp, true, timezone, timeFormat)}
                </td>
                <td className="px-3 py-2 text-xs text-foreground">{tx.merchant_info}</td>
                <td className="px-3 py-2 font-mono text-xs text-muted-foreground">
                  {tx.amount.toLocaleString(undefined, { maximumFractionDigits: 2 })} {tx.currency}
                </td>
                <td className="w-[200px] max-w-[200px] overflow-hidden px-3 py-2">
                  <EditableReason
                    value={tx.mute_reason}
                    onSave={(reason) => updateReason({ id: tx.id, reason })}
                  />
                </td>
                <td className="w-[7rem] whitespace-nowrap px-3 py-2 text-right">
                  <button
                    onClick={() => setConfirmUnmute(tx.id)}
                    className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                  >
                    <Eye size={12} />
                    {t('ignored.action.restore')}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {confirmUnmute && (
        <ConfirmModal
          title={t('ignored.transaction.restoreTitle')}
          message={t('ignored.confirm.restoreMessage')}
          confirmLabel={t('ignored.action.restore')}
          onConfirm={() => {
            ignoreTransaction(
              { id: confirmUnmute, muted: false },
              { onSuccess: () => qc.invalidateQueries({ queryKey: ['transactions'] }) },
            )
            setConfirmUnmute(null)
          }}
          onCancel={() => setConfirmUnmute(null)}
        />
      )}
    </>
  )
}

// ─── Merchant-wide ignore patterns tab ─────────────────────────────────────────

function MerchantTab() {
  const { t } = useI18n()
  const { data: merchants = [], isLoading } = useIgnoredMerchants()
  const { data: facets } = useFacets()
  const { mutate: addPattern, isPending: adding } = useIgnoreByMerchant()
  const { mutate: updateReason } = useUpdateMerchantReason()
  const { mutate: deletePattern } = useDeleteMutedMerchant()
  const { timezone } = useDisplay()
  const [input, setInput] = useState('')
  const [filterQuery, setFilterQuery] = useState('')
  const [merchantSuggest, setMerchantSuggest] = useState<MerchantSuggestState>(null)
  const [pendingDelete, setPendingDelete] = useState<MutedMerchantWithCount | null>(null)

  useEffect(() => {
    const handlePointerDown = (event: MouseEvent) => {
      if (!(event.target as Element).closest('[data-merchant-suggestions]')) {
        setMerchantSuggest(null)
      }
    }
    document.addEventListener('mousedown', handlePointerDown)
    return () => document.removeEventListener('mousedown', handlePointerDown)
  }, [])

  const merchantSuggestions = useMemo(() => {
    const query = input.trim().toLowerCase()
    if (!query) return []
    return [...new Set(facets?.merchants ?? [])]
      .filter(
        (merchant) => merchant.toLowerCase().includes(query) && merchant.toLowerCase() !== query,
      )
      .slice(0, 6)
  }, [facets?.merchants, input])

  const handleAdd = (nextPattern = input) => {
    const pattern = nextPattern.trim()
    if (!pattern) return
    addPattern(
      { pattern },
      {
        onSuccess: () => {
          setInput('')
          setMerchantSuggest(null)
        },
      },
    )
  }

  const filtered = filterQuery
    ? merchants.filter((m) => m.pattern.toLowerCase().includes(filterQuery.toLowerCase()))
    : merchants

  return (
    <>
      <div className="space-y-4">
        <p className="text-xs text-muted-foreground">{t('ignored.merchant.scanSummary')}</p>

        <div className="flex gap-2">
          <input
            value={input}
            onChange={(e) => {
              setInput(e.target.value)
              setMerchantSuggest({
                value: e.target.value,
                rect: e.currentTarget.getBoundingClientRect(),
              })
            }}
            onFocus={(e) =>
              setMerchantSuggest({
                value: input,
                rect: e.currentTarget.getBoundingClientRect(),
              })
            }
            onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
            placeholder={t('ignored.merchant.patternPlaceholder')}
            className="flex-1 rounded-md border border-border bg-secondary px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring"
          />
          <button
            onClick={() => handleAdd()}
            disabled={!input.trim() || adding}
            className={cn(
              'rounded-md px-4 py-2 text-sm transition-colors',
              input.trim() && !adding
                ? 'bg-primary text-primary-foreground hover:bg-primary/90'
                : 'cursor-not-allowed bg-secondary text-muted-foreground opacity-50',
            )}
          >
            {adding ? t('common.adding') : t('common.add')}
          </button>
        </div>

        {isLoading ? (
          <p className="text-xs text-muted-foreground">{t('common.loading')}</p>
        ) : merchants.length === 0 ? (
          <p className="text-xs text-muted-foreground">{t('ignored.merchant.empty')}</p>
        ) : (
          <>
            {merchants.length > 4 && (
              <input
                value={filterQuery}
                onChange={(e) => setFilterQuery(e.target.value)}
                placeholder={t('ignored.merchant.searchPlaceholder')}
                className="w-full rounded-md border border-border bg-secondary px-3 py-1.5 text-xs text-foreground placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring"
              />
            )}
            <div className="rounded-lg border border-border">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border bg-secondary/50">
                    <th className="px-3 py-2 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                      {t('ignored.merchant.pattern')}
                    </th>
                    <th className="px-3 py-2 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                      {t('common.reason')}
                    </th>
                    <th className="px-3 py-2 text-center text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                      {t('ignored.merchant.ignoredCount')}
                    </th>
                    <th className="px-3 py-2 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                      {t('ignored.merchant.added')}
                    </th>
                    <th className="px-3 py-2" />
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {filtered.length === 0 ? (
                    <tr>
                      <td
                        colSpan={5}
                        className="px-3 py-4 text-center text-xs text-muted-foreground"
                      >
                        {t('ignored.merchant.filterEmptyPrefix')} &quot;{filterQuery}&quot;
                      </td>
                    </tr>
                  ) : (
                    filtered.map((m) => (
                      <tr key={m.id} className="hover:bg-secondary/30">
                        <td className="px-3 py-2 font-mono text-xs text-foreground">{m.pattern}</td>
                        <td className="w-[180px] max-w-[180px] overflow-hidden px-3 py-2">
                          <EditableReason
                            value={m.reason}
                            onSave={(reason) => updateReason({ id: m.id, reason })}
                          />
                        </td>
                        <td className="px-3 py-2 text-center text-xs">
                          {m.muted_count > 0 ? (
                            <a
                              href={`/transactions?muted_only=1&merchant=${encodeURIComponent(m.pattern)}&show_filters=1`}
                              className="text-primary hover:underline"
                            >
                              {m.muted_count}
                            </a>
                          ) : (
                            <span className="text-muted-foreground">0</span>
                          )}
                        </td>
                        <td className="px-3 py-2 text-xs text-muted-foreground">
                          {formatDate(m.created_at, false, timezone)}
                        </td>
                        <td className="px-3 py-2 text-right">
                          <button
                            type="button"
                            onClick={() => setPendingDelete(m)}
                            className="inline-flex h-8 w-8 items-center justify-center rounded-md text-destructive transition-colors hover:bg-destructive/10 hover:ring-1 hover:ring-destructive/30"
                            aria-label={t('ignored.merchant.delete.aria', { pattern: m.pattern })}
                          >
                            <Trash2 className="h-4 w-4" />
                          </button>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </>
        )}

        {pendingDelete && (
          <DeleteMerchantModal
            merchant={pendingDelete}
            onCancel={() => setPendingDelete(null)}
            onConfirm={(unmute) => {
              deletePattern({ id: pendingDelete.id, unmute })
              setPendingDelete(null)
            }}
          />
        )}
      </div>

      {merchantSuggest &&
        merchantSuggest.value.trim() &&
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
                onClick={() => handleAdd(merchant)}
                className="block w-full rounded-md px-3 py-2 text-left text-xs text-foreground hover:bg-accent"
              >
                {merchant}
              </button>
            ))}
          </div>,
          document.body,
        )}
    </>
  )
}

// ─── Ignored page ─────────────────────────────────────────────────────────────

type IgnoredTab = 'individual' | 'merchant'

export default function IgnoredPage() {
  const { t } = useI18n()
  const [searchParams, setSearchParams] = useSearchParams()
  const tab = (searchParams.get('tab') ?? 'merchant') as IgnoredTab

  const setTab = (nextTab: IgnoredTab) => setSearchParams({ tab: nextTab }, { replace: true })

  return (
    <div className="mx-auto w-full max-w-4xl px-6 py-6">
      <div className="mb-6 space-y-2">
        <h1 className="text-lg font-semibold text-foreground">{t('ignored.page.title')}</h1>
        <p className="text-xs text-muted-foreground">{t('ignored.page.summary')}</p>
      </div>

      {/* URL-param tabs */}
      <div className="mb-6 flex gap-1 border-b border-border">
        {(
          [
            { id: 'merchant', label: t('ignored.merchant.heading') },
            { id: 'individual', label: t('ignored.individual.heading') },
          ] as { id: IgnoredTab; label: string }[]
        ).map((tabItem) => (
          <button
            key={tabItem.id}
            onClick={() => setTab(tabItem.id)}
            className={cn(
              '-mb-px border-b-2 px-4 py-2 text-sm transition-colors',
              tab === tabItem.id
                ? 'border-primary font-medium text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            )}
          >
            {tabItem.label}
          </button>
        ))}
      </div>

      {tab === 'individual' && <IndividualTab />}
      {tab === 'merchant' && <MerchantTab />}
    </div>
  )
}
