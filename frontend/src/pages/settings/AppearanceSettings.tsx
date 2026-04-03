import { api } from '@/api/client'
import { useEffect, useState } from 'react'

export function AppearanceSettings() {
  const [currency, setCurrency] = useState('')
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.status
      .get()
      .then((r) => {
        const c = r.data.stats?.base_currency
        if (c) setCurrency(c)
      })
      .catch(() => {
        // leave empty if unavailable
      })
      .finally(() => setLoading(false))
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaved(false)
    setError(null)
    try {
      await api.config.setBaseCurrency(currency.toUpperCase().trim())
      setSaved(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save')
    }
  }

  if (loading) return <p className="text-xs text-muted-foreground">Loading...</p>

  return (
    <form onSubmit={handleSubmit} className="max-w-sm space-y-4">
      <div>
        <label className="mb-1.5 block text-xs uppercase tracking-wider text-muted-foreground">
          Base currency
        </label>
        <input
          value={currency}
          onChange={(e) => {
            setCurrency(e.target.value)
            setSaved(false)
          }}
          placeholder="INR"
          maxLength={3}
          className="w-full rounded-md border border-border bg-input px-3 py-2 font-mono text-sm uppercase"
        />
        <p className="mt-1 text-xs text-muted-foreground">
          3-letter ISO 4217 code used for aggregate totals (e.g. INR, USD, EUR).
        </p>
      </div>
      {saved && <p className="text-xs text-success">Saved.</p>}
      {error && <p className="text-xs text-destructive">{error}</p>}
      <button
        type="submit"
        disabled={currency.length !== 3}
        className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-40"
      >
        Save
      </button>
    </form>
  )
}
