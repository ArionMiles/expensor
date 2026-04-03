import { useReaders } from '@/api/queries'
import type { PluginInfo } from '@/api/types'
import { ReaderLogo } from '@/components/ReaderLogo'
import { cn, getReaderDisplayName } from '@/lib/utils'

interface SelectReaderProps {
  selected: PluginInfo | null
  onSelect: (reader: PluginInfo) => void
  onNext: () => void
}

export function SelectReader({ selected, onSelect, onNext }: SelectReaderProps) {
  const { data: readers, isLoading, error } = useReaders()

  if (isLoading) {
    return (
      <div className="space-y-3">
        {[0, 1].map((i) => (
          <div
            key={i}
            className="animate-pulse rounded-md border border-border bg-secondary/30 p-4"
          >
            <div className="mb-2 h-3 w-24 rounded bg-secondary" />
            <div className="h-2.5 w-48 rounded bg-secondary" />
          </div>
        ))}
      </div>
    )
  }

  if (error || !readers) {
    return (
      <div className="rounded-md border border-destructive p-4 text-sm text-destructive">
        Failed to load readers: {error instanceof Error ? error.message : 'unknown error'}
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="mb-1 text-base font-semibold text-foreground">Select a reader</h2>
        <p className="text-sm text-muted-foreground">
          Choose how to import your transaction emails.
        </p>
      </div>

      <div className="grid gap-2">
        {readers.map((reader) => (
          <button
            key={reader.name}
            onClick={() => onSelect(reader)}
            className={cn(
              'w-full rounded-lg border p-4 text-left transition-colors',
              'focus:outline-none focus-visible:ring-2 focus-visible:ring-ring',
              selected?.name === reader.name
                ? 'border-primary bg-primary/10'
                : 'border-border bg-card hover:border-border hover:bg-accent',
            )}
            aria-pressed={selected?.name === reader.name}
          >
            <div className="flex items-start justify-between gap-4">
              <div className="flex items-start gap-3">
                <ReaderLogo name={reader.name} className="mt-0.5 h-7 w-7 flex-shrink-0" />
                <div>
                  <div className="mb-0.5 flex items-center gap-2">
                    <span className="text-sm font-medium text-foreground">
                      {getReaderDisplayName(reader.name)}
                    </span>
                    <span className="rounded-sm border border-border px-1.5 py-0.5 text-[10px] text-muted-foreground">
                      {reader.auth_type}
                    </span>
                  </div>
                  <p className="text-xs text-muted-foreground">{reader.description}</p>
                </div>
              </div>
              {selected?.name === reader.name && (
                <span className="flex-shrink-0 text-sm text-primary">✓</span>
              )}
            </div>
          </button>
        ))}
      </div>

      <div className="flex justify-end">
        <button
          onClick={onNext}
          disabled={!selected}
          className={cn(
            'rounded-md px-4 py-2 text-sm transition-colors',
            'focus:outline-none focus-visible:ring-2 focus-visible:ring-ring',
            selected
              ? 'bg-primary text-primary-foreground hover:bg-primary/90'
              : 'cursor-not-allowed bg-secondary text-muted-foreground opacity-50',
          )}
        >
          Next →
        </button>
      </div>
    </div>
  )
}
