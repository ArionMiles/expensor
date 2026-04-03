import { useCategories, useCreateCategory, useDeleteCategory } from '@/api/queries'
import { useState } from 'react'

export function CategoriesSettings() {
  const { data: categories = [], isLoading } = useCategories()
  const { mutate: create, isPending } = useCreateCategory()
  const { mutate: remove } = useDeleteCategory()
  const [newName, setNewName] = useState('')
  const [error, setError] = useState<string | null>(null)

  const handleCreate = () => {
    if (!newName.trim()) return
    setError(null)
    create(
      { name: newName.trim() },
      {
        onSuccess: () => setNewName(''),
        onError: (e) => setError(e instanceof Error ? e.message : 'Failed to create'),
      },
    )
  }

  if (isLoading) return <p className="text-xs text-muted-foreground">Loading...</p>

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <input
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') handleCreate()
          }}
          placeholder="New category name"
          className="rounded-md border border-border bg-input px-3 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
        />
        <button
          onClick={handleCreate}
          disabled={isPending || !newName.trim()}
          className="rounded-md bg-primary px-4 py-1.5 text-sm text-primary-foreground hover:bg-primary/90 disabled:opacity-40"
        >
          + Add
        </button>
      </div>
      {error && <p className="text-xs text-destructive">{error}</p>}
      <div className="overflow-hidden rounded-lg border border-border">
        <table className="w-full">
          <thead>
            <tr className="border-b border-border bg-secondary/50">
              <th className="px-4 py-2.5 text-left text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                Name
              </th>
              <th className="px-4 py-2.5 text-right text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                Actions
              </th>
            </tr>
          </thead>
          <tbody>
            {categories.map((c) => (
              <tr key={c.name} className="border-b border-border last:border-0">
                <td className="px-4 py-2.5 text-sm">
                  {c.name}
                  {c.is_default && (
                    <span className="ml-2 rounded-sm border border-border px-1 py-0.5 text-[10px] text-muted-foreground">
                      Default
                    </span>
                  )}
                </td>
                <td className="px-4 py-2.5 text-right">
                  <button
                    disabled={c.is_default}
                    onClick={() => remove(c.name)}
                    className="text-xs text-muted-foreground transition-colors hover:text-destructive disabled:cursor-not-allowed disabled:opacity-30"
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
