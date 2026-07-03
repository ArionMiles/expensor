export function toggleOrderedSelection({
  orderedIds,
  selectedIds,
  id,
  anchorId,
  extendRange,
}: {
  orderedIds: string[]
  selectedIds: Set<string>
  id: string
  anchorId: string | null
  extendRange: boolean
}) {
  const next = new Set(selectedIds)
  const shouldSelect = !next.has(id)
  const anchorIndex = anchorId ? orderedIds.indexOf(anchorId) : -1
  const currentIndex = orderedIds.indexOf(id)

  if (extendRange && anchorIndex >= 0 && currentIndex >= 0) {
    const [start, end] =
      anchorIndex < currentIndex ? [anchorIndex, currentIndex] : [currentIndex, anchorIndex]
    orderedIds.slice(start, end + 1).forEach((rangeId) => {
      if (shouldSelect) next.add(rangeId)
      else next.delete(rangeId)
    })
    return { selectedIds: next, anchorId: id }
  }

  if (shouldSelect) next.add(id)
  else next.delete(id)
  return { selectedIds: next, anchorId: id }
}
