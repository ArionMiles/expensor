import { api } from '@/api/client'
import { useStatus } from '@/api/queries'
import type { PluginInfo } from '@/api/types'
import { cn } from '@/lib/utils'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

interface ReviewAndStartProps {
  reader: PluginInfo
  onBack: () => void
}

type StartState = 'idle' | 'starting' | 'polling' | 'done' | 'error'

export function ReviewAndStart({ reader, onBack }: ReviewAndStartProps) {
  const navigate = useNavigate()
  const [startState, setStartState] = useState<StartState>('idle')
  const [startError, setStartError] = useState<string | null>(null)

  const { data: statusData } = useStatus(startState === 'polling')

  useEffect(() => {
    if (startState === 'polling' && statusData?.daemon?.running) {
      setStartState('done')
      const timer = setTimeout(() => navigate('/'), 1500)
      return () => clearTimeout(timer)
    }
  }, [startState, statusData?.daemon?.running, navigate])

  const handleStart = async () => {
    setStartError(null)
    setStartState('starting')
    try {
      await api.daemon.start(reader.name)
      setStartState('polling')
    } catch (err) {
      setStartError(err instanceof Error ? err.message : 'Failed to start daemon')
      setStartState('error')
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-base font-semibold text-foreground mb-1">Review & start</h2>
        <p className="text-sm text-muted-foreground">
          Confirm your configuration and start the daemon.
        </p>
      </div>

      <div className="rounded-lg border border-border overflow-hidden">
        {[
          ['Reader', reader.name],
          ['Auth type', reader.auth_type],
          ['Credentials', reader.requires_credentials_upload ? 'Uploaded' : 'Not required'],
          [
            'Config fields',
            reader.config_schema.length > 0
              ? `${reader.config_schema.length} fields saved`
              : 'None',
          ],
        ].map(([label, value], i) => (
          <div
            key={label}
            className={cn(
              'flex items-center justify-between px-4 py-2.5',
              i > 0 && 'border-t border-border',
            )}
          >
            <span className="text-xs text-muted-foreground uppercase tracking-wider">{label}</span>
            <span className="text-sm text-foreground font-mono">{value}</span>
          </div>
        ))}
      </div>

      {startState === 'polling' && (
        <div className="flex items-center gap-2 p-3 rounded-lg border border-border bg-secondary/30">
          <span className="w-1.5 h-1.5 rounded-full bg-warning animate-pulse" aria-hidden="true" />
          <span className="text-xs text-warning">Starting daemon, polling status...</span>
        </div>
      )}

      {startState === 'done' && (
        <div className="flex items-center gap-2 p-3 rounded-lg border border-success/30 bg-success/10">
          <span className="w-1.5 h-1.5 rounded-full bg-success" aria-hidden="true" />
          <span className="text-xs text-success">Daemon running — redirecting to dashboard...</span>
        </div>
      )}

      {startState === 'error' && startError && (
        <p className="text-xs text-destructive" role="alert">
          {startError}
        </p>
      )}

      <div className="flex items-center justify-between">
        <button
          onClick={onBack}
          disabled={startState === 'polling' || startState === 'done'}
          className="px-4 py-2 text-sm text-muted-foreground hover:text-foreground transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
        >
          ← Back
        </button>
        <button
          onClick={handleStart}
          disabled={startState === 'polling' || startState === 'done' || startState === 'starting'}
          className={cn(
            'px-5 py-2 text-sm rounded-md transition-colors',
            startState === 'idle' || startState === 'error'
              ? 'bg-success text-success-foreground hover:bg-success/90'
              : 'bg-secondary text-muted-foreground cursor-not-allowed opacity-50',
          )}
        >
          {startState === 'idle' || startState === 'error' ? 'Start daemon' : 'Starting...'}
        </button>
      </div>
    </div>
  )
}
