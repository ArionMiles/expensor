import {
  useApplyLabel,
  useCreateLabel,
  useDeleteLabel,
  useLabels,
  useUpdateLabel,
} from '@/api/queries'
import { useState } from 'react'

const PRESET_COLORS = [
  '#f59e0b',
  '#3b82f6',
  '#8b5cf6',
  '#06b6d4',
  '#10b981',
  '#ec4899',
  '#f97316',
  '#6366f1',
]

export function LabelsSettings() {
  const { data: labels = [], isLoading } = useLabels()
  const { mutate: createLabel, isPending: creating } = useCreateLabel()
  const { mutate: updateLabel } = useUpdateLabel()
  const { mutate: deleteLabel } = useDeleteLabel()
  const { mutate: applyLabel } = useApplyLabel()

  const [newName, setNewName] = useState('')
  const [newColor, setNewColor] = useState('#6366f1')
  const [applyState, setApplyState] = useState<{ name: string; pattern: string } | null>(null)

  const handleCreate = () => {
    if (!newName.trim()) return
    createLabel(
      { name: newName.trim(), color: newColor },
      {
        onSuccess: () => {
          setNewName('')
          setNewColor('#6366f1')
        },
      },
    )
  }

  if (isLoading) return <p className="text-xs text-muted-foreground">Loading...</p>

  return (
    <div className="space-y-6">
      {/* New label form */}
      <div className="flex items-end gap-3">
        <div>
          <label className="mb-1 block text-xs uppercase tracking-wider text-muted-foreground">
            Name
          </label>
          <input
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') handleCreate()
            }}
            placeholder="label name"
            className="rounded-md border border-border bg-input px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
        <div>
          <label className="mb-1 block text-xs uppercase tracking-wider text-muted-foreground">
            Color
          </label>
          <div className="flex gap-1.5">
            {PRESET_COLORS.map((c) => (
              <button
                key={c}
                onClick={() => setNewColor(c)}
                className={`h-6 w-6 rounded-full transition-transform ${newColor === c ? 'scale-125 ring-2 ring-offset-1 ring-offset-background' : ''}`}
                style={{ background: c }}
                aria-label={c}
              />
            ))}
          </div>
        </div>
        <button
          onClick={handleCreate}
          disabled={creating || !newName.trim()}
          className="rounded-md bg-primary px-4 py-1.5 text-sm text-primary-foreground hover:bg-primary/90 disabled:opacity-40"
        >
          + New label
        </button>
      </div>

      {/* Label table */}
      <div className="overflow-hidden rounded-lg border border-border">
        <table className="w-full">
          <thead>
            <tr className="border-b border-border bg-secondary/50">
              <th className="px-4 py-2.5 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                Color
              </th>
              <th className="px-4 py-2.5 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                Name
              </th>
              <th className="px-4 py-2.5 text-right text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                Actions
              </th>
            </tr>
          </thead>
          <tbody>
            {labels.map((l) => (
              <tr key={l.name} className="border-b border-border last:border-0">
                <td className="px-4 py-2.5">
                  <div className="flex gap-1">
                    {PRESET_COLORS.map((c) => (
                      <button
                        key={c}
                        onClick={() => updateLabel({ name: l.name, color: c })}
                        className={`h-4 w-4 rounded-full transition-transform ${l.color === c ? 'scale-125 ring-1 ring-offset-1 ring-offset-background' : 'opacity-50 hover:opacity-100'}`}
                        style={{ background: c }}
                        aria-label={`Set color ${c}`}
                      />
                    ))}
                  </div>
                </td>
                <td className="px-4 py-2.5">
                  <span className="flex items-center gap-2 text-sm">
                    <span className="h-2.5 w-2.5 rounded-full" style={{ background: l.color }} />
                    {l.name}
                  </span>
                </td>
                <td className="px-4 py-2.5 text-right">
                  <div className="flex items-center justify-end gap-2">
                    <button
                      onClick={() => setApplyState({ name: l.name, pattern: '' })}
                      className="text-xs text-muted-foreground transition-colors hover:text-foreground"
                    >
                      Apply to transactions
                    </button>
                    <button
                      onClick={() => deleteLabel(l.name)}
                      className="text-xs text-muted-foreground transition-colors hover:text-destructive"
                    >
                      Delete
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {labels.length === 0 && (
              <tr>
                <td colSpan={3} className="px-4 py-8 text-center text-xs text-muted-foreground">
                  No labels yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Apply-to-transactions modal */}
      {applyState && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm">
          <div className="w-full max-w-sm space-y-4 rounded-lg border border-border bg-card p-6 shadow-xl">
            <h2 className="text-sm font-semibold">
              Apply &quot;{applyState.name}&quot; to transactions
            </h2>
            <p className="text-xs text-muted-foreground">
              Applies this label to all transactions whose merchant name contains the pattern.
            </p>
            <input
              value={applyState.pattern}
              onChange={(e) => setApplyState({ ...applyState, pattern: e.target.value })}
              placeholder="e.g. swiggy"
              className="w-full rounded-md border border-border bg-input px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
              autoFocus
            />
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setApplyState(null)}
                className="px-4 py-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
              >
                Cancel
              </button>
              <button
                disabled={!applyState.pattern.trim()}
                onClick={() => {
                  applyLabel(
                    { name: applyState.name, pattern: applyState.pattern },
                    { onSuccess: () => setApplyState(null) },
                  )
                }}
                className="rounded-md bg-primary px-4 py-1.5 text-sm text-primary-foreground hover:bg-primary/90 disabled:opacity-40"
              >
                Apply
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
