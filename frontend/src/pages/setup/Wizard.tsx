import { api } from '@/api/client'
import {
  queryKeys,
  useDisconnectReader,
  useReaderAuthStatus,
  useReaderGuide,
  useReaderStatus,
  useReaders,
  useRevokeToken,
  useStatus,
} from '@/api/queries'
import type { PluginInfo, ReaderGuide } from '@/api/types'
import { ConfirmModal } from '@/components/ConfirmModal'
import { ReaderLogo } from '@/components/ReaderLogo'
import { cn, getReaderDisplayName } from '@/lib/utils'
import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { ConfigureStep } from './steps/ConfigureStep'
import { OAuthStep } from './steps/OAuthStep'
import { ReviewAndStart } from './steps/ReviewAndStart'
import { SelectReader } from './steps/SelectReader'
import { UploadCredentials } from './steps/UploadCredentials'

// ─── Helpers ─────────────────────────────────────────────────────────────────

function formatExpiry(expiry: string): string {
  const date = new Date(expiry)
  const now = new Date()
  const diffDays = Math.ceil((date.getTime() - now.getTime()) / 86_400_000)
  if (diffDays <= 0) return 'token expired'
  if (diffDays === 1) return 'expires tomorrow'
  if (diffDays < 30) return `expires in ${diffDays}d`
  if (diffDays < 365) return `expires in ${Math.floor(diffDays / 30)}mo`
  return `expires ${date.toLocaleDateString('en-US', { month: 'short', year: 'numeric' })}`
}

// ─── Reader guide panel ───────────────────────────────────────────────────────

function noteStyle(type: string): string {
  switch (type) {
    case 'warning':
      return 'border-l-2 border-warning/60 bg-warning/5 px-3 py-2 text-xs text-warning'
    case 'tip':
      return 'border-l-2 border-green-500/60 bg-green-500/5 px-3 py-2 text-xs text-green-500'
    case 'docker':
      return 'border-l-2 border-purple-500/60 bg-purple-500/5 px-3 py-2 text-xs text-purple-400'
    default:
      return 'border-l-2 border-blue-500/60 bg-blue-500/5 px-3 py-2 text-xs text-blue-400'
  }
}

function noteIcon(type: string): string {
  switch (type) {
    case 'warning':
      return '⚠'
    case 'tip':
      return '✓'
    case 'docker':
      return '🐳'
    default:
      return 'ℹ'
  }
}

function ReaderGuidePanel({ guide }: { guide: ReaderGuide }) {
  const [open, setOpen] = useState(true)

  return (
    <div className="w-72 shrink-0 self-start overflow-hidden rounded-lg border border-border bg-card shadow-sm">
      <button
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-center justify-between px-4 py-2.5 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground hover:text-foreground"
      >
        <span>Setup guide</span>
        <span>{open ? '▴' : '▾'}</span>
      </button>

      {open && (
        <div className="space-y-4 border-t border-border px-4 pb-4 pt-3">
          {guide.sections.map((section, i) => (
            <div key={i} className="space-y-1.5">
              <p className="text-xs font-semibold text-foreground">{section.title}</p>
              <ol className="space-y-1 pl-4">
                {section.steps.map((step, j) => (
                  <li key={j} className="list-decimal break-words text-xs text-muted-foreground">
                    {step.text}
                    {step.sub_steps && step.sub_steps.length > 0 && (
                      <ol className="mt-0.5 space-y-0.5 pl-4">
                        {step.sub_steps.map((sub, k) => (
                          <li
                            key={k}
                            className="list-[lower-alpha] break-words text-xs text-muted-foreground"
                          >
                            {sub}
                          </li>
                        ))}
                      </ol>
                    )}
                  </li>
                ))}
              </ol>
              {section.link && (
                <a
                  href={section.link.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-block text-xs text-primary hover:underline"
                >
                  {section.link.label} ↗
                </a>
              )}
            </div>
          ))}

          {guide.notes && guide.notes.length > 0 && (
            <div className="space-y-2 pt-1">
              {guide.notes.map((note, i) => (
                <div key={i} className={noteStyle(note.type)}>
                  <span className="mr-1.5">{noteIcon(note.type)}</span>
                  {note.text}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// ─── Wizard step flow ────────────────────────────────────────────────────────

type WizardStep = 'select' | 'credentials' | 'oauth' | 'configure' | 'review'

function getSteps(reader: PluginInfo | null): WizardStep[] {
  if (!reader) return ['select', 'review']
  const steps: WizardStep[] = ['select']
  if (reader.requires_credentials_upload) steps.push('credentials')
  if (reader.auth_type === 'oauth') steps.push('oauth')
  if (reader.config_schema.length > 0 || reader.auth_type === 'config') steps.push('configure')
  steps.push('review')
  return steps
}

const STEP_LABELS: Record<WizardStep, string> = {
  select: 'Select reader',
  credentials: 'Credentials',
  oauth: 'Authorize',
  configure: 'Configure',
  review: 'Review',
}

function WizardFlow({ initialReader }: { initialReader?: PluginInfo }) {
  const [selectedReader, setSelectedReader] = useState<PluginInfo | null>(initialReader ?? null)
  const [currentStep, setCurrentStep] = useState<WizardStep>(() => {
    if (initialReader) {
      const s = getSteps(initialReader)
      return s[1] ?? 'select'
    }
    return 'select'
  })

  const steps = getSteps(selectedReader)
  const currentIndex = steps.indexOf(currentStep)
  const { data: guide } = useReaderGuide(selectedReader?.name ?? '')

  const goNext = () => {
    const next = steps[currentIndex + 1]
    if (next) setCurrentStep(next)
  }
  const goBack = () => {
    const prev = steps[currentIndex - 1]
    if (prev) setCurrentStep(prev)
  }

  return (
    <div
      className={cn(
        'flex w-full items-start gap-6',
        guide && currentStep !== 'select' ? 'max-w-4xl' : 'max-w-lg',
      )}
    >
      <div className="w-full min-w-0 max-w-lg">
        {/* Step progress */}
        <div className="mb-8 flex items-center">
          {steps.map((step, idx) => (
            <div key={step} className="flex flex-1 items-center last:flex-none">
              <div className="flex flex-col items-center gap-1">
                <div
                  className={cn(
                    'flex h-6 w-6 items-center justify-center rounded-full border text-xs transition-colors',
                    idx < currentIndex
                      ? 'border-success bg-success/10 text-success'
                      : idx === currentIndex
                        ? 'border-primary bg-primary/10 text-primary'
                        : 'border-border text-muted-foreground',
                  )}
                >
                  {idx < currentIndex ? '✓' : idx + 1}
                </div>
                <span
                  className={cn(
                    'whitespace-nowrap text-[10px]',
                    idx === currentIndex ? 'text-primary' : 'text-muted-foreground',
                  )}
                >
                  {STEP_LABELS[step]}
                </span>
              </div>
              {idx < steps.length - 1 && (
                <div
                  className={cn(
                    'mx-2 mb-4 h-px flex-1 transition-colors',
                    idx < currentIndex ? 'bg-success' : 'bg-border',
                  )}
                />
              )}
            </div>
          ))}
        </div>

        <div className="rounded-lg border border-border bg-card p-6 shadow-sm">
          {currentStep === 'select' && (
            <SelectReader selected={selectedReader} onSelect={setSelectedReader} onNext={goNext} />
          )}
          {currentStep === 'credentials' && selectedReader && (
            <UploadCredentials readerName={selectedReader.name} onNext={goNext} onBack={goBack} />
          )}
          {currentStep === 'oauth' && selectedReader && (
            <OAuthStep readerName={selectedReader.name} onNext={goNext} onBack={goBack} />
          )}
          {currentStep === 'configure' && selectedReader && (
            <ConfigureStep
              readerName={selectedReader.name}
              configSchema={selectedReader.config_schema}
              onNext={goNext}
              onBack={goBack}
            />
          )}
          {currentStep === 'review' && selectedReader && (
            <ReviewAndStart reader={selectedReader} onBack={goBack} />
          )}
        </div>
      </div>
      {guide && currentStep !== 'select' && <ReaderGuidePanel guide={guide} />}
    </div>
  )
}

// ─── Overview: reader status cards ──────────────────────────────────────────

function InlineOAuthPanel({
  readerName,
  onSuccess,
}: {
  readerName: string
  onSuccess: () => void
}) {
  const [polling, setPolling] = useState(false)
  const [authStarted, setAuthStarted] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const { data: authStatus } = useReaderAuthStatus(readerName, polling ? 2000 : undefined)

  useEffect(() => {
    if (authStatus?.authenticated) {
      setPolling(false)
      onSuccess()
    }
  }, [authStatus?.authenticated, onSuccess])

  const handleStart = async () => {
    setError(null)
    try {
      const { data } = await api.readers.auth.start(readerName)
      window.open(data.url, '_blank', 'noopener,noreferrer')
      setAuthStarted(true)
      setPolling(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to start authorization')
    }
  }

  return (
    <div className="mt-4 space-y-3 border-t border-border pt-4">
      <p className="text-xs leading-relaxed text-muted-foreground">
        A Google authorization window will open in a new tab. Complete the flow there — this page
        will update automatically once authorized.
      </p>
      {error && (
        <p className="text-xs text-destructive" role="alert">
          {error}
        </p>
      )}
      <div className="flex flex-wrap items-center gap-4">
        <button
          onClick={handleStart}
          className="rounded-md bg-primary px-3 py-1.5 text-xs text-primary-foreground transition-colors hover:bg-primary/90"
        >
          {authStarted ? 'Reopen authorization tab →' : 'Open authorization tab →'}
        </button>
        {polling ? (
          <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-warning" />
            Waiting for authorization...
          </span>
        ) : authStarted ? (
          <button
            onClick={() => setPolling(true)}
            className="text-xs text-muted-foreground underline transition-colors hover:text-foreground"
          >
            Already authorized — check again
          </button>
        ) : null}
      </div>
    </div>
  )
}

type ReaderState = 'unconfigured' | 'needs-auth' | 'connected'

function ReaderCard({
  reader,
  onConfigure,
  justAuthorized,
}: {
  reader: PluginInfo
  onConfigure: (reader: PluginInfo) => void
  justAuthorized: boolean
}) {
  const qc = useQueryClient()
  const { data: status, isLoading } = useReaderStatus(reader.name)
  const revokeToken = useRevokeToken()
  const removeAll = useDisconnectReader()
  const [showAuthPanel, setShowAuthPanel] = useState(false)
  const [confirmAction, setConfirmAction] = useState<'disconnect' | 'removeAll' | null>(null)

  const isOAuth = reader.auth_type === 'oauth'
  const ready = status?.ready ?? false
  const authenticated = status?.authenticated ?? false
  const hasCredentials = isOAuth
    ? (status?.credentials_uploaded ?? false)
    : (status?.config_present ?? false)

  const readerState: ReaderState = ready
    ? 'connected'
    : hasCredentials && isOAuth && !authenticated
      ? 'needs-auth'
      : 'unconfigured'

  const { data: authDetails } = useReaderAuthStatus(reader.name, undefined, ready && isOAuth)

  const handleDisconnect = useCallback(() => {
    setConfirmAction('disconnect')
  }, [])

  const handleRemoveAll = useCallback(() => {
    setConfirmAction('removeAll')
  }, [])

  const executeConfirm = useCallback(async () => {
    if (confirmAction === 'disconnect') {
      await revokeToken.mutateAsync(reader.name)
      setShowAuthPanel(false)
    } else if (confirmAction === 'removeAll') {
      await removeAll.mutateAsync(reader.name)
      setShowAuthPanel(false)
    }
    setConfirmAction(null)
  }, [confirmAction, reader.name, revokeToken, removeAll])

  const handleAuthSuccess = useCallback(() => {
    setShowAuthPanel(false)
    qc.invalidateQueries({ queryKey: queryKeys.readerStatus(reader.name) })
    qc.invalidateQueries({ queryKey: queryKeys.readerAuthStatus(reader.name) })
  }, [qc, reader.name])

  const isBusy = revokeToken.isPending || removeAll.isPending

  const { data: statusData } = useStatus()
  const daemonRunning = statusData?.daemon?.running ?? false
  const [isStarting, setIsStarting] = useState(false)
  const [startError, setStartError] = useState<string | null>(null)

  const handleStartDaemon = useCallback(async () => {
    setIsStarting(true)
    setStartError(null)
    try {
      await api.daemon.start(reader.name)
      qc.invalidateQueries({ queryKey: queryKeys.status })
    } catch (err) {
      setStartError(err instanceof Error ? err.message : 'Failed to start daemon')
    } finally {
      setIsStarting(false)
    }
  }, [reader.name, qc])

  const stateBadge = {
    connected: (
      <span className="rounded-sm border border-success/50 bg-success/10 px-1.5 py-0.5 text-[10px] text-success">
        ● Connected
      </span>
    ),
    'needs-auth': (
      <span className="rounded-sm border border-warning/50 bg-warning/10 px-1.5 py-0.5 text-[10px] text-warning">
        ○ Auth required
      </span>
    ),
    unconfigured: (
      <span className="rounded-sm border border-border px-1.5 py-0.5 text-[10px] text-muted-foreground">
        ○ Not configured
      </span>
    ),
  }[readerState]

  return (
    <>
      <div
        className={cn(
          'overflow-hidden rounded-lg border bg-card shadow-sm transition-colors',
          justAuthorized ? 'border-success/50' : 'border-border',
        )}
      >
        {/* Colored left stripe */}
        <div className="flex">
          <div
            className={cn(
              'w-0.5 flex-shrink-0',
              readerState === 'connected'
                ? 'bg-success'
                : readerState === 'needs-auth'
                  ? 'bg-warning'
                  : 'bg-border',
            )}
          />

          <div className="min-w-0 flex-1">
            {/* Header */}
            <div className="flex items-start justify-between gap-4 px-5 pb-3 pt-4">
              <div className="flex min-w-0 items-start gap-3">
                <ReaderLogo name={reader.name} className="mt-0.5 h-8 w-8 flex-shrink-0" />
                <div className="min-w-0">
                  <div className="mb-0.5 flex flex-wrap items-center gap-2">
                    <span className="text-sm font-semibold text-foreground">
                      {getReaderDisplayName(reader.name)}
                    </span>
                    {!isLoading && stateBadge}
                    {justAuthorized && (
                      <span className="text-[10px] text-success">✓ just authorized</span>
                    )}
                  </div>
                  <p className="text-xs text-muted-foreground">{reader.description}</p>
                </div>
              </div>
              {readerState === 'connected' && isOAuth && authDetails?.expiry && (
                <span className="mt-0.5 flex-shrink-0 text-[10px] text-muted-foreground">
                  {formatExpiry(authDetails.expiry)}
                </span>
              )}
            </div>

            {/* Context message */}
            {!isLoading && readerState === 'needs-auth' && !showAuthPanel && (
              <div className="px-5 pb-3">
                <p className="text-xs text-warning/90">
                  Credentials uploaded. Complete OAuth authorization to grant read access to Gmail.
                </p>
              </div>
            )}
            {!isLoading && readerState === 'unconfigured' && (
              <div className="px-5 pb-3">
                <p className="text-xs text-muted-foreground">
                  {isOAuth
                    ? 'Requires a Google OAuth client secret file and account authorization.'
                    : 'Requires mailbox configuration to specify which emails to read.'}
                </p>
              </div>
            )}

            {/* Inline OAuth panel */}
            {showAuthPanel && (
              <div className="px-5 pb-4">
                <InlineOAuthPanel readerName={reader.name} onSuccess={handleAuthSuccess} />
              </div>
            )}

            {/* Actions */}
            {!isLoading && (
              <>
                <div className="flex flex-wrap items-center justify-between gap-3 px-5 pb-4">
                  <div className="flex items-center gap-2">
                    {readerState === 'unconfigured' && (
                      <button
                        onClick={() => onConfigure(reader)}
                        className="rounded-md bg-primary px-3 py-1.5 text-xs text-primary-foreground transition-colors hover:bg-primary/90"
                      >
                        Set up →
                      </button>
                    )}

                    {readerState === 'connected' && !daemonRunning && (
                      <button
                        onClick={handleStartDaemon}
                        disabled={isStarting || isBusy}
                        className="rounded-md bg-success px-3 py-1.5 text-xs text-success-foreground transition-colors hover:bg-success/90 disabled:cursor-not-allowed disabled:opacity-40"
                      >
                        {isStarting ? 'Starting...' : 'Start tracking →'}
                      </button>
                    )}

                    {(readerState === 'needs-auth' || readerState === 'connected') && isOAuth && (
                      <button
                        onClick={() => setShowAuthPanel(!showAuthPanel)}
                        disabled={isBusy}
                        className={cn(
                          'rounded-md border px-3 py-1.5 text-xs transition-colors disabled:opacity-40',
                          showAuthPanel
                            ? 'border-primary bg-primary/10 text-primary'
                            : 'border-primary text-primary hover:bg-primary hover:text-primary-foreground',
                        )}
                      >
                        {readerState === 'connected' ? 'Re-authorize' : 'Authorize →'}
                      </button>
                    )}

                    {readerState === 'connected' && isOAuth && (
                      <button
                        onClick={handleDisconnect}
                        disabled={isBusy}
                        className="rounded-md border border-border px-3 py-1.5 text-xs text-muted-foreground transition-colors hover:border-destructive hover:text-destructive disabled:opacity-40"
                      >
                        {revokeToken.isPending ? '...' : 'Disconnect'}
                      </button>
                    )}
                  </div>

                  {readerState !== 'unconfigured' && (
                    <button
                      onClick={handleRemoveAll}
                      disabled={isBusy}
                      className="text-[10px] text-muted-foreground transition-colors hover:text-destructive disabled:opacity-40"
                    >
                      {removeAll.isPending ? 'Removing...' : 'Remove all data'}
                    </button>
                  )}
                </div>
                {startError && <p className="px-5 pb-3 text-xs text-destructive">{startError}</p>}
              </>
            )}
          </div>
        </div>
      </div>

      {confirmAction === 'disconnect' && (
        <ConfirmModal
          title={`Disconnect ${getReaderDisplayName(reader.name)}?`}
          message="This revokes the OAuth token. Your credentials file is kept, so you can re-authorize without re-uploading."
          confirmLabel="Disconnect"
          variant="destructive"
          onConfirm={executeConfirm}
          onCancel={() => setConfirmAction(null)}
        />
      )}
      {confirmAction === 'removeAll' && (
        <ConfirmModal
          title={`Remove all data for ${getReaderDisplayName(reader.name)}?`}
          message="This permanently deletes the credentials file, token, and saved config. You will need to go through the full setup again."
          confirmLabel="Remove all data"
          variant="destructive"
          onConfirm={executeConfirm}
          onCancel={() => setConfirmAction(null)}
        />
      )}
    </>
  )
}

function SetupOverview({
  onConfigure,
  justAuthorizedReader,
}: {
  onConfigure: (reader: PluginInfo) => void
  justAuthorizedReader: string | null
}) {
  const { data: readers, isLoading, error } = useReaders()

  return (
    <div className="w-full max-w-lg">
      <div className="mb-8">
        <p className="mb-2 text-xs uppercase tracking-widest text-muted-foreground">Setup</p>
        <h1 className="mb-1 text-lg font-semibold text-foreground">Reader configuration</h1>
        <p className="text-sm text-muted-foreground">
          Configure how Expensor reads your bank transaction emails. The active reader is used by
          the background daemon to extract and store new transactions.
        </p>
      </div>

      {isLoading && (
        <div className="space-y-3">
          {[0, 1].map((i) => (
            <div
              key={i}
              className="animate-pulse rounded-lg border border-border bg-card p-5 shadow-sm"
            >
              <div className="mb-2 h-3 w-24 rounded bg-secondary" />
              <div className="h-2.5 w-48 rounded bg-secondary" />
            </div>
          ))}
        </div>
      )}

      {error && (
        <div className="rounded-lg border border-destructive bg-card p-4">
          <p className="text-xs text-destructive">
            Failed to load readers: {error instanceof Error ? error.message : 'unknown error'}
          </p>
        </div>
      )}

      {readers && (
        <div className="space-y-3">
          {readers.map((reader) => (
            <ReaderCard
              key={reader.name}
              reader={reader}
              onConfigure={onConfigure}
              justAuthorized={justAuthorizedReader === reader.name}
            />
          ))}
        </div>
      )}
    </div>
  )
}

// ─── Main entry point ────────────────────────────────────────────────────────

export function Wizard() {
  const [mode, setMode] = useState<'overview' | 'wizard'>('overview')
  const [configReader, setConfigReader] = useState<PluginInfo | null>(null)
  const [searchParams, setSearchParams] = useSearchParams()
  const qc = useQueryClient()

  const authSuccess = searchParams.get('auth') === 'success'
  const authReader = searchParams.get('reader') ?? null

  useEffect(() => {
    if (authSuccess && authReader) {
      qc.invalidateQueries({ queryKey: queryKeys.readerStatus(authReader) })
      qc.invalidateQueries({ queryKey: queryKeys.readerAuthStatus(authReader) })
      setSearchParams({}, { replace: true })
    }
  }, [authSuccess, authReader, qc, setSearchParams])

  const handleConfigure = (reader: PluginInfo) => {
    setConfigReader(reader)
    setMode('wizard')
  }

  return (
    <div className="flex flex-1 flex-col">
      <div className="flex flex-1 flex-col items-center justify-center px-4 py-12">
        {mode === 'overview' ? (
          <SetupOverview
            onConfigure={handleConfigure}
            justAuthorizedReader={authSuccess ? authReader : null}
          />
        ) : (
          <WizardFlow initialReader={configReader ?? undefined} />
        )}
      </div>
    </div>
  )
}

export default Wizard
