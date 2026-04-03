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
          <div key={i} className="p-4 rounded-md border border-border bg-secondary/30 animate-pulse">
            <div className="h-3 w-24 bg-secondary rounded mb-2" />
            <div className="h-2.5 w-48 bg-secondary rounded" />
          </div>
        ))}
      </div>
    )
  }

  if (error || !readers) {
    return (
      <div className="p-4 rounded-md border border-destructive text-sm text-destructive">
        Failed to load readers: {error instanceof Error ? error.message : 'unknown error'}
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-base font-semibold text-foreground mb-1">Select a reader</h2>
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
              'w-full text-left p-4 rounded-lg border transition-colors',
              'focus:outline-none focus-visible:ring-2 focus-visible:ring-ring',
              selected?.name === reader.name
                ? 'border-primary bg-primary/10'
                : 'border-border bg-card hover:bg-accent hover:border-border',
            )}
            aria-pressed={selected?.name === reader.name}
          >
            <div className="flex items-start justify-between gap-4">
              <div className="flex items-start gap-3">
                <ReaderLogo name={reader.name} className="w-7 h-7 mt-0.5 flex-shrink-0" />
                <div>
                  <div className="flex items-center gap-2 mb-0.5">
                    <span className="text-sm font-medium text-foreground">{getReaderDisplayName(reader.name)}</span>
                    <span className="text-[10px] px-1.5 py-0.5 rounded-sm border border-border text-muted-foreground">
                      {reader.auth_type}
                    </span>
                  </div>
                  <p className="text-xs text-muted-foreground">{reader.description}</p>
                </div>
              </div>
              {selected?.name === reader.name && (
                <span className="text-primary text-sm flex-shrink-0">✓</span>
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
            'px-4 py-2 text-sm rounded-md transition-colors',
            'focus:outline-none focus-visible:ring-2 focus-visible:ring-ring',
            selected
              ? 'bg-primary text-primary-foreground hover:bg-primary/90'
              : 'bg-secondary text-muted-foreground cursor-not-allowed opacity-50',
          )}
        >
          Next →
        </button>
      </div>
    </div>
  )
}
