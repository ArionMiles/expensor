import { Link, useSearchParams } from 'react-router-dom'
import { CircleAlert, RotateCcw } from 'lucide-react'
import { useExtractionDiagnostics, useUpdateExtractionDiagnosticStatus } from '@/api/queries'
import type {
  ExtractionDiagnostic,
  ExtractionDiagnosticListStatus,
  ExtractionDiagnosticStatus,
} from '@/api/types'
import { cn } from '@/lib/utils'

const STATUS_FILTERS: Array<{ label: string; value: ExtractionDiagnosticListStatus }> = [
  { label: 'Open', value: 'open' },
  { label: 'Resolved', value: 'resolved' },
  { label: 'Ignored', value: 'ignored' },
  { label: 'All', value: 'all' },
]

const VALID_FILTERS = new Set<ExtractionDiagnosticListStatus>(
  STATUS_FILTERS.map((filter) => filter.value),
)

function normalizeStatus(value: string | null): ExtractionDiagnosticListStatus {
  if (value && VALID_FILTERS.has(value as ExtractionDiagnosticListStatus)) {
    return value as ExtractionDiagnosticListStatus
  }
  return 'open'
}

function statusClass(status: ExtractionDiagnosticStatus) {
  switch (status) {
    case 'open':
      return 'border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-200'
    case 'resolved':
      return 'border-emerald-300 bg-emerald-50 text-emerald-800 dark:border-emerald-500/40 dark:bg-emerald-500/10 dark:text-emerald-200'
    case 'ignored':
      return 'border-muted-foreground/30 bg-secondary text-muted-foreground'
  }
}

function formatDate(value?: string | null) {
  if (!value) return '-'
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

function fixRulePath(diagnostic: ExtractionDiagnostic) {
  const diagnosticParam = encodeURIComponent(diagnostic.id)
  return `/rules/new?diagnostic=${diagnosticParam}`
}

function reasonLabel(reason: string) {
  switch (reason) {
    case 'amount_zero':
      return 'Amount zero'
    case 'merchant_empty':
      return 'Merchant empty'
    default:
      return reason.replace(/_/g, ' ')
  }
}

function DiagnosticActions({ diagnostic }: { diagnostic: ExtractionDiagnostic }) {
  const updateStatus = useUpdateExtractionDiagnosticStatus()
  const pending = updateStatus.isPending

  const setStatus = (status: ExtractionDiagnosticStatus) => {
    updateStatus.mutate({ id: diagnostic.id, status })
  }

  return (
    <div className="flex flex-wrap items-center justify-end gap-2">
      <Link
        to={fixRulePath(diagnostic)}
        className="rounded border border-border px-2 py-1 text-xs text-foreground transition-colors hover:bg-accent"
      >
        Fix rule
      </Link>
      {diagnostic.status === 'open' ? (
        <>
          <button
            type="button"
            disabled={pending}
            onClick={() => setStatus('resolved')}
            className="rounded border border-emerald-300 bg-emerald-50 px-2 py-1 text-xs font-medium text-emerald-800 transition-colors hover:bg-emerald-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-emerald-400/50 dark:bg-emerald-500/15 dark:text-emerald-100 dark:hover:bg-emerald-500/25"
          >
            Mark resolved
          </button>
          <button
            type="button"
            disabled={pending}
            onClick={() => setStatus('ignored')}
            className="rounded border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent disabled:cursor-not-allowed disabled:opacity-50"
          >
            Ignore
          </button>
        </>
      ) : (
        <button
          type="button"
          disabled={pending}
          onClick={() => setStatus('open')}
          className="inline-flex items-center gap-1 rounded border border-border px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent disabled:cursor-not-allowed disabled:opacity-50"
        >
          <RotateCcw size={12} />
          Reopen
        </button>
      )}
    </div>
  )
}

export function Diagnostics() {
  const [searchParams, setSearchParams] = useSearchParams()
  const status = normalizeStatus(searchParams.get('status'))
  const { data: diagnostics = [], isLoading, isFetching, error } = useExtractionDiagnostics(status)

  const setStatusFilter = (nextStatus: ExtractionDiagnosticListStatus) => {
    setSearchParams((params) => {
      const next = new URLSearchParams(params)
      next.set('status', nextStatus)
      return next
    })
  }

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-4 px-6 py-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div className="flex items-center gap-2">
            <CircleAlert
              size={18}
              data-testid="diagnostics-heading-icon"
              className="text-amber-800 dark:text-amber-200"
            />
            <h1 className="text-lg font-semibold text-foreground">Extraction diagnostics</h1>
          </div>
          <p className="mt-1 text-xs text-muted-foreground">
            Failed email extractions queued for regex review.
          </p>
        </div>

        <div className="inline-flex rounded-md border border-border bg-card p-1">
          {STATUS_FILTERS.map((filter) => (
            <button
              key={filter.value}
              type="button"
              aria-pressed={status === filter.value}
              onClick={() => setStatusFilter(filter.value)}
              className={cn(
                'rounded px-3 py-1.5 text-xs transition-colors',
                status === filter.value
                  ? 'bg-accent text-accent-foreground'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              {filter.label}
            </button>
          ))}
        </div>
      </div>

      {error ? (
        <div className="rounded border border-destructive/40 px-3 py-2 text-sm text-destructive">
          Failed to load diagnostics.
        </div>
      ) : null}

      <div className="overflow-x-auto rounded-lg border border-border">
        <table aria-label="Extraction diagnostics" className="w-full min-w-[980px] text-sm">
          <thead>
            <tr className="border-b border-border bg-secondary">
              {['Status', 'Message', 'Rule', 'Reasons', 'Received', ''].map((heading) => (
                <th
                  key={heading}
                  scope="col"
                  className="px-3 py-2 text-left text-xs uppercase tracking-wider text-muted-foreground"
                >
                  {heading}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {isLoading ? (
              <tr>
                <td colSpan={6} className="px-3 py-6 text-center text-xs text-muted-foreground">
                  Loading diagnostics...
                </td>
              </tr>
            ) : diagnostics.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-3 py-6 text-center text-xs text-muted-foreground">
                  No diagnostics in this state.
                </td>
              </tr>
            ) : (
              diagnostics.map((diagnostic) => (
                <tr key={diagnostic.id} className="align-top hover:bg-secondary/40">
                  <td className="px-3 py-3">
                    <span
                      className={cn(
                        'inline-flex rounded border px-2 py-0.5 text-xs capitalize',
                        statusClass(diagnostic.status),
                      )}
                    >
                      {diagnostic.status}
                    </span>
                  </td>
                  <td className="max-w-[320px] px-3 py-3">
                    <div className="font-medium text-foreground">{diagnostic.subject || '-'}</div>
                    <div className="mt-1 truncate text-xs text-muted-foreground">
                      {diagnostic.sender_email || diagnostic.sender || diagnostic.reader}
                    </div>
                    <div className="mt-1 truncate font-mono text-[11px] text-muted-foreground">
                      {diagnostic.message_id || diagnostic.id}
                    </div>
                  </td>
                  <td className="max-w-[220px] px-3 py-3">
                    <div className="font-medium text-foreground">
                      {diagnostic.rule_name || 'New rule'}
                    </div>
                    <div className="mt-1 truncate text-xs text-muted-foreground">
                      {diagnostic.source || diagnostic.reader}
                    </div>
                  </td>
                  <td className="px-3 py-3">
                    <div className="flex flex-wrap gap-1">
                      {diagnostic.failure_reasons.map((reason) => (
                        <span
                          key={reason}
                          className="rounded border border-border px-1.5 py-0.5 text-[11px] text-muted-foreground"
                        >
                          {reasonLabel(reason)}
                        </span>
                      ))}
                    </div>
                  </td>
                  <td className="whitespace-nowrap px-3 py-3 text-xs text-muted-foreground">
                    {formatDate(diagnostic.received_at ?? diagnostic.created_at)}
                  </td>
                  <td className="px-3 py-3">
                    <DiagnosticActions diagnostic={diagnostic} />
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {isFetching && !isLoading ? (
        <p className="text-xs text-muted-foreground">Refreshing diagnostics...</p>
      ) : null}
    </div>
  )
}

export default Diagnostics
