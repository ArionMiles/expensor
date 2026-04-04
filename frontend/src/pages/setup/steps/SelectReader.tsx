import { useState } from 'react'
import { useReaderGuide, useReaders } from '@/api/queries'
import type { PluginInfo, ReaderGuide } from '@/api/types'
import { ReaderLogo } from '@/components/ReaderLogo'
import { cn, getReaderDisplayName } from '@/lib/utils'

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
    <div className="mt-4 rounded-lg border border-border bg-card">
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
                  <li key={j} className="list-decimal text-xs text-muted-foreground">
                    {step}
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

function ReaderGuideWrapper({ name }: { name: string }) {
  const { data: guide } = useReaderGuide(name)
  if (!guide) return null
  return <ReaderGuidePanel guide={guide} />
}

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

      {selected && <ReaderGuideWrapper name={selected.name} />}

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
