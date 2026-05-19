import { useState } from 'react'
import { api } from '@/api/client'
import { cn } from '@/lib/utils'

interface ManualOAuthSectionProps {
  readerName: string
  redirectUri: string
  onSuccess: () => void
}

export function ManualOAuthSection({
  readerName,
  redirectUri,
  onSuccess,
}: ManualOAuthSectionProps) {
  const [open, setOpen] = useState(false)
  const [callbackUrl, setCallbackUrl] = useState('')
  const [isPending, setIsPending] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleExchange = async () => {
    setError(null)
    setIsPending(true)
    try {
      await api.readers.auth.exchange(readerName, callbackUrl)
      onSuccess()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Exchange failed')
    } finally {
      setIsPending(false)
    }
  }

  return (
    <div className="border-t border-border pt-3">
      <button
        onClick={() => setOpen((o) => !o)}
        className="flex items-center gap-1.5 text-xs text-muted-foreground transition-colors hover:text-foreground"
      >
        <span>{open ? '▴' : '▾'}</span>
        <span>Can&apos;t complete the redirect? Paste the URL manually</span>
      </button>

      {open && (
        <div className="mt-3 space-y-3">
          <p className="text-xs leading-relaxed text-muted-foreground">
            If Google redirects to a URL that returns a 404 (common on homeservers without a public
            domain), copy the full URL from your browser&apos;s address bar and paste it below.
          </p>
          <div className="rounded-md border border-border bg-secondary/40 px-3 py-2">
            <p className="mb-0.5 text-[10px] uppercase tracking-wider text-muted-foreground">
              Expected redirect URI
            </p>
            <p className="break-all font-mono text-xs text-foreground">{redirectUri}</p>
          </div>
          <textarea
            value={callbackUrl}
            onChange={(e) => {
              setCallbackUrl(e.target.value)
              setError(null)
            }}
            placeholder={`${redirectUri}?code=4/0A...&state=...`}
            rows={3}
            className={cn(
              'w-full resize-none rounded-md border border-border bg-secondary px-3 py-2',
              'font-mono text-xs text-foreground placeholder:text-muted-foreground',
              'focus:border-primary focus:outline-none focus:ring-1 focus:ring-ring',
            )}
          />
          {error && (
            <p className="text-xs text-destructive" role="alert">
              {error}
            </p>
          )}
          <button
            onClick={handleExchange}
            disabled={!callbackUrl.trim() || isPending}
            className={cn(
              'rounded-md px-3 py-1.5 text-xs transition-colors',
              callbackUrl.trim() && !isPending
                ? 'bg-primary text-primary-foreground hover:bg-primary/90'
                : 'cursor-not-allowed bg-secondary text-muted-foreground opacity-50',
            )}
          >
            {isPending ? 'Exchanging...' : 'Exchange token →'}
          </button>
        </div>
      )}
    </div>
  )
}
