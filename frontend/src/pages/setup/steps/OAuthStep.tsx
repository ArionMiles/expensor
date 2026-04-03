import { api } from '@/api/client'
import { useReaderAuthStatus } from '@/api/queries'
import { cn } from '@/lib/utils'
import { useEffect, useState } from 'react'

interface OAuthStepProps {
  readerName: string
  onNext: () => void
  onBack: () => void
}

export function OAuthStep({ readerName, onNext, onBack }: OAuthStepProps) {
  const [polling, setPolling] = useState(false)
  const [authError, setAuthError] = useState<string | null>(null)
  const [authStarted, setAuthStarted] = useState(false)

  const { data: authStatus } = useReaderAuthStatus(readerName, polling ? 2000 : undefined)

  useEffect(() => {
    if (authStatus?.authenticated) {
      setPolling(false)
      onNext()
    }
  }, [authStatus?.authenticated, onNext])

  const handleAuthorize = async () => {
    setAuthError(null)
    try {
      const { data } = await api.readers.auth.start(readerName)
      window.open(data.url, '_blank', 'noopener,noreferrer')
      setAuthStarted(true)
      setPolling(true)
    } catch (err) {
      setAuthError(err instanceof Error ? err.message : 'Failed to start authorization')
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-base font-semibold text-foreground mb-1">Authorize with Google</h2>
        <p className="text-sm text-muted-foreground">
          Grant Expensor read access to your Gmail messages for bank transaction emails.
        </p>
      </div>

      <div className="p-4 rounded-lg border border-border bg-secondary/30 space-y-3">
        <div className="flex items-center gap-2">
          <span
            className={cn(
              'w-1.5 h-1.5 rounded-full',
              polling ? 'bg-warning animate-pulse' : 'bg-muted-foreground',
            )}
            aria-hidden="true"
          />
          <span className="text-xs text-muted-foreground">
            {authStatus?.authenticated
              ? 'Authorized'
              : polling
                ? 'Waiting for authorization...'
                : authStarted
                  ? 'Complete authorization in the browser tab'
                  : 'Not yet authorized'}
          </span>
        </div>
        {polling && (
          <p className="text-xs text-muted-foreground">Polling every 2s...</p>
        )}
      </div>

      {authError && (
        <p className="text-xs text-destructive" role="alert">
          {authError}
        </p>
      )}

      <div className="space-y-2">
        <button
          onClick={handleAuthorize}
          className="w-full px-4 py-2.5 text-sm rounded-md bg-primary text-primary-foreground hover:bg-primary/90 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {authStarted ? 'Reopen authorization tab' : 'Authorize with Google →'}
        </button>

        {authStarted && !polling && (
          <button
            onClick={() => setPolling(true)}
            className="w-full px-4 py-2 text-xs text-muted-foreground hover:text-foreground transition-colors"
          >
            Already authorized — check again
          </button>
        )}
      </div>

      <div className="flex items-center justify-between">
        <button
          onClick={onBack}
          className="px-4 py-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          ← Back
        </button>
      </div>
    </div>
  )
}
