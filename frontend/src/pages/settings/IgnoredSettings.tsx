import {
  useDeleteMutedMerchant,
  useIgnoreByMerchant,
  useIgnoredMerchants,
  useIgnoreTransaction,
  useTransactions,
} from '@/api/queries'
import type { MutedMerchant } from '@/api/types'
import { useDisplay } from '@/contexts/DisplayContext'
import { cn, formatDate } from '@/lib/utils'
import { Eye } from 'lucide-react'
import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useI18n } from '@/i18n/I18nProvider'

// ─── Delete-with-unmute confirmation modal ────────────────────────────────────

function DeleteMerchantModal({
  merchant,
  onCancel,
  onConfirm,
}: {
  merchant: MutedMerchant
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
        <p className="text-xs text-muted-foreground">
          {t('ignored.merchant.delete.existingQuestion')}
        </p>
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

// ─── Individual ignored transactions ───────────────────────────────────────────

function IgnoredTransactionsSection() {
  const { t } = useI18n()
  const { data, isLoading } = useTransactions({ show_muted: true, page_size: 100 }, '')
  const { mutate: ignoreTransaction } = useIgnoreTransaction()
  const qc = useQueryClient()
  const { timezone, timeFormat } = useDisplay()

  const muted = (data?.transactions ?? []).filter((t) => t.muted)

  if (isLoading) return <p className="text-xs text-muted-foreground">{t('common.loading')}</p>

  if (muted.length === 0)
    return <p className="text-xs text-muted-foreground">{t('ignored.individual.empty')}</p>

  return (
    <div className="rounded-lg border border-border">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border bg-secondary/50">
            <th className="px-3 py-2 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
              {t('common.merchant')}
            </th>
            <th className="px-3 py-2 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
              {t('common.amount')}
            </th>
            <th className="px-3 py-2 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
              {t('common.date')}
            </th>
            <th className="px-3 py-2" />
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {muted.map((tx) => (
            <tr key={tx.id} className="hover:bg-secondary/30">
              <td className="px-3 py-2 text-xs text-foreground">{tx.merchant_info}</td>
              <td className="px-3 py-2 font-mono text-xs text-muted-foreground">
                {tx.amount.toLocaleString(undefined, { maximumFractionDigits: 2 })} {tx.currency}
              </td>
              <td className="px-3 py-2 text-xs text-muted-foreground">
                {formatDate(tx.timestamp, true, timezone, timeFormat)}
              </td>
              <td className="px-3 py-2 text-right">
                <button
                  onClick={() =>
                    ignoreTransaction(
                      { id: tx.id, muted: false },
                      { onSuccess: () => qc.invalidateQueries({ queryKey: ['transactions'] }) },
                    )
                  }
                  aria-label={t('ignored.confirm.restoreMessage')}
                  className="flex items-center gap-1 text-xs text-primary hover:underline"
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
  )
}

// ─── Merchant-wide ignore patterns ─────────────────────────────────────────────

function IgnoredMerchantsSection() {
  const { t } = useI18n()
  const { data: merchants = [], isLoading } = useIgnoredMerchants()
  const { mutate: addPattern, isPending: adding } = useIgnoreByMerchant()
  const { mutate: deletePattern } = useDeleteMutedMerchant()
  const [input, setInput] = useState('')
  const [filterQuery, setFilterQuery] = useState('')
  const [pendingDelete, setPendingDelete] = useState<MutedMerchant | null>(null)
  const { timezone, timeFormat } = useDisplay()

  const handleAdd = () => {
    const pattern = input.trim()
    if (!pattern) return
    addPattern({ pattern }, { onSuccess: () => setInput('') })
  }

  const filtered = filterQuery
    ? merchants.filter((m) => m.pattern.toLowerCase().includes(filterQuery.toLowerCase()))
    : merchants

  return (
    <div className="space-y-3">
      <p className="text-xs text-muted-foreground">{t('ignored.merchant.summary')}</p>

      <div className="flex gap-2">
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
          placeholder={t('ignored.merchant.settingsPlaceholder')}
          className="flex-1 rounded-md border border-border bg-secondary px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring"
        />
        <button
          onClick={handleAdd}
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
        <p className="text-xs text-muted-foreground">{t('ignored.merchant.settingsEmpty')}</p>
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
                    {t('ignored.merchant.added')}
                  </th>
                  <th className="px-3 py-2" />
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {filtered.length === 0 ? (
                  <tr>
                    <td colSpan={3} className="px-3 py-4 text-center text-xs text-muted-foreground">
                      {t('ignored.merchant.filterEmptyPrefix')} &quot;{filterQuery}&quot;
                    </td>
                  </tr>
                ) : (
                  filtered.map((m) => (
                    <tr key={m.id} className="hover:bg-secondary/30">
                      <td className="px-3 py-2 font-mono text-xs text-foreground">{m.pattern}</td>
                      <td className="px-3 py-2 text-xs text-muted-foreground">
                        {formatDate(m.created_at, true, timezone, timeFormat)}
                      </td>
                      <td className="px-3 py-2 text-right">
                        <button
                          onClick={() => setPendingDelete(m)}
                          className="text-xs text-destructive hover:underline"
                        >
                          {t('common.remove')}
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
  )
}

// ─── Combined ignored settings page ────────────────────────────────────────────

export function IgnoredSettings() {
  const { t } = useI18n()

  return (
    <div className="space-y-8">
      <div>
        <h2 className="mb-3 text-sm font-medium text-foreground">
          {t('ignored.individual.heading')}
        </h2>
        <IgnoredTransactionsSection />
      </div>
      <div>
        <h2 className="mb-3 text-sm font-medium text-foreground">
          {t('ignored.merchant.heading')}
        </h2>
        <IgnoredMerchantsSection />
      </div>
    </div>
  )
}
